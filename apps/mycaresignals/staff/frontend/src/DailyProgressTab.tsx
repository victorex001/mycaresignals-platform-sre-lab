import { useEffect, useMemo, useState } from "react";
import type { FormEvent } from "react";
import { apiFetch } from "./api";
import type { ChildTarget } from "./types";

type Props = {
  apiBase: string;
  childId: number;
  childInitials: string;
  onBackToDashboard: () => void;
  onOpenProgress: () => void;
  onOpenHistory: () => void;
};

const WELLBEING_SCORE_OPTIONS = [
  { value: "", label: "Not recorded" },
  { value: "0", label: "0 · Very difficult" },
  { value: "1", label: "1 · Difficult" },
  { value: "2", label: "2 · Mixed" },
  { value: "3", label: "3 · Mostly settled" },
  { value: "4", label: "4 · Strong wellbeing" },
] as const;

const PROGRESS_SCORE_OPTIONS = [
  { value: "", label: "Not recorded" },
  { value: "0", label: "0 · No progress seen" },
  { value: "1", label: "1 · Early signs" },
  { value: "2", label: "2 · Developing" },
  { value: "3", label: "3 · Clear progress" },
  { value: "4", label: "4 · Sustained progress" },
] as const;

type FormState = {
  observedOn: string;
  targetId: string;
  domain: string;
  wellbeingScore: string;
  progressScore: string;
  observationNote: string;
  youngPersonComment: string;
};

type PriorityDomain = {
  domain_key: string;
  focus_summary?: string;
  hoped_for_change?: string;
  sort_order?: number;
};

type FrameworkResponse = {
  priorities?: PriorityDomain[];
};

const today = () => new Date().toISOString().slice(0, 10);

const emptyForm = (): FormState => ({
  observedOn: today(),
  targetId: "",
  domain: "",
  wellbeingScore: "",
  progressScore: "",
  observationNote: "",
  youngPersonComment: "",
});

