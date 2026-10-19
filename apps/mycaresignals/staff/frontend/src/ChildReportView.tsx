// src/ChildReportView.tsx
import React, { useEffect, useState } from "react";
import type { Child } from "./types";
import { apiFetch } from "./api";

// 👇 adjust this path if your logo lives somewhere else
import myCareSignalsLogo from "./assets/mycaresignals-logo.png";

type Props = {
  apiBase: string;
  homeName: string;
  providerName: string;
  child: Child;
  onBack: () => void;
};

// --- Types mirroring backend ---

type Baseline = {
  summary_of_history?: string;
  recent_placement_pattern?: string;
  current_concerns?: string;
  strengths?: string;
  existing_plans?: string;
  childs_voice?: string;
  relapse_risks?: string;
  relapse_early_signals?: string;
  relapse_support_plan?: string;
  review_frequency?: string;
  next_review_date?: string | null;
  lead_reviewer?: string;
  other_professionals?: string;
  early_warning_signals?: string;
  notes_for_team?: string;
  status?: string;
};

type BaselinePriority = {
  domain_key: string;
  focus_summary: string;
  hoped_for_change: string;
  sort_order: number;
};

type BaselineApiResponse = {
  baseline?: Baseline;
  priorities?: BaselinePriority[];
};

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
  domains: DomainIndex[] | null;
};

type SignalResponse = {
  id: number;
  child_id: number;
  signal_kind: string;
  did_checkin: boolean;
  adjusted_support: boolean;
  informed_manager: boolean;
  informed_network: boolean;
  monitor_only: boolean;
  did_referral: boolean;
  referral_destinations: string;
  actions_summary: string;
  future_notes: string;
  external_log_ref: string;
  created_at: string;
};

type VoicePoint = {
  date: string;
  domains: Record<string, number>;
};

type VoiceTimelineResponse = {
  home_name: string;
  alias?: string;
  points: VoicePoint[] | null;
};

const DOMAIN_LABELS: Record<string, string> = {
  relationships: "Family / relationships",
  health: "Health / environment",
  safety: "Safety",
  education: "Education",
  identity: "Identity / voice",
  coping: "Coping",
  risk: "Risk & safety",
  behaviour: "Behaviour",
  emotional: "Emotional regulation",
};

