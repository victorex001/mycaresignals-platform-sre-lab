import { useEffect, useMemo, useState } from "react";
import type { FC } from "react";
import { apiFetch } from "./api";
import KeyworkSessionAccordion from "./KeyworkSessionAccordion";
import type {
  ChildGoal,
  ChildTarget,
  KeyworkManagerReview,
  KeyworkSession,
} from "./types";

type Props = {
  apiBase: string;
  childId: number;
  childInitials: string;
  onBackToKeywork: () => void;
  onBackToDashboard: () => void;
};

const errorMessage = (error: unknown, fallback: string): string =>
  error instanceof Error ? error.message : fallback;

const formatDate = (value: string): string => {
  const [year, month, day] = value.slice(0, 10).split("-").map(Number);
  if (!year || !month || !day) return value;
  return new Date(year, month - 1, day).toLocaleDateString(undefined, {
    dateStyle: "medium",
  });
};

const KeyworkHistoryTab: FC<Props> = ({
  apiBase,
  childId,
  childInitials,
  onBackToKeywork,
  onBackToDashboard,
}) => {
  const [sessions, setSessions] = useState<KeyworkSession[]>([]);
  const [targets, setTargets] = useState<ChildTarget[]>([]);
  const [goals, setGoals] = useState<ChildGoal[]>([]);
  const [targetFilter, setTargetFilter] = useState("");
  const [domainFilter, setDomainFilter] = useState("");
  const [dateFrom, setDateFrom] = useState("");
  const [dateTo, setDateTo] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    const loadHistory = async () => {
      setLoading(true);
      setError(null);
      try {
        const [sessionsResponse, targetsResponse, goalsResponse] = await Promise.all([
          apiFetch(`${apiBase}/v1/children/${childId}/sessions`),
          apiFetch(`${apiBase}/v1/children/${childId}/targets`),
          apiFetch(`${apiBase}/v1/children/${childId}/goals`),
        ]);
        if (!sessionsResponse.ok) {
          throw new Error("Failed to load Key Working history.");
        }
        if (!targetsResponse.ok) {
          throw new Error("Failed to load target information.");
        }
        if (!goalsResponse.ok) {
          throw new Error("Failed to load goal information.");
        }

        const sessionData: KeyworkSession[] = await sessionsResponse.json();
        const targetData: ChildTarget[] = await targetsResponse.json();
        const goalData: ChildGoal[] = await goalsResponse.json();
        if (!cancelled) {
          setSessions(Array.isArray(sessionData) ? sessionData : []);
          setTargets(Array.isArray(targetData) ? targetData : []);
          setGoals(Array.isArray(goalData) ? goalData : []);
        }
      } catch (caught: unknown) {
        if (!cancelled) {
          setError(errorMessage(caught, "Could not load Key Working history."));
          setSessions([]);
          setTargets([]);
          setGoals([]);
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    };

    loadHistory();
    return () => {
      cancelled = true;
    };
  }, [apiBase, childId]);

  const domains = useMemo(
    () =>
      Array.from(
        new Set(
          sessions
            .map((session) => session.primary_domain?.trim())
            .filter((domain): domain is string => Boolean(domain)),
        ),
      ).sort((left, right) => left.localeCompare(right)),
    [sessions],
  );

  const filteredSessions = useMemo(
    () =>
      sessions.filter((session) => {
        const sessionDate = session.session_at.slice(0, 10);
        if (targetFilter && session.target_id !== Number(targetFilter)) {
          return false;
        }
        if (domainFilter && session.primary_domain !== domainFilter) {
          return false;
        }
        if (dateFrom && sessionDate < dateFrom) return false;
        if (dateTo && sessionDate > dateTo) return false;
        return true;
      }),
    [sessions, targetFilter, domainFilter, dateFrom, dateTo],
  );

  const clearFilters = () => {
    setTargetFilter("");
    setDomainFilter("");
    setDateFrom("");
    setDateTo("");
  };

  const filtersActive = Boolean(targetFilter || domainFilter || dateFrom || dateTo);
  const awaitingManagerReview = sessions.filter(
    (session) =>
      session.signed_by_key_worker && !session.manager_review_decision,
  ).length;

  const handleManagerReview = (
    sessionId: number,
    review: KeyworkManagerReview,
  ) => {
    setSessions((current) =>
      current.map((session) =>
        session.id === sessionId
          ? {
              ...session,
              signed_by_manager: review.decision === "approved",
              manager_signed_at: review.reviewed_at,
              manager_review_decision: review.decision,
              manager_review_comment: review.comment,
              manager_reviewer_name: review.reviewer_name,
              can_manager_review: false,
            }
          : session,
      ),
    );
  };

  return (
    <section className="card keywork-history-dashboard">
      <div className="card-header keywork-history-dashboard-header">
        <div>
          <h2>Key Working History</h2>
          <p className="card-subtitle">
            Historical Key Working records for child{" "}
            <strong>{childInitials}</strong> · ID {childId}
          </p>
        </div>
        <div className="keywork-history-navigation">
          <button type="button" className="btn btn-primary" onClick={onBackToKeywork}>
            Record Key Working
          </button>
          <button
            type="button"
            className="btn btn-secondary"
            onClick={onBackToDashboard}
          >
            ← Child Home
          </button>
        </div>
      </div>

      {error && (
        <div className="alert alert-error">
          <strong>Error:</strong> {error}
        </div>
      )}

      <div className="keywork-history-overview" aria-label="History summary">
        <div>
          <span className="field-hint">Total sessions</span>
          <strong>{sessions.length}</strong>
        </div>
        <div>
          <span className="field-hint">Latest session</span>
          <strong>
            {sessions.length > 0 ? formatDate(sessions[0].session_at) : "None recorded"}
          </strong>
        </div>
        <div>
          <span className="field-hint">Awaiting manager review</span>
          <strong>{awaitingManagerReview}</strong>
        </div>
        {filtersActive && (
          <div>
            <span className="field-hint">Matching records</span>
            <strong>{filteredSessions.length}</strong>
          </div>
        )}
      </div>

      <div className="keywork-history-filters" aria-label="History filters">
        <div className="form-group">
          <label className="label" htmlFor="keywork-history-target-filter">
            Target
          </label>
          <select
            id="keywork-history-target-filter"
            className="input"
            value={targetFilter}
            onChange={(event) => setTargetFilter(event.target.value)}
          >
            <option value="">All targets</option>
            {targets.map((target) => (
              <option key={target.id} value={target.id}>
                {target.short_label || `Target #${target.id}`}
              </option>
            ))}
          </select>
        </div>
        <div className="form-group">
          <label className="label" htmlFor="keywork-history-domain-filter">
            Therapeutic domain
          </label>
          <select
            id="keywork-history-domain-filter"
            className="input"
            value={domainFilter}
            onChange={(event) => setDomainFilter(event.target.value)}
          >
            <option value="">All domains</option>
            {domains.map((domain) => (
              <option key={domain} value={domain}>
                {domain}
              </option>
            ))}
          </select>
        </div>
        <div className="form-group">
          <label className="label" htmlFor="keywork-history-date-from">
            Date from
          </label>
          <input
            id="keywork-history-date-from"
            type="date"
            className="input"
            value={dateFrom}
            onChange={(event) => setDateFrom(event.target.value)}
          />
        </div>
        <div className="form-group">
          <label className="label" htmlFor="keywork-history-date-to">
            Date to
          </label>
          <input
            id="keywork-history-date-to"
            type="date"
            className="input"
            value={dateTo}
            onChange={(event) => setDateTo(event.target.value)}
          />
        </div>
        <button
          type="button"
          className="btn btn-ghost"
          onClick={clearFilters}
          disabled={!filtersActive}
        >
          Clear filters
        </button>
      </div>

      <div className="keywork-history-archive">
        <div className="keywork-history-archive-heading">
          <h3>Historical records</h3>
          <span className="field-hint">
            Showing {filteredSessions.length} of {sessions.length}
          </span>
        </div>
        {loading && <p className="field-hint">Loading history…</p>}
        {!loading && sessions.length === 0 && (
          <p className="field-hint">No Key Working sessions recorded yet.</p>
        )}
        {!loading && sessions.length > 0 && filteredSessions.length === 0 && (
          <p className="field-hint">No sessions match the selected filters.</p>
        )}
        {filteredSessions.length > 0 && (
          <KeyworkSessionAccordion
            key={`${targetFilter}|${domainFilter}|${dateFrom}|${dateTo}`}
            sessions={filteredSessions}
            targets={targets}
            goals={goals}
            apiBase={apiBase}
            childId={childId}
            onManagerReview={handleManagerReview}
          />
        )}
      </div>
    </section>
  );
};

export default KeyworkHistoryTab;
