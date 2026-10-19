import { useEffect, useMemo, useState } from "react";
import type { FormEvent } from "react";
import { apiFetch } from "./api";
import type {
  ChildGoal,
  ChildRisk,
  ChildRiskReview,
  ChildTarget,
  RiskInformationStatus,
  RiskManagementApproach,
  RiskReviewOutcome,
  RiskStatus,
} from "./types";

type Props = {
  apiBase: string;
  childId: number;
  childInitials: string;
  onBackToDashboard: () => void;
  onOpenGoals: () => void;
};

type RiskForm = {
  title: string; category: string; domain: string; status: RiskStatus;
  natureOfRisk: string; whoIsAtRisk: string; informationStatus: RiskInformationStatus;
  sourceNote: string; initialLikelihood: string; initialSeverity: string;
  residualLikelihood: string; residualSeverity: string;
  managementApproach: RiskManagementApproach; proactiveStrategies: string;
  reactiveStrategies: string; effectiveFrom: string; nextReviewDate: string;
  goalIds: number[]; targetIds: number[];
};

type ReviewForm = {
  likelihood: string;
  severity: string;
  note: string;
  outcome: RiskReviewOutcome;
  nextReviewDate: string;
};

const DOMAINS = [
  "Emotional & Psychological Wellbeing", "Behaviour & Self-Regulation",
  "Relationships & Connection", "Daily Living, Learning & Participation",
  "Safety, Risk & Coping with Stress", "Skills, Identity & Future",
];
const INFORMATION_STATUSES: Array<{ value: RiskInformationStatus; label: string }> = [
  { value: "current_confirmed", label: "Current and confirmed" },
  { value: "reported", label: "Reported information" },
  { value: "historical", label: "Historical information" },
  { value: "allegation_not_established", label: "Allegation not established" },
  { value: "awaiting_confirmation", label: "Awaiting confirmation" },
];
const RATINGS = [1, 2, 3, 4, 5];
const today = () => new Date().toISOString().slice(0, 10);
const dateValue = (value?: string | null) => value?.slice(0, 10) ?? "";
const displayDate = (value?: string | null) => value
  ? new Date(`${value.slice(0, 10)}T00:00:00`).toLocaleDateString(undefined, { dateStyle: "medium" })
  : "Not set";
const sentence = (value: string) => value.replaceAll("_", " ").replace(/^./, (letter) => letter.toUpperCase());

const emptyRisk = (): RiskForm => ({
  title: "", category: "", domain: "", status: "active", natureOfRisk: "", whoIsAtRisk: "",
  informationStatus: "awaiting_confirmation", sourceNote: "", initialLikelihood: "", initialSeverity: "",
  residualLikelihood: "", residualSeverity: "", managementApproach: "controls_only",
  proactiveStrategies: "", reactiveStrategies: "", effectiveFrom: today(), nextReviewDate: "",
  goalIds: [], targetIds: [],
});

const riskRating = (likelihood?: number | null, severity?: number | null) => {
  if (!likelihood || !severity) return null;
  const score = likelihood * severity;
  return { score, band: score <= 4 ? "low" : score <= 9 ? "medium" : "high" } as const;
};

