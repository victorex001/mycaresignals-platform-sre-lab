import { useEffect, useMemo, useState } from "react";
import { apiFetch } from "./api";
import { CANONICAL_DOMAINS } from "./framework";
import type { CanonicalDomain } from "./framework";
import type { MonthlyPerspectiveComparison, MonthlyScoreStatistics } from "./types";

type Props = {
  apiBase: string;
  childId: number;
  childInitials: string;
  onBackToDashboard: () => void;
  onOpenDailyProgress: () => void;
  onOpenDailyHistory: () => void;
  onOpenKeywork: () => void;
  onOpenMonthlyVoice: (receiptId: number) => void;
};

const PERSISTENT_DIFFERENCE_WORDING = "Persistent difference in perspectives — review suggested.";

const displayMonth = (value: string) => {
  const date = new Date(`${value}-01T12:00:00`);
  return Number.isNaN(date.getTime())
    ? value
    : date.toLocaleDateString(undefined, { month: "long", year: "numeric" });
};

const displayDate = (value: string) => {
  const date = new Date(value);
  return Number.isNaN(date.getTime())
    ? value
    : date.toLocaleDateString(undefined, { dateStyle: "medium" });
};

const statisticValue = (statistics: MonthlyScoreStatistics, label: string) => {
  if (statistics.count === 0 || statistics.median == null || statistics.min == null || statistics.max == null) {
    return <p>{label}: <strong>No staff ratings</strong></p>;
  }
  const range = statistics.min === statistics.max ? `${statistics.min}` : `${statistics.min}–${statistics.max}`;
  return <p>{label}: <strong>median {statistics.median} · range {range}</strong></p>;
};

const recentMonthSequence = () => {
  const current = new Date();
  const last = new Date(Date.UTC(current.getUTCFullYear(), current.getUTCMonth(), 1));
  const first = new Date(last);
  first.setUTCMonth(first.getUTCMonth() - 11);
  const result: string[] = [];
  for (const cursor = new Date(first); cursor <= last; cursor.setUTCMonth(cursor.getUTCMonth() + 1)) {
    result.push(cursor.toISOString().slice(0, 7));
  }
  return result;
};

type GraphPoint = {
  month: string;
  x: number;
  y: number | null;
  value: number | null;
};

const graphValue = (value?: number | null) => (
  typeof value === "number" && Number.isFinite(value) ? value : null
);

const graphY = (value: number | null) => {
  if (value == null) return null;
  const clamped = Math.max(0, Math.min(4, value));
  return 154 - (clamped / 4) * 112;
};

const lineSegments = (points: GraphPoint[]) => {
  const segments: Array<[GraphPoint, GraphPoint]> = [];
  for (let index = 1; index < points.length; index += 1) {
    const previous = points[index - 1];
    const current = points[index];
    if (previous.y != null && current.y != null) segments.push([previous, current]);
  }
  return segments;
};

const formatGraphValue = (value: number | null) => (
  value == null ? "No evidence" : value.toFixed(value % 1 === 0 ? 0 : 1)
);

