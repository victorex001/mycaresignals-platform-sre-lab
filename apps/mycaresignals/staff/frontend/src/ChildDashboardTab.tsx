import React, { useEffect, useMemo, useRef, useState } from "react";
import { apiFetch } from "./api";
import type { Child, ChildCareOverview } from "./types";

type Props = {
  apiBase: string;
  homeName: string;
  child: Child;
  onBackToHome: () => void;
  onOpenPlan: () => void;
  onOpenCarePlan: () => void;
  onOpenRisks: () => void;
  onOpenDailyProgress: () => void;
  onOpenKeyworkNotes: () => void;
  onOpenKeyworkHistory: () => void;
  onOpenSummary: () => void;
  onOpenChildVoice: () => void;
  onOpenBaselineReport: () => void;
  onOpenChildReport: () => void;
  onOpenScoresTrends: () => void;
  onOpenFeedbackReview: () => void;
  focusAttention?: boolean;
};

const formatDate = (value?: string | null) => {
  if (!value) return "Not set";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "Not set";
  return date.toLocaleDateString(undefined, {
    day: "numeric",
    month: "short",
    year: "numeric",
  });
};

const formatStatus = (value?: string) =>
  value ? value.replaceAll("_", " ") : "Not started";

const plural = (
  count: number,
  singular: string,
  pluralForm = `${singular}s`,
) => `${count} ${count === 1 ? singular : pluralForm}`;

