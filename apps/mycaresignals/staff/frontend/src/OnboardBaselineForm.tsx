import { useEffect, useMemo, useState } from "react";
import type { ReactNode } from "react";
import { apiFetch } from "./api";
import {
  CANONICAL_DOMAINS,
  FRAMEWORK_SOURCES,
  INFORMATION_STATUSES,
  frameworkSourceLabel,
  informationStatusLabel,
} from "./framework";
import type { CanonicalDomain, FrameworkSource, InformationStatus } from "./framework";
import type { Child } from "./types";

type Pathway = "new_placement" | "existing_young_person";
type Step = 1 | 2 | 3 | 4 | 5 | 6;
type FrameworkStatus = "" | "draft" | "submitted" | "approved";

type DomainState = {
  currentUnderstanding: string;
  strengthsProtectiveFactors: string;
  supportNeeds: string;
  whatHelps: string;
  currentSupport: string;
  youngPersonView: string;
  informationStatus: InformationStatus;
  informationSource: FrameworkSource | "";
  sourceDetails: string;
  sourceDate: string;
  priority: boolean;
  priorityReason: string;
};

type SourceState = { sourceType: FrameworkSource; sourceName: string; sourceDate: string; notes: string };
type PriorityState = { domainKey: CanonicalDomain; focusSummary: string; hopedForChange: string };
type ReviewPlan = {
  reviewFrequency: string;
  nextReviewDate: string;
  leadReviewer: string;
  otherProfessionals: string;
  earlyWarningSignals: string;
  notesForTeam: string;
};

type FrameworkResponse = {
  baseline: {
    framework_version: number; onboarding_pathway: Pathway;
    immediate_information: string; immediate_information_status: InformationStatus;
    summary_of_history: string; background_information_status: InformationStatus;
    recent_placement_pattern: string; current_concerns: string; strengths: string;
    existing_plans: string; childs_voice: string; known_safety_information: string;
    safety_information_status: InformationStatus; review_frequency: string;
    next_review_date: string | null; lead_reviewer: string; other_professionals: string;
    early_warning_signals: string; notes_for_team: string;
    status: Exclude<FrameworkStatus, "">; approved_by?: string; approved_at?: string;
  };
  domains: Array<{
    domain_key: CanonicalDomain; current_understanding: string;
    strengths_protective_factors: string; support_needs: string; what_helps: string;
    current_support: string; young_person_view: string; information_status: InformationStatus;
    information_source: FrameworkSource | ""; source_details: string; source_date?: string;
    active: boolean; priority_reason: string;
  }>;
  priorities: Array<{ domain_key: CanonicalDomain; focus_summary: string; hoped_for_change: string }>;
  sources: Array<{ source_type: FrameworkSource; source_name: string; source_date?: string; notes: string }>;
};

type HistoryItem = {
  id: number; version: number; status: string; onboarding_pathway: Pathway;
  created_at: string; approved_by?: string; approved_at?: string;
};

type Props = {
  apiBase: string;
  homeId: number;
  child: Child;
  pathway?: Pathway;
  focusedView?: "awaiting_information";
  onFinished: () => void;
  onBack: () => void;
  onOpenAwaitingInformation?: () => void;
  onOpenLegacyBaseline?: () => void;
  onOpenGoals?: () => void;
  onOpenCarePlan?: () => void;
  onOpenRisks?: () => void;
};

const blankDomain = (): DomainState => ({
  currentUnderstanding: "", strengthsProtectiveFactors: "", supportNeeds: "",
  whatHelps: "", currentSupport: "", youngPersonView: "",
  informationStatus: "not_known_yet", informationSource: "", sourceDetails: "",
  sourceDate: "", priority: false, priorityReason: "",
});

const blankDomains = () => Object.fromEntries(
  CANONICAL_DOMAINS.map((domain) => [domain.key, blankDomain()]),
) as Record<CanonicalDomain, DomainState>;

const blankReview: ReviewPlan = {
  reviewFrequency: "", nextReviewDate: "", leadReviewer: "",
  otherProfessionals: "", earlyWarningSignals: "", notesForTeam: "",
};

const cleanDate = (value?: string | null) => value?.slice(0, 10) ?? "";

function Field({ label, hint, children }: { label: string; hint?: string; children: ReactNode }) {
  return <div className="form-group"><label className="label">{label}</label>{hint && <p className="field-hint">{hint}</p>}{children}</div>;
}

function StatusSelect({ value, onChange }: { value: InformationStatus; onChange: (value: InformationStatus) => void }) {
  return <select className="input" value={value} onChange={(event) => onChange(event.target.value as InformationStatus)}>
    {INFORMATION_STATUSES.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}
  </select>;
}

