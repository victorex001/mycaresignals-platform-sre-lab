import { useEffect, useMemo, useState } from "react";
import type { FormEvent } from "react";
import { apiFetch } from "./api";
import type { ChildGoal, ChildRisk, ChildTarget, GoalStatus, KeyworkSession } from "./types";
import { CANONICAL_DOMAINS } from "./framework";

type Props = {
  apiBase: string;
  childId: number;
  childInitials: string;
  onBackToDashboard: () => void;
  onOpenKeywork: () => void;
  onOpenRisks: () => void;
};

const DOMAIN_OPTIONS = CANONICAL_DOMAINS.map((domain) => ({
  value: domain.key,
  help: domain.help,
}));

const STATUS_OPTIONS: GoalStatus[] = ["active", "paused", "achieved", "closed"];

type GoalFormState = {
  goalType: string;
  domain: string;
  title: string;
  description: string;
  status: GoalStatus;
  startDate: string;
  targetDate: string;
  childVoice: string;
  formulationSummary: string;
  protectiveFactors: string;
};

type TargetFormState = {
  goalId: string;
  domain: string;
  shortLabel: string;
  description: string;
  status: GoalStatus;
  supportsNeeded: string;
  barriers: string;
  interventionStrategy: string;
  reviewDate: string;
};

const emptyGoalForm = (): GoalFormState => ({
  goalType: "care_plan", domain: "", title: "", description: "", status: "active", targetDate: "",
  startDate: "",
  childVoice: "", formulationSummary: "", protectiveFactors: "",
});

const emptyTargetForm = (): TargetFormState => ({
  goalId: "", domain: "", shortLabel: "", description: "", status: "active",
  supportsNeeded: "", barriers: "", interventionStrategy: "", reviewDate: "",
});

const cleanDate = (value?: string | null): string => value?.slice(0, 10) ?? "";

const displayDate = (value?: string | null): string => {
  if (!value) return "Not set";
  const [year, month, day] = value.slice(0, 10).split("-").map(Number);
  if (!year || !month || !day) return value;
  return new Date(year, month - 1, day).toLocaleDateString(undefined, { dateStyle: "medium" });
};

const errorMessage = (error: unknown, fallback: string): string =>
  error instanceof Error ? error.message : fallback;

const statusLabel = (status: string): string =>
  status.charAt(0).toUpperCase() + status.slice(1).toLowerCase();

