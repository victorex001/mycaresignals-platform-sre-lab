import React, { useEffect, useState } from "react";
import type { Child } from "./types";
import { apiFetch } from "./api";

type Baseline = {
  id: number;
  child_id: number;
  summary_of_history: string;
  recent_placement: string;
  current_concerns: string;
  strengths: string;
  existing_plans: string;
  childs_voice: string;
  relapse_risks: string;
  relapse_early_signals: string;
  relapse_support_plan: string;
  review_frequency: string;
  next_review_date: string | null;
  lead_reviewer: string;
  other_professionals: string;
  early_warning_signals: string;
  notes_for_team: string;
  status: "draft" | "submitted" | "approved";
  approved_by: string | null;
  approved_at: string | null;
  created_at: string;
};

type LegacyDomain = { domain_key: string; active: boolean; reason: string };

const legacyDomainLabels: Record<string, string> = {
  emotionalWellbeing: "Emotional wellbeing",
  behaviour: "Behaviour",
  relationships: "Relationships",
  riskAndSafety: "Risk & safety",
  dailyLiving: "Daily living / independence",
  education: "Education & engagement",
  physicalHealth: "Physical health",
  identityCulture: "Identity & culture",
};

type Props = {
  apiBase: string;
  child: Child;
  onBack: () => void;
};

const BaselineReportView: React.FC<Props> = ({ apiBase, child, onBack }) => {
  const [baseline, setBaseline] = useState<Baseline | null>(null);
  const [domains, setDomains] = useState<LegacyDomain[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [approveName, setApproveName] = useState("");
  const [approving, setApproving] = useState(false);

  useEffect(() => {
    const load = async () => {
      setLoading(true);
      setError(null);
      try {
        const res = await apiFetch(
          `${apiBase}/v1/children/${child.id}/onboarding/baseline`,
        );
        if (!res.ok) {
          const txt = await res.text();
          throw new Error(txt || "Could not load baseline");
        }
        const data = await res.json();
        setBaseline(data.baseline);
        setDomains(Array.isArray(data.domains) ? data.domains : []);
      } catch (err: any) {
        setError(err.message || "Error loading baseline");
      } finally {
        setLoading(false);
      }
    };

    load();
  }, [apiBase, child.id]);

  const handleApprove = async () => {
    if (!approveName.trim()) {
      alert("Please enter your name before approving.");
      return;
    }
    setApproving(true);
    try {
      const res = await apiFetch(
        `${apiBase}/v1/children/${child.id}/onboarding/approve`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ approved_by: approveName.trim() }),
        },
      );
      if (!res.ok) {
        const txt = await res.text();
        throw new Error(txt || "Approve failed");
      }
      // re-load baseline to get status + approved_at
      const refreshed = await apiFetch(
        `${apiBase}/v1/children/${child.id}/onboarding/baseline`,
      );
      const refreshedData = await refreshed.json();
      setBaseline(refreshedData.baseline);
      setDomains(Array.isArray(refreshedData.domains) ? refreshedData.domains : []);
    } catch (err: any) {
      alert(err.message || "Error approving baseline");
    } finally {
      setApproving(false);
    }
  };

  const handlePrint = () => {
    window.print();
  };

  return (
    <section className="card">
      <div className="card-header">
        <div>
          <h2>Historical baseline report – {child.initials}</h2>
          <p className="card-subtitle">
            Preserved eight-domain baseline. It has not been rewritten or
            automatically mapped to the modern framework.
          </p>
        </div>
        <button type="button" className="btn btn-secondary btn-xs" onClick={onBack}>
          ← Back to dashboard
        </button>
      </div>

      {loading && <p className="field-hint">Loading baseline…</p>}
      {error && (
        <div className="alert alert-error">
          <strong>Error:</strong> {error}
        </div>
      )}

      {!loading && baseline && (
        <div className="baseline-report">
          {/* Status banner */}
          <div className="alert">
            {baseline.status === "approved" ? (
              <span>
                ✅ Baseline approved by <strong>{baseline.approved_by}</strong>{" "}
                on{" "}
                {baseline.approved_at
                  ? new Date(baseline.approved_at).toLocaleDateString()
                  : ""}
                .
              </span>
            ) : (
              <span>
                ⏳ Baseline status: <strong>{baseline.status}</strong>. Not yet
                approved.
              </span>
            )}
          </div>

          {/* Manager approval controls */}
          {baseline.status !== "approved" && (
            <div className="card" style={{ marginBottom: "1rem" }}>
              <div className="card-header">
                <h3>Manager approval</h3>
              </div>
              <div className="form-row-2">
                <div className="form-group">
                  <label className="label">Manager name</label>
                  <input
                    className="input"
                    value={approveName}
                    onChange={(e) => setApproveName(e.target.value)}
                    placeholder="Name to display on the report"
                  />
                </div>
                <div className="form-group" style={{ display: "flex", alignItems: "flex-end" }}>
                  <button
                    type="button"
                    className="btn btn-primary"
                    onClick={handleApprove}
                    disabled={approving}
                  >
                    {approving ? "Approving…" : "Approve baseline"}
                  </button>
                </div>
              </div>
            </div>
          )}

          {/* Print button */}
          <div style={{ marginBottom: "1rem" }}>
            <button type="button" className="btn btn-secondary" onClick={handlePrint}>
              Print / Export to PDF
            </button>
          </div>

          {/* Very simple layout – you can style nicer later */}
          <h3>Background & concerns</h3>
          <p><strong>Background summary:</strong> {baseline.summary_of_history}</p>
          <p><strong>Recent placement pattern:</strong> {baseline.recent_placement}</p>
          <p><strong>Main concerns / needs on arrival:</strong> {baseline.current_concerns}</p>
          <p><strong>Strengths & protective factors:</strong> {baseline.strengths}</p>
          <p><strong>Existing plans / services:</strong> {baseline.existing_plans}</p>
          <p><strong>Child&apos;s voice:</strong> {baseline.childs_voice}</p>

          <h3>Relapse / regression</h3>
          <p><strong>Relapse risks:</strong> {baseline.relapse_risks}</p>
          <p><strong>Early warning signs:</strong> {baseline.relapse_early_signals}</p>
          <p><strong>Support plan if warning signs appear:</strong> {baseline.relapse_support_plan}</p>

          <h3>Historical eight-domain profile</h3>
          {domains.length === 0 ? <p>No historical domain detail was recorded.</p> : domains.map((domain) => (
            <div key={domain.domain_key} className="priority-block">
              <strong>{legacyDomainLabels[domain.domain_key] ?? domain.domain_key}</strong>
              <p>{domain.active ? "Active at the time" : "Not active at the time"}</p>
              <p>{domain.reason || "No reason recorded."}</p>
            </div>
          ))}

          <h3>Review & handover</h3>
          <p><strong>Review frequency:</strong> {baseline.review_frequency}</p>
          <p><strong>Next review date:</strong> {baseline.next_review_date}</p>
          <p><strong>Lead reviewer:</strong> {baseline.lead_reviewer}</p>
          <p><strong>Others to involve:</strong> {baseline.other_professionals}</p>
          <p><strong>System-level early warning signals:</strong> {baseline.early_warning_signals}</p>
          <p><strong>Notes for the team / handover:</strong> {baseline.notes_for_team}</p>
        </div>
      )}
    </section>
  );
};

export default BaselineReportView;