export const OnboardBaselineForm = ({
  apiBase, homeId, child, pathway: initialPathway = "existing_young_person",
  focusedView, onFinished, onBack, onOpenAwaitingInformation, onOpenLegacyBaseline, onOpenGoals, onOpenCarePlan, onOpenRisks,
}: Props) => {
  const [step, setStep] = useState<Step>(1);
  const [domainIndex, setDomainIndex] = useState(0);
  const [pathway, setPathway] = useState<Pathway>(initialPathway);
  const [immediateInformation, setImmediateInformation] = useState("");
  const [immediateStatus, setImmediateStatus] = useState<InformationStatus>("not_known_yet");
  const [summaryOfHistory, setSummaryOfHistory] = useState("");
  const [backgroundStatus, setBackgroundStatus] = useState<InformationStatus>("not_known_yet");
  const [recentPlacementPattern, setRecentPlacementPattern] = useState("");
  const [currentConcerns, setCurrentConcerns] = useState("");
  const [strengths, setStrengths] = useState("");
  const [existingPlans, setExistingPlans] = useState("");
  const [childsVoice, setChildsVoice] = useState("");
  const [knownSafetyInformation, setKnownSafetyInformation] = useState("");
  const [safetyStatus, setSafetyStatus] = useState<InformationStatus>("not_known_yet");
  const [domains, setDomains] = useState<Record<CanonicalDomain, DomainState>>(blankDomains);
  const [sources, setSources] = useState<SourceState[]>([]);
  const [priorities, setPriorities] = useState<PriorityState[]>([]);
  const [reviewPlan, setReviewPlan] = useState<ReviewPlan>(blankReview);
  const [status, setStatus] = useState<FrameworkStatus>("");
  const [version, setVersion] = useState(1);
  const [approvedBy, setApprovedBy] = useState("");
  const [approvedAt, setApprovedAt] = useState("");
  const [history, setHistory] = useState<HistoryItem[]>([]);
  const [legacyBaselineAvailable, setLegacyBaselineAvailable] = useState(false);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [itemDrafts, setItemDrafts] = useState<Record<string, { value: string; status: InformationStatus }>>({});

  const load = async () => {
    setLoading(true);
    try {
      const [frameworkResponse, historyResponse] = await Promise.all([
        apiFetch(`${apiBase}/v1/children/${child.id}/onboarding/framework`),
        apiFetch(`${apiBase}/v1/children/${child.id}/onboarding/framework/history`),
      ]);
      if (historyResponse.ok) setHistory(await historyResponse.json() as HistoryItem[]);
      if (frameworkResponse.status === 404) {
        const legacyResponse = await apiFetch(`${apiBase}/v1/children/${child.id}/onboarding/baseline`);
        setLegacyBaselineAvailable(legacyResponse.ok);
        return;
      }
      if (!frameworkResponse.ok) throw new Error("The framework could not be loaded.");
      const data = await frameworkResponse.json() as FrameworkResponse;
      const b = data.baseline;
      setPathway(b.onboarding_pathway || initialPathway);
      setImmediateInformation(b.immediate_information || ""); setImmediateStatus(b.immediate_information_status || "not_known_yet");
      setSummaryOfHistory(b.summary_of_history || ""); setBackgroundStatus(b.background_information_status || "not_known_yet");
      setRecentPlacementPattern(b.recent_placement_pattern || ""); setCurrentConcerns(b.current_concerns || "");
      setStrengths(b.strengths || ""); setExistingPlans(b.existing_plans || ""); setChildsVoice(b.childs_voice || "");
      setKnownSafetyInformation(b.known_safety_information || ""); setSafetyStatus(b.safety_information_status || "not_known_yet");
      setStatus(b.status); setVersion(b.framework_version); setApprovedBy(b.approved_by || ""); setApprovedAt(b.approved_at || "");
      setReviewPlan({
        reviewFrequency: b.review_frequency || "", nextReviewDate: cleanDate(b.next_review_date),
        leadReviewer: b.lead_reviewer || "", otherProfessionals: b.other_professionals || "",
        earlyWarningSignals: b.early_warning_signals || "", notesForTeam: b.notes_for_team || "",
      });
      const nextDomains = blankDomains();
      data.domains.forEach((item) => {
        if (!nextDomains[item.domain_key]) return;
        nextDomains[item.domain_key] = {
          currentUnderstanding: item.current_understanding || "",
          strengthsProtectiveFactors: item.strengths_protective_factors || "",
          supportNeeds: item.support_needs || "", whatHelps: item.what_helps || "",
          currentSupport: item.current_support || "", youngPersonView: item.young_person_view || "",
          informationStatus: item.information_status || "not_known_yet",
          informationSource: item.information_source || "", sourceDetails: item.source_details || "",
          sourceDate: cleanDate(item.source_date), priority: item.active,
          priorityReason: item.priority_reason || "",
        };
      });
      setDomains(nextDomains);
      setPriorities(data.priorities.map((item) => ({ domainKey: item.domain_key, focusSummary: item.focus_summary || "", hopedForChange: item.hoped_for_change || "" })));
      setSources(data.sources.map((item) => ({ sourceType: item.source_type, sourceName: item.source_name || "", sourceDate: cleanDate(item.source_date), notes: item.notes || "" })));
      if (b.status === "submitted" || b.status === "approved") setStep(6);
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "The framework could not be loaded.");
    } finally { setLoading(false); }
  };

  // `load` intentionally reads the current form state only when this child/API changes.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => { void load(); }, [apiBase, child.id]);

  const priorityDomains = useMemo(() => CANONICAL_DOMAINS.filter((item) => domains[item.key].priority), [domains]);
  const unresolvedCount = useMemo(() => [
    immediateStatus, backgroundStatus, safetyStatus,
    ...CANONICAL_DOMAINS.map((item) => domains[item.key].informationStatus),
  ].filter((item) => item === "awaiting_confirmation" || item === "not_known_yet").length, [immediateStatus, backgroundStatus, safetyStatus, domains]);

  const updateDomain = (key: CanonicalDomain, patch: Partial<DomainState>) => setDomains((current) => ({ ...current, [key]: { ...current[key], ...patch } }));
  const syncPriorities = () => setPriorities((current) => priorityDomains.map((item) => current.find((priority) => priority.domainKey === item.key) ?? { domainKey: item.key, focusSummary: "", hopedForChange: "" }));

  const buildPayload = (nextStatus: "draft" | "submitted") => ({
    onboarding_pathway: pathway,
    immediate_information: immediateInformation, immediate_information_status: immediateStatus,
    summary_of_history: summaryOfHistory, background_information_status: backgroundStatus,
    recent_placement_pattern: recentPlacementPattern, current_concerns: currentConcerns,
    strengths, existing_plans: existingPlans, childs_voice: childsVoice,
    known_safety_information: knownSafetyInformation, safety_information_status: safetyStatus,
    domains: CANONICAL_DOMAINS.map((item) => ({
      domain_key: item.key, current_understanding: domains[item.key].currentUnderstanding,
      strengths_protective_factors: domains[item.key].strengthsProtectiveFactors,
      support_needs: domains[item.key].supportNeeds, what_helps: domains[item.key].whatHelps,
      current_support: domains[item.key].currentSupport, young_person_view: domains[item.key].youngPersonView,
      information_status: domains[item.key].informationStatus, information_source: domains[item.key].informationSource,
      source_details: domains[item.key].sourceDetails, source_date: domains[item.key].sourceDate || null,
      priority: domains[item.key].priority, priority_reason: domains[item.key].priorityReason,
    })),
    priorities: priorities.map((item, index) => ({ domain_key: item.domainKey, focus_summary: item.focusSummary, hoped_for_change: item.hopedForChange, sort_order: index + 1 })),
    sources: sources.map((item) => ({ source_type: item.sourceType, source_name: item.sourceName, source_date: item.sourceDate || null, notes: item.notes })),
    review_plan: {
      review_frequency: reviewPlan.reviewFrequency, next_review_date: reviewPlan.nextReviewDate || null,
      lead_reviewer: reviewPlan.leadReviewer, other_professionals: reviewPlan.otherProfessionals,
      early_warning_signals: reviewPlan.earlyWarningSignals, notes_for_team: reviewPlan.notesForTeam,
    },
    status: nextStatus,
  });

  const savePayload = async (payload: ReturnType<typeof buildPayload>, successMessage: string) => {
    setSaving(true); setError(null); setNotice(null);
    try {
      const response = await apiFetch(`${apiBase}/v1/homes/${homeId}/children/${child.id}/onboarding/framework`, {
        method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(payload),
      });
      if (!response.ok) throw new Error((await response.text()) || "The framework could not be saved.");
      setNotice(successMessage);
      setItemDrafts({});
      await load();
    } catch (caught) { setError(caught instanceof Error ? caught.message : "The framework could not be saved."); }
    finally { setSaving(false); }
  };

  const save = async (nextStatus: "draft" | "submitted") => {
    await savePayload(buildPayload(nextStatus), nextStatus === "draft" ? "Draft saved. The team can safely continue later." : "Framework submitted for manager approval.");
  };

  const approve = async () => {
    setSaving(true); setError(null);
    try {
      const response = await apiFetch(`${apiBase}/v1/children/${child.id}/onboarding/framework/approve`, { method: "POST" });
      if (!response.ok) throw new Error((await response.text()) || "Manager approval failed.");
      setNotice("Framework approved. Staff can now choose an appropriate next action."); await load();
    } catch (caught) { setError(caught instanceof Error ? caught.message : "Manager approval failed."); }
    finally { setSaving(false); }
  };

  const sourceSummary = sources.length > 0
    ? sources.map((source) => [
      frameworkSourceLabel(source.sourceType),
      source.sourceName || "Unnamed source",
      source.sourceDate || "",
    ].filter(Boolean).join(" · ")).join("; ")
    : "No source recorded";
  const unresolvedItems = useMemo(() => {
    const isUnresolved = (value: InformationStatus) => value === "awaiting_confirmation" || value === "not_known_yet";
    const items: Array<{
      key: string;
      section: string;
      label: string;
      value: string;
      status: InformationStatus;
      source: string;
      sourceDate: string;
      recordedBy: string;
      apply: (payload: ReturnType<typeof buildPayload>, value: string, status: InformationStatus) => void;
    }> = [];
    if (isUnresolved(immediateStatus)) {
      items.push({
        key: "immediate_information",
        section: "What staff need to know now",
        label: "Immediate information",
        value: immediateInformation,
        status: immediateStatus,
        source: sourceSummary,
        sourceDate: sources[0]?.sourceDate || "",
        recordedBy: approvedBy || "Framework author",
        apply: (payload, value, status) => {
          payload.immediate_information = value;
          payload.immediate_information_status = status;
        },
      });
    }
    if (isUnresolved(backgroundStatus)) {
      items.push({
        key: "summary_of_history",
        section: "What staff need to know now",
        label: "Background and significant history",
        value: summaryOfHistory,
        status: backgroundStatus,
        source: sourceSummary,
        sourceDate: sources[0]?.sourceDate || "",
        recordedBy: approvedBy || "Framework author",
        apply: (payload, value, status) => {
          payload.summary_of_history = value;
          payload.background_information_status = status;
        },
      });
    }
    if (isUnresolved(safetyStatus)) {
      items.push({
        key: "known_safety_information",
        section: "What staff need to know now",
        label: "Known safety information",
        value: knownSafetyInformation,
        status: safetyStatus,
        source: sourceSummary,
        sourceDate: sources[0]?.sourceDate || "",
        recordedBy: approvedBy || "Framework author",
        apply: (payload, value, status) => {
          payload.known_safety_information = value;
          payload.safety_information_status = status;
        },
      });
    }
    CANONICAL_DOMAINS.forEach((domainMeta) => {
      const domain = domains[domainMeta.key];
      if (!isUnresolved(domain.informationStatus)) return;
      items.push({
        key: `domain:${domainMeta.key}`,
        section: domainMeta.key,
        label: "Domain understanding",
        value: domain.currentUnderstanding,
        status: domain.informationStatus,
        source: domain.informationSource ? frameworkSourceLabel(domain.informationSource) : "No source recorded",
        sourceDate: domain.sourceDate,
        recordedBy: approvedBy || "Framework author",
        apply: (payload, value, status) => {
          const target = payload.domains.find((item) => item.domain_key === domainMeta.key);
          if (!target) return;
          target.current_understanding = value;
          target.information_status = status;
        },
      });
    });
    return items;
  }, [
    immediateStatus, immediateInformation, backgroundStatus, summaryOfHistory,
    safetyStatus, knownSafetyInformation, domains, sources, sourceSummary, approvedBy,
  ]);

  const updateItemDraft = (key: string, patch: Partial<{ value: string; status: InformationStatus }>) =>
    setItemDrafts((current) => ({ ...current, [key]: { ...current[key], ...patch } as { value: string; status: InformationStatus } }));
  const currentItemValue = (item: (typeof unresolvedItems)[number]) => itemDrafts[item.key]?.value ?? item.value;
  const currentItemStatus = (item: (typeof unresolvedItems)[number]) => itemDrafts[item.key]?.status ?? item.status;
  const saveUnresolvedItem = async (item: (typeof unresolvedItems)[number], action: "confirm" | "update" | "still_awaiting") => {
    const value = currentItemValue(item).trim();
    const nextStatus = action === "confirm" ? "known_confirmed" : action === "still_awaiting" ? "awaiting_confirmation" : currentItemStatus(item);
    if (action === "confirm" && value === "") {
      setError("Add or confirm the information before marking it known / confirmed.");
      return;
    }
    const payload = buildPayload("draft");
    item.apply(payload, value, nextStatus);
    const message = action === "confirm"
      ? "Information confirmed in a successor framework draft. The approved version remains in history."
      : action === "still_awaiting"
        ? "Item kept awaiting confirmation in a successor framework draft."
        : "Information updated in a successor framework draft. The approved version remains in history.";
    await savePayload(payload, message);
  };

  const addSource = () => setSources((current) => [...current, {
    sourceType: pathway === "new_placement" ? "local_authority_information" : "existing_care_plan",
    sourceName: "", sourceDate: "", notes: "",
  }]);

  if (loading) return <p className="field-hint">Loading framework…</p>;
  const labels = ["Placement basics", "What we know now", "Understand", "Priorities", "Review", "Approval"];
  const currentDomain = CANONICAL_DOMAINS[domainIndex];
  const domain = domains[currentDomain.key];
  const readOnly = status === "submitted" || status === "approved";

  if (focusedView === "awaiting_information") {
    return <section className="framework-page framework-awaiting-page">
      <header className="framework-header">
        <button type="button" className="text-button" onClick={onBack}>← Initial Framework</button>
        <div>
          <p className="current-care-eyebrow">Initial framework · awaiting confirmation</p>
          <h2>What information still needs confirmation?</h2>
          <p>Review only unresolved framework information. This does not edit Care Plans, Risks or Missing From Home plans.</p>
        </div>
        {status && <span className={`status-pill status-${status}`}>{status}</span>}
      </header>
      {error && <div className="alert alert-error">{error}</div>}{notice && <div className="alert alert-success">{notice}</div>}
      <div className="framework-unresolved framework-unresolved-review">
        <strong>AWAITING CONFIRMATION — {unresolvedCount} ITEMS</strong>
        <span>Changes are saved as the latest framework draft; approved historical versions remain readable.</span>
      </div>
      {unresolvedItems.length === 0 ? (
        <div className="framework-panel">
          <p className="current-care-eyebrow">No unresolved items</p>
          <h3>All currently recorded Framework information has been reviewed.</h3>
          <p className="field-hint">No score or completion percentage is created.</p>
        </div>
      ) : (
        <div className="framework-awaiting-list">
          {unresolvedItems.map((item) => (
            <article key={item.key} className="framework-awaiting-card">
              <div className="framework-awaiting-card-header">
                <div>
                  <p className="current-care-eyebrow">{item.section}</p>
                  <h3>{item.label}</h3>
                </div>
                <span className="status-pill status-draft">{informationStatusLabel(item.status)}</span>
              </div>
              <Field label="Current value or note">
                <textarea
                  className="input"
                  rows={4}
                  value={currentItemValue(item)}
                  onChange={(event) => updateItemDraft(item.key, { value: event.target.value })}
                />
              </Field>
              <div className="form-row-2">
                <Field label="Information state">
                  <StatusSelect value={currentItemStatus(item)} onChange={(value) => updateItemDraft(item.key, { status: value })} />
                </Field>
                <dl className="framework-awaiting-meta">
                  <div><dt>Source / provenance</dt><dd>{item.source}</dd></div>
                  <div><dt>Date recorded</dt><dd>{item.sourceDate || "Not recorded"}</dd></div>
                  <div><dt>Who recorded it</dt><dd>{item.recordedBy}</dd></div>
                </dl>
              </div>
              <div className="framework-awaiting-actions">
                <button type="button" className="btn btn-primary btn-xs" disabled={saving} onClick={() => void saveUnresolvedItem(item, "confirm")}>Confirm</button>
                <button type="button" className="btn btn-secondary btn-xs" disabled={saving} onClick={() => void saveUnresolvedItem(item, "update")}>Update</button>
                <button type="button" className="btn btn-ghost btn-xs" disabled={saving} onClick={() => void saveUnresolvedItem(item, "still_awaiting")}>Still awaiting</button>
              </div>
            </article>
          ))}
        </div>
      )}
    </section>;
  }

  return <section className="framework-page">
    <header className="framework-header">
      <button type="button" className="text-button" onClick={onBack}>← Back</button>
      <div><p className="current-care-eyebrow">Initial framework · Version {version}</p><h2>Understand {child.first_name || child.initials}</h2><p>A human-led starting profile of strengths, needs and current priorities.</p></div>
      {status && <span className={`status-pill status-${status}`}>{status}</span>}
    </header>

    {legacyBaselineAvailable && <div className="framework-legacy-notice"><strong>Historical eight-domain baseline preserved.</strong><span>It has not been rewritten or automatically mapped.</span>{onOpenLegacyBaseline && <button type="button" className="text-button" onClick={onOpenLegacyBaseline}>View legacy baseline</button>}</div>}
    <ol className="framework-stepper" aria-label="Framework progress">{labels.map((label, index) => <li key={label} className={step === index + 1 ? "active" : step > index + 1 ? "complete" : ""}><span>{index + 1}</span>{label}</li>)}</ol>
    {error && <div className="alert alert-error">{error}</div>}{notice && <div className="alert alert-success">{notice}</div>}

    {step === 1 && <div className="framework-panel">
      <p className="current-care-eyebrow">Placement basics</p><h3>How are we starting this framework?</h3>
      <div className="framework-choice-grid">
        <button type="button" className={`framework-choice ${pathway === "new_placement" ? "selected" : ""}`} onClick={() => setPathway("new_placement")}><strong>New placement</strong><span>Begin from information supplied at admission.</span></button>
        <button type="button" className={`framework-choice ${pathway === "existing_young_person" ? "selected" : ""}`} onClick={() => setPathway("existing_young_person")}><strong>Existing young person</strong><span>Use current knowledge and existing records.</span></button>
      </div>
      <dl className="framework-essentials"><div><dt>Young person</dt><dd>{child.first_name || child.initials}</dd></div><div><dt>Date of birth</dt><dd>{cleanDate(child.date_of_birth)}</dd></div><div><dt>Admission date</dt><dd>{cleanDate(child.admission_date) || "Not recorded"}</dd></div><div><dt>Legal status</dt><dd>{child.legal_status || "Not recorded"}</dd></div></dl>
      <p className="field-hint">Missing information is allowed. Record uncertainty rather than guessing.</p>
    </div>}

    {step === 2 && <div className="framework-panel">
      <p className="current-care-eyebrow">What staff need to know now</p><h3>Capture essentials without inventing certainty</h3>
      <div className="form-row-2"><Field label="What staff need to know immediately"><textarea className="input" rows={4} value={immediateInformation} onChange={(event) => setImmediateInformation(event.target.value)} /></Field><Field label="Information status"><StatusSelect value={immediateStatus} onChange={setImmediateStatus} /></Field></div>
      <div className="form-row-2"><Field label="Background and significant history"><textarea className="input" rows={4} value={summaryOfHistory} onChange={(event) => setSummaryOfHistory(event.target.value)} /></Field><Field label="Background information status"><StatusSelect value={backgroundStatus} onChange={setBackgroundStatus} /></Field></div>
      <Field label="Recent placement pattern"><textarea className="input" rows={3} value={recentPlacementPattern} onChange={(event) => setRecentPlacementPattern(event.target.value)} /></Field>
      <Field label="Current concerns or support needs"><textarea className="input" rows={3} value={currentConcerns} onChange={(event) => setCurrentConcerns(event.target.value)} /></Field>
      <Field label="Known strengths and protective factors"><textarea className="input" rows={3} value={strengths} onChange={(event) => setStrengths(event.target.value)} /></Field>
      <Field label="Existing plans, assessments or services"><textarea className="input" rows={3} value={existingPlans} onChange={(event) => setExistingPlans(event.target.value)} /></Field>
      <Field label="Young person’s overall view, where known"><textarea className="input" rows={3} value={childsVoice} onChange={(event) => setChildsVoice(event.target.value)} /></Field>
      <div className="framework-safety-boundary"><div><strong>Known safety information requiring review</strong><p>This records information for human review. It does not create a Risk record or make a safeguarding decision.</p></div><StatusSelect value={safetyStatus} onChange={setSafetyStatus} /><textarea className="input" rows={3} value={knownSafetyInformation} onChange={(event) => setKnownSafetyInformation(event.target.value)} /></div>
      <h3>Sources used</h3><p className="field-hint">Record source names and dates. Documents are not parsed by AI.</p>
      {sources.map((source, index) => <div className="framework-source-row" key={index}>
        <select className="input" value={source.sourceType} onChange={(event) => setSources((current) => current.map((item, itemIndex) => itemIndex === index ? { ...item, sourceType: event.target.value as FrameworkSource } : item))}>{FRAMEWORK_SOURCES.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}</select>
        <input className="input" aria-label="Source name" placeholder="Source name or reference" value={source.sourceName} onChange={(event) => setSources((current) => current.map((item, itemIndex) => itemIndex === index ? { ...item, sourceName: event.target.value } : item))} />
        <input className="input" aria-label="Source date" type="date" value={source.sourceDate} onChange={(event) => setSources((current) => current.map((item, itemIndex) => itemIndex === index ? { ...item, sourceDate: event.target.value } : item))} />
        <button type="button" className="btn btn-ghost btn-xs" onClick={() => setSources((current) => current.filter((_, itemIndex) => itemIndex !== index))}>Remove</button>
      </div>)}
      <button type="button" className="btn btn-secondary btn-xs" onClick={addSource}>+ Add source</button>
    </div>}

    {step === 3 && <div className="framework-panel">
      <p className="current-care-eyebrow">Understand the young person · Domain {domainIndex + 1} of 6</p><h3>{currentDomain.key}</h3><p>{currentDomain.help}</p><div className="framework-domain-progress"><span style={{ width: `${((domainIndex + 1) / 6) * 100}%` }} /></div>
      <div className="form-row-2">
        <Field label="What we currently understand"><textarea className="input" rows={3} value={domain.currentUnderstanding} onChange={(event) => updateDomain(currentDomain.key, { currentUnderstanding: event.target.value })} /></Field>
        <Field label="Strengths / protective factors"><textarea className="input" rows={3} value={domain.strengthsProtectiveFactors} onChange={(event) => updateDomain(currentDomain.key, { strengthsProtectiveFactors: event.target.value })} /></Field>
        <Field label="Current concerns / support needs"><textarea className="input" rows={3} value={domain.supportNeeds} onChange={(event) => updateDomain(currentDomain.key, { supportNeeds: event.target.value })} /></Field>
        <Field label="What appears to help"><textarea className="input" rows={3} value={domain.whatHelps} onChange={(event) => updateDomain(currentDomain.key, { whatHelps: event.target.value })} /></Field>
        <Field label="Current support already in place"><textarea className="input" rows={3} value={domain.currentSupport} onChange={(event) => updateDomain(currentDomain.key, { currentSupport: event.target.value })} /></Field>
        <Field label="Young person’s view, where known"><textarea className="input" rows={3} value={domain.youngPersonView} onChange={(event) => updateDomain(currentDomain.key, { youngPersonView: event.target.value })} /></Field>
      </div>
      <div className="form-row-2"><Field label="Information status"><StatusSelect value={domain.informationStatus} onChange={(value) => updateDomain(currentDomain.key, { informationStatus: value })} /></Field><Field label="Main information source"><select className="input" value={domain.informationSource} onChange={(event) => updateDomain(currentDomain.key, { informationSource: event.target.value as FrameworkSource | "" })}><option value="">No source recorded yet</option>{FRAMEWORK_SOURCES.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}</select></Field><Field label="Source detail"><input className="input" value={domain.sourceDetails} onChange={(event) => updateDomain(currentDomain.key, { sourceDetails: event.target.value })} /></Field><Field label="Source date"><input className="input" type="date" value={domain.sourceDate} onChange={(event) => updateDomain(currentDomain.key, { sourceDate: event.target.value })} /></Field></div>
      <label className="framework-priority-check"><input type="checkbox" checked={domain.priority} disabled={!domain.priority && priorityDomains.length >= 3} onChange={(event) => updateDomain(currentDomain.key, { priority: event.target.checked })} /><span><strong>Current priority</strong><small>Select up to three. This does not create goals or plans.</small></span></label>
      {domain.priority && <Field label="Why this is a priority"><textarea className="input" rows={2} value={domain.priorityReason} onChange={(event) => updateDomain(currentDomain.key, { priorityReason: event.target.value })} /></Field>}
      <div className="framework-domain-nav"><button type="button" className="btn btn-secondary" disabled={domainIndex === 0} onClick={() => setDomainIndex((index) => index - 1)}>Previous domain</button>{domainIndex < 5 ? <button type="button" className="btn btn-primary" onClick={() => setDomainIndex((index) => index + 1)}>Next domain</button> : <button type="button" className="btn btn-primary" onClick={() => { syncPriorities(); setStep(4); }}>Continue to priorities</button>}</div>
    </div>}

    {step === 4 && <div className="framework-panel">
      <p className="current-care-eyebrow">Priority domains</p><h3>Describe the small number of current priorities</h3>
      {priorities.length === 0 && <p className="framework-empty">No priority domains selected. Return to the domain profiles to choose up to three.</p>}
      {priorities.map((priority, index) => <div className="priority-block" key={priority.domainKey}><strong>{priority.domainKey}</strong><p className="field-hint">{domains[priority.domainKey].priorityReason || "No priority reason recorded yet."}</p><Field label="Focus"><textarea className="input" rows={2} value={priority.focusSummary} onChange={(event) => setPriorities((current) => current.map((item, itemIndex) => itemIndex === index ? { ...item, focusSummary: event.target.value } : item))} /></Field><Field label="What would positive everyday-life change look like?"><textarea className="input" rows={2} value={priority.hopedForChange} onChange={(event) => setPriorities((current) => current.map((item, itemIndex) => itemIndex === index ? { ...item, hopedForChange: event.target.value } : item))} /></Field></div>)}
      <div className="form-row-2"><Field label="Review frequency"><input className="input" value={reviewPlan.reviewFrequency} onChange={(event) => setReviewPlan((current) => ({ ...current, reviewFrequency: event.target.value }))} placeholder="e.g. every 6 weeks" /></Field><Field label="Next review date"><input className="input" type="date" value={reviewPlan.nextReviewDate} onChange={(event) => setReviewPlan((current) => ({ ...current, nextReviewDate: event.target.value }))} /></Field><Field label="Lead reviewer"><input className="input" value={reviewPlan.leadReviewer} onChange={(event) => setReviewPlan((current) => ({ ...current, leadReviewer: event.target.value }))} /></Field><Field label="Others to involve"><input className="input" value={reviewPlan.otherProfessionals} onChange={(event) => setReviewPlan((current) => ({ ...current, otherProfessionals: event.target.value }))} /></Field></div>
      <Field label="Signals that should prompt an earlier review"><textarea className="input" rows={2} value={reviewPlan.earlyWarningSignals} onChange={(event) => setReviewPlan((current) => ({ ...current, earlyWarningSignals: event.target.value }))} /></Field>
      <Field label="Notes for the team / handover"><textarea className="input" rows={2} value={reviewPlan.notesForTeam} onChange={(event) => setReviewPlan((current) => ({ ...current, notesForTeam: event.target.value }))} /></Field>
    </div>}

    {step === 5 && <div className="framework-panel framework-review">
      <p className="current-care-eyebrow">Review</p><h3>Check the starting framework</h3><div className="framework-unresolved"><strong>AWAITING CONFIRMATION — {unresolvedCount} ITEMS</strong><span>Unresolved information remains visible and does not need to be guessed.</span></div>
      <dl className="framework-summary-list"><div><dt>Pathway</dt><dd>{pathway === "new_placement" ? "New placement" : "Existing young person"}</dd></div><div><dt>Information needed now</dt><dd>{informationStatusLabel(immediateStatus)}</dd></div><div><dt>Safety information</dt><dd>{informationStatusLabel(safetyStatus)}</dd></div><div><dt>Sources recorded</dt><dd>{sources.length}</dd></div><div><dt>Priority domains</dt><dd>{priorities.length}</dd></div><div><dt>Next review</dt><dd>{reviewPlan.nextReviewDate || "Not set"}</dd></div><div><dt>Lead reviewer</dt><dd>{reviewPlan.leadReviewer || "Not recorded"}</dd></div></dl>
      <div className="framework-domain-summary-grid">{CANONICAL_DOMAINS.map((item) => <article key={item.key}><strong>{item.key}</strong><span>{informationStatusLabel(domains[item.key].informationStatus)}</span><p>{domains[item.key].currentUnderstanding || "No current understanding recorded yet."}</p>{domains[item.key].informationSource && <small>Source: {frameworkSourceLabel(domains[item.key].informationSource)}</small>}</article>)}</div>
      <div className="framework-boundary-note">Submitting does not create Risks, Care Plans, Goals or Targets. A manager reviews and approves the framework; staff choose any later actions.</div>
    </div>}

    {step === 6 && <div className="framework-panel"><p className="current-care-eyebrow">Manager approval</p><h3>{status === "approved" ? "Therapeutic framework approved" : "Framework awaiting manager approval"}</h3><div className="framework-unresolved"><strong>AWAITING CONFIRMATION — {unresolvedCount} ITEMS</strong><span>Approval does not hide unresolved information.</span>{status === "approved" && unresolvedCount > 0 && onOpenAwaitingInformation && <button type="button" className="btn btn-secondary btn-xs" onClick={onOpenAwaitingInformation}>Review awaiting information →</button>}</div>{status === "approved" ? <><p>Approved by <strong>{approvedBy || "Manager"}</strong>{approvedAt ? ` on ${new Date(approvedAt).toLocaleDateString()}` : ""}.</p><div className="framework-human-actions"><p>Choose a human-led next action where appropriate:</p>{onOpenGoals && <button type="button" className="btn btn-secondary" onClick={onOpenGoals}>Create Goal</button>}{onOpenCarePlan && <button type="button" className="btn btn-secondary" onClick={onOpenCarePlan}>Review Care Plan</button>}{onOpenRisks && <button type="button" className="btn btn-secondary" onClick={onOpenRisks}>Open Risk &amp; Safety</button>}</div></> : <><p>A manager must make the approval decision. Unresolved information may remain where policy permits.</p><button type="button" className="btn btn-primary" disabled={saving || status !== "submitted"} onClick={approve}>{saving ? "Approving…" : "Approve as manager"}</button></>}{history.length > 0 && <div className="framework-history"><h4>Framework history</h4>{history.map((item) => <div key={item.id}><strong>Version {item.version}</strong><span>{item.status}</span><time>{new Date(item.created_at).toLocaleDateString()}</time></div>)}</div>}</div>}

    {!readOnly && step !== 3 && step !== 6 && <footer className="framework-footer"><button type="button" className="btn btn-secondary" disabled={step === 1} onClick={() => setStep((current) => Math.max(1, current - 1) as Step)}>Back</button><button type="button" className="btn btn-ghost" disabled={saving} onClick={() => void save("draft")}>{saving ? "Saving…" : "Save draft"}</button>{step < 5 && <button type="button" className="btn btn-primary" onClick={() => setStep((current) => Math.min(5, current + 1) as Step)}>Continue</button>}{step === 5 && <button type="button" className="btn btn-primary" disabled={saving} onClick={() => void save("submitted")}>{saving ? "Submitting…" : "Submit for manager approval"}</button>}</footer>}
    {readOnly && <footer className="framework-footer"><button type="button" className="btn btn-secondary" onClick={onFinished}>Return to Child Home</button></footer>}
  </section>;
};
