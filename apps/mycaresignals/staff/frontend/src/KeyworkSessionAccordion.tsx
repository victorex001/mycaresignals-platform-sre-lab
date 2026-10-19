import { useMemo, useState } from "react";
import type { FC, FormEvent } from "react";
import { apiFetch } from "./api";
import type {
  ChildGoal,
  ChildTarget,
  KeyworkManagerReview,
  KeyworkSession,
} from "./types";

type Props = {
  sessions: KeyworkSession[];
  targets: ChildTarget[];
  goals?: ChildGoal[];
  apiBase?: string;
  childId?: number;
  onManagerReview?: (sessionId: number, review: KeyworkManagerReview) => void;
};

type ManagerDecision = "approved" | "returned_for_review";

const formatSessionDate = (value: string): string => {
  const datePart = value.slice(0, 10);
  const [year, month, day] = datePart.split("-").map(Number);
  if (!year || !month || !day) return value;
  return new Date(year, month - 1, day).toLocaleDateString(undefined, {
    dateStyle: "medium",
  });
};

const formatSignedAt = (value: string): string =>
  new Date(value).toLocaleString(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  });

const errorMessage = (error: unknown): string =>
  error instanceof Error ? error.message : "The manager review could not be saved.";

const KeyworkSessionAccordion: FC<Props> = ({
  sessions,
  targets,
  goals = [],
  apiBase,
  childId,
  onManagerReview,
}) => {
  const newestSessionId = sessions[0]?.id ?? null;
  const [expandedSessionId, setExpandedSessionId] = useState<number | null>(
    newestSessionId,
  );
  const [reviewingSessionId, setReviewingSessionId] = useState<number | null>(
    null,
  );
  const [decision, setDecision] = useState<ManagerDecision>("approved");
  const [comment, setComment] = useState("");
  const [saving, setSaving] = useState(false);
  const [reviewError, setReviewError] = useState<string | null>(null);
  const targetsById = useMemo(
    () => new Map(targets.map((target) => [target.id, target])),
    [targets],
  );
  const goalsById = useMemo(
    () => new Map(goals.map((goal) => [goal.id, goal])),
    [goals],
  );

  const openReview = (sessionId: number) => {
    setReviewingSessionId(sessionId);
    setDecision("approved");
    setComment("");
    setReviewError(null);
  };

  const closeReview = () => {
    setReviewingSessionId(null);
    setDecision("approved");
    setComment("");
    setReviewError(null);
  };

  const submitReview = async (event: FormEvent, sessionId: number) => {
    event.preventDefault();
    setReviewError(null);
    const reviewComment = comment.trim();
    if (decision === "returned_for_review" && !reviewComment) {
      setReviewError("Add a manager comment when returning for follow-up.");
      return;
    }
    if (!apiBase || !childId || !onManagerReview) {
      setReviewError("Manager review is unavailable from this view.");
      return;
    }
    setSaving(true);
    try {
      const response = await apiFetch(
        `${apiBase}/v1/children/${childId}/sessions/${sessionId}/manager-review`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ decision, comment: reviewComment }),
        },
      );
      if (!response.ok) {
        const payload = (await response.json().catch(() => null)) as
          | { error?: string }
          | null;
        throw new Error(payload?.error || "The manager review could not be saved.");
      }
      const payload = (await response.json()) as {
        manager_review: KeyworkManagerReview;
      };
      onManagerReview(sessionId, payload.manager_review);
      closeReview();
    } catch (caught: unknown) {
      setReviewError(errorMessage(caught));
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="keywork-history-list">
      {sessions.map((session) => {
        const linkedTarget = session.target_id
          ? targetsById.get(session.target_id)
          : undefined;
        const isExpanded = expandedSessionId === session.id;
        const isReviewing = reviewingSessionId === session.id;
        const detailId = `keywork-session-details-${session.id}`;
        const targetLabel = linkedTarget
          ? linkedTarget.short_label || `Target #${linkedTarget.id}`
          : session.target_id
            ? `Target #${session.target_id}`
            : "Target unavailable";
        const parentGoal = linkedTarget?.goal_id
          ? goalsById.get(linkedTarget.goal_id)
          : undefined;
        const managerState = session.manager_review_decision
          ? session.manager_review_decision === "approved"
            ? "Manager approved"
            : "Returned for follow-up"
          : session.signed_by_key_worker
            ? "Manager review pending"
            : "Manager review not ready";

        return (
          <article key={session.id} className="keywork-history-card">
            <button
              type="button"
              className="keywork-history-toggle"
              aria-expanded={isExpanded}
              aria-controls={detailId}
              onClick={() =>
                setExpandedSessionId(isExpanded ? null : session.id)
              }
            >
              <span className="keywork-history-summary">
                <span className="keywork-history-summary-topline">
                  <strong>{formatSessionDate(session.session_at)}</strong>
                  <span className="keywork-session-type">Key working</span>
                </span>
                <span>
                  <strong>Target:</strong> {targetLabel}
                </span>
                {parentGoal && (
                  <span>
                    <strong>Goal:</strong> {parentGoal.title}
                  </span>
                )}
                <span className="keywork-history-summary-status">
                  {session.primary_domain && <span>{session.primary_domain}</span>}
                  <span>
                    {session.signed_by_key_worker ? "✓ Complete" : "○ Not complete"}
                  </span>
                  <span
                    className={`manager-review-badge manager-review-badge-${session.manager_review_decision ?? "pending"}`}
                  >
                    {managerState}
                  </span>
                </span>
              </span>
              <span className="keywork-history-toggle-indicator">
                <span>{isExpanded ? "Collapse" : "Expand"}</span>
                <span aria-hidden="true">{isExpanded ? "▴" : "▾"}</span>
              </span>
            </button>

            {isExpanded && (
              <div id={detailId} className="keywork-history-details">
                <div className="keywork-history-staff">
                  <strong>Recorded by</strong>
                  <span>{session.staff_name || "Staff attribution unavailable"}</span>
                </div>

                <div className="keywork-history-content">
                  {session.young_person_presentation && (
                    <div>
                      <h4>Young person&apos;s presentation</h4>
                      <p>{session.young_person_presentation}</p>
                    </div>
                  )}
                  {session.young_person_voice_note && (
                    <div>
                      <h4>Young person&apos;s voice</h4>
                      <p>{session.young_person_voice_note}</p>
                    </div>
                  )}
                  {session.key_worker_reflection && (
                    <div>
                      <h4>Key worker reflection</h4>
                      <p>{session.key_worker_reflection}</p>
                    </div>
                  )}
                  {session.notes && (
                    <div>
                      <h4>Additional notes</h4>
                      <p>{session.notes}</p>
                    </div>
                  )}
                  {session.scores?.length > 0 && (
                    <div>
                      <h4>Session scores</h4>
                      <div className="manager-review-score-list">
                        {session.scores.map((score, index) => {
                          const scoreTarget = targetsById.get(score.target_id);
                          return (
                            <div key={`${score.target_id}-${index}`}>
                              <strong>
                                {scoreTarget?.short_label || `Target #${score.target_id}`}
                              </strong>
                              <span>
                                Wellbeing: {score.wellbeing_score ?? "Not recorded"} · Progress:{" "}
                                {score.progress_score ?? "Not recorded"}
                              </span>
                              {score.comment && <p>{score.comment}</p>}
                            </div>
                          );
                        })}
                      </div>
                    </div>
                  )}
                </div>

                <div className="keywork-signoff-state">
                  <strong>
                    {session.signed_by_key_worker
                      ? "Key worker confirmed complete"
                      : "Not confirmed complete"}
                  </strong>
                  {session.signed_by_key_worker && session.key_worker_signed_at && (
                    <span>Signed {formatSignedAt(session.key_worker_signed_at)}</span>
                  )}
                </div>

                <section className="manager-review-state" aria-label="Manager review">
                  <div>
                    <h4>Manager review</h4>
                    <strong>{managerState}</strong>
                  </div>
                  {session.manager_review_decision && (
                    <div className="manager-review-result">
                      <span>
                        {session.manager_reviewer_name
                          ? `Reviewed by ${session.manager_reviewer_name}`
                          : "Reviewer attribution unavailable"}
                        {session.manager_signed_at &&
                          ` · ${formatSignedAt(session.manager_signed_at)}`}
                      </span>
                      {session.manager_review_comment && (
                        <p>{session.manager_review_comment}</p>
                      )}
                    </div>
                  )}
                  {!session.signed_by_key_worker && (
                    <p>The key worker must confirm this record is complete before manager review.</p>
                  )}
                  {session.can_manager_review && !isReviewing && (
                    <button
                      type="button"
                      className="btn btn-secondary btn-xs"
                      onClick={() => openReview(session.id)}
                    >
                      Review session
                    </button>
                  )}
                  {session.can_manager_review && isReviewing && (
                    <form
                      className="manager-review-form"
                      onSubmit={(event) => void submitReview(event, session.id)}
                    >
                      <p className="field-hint">
                        Review the signed record above. This action does not edit the original session.
                      </p>
                      <fieldset>
                        <legend>Decision</legend>
                        <label>
                          <input
                            type="radio"
                            name={`manager-decision-${session.id}`}
                            value="approved"
                            checked={decision === "approved"}
                            onChange={() => setDecision("approved")}
                          />
                          <span>Approve</span>
                        </label>
                        <label>
                          <input
                            type="radio"
                            name={`manager-decision-${session.id}`}
                            value="returned_for_review"
                            checked={decision === "returned_for_review"}
                            onChange={() => setDecision("returned_for_review")}
                          />
                          <span>Return for follow-up</span>
                        </label>
                      </fieldset>
                      <div className="form-group">
                        <label className="label" htmlFor={`manager-review-comment-${session.id}`}>
                          Manager comment
                          {decision === "returned_for_review" && (
                            <span className="required"> *</span>
                          )}
                        </label>
                        <textarea
                          id={`manager-review-comment-${session.id}`}
                          className="input"
                          rows={3}
                          value={comment}
                          onChange={(event) => setComment(event.target.value)}
                          required={decision === "returned_for_review"}
                          maxLength={4000}
                        />
                      </div>
                      {reviewError && <div className="alert alert-error">{reviewError}</div>}
                      <div className="button-row">
                        <button type="submit" className="btn btn-primary btn-xs" disabled={saving}>
                          {saving ? "Saving review…" : "Save manager review"}
                        </button>
                        <button type="button" className="btn btn-ghost btn-xs" onClick={closeReview} disabled={saving}>
                          Cancel
                        </button>
                      </div>
                    </form>
                  )}
                </section>
              </div>
            )}
          </article>
        );
      })}
    </div>
  );
};

export default KeyworkSessionAccordion;