function ProgressEvidenceGraph({
  months,
  byMonth,
}: {
  months: string[];
  byMonth: Map<string, MonthlyPerspectiveComparison>;
}) {
  const width = 720;
  const height = 220;
  const left = 48;
  const right = 24;
  const pointGap = months.length > 1 ? (width - left - right) / (months.length - 1) : 0;
  const xFor = (index: number) => left + pointGap * index;
  const hasEvidence = (item?: MonthlyPerspectiveComparison) => Boolean(
    item && (
      item.observation_count > 0
      || item.young_person_rating != null
      || item.keyworking_session_count > 0
    ),
  );
  const series = [
    {
      key: "daily-progress",
      label: "Daily Progress",
      className: "daily-progress",
      points: months.map((month, index) => {
        const value = graphValue(byMonth.get(month)?.progress.median);
        return { month, x: xFor(index), y: graphY(value), value };
      }),
    },
    {
      key: "daily-wellbeing",
      label: "Staff wellbeing",
      className: "daily-wellbeing",
      points: months.map((month, index) => {
        const value = graphValue(byMonth.get(month)?.wellbeing.median);
        return { month, x: xFor(index), y: graphY(value), value };
      }),
    },
    {
      key: "young-person",
      label: "Young Person Voice",
      className: "young-person",
      points: months.map((month, index) => {
        const value = graphValue(byMonth.get(month)?.young_person_rating);
        return { month, x: xFor(index), y: graphY(value), value };
      }),
    },
  ];

  return (
    <section className="progress-evidence-chart" aria-labelledby="progress-chart-heading">
      <div className="progress-chart-heading-row">
        <div>
          <p className="current-care-eyebrow">Evidence timeline</p>
          <h3 id="progress-chart-heading">Recorded evidence only — no filled-in gaps</h3>
        </div>
        <div className="progress-chart-legend" aria-label="Chart legend">
          {series.map((entry) => <span key={entry.key} className={entry.className}>{entry.label}</span>)}
          <span className="keyworking">Key Working</span>
        </div>
      </div>
      <div className="progress-chart-scroll" role="img" aria-label="Monthly evidence graph for the selected care domain">
        <svg viewBox={`0 0 ${width} ${height}`} className="progress-chart-svg" aria-hidden="true">
          {[0, 1, 2, 3, 4].map((rating) => {
            const y = graphY(rating) ?? 154;
            return (
              <g key={rating}>
                <line x1={left} x2={width - right} y1={y} y2={y} className="progress-chart-grid" />
                <text x={18} y={y + 4} className="progress-chart-axis">{rating}</text>
              </g>
            );
          })}
          {months.map((month, index) => {
            const x = xFor(index);
            const item = byMonth.get(month);
            return (
              <g key={month}>
                <line x1={x} x2={x} y1={34} y2={166} className={hasEvidence(item) ? "progress-chart-month-line has-evidence" : "progress-chart-month-line"} />
                {item?.keyworking_session_count ? (
                  <rect
                    x={x - 7}
                    y={174 - Math.min(item.keyworking_session_count, 4) * 8}
                    width="14"
                    height={Math.min(item.keyworking_session_count, 4) * 8}
                    rx="4"
                    className="progress-chart-keyworking-bar"
                  />
                ) : null}
                <text x={x} y={204} className="progress-chart-month-label">
                  {displayMonth(month).replace(" ", "\n")}
                </text>
              </g>
            );
          })}
          {series.map((entry) => (
            <g key={entry.key} className={`progress-chart-series ${entry.className}`}>
              {lineSegments(entry.points).map(([from, to]) => (
                <line key={`${from.month}-${to.month}`} x1={from.x} y1={from.y!} x2={to.x} y2={to.y!} />
              ))}
              {entry.points.map((point) => (
                point.y == null ? null : <circle key={point.month} cx={point.x} cy={point.y} r="5.5" />
              ))}
            </g>
          ))}
        </svg>
      </div>
      <div className="progress-chart-month-summaries">
        {months.map((month) => {
          const item = byMonth.get(month);
          return (
            <div key={month} className={hasEvidence(item) ? "has-evidence" : ""}>
              <strong>{displayMonth(month)}</strong>
              {!item && <span>No evidence recorded this month</span>}
              {item && (
                <span>
                  Progress {formatGraphValue(graphValue(item.progress.median))}
                  {" · "}Voice {formatGraphValue(graphValue(item.young_person_rating))}
                  {" · "}Keywork {item.keyworking_session_count}
                </span>
              )}
            </div>
          );
        })}
      </div>
      <p className="field-hint">
        Daily Progress, Key Working and Young Person Voice are shown as separate evidence streams. Missing months stay empty.
      </p>
    </section>
  );
}

