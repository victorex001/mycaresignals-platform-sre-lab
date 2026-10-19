import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { apiFetch } from "./api";
import {
  CARE_PLAN_SECTIONS,
  carePlanDate,
  carePlanReadiness,
  carePlanSection,
  carePlanStatusLabel,
} from "./carePlanConfig";
import type { CarePlanSectionKey, Child, ChildCarePlan } from "./types";

type Props = {
  apiBase: string;
  child: Child;
  onBackToDashboard: () => void;
  onOpenSection: (section: CarePlanSectionKey) => void;
  onOpenReview: () => void;
  onOpenHistory: () => void;
};

export default function CarePlanTab({
  apiBase,
  child,
  onBackToDashboard,
  onOpenSection,
  onOpenReview,
  onOpenHistory,
}: Props) {
  const [plan, setPlan] = useState<ChildCarePlan | null>(null);
  const [canEdit, setCanEdit] = useState(false);
  const [canReview, setCanReview] = useState(false);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [effectiveFrom, setEffectiveFrom] = useState(new Date().toISOString().slice(0, 10));
  const [reviewDate, setReviewDate] = useState("");

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const [planResponse, accessResponse] = await Promise.all([
        apiFetch(`${apiBase}/v1/children/${child.id}/care-plans`),
        apiFetch(`${apiBase}/v1/children/${child.id}/care-plans/access`),
      ]);
      if (!planResponse.ok) throw new Error("The current Care Plan could not be loaded.");
      const data = await planResponse.json() as { plan: ChildCarePlan | null };
      setPlan(data.plan);
      const access = accessResponse.ok ? await accessResponse.json() as { can_edit: boolean; can_review: boolean } : { can_edit: false, can_review: false };
      setCanEdit(access.can_edit);
      setCanReview(access.can_review);
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "The current Care Plan could not be loaded.");
    } finally {
      setLoading(false);
    }
  };

  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => { void load(); }, [child.id]);

  const createPlan = async (event: FormEvent) => {
    event.preventDefault();
    setSaving(true);
    setError(null);
    setSuccess(null);
    try {
      const response = await apiFetch(`${apiBase}/v1/children/${child.id}/care-plans`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ effective_from: effectiveFrom, review_date: reviewDate || null }),
      });
      if (!response.ok) throw new Error((await response.text()) || "The draft Care Plan could not be created.");
      setCreateOpen(false);
      await load();
      setSuccess("Draft Care Plan created. Each area can now be considered separately.");
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "The draft Care Plan could not be created.");
    } finally {
      setSaving(false);
    }
  };

  const activatePlan = async () => {
    if (!plan) return;
    setSaving(true);
    setError(null);
    setSuccess(null);
    try {
      const response = await apiFetch(`${apiBase}/v1/children/${child.id}/care-plans/${plan.id}/activate`, { method: "POST" });
      if (!response.ok) throw new Error((await response.text()) || "The Care Plan could not be activated.");
      await load();
      setSuccess("Care Plan activated as the current agreed plan.");
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "The Care Plan could not be activated.");
    } finally {
      setSaving(false);
    }
  };

  const readiness = plan ? carePlanReadiness(plan) : { understood: 0, needsInformation: 0 };

  return (
    <section className="card care-plan-builder care-plan-current">
      <header className="card-header care-plan-header">
        <div><span className="care-eyebrow">Current agreed plan · {child.initials}</span><h2>Care Plan</h2><p className="card-subtitle">What is the young person&apos;s current agreed Care Plan?</p></div>
        <button type="button" className="btn btn-secondary btn-xs" onClick={onBackToDashboard}>← Child Home</button>
      </header>

      {error && <div className="alert alert-error">{error}</div>}
      {success && <div className="alert alert-success">{success}</div>}
      {loading && <div className="care-empty-state">Loading the current Care Plan…</div>}

      {!loading && (!plan || plan.status === "archived") && (
        <div className="care-empty-state">
          <strong>No current Care Plan</strong>
          <p>{plan ? "The previous plan remains available in Plan History." : "Start a draft with eight clear planning areas."}</p>
          <div className="care-primary-actions"><button className="btn btn-secondary" type="button" onClick={onOpenHistory}>Plan History</button>{!createOpen && <button className="btn btn-primary" type="button" onClick={() => setCreateOpen(true)}>Start draft Care Plan</button>}</div>
        </div>
      )}

      {createOpen && (
        <form className="care-plan-create-form" onSubmit={(event) => void createPlan(event)}>
          <div><h3>Start a draft Care Plan</h3><p className="field-hint">This creates a draft only. It does not change goals, risks or care decisions.</p></div>
          <div className="form-row-2"><div className="form-group"><label className="label" htmlFor="care-effective">Effective from</label><input id="care-effective" className="input" type="date" value={effectiveFrom} onChange={(event) => setEffectiveFrom(event.target.value)} required /></div><div className="form-group"><label className="label" htmlFor="care-review-date">Next review date</label><input id="care-review-date" className="input" type="date" value={reviewDate} onChange={(event) => setReviewDate(event.target.value)} /></div></div>
          <div className="care-form-actions"><button className="btn btn-ghost" type="button" onClick={() => setCreateOpen(false)}>Cancel</button><button className="btn btn-primary" type="submit" disabled={saving}>{saving ? "Creating…" : "Create draft"}</button></div>
        </form>
      )}

      {!loading && plan && plan.status !== "archived" && (
        <>
          <section className="care-current-summary" aria-label="Care Plan summary">
            <div><span>Plan status</span><strong className={`care-status care-status-${plan.status}`}>{carePlanStatusLabel(plan.status)}</strong></div>
            <div><span>Effective from</span><strong>{carePlanDate(plan.effective_from)}</strong></div>
            <div><span>Next review</span><strong>{carePlanDate(plan.review_date)}</strong></div>
            <div className="care-readiness-summary"><span>Readiness</span><strong>{readiness.understood} of 8 areas understood</strong><small>{readiness.needsInformation} area{readiness.needsInformation === 1 ? "" : "s"} need more information</small></div>
          </section>

          <div className="care-primary-actions">
            {plan.status === "active" && canReview && <button className="btn btn-primary" type="button" onClick={onOpenReview}>Review Care Plan</button>}
            {plan.status === "draft" && canReview && <button className="btn btn-primary" type="button" disabled={saving} onClick={() => void activatePlan()}>{saving ? "Activating…" : "Activate Care Plan · manager"}</button>}
            <button className="btn btn-secondary" type="button" onClick={onOpenHistory}>Plan History</button>
          </div>

          {plan.status === "draft" && <p className="field-hint care-current-note">This is a draft, not the current agreed plan. A manager must explicitly activate it.</p>}

          <section aria-labelledby="care-plan-areas-heading">
            <div className="care-simple-heading"><div><span className="care-eyebrow">Eight planning areas</span><h3 id="care-plan-areas-heading">Choose one area to view</h3></div></div>
            <div className="care-section-card-grid">
              {CARE_PLAN_SECTIONS.map((config, index) => {
                const section = carePlanSection(plan, config.key);
                return (
                  <article className="care-section-launcher" key={config.key}>
                    <div className="care-section-launcher-heading"><span className="care-section-number">{index + 1}</span><span className={`care-status care-status-${section?.information_status ?? "not_known_yet"}`}>{carePlanStatusLabel(section?.information_status ?? "not_known_yet")}</span></div>
                    <div><h3>{config.title}</h3><p>{config.description}</p></div>
                    <button className="btn btn-secondary" type="button" onClick={() => onOpenSection(config.key)}>{canEdit ? "View / Edit" : "View section"}</button>
                  </article>
                );
              })}
            </div>
          </section>
        </>
      )}
    </section>
  );
}