const GoalsTargetsTab = ({
  apiBase, childId, childInitials, onBackToDashboard, onOpenKeywork, onOpenRisks,
}: Props) => {
  const [goals, setGoals] = useState<ChildGoal[]>([]);
  const [targets, setTargets] = useState<ChildTarget[]>([]);
  const [sessions, setSessions] = useState<KeyworkSession[]>([]);
  const [risks, setRisks] = useState<ChildRisk[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [goalForm, setGoalForm] = useState<GoalFormState>(emptyGoalForm);
  const [targetForm, setTargetForm] = useState<TargetFormState>(emptyTargetForm);
  const [goalFormOpen, setGoalFormOpen] = useState(false);
  const [targetFormOpen, setTargetFormOpen] = useState(false);
  const [editingGoalId, setEditingGoalId] = useState<number | null>(null);
  const [editingTargetId, setEditingTargetId] = useState<number | null>(null);
  const [expandedGoalIds, setExpandedGoalIds] = useState<Set<number>>(new Set());

  const loadData = async () => {
    setLoading(true);
    setError(null);
    try {
      const [goalResponse, targetResponse, sessionResponse, riskResponse] = await Promise.all([
        apiFetch(`${apiBase}/v1/children/${childId}/goals`),
        apiFetch(`${apiBase}/v1/children/${childId}/targets`),
        apiFetch(`${apiBase}/v1/children/${childId}/sessions`),
        apiFetch(`${apiBase}/v1/children/${childId}/risks`),
      ]);
      if (!goalResponse.ok) throw new Error("Goals could not be loaded.");
      if (!targetResponse.ok) throw new Error("Targets could not be loaded.");
      const goalData: ChildGoal[] = await goalResponse.json();
      const targetData: ChildTarget[] = await targetResponse.json();
      const sessionData: KeyworkSession[] = sessionResponse.ok ? await sessionResponse.json() : [];
      const riskData: ChildRisk[] = riskResponse.ok ? await riskResponse.json() : [];
      setGoals(Array.isArray(goalData) ? goalData : []);
      setTargets(Array.isArray(targetData) ? targetData : []);
      setSessions(Array.isArray(sessionData) ? sessionData : []);
      setRisks(Array.isArray(riskData) ? riskData : []);
    } catch (caught: unknown) {
      setError(errorMessage(caught, "The care plan could not be loaded."));
      setGoals([]); setTargets([]); setSessions([]); setRisks([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    setGoalFormOpen(false); setTargetFormOpen(false);
    setEditingGoalId(null); setEditingTargetId(null); setSuccess(null);
    void loadData();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [childId]);

  const targetsByGoal = useMemo(() => {
    const grouped = new Map<number, ChildTarget[]>();
    targets.forEach((target) => {
      if (!target.goal_id) return;
      grouped.set(target.goal_id, [...(grouped.get(target.goal_id) ?? []), target]);
    });
    return grouped;
  }, [targets]);

  const sessionsByTarget = useMemo(() => {
    const grouped = new Map<number, KeyworkSession[]>();
    sessions.forEach((session) => {
      if (!session.target_id) return;
      grouped.set(session.target_id, [...(grouped.get(session.target_id) ?? []), session]);
    });
    return grouped;
  }, [sessions]);

  const orderedGoals = useMemo(() => goals.slice().sort((left, right) => {
    if (left.status === "active" && right.status !== "active") return -1;
    if (left.status !== "active" && right.status === "active") return 1;
    return left.title.localeCompare(right.title);
  }), [goals]);

  const unlinkedTargets = targets.filter((target) => !target.goal_id);
  const activeGoalCount = goals.filter((goal) => goal.status === "active").length;
  const activeTargetCount = targets.filter((target) => target.status === "active").length;
  const today = new Date().toISOString().slice(0, 10);
  const reviewDueCount = targets.filter((target) =>
    target.status === "active" && Boolean(target.review_date) && cleanDate(target.review_date) <= today,
  ).length;

  const closeGoalForm = () => {
    setGoalFormOpen(false); setEditingGoalId(null); setGoalForm(emptyGoalForm());
  };
  const closeTargetForm = () => {
    setTargetFormOpen(false); setEditingTargetId(null); setTargetForm(emptyTargetForm());
  };
  const startCreateGoal = () => {
    closeTargetForm(); setGoalForm(emptyGoalForm()); setEditingGoalId(null);
    setGoalFormOpen(true); setSuccess(null);
  };
  const startEditGoal = (goal: ChildGoal) => {
    closeTargetForm();
    setGoalForm({
      goalType: goal.goal_type || "care_plan", domain: goal.domain ?? "", title: goal.title, description: goal.description ?? "",
      status: goal.status as GoalStatus, targetDate: cleanDate(goal.target_date),
      startDate: cleanDate(goal.start_date),
      childVoice: goal.child_voice ?? "", formulationSummary: goal.formulation_summary ?? "",
      protectiveFactors: goal.protective_factors ?? "",
    });
    setEditingGoalId(goal.id); setGoalFormOpen(true); setSuccess(null);
  };
  const startCreateTarget = (goal?: ChildGoal) => {
    closeGoalForm();
    setTargetForm({ ...emptyTargetForm(), goalId: goal ? String(goal.id) : "", domain: goal?.domain ?? "" });
    setEditingTargetId(null); setTargetFormOpen(true); setSuccess(null);
  };
  const startEditTarget = (target: ChildTarget) => {
    closeGoalForm();
    setTargetForm({
      goalId: target.goal_id ? String(target.goal_id) : "", domain: target.domain,
      shortLabel: target.short_label, description: target.description ?? "",
      status: target.status as GoalStatus, supportsNeeded: target.supports_needed ?? "",
      barriers: target.barriers ?? "", interventionStrategy: target.intervention_strategy ?? "",
      reviewDate: cleanDate(target.review_date),
    });
    setEditingTargetId(target.id); setTargetFormOpen(true); setSuccess(null);
  };

  const saveGoal = async (event: FormEvent) => {
    event.preventDefault(); setError(null); setSuccess(null);
    if (!goalForm.domain || !goalForm.title.trim()) {
      setError("Choose a domain and describe the desired outcome."); return;
    }
    const editing = editingGoalId !== null;
    setSaving(true);
    try {
      const response = await apiFetch(
        editing ? `${apiBase}/v1/children/${childId}/goals/${editingGoalId}` : `${apiBase}/v1/children/${childId}/goals`,
        {
          method: editing ? "PATCH" : "POST", headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            goal_type: goalForm.goalType, domain: goalForm.domain, title: goalForm.title.trim(),
            description: goalForm.description.trim(), start_date: editing ? goalForm.startDate || null : today,
            target_date: goalForm.targetDate || null, status: goalForm.status,
            child_voice: goalForm.childVoice.trim(), formulation_summary: goalForm.formulationSummary.trim(),
            protective_factors: goalForm.protectiveFactors.trim(),
          }),
        },
      );
      if (!response.ok) throw new Error((await response.text()) || "The goal could not be saved.");
      closeGoalForm(); await loadData();
      setSuccess(editing ? "Goal updated." : "Goal created. Add targets when the team is ready.");
    } catch (caught: unknown) {
      setError(errorMessage(caught, "The goal could not be saved."));
    } finally { setSaving(false); }
  };

  const saveTarget = async (event: FormEvent) => {
    event.preventDefault(); setError(null); setSuccess(null);
    if (!targetForm.goalId || !targetForm.domain || !targetForm.shortLabel.trim()) {
      setError("Choose a goal and domain, then describe the progress to work towards."); return;
    }
    const editing = editingTargetId !== null;
    setSaving(true);
    try {
      const response = await apiFetch(
        editing ? `${apiBase}/v1/children/${childId}/targets/${editingTargetId}` : `${apiBase}/v1/children/${childId}/targets`,
        {
          method: editing ? "PATCH" : "POST", headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            goal_id: Number(targetForm.goalId), domain: targetForm.domain,
            short_label: targetForm.shortLabel.trim(), description: targetForm.description.trim(),
            status: targetForm.status, supports_needed: targetForm.supportsNeeded.trim(),
            barriers: targetForm.barriers.trim(), intervention_strategy: targetForm.interventionStrategy.trim(),
            review_date: targetForm.reviewDate || null,
          }),
        },
      );
      if (!response.ok) throw new Error((await response.text()) || "The target could not be saved.");
      closeTargetForm(); await loadData();
      setSuccess(editing ? "Target updated." : "Target created and available in Key Working.");
    } catch (caught: unknown) {
      setError(errorMessage(caught, "The target could not be saved."));
    } finally { setSaving(false); }
  };

  const toggleGoal = (goalId: number) => setExpandedGoalIds((current) => {
    const next = new Set(current);
    if (next.has(goalId)) next.delete(goalId); else next.add(goalId);
    return next;
  });

  const renderTarget = (target: ChildTarget) => {
    const evidenceCount = sessionsByTarget.get(target.id)?.length ?? 0;
    const dailyObservationCount = target.daily_observation_count ?? 0;
    const keyworkingSessionCount = target.keyworking_session_count ?? evidenceCount;
    const reviewDate = cleanDate(target.review_date);
    const reviewState = target.status === "active" && reviewDate
      ? reviewDate < today ? "Overdue" : reviewDate === today ? "Review due" : null
      : null;
    const vagueTarget = target.short_label.trim().length < 20 && target.description.trim().length < 40;
    const linkedRiskCount = risks.filter((risk) => risk.target_ids?.includes(target.id)).length;
    return (
      <article key={target.id} className={`care-target-card ${target.status === "active" ? "is-active" : ""}`}>
        <div className="care-target-heading">
          <div><span className="care-target-kicker">Working target</span><h4>{target.short_label}</h4></div>
          <span className={`care-status care-status-${target.status}`}>{statusLabel(target.status)}</span>
        </div>
        <div className="care-meta-row">
          <span>{target.domain}</span>
          <span>{dailyObservationCount} daily staff {dailyObservationCount === 1 ? "observation" : "observations"}</span>
          <span>{keyworkingSessionCount} Key Working {keyworkingSessionCount === 1 ? "session" : "sessions"}</span>
          <span>Linked risks: {linkedRiskCount}</span>
          {target.review_date && <span>Review {displayDate(target.review_date)}</span>}
          {reviewState && <span className="care-review-alert">{reviewState}</span>}
        </div>
        {target.description && <p className="care-card-copy">{target.description}</p>}
        <dl className="care-detail-grid">
          <div><dt>How staff will support this</dt><dd>{target.supports_needed || "Not recorded yet"}</dd></div>
          <div><dt>What might make this difficult</dt><dd>{target.barriers || "Not recorded yet"}</dd></div>
          <div><dt>Agreed approach</dt><dd>{target.intervention_strategy || "Not recorded yet"}</dd></div>
        </dl>
        {(vagueTarget || !target.intervention_strategy) && (
          <div className="care-guidance" aria-label="Target quality guidance">
            {vagueTarget && <span>Consider describing what observable progress would look like.</span>}
            {!target.intervention_strategy && <span>Consider recording how staff will support this target.</span>}
          </div>
        )}
        <div className="care-card-actions">
          <button type="button" className="btn btn-ghost btn-xs" onClick={() => startEditTarget(target)}>Edit target</button>
          {target.status === "active" && <button type="button" className="btn btn-secondary btn-xs" onClick={onOpenKeywork}>Use in Key Working</button>}
          {linkedRiskCount > 0 && <button type="button" className="btn btn-ghost btn-xs" onClick={onOpenRisks}>Open Risk &amp; Safety</button>}
        </div>
      </article>
    );
  };

  return (
    <section className="card care-plan-workspace">
      <div className="card-header care-plan-header">
        <div>
          <span className="care-eyebrow">Care planning · {childInitials}</span>
          <h2>Goals &amp; Targets</h2>
          <p className="card-subtitle">Plan what we are working towards with this young person and track the evidence of progress.</p>
        </div>
        <div className="care-header-actions">
          <button type="button" className="btn btn-ghost" onClick={onBackToDashboard}>← Child Home</button>
          <button type="button" className="btn btn-primary" onClick={startCreateGoal}>Create goal</button>
        </div>
      </div>

      {error && <div className="alert alert-error">{error}</div>}
      {success && <div className="alert alert-success">{success}</div>}

      <div className="care-plan-summary" aria-label="Current plan summary">
        <div><strong>{activeGoalCount}</strong><span>Active goals</span></div>
        <div><strong>{activeTargetCount}</strong><span>Active targets</span></div>
        <div><strong>{reviewDueCount}</strong><span>Targets due for review</span></div>
      </div>

      {goalFormOpen && (
        <form className="care-guided-form" onSubmit={saveGoal}>
          <div className="care-form-heading">
            <div><span className="care-eyebrow">{editingGoalId ? "Update the plan" : "New goal"}</span><h3>{editingGoalId ? "Edit goal" : "What are we working towards?"}</h3></div>
            <button type="button" className="btn btn-ghost btn-xs" onClick={closeGoalForm}>Cancel</button>
          </div>
          <div className="care-form-step"><span className="care-step-number">1</span><div className="form-group">
            <label className="label" htmlFor="goal-domain">Which area of the young person&apos;s life does this relate to?</label>
            <select id="goal-domain" className="input" value={goalForm.domain} onChange={(event) => setGoalForm({ ...goalForm, domain: event.target.value })} required>
              <option value="">Choose a domain…</option>{DOMAIN_OPTIONS.map((option) => <option key={option.value} value={option.value}>{option.value}</option>)}
            </select>
            {goalForm.domain && <p className="field-hint">{DOMAIN_OPTIONS.find((option) => option.value === goalForm.domain)?.help}</p>}
          </div></div>
          <div className="care-form-step"><span className="care-step-number">2</span><div className="form-group">
            <label className="label" htmlFor="goal-title">What are we hoping to help the young person achieve?</label>
            <input id="goal-title" className="input" value={goalForm.title} onChange={(event) => setGoalForm({ ...goalForm, title: event.target.value })} placeholder="A clear, hopeful outcome" required />
          </div></div>
          <div className="care-form-step"><span className="care-step-number">3</span><div className="form-group">
            <label className="label" htmlFor="goal-rationale">Why does this matter now?</label>
            <textarea id="goal-rationale" className="input" rows={3} value={goalForm.description} onChange={(event) => setGoalForm({ ...goalForm, description: event.target.value })} placeholder="Record the current need and professional rationale in concise language." />
          </div></div>
          <div className="form-row-2">
            <div className="form-group"><label className="label" htmlFor="goal-status">Current status</label><select id="goal-status" className="input" value={goalForm.status} onChange={(event) => setGoalForm({ ...goalForm, status: event.target.value as GoalStatus })}>{STATUS_OPTIONS.map((status) => <option key={status} value={status}>{statusLabel(status)}</option>)}</select></div>
            <div className="form-group"><label className="label" htmlFor="goal-target-date">Hoped-for timeframe</label><input id="goal-target-date" type="date" className="input" value={goalForm.targetDate} onChange={(event) => setGoalForm({ ...goalForm, targetDate: event.target.value })} /></div>
          </div>
          <details className="care-context-details"><summary>Include young person voice and planning context</summary><div className="care-context-fields">
            <div className="form-group"><label className="label" htmlFor="goal-child-voice">Young person&apos;s voice</label><textarea id="goal-child-voice" className="input" rows={2} value={goalForm.childVoice} onChange={(event) => setGoalForm({ ...goalForm, childVoice: event.target.value })} /></div>
            <div className="form-group"><label className="label" htmlFor="goal-formulation">Shared understanding</label><textarea id="goal-formulation" className="input" rows={2} value={goalForm.formulationSummary} onChange={(event) => setGoalForm({ ...goalForm, formulationSummary: event.target.value })} /></div>
            <div className="form-group"><label className="label" htmlFor="goal-strengths">Strengths and protective factors</label><textarea id="goal-strengths" className="input" rows={2} value={goalForm.protectiveFactors} onChange={(event) => setGoalForm({ ...goalForm, protectiveFactors: event.target.value })} /></div>
          </div></details>
          <div className="care-form-actions"><button type="button" className="btn btn-ghost" onClick={closeGoalForm}>Cancel</button><button type="submit" className="btn btn-primary" disabled={saving}>{saving ? "Saving…" : editingGoalId ? "Save goal" : "Create goal"}</button></div>
        </form>
      )}

      {targetFormOpen && (
        <form className="care-guided-form care-target-form" onSubmit={saveTarget}>
          <div className="care-form-heading"><div><span className="care-eyebrow">{editingTargetId ? "Update the plan" : "New working target"}</span><h3>{editingTargetId ? "Edit target" : "Turn the goal into practical progress"}</h3></div><button type="button" className="btn btn-ghost btn-xs" onClick={closeTargetForm}>Cancel</button></div>
          <div className="form-row-2">
            <div className="form-group"><label className="label" htmlFor="target-goal">Which goal does this support?</label><select id="target-goal" className="input" value={targetForm.goalId} onChange={(event) => { const goal = goals.find((item) => item.id === Number(event.target.value)); setTargetForm({ ...targetForm, goalId: event.target.value, domain: goal?.domain ?? targetForm.domain }); }} required><option value="">Choose a goal…</option>{goals.map((goal) => <option key={goal.id} value={goal.id}>{goal.title}</option>)}</select></div>
            <div className="form-group"><label className="label" htmlFor="target-domain">Domain</label><select id="target-domain" className="input" value={targetForm.domain} onChange={(event) => setTargetForm({ ...targetForm, domain: event.target.value })} required><option value="">Choose a domain…</option>{DOMAIN_OPTIONS.map((option) => <option key={option.value} value={option.value}>{option.value}</option>)}</select></div>
          </div>
          <div className="form-group"><label className="label" htmlFor="target-progress">What progress are we hoping to see?</label><input id="target-progress" className="input" value={targetForm.shortLabel} onChange={(event) => setTargetForm({ ...targetForm, shortLabel: event.target.value })} placeholder="Describe observable, meaningful progress" required />{targetForm.shortLabel.trim().length > 0 && targetForm.shortLabel.trim().length < 20 && <p className="care-inline-guidance">Consider describing what observable progress would look like.</p>}</div>
          <div className="form-group"><label className="label" htmlFor="target-description">What will this look like in everyday life?</label><textarea id="target-description" className="input" rows={3} value={targetForm.description} onChange={(event) => setTargetForm({ ...targetForm, description: event.target.value })} /></div>
          <div className="care-support-grid">
            <div className="form-group"><label className="label" htmlFor="target-support">How will staff support this?</label><textarea id="target-support" className="input" rows={3} value={targetForm.supportsNeeded} onChange={(event) => setTargetForm({ ...targetForm, supportsNeeded: event.target.value })} /></div>
            <div className="form-group"><label className="label" htmlFor="target-barriers">What might make this difficult?</label><textarea id="target-barriers" className="input" rows={3} value={targetForm.barriers} onChange={(event) => setTargetForm({ ...targetForm, barriers: event.target.value })} /></div>
            <div className="form-group"><label className="label" htmlFor="target-strategy">What approach will staff use?</label><textarea id="target-strategy" className="input" rows={3} value={targetForm.interventionStrategy} onChange={(event) => setTargetForm({ ...targetForm, interventionStrategy: event.target.value })} />{!targetForm.interventionStrategy.trim() && <p className="care-inline-guidance">Consider recording how staff will support this target.</p>}</div>
          </div>
          <div className="form-row-2">
            <div className="form-group"><label className="label" htmlFor="target-review-date">Review date</label><input id="target-review-date" type="date" className="input" value={targetForm.reviewDate} onChange={(event) => setTargetForm({ ...targetForm, reviewDate: event.target.value })} /></div>
            <div className="form-group"><label className="label" htmlFor="target-status">Current status</label><select id="target-status" className="input" value={targetForm.status} onChange={(event) => setTargetForm({ ...targetForm, status: event.target.value as GoalStatus })}>{STATUS_OPTIONS.map((status) => <option key={status} value={status}>{statusLabel(status)}</option>)}</select></div>
          </div>
          <div className="care-form-actions"><button type="button" className="btn btn-ghost" onClick={closeTargetForm}>Cancel</button><button type="submit" className="btn btn-primary" disabled={saving}>{saving ? "Saving…" : editingTargetId ? "Save target" : "Create target"}</button></div>
        </form>
      )}

      <div className="care-goals-section">
        <div className="care-section-heading"><div><h3>Current care plan</h3><p className="field-hint">Active goals appear first. Open a goal to review its targets and planning context.</p></div></div>
        {loading && <div className="care-empty-state">Loading goals and targets…</div>}
        {!loading && goals.length === 0 && <div className="care-empty-state"><strong>No goals recorded yet</strong><p>Start with one meaningful outcome that the young person and team can work towards.</p><button type="button" className="btn btn-primary" onClick={startCreateGoal}>Create the first goal</button></div>}
        {!loading && orderedGoals.map((goal) => {
          const goalTargets = targetsByGoal.get(goal.id) ?? [];
          const activeTargets = goalTargets.filter((target) => target.status === "active");
          const recentCutoff = Date.now() - 90 * 24 * 60 * 60 * 1000;
          const recentEvidence = goalTargets.reduce((total, target) => total + (sessionsByTarget.get(target.id) ?? []).filter((session) => new Date(session.session_at).getTime() >= recentCutoff).length, 0);
          const linkedRiskCount = risks.filter((risk) => risk.goal_ids?.includes(goal.id)).length;
          const expanded = expandedGoalIds.has(goal.id);
          return <article key={goal.id} className={`care-goal-card ${goal.status === "active" ? "is-active" : ""}`}>
            <div className="care-goal-main">
              <div className="care-goal-title-row"><div><span className="care-domain-label">{goal.domain || "Domain not recorded"}</span><h3>{goal.title}</h3></div><span className={`care-status care-status-${goal.status}`}>{statusLabel(goal.status)}</span></div>
              {goal.description && <p className="care-card-copy">{goal.description}</p>}
              <div className="care-meta-row"><span><strong>{activeTargets.length}</strong> active {activeTargets.length === 1 ? "target" : "targets"}</span><span><strong>{goalTargets.length}</strong> total</span><span><strong>{recentEvidence}</strong> Key Working {recentEvidence === 1 ? "record" : "records"} in 90 days</span><span>Linked risks: {linkedRiskCount}</span>{goal.target_date && <span>Timeframe {displayDate(goal.target_date)}</span>}</div>
              <div className="care-card-actions"><button type="button" className="btn btn-secondary btn-xs" aria-expanded={expanded} onClick={() => toggleGoal(goal.id)}>{expanded ? "Hide details" : "View details & targets"}</button><button type="button" className="btn btn-ghost btn-xs" onClick={() => startEditGoal(goal)}>Edit goal</button><button type="button" className="btn btn-ghost btn-xs" onClick={() => startCreateTarget(goal)}>Add target</button>{linkedRiskCount > 0 && <button type="button" className="btn btn-ghost btn-xs" onClick={onOpenRisks}>Open Risk &amp; Safety</button>}</div>
            </div>
            {expanded && <div className="care-goal-details">
              {(goal.child_voice || goal.formulation_summary || goal.protective_factors) && <dl className="care-detail-grid care-goal-context">{goal.child_voice && <div><dt>Young person&apos;s voice</dt><dd>{goal.child_voice}</dd></div>}{goal.formulation_summary && <div><dt>Shared understanding</dt><dd>{goal.formulation_summary}</dd></div>}{goal.protective_factors && <div><dt>Strengths and protective factors</dt><dd>{goal.protective_factors}</dd></div>}</dl>}
              <div className="care-targets-heading"><div><h4>Targets supporting this goal</h4><p className="field-hint">Active targets are available in the Key Working target selector.</p></div><button type="button" className="btn btn-primary btn-xs" onClick={() => startCreateTarget(goal)}>Add target</button></div>
              {goalTargets.length === 0 ? <div className="care-empty-inline">No targets yet. Add a practical next step when the team is ready.</div> : <div className="care-target-list">{goalTargets.slice().sort((left, right) => left.status === "active" && right.status !== "active" ? -1 : left.status !== "active" && right.status === "active" ? 1 : 0).map(renderTarget)}</div>}
            </div>}
          </article>;
        })}
      </div>

      {!loading && unlinkedTargets.length > 0 && <details className="care-unlinked"><summary>{unlinkedTargets.length} existing unlinked {unlinkedTargets.length === 1 ? "target" : "targets"}</summary><p className="field-hint">These older targets remain available. Edit one to connect it to a goal.</p><div className="care-target-list">{unlinkedTargets.map(renderTarget)}</div></details>}
    </section>
  );
};

export default GoalsTargetsTab;
