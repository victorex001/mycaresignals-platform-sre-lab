// src/ChildrenTab.tsx
import React, { useEffect, useState } from "react";
import type { Child } from "./types";
import { apiFetch } from "./api";

type Props = {
  apiBase: string;
  homeId: number;
  homeName: string;
  providerName: string;
  selectedChild: Child | null;
  onOpenChildDashboard: (child: Child) => void;
  onCreateChild: () => void;
  // 🆕 separate callback for continuing a draft baseline
  onContinueBaseline: (child: Child) => void;
};

// What the domain index endpoint returns (we only use a few fields)
type DomainIndexResponse = {
  child_id: number;
  window_days: number;
  overall_index: number | null;
  risk_index: number | null;
  emotional_index: number | null;
  behaviour_index: number | null;
  change_overall?: number | null;
  change_risk?: number | null;
  change_emotional?: number | null;
  change_behaviour?: number | null;
};

type AlertLevel = "none" | "amber" | "red";

type ChildAlert = {
  level: AlertLevel;
  message: string;
};

const defaultAlert: ChildAlert = {
  level: "none",
  message: "No alerts",
};

// 🔹 Helper to normalise baseline status coming from API
type BaselineStatus = "none" | "draft" | "submitted" | "approved";

const getBaselineStatus = (c: Child): BaselineStatus => {
  const raw = (c as any).baseline_status as string | undefined | null;
  if (!raw) return "none";
  if (raw === "draft" || raw === "submitted" || raw === "approved") return raw;
  return "none";
};

