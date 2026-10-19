// src/PsychDashboardTab.tsx
import React, { useEffect, useState } from "react";
import type { Child } from "./types";
import { apiFetch } from "./api";

// Re-use the same types as in ChildrenTab
type DomainIndex = {
  domain: string;
  average_wellbeing: number;
  average_progress: number;
  index: number;
  previous_index?: number | null;
  change?: number | null;
};

type OverallIndex = {
  index: number;
  previous_index?: number | null;
  change?: number | null;
};

type ChildDomainIndex = {
  child_id: number;
  window_days: number;
  overall: OverallIndex;
  domains: DomainIndex[];
};

type EarlyWarningLevel = "ok" | "amber" | "red";

type ChildSignal = {
  level: EarlyWarningLevel;
  message: string;
};

type Props = {
  apiBase: string;
  homeId: number;
};

function findDomain(domains: DomainIndex[], needle: string): DomainIndex | undefined {
  const n = needle.toLowerCase();
  return domains.find((d) => d.domain.toLowerCase().includes(n));
}

function deriveSignalFromIndex(idx: ChildDomainIndex): ChildSignal {
  const risk = findDomain(idx.domains, "risk") || findDomain(idx.domains, "safety");
  const emotional = findDomain(idx.domains, "emotional");
  const behaviour = findDomain(idx.domains, "behaviour");

  let level: EarlyWarningLevel = "ok";
  let message = "No current trend alerts.";

  if (risk) {
    const riskChange = risk.change ?? 0;
    if (risk.index >= 7.5 || riskChange >= 3) {
      return {
        level: "red",
        message: "Risk / safety has increased significantly over this window.",
      };
    }
  }

  const emoChange = emotional?.change ?? 0;
  const behChange = behaviour?.change ?? 0;

  if (emoChange <= -2 || behChange <= -2) {
    return {
      level: "amber",
      message: "Emotional / behaviour scores have worsened since last window.",
    };
  }

  return { level, message };
}

function getSignalLabel(level: EarlyWarningLevel): string {
  switch (level) {
    case "red":
      return "High concern";
    case "amber":
      return "Watch closely";
    default:
      return "OK";
  }
}

function signalClass(level: EarlyWarningLevel): string {
  return `signal-pill signal-pill-${level}`;
}

