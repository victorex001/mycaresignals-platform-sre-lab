import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { apiFetch } from "./api";
import {
  CARE_PLAN_INFORMATION_STATUSES,
  CARE_PLAN_SECTIONS,
  carePlanDate,
  carePlanSection,
  carePlanStatusLabel,
} from "./carePlanConfig";
import type {
  CarePlanInformationStatus,
  CarePlanSectionKey,
  Child,
  ChildCarePlan,
  ChildCarePlanSection,
} from "./types";

type Props = {
  apiBase: string;
  child: Child;
  sectionKey: CarePlanSectionKey;
  onBackToPlan: () => void;
  onOpenBaseline: () => void;
};

type SectionForm = {
  informationStatus: CarePlanInformationStatus;
  currentUnderstanding: string;
  strengths: string;
  staffSupport: string;
  youngPersonInvolvementNote: string;
};

const formFromSection = (section?: ChildCarePlanSection): SectionForm => ({
  informationStatus: section?.information_status ?? "not_known_yet",
  currentUnderstanding: section?.current_understanding ?? "",
  strengths: section?.strengths ?? "",
  staffSupport: section?.staff_support ?? "",
  youngPersonInvolvementNote: section?.young_person_involvement_note ?? "",
});

export default function CarePlanSectionView({ apiBase, child, sectionKey, onBackToPlan, onOpenBaseline }: Props) {
  const config = CARE_PLAN_SECTIONS.find((item) => item.key === sectionKey);
  const [plan, setPlan] = useState<ChildCarePlan | null>(null);
  const [canEdit, setCanEdit] = useState(false);
  const [form, setForm] = useState<SectionForm>(formFromSection());
  const [editing, setEditing] = useState(false);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const [planResponse, accessResponse] = await Promise.all([
        apiFetch(`${apiBase}/v1/children/${child.id}/care-plans`),
        apiFetch(`${apiBase}/v1/children/${child.id}/care-plans/access`),
      ]);
      if (!planResponse.ok) throw new Error("The Care Plan section could not be loaded.");
      const data = await planResponse.json() as { plan: ChildCarePlan | null };
      const access = accessResponse.ok ? await accessResponse.json() as { can_edit: boolean } : { can_edit: false };
      setPlan(data.plan);
      setCanEdit(access.can_edit);
      setForm(formFromSection(data.plan ? carePlanSection(data.plan, sectionKey) : undefined));
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "The Care Plan section could not be loaded.");
    } finally {
      setLoading(false);
    }
  };

  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => { void load(); }, [child.id, sectionKey]);

  const save = async (event: FormEvent) => {
    event.preventDefault();
    if (!plan || !config) return;
    setSaving(true);
    setError(null);
    setSuccess(null);
    try {
      const response = await apiFetch(`${apiBase}/v1/children/${child.id}/care-plans/${plan.id}/sections/${sectionKey}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          information_status: form.informationStatus,
          current_understanding: form.currentUnderstanding,
          strengths: form.strengths,
          staff_support: form.staffSupport,
          young_person_involvement_note: form.youngPersonInvolvementNote,
        }),
      });
      if (!response.ok) throw new Error((await response.text()) || "This section could not be saved.");
      await load();
      setEditing(false);
      setSuccess(`${config.title} saved.`);
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "This section could not be saved.");
    } finally {
      setSaving(false);
    }
  };

  if (!config) return <div className="alert alert-error">Unknown Care Plan section.</div>;
  const section = plan ? carePlanSection(plan, sectionKey) : undefined;
  const editable = Boolean(plan && plan.status !== "archived" && canEdit);

  return (
    <section className="card care-plan-builder care-section-page">
      <header className="card-header care-plan-header">
        <div><span className="care-eyebrow">Care Plan area · {child.initials}</span><h2>{config.title}</h2><p className="card-subtitle">{config.description}</p></div>
        <button className="btn btn-secondary btn-xs" type="button" onClick={onBackToPlan}>← Current Care Plan</button>
      </header>
      {error && <div className="alert alert-error">{error}</div>}
      {success && <div className="alert alert-success">{success}</div>}
      {loading && <div className="care-empty-state">Loading this Care Plan area…</div>}
      {!loading && !plan && <div className="care-empty-state">No current Care Plan is available.</div>}
      {!loading && plan && (
        <>
          <div className="care-section-context"><span className={`care-status care-status-${section?.information_status ?? "not_known_yet"}`}>{carePlanStatusLabel(section?.information_status ?? "not_known_yet")}</span><span>{carePlanStatusLabel(plan.status)} plan</span><span>Effective {carePlanDate(plan.effective_from)}</span></div>
          {sectionKey === "about_me" && <div className="care-source-note"><strong>Existing source</strong><span>The Initial Framework remains available as separate source information and is not silently copied into this plan.</span><button className="btn btn-ghost btn-xs" type="button" onClick={onOpenBaseline}>Open Initial Framework</button></div>}

          {!editing && (
            <div className="care-section-reading">
              <div><h3>Current understanding</h3><p>{section?.current_understanding || "Not recorded"}</p></div>
              <div><h3>Strengths to build on</h3><p>{section?.strengths || "Not recorded"}</p></div>
              <div><h3>Everyday staff support</h3><p>{section?.staff_support || "Not recorded"}</p></div>
              <div><h3>Young person involvement</h3><p>{section?.young_person_involvement_note || "Not recorded — Young Person Voice remains in its own record."}</p></div>
            </div>
          )}

          {editing && (
            <form className="care-plan-section-form care-section-focused-form" onSubmit={(event) => void save(event)}>
              <div className="form-group"><label className="label" htmlFor="care-section-status">Information state</label><select id="care-section-status" className="input" value={form.informationStatus} onChange={(event) => setForm({ ...form, informationStatus: event.target.value as CarePlanInformationStatus })}>{CARE_PLAN_INFORMATION_STATUSES.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}</select><p className="field-hint">Uncertainty is valid. Do not fill gaps with assumptions.</p></div>
              <div className="form-group"><label className="label" htmlFor="care-section-understanding">What do we currently understand?</label><textarea id="care-section-understanding" className="input" rows={5} value={form.currentUnderstanding} onChange={(event) => setForm({ ...form, currentUnderstanding: event.target.value })} /></div>
              <div className="form-group"><label className="label" htmlFor="care-section-strengths">What is going well or what strengths can we build on?</label><textarea id="care-section-strengths" className="input" rows={4} value={form.strengths} onChange={(event) => setForm({ ...form, strengths: event.target.value })} /></div>
              <div className="form-group"><label className="label" htmlFor="care-section-support">How will staff support the young person in everyday care?</label><textarea id="care-section-support" className="input" rows={4} value={form.staffSupport} onChange={(event) => setForm({ ...form, staffSupport: event.target.value })} /></div>
              <div className="form-group"><label className="label" htmlFor="care-section-involvement">How has the young person been involved?</label><textarea id="care-section-involvement" className="input" rows={4} value={form.youngPersonInvolvementNote} onChange={(event) => setForm({ ...form, youngPersonInvolvementNote: event.target.value })} /><p className="field-hint">Record involvement context here; do not copy Young Person Voice quotes.</p></div>
              <div className="care-form-actions"><button className="btn btn-ghost" type="button" onClick={() => { setForm(formFromSection(section)); setEditing(false); }}>Cancel</button><button className="btn btn-primary" type="submit" disabled={saving}>{saving ? "Saving…" : "Save section"}</button></div>
            </form>
          )}

          {!editing && editable && <div className="care-primary-actions"><button className="btn btn-primary" type="button" onClick={() => setEditing(true)}>Edit this section</button></div>}
          {!editable && <p className="field-hint">This section is read-only for your role or because the plan is archived.</p>}
        </>
      )}
    </section>
  );
}
