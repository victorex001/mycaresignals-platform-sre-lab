import { useEffect, useState } from "react";
import { apiFetch } from "./api";
import type {
  Child,
  MFHInformationStatus,
  MFHPlanContent,
  MFHReuseCandidate,
  MFHWorkspace,
  MissingFromHomePlan,
} from "./types";

type Props = {
  apiBase: string;
  child: Child;
  onBackToDashboard: () => void;
  onOpenRisks: () => void;
  onOpenKeywork: () => void;
};

const steps = [
  "Current situation", "Missing history", "People & places", "Triggers / warning signs",
  "If they go missing", "Notifications", "Return & follow-up", "Review & approve",
];

const informationStatuses: Array<{ value: MFHInformationStatus; label: string }> = [
  { value: "known_confirmed", label: "Known / confirmed" },
  { value: "partly_known", label: "Partly known" },
  { value: "awaiting_confirmation", label: "Awaiting confirmation" },
  { value: "not_known_yet", label: "Not known yet" },
];

const blankContent = (): MFHPlanContent => ({
  current_situation: { supervision_arrangement: "", free_time_arrangement: "", confirmed_legal_context: "", current_risk_context: "" },
  missing_history: { history_type: "", patterns: "", circumstances: "", what_helped_return: "", source: "", information_status: "" },
  people_places: [],
  triggers: [],
  operational_response: { immediate_checks: "", contact_attempts: "", local_search_expectations: "", police_policy_wording: "", information_for_police: "", safeguarding_considerations: "" },
  notifications: [],
  return_follow_up: { immediate_welfare_response: "", medical_safeguarding_check: "", calm_welcome_recovery_approach: "", return_conversation: "", independent_return_interview: "", professional_notifications: "", key_working_follow_up: "", consider_risk_review: false, consider_care_plan_review: false },
});

const displayDate = (value?: string) => {
  if (!value) return "Not recorded";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleDateString(undefined, { dateStyle: "medium" });
};

const statusLabel = (value: string) => informationStatuses.find((item) => item.value === value)?.label ?? value.replaceAll("_", " ");

function SourceChoices({
  candidates,
  decisions,
  changedKeys,
  onDecision,
}: {
  candidates: MFHReuseCandidate[];
  decisions: Record<string, boolean | undefined>;
  changedKeys: Set<string>;
  onDecision: (key: string, included: boolean) => void;
}) {
  if (candidates.length === 0) return <p className="current-care-empty">No existing information is available for this section.</p>;
  return (
    <div className="mfh-source-list">
      {candidates.map((candidate) => (
        <article key={candidate.candidate_key} className={`mfh-source-card ${decisions[candidate.candidate_key] === true ? "is-included" : ""}`}>
          <div><p className="current-care-eyebrow">{candidate.label}</p><p>{candidate.value}</p>{changedKeys.has(candidate.candidate_key) && <p className="field-hint"><strong>Source changed — make a fresh inclusion decision.</strong></p>}</div>
          <dl><div><dt>Source</dt><dd>{candidate.source_label}</dd></div><div><dt>Status</dt><dd>{statusLabel(candidate.information_status)}</dd></div></dl>
          <div className="mfh-source-actions">
            <button type="button" className={decisions[candidate.candidate_key] === true ? "btn btn-primary btn-xs" : "btn btn-secondary btn-xs"} onClick={() => onDecision(candidate.candidate_key, true)}>Use in this plan</button>
            <button type="button" className={decisions[candidate.candidate_key] === false ? "btn btn-primary btn-xs" : "btn btn-ghost btn-xs"} onClick={() => onDecision(candidate.candidate_key, false)}>Not relevant</button>
          </div>
        </article>
      ))}
    </div>
  );
}

function TextArea({ label, value, onChange, required, hint, rows = 3 }: { label: string; value: string; onChange: (value: string) => void; required?: boolean; hint?: string; rows?: number }) {
  return <div className="form-group"><label className="label">{label}{required && <span className="required"> *</span>}</label><textarea className="input" rows={rows} value={value} onChange={(event) => onChange(event.target.value)} />{hint && <p className="field-hint">{hint}</p>}</div>;
}