const ChildReportView: React.FC<Props> = ({
  apiBase,
  homeName,
  providerName,
  child,
  onBack,
}) => {
  const [baseline, setBaseline] = useState<Baseline | null>(null);
  const [priorities, setPriorities] = useState<BaselinePriority[]>([]);
  const [indexData, setIndexData] = useState<ChildDomainIndex | null>(null);
  const [signals, setSignals] = useState<SignalResponse[]>([]);
  const [voice, setVoice] = useState<VoiceTimelineResponse | null>(null);

  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);

  // local-only notes (not saved, just for printing)
  const [localNotes, setLocalNotes] = useState("");

  const initials =
    child.initials ||
    `${child.first_name ?? ""} ${child.last_name ?? ""}`.trim() ||
    `Child ${child.id}`;

  const reportDate = new Date().toLocaleDateString("en-GB");

  const formattedDOB =
    child.date_of_birth
      ? new Date(child.date_of_birth).toLocaleDateString("en-GB")
      : "—";

  const formattedSince =
    child.admission_date
      ? new Date(child.admission_date).toLocaleDateString("en-GB")
      : "—";

  useEffect(() => {
    const loadAll = async () => {
      setLoading(true);
      setLoadError(null);

      try {
        // 1) Baseline + priorities
        const baselineRes = await apiFetch(
          `${apiBase}/v1/children/${child.id}/onboarding/baseline`,
        );
        if (baselineRes.ok) {
          const json: BaselineApiResponse = await baselineRes.json();
          if (json.baseline) {
            setBaseline(json.baseline);
          }
          if (Array.isArray(json.priorities)) {
            setPriorities(
              json.priorities.slice().sort((a, b) => a.sort_order - b.sort_order),
            );
          }
        }

        // 2) Pathway Index
        const indexRes = await apiFetch(
          `${apiBase}/v1/children/${child.id}/domain_index?window_days=42`,
        );
        if (indexRes.ok) {
          const idxJson: ChildDomainIndex = await indexRes.json();
          // normalise domains to an array
          setIndexData({
            ...idxJson,
            domains: Array.isArray(idxJson.domains) ? idxJson.domains : [],
          });
        }

        // 3) Signal responses (what the team did)
        const sigRes = await apiFetch(
          `${apiBase}/v1/children/${child.id}/signal_responses?limit=10`,
        );
        if (sigRes.ok) {
          const sigJson = await sigRes.json();
          setSignals(Array.isArray(sigJson) ? sigJson : []);
        }

        // 4) Child-voice timeline
        const voiceRes = await apiFetch(
          `${apiBase}/v1/children/${child.id}/checkins`,
        );
        if (voiceRes.ok) {
          const voiceJson: VoiceTimelineResponse = await voiceRes.json();
          setVoice({
            ...voiceJson,
            points: Array.isArray(voiceJson.points) ? voiceJson.points : [],
          });
        }
      } catch (err: any) {
        setLoadError(
          err?.message || "Could not load information for this report",
        );
      } finally {
        setLoading(false);
      }
    };

    loadAll();
  }, [
    apiBase,
    child.id,
    child.initials,
    child.first_name,
    child.last_name,
    homeName,
  ]);

  const handlePrint = () => {
    window.print();
  };

  const renderDomainSummary = () => {
    const domains = indexData?.domains ?? [];

    if (!domains || domains.length === 0) {
      return (
        <p className="field-hint">
          No Pathway Index data yet – once key-working scores are recorded, a
          wellbeing summary will appear here.
        </p>
      );
    }

    const topDomains = domains
      .slice()
      .sort((a, b) => b.index - a.index)
      .slice(0, 4);

    return (
      <ul className="doc-list">
        {topDomains.map((d) => (
          <li key={d.domain} className="doc-list-item">
            <strong>{DOMAIN_LABELS[d.domain] || d.domain}</strong>{" "}
            <span className="field-hint">
              Index {d.index.toFixed(1)}
              {typeof d.change === "number" && (
                <>
                  {" "}
                  · change vs previous: {d.change >= 0 ? "+" : ""}
                  {d.change.toFixed(1)}
                </>
              )}
            </span>
          </li>
        ))}
      </ul>
    );
  };

  const renderVoiceSummary = () => {
    const points = voice?.points ?? [];

    if (!points || points.length === 0) {
      return (
        <p className="field-hint">
          No child-voice check-ins recorded yet in this period.
        </p>
      );
    }

    const totals: Record<string, { sum: number; count: number }> = {};

    points.forEach((p) => {
      Object.entries(p.domains || {}).forEach(([key, val]) => {
        if (typeof val !== "number") return;
        if (!totals[key]) {
          totals[key] = { sum: 0, count: 0 };
        }
        totals[key].sum += val;
        totals[key].count += 1;
      });
    });

    const averages = Object.entries(totals).map(([key, v]) => ({
      key,
      avg: v.sum / Math.max(1, v.count),
    }));

    // lower scores = areas where child is struggling more
    const lowest = averages
      .slice()
      .sort((a, b) => a.avg - b.avg)
      .slice(0, 3);

    if (!lowest.length) {
      return (
        <p className="field-hint">
          Child-voice scores are currently balanced – no clear low-scoring
          domains.
        </p>
      );
    }

    return (
      <ul className="doc-list">
        {lowest.map((d) => (
          <li key={d.key} className="doc-list-item">
            <strong>{DOMAIN_LABELS[d.key] || d.key}</strong>{" "}
            <span className="field-hint">
              Average score {d.avg.toFixed(1)} (lower scores = more difficulty)
            </span>
          </li>
        ))}
      </ul>
    );
  };

  // normalised arrays for safe rendering
  const priorityList = Array.isArray(priorities) ? priorities : [];
  const signalList = Array.isArray(signals) ? signals : [];

  return (
    <section className="card report-root">
      {/* Header with logo + meta + toolbar */}
      <div className="card-header report-header">
        <div className="report-header-left">
          <div className="report-logo-box">
            <img
              src={myCareSignalsLogo}
              alt="myCareSignals logo"
              className="report-logo"
            />
          </div>

          <div className="report-header-main">
            <h2>MYCARESIGNALS REPORT</h2>
            <p className="card-subtitle">
              Wellbeing and progress summary based on the child&apos;s baseline,
              key-working and care signals.
            </p>

            <div className="summary report-summary">
              <p>
                <span className="summary-label">Child</span> {initials}
              </p>
              <p>
                <span className="summary-label">DOB</span> {formattedDOB}
              </p>
              <p>
                <span className="summary-label">At this home since</span>{" "}
                {formattedSince}
              </p>
              <p>
                <span className="summary-label">Home</span> {homeName}
              </p>
              <p>
                <span className="summary-label">Provider</span> {providerName}
              </p>
              <p>
                <span className="summary-label">Date of report</span>{" "}
                {reportDate}
              </p>
            </div>
          </div>
        </div>

        <div className="report-header-actions no-print">
          <button
            type="button"
            className="btn btn-secondary btn-xs"
            onClick={onBack}
          >
            ← Back to child dashboard
          </button>
          <button
            type="button"
            className="btn btn-primary btn-xs"
            onClick={handlePrint}
          >
            Print / Save as PDF
          </button>
        </div>
      </div>

      {loading && (
        <p className="field-hint">Gathering information for the report…</p>
      )}

      {loadError && (
        <div className="alert alert-error">
          <strong>Error:</strong> {loadError}
        </div>
      )}

      {!loading && !loadError && (
        <div className="report-body">
          {/* SECTION 1 – Snapshot & context */}
          <div className="card card-sub report-section">
            <h3>1. Snapshot &amp; context</h3>
            <p className="field-hint">
              Brief summary of the child&apos;s history and what the team is
              currently working on.
            </p>

            {child.legal_status && (
              <div className="summary report-summary">
                <p>
                  <span className="summary-label">Legal status</span>{" "}
                  {child.legal_status}
                </p>
              </div>
            )}

            {baseline ? (
              <>
                {baseline.summary_of_history && (
                  <div className="form-group">
                    <h4 className="label">Summary of history</h4>
                    <p>{baseline.summary_of_history}</p>
                  </div>
                )}

                {baseline.current_concerns && (
                  <div className="form-group">
                    <h4 className="label">
                      Current concerns / reasons for placement
                    </h4>
                    <p>{baseline.current_concerns}</p>
                  </div>
                )}

                {baseline.strengths && (
                  <div className="form-group">
                    <h4 className="label">Key strengths</h4>
                    <p>{baseline.strengths}</p>
                  </div>
                )}

                {baseline.existing_plans && (
                  <div className="form-group">
                    <h4 className="label">
                      Existing plans / professionals involved
                    </h4>
                    <p>{baseline.existing_plans}</p>
                  </div>
                )}

                {priorityList.length > 0 && (
                  <div className="form-group">
                    <h4 className="label">Priority domains (staff plan)</h4>
                    <ul className="doc-list">
                      {priorityList.map((p) => (
                        <li key={p.domain_key} className="doc-list-item">
                          <strong>
                            {DOMAIN_LABELS[p.domain_key] || p.domain_key}
                          </strong>
                          {p.focus_summary && (
                            <div className="field-hint">
                              Focus: {p.focus_summary}
                            </div>
                          )}
                          {p.hoped_for_change && (
                            <div className="field-hint">
                              Hoped-for change: {p.hoped_for_change}
                            </div>
                          )}
                        </li>
                      ))}
                    </ul>
                  </div>
                )}
              </>
            ) : (
              <p className="field-hint">
                No baseline captured yet. Once the baseline is completed, key
                context will be summarised here.
              </p>
            )}
          </div>

          {/* SECTION 2 – Risk, relapse planning & signal responses */}
          <div className="card card-sub report-section">
            <h3>2. Risk, relapse planning &amp; responses</h3>
            <p className="field-hint">
              How the team are thinking about risk/relapse and what has been
              done in response to recent changes or concerns.
            </p>

            {baseline && (
              <>
                {baseline.relapse_risks && (
                  <div className="form-group">
                    <h4 className="label">Known relapse risks</h4>
                    <p>{baseline.relapse_risks}</p>
                  </div>
                )}
                {baseline.relapse_early_signals && (
                  <div className="form-group">
                    <h4 className="label">Early warning signs</h4>
                    <p>{baseline.relapse_early_signals}</p>
                  </div>
                )}
                {baseline.relapse_support_plan && (
                  <div className="form-group">
                    <h4 className="label">
                      Agreed relapse / safety support plan
                    </h4>
                    <p>{baseline.relapse_support_plan}</p>
                  </div>
                )}
              </>
            )}

            <div className="form-group">
              <h4 className="label">
                Recent responses to early-warning signals
              </h4>
              {signalList.length === 0 ? (
                <p className="field-hint">
                  No signal responses logged yet for this child.
                </p>
              ) : (
                <ul className="doc-list">
                  {signalList.map((r) => (
                    <li key={r.id} className="doc-list-item">
                      <div className="report-signal-meta">
                        {new Date(r.created_at).toLocaleString()} ·{" "}
                        {r.signal_kind === "risk"
                          ? "Risk & safety"
                          : r.signal_kind === "emotional_behaviour"
                          ? "Emotional & behaviour"
                          : "Overall wellbeing"}
                      </div>
                      <div style={{ fontSize: "0.9rem" }}>
                        {r.actions_summary || "No summary recorded."}
                      </div>
                      {r.future_notes && (
                        <div className="field-hint">
                          Next watch points: {r.future_notes}
                        </div>
                      )}
                      {r.did_referral && (
                        <div className="field-hint">
                          Referrals:{" "}
                          {r.referral_destinations || "Referral recorded"}
                        </div>
                      )}
                      {r.external_log_ref && (
                        <div className="field-hint">
                          Full record in: {r.external_log_ref}
                        </div>
                      )}
                    </li>
                  ))}
                </ul>
              )}
            </div>
          </div>

          {/* SECTION 3 – Wellbeing & child voice over time */}
          <div className="card card-sub report-section">
            <h3>3. Wellbeing &amp; child voice over time</h3>
            <p className="field-hint">
              Brings together the Pathway Index and the child&apos;s own scores
              from the feedback form.
            </p>

            <div className="form-group">
              <h4 className="label">Pathway Index snapshot</h4>
              {indexData ? (
                <>
                  <p className="field-hint">
                    Overall index:{" "}
                    <strong>{indexData.overall.index.toFixed(1)}</strong>
                    {indexData.overall.change != null && (
                      <>
                        {" "}
                        · change vs previous:{" "}
                        <strong>
                          {indexData.overall.change >= 0 ? "+" : ""}
                          {indexData.overall.change.toFixed(1)}
                        </strong>
                      </>
                    )}{" "}
                    (window: last {indexData.window_days} days)
                  </p>
                  {renderDomainSummary()}
                </>
              ) : (
                <p className="field-hint">
                  No Pathway Index data yet for this period.
                </p>
              )}
            </div>

            <div className="form-group">
              <h4 className="label">What the child is telling us</h4>
              {renderVoiceSummary()}
            </div>

            {baseline && baseline.childs_voice && (
              <div className="form-group">
                <h4 className="label">Child&apos;s voice (from baseline)</h4>
                <p>{baseline.childs_voice}</p>
              </div>
            )}
          </div>

          {/* SECTION 4 – Reflections & next steps */}
          <div className="card card-sub report-section">
            <h3>4. Reflections &amp; next steps</h3>
            <p className="field-hint">
              Space for the team to summarise what is going well, what they are
              worried about, and agreed next steps. These notes are not stored
              in the system – they will only appear on the printed report.
            </p>

            {baseline && baseline.notes_for_team && (
              <div className="form-group">
                <h4 className="label">Notes for the team (baseline)</h4>
                <p>{baseline.notes_for_team}</p>
              </div>
            )}

            <div className="form-group">
              <h4 className="label">Local notes for this report (not saved)</h4>
              <textarea
                className="input"
                rows={5}
                value={localNotes}
                onChange={(e) => setLocalNotes(e.target.value)}
                placeholder="Summarise progress, worries and next steps here before printing. This text is not stored in the database."
              />
            </div>
          </div>
        </div>
      )}
    </section>
  );
};

export default ChildReportView;
