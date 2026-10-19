import { useEffect, useState } from "react";
import { apiFetch } from "./api";
import {
  CARE_PLAN_SECTIONS,
  carePlanDate,
  carePlanReviewOutcomeLabel,
  carePlanSection,
  carePlanStatusLabel,
  carePlanVersion,
} from "./carePlanConfig";
import type { Child, ChildCarePlan, ChildCarePlanReview } from "./types";

type Props = {
  apiBase: string;
  child: Child;
  onBackToPlan: () => void;
};

export default function CarePlanHistoryView({ apiBase, child, onBackToPlan }: Props) {
  const [plans, setPlans] = useState<ChildCarePlan[]>([]);
  const [selectedPlan, setSelectedPlan] = useState<ChildCarePlan | null>(null);
  const [reviews, setReviews] = useState<ChildCarePlanReview[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadReviews = async (plan: ChildCarePlan) => {
    const response = await apiFetch(`${apiBase}/v1/children/${child.id}/care-plans/${plan.id}/reviews`);
    setReviews(response.ok ? await response.json() : []);
  };

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await apiFetch(`${apiBase}/v1/children/${child.id}/care-plans/history`);
      if (!response.ok) throw new Error("Plan History could not be loaded.");
      const history = await response.json() as ChildCarePlan[];
      setPlans(history);
      const current = history.find((plan) => plan.status === "active") ?? history.at(-1) ?? null;
      setSelectedPlan(current);
      if (current) await loadReviews(current); else setReviews([]);
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Plan History could not be loaded.");
    } finally {
      setLoading(false);
    }
  };

  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => { void load(); }, [child.id]);

  const openPlan = async (plan: ChildCarePlan) => {
    setSelectedPlan(plan);
    setError(null);
    await loadReviews(plan);
  };

  const orderedPlans = [...plans].reverse();

  return (
    <section className="card care-plan-builder care-plan-history-page">
      <header className="card-header care-plan-header"><div><span className="care-eyebrow">Read-only record · {child.initials}</span><h2>Plan History</h2><p className="card-subtitle">Care Plan versions and professional review decisions.</p></div><button className="btn btn-secondary btn-xs" type="button" onClick={onBackToPlan}>← Current Care Plan</button></header>
      {error && <div className="alert alert-error">{error}</div>}
      {loading && <div className="care-empty-state">Loading Plan History…</div>}
      {!loading && plans.length === 0 && <div className="care-empty-state">No Care Plan versions have been recorded.</div>}
      {!loading && plans.length > 0 && (
        <>
          <nav className="care-history-list" aria-label="Care Plan versions">
            {orderedPlans.map((plan) => (
              <button key={plan.id} type="button" className={selectedPlan?.id === plan.id ? "is-selected" : ""} onClick={() => void openPlan(plan)}>
                <span><strong>Plan {carePlanVersion(plans, plan.id)}</strong><small>Effective {carePlanDate(plan.effective_from)}</small></span>
                <span className={`care-status care-status-${plan.status}`}>{carePlanStatusLabel(plan.status)}</span>
              </button>
            ))}
          </nav>

          {selectedPlan && (
            <section className="care-history-document" aria-label={`Plan ${carePlanVersion(plans, selectedPlan.id)} read-only view`}>
              <div className="care-viewing-banner"><div><span className="care-eyebrow">Read-only version</span><h3>Plan {carePlanVersion(plans, selectedPlan.id)} — {carePlanStatusLabel(selectedPlan.status)}</h3></div><div className="care-review-period"><span>Effective {carePlanDate(selectedPlan.effective_from)}</span><span>Next review {carePlanDate(selectedPlan.review_date)}</span></div></div>
              <div className="care-history-sections">
                {CARE_PLAN_SECTIONS.map((config) => {
                  const section = carePlanSection(selectedPlan, config.key);
                  return <details key={config.key}><summary><strong>{config.title}</strong><span className={`care-status care-status-${section?.information_status ?? "not_known_yet"}`}>{carePlanStatusLabel(section?.information_status ?? "not_known_yet")}</span></summary><div className="care-section-reading"><div><h4>Current understanding</h4><p>{section?.current_understanding || "Not recorded"}</p></div><div><h4>Strengths</h4><p>{section?.strengths || "Not recorded"}</p></div><div><h4>Everyday staff support</h4><p>{section?.staff_support || "Not recorded"}</p></div><div><h4>Young person involvement</h4><p>{section?.young_person_involvement_note || "Not recorded"}</p></div></div></details>;
                })}
              </div>

              <section className="care-review-history"><div className="care-section-heading"><div><span className="care-eyebrow">Professional decisions</span><h3>Review History</h3></div><span>{reviews.length} review{reviews.length === 1 ? "" : "s"}</span></div>{reviews.length === 0 ? <p className="field-hint">No manager review was recorded for this version. A version can pre-date the review workflow or have been created through the earlier draft/activation lifecycle.</p> : reviews.map((review) => <details className="care-review-item" key={review.id}><summary><div><strong>{carePlanDate(review.reviewed_at)} · {review.reviewer_display_name}</strong><span>{carePlanReviewOutcomeLabel(review)}</span></div></summary><div className="care-review-item-body"><div><h4>Professional review</h4><p>{review.review_note}</p></div><div><h4>Young person involvement</h4><p>{review.young_person_involvement_note || "Not separately recorded."}</p></div><div className="care-review-period"><span>Next review: {carePlanDate(review.next_review_date)}</span></div></div></details>)}</section>
            </section>
          )}
        </>
      )}
    </section>
  );
}