function PlanDocument({ plan, onOpenRisks, onOpenKeywork }: { plan: MissingFromHomePlan; onOpenRisks: () => void; onOpenKeywork: () => void }) {
  const c = plan.content;
  const included = plan.source_snapshots.filter((item) => item.included);
  const section = (title: string, rows: Array<[string, string | boolean | undefined]>) => (
    <section className="mfh-document-section"><h3>{title}</h3><dl>{rows.map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{typeof value === "boolean" ? (value ? "Yes — human review may be appropriate" : "No prompt recorded") : value || "Not recorded"}</dd></div>)}</dl></section>
  );
  return <div className="mfh-document">
    {plan.sources_changed && <div className="alert alert-warning"><strong>Information used by this Missing From Home Plan has changed.</strong><p>Review the plan before relying on the changed source information.</p></div>}
    <div className="mfh-document-meta"><span>Version {plan.version}</span><span>{statusLabel(plan.status)}</span><span>Review {displayDate(plan.review_date)}</span></div>
    {section("Current situation", [["Supervision arrangement", c.current_situation.supervision_arrangement], ["Free-time / community arrangement", c.current_situation.free_time_arrangement], ["Confirmed legal context", c.current_situation.confirmed_legal_context], ["Current risk context", c.current_situation.current_risk_context]])}
    {included.length > 0 && <section className="mfh-document-section"><h3>Reused information and provenance</h3>{included.map((item) => <div key={item.candidate_key} className="mfh-document-source"><strong>{item.label}</strong><p>{item.value}</p><small>Source: {item.source_label} · {statusLabel(item.information_status)}{item.changed ? " · Source has changed" : ""}</small></div>)}</section>}
    {section("Missing history", [["History", statusLabel(c.missing_history.history_type)], ["Patterns", c.missing_history.patterns], ["Circumstances", c.missing_history.circumstances], ["What previously helped", c.missing_history.what_helped_return], ["Source", c.missing_history.source], ["Information status", statusLabel(c.missing_history.information_status)]])}
    <section className="mfh-document-section"><h3>People & places</h3>{c.people_places.length === 0 ? <p>None recorded.</p> : c.people_places.map((item, index) => <div key={`${item.kind}-${index}`} className="mfh-document-source"><strong>{item.name_or_label}</strong><p>{item.details}</p><p>{item.contact_or_route}</p><small>{statusLabel(item.kind)} · Source: {item.source} · {statusLabel(item.information_status)}</small></div>)}</section>
    <section className="mfh-document-section"><h3>Triggers / warning signs</h3>{c.triggers.length === 0 ? <p>None recorded.</p> : c.triggers.map((item, index) => <div key={index} className="mfh-document-source"><p>{item.description}</p><small>Source: {item.source} · {statusLabel(item.information_status)}</small></div>)}</section>
    {section("If they go missing", [["Immediate checks", c.operational_response.immediate_checks], ["Attempts to contact", c.operational_response.contact_attempts], ["Local / home search", c.operational_response.local_search_expectations], ["Provider / home Police procedure", c.operational_response.police_policy_wording], ["Information for Police", c.operational_response.information_for_police], ["Child-specific safeguarding considerations", c.operational_response.safeguarding_considerations]])}
    <section className="mfh-document-section"><h3>Notifications</h3>{c.notifications.length === 0 ? <p>None recorded.</p> : c.notifications.map((item, index) => <div key={index} className="mfh-document-source"><strong>{item.role}</strong><p>{item.contact_name} · {item.contact_details}</p><p>{item.when_to_notify}</p><small>Source: {item.source || "Plan author"}</small></div>)}</section>
    {section("Return & follow-up", [["Immediate welfare response", c.return_follow_up.immediate_welfare_response], ["Medical / safeguarding check", c.return_follow_up.medical_safeguarding_check], ["Calm welcome / recovery", c.return_follow_up.calm_welcome_recovery_approach], ["Return conversation", c.return_follow_up.return_conversation], ["Independent return interview", c.return_follow_up.independent_return_interview], ["Professional notifications", c.return_follow_up.professional_notifications], ["Key Working follow-up", c.return_follow_up.key_working_follow_up], ["Risk & Safety review prompt", c.return_follow_up.consider_risk_review], ["Care Plan review prompt", c.return_follow_up.consider_care_plan_review]])}
    <div className="mfh-document-actions"><button type="button" className="btn btn-secondary" onClick={onOpenRisks}>Open linked Risk &amp; Safety</button><button type="button" className="btn btn-secondary" onClick={onOpenKeywork}>Open Key Working</button></div>
    <footer><p>Created by {plan.created_by_name} · {displayDate(plan.created_at)}</p>{plan.submitted_by_name && <p>Submitted by {plan.submitted_by_name} · {displayDate(plan.submitted_at)}</p>}{plan.approved_by_name && <p>Approved by {plan.approved_by_name} · {displayDate(plan.approved_at)}</p>}</footer>
  </div>;
}