export default function ProgressOverTimeTab({
  apiBase,
  childId,
  childInitials,
  onBackToDashboard,
  onOpenDailyProgress,
  onOpenDailyHistory,
  onOpenKeywork,
  onOpenMonthlyVoice,
}: Props) {
  const [items, setItems] = useState<MonthlyPerspectiveComparison[]>([]);
  const [selectedDomain, setSelectedDomain] = useState<CanonicalDomain>(CANONICAL_DOMAINS[0].key);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const months = useMemo(() => recentMonthSequence(), []);

  useEffect(() => {
    let cancelled = false;
    void Promise.resolve()
      .then(async () => {
        if (cancelled) return;
        setLoading(true);
        setError(null);
        setItems([]);
        const response = await apiFetch(`${apiBase}/v1/children/${childId}/daily-progress/monthly?from=${months[0]}&to=${months.at(-1)}`);
        if (!response.ok) throw new Error("Progress Over Time could not be loaded.");
        const payload = await response.json() as { items: MonthlyPerspectiveComparison[] };
        if (!cancelled) setItems(Array.isArray(payload.items) ? payload.items : []);
      })
      .catch((caught) => {
        if (!cancelled) {
          setItems([]);
          setError(caught instanceof Error ? caught.message : "Progress Over Time could not be loaded.");
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => { cancelled = true; };
  }, [apiBase, childId, months]);

  const selectedItems = useMemo(
    () => items.filter((item) => item.domain === selectedDomain),
    [items, selectedDomain],
  );
  const byMonth = useMemo(
    () => new Map(selectedItems.map((item) => [item.period_month, item])),
    [selectedItems],
  );

  return (
    <section className="card progress-over-time-page">
      <header className="card-header daily-progress-header">
        <div>
          <span className="care-eyebrow">Connected evidence · {childInitials}</span>
          <h2>Progress Over Time</h2>
          <p className="card-subtitle">
            See staff observations, Young Person Monthly Voice and connected therapeutic evidence over time. Evidence sources remain separate.
          </p>
        </div>
        <div className="care-header-actions">
          <button type="button" className="btn btn-ghost" onClick={onBackToDashboard}>← Child Home</button>
          <button type="button" className="btn btn-secondary" onClick={onOpenDailyProgress}>Daily Progress →</button>
        </div>
      </header>

      {error && <div className="alert alert-error">{error}</div>}

      <section className="progress-domain-picker" aria-labelledby="progress-domain-heading">
        <div>
          <p className="current-care-eyebrow">One domain at a time</p>
          <h3 id="progress-domain-heading">Choose a care domain</h3>
        </div>
        <div className="progress-domain-options">
          {CANONICAL_DOMAINS.map((domain) => (
            <button
              key={domain.key}
              type="button"
              className={selectedDomain === domain.key ? "is-selected" : ""}
              aria-pressed={selectedDomain === domain.key}
              onClick={() => setSelectedDomain(domain.key)}
            >
              {domain.key}
            </button>
          ))}
        </div>
      </section>

      <section className="progress-domain-timeline">
        <div className="progress-selected-domain">
          <p className="current-care-eyebrow">Selected domain</p>
          <h3>{selectedDomain}</h3>
        </div>
        {loading && <p className="field-hint">Loading progress patterns…</p>}
        {!loading && selectedItems.length === 0 && (
          <div className="current-care-empty">
            <strong>No evidence recorded for this domain in the last 12 months.</strong>
            <p>Missing months are shown explicitly below. No values are estimated or carried forward.</p>
          </div>
        )}
        {!loading && !error && <ProgressEvidenceGraph months={months} byMonth={byMonth} />}
        {!loading && !error && <div className="progress-month-list">
          {months.map((month) => {
            const item = byMonth.get(month);
            return (
              <article key={month} className={`progress-month-row ${item?.persistent_review_suggested ? "review-suggested" : ""}`}>
                <header>
                  <h4>{displayMonth(month)}</h4>
                  {item?.persistent_review_suggested && <span className="review-suggested-label">Review suggested</span>}
                </header>
                {!item && <div className="progress-missing-month"><strong>No evidence recorded this month</strong><span>No Daily Progress, verified Monthly Voice or Key Working record is available. No data has been interpolated.</span></div>}
                {item && <div className="progress-evidence-streams">
                  <section>
                    <span className="perspective-label">Staff Daily Progress</span>
                    <strong>{`${item.observation_count} observation${item.observation_count === 1 ? "" : "s"}`}</strong>
                    {statisticValue(item.progress, "Progress")}
                    {statisticValue(item.wellbeing, "Wellbeing")}
                    <button type="button" className="text-button" onClick={onOpenDailyHistory}>View Daily Progress history</button>
                  </section>
                  <section className="young-perspective">
                    <span className="perspective-label">Young Person Monthly Voice</span>
                    <strong>{item?.young_person_rating == null ? "No verified monthly rating" : `${item.young_person_rating} / 4`}</strong>
                    {item?.young_person_submitted_at && <p>Submitted {displayDate(item.young_person_submitted_at)}</p>}
                    {item?.young_person_source_receipt_id && (
                      <button type="button" className="text-button" onClick={() => onOpenMonthlyVoice(item.young_person_source_receipt_id!)}>View Monthly Voice</button>
                    )}
                  </section>
                  <section>
                    <span className="perspective-label">Key Working</span>
                    <strong>{item.keyworking_session_count > 0 ? `${item.keyworking_session_count} relevant session${item.keyworking_session_count === 1 ? "" : "s"}` : "No Key Working evidence this month"}</strong>
                    <button type="button" className="text-button" onClick={onOpenKeywork}>View Key Working history</button>
                  </section>
                </div>}
                {item?.comparison_status === "not_enough_comparable_data" && <p className="field-hint">Not enough comparable data yet.</p>}
                {item?.comparison_status === "conflicting_monthly_voice_records" && <p className="perspective-review-note">Monthly voice records conflict. Human review is required before comparison.</p>}
                {item?.persistent_review_suggested && <p className="perspective-review-note">{PERSISTENT_DIFFERENCE_WORDING}</p>}
              </article>
            );
          })}
        </div>}
      </section>
    </section>
  );
}
