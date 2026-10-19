import { useCallback, useEffect, useMemo, useState } from "react";
import { formatShortDateTime } from "@mycaresignals/shared/utilities/dateTime";
import { QR_API_BASE } from "../api/qrClient";
import "../styles/managerFeedback.css";

type Status = "new" | "acknowledged" | "no_action_required" | "follow_up_required" | "closed";
type Outcome = Exclude<Status, "new">;

type Review = {
  id: number; reviewer: string; reviewed_at: string; outcome: Outcome;
  manager_note: string; follow_up_required: boolean; follow_up_status: string;
};

type FeedbackItem = {
  id: string; received_at: string; source_type: string; rating?: number | null;
  recommend: boolean; current_status: Status; follow_up_status: string;
  reviewed_at?: string | null; requires_human_review: boolean; review_priority: string;
};

type FeedbackDetail = FeedbackItem & {
  reason?: string | null; what_went_well: string; what_to_improve?: string | null;
  answers?: Record<string, unknown>; matched_rule_ids?: string[]; rule_version?: string; reviews?: Review[];
};

type InboxSummary = { responses: number; needs_review: number; follow_up: number; reviewed: number };
type MonthlyReport = {
  month: string; total_responses: number; by_source: Record<string, number>; average_rating?: number | null;
  recommendation_count: number;
  went_well: Array<{ received_at: string; source_type: string; rating?: number | null; text: string }>;
  areas_for_improvement: Array<{ received_at: string; source_type: string; text: string }>;
  follow_up: { required: number; open: number; closed: number };
  review_prompts: { total: number; by_status: Record<string, number> };
};

const sourceLabels: Record<string, string> = {
  Parent: "Parent / Carer", SocialWorker: "Social Worker", Professional: "Professional",
  Ofsted: "Ofsted / Reg 44", Other: "Other",
};

const statusLabels: Record<string, string> = {
  new: "Needs review", acknowledged: "Acknowledged", no_action_required: "No action required",
  follow_up_required: "Follow-up required", closed: "Closed",
};

const outcomes: Array<{ value: Outcome; label: string }> = [
  { value: "acknowledged", label: "Acknowledge" },
  { value: "no_action_required", label: "No action required" },
  { value: "follow_up_required", label: "Follow-up required" },
  { value: "closed", label: "Close" },
];

const currentMonth = () => new Date().toISOString().slice(0, 7);