const RiskSafetyTab = ({ apiBase, childId, childInitials, onBackToDashboard, onOpenGoals }: Props) => {
  const [risks, setRisks] = useState<ChildRisk[]>([]);
  const [goals, setGoals] = useState<ChildGoal[]>([]);
  const [targets, setTargets] = useState<ChildTarget[]>([]);
  const [reviewsByRisk, setReviewsByRisk] = useState<Record<number, ChildRiskReview[]>>({});
  const [form, setForm] = useState<RiskForm>(emptyRisk);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [formOpen, setFormOpen] = useState(false);
  const [reviewingRiskId, setReviewingRiskId] = useState<number | null>(null);
  const [reviewForm, setReviewForm] = useState<ReviewForm>({ likelihood: "", severity: "", note: "", outcome: "continue", nextReviewDate: "" });
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  const load = async () => {
    setLoading(true); setError(null);
    try {
      const [riskResponse, goalResponse, targetResponse] = await Promise.all([
        apiFetch(`${apiBase}/v1/children/${childId}/risks`),
        apiFetch(`${apiBase}/v1/children/${childId}/goals`),
        apiFetch(`${apiBase}/v1/children/${childId}/targets`),
      ]);
      if (!riskResponse.ok) throw new Error("The current risk and safety picture could not be loaded.");
      if (!goalResponse.ok || !targetResponse.ok) throw new Error("Planning links could not be loaded.");
      const [riskData, goalData, targetData] = await Promise.all([
        riskResponse.json(), goalResponse.json(), targetResponse.json(),
      ]);
      setRisks(Array.isArray(riskData) ? riskData : []);
      setGoals(Array.isArray(goalData) ? goalData : []);
      setTargets(Array.isArray(targetData) ? targetData : []);
      const riskList: ChildRisk[] = Array.isArray(riskData) ? riskData : [];
      const reviewEntries = await Promise.all(riskList.map(async (risk) => {
        const response = await apiFetch(`${apiBase}/v1/children/${childId}/risks/${risk.id}/reviews`);
        const data: ChildRiskReview[] = response.ok ? await response.json() : [];
        return [risk.id, Array.isArray(data) ? data : []] as const;
      }));
      setReviewsByRisk(Object.fromEntries(reviewEntries));
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Risk & Safety could not be loaded.");
    } finally { setLoading(false); }
  };

  useEffect(() => { void load(); }, [childId]); // eslint-disable-line react-hooks/exhaustive-deps

  const activeRisks = risks.filter((risk) => risk.status === "active");
  const bandCounts = useMemo(() => activeRisks.reduce((counts, risk) => {
    const rating = riskRating(risk.residual_likelihood, risk.residual_severity);
    if (rating) counts[rating.band] += 1;
    return counts;
  }, { low: 0, medium: 0, high: 0 }), [activeRisks]);
  const reviewsDue = activeRisks.filter((risk) => risk.next_review_date && dateValue(risk.next_review_date) <= today()).length;
  const targetsForSelectedGoals = form.goalIds.length === 0
    ? targets : targets.filter((target) => target.goal_id && form.goalIds.includes(target.goal_id));

  const closeForm = () => { setFormOpen(false); setEditingId(null); setForm(emptyRisk()); };
  const createRisk = () => { setForm(emptyRisk()); setEditingId(null); setFormOpen(true); setSuccess(null); };
  const editRisk = (risk: ChildRisk) => {
    setForm({
      title: risk.title, category: risk.category, domain: risk.domain ?? "", status: risk.status,
      natureOfRisk: risk.nature_of_risk, whoIsAtRisk: risk.who_is_at_risk ?? "",
      informationStatus: risk.information_status, sourceNote: risk.source_note ?? "",
      initialLikelihood: risk.initial_likelihood ? String(risk.initial_likelihood) : "",
      initialSeverity: risk.initial_severity ? String(risk.initial_severity) : "",
      residualLikelihood: risk.residual_likelihood ? String(risk.residual_likelihood) : "",
      residualSeverity: risk.residual_severity ? String(risk.residual_severity) : "",
      managementApproach: risk.management_approach, proactiveStrategies: risk.proactive_strategies ?? "",
      reactiveStrategies: risk.reactive_strategies ?? "", effectiveFrom: dateValue(risk.effective_from),
      nextReviewDate: dateValue(risk.next_review_date), goalIds: risk.goal_ids ?? [], targetIds: risk.target_ids ?? [],
    });
    setEditingId(risk.id); setFormOpen(true); setSuccess(null);
  };
  const startReview = (risk: ChildRisk) => {
    setReviewingRiskId(risk.id);
    setReviewForm({
      likelihood: risk.residual_likelihood ? String(risk.residual_likelihood) : "",
      severity: risk.residual_severity ? String(risk.residual_severity) : "",
      note: "", outcome: "continue", nextReviewDate: dateValue(risk.next_review_date),
    });
    setError(null); setSuccess(null);
  };
  const toggleID = (ids: number[], id: number) => ids.includes(id) ? ids.filter((value) => value !== id) : [...ids, id];
  const numberOrNull = (value: string) => value ? Number(value) : null;

  const save = async (event: FormEvent) => {
    event.preventDefault(); setError(null); setSuccess(null);
    if (!form.title.trim() || !form.category.trim() || !form.natureOfRisk.trim()) {
      setError("Record a title, category and clear description of the concern."); return;
    }
    const pairs = [[form.initialLikelihood, form.initialSeverity], [form.residualLikelihood, form.residualSeverity]];
    if (pairs.some(([likelihood, severity]) => Boolean(likelihood) !== Boolean(severity))) {
      setError("For each rating, choose both likelihood and severity or leave both unknown."); return;
    }
    const editing = editingId !== null;
    setSaving(true);
    try {
      const response = await apiFetch(editing
        ? `${apiBase}/v1/children/${childId}/risks/${editingId}`
        : `${apiBase}/v1/children/${childId}/risks`, {
        method: editing ? "PATCH" : "POST", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          title: form.title.trim(), category: form.category.trim(), domain: form.domain,
          status: form.status, nature_of_risk: form.natureOfRisk.trim(), who_is_at_risk: form.whoIsAtRisk.trim(),
          information_status: form.informationStatus, source_note: form.sourceNote.trim(),
          initial_likelihood: numberOrNull(form.initialLikelihood), initial_severity: numberOrNull(form.initialSeverity),
          residual_likelihood: numberOrNull(form.residualLikelihood), residual_severity: numberOrNull(form.residualSeverity),
          management_approach: form.managementApproach, proactive_strategies: form.proactiveStrategies.trim(),
          reactive_strategies: form.reactiveStrategies.trim(), effective_from: form.effectiveFrom || null,
          next_review_date: form.nextReviewDate || null, goal_ids: form.goalIds, target_ids: form.targetIds,
        }),
      });
      if (!response.ok) throw new Error((await response.text()) || "The risk record could not be saved.");
      closeForm(); await load(); setSuccess(editing ? "Risk record updated." : "Risk record created.");
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "The risk record could not be saved.");
    } finally { setSaving(false); }
  };

  const saveReview = async (event: FormEvent, risk: ChildRisk) => {
    event.preventDefault(); setError(null); setSuccess(null);
    if (!reviewForm.likelihood || !reviewForm.severity || !reviewForm.note.trim()) {
      setError("Choose both reviewed ratings and record the practitioner review note."); return;
    }
    setSaving(true);
    try {
      const response = await apiFetch(`${apiBase}/v1/children/${childId}/risks/${risk.id}/reviews`, {
        method: "POST", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          reviewed_residual_likelihood: Number(reviewForm.likelihood),
          reviewed_residual_severity: Number(reviewForm.severity),
          review_note: reviewForm.note.trim(), outcome: reviewForm.outcome,
          next_review_date: reviewForm.outcome === "close" ? null : reviewForm.nextReviewDate || null,
        }),
      });
      if (!response.ok) throw new Error((await response.text()) || "The professional review could not be recorded.");
      setReviewingRiskId(null); await load(); setSuccess("Professional risk review recorded in immutable history.");
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "The professional review could not be recorded.");
    } finally { setSaving(false); }
  };

  const ratingSelect = (id: string, label: string, value: string, onChange: (value: string) => void) => (
    <div className="form-group"><label className="label" htmlFor={id}>{label}</label>
      <select className="input" id={id} value={value} onChange={(event) => onChange(event.target.value)}>
        <option value="">Unknown</option>{RATINGS.map((rating) => <option key={rating} value={rating}>{rating}</option>)}
      </select>
    </div>
  );
  const initialRating = riskRating(numberOrNull(form.initialLikelihood), numberOrNull(form.initialSeverity));
  const residualRating = riskRating(numberOrNull(form.residualLikelihood), numberOrNull(form.residualSeverity));

  return <section className="card risk-workspace">
    <div className="card-header care-plan-header"><div><span className="care-eyebrow">Professional safety picture · {childInitials}</span>
      <h2>Risk &amp; Safety</h2><p className="card-subtitle">Structure the team&apos;s current assessment, agreed controls and review needs. Professional decisions remain with staff.</p>
    </div><div className="care-header-actions"><button className="btn btn-ghost" type="button" onClick={onBackToDashboard}>← Child Home</button><button className="btn btn-primary" type="button" onClick={createRisk}>Record risk</button></div></div>
    {error && <div className="alert alert-error">{error}</div>}{success && <div className="alert alert-success">{success}</div>}
    <div className="risk-summary" aria-label="Current risk record summary">
      <div><strong>{activeRisks.length}</strong><span>Active risks</span></div><div className="risk-high"><strong>{bandCounts.high}</strong><span>High ratings</span></div>
      <div className="risk-medium"><strong>{bandCounts.medium}</strong><span>Medium ratings</span></div><div className="risk-low"><strong>{bandCounts.low}</strong><span>Low ratings</span></div>
      <div><strong>{reviewsDue}</strong><span>Reviews due</span></div>
    </div><p className="field-hint">Bands show only the recorded residual likelihood × severity. They do not determine whether the young person is safe or what action staff should take.</p>

    {formOpen && <form className="care-guided-form risk-form" onSubmit={save}>
      <div className="care-form-heading"><div><span className="care-eyebrow">{editingId ? "Update current record" : "New risk record"}</span><h3>{editingId ? "Edit risk" : "Record the professional safety picture"}</h3></div><button type="button" className="btn btn-ghost btn-xs" onClick={closeForm}>Cancel</button></div>
      <fieldset><legend>1. What is the concern?</legend><div className="form-row-2"><div className="form-group"><label className="label" htmlFor="risk-title">Short, clear title</label><input id="risk-title" className="input" value={form.title} onChange={(e) => setForm({ ...form, title: e.target.value })} required /></div><div className="form-group"><label className="label" htmlFor="risk-category">Team&apos;s agreed category</label><input id="risk-category" className="input" value={form.category} onChange={(e) => setForm({ ...form, category: e.target.value })} required /></div></div>
        <div className="form-group"><label className="label" htmlFor="risk-nature">What is the nature of the risk?</label><textarea id="risk-nature" className="input" rows={3} value={form.natureOfRisk} onChange={(e) => setForm({ ...form, natureOfRisk: e.target.value })} required /></div>
        <div className="form-row-2"><div className="form-group"><label className="label" htmlFor="risk-who">Who may be at risk?</label><textarea id="risk-who" className="input" rows={2} value={form.whoIsAtRisk} onChange={(e) => setForm({ ...form, whoIsAtRisk: e.target.value })} /></div><div className="form-group"><label className="label" htmlFor="risk-domain">Related domain <span className="label-pill">Optional</span></label><select id="risk-domain" className="input" value={form.domain} onChange={(e) => setForm({ ...form, domain: e.target.value })}><option value="">No domain selected</option>{DOMAINS.map((domain) => <option key={domain}>{domain}</option>)}</select></div></div>
      </fieldset>
      <fieldset><legend>2. What do we know about this information?</legend><div className="form-row-2"><div className="form-group"><label className="label" htmlFor="risk-info">Information status</label><select id="risk-info" className="input" value={form.informationStatus} onChange={(e) => setForm({ ...form, informationStatus: e.target.value as RiskInformationStatus })}>{INFORMATION_STATUSES.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}</select></div><div className="form-group"><label className="label" htmlFor="risk-source">Source / provenance</label><textarea id="risk-source" className="input" rows={2} value={form.sourceNote} onChange={(e) => setForm({ ...form, sourceNote: e.target.value })} placeholder="Record the source and what remains to be confirmed." /></div></div></fieldset>
      <fieldset><legend>3. Professional rating</legend><p className="field-hint">Choose the component ratings. The application performs multiplication only.</p><div className="risk-rating-grid">{ratingSelect("initial-l", "Initial likelihood", form.initialLikelihood, (value) => setForm({ ...form, initialLikelihood: value }))}{ratingSelect("initial-s", "Initial severity", form.initialSeverity, (value) => setForm({ ...form, initialSeverity: value }))}<div className="risk-calculated"><span>Initial rating</span><strong>{initialRating ? `${initialRating.score} · ${sentence(initialRating.band)}` : "Unknown"}</strong></div>{ratingSelect("residual-l", "Residual likelihood", form.residualLikelihood, (value) => setForm({ ...form, residualLikelihood: value }))}{ratingSelect("residual-s", "Residual severity", form.residualSeverity, (value) => setForm({ ...form, residualSeverity: value }))}<div className="risk-calculated"><span>Current recorded residual rating</span><strong>{residualRating ? `${residualRating.score} · ${sentence(residualRating.band)}` : "Unknown"}</strong></div></div></fieldset>
      <fieldset><legend>4. What helps keep the young person safe?</legend><textarea className="input" rows={3} value={form.proactiveStrategies} onChange={(e) => setForm({ ...form, proactiveStrategies: e.target.value })} placeholder="Record proactive staff and environmental strategies." /></fieldset>
      <fieldset><legend>5. What should staff do if this happens?</legend><textarea className="input" rows={3} value={form.reactiveStrategies} onChange={(e) => setForm({ ...form, reactiveStrategies: e.target.value })} placeholder="Record the agreed response and escalation steps." /></fieldset>
      <fieldset><legend>6. How is this risk managed?</legend><div className="risk-choice-grid"><label className={form.managementApproach === "controls_only" ? "is-selected" : ""}><input type="radio" checked={form.managementApproach === "controls_only"} onChange={() => setForm({ ...form, managementApproach: "controls_only" })} /> <strong>Staff / environmental controls only</strong><span>No therapeutic target is required.</span></label><label className={form.managementApproach === "controls_and_developmental_work" ? "is-selected" : ""}><input type="radio" checked={form.managementApproach === "controls_and_developmental_work"} onChange={() => setForm({ ...form, managementApproach: "controls_and_developmental_work" })} /> <strong>Controls + developmental work</strong><span>Planning links can show the work alongside controls.</span></label></div></fieldset>
      <fieldset><legend>7. Links to planning <span className="label-pill">Optional</span></legend><div className="risk-link-grid"><div><h4>Goals</h4>{goals.length === 0 ? <p className="field-hint">No goals recorded.</p> : goals.map((goal) => <label className="risk-check" key={goal.id}><input type="checkbox" checked={form.goalIds.includes(goal.id)} onChange={() => setForm({ ...form, goalIds: toggleID(form.goalIds, goal.id) })} />{goal.title}</label>)}</div><div><h4>Targets</h4>{targetsForSelectedGoals.length === 0 ? <p className="field-hint">No matching targets recorded.</p> : targetsForSelectedGoals.map((target) => <label className="risk-check" key={target.id}><input type="checkbox" checked={form.targetIds.includes(target.id)} onChange={() => setForm({ ...form, targetIds: toggleID(form.targetIds, target.id) })} />{target.short_label}</label>)}</div></div>{form.managementApproach === "controls_and_developmental_work" && form.goalIds.length === 0 && form.targetIds.length === 0 && <p className="care-inline-guidance">Developmental work is selected but no goal or target is currently linked. Review suggested.</p>}</fieldset>
      <fieldset><legend>8. Review</legend><div className="form-row-2"><div className="form-group"><label className="label" htmlFor="risk-effective">Effective date</label><input id="risk-effective" type="date" className="input" value={form.effectiveFrom} onChange={(e) => setForm({ ...form, effectiveFrom: e.target.value })} /></div><div className="form-group"><label className="label" htmlFor="risk-review">Next review date</label><input id="risk-review" type="date" className="input" value={form.nextReviewDate} onChange={(e) => setForm({ ...form, nextReviewDate: e.target.value })} /></div></div><div className="form-group"><label className="label" htmlFor="risk-status">Current status</label><select id="risk-status" className="input" value={form.status} onChange={(e) => setForm({ ...form, status: e.target.value as RiskStatus })}><option value="active">Active</option><option value="paused">Paused</option><option value="closed">Closed</option></select></div></fieldset>
      <div className="care-form-actions"><button type="button" className="btn btn-ghost" onClick={closeForm}>Cancel</button><button type="submit" className="btn btn-primary" disabled={saving}>{saving ? "Saving…" : editingId ? "Save risk record" : "Create risk record"}</button></div>
    </form>}

    <div className="risk-list-heading"><div><h3>Current records</h3><p className="field-hint">Open a record for the full professional picture and current controls.</p></div></div>
    {loading && <div className="care-empty-state">Loading Risk &amp; Safety…</div>}
    {!loading && risks.length === 0 && <div className="care-empty-state"><strong>No risk records yet</strong><p>Record the current professional safety picture when the team has agreed it.</p></div>}
    <div className="risk-card-list">{risks.map((risk) => {
      const rating = riskRating(risk.residual_likelihood, risk.residual_severity);
      const linkedGoals = goals.filter((goal) => risk.goal_ids?.includes(goal.id));
      const linkedTargets = targets.filter((target) => risk.target_ids?.includes(target.id));
      const schedule = risk.status === "active" && risk.next_review_date
        ? dateValue(risk.next_review_date) < today() ? "Overdue" : dateValue(risk.next_review_date) === today() ? "Review due" : "Upcoming"
        : null;
      const history = reviewsByRisk[risk.id] ?? [];
      const currentReviewRating = riskRating(numberOrNull(reviewForm.likelihood), numberOrNull(reviewForm.severity));
      return <details key={risk.id} className={`risk-card risk-band-${rating?.band ?? "unknown"}`}>
        <summary><div><span className="care-domain-label">{risk.category}</span><h3>{risk.title}</h3><div className="care-meta-row"><span>{sentence(risk.information_status)}</span><span>{risk.management_approach === "controls_only" ? "Staff/environmental controls" : "Controls + developmental work"}</span><span>{(risk.goal_ids?.length ?? 0) + (risk.target_ids?.length ?? 0)} planning links</span>{schedule && <span className={schedule === "Overdue" || schedule === "Review due" ? "care-review-alert" : ""}>{schedule}</span>}</div></div><div className="risk-card-score"><span>Current recorded residual rating</span><strong>{rating ? `${rating.score} · ${sentence(rating.band)}` : "Unknown"}</strong></div></summary>
        <div className="risk-card-body"><dl className="care-detail-grid"><div><dt>Nature of risk</dt><dd>{risk.nature_of_risk}</dd></div><div><dt>Who may be at risk</dt><dd>{risk.who_is_at_risk || "Not recorded"}</dd></div><div><dt>Information status</dt><dd>{sentence(risk.information_status)}</dd></div><div><dt>Source / provenance</dt><dd>{risk.source_note || "Not recorded"}</dd></div><div><dt>Initial rating</dt><dd>{riskRating(risk.initial_likelihood, risk.initial_severity)?.score ?? "Unknown"}</dd></div><div><dt>Next review</dt><dd>{displayDate(risk.next_review_date)}</dd></div><div><dt>Proactive strategies</dt><dd>{risk.proactive_strategies || "Not recorded"}</dd></div><div><dt>Reactive strategies</dt><dd>{risk.reactive_strategies || "Not recorded"}</dd></div></dl><div className="risk-linked-plan"><div><strong>Linked goals</strong><span>{linkedGoals.map((goal) => goal.title).join(", ") || "None"}</span></div><div><strong>Linked targets</strong><span>{linkedTargets.map((target) => target.short_label).join(", ") || "None"}</span></div></div><div className="care-card-actions"><button className="btn btn-primary btn-xs" type="button" onClick={() => startReview(risk)}>Review Risk</button><button className="btn btn-ghost btn-xs" type="button" onClick={() => editRisk(risk)}>Edit risk</button>{(linkedGoals.length > 0 || linkedTargets.length > 0) && <button className="btn btn-secondary btn-xs" type="button" onClick={onOpenGoals}>Open linked planning</button>}</div>
          {reviewingRiskId === risk.id && <form className="risk-review-form" onSubmit={(event) => void saveReview(event, risk)}><div className="care-form-heading"><div><span className="care-eyebrow">Professional review</span><h4>Review Risk</h4></div><button type="button" className="btn btn-ghost btn-xs" onClick={() => setReviewingRiskId(null)}>Cancel</button></div><div className="risk-review-current"><div><span>Current recorded residual rating</span><strong>{rating ? `${rating.score} · ${sentence(rating.band)}` : "Unknown"}</strong></div><div><span>Current controls</span><strong>{risk.proactive_strategies || risk.reactive_strategies ? "Recorded" : "Not recorded"}</strong></div><div><span>Planning links</span><strong>{linkedGoals.length} goals · {linkedTargets.length} targets</strong></div></div><div className="form-row-2">{ratingSelect(`review-l-${risk.id}`, "Reviewed residual likelihood", reviewForm.likelihood, (value) => setReviewForm({ ...reviewForm, likelihood: value }))}{ratingSelect(`review-s-${risk.id}`, "Reviewed residual severity", reviewForm.severity, (value) => setReviewForm({ ...reviewForm, severity: value }))}</div><div className="risk-calculated"><span>Practitioner decision · reviewed residual rating</span><strong>{currentReviewRating ? `${currentReviewRating.score} · ${sentence(currentReviewRating.band)}` : "Choose both ratings"}</strong></div><div className="form-group"><label className="label" htmlFor={`review-note-${risk.id}`}>Professional review note</label><textarea id={`review-note-${risk.id}`} className="input" rows={3} value={reviewForm.note} onChange={(event) => setReviewForm({ ...reviewForm, note: event.target.value })} required /></div><div className="form-row-2"><div className="form-group"><label className="label" htmlFor={`review-outcome-${risk.id}`}>Practitioner decision</label><select id={`review-outcome-${risk.id}`} className="input" value={reviewForm.outcome} onChange={(event) => setReviewForm({ ...reviewForm, outcome: event.target.value as RiskReviewOutcome })}><option value="continue">Continue current plan and rating</option><option value="modify">Modify current residual rating</option><option value="close">Close risk</option></select></div>{reviewForm.outcome !== "close" && <div className="form-group"><label className="label" htmlFor={`review-date-${risk.id}`}>Next review date</label><input id={`review-date-${risk.id}`} type="date" className="input" value={reviewForm.nextReviewDate} onChange={(event) => setReviewForm({ ...reviewForm, nextReviewDate: event.target.value })} /></div>}</div>{reviewForm.outcome === "continue" && (Number(reviewForm.likelihood) !== risk.residual_likelihood || Number(reviewForm.severity) !== risk.residual_severity) && <p className="care-inline-guidance">Continue preserves the current rating. Choose Modify if the practitioner is changing it.</p>}<div className="care-form-actions"><button type="button" className="btn btn-ghost" onClick={() => setReviewingRiskId(null)}>Cancel</button><button type="submit" className="btn btn-primary" disabled={saving}>{saving ? "Recording…" : "Record immutable review"}</button></div></form>}
          <details className="risk-review-history"><summary>Review History ({history.length})</summary>{history.length === 0 ? <p className="field-hint">No formal reviews recorded yet.</p> : <div className="risk-review-list">{history.map((review) => { const previous = riskRating(review.previous_residual_likelihood, review.previous_residual_severity); const reviewed = riskRating(review.reviewed_residual_likelihood, review.reviewed_residual_severity); return <article key={review.id}><div className="risk-review-heading"><div><strong>{displayDate(review.reviewed_at)}</strong><span>{review.reviewer_display_name}</span></div><span className="care-status">{sentence(review.outcome)}</span></div><div className="care-meta-row"><span>Previous: {previous ? `${previous.score} · ${sentence(previous.band)}` : "Unknown"}</span><span>Reviewed: {reviewed ? `${reviewed.score} · ${sentence(reviewed.band)}` : "Unknown"}</span>{review.next_review_date && <span>Next review {displayDate(review.next_review_date)}</span>}</div><p>{review.review_note}</p></article>; })}</div>}</details>
        </div>
      </details>;
    })}</div>

    {!loading && activeRisks.length > 0 && <div className="risk-coverage"><div className="risk-list-heading"><div><h3>Plan coverage</h3><p className="field-hint">A concise view for practitioner review. Missing links are advisory only.</p></div></div>{activeRisks.map((risk) => {
      const controls = Boolean(risk.proactive_strategies || risk.reactive_strategies);
      const missingDevelopment = risk.management_approach === "controls_and_developmental_work" && risk.goal_ids.length === 0 && risk.target_ids.length === 0;
      return <div key={risk.id} className="risk-coverage-row"><strong>{risk.title}</strong><span>Controls recorded: {controls ? "YES" : "NO"}</span><span>{risk.management_approach === "controls_only" ? "Staff/environmental controls" : "Controls + developmental work"}</span><span>Goals: {risk.goal_ids.length}</span><span>Targets: {risk.target_ids.length}</span>{missingDevelopment && <span className="care-inline-guidance">Developmental work is selected but no goal or target is currently linked. Review suggested.</span>}</div>;
    })}</div>}
  </section>;
};

export default RiskSafetyTab;
