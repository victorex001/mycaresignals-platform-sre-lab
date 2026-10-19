// src/TherapeuticSummaryTab.tsx
import React, { useEffect, useState } from "react";
import type {
  ChildDomainIndex,
  PsychologicalReview,
} from "./types";
import { apiFetch } from "./api";

type Props = {
  apiBase: string;
  childId: number;
  childInitials: string;
};

type ReviewFormState = {
  window_days: number;
  review_from: string;
  review_to: string;
  formulation: string;
  protective_factors: string;
  recommendations: string;
};

const TherapeuticSummaryTab: React.FC<Props> = ({
  apiBase,
  childId,
  childInitials,
}) => {
  const [domainIndex, setDomainIndex] = useState<ChildDomainIndex | null>(null);
  const [reviews, setReviews] = useState<PsychologicalReview[]>([]);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // default window: 42 days (6 weeks)
  const [windowDays, setWindowDays] = useState(42);

  const [form, setForm] = useState<ReviewFormState>(() => {
    const today = new Date();
    const past = new Date();
    past.setDate(today.getDate() - 41);

    const fmt = (d: Date) => d.toISOString().slice(0, 10);

    return {
      window_days: 42,
      review_from: fmt(past),
      review_to: fmt(today),
      formulation: "",
      protective_factors: "",
      recommendations: "",
    };
  });

  const formatIndex = (n: number | null | undefined) =>
    n === null || n === undefined ? "—" : n.toFixed(1);

  const loadData = async () => {
    setLoading(true);
    setError(null);
    try {
      // 1) Domain index for windowDays
      const idxRes = await apiFetch(
        `${apiBase}/v1/children/${childId}/domain_index?window_days=${windowDays}`
      );
      if (!idxRes.ok) {
        throw new Error("Failed to load domain summary");
      }
      const idxData: ChildDomainIndex = await idxRes.json();
      setDomainIndex(idxData);

      // 2) Existing psych reviews
      const revRes = await apiFetch(
        `${apiBase}/v1/children/${childId}/reviews`
      );
      if (!revRes.ok) {
        throw new Error("Failed to load reviews");
      }
      const revData: PsychologicalReview[] = await revRes.json();
      setReviews(Array.isArray(revData) ? revData : []);
    } catch (err: any) {
      setError(err.message || "Could not load therapeutic summary");
      setDomainIndex(null);
      setReviews([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [childId, windowDays]);

  const handleChange = (patch: Partial<ReviewFormState>) => {
    setForm((prev) => ({ ...prev, ...patch }));
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setSaving(true);
    try {
      const body = {
        window_days: form.window_days || windowDays,
        review_from: form.review_from,
        review_to: form.review_to,
        formulation: form.formulation,
        protective_factors: form.protective_factors,
        recommendations: form.recommendations,
      };

      const res = await apiFetch(
        `${apiBase}/v1/children/${childId}/reviews`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(body),
        }
      );
      if (!res.ok) {
        const txt = await res.text();
        throw new Error(txt || "Failed to save review");
      }

      // reset text fields but keep dates/window
      setForm((prev) => ({
        ...prev,
        formulation: "",
        protective_factors: "",
        recommendations: "",
      }));

      await loadData();
    } catch (err: any) {
      setError(err.message || "Could not save review");
    } finally {
      setSaving(false);
    }
  };

  // const latestReview = reviews[0];

  const handlePrint = () => {
    window.print();
  };

  return (
    <section className="card">
      <div className="card-header">
        <div>
          <h2>Psychological review &amp; summary</h2>
          <p className="card-subtitle">
            Use scores and domains to write a short formulation and update the
            plan for <strong>{childInitials}</strong>. This replaces separate
            BERRI summaries.
          </p>
        </div>
        <button
          type="button"
          className="btn btn-secondary btn-xs"
          onClick={handlePrint}
        >
          Print / export
        </button>
      </div>

      {error && (
        <div className="alert alert-error">
          <strong>Error:</strong> {error}
        </div>
      )}

      {loading && <p className="field-hint">Loading domain summary…</p>}

      {/* Domain summary */}
      {domainIndex && (
        <div className="card card-sub">
          <div className="chart-header">
            <h3>Domain summary (last {domainIndex.window_days} days)</h3>
            <select
              className="input"
              style={{ maxWidth: "160px" }}
              value={windowDays}
              onChange={(e) => {
                const val = Number(e.target.value || 28);
                setWindowDays(val);
                handleChange({ window_days: val });
              }}
            >
              <option value={28}>Last 4 weeks</option>
              <option value={42}>Last 6 weeks</option>
              <option value={84}>Last 12 weeks</option>
            </select>
          </div>

          <p className="field-hint">
            Overall Pathway Index:{" "}
            <strong>{formatIndex(domainIndex.overall.index)}</strong>
            {domainIndex.overall.change !== undefined &&
              domainIndex.overall.change !== null && (
                <>
                  {" "}
                  (change{" "}
                  {domainIndex.overall.change >= 0 ? "+" : ""}
                  {formatIndex(domainIndex.overall.change)})
                </>
              )}
          </p>

          {domainIndex.domains.length === 0 && (
            <p className="field-hint">
              No domain scores yet — log more key-working sessions to see this.
            </p>
          )}

          {domainIndex.domains.length > 0 && (
            <table className="simple-table">
              <thead>
                <tr>
                  <th>Domain</th>
                  <th>Avg wellbeing</th>
                  <th>Avg progress</th>
                  <th>Index</th>
                  <th>Change vs prev window</th>
                </tr>
              </thead>
              <tbody>
                {domainIndex.domains.map((d) => (
                  <tr key={d.domain}>
                    <td>{d.domain}</td>
                    <td>{formatIndex(d.average_wellbeing)}</td>
                    <td>{formatIndex(d.average_progress)}</td>
                    <td>{formatIndex(d.index)}</td>
                    <td>
                      {d.change === undefined || d.change === null
                        ? "—"
                        : `${d.change >= 0 ? "+" : ""}${formatIndex(
                            d.change
                          )}`}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}

      {/* New review form */}
      <div className="card card-sub">
        <h3>Write psychological review</h3>
        <p className="field-hint">
          Use this section during your regular consultation / review to capture
          formulation, protective factors and recommendations. It will be saved
          against this child.
        </p>

        <form className="form-grid" onSubmit={handleSubmit}>
          <div className="form-row-2">
            <div className="form-group">
              <label className="label">Review from</label>
              <input
                type="date"
                className="input"
                value={form.review_from}
                onChange={(e) =>
                  handleChange({ review_from: e.target.value })
                }
              />
            </div>
            <div className="form-group">
              <label className="label">Review to</label>
              <input
                type="date"
                className="input"
                value={form.review_to}
                onChange={(e) => handleChange({ review_to: e.target.value })}
              />
            </div>
          </div>

          <div className="form-group">
            <label className="label">Formulation summary</label>
            <textarea
              className="input"
              rows={4}
              value={form.formulation}
              onChange={(e) =>
                handleChange({ formulation: e.target.value })
              }
              placeholder="Brief psychological formulation based on scores, presentation, history and current context."
            />
          </div>

          <div className="form-group">
            <label className="label">Protective factors</label>
            <textarea
              className="input"
              rows={3}
              value={form.protective_factors}
              onChange={(e) =>
                handleChange({ protective_factors: e.target.value })
              }
              placeholder="What is helping this child (relationships, skills, supports, interests, stability, etc.)?"
            />
          </div>

          <div className="form-group">
            <label className="label">Recommendations / plan</label>
            <textarea
              className="input"
              rows={4}
              value={form.recommendations}
              onChange={(e) =>
                handleChange({ recommendations: e.target.value })
              }
              placeholder="Concrete recommendations for staff, care planning, risk management, and therapeutic focus."
            />
          </div>

          <button
            type="submit"
            className="btn btn-primary"
            disabled={saving}
          >
            {saving ? "Saving review…" : "Save review"}
          </button>
        </form>
      </div>

      {/* Previous reviews */}
      <div className="card card-sub">
        <h3>Previous reviews</h3>
        {reviews.length === 0 && (
          <p className="field-hint">
            No previous psychological reviews saved yet.
          </p>
        )}
        {reviews.length > 0 && (
          <ul className="doc-list">
            {reviews.map((rev) => (
              <li key={rev.id} className="doc-list-item">
                <strong>
                  Review {rev.review_from} → {rev.review_to}
                </strong>{" "}
                – {rev.created_by} ·{" "}
                {new Date(rev.created_at).toLocaleString(undefined, {
                  dateStyle: "medium",
                  timeStyle: "short",
                })}
                {rev.formulation && (
                  <div className="field-hint">
                    <strong>Formulation:</strong> {rev.formulation}
                  </div>
                )}
                {rev.protective_factors && (
                  <div className="field-hint">
                    <strong>Protective factors:</strong>{" "}
                    {rev.protective_factors}
                  </div>
                )}
                {rev.recommendations && (
                  <div className="field-hint">
                    <strong>Recommendations:</strong>{" "}
                    {rev.recommendations}
                  </div>
                )}
              </li>
            ))}
          </ul>
        )}
      </div>
    </section>
  );
};

export default TherapeuticSummaryTab;