const DailyProgressTab = ({
  apiBase,
  childId,
  childInitials,
  onBackToDashboard,
  onOpenProgress,
  onOpenHistory,
}: Props) => {
  const [targets, setTargets] = useState<ChildTarget[]>([]);
  const [priorityDomains, setPriorityDomains] = useState<PriorityDomain[]>([]);
  const [form, setForm] = useState<FormState>(emptyForm);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  const priorityDomainKeys = useMemo(
    () => new Set(priorityDomains.map((priority) => priority.domain_key)),
    [priorityDomains],
  );

  const priorityTargets = useMemo(
    () => targets.filter(
      (target) => target.status === "active" && priorityDomainKeys.has(target.domain),
    ),
    [priorityDomainKeys, targets],
  );

  const targetsByPriority = useMemo(
    () => priorityDomains.map((priority) => ({
      priority,
      targets: priorityTargets.filter((target) => target.domain === priority.domain_key),
    })),
    [priorityDomains, priorityTargets],
  );

  const selectedTarget = useMemo(
    () => priorityTargets.find((target) => target.id === Number(form.targetId)),
    [form.targetId, priorityTargets],
  );

  const selectedPriority = useMemo(
    () => priorityDomains.find((priority) => priority.domain_key === form.domain),
    [form.domain, priorityDomains],
  );

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const [targetResponse, frameworkResponse] = await Promise.all([
        apiFetch(`${apiBase}/v1/children/${childId}/targets`),
        apiFetch(`${apiBase}/v1/children/${childId}/onboarding/framework`),
      ]);
      if (!targetResponse.ok) {
        throw new Error("Daily progress could not be loaded.");
      }
      const targetData = await targetResponse.json() as ChildTarget[];
      const frameworkData = frameworkResponse.ok
        ? await frameworkResponse.json() as FrameworkResponse
        : { priorities: [] };
      setTargets(Array.isArray(targetData) ? targetData : []);
      setPriorityDomains(
        Array.isArray(frameworkData.priorities)
          ? frameworkData.priorities.slice(0, 3)
          : [],
      );
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Daily progress could not be loaded.");
      setTargets([]);
      setPriorityDomains([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    setForm(emptyForm());
    setSuccess(null);
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [childId]);

  const selectTarget = (target: ChildTarget) => {
    setForm((current) => ({
      ...current,
      targetId: String(target.id),
      domain: target.domain,
    }));
  };

  const selectPriorityDomain = (domain: string) => {
    setForm((current) => ({
      ...current,
      targetId: "",
      domain,
    }));
  };

  const save = async (event: FormEvent) => {
    event.preventDefault();
    setError(null);
    setSuccess(null);
    if (!form.targetId && !form.domain) {
      setError("Choose a current target or a priority domain.");
      return;
    }
    if (form.targetId && !selectedTarget) {
      setError("Choose a current active target from the priority domains.");
      return;
    }
    if (!form.targetId && !priorityDomainKeys.has(form.domain)) {
      setError("Choose one of the current priority domains.");
      return;
    }
    if (!form.wellbeingScore && !form.progressScore && !form.observationNote.trim()) {
      setError("Record a rating or a concise staff observation.");
      return;
    }
    setSaving(true);
    try {
      const response = await apiFetch(`${apiBase}/v1/children/${childId}/daily-progress`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          observed_on: form.observedOn,
          domain: form.domain,
          target_id: form.targetId ? Number(form.targetId) : null,
          wellbeing_score: form.wellbeingScore === "" ? null : Number(form.wellbeingScore),
          progress_score: form.progressScore === "" ? null : Number(form.progressScore),
          observation_note: form.observationNote.trim(),
          young_person_comment: form.youngPersonComment.trim(),
        }),
      });
      if (!response.ok) {
        throw new Error((await response.text()) || "The daily observation could not be saved.");
      }
      setForm(emptyForm());
      setSuccess("Daily staff observation recorded. It cannot be edited or deleted.");
      await load();
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "The daily observation could not be saved.");
    } finally {
      setSaving(false);
    }
  };

  return (
    <section className="card daily-progress-page">
      <header className="card-header daily-progress-header">
        <div>
          <span className="care-eyebrow">Daily care evidence · {childInitials}</span>
          <h2>Daily Progress</h2>
          <p className="card-subtitle">
            Record what you noticed today.
          </p>
        </div>
        <div className="care-header-actions">
          <button type="button" className="btn btn-ghost" onClick={onBackToDashboard}>← Child Home</button>
          <button type="button" className="btn btn-secondary" onClick={onOpenProgress}>Progress Over Time →</button>
          <button type="button" className="btn btn-secondary" onClick={onOpenHistory}>Daily Progress History →</button>
        </div>
      </header>

      {error && <div className="alert alert-error">{error}</div>}
      {success && <div className="alert alert-success">{success}</div>}

      <div className="daily-progress-layout">
        <form className="daily-progress-form" onSubmit={save}>
          <div>
            <p className="current-care-eyebrow">Staff observation</p>
            <h3>What did I notice today?</h3>
            <p className="field-hint">This is an append-only care record. Use factual, respectful language.</p>
          </div>

          {loading && <p className="field-hint">Loading current priorities…</p>}
          {!loading && priorityDomains.length === 0 && (
            <div className="alert alert-warning">
              No current priority domains are available for Daily Progress. A manager should review the Initial Framework / Goals &amp; Targets.
            </div>
          )}

          {!loading && priorityDomains.length > 0 && (
            <section className="daily-priority-picker" aria-label="Current targets and priority domains">
              <div>
                <p className="current-care-eyebrow">Current live targets</p>
                <h4>Choose the target this observation relates to</h4>
              </div>
              {priorityTargets.length === 0 && (
                <p className="field-hint">No active targets are recorded within the current priority domains.</p>
              )}
              {targetsByPriority.map(({ priority, targets: domainTargets }) => (
                <div className="daily-priority-group" key={priority.domain_key}>
                  <div className="daily-priority-heading">
                    <strong>{priority.domain_key}</strong>
                    {priority.focus_summary && <span>{priority.focus_summary}</span>}
                  </div>
                  {domainTargets.length > 0 && (
                    <div className="daily-target-card-grid">
                      {domainTargets.map((target) => (
                        <button
                          type="button"
                          key={target.id}
                          className={`daily-target-choice ${form.targetId === String(target.id) ? "is-selected" : ""}`}
                          onClick={() => selectTarget(target)}
                        >
                          <strong>{target.short_label}</strong>
                          <span>{target.domain}</span>
                        </button>
                      ))}
                    </div>
                  )}
                </div>
              ))}
              <div className="daily-domain-only">
                <div>
                  <p className="current-care-eyebrow">Domain-only observation</p>
                  <h4>Record against a priority domain only</h4>
                </div>
                <div className="daily-domain-card-grid">
                  {priorityDomains.map((priority) => (
                    <button
                      type="button"
                      key={priority.domain_key}
                      className={`daily-domain-choice ${!form.targetId && form.domain === priority.domain_key ? "is-selected" : ""}`}
                      onClick={() => selectPriorityDomain(priority.domain_key)}
                    >
                      <strong>{priority.domain_key}</strong>
                      <span>{priority.hoped_for_change || priority.focus_summary || "Current priority domain"}</span>
                    </button>
                  ))}
                </div>
              </div>
            </section>
          )}

          <div className="form-group">
            <label className="label" htmlFor="daily-observed-on">Date observed</label>
            <input id="daily-observed-on" type="date" className="input" max={today()} value={form.observedOn} onChange={(event) => setForm({ ...form, observedOn: event.target.value })} required />
          </div>

          <div className="daily-selection-summary">
            <div>
              <span className="label">Selected target</span>
              <p>{selectedTarget?.short_label ?? "No SMART target selected"}</p>
            </div>
            <div>
              <span className="label">Domain</span>
              <p>{selectedTarget?.domain ?? selectedPriority?.domain_key ?? "Choose a target or priority domain"}</p>
              {selectedTarget && <p className="field-hint">Derived from the selected target and locked.</p>}
            </div>
          </div>

          <div className="form-row-2">
            <div className="form-group">
              <label className="label" htmlFor="daily-wellbeing">Observed wellbeing <span className="label-pill">Optional</span></label>
              <select id="daily-wellbeing" className="input" value={form.wellbeingScore} onChange={(event) => setForm({ ...form, wellbeingScore: event.target.value })}>
                {WELLBEING_SCORE_OPTIONS.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
              </select>
              <p className="field-hint">How was the young person doing or feeling?</p>
            </div>
            <div className="form-group">
              <label className="label" htmlFor="daily-progress-score">Observed progress <span className="label-pill">Optional</span></label>
              <select id="daily-progress-score" className="input" value={form.progressScore} onChange={(event) => setForm({ ...form, progressScore: event.target.value })}>
                {PROGRESS_SCORE_OPTIONS.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
              </select>
              <p className="field-hint">How much movement was observed toward the target or domain outcome?</p>
            </div>
          </div>

          <div className="form-group">
            <label className="label" htmlFor="daily-observation">Concise observation</label>
            <textarea id="daily-observation" className="input" rows={4} maxLength={2000} value={form.observationNote} onChange={(event) => setForm({ ...form, observationNote: event.target.value })} placeholder="What was seen or heard, and what support was offered?" />
          </div>
          <div className="form-group young-person-comment-field">
            <label className="label" htmlFor="daily-young-person-comment">Young person&apos;s words heard today <span className="label-pill">Optional, verbatim</span></label>
            <textarea id="daily-young-person-comment" className="input" rows={2} maxLength={1000} value={form.youngPersonComment} onChange={(event) => setForm({ ...form, youngPersonComment: event.target.value })} />
            <p className="field-hint">This contextual note is not the verified monthly young-person voice rating.</p>
          </div>
          <div className="daily-record-actions">
            <button type="submit" className="btn btn-primary" disabled={saving || loading || priorityDomains.length === 0}>{saving ? "Recording…" : "Record daily observation"}</button>
            <button type="button" className="text-button daily-view-all" onClick={onOpenHistory}>
              View Daily Progress History →
            </button>
          </div>
        </form>
      </div>
    </section>
  );
};

export default DailyProgressTab;
