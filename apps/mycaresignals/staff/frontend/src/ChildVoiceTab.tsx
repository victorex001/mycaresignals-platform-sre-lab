import { useEffect, useMemo, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { formatShortDateTime } from "@mycaresignals/shared/utilities/dateTime";
import { apiFetch } from "./api";
import type { Child, ExternalFeedbackReceipt, FeedbackReviewOutcome } from "./types";
import { CANONICAL_DOMAINS } from "./framework";

type Props = {
  apiBase: string;
  homeName: string;
  child: Child;
  onBackToDashboard: () => void;
};

type MonthlyComparison = {
  period_month: string;
  domain: string;
  staff_descriptive_value?: number;
  young_person_rating?: number;
  perspective_difference?: number;
  comparison_status: string;
  persistent_review_suggested: boolean;
};

const domains = CANONICAL_DOMAINS.map((domain) => domain.key);

const outcomes: Array<{ value: FeedbackReviewOutcome; label: string }> = [
  { value: "acknowledge", label: "Acknowledge" },
  { value: "no_further_action", label: "No further action" },
  { value: "follow_up_required", label: "Follow-up required" },
  { value: "escalate_for_risk_review", label: "Request professional Risk review" },
];

const promptLabel = (rule: string) => {
  switch (rule) {
    case "safety_language_urgent": return "urgent safety language needs human review";
    case "safety_language_review": return "safety language needs human review";
    case "follow_up_requested": return "the young person asked to talk with someone";
    default: return "a human review prompt was raised";
  }
};

const monthLabel = (value?: string) => {
  if (!value) return "Month not recorded";
  const parsed = new Date(`${value}-01T00:00:00Z`);
  return parsed.toLocaleDateString(undefined, { month: "long", year: "numeric", timeZone: "UTC" });
};

const fieldLabel = (value: string) =>
  value.replace(/^q\d+_/, "").replaceAll("_", " ").replace(/\b\w/g, (letter) => letter.toUpperCase());

const voiceEntries = (payload?: Record<string, unknown> | null) => {
  const excluded = new Set([
    "context_token", "home", "young_person_alias", "period_month", "domain_ratings",
    "q9_want_talk", "q9_follow_up_preference", "all_answers_json",
  ]);
  return Object.entries(payload ?? {})
    .filter(([key, value]) => !excluded.has(key) && typeof value === "string" && value.trim()) as Array<[string, string]>;
};

export default function ChildVoiceTab({ apiBase, child, onBackToDashboard }: Props) {
  const [searchParams] = useSearchParams();
  const focusedReceipt = Number(searchParams.get("receipt") || 0);
  const [items, setItems] = useState<ExternalFeedbackReceipt[]>([]);
  const [selected, setSelected] = useState<ExternalFeedbackReceipt | null>(null);
  const [comparisons, setComparisons] = useState<MonthlyComparison[]>([]);
  const [canReview, setCanReview] = useState(false);
  const [outcome, setOutcome] = useState<FeedbackReviewOutcome>("acknowledge");
  const [rationale, setRationale] = useState("");
  const [loading, setLoading] = useState(true);
  const [detailLoading, setDetailLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const openDetail = async (id: number) => {
    setDetailLoading(true);
    setError(null);
    try {
      const response = await apiFetch(`${apiBase}/v1/children/${child.id}/monthly-voice/${id}`);
      if (!response.ok) throw new Error(`Could not load monthly voice (${response.status})`);
      setSelected(await response.json() as ExternalFeedbackReceipt);
    } catch (caught) {
      setSelected(null);
      setError(caught instanceof Error ? caught.message : "Could not load monthly voice");
    } finally {
      setDetailLoading(false);
    }
  };

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const [voiceResponse, comparisonResponse] = await Promise.all([
        apiFetch(`${apiBase}/v1/children/${child.id}/monthly-voice`),
        apiFetch(`${apiBase}/v1/children/${child.id}/daily-progress/monthly`),
      ]);
      if (!voiceResponse.ok || !comparisonResponse.ok) throw new Error("Could not load Young Person Voice");
      const voicePayload = await voiceResponse.json() as { items: ExternalFeedbackReceipt[] };
      const comparisonPayload = await comparisonResponse.json() as { items: MonthlyComparison[] };
      const nextItems = Array.isArray(voicePayload.items) ? voicePayload.items : [];
      setItems(nextItems);
      setComparisons(Array.isArray(comparisonPayload.items) ? comparisonPayload.items : []);
      const requested = focusedReceipt > 0 ? nextItems.find((item) => item.id === focusedReceipt) : undefined;
      if (requested) await openDetail(requested.id);
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Could not load Young Person Voice");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { void load(); }, [apiBase, child.id, focusedReceipt]);

  useEffect(() => {
    let cancelled = false;
    void apiFetch(`${apiBase}/v1/children/${child.id}/monthly-voice/review-access`)
      .then((response) => { if (!cancelled) setCanReview(response.ok); })
      .catch(() => { if (!cancelled) setCanReview(false); });
    return () => { cancelled = true; };
  }, [apiBase, child.id]);

  const selectedComparisons = useMemo(() => {
    if (!selected?.period_month) return [];
    return comparisons.filter((item) => item.period_month === selected.period_month);
  }, [comparisons, selected]);

  const comparisonFor = (domain: string) => selectedComparisons.find((item) => item.domain === domain);

  const submitReview = async () => {
    if (!selected || rationale.trim().length < 3) {
      setError("Add a short professional rationale before recording the review.");
      return;
    }
    setSaving(true);
    setError(null);
    try {
      const response = await apiFetch(`${apiBase}/v1/children/${child.id}/monthly-voice/${selected.id}/reviews`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ outcome, rationale: rationale.trim() }),
      });
      if (!response.ok) throw new Error((await response.text()) || `Review failed (${response.status})`);
      setOutcome("acknowledge");
      setRationale("");
      await load();
      await openDetail(selected.id);
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Could not record monthly voice review");
    } finally {
      setSaving(false);
    }
  };

  return (
    <section className="card feedback-review-page">
      <header className="card-header">
        <div>
          <p className="current-care-eyebrow">Voice &amp; progress</p>
          <h2>Young Person Voice</h2>
          <p className="card-subtitle">Verified monthly voice for <strong>{child.initials}</strong>, kept distinct from staff Daily Progress observations.</p>
        </div>
        <button type="button" className="btn btn-secondary btn-xs" onClick={onBackToDashboard}>← Child Home</button>
      </header>

      {error && <div className="alert alert-error">{error}</div>}
      {loading && <p className="field-hint">Loading Young Person Voice…</p>}
      {!loading && items.length === 0 && <div className="empty-state">No verified monthly voice has been received for this young person.</div>}

      <div className="feedback-master-detail">
        <div className="feedback-compact-list">
          {items.map((item) => (
            <article key={item.id} className={`feedback-compact-card ${selected?.id === item.id ? "is-selected" : ""}`}>
              <div>
                <span className="status-pill">Young Person Monthly Voice</span>
                <h3>{monthLabel(item.period_month)}</h3>
                <div className="voice-card-signals">
                  {item.follow_up_requested && <span className="status-pill status-pill-warning">Follow-up requested</span>}
                  {item.requires_human_review && <span className="status-pill status-pill-warning">Review suggested</span>}
                </div>
              </div>
              <button type="button" className="btn btn-secondary btn-xs" onClick={() => void openDetail(item.id)}>View monthly voice</button>
            </article>
          ))}
        </div>

        <div className="feedback-detail-panel">
          {!selected && !detailLoading && <div className="empty-state">Choose a month to read the young person’s voice and perspective context.</div>}
          {detailLoading && <p className="field-hint">Loading monthly voice…</p>}
          {selected && !detailLoading && (
            <>
              <header className="feedback-detail-heading"><div><p className="current-care-eyebrow">Young Person Monthly Voice</p><h3>{monthLabel(selected.period_month)}</h3><p>Submitted {formatShortDateTime(selected.received_at)}</p></div><span className={`status-pill ${selected.triage_status === "follow_up" ? "status-pill-warning" : ""}`}>{selected.triage_status.replaceAll("_", " ")}</span></header>

              {selected.follow_up_requested && <div className="feedback-review-prompt"><strong>Follow-up requested.</strong>{selected.follow_up_preference ? ` Preferred support: ${selected.follow_up_preference}.` : " Ask the young person how they would prefer to talk."}</div>}
              {selected.requires_human_review && <div className="feedback-review-prompt"><strong>Review suggested:</strong> {selected.matched_rule_ids.map(promptLabel).join("; ")}. This is not a Risk assessment or automated care decision.</div>}

              <section><h4>1. Young person’s voice</h4><dl className="feedback-source-fields">{voiceEntries(selected.raw_payload).map(([key, value]) => <div key={key}><dt>{fieldLabel(key)}</dt><dd>{value}</dd></div>)}</dl></section>

              <section><h4>2. Six domain ratings</h4><div className="voice-domain-grid">{domains.map((domain) => <div key={domain}><span>{domain}</span><strong>{selected.domain_ratings?.[domain] ?? "—"} / 4</strong></div>)}</div></section>

              <section><h4>3. Staff and young-person perspectives</h4><p className="field-hint">These perspectives remain separate. Differences are prompts for human review, not conclusions.</p><div className="voice-comparison-list">{domains.map((domain) => { const comparison = comparisonFor(domain); return <div key={domain}><strong>{domain}</strong><span>Staff observations: {comparison?.staff_descriptive_value == null ? "Not enough data" : `${comparison.staff_descriptive_value.toFixed(1)} / 4`}</span><span>Young person: {selected.domain_ratings?.[domain] ?? "—"} / 4</span>{comparison?.perspective_difference != null && <span>Difference: {comparison.perspective_difference.toFixed(1)}</span>}{comparison?.persistent_review_suggested && <em>Persistent difference in perspectives — review suggested.</em>}</div>; })}</div></section>

              {selected.reviews && selected.reviews.length > 0 && <section><h4>Review history</h4><div className="feedback-review-history">{selected.reviews.map((review) => <div key={review.id}><strong>{review.outcome.replaceAll("_", " ")}</strong><span>{formatShortDateTime(review.reviewed_at)} · {review.reviewer_display_name}</span><p>{review.rationale}</p></div>)}</div></section>}

              {canReview && <section className="feedback-action-panel"><h4>4. Follow-up / review action</h4><label className="form-group"><span className="label">Outcome</span><select className="input" value={outcome} onChange={(event) => setOutcome(event.target.value as FeedbackReviewOutcome)}>{outcomes.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}</select></label><label className="form-group"><span className="label">Professional rationale</span><textarea className="textarea" value={rationale} onChange={(event) => setRationale(event.target.value)} placeholder="Record the human follow-up or review decision." /></label><button type="button" className="btn btn-primary" disabled={saving} onClick={() => void submitReview()}>{saving ? "Recording…" : "Record review"}</button></section>}
            </>
          )}
        </div>
      </div>
    </section>
  );
}
