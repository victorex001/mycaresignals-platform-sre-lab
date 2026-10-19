import { useEffect, useMemo, useState } from "react";
import type { FormEvent } from "react";
import { apiFetch } from "./api";
import {
  CARE_PLAN_SECTIONS,
  carePlanDate,
  carePlanDomainMatches,
  carePlanReviewOutcomeLabel,
} from "./carePlanConfig";
import type {
  CarePlanReviewEvidence,
  Child,
  ChildCareOverview,
  ChildCarePlan,
  ChildCarePlanReview,
  ChildGoal,
  ChildRisk,
} from "./types";

type Props = {
  apiBase: string;
  child: Child;
  onBackToPlan: () => void;
  onOpenGoals: () => void;
  onOpenRisks: () => void;
  onOpenKeywork: () => void;
  onOpenDailyProgress: () => void;
  onOpenVoice: () => void;
  onOpenHistory: () => void;
};

type ReviewDecision = "continue" | "needs_information" | "revise" | "close";

export default function CarePlanReviewView({
  apiBase,
  child,
  onBackToPlan,
  onOpenGoals,
  onOpenRisks,
  onOpenKeywork,
  onOpenDailyProgress,
  onOpenVoice,
  onOpenHistory,
}: Props) {
  const [plan, setPlan] = useState<ChildCarePlan | null>(null);
  const [evidence, setEvidence] = useState<CarePlanReviewEvidence | null>(null);
  const [reviews, setReviews] = useState<ChildCarePlanReview[]>([]);
  const [goals, setGoals] = useState<ChildGoal[]>([]);
  const [risks, setRisks] = useState<ChildRisk[]>([]);
  const [dailyProgress, setDailyProgress] = useState<ChildCareOverview["recent_daily_progress"]>([]);
  const [canReview, setCanReview] = useState(false);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [reviewNote, setReviewNote] = useState("");
  const [involvementNote, setInvolvementNote] = useState("");
  const [decision, setDecision] = useState<ReviewDecision>("continue");
  const [nextReviewDate, setNextReviewDate] = useState("");

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const [planResponse, accessResponse, goalsResponse, risksResponse, overviewResponse] = await Promise.all([
        apiFetch(`${apiBase}/v1/children/${child.id}/care-plans`),
        apiFetch(`${apiBase}/v1/children/${child.id}/care-plans/access`),
        apiFetch(`${apiBase}/v1/children/${child.id}/goals`),
        apiFetch(`${apiBase}/v1/children/${child.id}/risks`),
        apiFetch(`${apiBase}/v1/children/${child.id}/care-overview`),
      ]);
      if (!planResponse.ok) throw new Error("The Care Plan review could not be loaded.");
      const data = await planResponse.json() as { plan: ChildCarePlan | null };
      const access = accessResponse.ok ? await accessResponse.json() as { can_review: boolean } : { can_review: false };
      setPlan(data.plan);
      setCanReview(access.can_review);
      setGoals(goalsResponse.ok ? await goalsResponse.json() : []);
      setRisks(risksResponse.ok ? await risksResponse.json() : []);
      const overview = overviewResponse.ok ? await overviewResponse.json() as ChildCareOverview : null;
      setDailyProgress(overview?.recent_daily_progress ?? []);
      if (data.plan?.status === "active") {
        const [evidenceResponse, reviewsResponse] = await Promise.all([
          apiFetch(`${apiBase}/v1/children/${child.id}/care-plans/${data.plan.id}/review-evidence`),
          apiFetch(`${apiBase}/v1/children/${child.id}/care-plans/${data.plan.id}/reviews`),
        ]);
        setEvidence(evidenceResponse.ok ? await evidenceResponse.json() : null);
        setReviews(reviewsResponse.ok ? await reviewsResponse.json() : []);
        setNextReviewDate(data.plan.review_date?.slice(0, 10) ?? "");
      } else {
        setEvidence(null);
        setReviews([]);
      }
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "The Care Plan review could not be loaded.");
    } finally {
      setLoading(false);
    }
  };

  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => { void load(); }, [child.id]);

  const prompts = useMemo(() => {
    if (!plan || !evidence) return [];
    const items: string[] = [];
    if (evidence.sections_awaiting_confirmation_count > 0) items.push(`${evidence.sections_awaiting_confirmation_count} area${evidence.sections_awaiting_confirmation_count === 1 ? " is" : "s are"} awaiting confirmation. Review if appropriate.`);
    if (evidence.sections_not_known_count > 0) items.push(`${evidence.sections_not_known_count} area${evidence.sections_not_known_count === 1 ? " has" : "s have"} information not known yet. Review if appropriate.`);
    if (evidence.targets_due_count > 0) items.push(`${evidence.targets_due_count} active SMART target${evidence.targets_due_count === 1 ? " is" : "s are"} due for review.`);
    const activeGoals = goals.filter((goal) => goal.status === "active");
    for (const section of CARE_PLAN_SECTIONS.filter((item) => item.key !== "about_me")) {
      if (!activeGoals.some((goal) => carePlanDomainMatches(goal.domain, section))) items.push(`${section.title} has no current goal. This may be appropriate — review if appropriate.`);
    }
    const unlinkedRisks = risks.filter((risk) => risk.status === "active" && risk.management_approach === "controls_and_developmental_work" && (risk.goal_ids?.length ?? 0) === 0 && (risk.target_ids?.length ?? 0) === 0);
    if (unlinkedRisks.length > 0) items.push(`${unlinkedRisks.length} active risk${unlinkedRisks.length === 1 ? " has" : "s have"} developmental work without a current planning link. Review if appropriate.`);
    return items.slice(0, 6);
  }, [evidence, goals, plan, risks]);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (!plan) return;
    setSaving(true);
    setError(null);
    setSuccess(null);
    try {
      const recordedNote = decision === "needs_information"
        ? `Needs more information.\n\n${reviewNote.trim()}`
        : reviewNote.trim();
      const response = await apiFetch(`${apiBase}/v1/children/${child.id}/care-plans/${plan.id}/reviews`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          review_note: recordedNote,
          young_person_involvement_note: involvementNote,
          outcome: decision === "needs_information" ? "continue" : decision,
          next_review_date: decision === "close" ? null : nextReviewDate || null,
        }),
      });
      if (!response.ok) throw new Error((await response.text()) || "The professional review could not be recorded.");
      const result = await response.json() as { successor_plan?: ChildCarePlan | null };
      await load();
      setReviewNote("");
      setInvolvementNote("");
      setDecision("continue");
      setSuccess(result.successor_plan
        ? "Review recorded. A separate successor draft is ready; the approved plan was not silently changed."
        : decision === "close"
          ? "Review recorded and the Care Plan closed."
          : "Review recorded. The current plan remains active.");
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "The professional review could not be recorded.");
    } finally {
      setSaving(false);
    }
  };

  return (
    <section className="card care-plan-builder care-plan-review-page">
      <header className="card-header care-plan-header"><div><span className="care-eyebrow">Manager review · {child.initials}</span><h2>Review Care Plan</h2><p className="card-subtitle">Does the current Care Plan need to change?</p></div><button className="btn btn-secondary btn-xs" type="button" onClick={onBackToPlan}>← Current Care Plan</button></header>
      {error && <div className="alert alert-error">{error}</div>}
      {success && <div className="alert alert-success">{success}</div>}
      {loading && <div className="care-empty-state">Loading review context…</div>}
      {!loading && !canReview && <div className="alert alert-warning">Manager access is required to review a Care Plan.</div>}
      {!loading && canReview && (!plan || plan.status !== "active") && <div className="care-empty-state">There is no active Care Plan to review.</div>}
      {!loading && canReview && plan?.status === "active" && evidence && (
        <>
          <div className="care-review-period"><span>Plan effective: {carePlanDate(plan.effective_from)}</span><span>Evidence since: {carePlanDate(evidence.evidence_since)}</span><span>Current review date: {carePlanDate(plan.review_date)}</span></div>
          <section className="care-review-evidence" aria-labelledby="care-review-evidence-heading">
            <div className="care-simple-heading"><div><span className="care-eyebrow">Context, not conclusions</span><h3 id="care-review-evidence-heading">Evidence since the last review</h3><p>Counts organise available records. They do not make a professional decision.</p></div></div>
            <div className="care-review-evidence-grid">
              <button type="button" onClick={onOpenGoals}><span>Goals &amp; Targets</span><strong>{evidence.active_goal_count} active goals · {evidence.active_target_count} active SMART targets</strong><p>{evidence.targets_changed_count} changed · {evidence.targets_due_count} due for review</p></button>
              <button type="button" onClick={onOpenRisks}><span>Risk &amp; Safety</span><strong>{evidence.active_risk_count} active risks</strong><p>{evidence.risk_reviews_completed_count} reviews completed in this period</p></button>
              <button type="button" onClick={onOpenKeywork}><span>Key Working</span><strong>{evidence.keyworking_session_count} sessions recorded</strong><p>Open the separate evidence record</p></button>
              <button type="button" onClick={onOpenDailyProgress}><span>Daily Progress</span><strong>{dailyProgress.filter((item) => new Date(item.observed_on).getTime() >= new Date(evidence.evidence_since).getTime()).length} recent records</strong><p>Open observations and progress context</p></button>
              <button type="button" onClick={onOpenVoice}><span>Young Person Voice</span><strong>Open their perspective</strong><p>Voice remains a separate first-class record</p></button>
              <div><span>Unresolved Care Plan information</span><strong>{evidence.sections_not_known_count} not known · {evidence.sections_awaiting_confirmation_count} awaiting confirmation</strong><p>Information states are not automated judgements</p></div>
            </div>
          </section>

          {prompts.length > 0 && <aside className="care-plan-guidance"><div><span className="care-eyebrow">Planning prompts</span><h3>Review if appropriate</h3></div><ul>{prompts.map((prompt) => <li key={prompt}>{prompt}</li>)}</ul><p>Prompts do not block or determine the review outcome.</p></aside>}

          <form className="care-plan-review-form" onSubmit={(event) => void submit(event)}>
            <div><span className="care-eyebrow">Human decision</span><h3>Record professional review</h3></div>
            <div className="form-group"><label className="label" htmlFor="care-review-note">Professional review</label><textarea id="care-review-note" className="input" rows={6} required value={reviewNote} onChange={(event) => setReviewNote(event.target.value)} placeholder="What is working? What has changed? What needs attention?" /></div>
            <div className="form-group"><label className="label" htmlFor="care-review-involvement">How has the young person been involved?</label><textarea id="care-review-involvement" className="input" rows={3} value={involvementNote} onChange={(event) => setInvolvementNote(event.target.value)} /></div>
            <fieldset className="care-review-outcomes"><legend>Professional outcome</legend>
              <label><input type="radio" name="review-decision" checked={decision === "continue"} onChange={() => setDecision("continue")} /><span><strong>Continue unchanged</strong><small>The active plan remains current.</small></span></label>
              <label><input type="radio" name="review-decision" checked={decision === "needs_information"} onChange={() => setDecision("needs_information")} /><span><strong>Needs more information</strong><small>Keep the plan active, record the information gap and set the next review.</small></span></label>
              <label><input type="radio" name="review-decision" checked={decision === "revise"} onChange={() => setDecision("revise")} /><span><strong>Revise</strong><small>Create a successor draft without silently changing this active plan.</small></span></label>
              <label><input type="radio" name="review-decision" checked={decision === "close"} onChange={() => { setDecision("close"); setNextReviewDate(""); }} /><span><strong>Close</strong><small>Archive this plan without changing Goals, Risk or evidence records.</small></span></label>
            </fieldset>
            {decision !== "close" && <div className="form-group"><label className="label" htmlFor="care-next-review">Next review date {decision === "continue" || decision === "needs_information" ? "(required)" : "for successor draft"}</label><input id="care-next-review" className="input" type="date" required={decision === "continue" || decision === "needs_information"} value={nextReviewDate} onChange={(event) => setNextReviewDate(event.target.value)} /></div>}
            <div className="care-form-actions"><button className="btn btn-primary" type="submit" disabled={saving}>{saving ? "Recording review…" : "Record professional review"}</button></div>
          </form>

          <section className="care-review-history"><div className="care-section-heading"><div><span className="care-eyebrow">Previous decisions</span><h3>Review History</h3></div><button className="btn btn-ghost btn-xs" type="button" onClick={onOpenHistory}>Open full Plan History</button></div>{reviews.length === 0 ? <p className="field-hint">No professional reviews have been recorded for this plan.</p> : reviews.map((review) => <article className="care-review-summary" key={review.id}><div><strong>{carePlanDate(review.reviewed_at)} · {review.reviewer_display_name}</strong><span>{carePlanReviewOutcomeLabel(review)}</span></div><p>{review.review_note}</p></article>)}</section>
        </>
      )}
    </section>
  );
}
