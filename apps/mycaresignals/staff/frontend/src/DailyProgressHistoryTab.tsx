import { useEffect, useMemo, useState } from "react";
import { apiFetch } from "./api";
import { CANONICAL_DOMAINS } from "./framework";
import type { ChildDailyProgress } from "./types";

type Props = {
  apiBase: string;
  childId: number;
  childInitials: string;
  onBackToDashboard: () => void;
  onOpenDailyProgress: () => void;
};

const displayDate = (value: string) => {
  const date = new Date(`${value.slice(0, 10)}T12:00:00`);
  return Number.isNaN(date.getTime())
    ? value
    : date.toLocaleDateString(undefined, { dateStyle: "medium" });
};

const scoreLabel = (value?: number | null) =>
  value == null ? "Not recorded" : `${value} / 4`;

export default function DailyProgressHistoryTab({
  apiBase,
  childId,
  childInitials,
  onBackToDashboard,
  onOpenDailyProgress,
}: Props) {
  const [entries, setEntries] = useState<ChildDailyProgress[]>([]);
  const [domain, setDomain] = useState("");
  const [month, setMonth] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    void Promise.resolve()
      .then(async () => {
        if (cancelled) return;
        setLoading(true);
        setError(null);
        setEntries([]);
        const response = await apiFetch(`${apiBase}/v1/children/${childId}/daily-progress?limit=100`);
        if (!response.ok) throw new Error("Daily Progress history could not be loaded.");
        const payload = await response.json() as { items: ChildDailyProgress[] };
        if (!cancelled) setEntries(Array.isArray(payload.items) ? payload.items : []);
      })
      .catch((caught) => {
        if (!cancelled) {
          setEntries([]);
          setError(caught instanceof Error ? caught.message : "Daily Progress history could not be loaded.");
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => { cancelled = true; };
  }, [apiBase, childId]);

  const filteredEntries = useMemo(
    () => entries.filter((entry) =>
      (!domain || entry.domain === domain)
      && (!month || entry.observed_on.slice(0, 7) === month),
    ),
    [domain, entries, month],
  );

  return (
    <section className="card daily-history-page">
      <header className="card-header daily-progress-header">
        <div>
          <span className="care-eyebrow">Daily care evidence · {childInitials}</span>
          <h2>Daily Progress history</h2>
          <p className="card-subtitle">View Daily Progress history.</p>
        </div>
        <div className="care-header-actions">
          <button type="button" className="btn btn-ghost" onClick={onBackToDashboard}>← Child Home</button>
          <button type="button" className="btn btn-secondary" onClick={onOpenDailyProgress}>Daily Progress →</button>
        </div>
      </header>

      {error && <div className="alert alert-error">{error}</div>}

      <div className="daily-history-filters" aria-label="Filter Daily Progress history">
        <div className="form-group">
          <label className="label" htmlFor="daily-history-domain">Care domain</label>
          <select id="daily-history-domain" className="input" value={domain} onChange={(event) => setDomain(event.target.value)}>
            <option value="">All domains</option>
            {CANONICAL_DOMAINS.map((item) => <option key={item.key} value={item.key}>{item.key}</option>)}
          </select>
        </div>
        <div className="form-group">
          <label className="label" htmlFor="daily-history-month">Month observed</label>
          <input id="daily-history-month" className="input" type="month" value={month} onChange={(event) => setMonth(event.target.value)} />
        </div>
        {(domain || month) && (
          <button type="button" className="btn btn-ghost" onClick={() => { setDomain(""); setMonth(""); }}>Clear filters</button>
        )}
      </div>

      {loading && <p className="field-hint">Loading Daily Progress history…</p>}
      {!loading && filteredEntries.length === 0 && <p className="current-care-empty">No Daily Progress records match these filters.</p>}
      <div className="daily-history-list">
        {filteredEntries.map((entry) => (
          <article key={entry.id} className="daily-entry-card">
            <div className="daily-entry-heading"><strong>{displayDate(entry.observed_on)}</strong><span>{entry.domain}</span></div>
            {entry.target_label && <p className="daily-target-link">Target · {entry.target_label}</p>}
            <div className="daily-score-pair">
              <span>Observed wellbeing <strong>{scoreLabel(entry.wellbeing_score)}</strong></span>
              <span>Observed progress <strong>{scoreLabel(entry.progress_score)}</strong></span>
            </div>
            {entry.observation_note && <p>{entry.observation_note}</p>}
            {entry.young_person_comment && <blockquote>Young person&apos;s words: {entry.young_person_comment}</blockquote>}
            <small>Recorded by {entry.recorded_by_staff_name}</small>
          </article>
        ))}
      </div>
    </section>
  );
}