const ChildrenTab: React.FC<Props> = ({
  apiBase,
  homeId,
  homeName,
  providerName,
  onOpenChildDashboard,
  onCreateChild,
  onContinueBaseline,
}) => {
  const [children, setChildren] = useState<Child[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // early-warning alerts per child ID
  const [alertsByChildId, setAlertsByChildId] = useState<
    Record<number, ChildAlert>
  >({});

  const renderChildLabel = (c: Child) => {
    const name =
      c.first_name || c.last_name
        ? `${c.first_name ?? ""} ${c.last_name ?? ""}`.trim()
        : `Child ${c.id}`;
    const initials = c.initials || "";
    return `${name} ${initials ? `(${initials})` : ""}`.trim();
  };

  const deriveAlert = (idx?: DomainIndexResponse | null): ChildAlert => {
    if (!idx || idx.overall_index == null) {
      return defaultAlert;
    }

    const risk = idx.risk_index ?? 0;
    const emo = idx.emotional_index ?? 0;
    const beh = idx.behaviour_index ?? 0;
    const dRisk = idx.change_risk ?? 0;
    const dEmo = idx.change_emotional ?? 0;
    const dBeh = idx.change_behaviour ?? 0;

    // 🔴 Red – clear safeguarding concern
    if (risk >= 3.5 || dRisk >= 2) {
      if (dRisk >= 2) {
        return {
          level: "red",
          message: "Risk has risen sharply in the last 6 weeks",
        };
      }
      return {
        level: "red",
        message: "Risk levels are high – review safety plan",
      };
    }

    // 🟠 Amber – emotional / behaviour deterioration
    if (emo >= 3 || beh >= 3 || dEmo >= 1.5 || dBeh >= 1.5) {
      return {
        level: "amber",
        message: "Emotional / behaviour scores worsening",
      };
    }

    // 🟢 No specific signal
    return defaultAlert;
  };

  const loadAlertsFor = async (list: Child[]) => {
    if (list.length === 0) {
      setAlertsByChildId({});
      return;
    }

    const entries: [number, ChildAlert][] = [];

    await Promise.all(
      list.map(async (c) => {
        try {
          const res = await apiFetch(
            `${apiBase}/v1/children/${c.id}/domain_index?window_days=42`,
          );
          if (!res.ok) {
            entries.push([c.id, defaultAlert]);
            return;
          }
          const data: DomainIndexResponse = await res.json();
          entries.push([c.id, deriveAlert(data)]);
        } catch {
          entries.push([c.id, defaultAlert]);
        }
      }),
    );

    const next: Record<number, ChildAlert> = {};
    for (const [id, alert] of entries) {
      next[id] = alert;
    }
    setAlertsByChildId(next);
  };

  useEffect(() => {
    const loadChildren = async () => {
      setLoading(true);
      setError(null);
      try {
        const res = await apiFetch(`${apiBase}/v1/homes/${homeId}/children`);
        if (!res.ok) {
          const txt = await res.text();
          throw new Error(txt || "Failed to load children");
        }

        const raw = await res.json();

        // 🔒 SAFETY: whatever comes back, we coerce it to an array
        const data: Child[] = Array.isArray(raw) ? raw : [];

        setChildren(data);
        await loadAlertsFor(data);
      } catch (err: any) {
        setError(err.message || "Could not load children");
        setChildren([]);
        setAlertsByChildId({});
      } finally {
        setLoading(false);
      }
    };

    loadChildren();
  }, [apiBase, homeId]);

  return (
    <section className="section-band">
      <div className="card card-emphasis section-band-inner">
        <div className="card-header">
          <div>
            <h2>Children at this home</h2>
            <p className="card-subtitle">
              {homeName} · Provider: {providerName}. Pick a child to open their
              therapeutic workspace.
            </p>
          </div>
          <button
            type="button"
            className="btn btn-primary btn-xs"
            onClick={onCreateChild}
          >
            + Add young person
          </button>
        </div>

        {error && (
          <div className="alert alert-error">
            <strong>Error:</strong> {error}
          </div>
        )}

        {loading && <p className="field-hint">Loading children…</p>}

        {!loading && children.length === 0 && !error && (
          <p className="field-hint">
            No young people have been added for this home yet. Use “Add young
            person” to begin.
          </p>
        )}

        {!loading && children.length > 0 && (
          <ul className="child-list">
            {children.map((c) => {
              const alert = alertsByChildId[c.id] ?? defaultAlert;
              const baselineStatus = getBaselineStatus(c);

              let baselineLabel = "No baseline yet";
              let baselineBg = "#e5e7eb";
              let baselineColor = "#4b5563";

              if (baselineStatus === "draft") {
                baselineLabel = "Baseline: draft";
                baselineBg = "#fef3c7";
                baselineColor = "#92400e";
              } else if (baselineStatus === "submitted") {
                baselineLabel = "Baseline: submitted";
                baselineBg = "#dbeafe";
                baselineColor = "#1d4ed8";
              } else if (baselineStatus === "approved") {
                baselineLabel = "Baseline: approved";
                baselineBg = "#dcfce7";
                baselineColor = "#166534";
              }

              const showContinueBaseline = baselineStatus === "draft";

              return (
                <li key={c.id} className="child-list-item"
                style={{ marginBottom: "2.5rem" }}>
                  <div>
                    {/* Small rounded button for the child – open workspace */}
                    <button
                      type="button"
                      className="child-pill"
                      onClick={() => onOpenChildDashboard(c)}
                    >
                      <span className="child-pill-title">
                        {renderChildLabel(c)}
                      </span>
                    </button>

                    {/* Info row: ID, baseline status, alerts, actions */}
                    <div
                      style={{
                        marginTop: "0.25rem",
                        display: "flex",
                        flexWrap: "wrap",
                        gap: "0.35rem",
                        fontSize: "0.78rem",
                        alignItems: "center",
                      }}
                    >
                      <span
                        style={{
                          padding: "0.15rem 0.55rem",
                          borderRadius: 999,
                          background: "#e5e7eb",
                          color: "#6b7280",
                          fontWeight: 500,
                        }}
                      >
                        ID: {c.id}
                      </span>

                      {/* Baseline status pill */}
                      <span
                        style={{
                          padding: "0.15rem 0.55rem",
                          borderRadius: 999,
                          background: baselineBg,
                          color: baselineColor,
                          fontWeight: 500,
                        }}
                      >
                        {baselineLabel}
                      </span>

                      {/* 🆕 Continue baseline button (only if draft) */}
                      {showContinueBaseline && (
                        <button
                          type="button"
                          className="btn btn-secondary btn-xxs"
                          onClick={() => onContinueBaseline(c)}
                        >
                          View legacy baseline draft
                        </button>
                      )}

                      {/* Alert pill */}
                      <span
                        className={`child-alert-pill child-alert-${alert.level}`}
                      >
                        {alert.level === "red"
                          ? `🚨 ${alert.message}`
                          : alert.level === "amber"
                          ? `⚠️ ${alert.message}`
                          : `🟢 ${alert.message}`}
                      </span>
                    </div>
                  </div>
                </li>
              );
            })}
          </ul>
        )}
      </div>
    </section>
  );
};

export default ChildrenTab;