export default function ManagerFeedbackPage() {
  const [token, setToken] = useState(() => sessionStorage.getItem("qr-admin-token") || "");
  const [tokenInput, setTokenInput] = useState("");
  const [items, setItems] = useState<FeedbackItem[]>([]);
  const [summary, setSummary] = useState<InboxSummary>({ responses: 0, needs_review: 0, follow_up: 0, reviewed: 0 });
  const [selected, setSelected] = useState<FeedbackDetail | null>(null);
  const [month, setMonth] = useState(currentMonth);
  const [source, setSource] = useState("");
  const [status, setStatus] = useState("");
  const [requiresReview, setRequiresReview] = useState("");
  const [outcome, setOutcome] = useState<Outcome>("acknowledged");
  const [managerNote, setManagerNote] = useState("");
  const [report, setReport] = useState<MonthlyReport | null>(null);
  const [view, setView] = useState<"inbox" | "report">("inbox");
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const query = useMemo(() => {
    const params = new URLSearchParams();
    if (month) params.set("month", month);
    if (source) params.set("source", source);
    if (status) params.set("status", status);
    if (requiresReview) params.set("requires_review", requiresReview);
    return params.toString();
  }, [month, source, status, requiresReview]);

  const adminFetch = useCallback((path: string, init?: RequestInit) => fetch(`${QR_API_BASE}${path}`, {
    ...init,
    headers: { "X-Admin-Token": token, ...(init?.body ? { "Content-Type": "application/json" } : {}), ...init?.headers },
  }), [token]);

  const handleUnauthorized = useCallback((response: Response) => {
    if (response.status !== 401) return false;
    sessionStorage.removeItem("qr-admin-token");
    setToken("");
    setError("Management access expired. Enter the admin token again.");
    return true;
  }, []);

  const loadInbox = useCallback(async () => {
    if (!token) return;
    setLoading(true); setError("");
    try {
      const response = await adminFetch(`/api/v1/admin/feedback?${query}`);
      if (handleUnauthorized(response)) return;
      if (!response.ok) throw new Error("Could not load feedback inbox");
      const payload = await response.json() as { items: FeedbackItem[]; summary: InboxSummary };
      const nextItems = payload.items || [];
      setItems(nextItems); setSummary(payload.summary);
      setSelected((current) => current && nextItems.some((item) => item.id === current.id) ? current : null);
    } catch (caught) { setError(caught instanceof Error ? caught.message : "Could not load feedback inbox"); }
    finally { setLoading(false); }
  }, [adminFetch, handleUnauthorized, query, token]);

  useEffect(() => { void loadInbox(); }, [loadInbox]);

  const openDetail = async (id: string) => {
    setError("");
    const response = await adminFetch(`/api/v1/admin/feedback/${id}`);
    if (handleUnauthorized(response)) return;
    if (!response.ok) { setError("Could not load this response"); return; }
    setSelected(await response.json() as FeedbackDetail);
  };

  const saveReview = async () => {
    if (!selected || managerNote.trim().length < 3) { setError("Add a short manager note."); return; }
    setSaving(true); setError("");
    try {
      const response = await adminFetch(`/api/v1/admin/feedback/${selected.id}/reviews`, {
        method: "POST", body: JSON.stringify({ outcome, manager_note: managerNote.trim() }),
      });
      if (handleUnauthorized(response)) return;
      if (!response.ok) throw new Error("Could not record the management action");
      setManagerNote(""); setOutcome("acknowledged");
      await loadInbox(); await openDetail(selected.id);
    } catch (caught) { setError(caught instanceof Error ? caught.message : "Could not record the management action"); }
    finally { setSaving(false); }
  };

  const exportCurrentView = async () => {
    const response = await adminFetch(`/api/v1/admin/feedback/export?${query}`);
    if (handleUnauthorized(response)) return;
    if (!response.ok) { setError("Could not export this view"); return; }
    const url = URL.createObjectURL(await response.blob());
    const link = document.createElement("a"); link.href = url; link.download = `visitor-feedback-${month || "filtered"}.csv`; link.click();
    URL.revokeObjectURL(url);
  };

  const loadReport = async () => {
    setView("report"); setError("");
    const response = await adminFetch(`/api/v1/admin/feedback/monthly-report?month=${encodeURIComponent(month)}`);
    if (handleUnauthorized(response)) return;
    if (!response.ok) { setError("Could not build the monthly report"); return; }
    setReport(await response.json() as MonthlyReport);
  };

  if (!token) return (
    <section className="manager-login">
      <h1>Management access</h1><p>Enter the configured admin token to open the feedback workspace.</p>
      {error && <p className="manager-error">{error}</p>}
      <form onSubmit={(event) => { event.preventDefault(); if (!tokenInput.trim()) return; sessionStorage.setItem("qr-admin-token", tokenInput.trim()); setToken(tokenInput.trim()); }}>
        <label>Admin token<input type="password" value={tokenInput} onChange={(event) => setTokenInput(event.target.value)} autoComplete="current-password" /></label>
        <button type="submit">Open workspace</button>
      </form>
      <p className="manager-auth-note">This service currently uses one static admin token and cannot identify individual reviewers.</p>
    </section>
  );

  return (
    <section className="manager-workspace">
      <header className="manager-heading">
        <div><p className="manager-eyebrow">Management workspace</p><h1>Visitor &amp; Professional Feedback</h1><p>Review feedback about the home, record follow-up and export evidence for monitoring and reporting.</p></div>
        <div className="manager-heading-actions"><button onClick={() => setView("inbox")}>Inbox</button><button onClick={() => void loadReport()}>Monthly report</button><button className="quiet" onClick={() => { sessionStorage.removeItem("qr-admin-token"); setToken(""); }}>Lock</button></div>
      </header>
      {error && <p className="manager-error">{error}</p>}

      {view === "inbox" ? <>
        <div className="manager-summary">
          <div><span>Responses this month</span><strong>{summary.responses}</strong></div><div><span>Needs review</span><strong>{summary.needs_review}</strong></div>
          <div><span>Follow-up</span><strong>{summary.follow_up}</strong></div><div><span>Reviewed</span><strong>{summary.reviewed}</strong></div>
        </div>
        <div className="manager-filters">
          <label>Month<input type="month" value={month} onChange={(event) => setMonth(event.target.value)} /></label>
          <label>Source<select value={source} onChange={(event) => setSource(event.target.value)}><option value="">All source types</option>{Object.entries(sourceLabels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label>
          <label>Status<select value={status} onChange={(event) => setStatus(event.target.value)}><option value="">All statuses</option>{Object.entries(statusLabels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label>
          <label>Review prompt<select value={requiresReview} onChange={(event) => setRequiresReview(event.target.value)}><option value="">All responses</option><option value="true">Prompt raised</option><option value="false">No prompt</option></select></label>
          <button onClick={() => void exportCurrentView()}>Export filtered view</button>
        </div>
        <div className="manager-queue-detail">
          <div className="manager-queue" aria-label="Feedback queue">
            {loading && <p>Loading…</p>}{!loading && items.length === 0 && <p>No feedback matches these filters.</p>}
            {items.map((item) => <article className={selected?.id === item.id ? "queue-item selected" : "queue-item"} key={item.id}>
              <div><strong>{sourceLabels[item.source_type] || item.source_type}</strong><span>{formatShortDateTime(item.received_at)}</span><span>{item.rating ? `Rating ${item.rating}/5` : "Rating not supplied"}</span><span className="status-badge">{statusLabels[item.current_status]}</span>{item.requires_human_review && <span className={`prompt ${item.review_priority}`}>{item.review_priority === "urgent" ? "Urgent review" : item.review_priority === "priority" ? "Priority review" : "Review suggested"}</span>}</div>
              <button onClick={() => void openDetail(item.id)}>View</button>
            </article>)}
          </div>
          <div className="manager-detail">
            {!selected ? <p>Select one response to review it.</p> : <>
              <header><div><p className="manager-eyebrow">{sourceLabels[selected.source_type] || selected.source_type}</p><h2>Feedback received</h2><p>{formatShortDateTime(selected.received_at)} · {selected.rating ? `Rating ${selected.rating}/5` : "No rating"}</p></div><span className="status-badge">{statusLabels[selected.current_status]}</span></header>
              {selected.requires_human_review && <div className={`review-prompt ${selected.review_priority}`}>{selected.review_priority === "urgent" ? "Urgent review" : selected.review_priority === "priority" ? "Priority review" : "Review suggested"}. This is a management prompt, not a child risk level.</div>}
              {selected.reason && <section><h3>Reason</h3><p>{selected.reason}</p></section>}
              <section><h3>What went well</h3><p>{selected.what_went_well}</p></section>
              {selected.what_to_improve && <section><h3>Area for improvement</h3><p>{selected.what_to_improve}</p></section>}
              <section><h3>Review history</h3>{!selected.reviews?.length ? <p>No management actions recorded.</p> : <ol className="review-history">{selected.reviews.map((review) => <li key={review.id}><strong>{statusLabels[review.outcome]}</strong><span>{formatShortDateTime(review.reviewed_at)} · {review.reviewer}</span><p>{review.manager_note}</p></li>)}</ol>}</section>
              <section className="manager-action"><h3>Record management action</h3><label>Outcome<select value={outcome} onChange={(event) => setOutcome(event.target.value as Outcome)}>{outcomes.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}</select></label><label>Manager note<textarea value={managerNote} onChange={(event) => setManagerNote(event.target.value)} placeholder={outcome === "follow_up_required" ? "Record the follow-up needed." : "Record the management decision."} /></label><button disabled={saving} onClick={() => void saveReview()}>{saving ? "Recording…" : "Record action"}</button></section>
              <details><summary>Technical review details</summary><p>Rule set: {selected.rule_version || "Not recorded"}</p><p>Matched prompts: {selected.matched_rule_ids?.join(", ") || "None"}</p></details>
            </>}
          </div>
        </div>
      </> : <MonthlyReportView month={month} report={report} onMonthChange={setMonth} onRefresh={loadReport} />}
    </section>
  );
}

function MonthlyReportView({ month, report, onMonthChange, onRefresh }: { month: string; report: MonthlyReport | null; onMonthChange: (value: string) => void; onRefresh: () => void }) {
  return <div className="monthly-report">
    <div className="report-controls"><label>Report month<input type="month" value={month} onChange={(event) => onMonthChange(event.target.value)} /></label><button onClick={onRefresh}>Refresh report</button><button onClick={() => window.print()}>Print</button></div>
    {!report ? <p>Loading report…</p> : <>
      <header><h2>Monthly feedback report · {report.month}</h2><p>Deterministic management evidence from source records. No automated judgement is applied.</p></header>
      <div className="report-grid"><section><h3>Month summary</h3><p><strong>{report.total_responses}</strong> responses</p><p>Average rating: {report.average_rating == null ? "Not available" : `${report.average_rating.toFixed(1)}/5`}</p><p>Would recommend: {report.recommendation_count}</p>{Object.entries(report.by_source).map(([source, count]) => <p key={source}>{sourceLabels[source] || source}: {count}</p>)}</section><section><h3>Follow-up</h3><p>Required: {report.follow_up.required}</p><p>Open: {report.follow_up.open}</p><p>Completed/closed: {report.follow_up.closed}</p></section><section><h3>Review prompts</h3><p>Prompts raised: {report.review_prompts.total}</p>{Object.entries(report.review_prompts.by_status).map(([status, count]) => <p key={status}>{statusLabels[status] || status}: {count}</p>)}</section></div>
      <section><h3>What people said went well</h3>{report.went_well.length === 0 ? <p>No reviewed positive entries for this month.</p> : report.went_well.map((entry, index) => <article className="report-entry" key={`${entry.received_at}-${index}`}><strong>{sourceLabels[entry.source_type] || entry.source_type} · {formatShortDateTime(entry.received_at)}</strong><p>{entry.text}</p></article>)}</section>
      <section><h3>Areas for improvement</h3>{report.areas_for_improvement.length === 0 ? <p>No improvement entries for this month.</p> : report.areas_for_improvement.map((entry, index) => <article className="report-entry" key={`${entry.received_at}-${index}`}><strong>{sourceLabels[entry.source_type] || entry.source_type} · {formatShortDateTime(entry.received_at)}</strong><p>{entry.text}</p></article>)}</section>
    </>}
  </div>;
}