const PsychDashboardTab: React.FC<Props> = ({ apiBase, homeId }) => {
  const [children, setChildren] = useState<Child[]>([]);
  const [indices, setIndices] = useState<Record<number, ChildDomainIndex>>({});
  const [signals, setSignals] = useState<Record<number, ChildSignal>>({});
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Load children
  useEffect(() => {
    const load = async () => {
      setLoading(true);
      setError(null);
      try {
        const res = await apiFetch(`${apiBase}/v1/homes/${homeId}/children`);
        if (!res.ok) {
          const txt = await res.text();
          throw new Error(txt || "Failed to load children");
        }
        const data: Child[] = await res.json();
        setChildren(data);
      } catch (err: any) {
        setError(err.message || "Could not load children");
        setChildren([]);
      } finally {
        setLoading(false);
      }
    };
    load();
  }, [apiBase, homeId]);

  // Load domain indices + signals
  useEffect(() => {
    const loadIndices = async () => {
      if (children.length === 0) {
        setIndices({});
        setSignals({});
        return;
      }

      try {
        const newIndices: Record<number, ChildDomainIndex> = {};
        const newSignals: Record<number, ChildSignal> = {};

        await Promise.all(
          children.map(async (c) => {
            try {
              const res = await apiFetch(
                `${apiBase}/v1/children/${c.id}/domain_index?window_days=42`
              );
              if (!res.ok) return;

              const idx: ChildDomainIndex = await res.json();
              newIndices[c.id] = idx;

              if (idx.domains && idx.domains.length > 0) {
                newSignals[c.id] = deriveSignalFromIndex(idx);
              }
            } catch {
              // ignore per child
            }
          })
        );

        setIndices(newIndices);
        setSignals(newSignals);
      } catch {
        // global errors already handled above
      }
    };

    loadIndices();
  }, [children, apiBase]);

  const renderChildName = (c: Child) => {
    if (c.first_name || c.last_name) {
      return `${c.first_name ?? ""} ${c.last_name ?? ""}`.trim();
    }
    return `Child ${c.id}`;
  };

  const getDomainIndex = (childId: number, needle: string): DomainIndex | undefined => {
    const idx = indices[childId];
    if (!idx) return undefined;
    return findDomain(idx.domains, needle);
  };

  return (
    <section className="card">
      <div className="card-header">
        <div>
          <h2>Psychology dashboard</h2>
          <p className="card-subtitle">
            Cross-home view for this setting — Pathway Index, risk and trend
            alerts across all children.
          </p>
        </div>
      </div>

      {error && (
        <div className="alert alert-error">
          <strong>Error:</strong> {error}
        </div>
      )}

      {loading && <p className="field-hint">Loading children…</p>}

      {!loading && children.length === 0 && (
        <p className="field-hint">
          No children onboarded for this home yet.
        </p>
      )}

      {!loading && children.length > 0 && (
        <div className="chart-card">
          <div className="chart-header">
            <h3>Pathway Index & trend alerts</h3>
            <p className="field-hint">
              42-day window · comparing current vs previous period.
            </p>
          </div>

          <table className="simple-table">
            <thead>
              <tr>
                <th>Child</th>
                <th>Overall index</th>
                <th>Risk / safety</th>
                <th>Emotional</th>
                <th>Behaviour</th>
                <th>Signal</th>
              </tr>
            </thead>
            <tbody>
              {children.map((c) => {
                const idx = indices[c.id];
                const signal = signals[c.id];

                const risk = getDomainIndex(c.id, "risk") || getDomainIndex(c.id, "safety");
                const emotional = getDomainIndex(c.id, "emotional");
                const behaviour = getDomainIndex(c.id, "behaviour");

                const overall = idx?.overall;

                return (
                  <tr key={c.id}>
                    <td>
                      <div className="child-name">{renderChildName(c)}</div>
                      <div className="child-meta">ID {c.id}</div>
                    </td>
                    <td>
                      {overall ? overall.index.toFixed(1) : "—"}
                      {overall && overall.change != null && (
                        <span className="field-hint" style={{ marginLeft: 4 }}>
                          ({overall.change >= 0 ? "+" : ""}
                          {overall.change.toFixed(1)})
                        </span>
                      )}
                    </td>
                    <td>
                      {risk ? risk.index.toFixed(1) : "—"}
                      {risk && risk.change != null && (
                        <span className="field-hint" style={{ marginLeft: 4 }}>
                          ({risk.change >= 0 ? "+" : ""}
                          {risk.change.toFixed(1)})
                        </span>
                      )}
                    </td>
                    <td>
                      {emotional ? emotional.index.toFixed(1) : "—"}
                      {emotional && emotional.change != null && (
                        <span className="field-hint" style={{ marginLeft: 4 }}>
                          ({emotional.change >= 0 ? "+" : ""}
                          {emotional.change.toFixed(1)})
                        </span>
                      )}
                    </td>
                    <td>
                      {behaviour ? behaviour.index.toFixed(1) : "—"}
                      {behaviour && behaviour.change != null && (
                        <span className="field-hint" style={{ marginLeft: 4 }}>
                          ({behaviour.change >= 0 ? "+" : ""}
                          {behaviour.change.toFixed(1)})
                        </span>
                      )}
                    </td>
                    <td>
                      {signal ? (
                        <span
                          className={signalClass(signal.level)}
                          title={signal.message}
                        >
                          {getSignalLabel(signal.level)}
                        </span>
                      ) : (
                        <span className="field-hint">No alerts</span>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
};

export default PsychDashboardTab;