export default function MissingFromHomePlanTab({ apiBase, child, onBackToDashboard, onOpenRisks, onOpenKeywork }: Props) {
  const [workspace, setWorkspace] = useState<MFHWorkspace | null>(null);
  const [selectedPlan, setSelectedPlan] = useState<MissingFromHomePlan | null>(null);
  const [content, setContent] = useState<MFHPlanContent>(blankContent);
  const [sourceDecisions, setSourceDecisions] = useState<Record<string, boolean | undefined>>({});
  const [reviewDate, setReviewDate] = useState("");
  const [step, setStep] = useState(0);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  const load = async () => {
    setLoading(true); setError(null);
    try {
      const response = await apiFetch(`${apiBase}/v1/children/${child.id}/missing-from-home-plans`);
      if (!response.ok) throw new Error((await response.text()) || "Missing From Home workspace could not be loaded.");
      const data = await response.json() as MFHWorkspace;
      setWorkspace(data); setSelectedPlan(data.plan);
      if (data.plan?.status === "draft") {
        setContent(data.plan.content ?? blankContent()); setReviewDate(data.plan.review_date?.slice(0, 10) ?? "");
        setSourceDecisions(Object.fromEntries(data.plan.source_snapshots.map((item) => [item.candidate_key, item.changed ? undefined : item.included])));
      }
    } catch (caught) { setError(caught instanceof Error ? caught.message : "Missing From Home workspace could not be loaded."); }
    finally { setLoading(false); }
  };

  // Loading is intentionally keyed to the selected child; `load` captures the
  // current draft state setters and API base for that rendered workspace.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => { void load(); }, [child.id]);

  const candidatesFor = (category: string) => workspace?.reuse_candidates.filter((item) => item.category === category) ?? [];
  const activeRisk = workspace?.reuse_candidates.find((item) => item.source_type === "risk_safety");
  const changedSourceKeys = new Set(selectedPlan?.source_snapshots.filter((item) => item.changed).map((item) => item.candidate_key) ?? []);
  const update = <K extends keyof MFHPlanContent>(key: K, value: MFHPlanContent[K]) => setContent((current) => ({ ...current, [key]: value }));

  const saveDraft = async () => {
    if (!selectedPlan || selectedPlan.status !== "draft") return false;
    setSaving(true); setError(null); setSuccess(null);
    try {
      const response = await apiFetch(`${apiBase}/v1/children/${child.id}/missing-from-home-plans/${selectedPlan.id}`, { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ content, source_decisions: Object.entries(sourceDecisions).filter(([, value]) => value !== undefined).map(([candidate_key, included]) => ({ candidate_key, included })), review_date: reviewDate || null }) });
      if (!response.ok) throw new Error((await response.text()) || "Draft could not be saved.");
      setSuccess("Draft saved. You can safely return to it later."); return true;
    } catch (caught) { setError(caught instanceof Error ? caught.message : "Draft could not be saved."); return false; }
    finally { setSaving(false); }
  };

  const action = async (name: "submit" | "approve" | "revise" | "archive") => {
    if (!selectedPlan) return;
    setSaving(true); setError(null); setSuccess(null);
    try {
      if (name === "submit" && !(await saveDraft())) return;
      const response = await apiFetch(`${apiBase}/v1/children/${child.id}/missing-from-home-plans/${selectedPlan.id}/${name}`, { method: "POST" });
      if (!response.ok) throw new Error((await response.text()) || `Plan could not be ${name}d.`);
      await load(); setStep(0); setSuccess(name === "approve" ? "Missing From Home Plan approved." : name === "revise" ? "Successor draft created." : `Plan ${name}d.`);
    } catch (caught) { setError(caught instanceof Error ? caught.message : "Plan action failed."); }
    finally { setSaving(false); }
  };

  const startDraft = async () => {
    setSaving(true); setError(null);
    try { const response = await apiFetch(`${apiBase}/v1/children/${child.id}/missing-from-home-plans`, { method: "POST" }); if (!response.ok) throw new Error(await response.text()); await load(); }
    catch (caught) { setError(caught instanceof Error ? caught.message : "Draft could not be started."); }
    finally { setSaving(false); }
  };

  const openVersion = async (plan: MissingFromHomePlan) => {
    const response = await apiFetch(`${apiBase}/v1/children/${child.id}/missing-from-home-plans/${plan.id}`);
    if (response.ok) setSelectedPlan(((await response.json()) as { plan: MissingFromHomePlan }).plan);
  };

  const renderStep = () => {
    const current = content.current_situation, history = content.missing_history, operational = content.operational_response, follow = content.return_follow_up;
    if (step === 0) return <div className="mfh-step"><h3>Current situation</h3><p>Confirm what applies now. Existing information is offered with its source and is never included automatically.</p><SourceChoices candidates={candidatesFor("current_situation")} decisions={sourceDecisions} changedKeys={changedSourceKeys} onDecision={(key, included) => setSourceDecisions((value) => ({ ...value, [key]: included }))} />{activeRisk ? <button type="button" className="text-button" onClick={onOpenRisks}>Open active Missing From Home risk in Risk &amp; Safety →</button> : <p className="field-hint">No active Missing From Home risk is linked. This plan will not create one.</p>}<TextArea label="Current supervision arrangement" required value={current.supervision_arrangement} onChange={(value) => update("current_situation", { ...current, supervision_arrangement: value })} /><TextArea label="Current free-time / community arrangement" value={current.free_time_arrangement} onChange={(value) => update("current_situation", { ...current, free_time_arrangement: value })} /><TextArea label="Relevant legal context — manually confirmed" value={current.confirmed_legal_context} onChange={(value) => update("current_situation", { ...current, confirmed_legal_context: value })} hint="Only record current information confirmed by an appropriate professional source." /><TextArea label="Current MFH risk context" value={current.current_risk_context} onChange={(value) => update("current_situation", { ...current, current_risk_context: value })} /></div>;
    if (step === 1) return <div className="mfh-step"><h3>Missing history</h3><fieldset className="mfh-choice-list"><legend>Missing history</legend>{[["none_known", "No known previous missing episodes"], ["previous_episodes", "Previous episodes"], ["frequent_repeated", "Frequent / repeated episodes"], ["awaiting_information", "Awaiting information"]].map(([value, label]) => <label key={value}><input type="radio" name="mfh-history" checked={history.history_type === value} onChange={() => update("missing_history", { ...history, history_type: value })} /><span>{label}</span></label>)}</fieldset><TextArea label="Patterns" value={history.patterns} onChange={(value) => update("missing_history", { ...history, patterns: value })} /><TextArea label="Circumstances" value={history.circumstances} onChange={(value) => update("missing_history", { ...history, circumstances: value })} /><TextArea label="What has previously helped them return safely?" value={history.what_helped_return} onChange={(value) => update("missing_history", { ...history, what_helped_return: value })} /><div className="form-row-2"><div className="form-group"><label className="label">Source / provenance</label><input className="input" value={history.source} onChange={(event) => update("missing_history", { ...history, source: event.target.value })} /></div><div className="form-group"><label className="label">Information status</label><select className="input" value={history.information_status} onChange={(event) => update("missing_history", { ...history, information_status: event.target.value as MFHInformationStatus })}><option value="">Choose status…</option>{informationStatuses.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}</select></div></div></div>;
    if (step === 2) return <div className="mfh-step"><h3>People &amp; places</h3><p>Keep child-specific contacts, locations and travel information here—not on Child Home.</p>{content.people_places.map((item, index) => <div className="mfh-repeat-card" key={index}><div className="form-row-2"><select className="input" value={item.kind} onChange={(event) => { const values = [...content.people_places]; values[index] = { ...item, kind: event.target.value }; update("people_places", values); }}>{[["family_contact", "Likely family contact"], ["friend_peer", "Likely friend / peer"], ["previous_area", "Previous area"], ["known_location", "Known location"], ["travel_route", "Travel route"], ["other_place", "Other place"]].map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select><input className="input" placeholder="Name or safe label" value={item.name_or_label} onChange={(event) => { const values = [...content.people_places]; values[index] = { ...item, name_or_label: event.target.value }; update("people_places", values); }} /></div><TextArea label="Details" value={item.details} onChange={(value) => { const values = [...content.people_places]; values[index] = { ...item, details: value }; update("people_places", values); }} /><TextArea label="Contact / route information" value={item.contact_or_route} onChange={(value) => { const values = [...content.people_places]; values[index] = { ...item, contact_or_route: value }; update("people_places", values); }} rows={2} /><div className="form-row-2"><input className="input" placeholder="Source" value={item.source} onChange={(event) => { const values = [...content.people_places]; values[index] = { ...item, source: event.target.value }; update("people_places", values); }} /><select className="input" value={item.information_status} onChange={(event) => { const values = [...content.people_places]; values[index] = { ...item, information_status: event.target.value as MFHInformationStatus }; update("people_places", values); }}>{informationStatuses.map((status) => <option key={status.value} value={status.value}>{status.label}</option>)}</select></div><button type="button" className="btn btn-ghost btn-xs" onClick={() => update("people_places", content.people_places.filter((_, itemIndex) => itemIndex !== index))}>Remove</button></div>)}<button type="button" className="btn btn-secondary" onClick={() => update("people_places", [...content.people_places, { kind: "family_contact", name_or_label: "", details: "", contact_or_route: "", source: "", information_status: "awaiting_confirmation" }])}>Add person or place</button></div>;
    if (step === 3) return <div className="mfh-step"><h3>Triggers / warning signs</h3><p>Choose whether existing information is relevant. The system does not infer triggers.</p><SourceChoices candidates={candidatesFor("triggers")} decisions={sourceDecisions} changedKeys={changedSourceKeys} onDecision={(key, included) => setSourceDecisions((value) => ({ ...value, [key]: included }))} />{content.triggers.map((item, index) => <div className="mfh-repeat-card" key={index}><TextArea label="MFH-specific trigger / warning sign" value={item.description} onChange={(value) => { const values = [...content.triggers]; values[index] = { ...item, description: value }; update("triggers", values); }} /><div className="form-row-2"><input className="input" placeholder="Source" value={item.source} onChange={(event) => { const values = [...content.triggers]; values[index] = { ...item, source: event.target.value }; update("triggers", values); }} /><select className="input" value={item.information_status} onChange={(event) => { const values = [...content.triggers]; values[index] = { ...item, information_status: event.target.value as MFHInformationStatus }; update("triggers", values); }}>{informationStatuses.map((status) => <option key={status.value} value={status.value}>{status.label}</option>)}</select></div><button type="button" className="btn btn-ghost btn-xs" onClick={() => update("triggers", content.triggers.filter((_, itemIndex) => itemIndex !== index))}>Remove</button></div>)}<button type="button" className="btn btn-secondary" onClick={() => update("triggers", [...content.triggers, { description: "", source: "", information_status: "awaiting_confirmation" }])}>Add MFH-specific trigger</button></div>;
    if (step === 4) return <div className="mfh-step"><h3>If they go missing</h3><p>Record provider policy and child-specific actions. MyCareSignals does not decide Police risk classification.</p><TextArea label="Immediate checks" required value={operational.immediate_checks} onChange={(value) => update("operational_response", { ...operational, immediate_checks: value })} /><TextArea label="Attempts to contact" required value={operational.contact_attempts} onChange={(value) => update("operational_response", { ...operational, contact_attempts: value })} /><TextArea label="Local / home search expectations" required value={operational.local_search_expectations} onChange={(value) => update("operational_response", { ...operational, local_search_expectations: value })} /><TextArea label="Provider / home Police reporting procedure" required value={operational.police_policy_wording} onChange={(value) => update("operational_response", { ...operational, police_policy_wording: value })} hint="Do not invent a universal threshold. Record the applicable provider procedure and professional decision route." /><TextArea label="Child-specific information staff should give Police" required value={operational.information_for_police} onChange={(value) => update("operational_response", { ...operational, information_for_police: value })} /><TextArea label="Child-specific safeguarding considerations" value={operational.safeguarding_considerations} onChange={(value) => update("operational_response", { ...operational, safeguarding_considerations: value })} /></div>;
    if (step === 5) return <div className="mfh-step"><h3>Notifications</h3><SourceChoices candidates={candidatesFor("notifications")} decisions={sourceDecisions} changedKeys={changedSourceKeys} onDecision={(key, included) => setSourceDecisions((value) => ({ ...value, [key]: included }))} />{content.notifications.map((item, index) => <div className="mfh-repeat-card" key={index}><div className="form-row-2"><input className="input" placeholder="Role, e.g. Social Worker" value={item.role} onChange={(event) => { const values = [...content.notifications]; values[index] = { ...item, role: event.target.value }; update("notifications", values); }} /><input className="input" placeholder="Contact name" value={item.contact_name} onChange={(event) => { const values = [...content.notifications]; values[index] = { ...item, contact_name: event.target.value }; update("notifications", values); }} /></div><TextArea label="Contact details" value={item.contact_details} onChange={(value) => { const values = [...content.notifications]; values[index] = { ...item, contact_details: value }; update("notifications", values); }} rows={2} /><TextArea label="When / how to notify" value={item.when_to_notify} onChange={(value) => { const values = [...content.notifications]; values[index] = { ...item, when_to_notify: value }; update("notifications", values); }} rows={2} /><input className="input" placeholder="Source" value={item.source} onChange={(event) => { const values = [...content.notifications]; values[index] = { ...item, source: event.target.value }; update("notifications", values); }} /><button type="button" className="btn btn-ghost btn-xs" onClick={() => update("notifications", content.notifications.filter((_, itemIndex) => itemIndex !== index))}>Remove</button></div>)}<button type="button" className="btn btn-secondary" onClick={() => update("notifications", [...content.notifications, { role: "", contact_name: "", contact_details: "", when_to_notify: "", source: "" }])}>Add notification contact</button></div>;
    if (step === 6) return <div className="mfh-step"><h3>Return &amp; follow-up</h3><TextArea label="Immediate welfare response" required value={follow.immediate_welfare_response} onChange={(value) => update("return_follow_up", { ...follow, immediate_welfare_response: value })} /><TextArea label="Medical / safeguarding check where applicable" value={follow.medical_safeguarding_check} onChange={(value) => update("return_follow_up", { ...follow, medical_safeguarding_check: value })} /><TextArea label="Calm welcome / recovery approach" required value={follow.calm_welcome_recovery_approach} onChange={(value) => update("return_follow_up", { ...follow, calm_welcome_recovery_approach: value })} /><TextArea label="Return conversation" value={follow.return_conversation} onChange={(value) => update("return_follow_up", { ...follow, return_conversation: value })} /><TextArea label="Independent return interview where applicable" value={follow.independent_return_interview} onChange={(value) => update("return_follow_up", { ...follow, independent_return_interview: value })} /><TextArea label="Professional notifications" required value={follow.professional_notifications} onChange={(value) => update("return_follow_up", { ...follow, professional_notifications: value })} /><TextArea label="Key Working follow-up" value={follow.key_working_follow_up} onChange={(value) => update("return_follow_up", { ...follow, key_working_follow_up: value })} /><div className="mfh-review-prompts"><label><input type="checkbox" checked={follow.consider_risk_review} onChange={(event) => update("return_follow_up", { ...follow, consider_risk_review: event.target.checked })} /><span>Risk &amp; Safety review may be appropriate.</span></label><label><input type="checkbox" checked={follow.consider_care_plan_review} onChange={(event) => update("return_follow_up", { ...follow, consider_care_plan_review: event.target.checked })} /><span>Care Plan review may be appropriate.</span></label><p className="field-hint">These are human prompts only. No Risk or Care Plan record is changed automatically.</p></div></div>;
    return <div className="mfh-step"><h3>Review &amp; submit</h3><p>Check the complete professional plan before submitting it for authoritative manager approval.</p><div className="form-group"><label className="label">Review date <span className="required">*</span></label><input className="input" type="date" value={reviewDate} onChange={(event) => setReviewDate(event.target.value)} /></div>{selectedPlan && <PlanDocument plan={{ ...selectedPlan, content, review_date: reviewDate || undefined, source_snapshots: workspace?.reuse_candidates.filter((candidate) => sourceDecisions[candidate.candidate_key] !== undefined).map((candidate) => ({ ...candidate, snapshot_sha256: "", included: sourceDecisions[candidate.candidate_key] === true, changed: false })) ?? [] }} onOpenRisks={onOpenRisks} onOpenKeywork={onOpenKeywork} />}</div>;
  };

  if (loading) return <p className="field-hint">Loading Missing From Home planning…</p>;
  const plan = selectedPlan;
  const editable = plan?.status === "draft" && workspace?.can_edit;
  return <section className="card mfh-page"><header className="card-header mfh-header"><div><span className="care-eyebrow">Professional operational plan · {child.initials}</span><h2>Missing From Home</h2><p className="card-subtitle">Enter once → reuse safely → human review → approved plan.</p></div><button type="button" className="btn btn-ghost" onClick={onBackToDashboard}>← Child Home</button></header>{error && <div className="alert alert-error">{error}</div>}{success && <div className="alert alert-success">{success}</div>}
    {!plan && <div className="mfh-empty"><h3>No Missing From Home Plan yet</h3><p>Start a guided child-specific operational plan. No Risk, Care Plan or Goal will be created automatically.</p>{workspace?.can_edit && <button type="button" className="btn btn-primary" disabled={saving} onClick={() => void startDraft()}>Start Missing From Home Plan</button>}</div>}
    {editable && <>{plan.sources_changed && <div className="alert alert-warning"><strong>Information used by this Missing From Home Plan has changed.</strong><p>Review the affected source and explicitly include the current information or mark it not relevant.</p></div>}<nav className="mfh-stepper" aria-label="Missing From Home plan steps">{steps.map((label, index) => <button key={label} type="button" className={step === index ? "is-active" : index < step ? "is-complete" : ""} onClick={() => setStep(index)}><span>{index + 1}</span>{label}</button>)}</nav>{renderStep()}<div className="mfh-step-actions"><button type="button" className="btn btn-ghost" disabled={step === 0 || saving} onClick={() => setStep((value) => Math.max(0, value - 1))}>← Back</button><button type="button" className="btn btn-secondary" disabled={saving} onClick={() => void saveDraft()}>{saving ? "Saving…" : "Save draft"}</button>{step < steps.length - 1 ? <button type="button" className="btn btn-primary" disabled={saving} onClick={() => void saveDraft().then((saved) => saved && setStep((value) => Math.min(steps.length - 1, value + 1)))}>Save &amp; continue →</button> : <button type="button" className="btn btn-primary" disabled={saving} onClick={() => void action("submit")}>Submit for manager approval</button>}</div></>}
    {plan && plan.status !== "draft" && <div className="mfh-read-view"><div className="mfh-lifecycle-banner"><div><span className="status-badge">{statusLabel(plan.status)}</span><h3>Missing From Home Plan · Version {plan.version}</h3></div><div className="mfh-lifecycle-actions">{plan.status === "submitted" && workspace?.can_approve && <button type="button" className="btn btn-primary" disabled={saving} onClick={() => void action("approve")}>Approve Plan · manager</button>}{plan.status === "approved" && workspace?.can_edit && <button type="button" className="btn btn-secondary" disabled={saving} onClick={() => void action("revise")}>Review plan → create Version {plan.version + 1}</button>}{plan.status === "approved" && workspace?.can_approve && <button type="button" className="btn btn-ghost" disabled={saving} onClick={() => void action("archive")}>Archive plan</button>}</div></div><PlanDocument plan={plan} onOpenRisks={onOpenRisks} onOpenKeywork={onOpenKeywork} /></div>}
    {(workspace?.history.length ?? 0) > 0 && <section className="mfh-history"><h3>Plan versions</h3><div>{workspace!.history.map((item) => <button key={item.id} type="button" className={plan?.id === item.id ? "is-selected" : ""} onClick={() => void openVersion(item)}><strong>Version {item.version}</strong><span>{statusLabel(item.status)} · {displayDate(item.updated_at)}</span></button>)}</div></section>}
  </section>;
}
