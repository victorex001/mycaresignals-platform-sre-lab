// src/DailyScoresTab.tsx
import React, { useEffect, useMemo, useState } from "react";
import type {
  KeyworkSession, // daily score sessions
  TimelineItem, // per-target timeline entries
  ChildTarget,
  ChildDomainIndex,
} from "./types";

import {
  ResponsiveContainer,
  LineChart,
  Line,
  CartesianGrid,
  XAxis,
  YAxis,
  Tooltip,
  Legend,
} from "recharts";

import { computeSafeguardingFlag } from "./safeguarding";
import { apiFetch } from "./api";

type InsightsTab = "trends" | "domains" | "history";
type ChartGranularity = "daily" | "weekly";

type Props = {
  apiBase: string;
  childId: number;
  childInitials: string;
  defaultInsightsTab?: InsightsTab;
};

const MS_PER_DAY = 24 * 60 * 60 * 1000;
const diffInDays = (a: Date, b: Date): number =>
  Math.floor((a.getTime() - b.getTime()) / MS_PER_DAY);

const DailyScoresTab: React.FC<Props> = ({
  apiBase,
  childId,
  childInitials,
  defaultInsightsTab = "trends",
}) => {
  const [error, setError] = useState<string | null>(null);

  const [sessions, setSessions] = useState<KeyworkSession[]>([]);
  const [timeline, setTimeline] = useState<TimelineItem[]>([]);
  const [targets, setTargets] = useState<ChildTarget[]>([]);
  const [domainIndex, setDomainIndex] = useState<ChildDomainIndex | null>(
    null,
  );
  const [loadingHistory, setLoadingHistory] = useState(false);

  const [insightsTab, setInsightsTab] =
    useState<InsightsTab>(defaultInsightsTab);

  // Target filter for chart
  const [selectedTargetId, setSelectedTargetId] = useState<number | "all">(
    "all",
  );

  // Date-range filter for chart
  const [chartStartDate, setChartStartDate] = useState<string>(""); // YYYY-MM-DD
  const [chartEndDate, setChartEndDate] = useState<string>(""); // YYYY-MM-DD

  // NEW: simple toggle between daily & weekly view
  const [chartGranularity, setChartGranularity] =
    useState<ChartGranularity>("daily");

  // ---------- Load history ----------
  const loadHistory = async () => {
    setLoadingHistory(true);
    setError(null);

    try {
      const [sessRes, tlRes, targetsRes, domainRes] = await Promise.all([
        apiFetch(`${apiBase}/v1/children/${childId}/sessions`),
        apiFetch(`${apiBase}/v1/children/${childId}/timeline`),
        apiFetch(`${apiBase}/v1/children/${childId}/targets`),
        apiFetch(`${apiBase}/v1/children/${childId}/domain_index?window_days=28`),
      ]);

      if (!sessRes.ok) throw new Error("Failed to load sessions");
      if (!tlRes.ok) throw new Error("Failed to load timeline");
      if (!targetsRes.ok) throw new Error("Failed to load targets");

      const sessData: KeyworkSession[] = await sessRes.json();
      const tlData: TimelineItem[] = await tlRes.json();
      const targetsData: ChildTarget[] = await targetsRes.json();

      setSessions(Array.isArray(sessData) ? sessData : []);
      setTimeline(Array.isArray(tlData) ? tlData : []);
      setTargets(Array.isArray(targetsData) ? targetsData : []);

      if (domainRes.ok) {
        const idxData: ChildDomainIndex = await domainRes.json();
        setDomainIndex(idxData);
      } else {
        setDomainIndex(null);
      }
    } catch (err: any) {
      setError(err.message || "Could not load daily scores history");
      setSessions([]);
      setTimeline([]);
      setTargets([]);
      setDomainIndex(null);
    } finally {
      setLoadingHistory(false);
    }
  };

  useEffect(() => {
    loadHistory();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [childId]);

  // ---------- Quick date-range presets ----------
  const applyRangePreset = (days: number) => {
    const today = new Date();
    const end = today.toISOString().slice(0, 10);

    const startDate = new Date();
    startDate.setDate(today.getDate() - (days - 1));
    const start = startDate.toISOString().slice(0, 10);

    setChartStartDate(start);
    setChartEndDate(end);
  };

  const clearRange = () => {
    setChartStartDate("");
    setChartEndDate("");
  };

  // ---------- Chart data (avg per day, filtered by target + date range) ----------
  type ChartPoint = {
    dateKey: string; // YYYY-MM-DD
    dateLabel: string; // e.g. "24 Nov" or "Wk of 24 Nov"
    wellbeing: number;
    progress: number;
  };

  const dailyChartData: ChartPoint[] = useMemo(() => {
    if (!timeline || timeline.length === 0) return [];

    const startFilter = chartStartDate ? new Date(chartStartDate) : null;
    const endFilter = chartEndDate ? new Date(chartEndDate) : null;
    if (endFilter) {
      // include whole end day
      endFilter.setHours(23, 59, 59, 999);
    }

    const byDate = new Map<
      string,
      { label: string; wellbeing: number; progress: number; count: number }
    >();

    timeline.forEach((item) => {
      // target filter
      if (selectedTargetId !== "all" && item.target_id !== selectedTargetId) {
        return;
      }

      const d = new Date(item.session_at);
      if (isNaN(d.getTime())) return;

      // date-range filter
      if (startFilter && d < startFilter) return;
      if (endFilter && d > endFilter) return;

      const key = d.toISOString().slice(0, 10); // YYYY-MM-DD
      const label = d.toLocaleDateString(undefined, {
        month: "short",
        day: "numeric",
      });

      const current = byDate.get(key);
      if (current) {
        current.wellbeing += item.wellbeing_score;
        current.progress += item.progress_score;
        current.count += 1;
      } else {
        byDate.set(key, {
          label,
          wellbeing: item.wellbeing_score,
          progress: item.progress_score,
          count: 1,
        });
      }
    });

    return Array.from(byDate.entries())
      .sort((a, b) => (a[0] < b[0] ? -1 : 1))
      .map(([key, v]) => ({
        dateKey: key,
        dateLabel: v.label,
        wellbeing: v.count ? v.wellbeing / v.count : 0,
        progress: v.count ? v.progress / v.count : 0,
      }));
  }, [timeline, selectedTargetId, chartStartDate, chartEndDate]);

  // ---------- Weekly-aggregated chart data ----------
  const weeklyChartData: ChartPoint[] = useMemo(() => {
    if (!dailyChartData.length) return [];

    const byWeek = new Map<
      string,
      { label: string; wellbeing: number; progress: number; count: number }
    >();

    dailyChartData.forEach((p) => {
      const d = new Date(p.dateKey);
      if (isNaN(d.getTime())) return;

      // get Monday of this week
      const day = d.getDay(); // 0 = Sun, 1 = Mon
      const diff = (day + 6) % 7; // days since Monday
      d.setDate(d.getDate() - diff);

      const weekKey = d.toISOString().slice(0, 10);
      const weekLabelDate = d.toLocaleDateString(undefined, {
        month: "short",
        day: "numeric",
      });

      const current = byWeek.get(weekKey);
      if (current) {
        current.wellbeing += p.wellbeing;
        current.progress += p.progress;
        current.count += 1;
      } else {
        byWeek.set(weekKey, {
          label: `Wk of ${weekLabelDate}`,
          wellbeing: p.wellbeing,
          progress: p.progress,
          count: 1,
        });
      }
    });

    return Array.from(byWeek.entries())
      .sort((a, b) => (a[0] < b[0] ? -1 : 1))
      .map(([key, v]) => ({
        dateKey: key,
        dateLabel: v.label,
        wellbeing: v.count ? v.wellbeing / v.count : 0,
        progress: v.count ? v.progress / v.count : 0,
      }));
  }, [dailyChartData]);

  const chartData: ChartPoint[] =
    chartGranularity === "daily" ? dailyChartData : weeklyChartData;

  const selectedTarget: ChildTarget | null = useMemo(() => {
    if (selectedTargetId === "all") return null;
    return targets.find((t) => t.id === selectedTargetId) || null;
  }, [selectedTargetId, targets]);

  // ---------- Data quality (how many days in the span are scored) ----------
  const dataQuality = useMemo(() => {
    if (!timeline.length) return null;

    const uniqueDates = new Set<string>();
    let minDate: Date | null = null;
    let maxDate: Date | null = null;

    timeline.forEach((item) => {
      // respect target filter
      if (selectedTargetId !== "all" && item.target_id !== selectedTargetId) {
        return;
      }
      const d = new Date(item.session_at);
      if (isNaN(d.getTime())) return;

      const key = d.toISOString().slice(0, 10);
      uniqueDates.add(key);

      if (!minDate || d < minDate) minDate = d;
      if (!maxDate || d > maxDate) maxDate = d;
    });

    if (!minDate || !maxDate) return null;

    const totalDays = diffInDays(maxDate, minDate) + 1;
    if (totalDays <= 0) return null;

    const scoredDays = uniqueDates.size;
    const pct = Math.round((scoredDays / totalDays) * 100);

    return { scoredDays, totalDays, pct };
  }, [timeline, selectedTargetId]);

  // ---------- Summary for staff (plain-English stats) ----------
  const chartSummary = useMemo(() => {
    if (!chartData.length) return null;

    let wellbeingSum = 0;
    let progressSum = 0;
    let lowDays = 0;

    chartData.forEach((p) => {
      wellbeingSum += p.wellbeing;
      progressSum += p.progress;
      if (p.wellbeing <= 1 || p.progress <= 1) {
        lowDays += 1;
      }
    });

    const totalDays = chartData.length;
    const avgWellbeing = wellbeingSum / totalDays;
    const avgProgress = progressSum / totalDays;

    return { totalDays, avgWellbeing, avgProgress, lowDays };
  }, [chartData]);

  // ---------- Safeguarding flag ----------
  const safeguardingFlag = useMemo(
    () => computeSafeguardingFlag(domainIndex),
    [domainIndex],
  );

  const renderTargetOptionLabel = (t: ChildTarget) => {
    const base = t.short_label || `Target #${t.id}`;
    const domainPart = t.domain ? ` · ${t.domain}` : "";
    return `${base}${domainPart} [#${t.id}]`;
  };

  const formatIndex = (n: number | null | undefined) =>
    n === null || n === undefined ? "—" : n.toFixed(1);

  // ---------- UI ----------
  return (
    <section className="card">
      <div className="card-header">
        <div>
          <h2>Scores &amp; timeline</h2>
          <p className="card-subtitle">
            Review recorded Key Working scores, trends, and domain insights for
            child <strong>{childInitials}</strong> · ID {childId}. New sessions
            are recorded in Key Working.
          </p>
        </div>
      </div>

      {error && (
        <div className="alert alert-error">
          <strong>Error:</strong> {error}
        </div>
      )}

      {/* ---------- Insights sub-tabs ---------- */}
      <div className="card card-sub" style={{ marginTop: "1rem" }}>
        <div className="insights-tabs" style={{ marginBottom: "1rem" }}>
          <button
            type="button"
            className={
              insightsTab === "trends"
                ? "btn btn-secondary btn-xs active"
                : "btn btn-ghost btn-xs"
            }
            onClick={() => setInsightsTab("trends")}
          >
            Trends
          </button>
          <button
            type="button"
            className={
              insightsTab === "domains"
                ? "btn btn-secondary btn-xs active"
                : "btn btn-ghost btn-xs"
            }
            style={{ marginLeft: "0.5rem" }}
            onClick={() => setInsightsTab("domains")}
          >
            Domain summary
          </button>
          <button
            type="button"
            className={
              insightsTab === "history"
                ? "btn btn-secondary btn-xs active"
                : "btn btn-ghost btn-xs"
            }
            style={{ marginLeft: "0.5rem" }}
            onClick={() => setInsightsTab("history")}
          >
            History
          </button>
        </div>

        {/* --- Trends tab: progress chart --- */}
        {insightsTab === "trends" && (
          <>
            {/* Target filter */}
            <div
              className="form-row-2"
              style={{ marginBottom: "0.75rem", gap: "1rem" }}
            >
              <div className="form-group" style={{ flex: 1 }}>
                <label className="label">Show scores for</label>
                <select
                  className="input"
                  value={
                    selectedTargetId === "all"
                      ? "all"
                      : String(selectedTargetId)
                  }
                  onChange={(e) => {
                    const val = e.target.value;
                    if (val === "all") {
                      setSelectedTargetId("all");
                    } else {
                      setSelectedTargetId(Number(val));
                    }
                  }}
                >
                  <option value="all">All targets</option>
                  {targets.map((t) => (
                    <option key={t.id} value={t.id}>
                      {renderTargetOptionLabel(t)}
                    </option>
                  ))}
                </select>
              </div>

              {/* Date range filter */}
              <div className="form-group" style={{ flex: 2 }}>
                <label className="label">Date range (for chart)</label>
                <div
                  className="form-row-2"
                  style={{ alignItems: "center", gap: "0.5rem" }}
                >
                  <input
                    type="date"
                    className="input"
                    value={chartStartDate}
                    onChange={(e) => setChartStartDate(e.target.value)}
                  />
                  <span style={{ textAlign: "center" }}>to</span>
                  <input
                    type="date"
                    className="input"
                    value={chartEndDate}
                    onChange={(e) => setChartEndDate(e.target.value)}
                  />
                </div>
                <div
                  style={{
                    marginTop: "0.4rem",
                    display: "flex",
                    gap: "0.5rem",
                    flexWrap: "wrap",
                  }}
                >
                  <button
                    type="button"
                    className="btn btn-ghost btn-xs"
                    onClick={() => applyRangePreset(7)}
                  >
                    Last 7 days
                  </button>
                  <button
                    type="button"
                    className="btn btn-ghost btn-xs"
                    onClick={() => applyRangePreset(28)}
                  >
                    Last 28 days
                  </button>
                  <button
                    type="button"
                    className="btn btn-ghost btn-xs"
                    onClick={clearRange}
                  >
                    Reset
                  </button>
                </div>

                {/* NEW: daily vs weekly toggle */}
                <div
                  style={{
                    marginTop: "0.4rem",
                    display: "flex",
                    gap: "0.5rem",
                    flexWrap: "wrap",
                  }}
                >
                  <button
                    type="button"
                    className={
                      chartGranularity === "daily"
                        ? "btn btn-secondary btn-xs"
                        : "btn btn-ghost btn-xs"
                    }
                    onClick={() => setChartGranularity("daily")}
                  >
                    Daily view
                  </button>
                  <button
                    type="button"
                    className={
                      chartGranularity === "weekly"
                        ? "btn btn-secondary btn-xs"
                        : "btn btn-ghost btn-xs"
                    }
                    onClick={() => setChartGranularity("weekly")}
                  >
                    Weekly (simple) view
                  </button>
                </div>
              </div>
            </div>

            {/* Data quality line */}
            {dataQuality && (
              <p className="field-hint" style={{ marginTop: "0.4rem" }}>
                <strong>Data quality:</strong> {dataQuality.scoredDays} of{" "}
                {dataQuality.totalDays} days scored (
                {dataQuality.pct}
                %).
              </p>
            )}

            {/* Simple text summary for staff */}
            {chartSummary && (
              <p className="field-hint" style={{ marginTop: "0.4rem" }}>
                <strong>
                  In this view ({chartGranularity === "daily" ? "per day" : "per week"}
                  ):
                </strong>{" "}
                average wellbeing{" "}
                {chartSummary.avgWellbeing.toFixed(1)}/4, average progress{" "}
                {chartSummary.avgProgress.toFixed(1)}/4, across{" "}
                {chartSummary.totalDays}{" "}
                {chartGranularity === "daily" ? "scored days" : "weeks"}.
                {chartSummary.lowDays > 0
                  ? ` There are ${chartSummary.lowDays} ${
                      chartGranularity === "daily" ? "days" : "weeks"
                    } where scores were 0–1 – worth checking the notes for what was happening.`
                  : " No very low (0–1) periods in this range."}
              </p>
            )}

            <h3>
              Progress chart{" "}
              {selectedTarget
                ? `(target: ${
                    selectedTarget.short_label || `Target #${selectedTarget.id}`
                  })`
                : "(all targets)"}
            </h3>
            <p className="field-hint">
              <strong>X-axis</strong>:{" "}
              {chartGranularity === "daily"
                ? "date of scored days."
                : "week (average of scored days in that week)."}{" "}
              <strong>Y-axis</strong>: average score (0–4). Blue line =
              wellbeing, green line = progress.
            </p>
            {/* New helper copy for staff */}
            <div className="field-hint" style={{ marginTop: "0.35rem" }}>
              <strong>How to use this graph (for staff)</strong>
              <ul style={{ marginTop: "0.25rem", paddingLeft: "1.2rem" }}>
                <li>Don&apos;t worry about every up and down – look at the line over weeks.</li>
                <li>Higher numbers (3–4) = doing well for this target.</li>
                <li>
                  Lower numbers (0–1) = struggling – think about what support helped or is
                  needed.
                </li>
                <li>
                  If you see several low days in a row, talk about this in key-working and
                  with your manager.
                </li>
              </ul>
            </div>
            {loadingHistory && <p className="field-hint">Loading…</p>}
            {!loadingHistory && chartData.length === 0 && (
              <p className="field-hint">
                No scores for this combination of target and date range yet.
              </p>
            )}
            {!loadingHistory && chartData.length > 0 && (
              <div style={{ width: "100%", height: 380, marginTop: "0.75rem" }}>
                <ResponsiveContainer width="100%" height="100%">
                  <LineChart
                    data={chartData}
                    margin={{ top: 8, right: 16, bottom: 8, left: 0 }}
                  >
                    <CartesianGrid strokeDasharray="3 3" />
                    <XAxis dataKey="dateLabel" />
                    <YAxis domain={[0, 4]} />
                    <Tooltip />
                    <Legend />
                    <Line
                      type="monotone"
                      dataKey="wellbeing"
                      name="Wellbeing"
                      stroke="#3b82f6"
                      dot
                    />
                    <Line
                      type="monotone"
                      dataKey="progress"
                      name="Progress"
                      stroke="#22c55e"
                      dot
                    />
                  </LineChart>
                </ResponsiveContainer>
              </div>
            )}
          </>
        )}

        {/* --- Domain summary tab --- */}
        {insightsTab === "domains" && (
          <>
            {domainIndex && safeguardingFlag.level !== "none" && (
              <div
                className={`alert ${
                  safeguardingFlag.level === "red"
                    ? "alert-error"
                    : "alert-warning"
                }`}
                style={{ marginBottom: "1rem" }}
              >
                <strong>
                  {safeguardingFlag.level === "red"
                    ? "Safeguarding trend alert: "
                    : "Early warning: "}
                </strong>
                {safeguardingFlag.headline}
                {safeguardingFlag.reasons.length > 0 && (
                  <ul style={{ marginTop: "0.5rem", paddingLeft: "1.2rem" }}>
                    {safeguardingFlag.reasons.map((reason, i) => (
                      <li key={i}>{reason}</li>
                    ))}
                  </ul>
                )}
              </div>
            )}

            <h3>Domain summary (last {domainIndex?.window_days ?? 28} days)</h3>

            {domainIndex && (
              <p className="field-hint">
                Overall Pathway Index:{" "}
                <strong>{formatIndex(domainIndex.overall.index)}</strong>{" "}
                {domainIndex.overall.change !== undefined &&
                  domainIndex.overall.change !== null && (
                    <>
                      {" "}
                      (change{" "}
                      {domainIndex.overall.change >= 0 ? "+" : ""}
                      {formatIndex(domainIndex.overall.change)})
                    </>
                  )}
              </p>
            )}

            {(!domainIndex || domainIndex.domains.length === 0) && (
              <p className="field-hint">
                No domain scores yet — log some scores to see this summary.
              </p>
            )}

            {domainIndex && domainIndex.domains.length > 0 && (
              <table className="simple-table">
                <thead>
                  <tr>
                    <th>Domain</th>
                    <th>Avg wellbeing</th>
                    <th>Avg progress</th>
                    <th>Index</th>
                    <th>Change vs prev window</th>
                  </tr>
                </thead>
                <tbody>
                  {domainIndex.domains.map((d) => (
                    <tr key={d.domain}>
                      <td>{d.domain}</td>
                      <td>{formatIndex(d.average_wellbeing)}</td>
                      <td>{formatIndex(d.average_progress)}</td>
                      <td>{formatIndex(d.index)}</td>
                      <td>
                        {d.change === undefined || d.change === null
                          ? "—"
                          : `${d.change >= 0 ? "+" : ""}${formatIndex(
                              d.change,
                            )}`}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </>
        )}

        {/* --- History tab --- */}
        {insightsTab === "history" && (
          <>
            <h3>Recent scored days</h3>
            {loadingHistory && <p className="field-hint">Loading…</p>}
            {!loadingHistory && sessions.length === 0 && (
              <p className="field-hint">
                No scored Key Working sessions have been recorded yet.
              </p>
            )}
            {sessions.length > 0 && (
              <ul className="doc-list" style={{ marginBottom: "1.5rem" }}>
                {sessions.slice(0, 10).map((s) => (
                  <li key={s.id} className="doc-list-item">
                    <strong>
                      {new Date(s.session_at).toLocaleDateString(undefined, {
                        dateStyle: "medium",
                      })}
                    </strong>{" "}
                    – {s.staff_name || "Keyworker"}
                    {s.notes && (
                      <div className="field-hint">
                        <em>Key-working note:</em> {s.notes}
                      </div>
                    )}
                  </li>
                ))}
              </ul>
            )}

            <h3>Timeline (scores)</h3>
            {loadingHistory && <p className="field-hint">Loading…</p>}
            {!loadingHistory && timeline.length === 0 && (
              <p className="field-hint">
                No scores yet. Scores recorded in Key Working will appear here.
              </p>
            )}
            {timeline.length > 0 && (
              <ul className="doc-list">
                {timeline.map((item, idx) => (
                  <li key={idx} className="doc-list-item">
                    <strong>
                      {new Date(item.session_at).toLocaleDateString()} –{" "}
                      {item.target_name} ({item.target_domain})
                    </strong>
                    <div className="field-hint">
                      Wellbeing {item.wellbeing_score} /4 · Progress{" "}
                      {item.progress_score} /4
                    </div>
                    {item.comment && (
                      <div className="field-hint">{item.comment}</div>
                    )}
                  </li>
                ))}
              </ul>
            )}
          </>
        )}
      </div>
    </section>
  );
};

export default DailyScoresTab;
