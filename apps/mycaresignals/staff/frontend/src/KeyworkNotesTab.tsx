// src/KeyworkNotesTab.tsx
import React, { useEffect, useMemo, useState } from "react";
import type { ChildGoal, ChildTarget, KeyworkSession } from "./types";
import { apiFetch } from "./api";
import KeyworkSessionAccordion from "./KeyworkSessionAccordion";

type Props = {
  apiBase: string;
  childId: number;
  childInitials: string;
  onBackToDashboard: () => void;
  onOpenHistory: () => void;
};

type ScoreValue = number | "";

const scoreOptions = [0, 1, 2, 3, 4] as const;

const localDateToday = (): string => {
  const today = new Date();
  const year = today.getFullYear();
  const month = String(today.getMonth() + 1).padStart(2, "0");
  const day = String(today.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
};

const errorMessage = (error: unknown, fallback: string): string =>
  error instanceof Error ? error.message : fallback;

const KeyworkNotesTab: React.FC<Props> = ({
  apiBase,
  childId,
  childInitials,
  onBackToDashboard,
  onOpenHistory,
}) => {
  const [sessionDate, setSessionDate] = useState(localDateToday);
  const [selectedTargetId, setSelectedTargetId] = useState<number | "">("");
  const [presentation, setPresentation] = useState("");
  const [youngPersonVoice, setYoungPersonVoice] = useState("");
  const [reflection, setReflection] = useState("");
  const [wellbeingScore, setWellbeingScore] = useState<ScoreValue>("");
  const [progressScore, setProgressScore] = useState<ScoreValue>("");
  const [scoreComment, setScoreComment] = useState("");
  const [additionalNotes, setAdditionalNotes] = useState("");
  const [completionConfirmed, setCompletionConfirmed] = useState(false);

  const [targets, setTargets] = useState<ChildTarget[]>([]);
  const [goals, setGoals] = useState<ChildGoal[]>([]);
  const [sessions, setSessions] = useState<KeyworkSession[]>([]);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  const activeTargets = useMemo(
    () => targets.filter((target) => target.status.toLowerCase() === "active"),
    [targets],
  );
  const selectedTarget = useMemo(
    () =>
      activeTargets.find((target) => target.id === selectedTargetId) ?? null,
    [activeTargets, selectedTargetId],
  );
  const goalsById = useMemo(
    () => new Map(goals.map((goal) => [goal.id, goal])),
    [goals],
  );
  const loadData = async () => {
    setLoading(true);
    setError(null);
    try {
      const [targetsRes, sessionsRes, goalsRes] = await Promise.all([
        apiFetch(`${apiBase}/v1/children/${childId}/targets`),
        apiFetch(`${apiBase}/v1/children/${childId}/sessions`),
        apiFetch(`${apiBase}/v1/children/${childId}/goals`),
      ]);

      if (!targetsRes.ok) throw new Error("Failed to load plan targets.");
      if (!sessionsRes.ok && sessionsRes.status !== 404) {
        throw new Error("Failed to load previous sessions.");
      }
      if (!goalsRes.ok) throw new Error("Failed to load plan goals.");

      const targetData: ChildTarget[] = await targetsRes.json();
      setTargets(Array.isArray(targetData) ? targetData : []);
      const goalData: ChildGoal[] = await goalsRes.json();
      setGoals(Array.isArray(goalData) ? goalData : []);

      if (sessionsRes.ok) {
        const sessionData: KeyworkSession[] = await sessionsRes.json();
        const loadedSessions = Array.isArray(sessionData) ? sessionData : [];
        setSessions(loadedSessions);
      } else {
        setSessions([]);
      }
    } catch (caught: unknown) {
      setError(errorMessage(caught, "Could not load key-working data."));
      setTargets([]);
      setGoals([]);
      setSessions([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    setSessionDate(localDateToday());
    setSelectedTargetId("");
    setPresentation("");
    setYoungPersonVoice("");
    setReflection("");
    setWellbeingScore("");
    setProgressScore("");
    setScoreComment("");
    setAdditionalNotes("");
    setCompletionConfirmed(false);
    setSuccess(null);
    loadData();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [childId]);

  useEffect(() => {
    if (
      selectedTargetId !== "" &&
      !activeTargets.some((target) => target.id === selectedTargetId)
    ) {
      setSelectedTargetId("");
    }
  }, [activeTargets, selectedTargetId]);

  const resetForm = () => {
    setSessionDate(localDateToday());
    setSelectedTargetId("");
    setPresentation("");
    setYoungPersonVoice("");
    setReflection("");
    setWellbeingScore("");
    setProgressScore("");
    setScoreComment("");
    setAdditionalNotes("");
    setCompletionConfirmed(false);
  };

  const handleSaveSession = async (event: React.FormEvent) => {
    event.preventDefault();
    setError(null);
    setSuccess(null);

    if (!/^\d{4}-\d{2}-\d{2}$/.test(sessionDate)) {
      setError("Choose a valid session date.");
      return;
    }
    if (!selectedTarget) {
      setError("Select an active primary target.");
      return;
    }
    if (wellbeingScore === "" || progressScore === "") {
      setError("Select both wellbeing and progress scores.");
      return;
    }
    if (!completionConfirmed) {
      setError("Confirm that the key-working record is complete before saving.");
      return;
    }

    const body = {
      session_at: sessionDate,
      session_type: "keywork",
      notes: additionalNotes.trim(),
      primary_domain: selectedTarget.domain,
      target_id: selectedTarget.id,
      young_person_presentation: presentation.trim(),
      young_person_voice_note: youngPersonVoice.trim(),
      key_worker_reflection: reflection.trim(),
      signed_by_key_worker: true,
      scores: [
        {
          target_id: selectedTarget.id,
          wellbeing_score: wellbeingScore,
          progress_score: progressScore,
          comment: scoreComment.trim(),
        },
      ],
    };

    setSaving(true);
    try {
      const response = await apiFetch(
        `${apiBase}/v1/children/${childId}/sessions`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(body),
        },
      );

      if (!response.ok) {
        throw new Error("The key-working session could not be saved.");
      }

      resetForm();
      await loadData();
      setSuccess("Key-working session saved and added to the session history.");
    } catch (caught: unknown) {
      setError(errorMessage(caught, "Could not save the key-working session."));
    } finally {
      setSaving(false);
    }
  };

  const renderTargetLabel = (target: ChildTarget): string => {
    const domain = target.domain || "No domain";
    const label = target.short_label || `Target #${target.id}`;
    const goal = target.goal_id ? goalsById.get(target.goal_id) : undefined;
    return goal ? `${goal.title} — ${label} [${domain}]` : `[${domain}] ${label}`;
  };

  const canSave =
    !saving &&
    activeTargets.length > 0 &&
    selectedTarget !== null &&
    wellbeingScore !== "" &&
    progressScore !== "" &&
    completionConfirmed;

  return (
    <section className="card">
      <div className="card-header">
        <div>
          <h2>Key Working</h2>
          <p className="card-subtitle">
            Record a structured therapeutic session for child{" "}
            <strong>{childInitials}</strong> · ID {childId}
          </p>
        </div>
        <button type="button" className="btn btn-ghost" onClick={onBackToDashboard}>
          ← Child Home
        </button>
      </div>

      {error && (
        <div className="alert alert-error">
          <strong>Error:</strong> {error}
        </div>
      )}
      {success && <div className="alert alert-success">{success}</div>}

      <div className="card card-sub keywork-entry-card">
        <h3>Record a Key Working session</h3>
        <p className="field-hint">
          Staff attribution comes from your authenticated account. Session dates
          are stored as dates only; a time of day is not retained.
        </p>

        {loading && targets.length === 0 && (
          <p className="field-hint">Loading targets and session history…</p>
        )}

        {!loading && activeTargets.length === 0 && (
          <div className="alert alert-warning">
            No active targets are available. Create or activate a target in
            Goals &amp; targets before recording a Key Working session.
          </div>
        )}

        <form className="form-grid" onSubmit={handleSaveSession}>
          <div className="form-row-2">
            <div className="form-group">
              <label className="label" htmlFor="keywork-session-date">
                Session date <span className="required">*</span>
              </label>
              <input
                id="keywork-session-date"
                type="date"
                className="input"
                value={sessionDate}
                onChange={(event) => setSessionDate(event.target.value)}
                required
              />
            </div>
            <div className="form-group">
              <span className="label">Session type</span>
              <div className="keywork-readonly-value">Key working</div>
            </div>
          </div>

          <div className="form-group">
            <label className="label" htmlFor="keywork-primary-target">
              Primary target <span className="required">*</span>
            </label>
            <select
              id="keywork-primary-target"
              className="input"
              value={selectedTargetId}
              onChange={(event) =>
                setSelectedTargetId(
                  event.target.value === "" ? "" : Number(event.target.value),
                )
              }
              disabled={loading || activeTargets.length === 0}
              required
            >
              <option value="">Select an active target…</option>
              {activeTargets.map((target) => (
                <option key={target.id} value={target.id}>
                  {renderTargetLabel(target)}
                </option>
              ))}
            </select>
          </div>

          <div className="form-group">
            <span className="label">Primary therapeutic domain</span>
            <div className="keywork-readonly-value">
              {selectedTarget?.domain || "Select a primary target to derive its domain"}
            </div>
          </div>

          {selectedTarget?.goal_id && goalsById.get(selectedTarget.goal_id) && (
            <div className="form-group">
              <span className="label">Care-plan goal</span>
              <div className="keywork-readonly-value">
                {goalsById.get(selectedTarget.goal_id)?.title}
              </div>
            </div>
          )}

          <div className="keywork-form-section">
            <div>
              <h4>Therapeutic record</h4>
              <p className="field-hint">
                Keep the young person&apos;s own voice separate from professional
                observation and reflection.
              </p>
            </div>

            <div className="form-group">
              <label className="label" htmlFor="keywork-presentation">
                Young person&apos;s presentation
              </label>
              <p className="field-hint">
                What you observed about how the young person presented during
                the session.
              </p>
              <textarea
                id="keywork-presentation"
                className="input"
                rows={3}
                value={presentation}
                onChange={(event) => setPresentation(event.target.value)}
              />
            </div>

            <div className="form-group">
              <label className="label" htmlFor="keywork-young-person-voice">
                Young person&apos;s voice
              </label>
              <p className="field-hint">
                Record their words or perspective without rewriting or
                interpreting them.
              </p>
              <textarea
                id="keywork-young-person-voice"
                className="input"
                rows={3}
                value={youngPersonVoice}
                onChange={(event) => setYoungPersonVoice(event.target.value)}
              />
            </div>

            <div className="form-group">
              <label className="label" htmlFor="keywork-reflection">
                Key worker reflection
              </label>
              <p className="field-hint">
                Professional reflection on progress, concerns, themes, and next
                steps.
              </p>
              <textarea
                id="keywork-reflection"
                className="input"
                rows={3}
                value={reflection}
                onChange={(event) => setReflection(event.target.value)}
              />
            </div>
          </div>

          <div className="keywork-form-section">
            <div>
              <h4>Session scores</h4>
              <p className="field-hint">
                Select both scores explicitly. Zero is a valid score and remains
                distinct from an unselected score.
              </p>
            </div>

            <div className="form-row-2">
              <div className="form-group">
                <label className="label" htmlFor="keywork-wellbeing-score">
                  Wellbeing score <span className="required">*</span>
                </label>
                <select
                  id="keywork-wellbeing-score"
                  className="input"
                  value={wellbeingScore}
                  onChange={(event) =>
                    setWellbeingScore(
                      event.target.value === "" ? "" : Number(event.target.value),
                    )
                  }
                  required
                >
                  <option value="">Not selected</option>
                  {scoreOptions.map((score) => (
                    <option key={score} value={score}>
                      {score}
                    </option>
                  ))}
                </select>
              </div>
              <div className="form-group">
                <label className="label" htmlFor="keywork-progress-score">
                  Progress score <span className="required">*</span>
                </label>
                <select
                  id="keywork-progress-score"
                  className="input"
                  value={progressScore}
                  onChange={(event) =>
                    setProgressScore(
                      event.target.value === "" ? "" : Number(event.target.value),
                    )
                  }
                  required
                >
                  <option value="">Not selected</option>
                  {scoreOptions.map((score) => (
                    <option key={score} value={score}>
                      {score}
                    </option>
                  ))}
                </select>
              </div>
            </div>

            <div className="form-group">
              <label className="label" htmlFor="keywork-score-comment">
                Score comment <span className="label-pill">Optional</span>
              </label>
              <textarea
                id="keywork-score-comment"
                className="input"
                rows={2}
                value={scoreComment}
                onChange={(event) => setScoreComment(event.target.value)}
              />
            </div>
          </div>

          <div className="form-group">
            <label className="label" htmlFor="keywork-additional-notes">
              Additional notes <span className="label-pill">Optional</span>
            </label>
            <textarea
              id="keywork-additional-notes"
              className="input"
              rows={3}
              value={additionalNotes}
              onChange={(event) => setAdditionalNotes(event.target.value)}
            />
          </div>

          <label className="keywork-signoff" htmlFor="keywork-complete">
            <input
              id="keywork-complete"
              type="checkbox"
              checked={completionConfirmed}
              onChange={(event) => setCompletionConfirmed(event.target.checked)}
              required
            />
            <span>
              I confirm this key-working record is complete and accurately
              reflects the session.
            </span>
          </label>

          <button type="submit" className="btn btn-primary" disabled={!canSave}>
            {saving ? "Saving session…" : "Save session"}
          </button>
        </form>
      </div>

      <div className="card card-sub keywork-history" style={{ marginTop: "1rem" }}>
        <div className="card-header">
          <div>
            <h3>Recent Key Working</h3>
            <p className="field-hint">
              The newest records are shown here. Use the history dashboard to
              browse and filter the full archive.
            </p>
          </div>
          <button
            type="button"
            className="btn btn-primary"
            onClick={onOpenHistory}
          >
            View all Key Working history
          </button>
        </div>
        <p className="field-hint">
          Detailed wellbeing and progress scores remain available in Scores &amp;
          timeline because this session-list response does not include scores.
        </p>
        {loading && <p className="field-hint">Loading…</p>}
        {!loading && sessions.length === 0 && (
          <p className="field-hint">No Key Working sessions recorded yet.</p>
        )}
        {sessions.length > 0 && (
          <KeyworkSessionAccordion
            key={sessions[0].id}
            sessions={sessions.slice(0, 3)}
            targets={targets}
            goals={goals}
          />
        )}
      </div>
    </section>
  );
};

export default KeyworkNotesTab;