const ChildDashboardTab: React.FC<Props> = ({
  apiBase,
  homeName,
  child,
  onBackToHome,
  onOpenPlan,
  onOpenCarePlan,
  onOpenRisks,
  onOpenDailyProgress,
  onOpenKeyworkNotes,
  onOpenKeyworkHistory,
  onOpenSummary,
  onOpenChildVoice,
  onOpenBaselineReport,
  onOpenChildReport,
  onOpenScoresTrends,
  onOpenFeedbackReview,
  focusAttention = false,
}) => {
  const [overview, setOverview] = useState<ChildCareOverview | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const attentionRef = useRef<HTMLElement | null>(null);

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      setLoading(true);
      setError(null);
      try {
        const response = await apiFetch(
          `${apiBase}/v1/children/${child.id}/care-overview`,
        );
        if (!response.ok) {
          throw new Error(
            `Could not load current care overview (${response.status})`,
          );
        }
        const payload: ChildCareOverview = await response.json();
        if (!cancelled) setOverview(payload);
      } catch (loadError) {
        if (!cancelled) {
          setOverview(null);
          setError(
            loadError instanceof Error
              ? loadError.message
              : "Could not load current care overview",
          );
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    };
    void load();
    return () => {
      cancelled = true;
    };
  }, [apiBase, child.id]);

  useEffect(() => {
    if (focusAttention && overview) {
      attentionRef.current?.scrollIntoView({ behavior: "smooth", block: "start" });
      attentionRef.current?.focus({ preventScroll: true });
    }
  }, [focusAttention, overview]);

  const sourceActions = useMemo(
    () => ({
      dashboard: undefined,
      goals: onOpenPlan,
      risks: onOpenRisks,
      "care-plan": onOpenCarePlan,
      "daily-progress": onOpenDailyProgress,
      "keywork-history": onOpenKeyworkHistory,
      "child-voice": onOpenChildVoice,
      "baseline-report": onOpenBaselineReport,
      "feedback-review": onOpenFeedbackReview,
    }),
    [
      onOpenBaselineReport,
      onOpenCarePlan,
      onOpenChildVoice,
      onOpenKeyworkHistory,
      onOpenDailyProgress,
      onOpenPlan,
      onOpenRisks,
      onOpenFeedbackReview,
    ],
  );

  const displayName =
    overview?.child.display_name ||
    child.first_name ||
    child.initials ||
    "Young person";
  const timelineLead = overview?.timeline.slice(0, 7) ?? [];
  const timelineMore = overview?.timeline.slice(7) ?? [];

  const sourceButton = (source: string) => {
    const action = sourceActions[source as keyof typeof sourceActions];
    if (!action) return null;
    return (
      <button type="button" className="text-button" onClick={action}>
        Open source
      </button>
    );
  };

  return (
    <section className="card current-care-page">
      <header className="current-care-header">
        <div>
          <button
            type="button"
            className="text-button"
            onClick={onBackToHome}
          >
            ← Child Home
          </button>
          <p className="current-care-eyebrow">Unified Child Journey</p>
          <h2>{displayName}</h2>
          <p className="current-care-placement">
            {overview?.child.home_name || homeName}
            {(overview?.child.admission_date || child.admission_date) && (
              <>
                {" "}· Placement started{" "}
                {formatDate(
                  overview?.child.admission_date || child.admission_date,
                )}
              </>
            )}
          </p>
        </div>
        <div className="current-care-header-actions">
          <button
            type="button"
            className="btn btn-primary"
            onClick={onOpenCarePlan}
          >
            Open Care Plan
          </button>
          <button
            type="button"
            className="btn btn-secondary"
            onClick={onOpenChildReport}
          >
            Full child report
          </button>
        </div>
      </header>

      {loading && (
        <div className="current-care-loading" role="status">
          Building the current care picture…
        </div>
      )}
      {error && (
        <div className="alert alert-error">
          <strong>Current care overview unavailable.</strong> {error}
        </div>
      )}

      {!loading && overview && (
        <>
          <section
            className="current-care-position"
            aria-label="Placement and Care Plan position"
          >
            <div>
              <span>Care Plan</span>
              <strong>
                {overview.care_plan.exists
                  ? `${formatStatus(overview.care_plan.status)} · Plan ${overview.care_plan.version}`
                  : "Not started"}
              </strong>
            </div>
            <div>
              <span>Next Care Plan review</span>
              <strong>{formatDate(overview.care_plan.review_date)}</strong>
            </div>
            <div>
              <span>Onboarding baseline</span>
              <strong>{formatStatus(overview.baseline.status)}</strong>
            </div>
          </section>

          <section ref={attentionRef} tabIndex={-1} className="current-care-section attention-section">
            <div className="current-care-section-heading">
              <div>
                <p className="current-care-eyebrow">
                  Scheduling and completeness
                </p>
                <h3>Needs attention</h3>
              </div>
            </div>
            {overview.needs_attention.length === 0 ? (
              <p className="attention-clear">
                No current review actions are due.
              </p>
            ) : (
              <ul className="attention-list">
                {overview.needs_attention.map((item) => (
                  <li key={`${item.kind}-${item.label}`}>
                    <span>{item.label}</span>
                    {sourceActions[
                      item.source as keyof typeof sourceActions
                    ] && (
                      <button
                        type="button"
                        className="text-button"
                        onClick={
                          sourceActions[
                            item.source as keyof typeof sourceActions
                          ]
                        }
                      >
                        Open record
                      </button>
                    )}
                  </li>
                ))}
              </ul>
            )}
          </section>

          <section className="current-care-section">
            <div className="current-care-section-heading">
              <div>
                <p className="current-care-eyebrow">What matters right now</p>
                <h3>Current priorities</h3>
              </div>
              <button
                type="button"
                className="text-button"
                onClick={onOpenBaselineReport}
              >
                Open baseline
              </button>
            </div>
            {overview.current_priorities.length === 0 ? (
              <p className="current-care-empty">
                No current priority activity is recorded yet. The baseline,
                goals, risks and Key Working records will build this view.
              </p>
            ) : (
              <div className="priority-domain-grid">
                {overview.current_priorities.map((priority) => (
                  <article
                    className="priority-domain-card"
                    key={priority.domain_key}
                  >
                    <h4>{priority.label}</h4>
                    {priority.focus_summary && <p>{priority.focus_summary}</p>}
                    {priority.hoped_for_change && (
                      <p className="priority-hoped-for">
                        Hoped-for change: {priority.hoped_for_change}
                      </p>
                    )}
                    <div className="priority-domain-counts">
                      {priority.active_goals > 0 && (
                        <span>
                          {plural(priority.active_goals, "active goal")}
                        </span>
                      )}
                      {priority.active_targets > 0 && (
                        <span>
                          {plural(priority.active_targets, "active target")}
                        </span>
                      )}
                      {priority.active_risks > 0 && (
                        <span>
                          {plural(priority.active_risks, "active risk")}
                        </span>
                      )}
                      {priority.recent_keyworking > 0 && (
                        <span>
                          {plural(
                            priority.recent_keyworking,
                            "recent Key Working session",
                          )}
                        </span>
                      )}
                      {priority.target_reviews_due > 0 && (
                        <span>
                          {plural(
                            priority.target_reviews_due,
                            "target review",
                          )}{" "}
                          due
                        </span>
                      )}
                    </div>
                  </article>
                ))}
              </div>
            )}
          </section>

          <div className="current-care-two-column">
            <section className="current-care-section care-story-card daily-overview-card">
              <div className="current-care-section-heading">
                <div>
                  <p className="current-care-eyebrow">What staff are observing</p>
                  <h3>Recent Daily Progress</h3>
                </div>
                <button type="button" className="text-button" onClick={onOpenDailyProgress}>
                  Open Daily Progress
                </button>
              </div>
              {overview.recent_daily_progress.length === 0 ? (
                <p className="current-care-empty">No daily staff observations are recorded yet.</p>
              ) : (
                <ul className="care-story-list">
                  {overview.recent_daily_progress.map((entry, index) => (
                    <li key={`${entry.observed_on}-${entry.domain}-${index}`}>
                      <strong>{entry.target_label || entry.domain}</strong>
                      <span>{formatDate(entry.observed_on)} · {entry.domain}</span>
                      <span>
                        Staff-observed progress {entry.progress_score == null ? "not rated" : `${entry.progress_score} / 4`}
                        {" · "}
                        wellbeing {entry.wellbeing_score == null ? "not rated" : `${entry.wellbeing_score} / 4`}
                      </span>
                    </li>
                  ))}
                </ul>
              )}
              {overview.daily_progress_review_suggested && (
                <p className="professional-review-note">Persistent difference in perspectives — review suggested.</p>
              )}
            </section>

            <section className="current-care-section care-story-card">
              <div className="current-care-section-heading">
                <div>
                  <p className="current-care-eyebrow">
                    What we are working towards
                  </p>
                  <h3>Goals &amp; Targets</h3>
                </div>
                <button
                  type="button"
                  className="text-button"
                  onClick={onOpenPlan}
                >
                  Open Goals &amp; Targets
                </button>
              </div>
              <div className="story-count-line">
                <strong>
                  {plural(overview.goals_targets.active_goals, "active goal")}
                </strong>
                <span>
                  {plural(
                    overview.goals_targets.active_targets,
                    "active target",
                  )}
                </span>
                {overview.goals_targets.targets_due > 0 && (
                  <span>
                    {plural(
                      overview.goals_targets.targets_due,
                      "target review",
                    )}{" "}
                    due
                  </span>
                )}
                {overview.goals_targets.achieved_recently > 0 && (
                  <span>
                    {plural(
                      overview.goals_targets.achieved_recently,
                      "target",
                    )}{" "}
                    achieved recently
                  </span>
                )}
              </div>
              {overview.goals_targets.goals.length === 0 ? (
                <p className="current-care-empty">
                  No active goals are recorded.
                </p>
              ) : (
                <ul className="care-story-list">
                  {overview.goals_targets.goals.map((goal) => (
                    <li key={`${goal.title}-${goal.domain ?? ""}`}>
                      <strong>{goal.title}</strong>
                      {goal.domain && <span>{formatStatus(goal.domain)}</span>}
                    </li>
                  ))}
                </ul>
              )}
              {overview.goals_targets.targets.length > 0 && (
                <div className="compact-target-list">
                  {overview.goals_targets.targets.map((target) => (
                    <div key={`${target.label}-${target.goal_title ?? ""}`}>
                      <strong>{target.label}</strong>
                      <span>
                        {target.goal_title && <>Goal: {target.goal_title}</>}
                        {target.review_date && (
                          <> · Review {formatDate(target.review_date)}</>
                        )}
                      </span>
                    </div>
                  ))}
                </div>
              )}
            </section>

            <section className="current-care-section care-story-card">
              <div className="current-care-section-heading">
                <div>
                  <p className="current-care-eyebrow">
                    What we are worried about
                  </p>
                  <h3>Current safety picture</h3>
                </div>
                <button
                  type="button"
                  className="text-button"
                  onClick={onOpenRisks}
                >
                  Open Risk &amp; Safety
                </button>
              </div>
              <p className="current-care-language-note">
                Current recorded risk information
              </p>
              <div className="story-count-line">
                <strong>
                  {plural(overview.risk_safety.active_risks, "active risk")}
                </strong>
                {overview.risk_safety.recorded_high > 0 && (
                  <span>{overview.risk_safety.recorded_high} recorded high</span>
                )}
                {overview.risk_safety.recorded_medium > 0 && (
                  <span>
                    {overview.risk_safety.recorded_medium} recorded medium
                  </span>
                )}
                {overview.risk_safety.recorded_low > 0 && (
                  <span>{overview.risk_safety.recorded_low} recorded low</span>
                )}
                {overview.risk_safety.recorded_not_rated > 0 && (
                  <span>
                    {overview.risk_safety.recorded_not_rated} not rated
                  </span>
                )}
                {overview.risk_safety.recent_completed_reviews > 0 && (
                  <span>
                    {plural(
                      overview.risk_safety.recent_completed_reviews,
                      "review",
                    )}{" "}
                    completed in the last 30 days
                  </span>
                )}
              </div>
              {overview.risk_safety.items.length === 0 ? (
                <p className="current-care-empty">
                  No active risk information is recorded.
                </p>
              ) : (
                <ul className="care-story-list">
                  {overview.risk_safety.items.map((risk) => (
                    <li key={risk.title}>
                      <strong>{risk.title}</strong>
                      <span>
                        {risk.recorded_rating != null
                          ? `Recorded residual rating ${risk.recorded_rating} / ${risk.recorded_band}`
                          : "Recorded residual rating not set"}
                        {risk.next_review_date && (
                          <> · Review {formatDate(risk.next_review_date)}</>
                        )}
                      </span>
                    </li>
                  ))}
                </ul>
              )}
              {overview.risk_safety.reviews_due > 0 && (
                <p className="professional-review-note">
                  {plural(overview.risk_safety.reviews_due, "review")} due ·
                  Professional review required
                </p>
              )}
            </section>
          </div>

          <section className="current-care-section care-plan-position">
            <div className="current-care-section-heading">
              <div>
                <p className="current-care-eyebrow">Current plan position</p>
                <h3>Care Plan</h3>
              </div>
              <button
                type="button"
                className="btn btn-secondary btn-xs"
                onClick={onOpenCarePlan}
              >
                Open Care Plan
              </button>
            </div>
            {!overview.care_plan.exists ? (
              <p className="current-care-empty">
                No Care Plan has been started.
              </p>
            ) : (
              <div className="care-position-grid">
                <div>
                  <span>Status</span>
                  <strong>
                    {formatStatus(overview.care_plan.status)} — Plan{" "}
                    {overview.care_plan.version}
                  </strong>
                </div>
                <div>
                  <span>Effective from</span>
                  <strong>
                    {formatDate(overview.care_plan.effective_from)}
                  </strong>
                </div>
                <div>
                  <span>Next review</span>
                  <strong>{formatDate(overview.care_plan.review_date)}</strong>
                </div>
                <div>
                  <span>Latest review</span>
                  <strong>
                    {overview.care_plan.latest_review_outcome
                      ? `${formatStatus(overview.care_plan.latest_review_outcome)} · ${formatDate(overview.care_plan.latest_reviewed_at)}`
                      : "No review recorded"}
                  </strong>
                </div>
              </div>
            )}
            {overview.care_plan.exists && (
              <div className="care-position-notes">
                {overview.care_plan.sections_awaiting_confirmation > 0 && (
                  <span>
                    {plural(
                      overview.care_plan.sections_awaiting_confirmation,
                      "section",
                    )}{" "}
                    awaiting confirmation
                  </span>
                )}
                {overview.care_plan.sections_not_known > 0 && (
                  <span>
                    {plural(
                      overview.care_plan.sections_not_known,
                      "section",
                    )}{" "}
                    not known yet
                  </span>
                )}
                {overview.care_plan.successor_draft_exists && (
                  <span>
                    Successor draft · Plan{" "}
                    {overview.care_plan.successor_draft_version}
                  </span>
                )}
              </div>
            )}
          </section>

          <div className="current-care-two-column">
            <section className="current-care-section care-story-card">
              <div className="current-care-section-heading">
                <div>
                  <p className="current-care-eyebrow">Recent direct work</p>
                  <h3>Recent Key Working</h3>
                </div>
                <button
                  type="button"
                  className="text-button"
                  onClick={onOpenKeyworkNotes}
                >
                  Open Key Working
                </button>
              </div>
              {overview.recent_keyworking.length === 0 ? (
                <p className="current-care-empty">
                  No Key Working sessions are recorded.
                </p>
              ) : (
                <ul className="care-story-list">
                  {overview.recent_keyworking.map((session, index) => (
                    <li key={`${session.session_at}-${index}`}>
                      <strong>{formatDate(session.session_at)}</strong>
                      <span>
                        {session.target_label
                          ? `Target: ${session.target_label}`
                          : "No target linked"}
                      </span>
                      <span>
                        {session.domain
                          ? formatStatus(session.domain)
                          : "No domain recorded"}{" "}
                        ·{" "}
                        {session.signed_by_key_worker
                          ? "Signed by key worker"
                          : "Awaiting key-worker sign-off"}
                      </span>
                    </li>
                  ))}
                </ul>
              )}
              <button
                type="button"
                className="text-button section-footer-link"
                onClick={onOpenKeyworkHistory}
              >
                View Key Working history →
              </button>
            </section>

            <section className="current-care-section care-story-card voice-overview">
              <div className="current-care-section-heading">
                <div>
                  <p className="current-care-eyebrow">
                    Kept distinct from staff interpretation
                  </p>
                  <h3>Young Person&apos;s Voice</h3>
                </div>
              </div>
              <strong className="voice-count">
                {plural(
                  overview.child_voice.recent_entry_count,
                  "recent check-in",
                )}
              </strong>
              <p>
                {overview.child_voice.latest_date
                  ? `Latest entry ${formatDate(overview.child_voice.latest_date)}`
                  : "No check-in is recorded yet."}
              </p>
              <button
                type="button"
                className="btn btn-secondary btn-xs"
                onClick={onOpenChildVoice}
              >
                Open Child Voice
              </button>
            </section>
          </div>

          <section className="current-care-section journey-section">
            <div className="current-care-section-heading">
              <div>
                <p className="current-care-eyebrow">Connected evidence story</p>
                <h3>Journey timeline</h3>
              </div>
            </div>
            {overview.timeline.length === 0 ? (
              <p className="current-care-empty">
                The journey timeline will grow as records are added.
              </p>
            ) : (
              <>
                <ol className="journey-timeline">
                  {timelineLead.map((event, index) => (
                    <li key={`${event.occurred_at}-${event.type}-${index}`}>
                      <time>{formatDate(event.occurred_at)}</time>
                      <div>
                        <strong>{event.title}</strong>
                        {event.detail && <p>{event.detail}</p>}
                        {sourceButton(event.source)}
                      </div>
                    </li>
                  ))}
                </ol>
                {timelineMore.length > 0 && (
                  <details className="journey-more">
                    <summary>Show {timelineMore.length} earlier events</summary>
                    <ol className="journey-timeline">
                      {timelineMore.map((event, index) => (
                        <li
                          key={`${event.occurred_at}-${event.type}-more-${index}`}
                        >
                          <time>{formatDate(event.occurred_at)}</time>
                          <div>
                            <strong>{event.title}</strong>
                            {event.detail && <p>{event.detail}</p>}
                            {sourceButton(event.source)}
                          </div>
                        </li>
                      ))}
                    </ol>
                  </details>
                )}
              </>
            )}
          </section>

          <nav
            className="current-care-secondary-nav"
            aria-label="Other child records"
          >
            <span>Other records</span>
            <button
              type="button"
              className="text-button"
              onClick={onOpenSummary}
            >
              Therapeutic summary
            </button>
            <button
              type="button"
              className="text-button"
              onClick={onOpenDailyProgress}
            >
              Daily Progress
            </button>
            <button
              type="button"
              className="text-button"
              onClick={onOpenScoresTrends}
            >
              Scores &amp; trends
            </button>
          </nav>
        </>
      )}
    </section>
  );
};

export default ChildDashboardTab;
