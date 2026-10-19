// frontend/src/components/VisitorFeedbackForm.tsx
import { useState } from "react";
import type { MessageResponse } from "@mycaresignals/shared/types/http";
import { postJson } from "../config/api";
import type { HomeConfig } from "../config/homeConfig";

type Props = {
  home: HomeConfig;
  contextToken: string;
};

type WhoOption = "Parent" | "SocialWorker" | "Ofsted" | "Professional" | "Other";

const WHO_OPTIONS: { value: WhoOption; label: string }[] = [
  { value: "Parent",       label: "Parent / Carer" },
  { value: "SocialWorker", label: "Social worker" },
  { value: "Ofsted",       label: "Ofsted / Reg 44" },
  { value: "Professional", label: "Other professional" },
  { value: "Other",        label: "Other" },
];

export default function VisitorFeedbackForm({ home, contextToken }: Props) {
  const [who, setWho] = useState<WhoOption | "">("");
  const [reason, setReason] = useState("");
  const [overallRating, setOverallRating] = useState<string>(""); // "" = no rating
  const [whatWentWell, setWhatWentWell] = useState("");
  const [whatToImprove, setWhatToImprove] = useState("");
  const [recommend, setRecommend] = useState(false);
  const [contactName, setContactName] = useState("");
  const [contactEmail, setContactEmail] = useState("");
  const [contactWanted, setContactWanted] = useState(false);

  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setMessage(null);
    setError(null);

    if (!who) {
      setError("Please tell us who you are.");
      return;
    }
    if (!whatWentWell.trim()) {
      setError("Please tell us what went well.");
      return;
    }

    setLoading(true);

    try {
      const ratingNum =
        overallRating && overallRating !== "none"
          ? Number(overallRating)
          : undefined;

      const body: any = {
        context_token: contextToken,
        who,
        reason: reason.trim() || "",
        rating: ratingNum,
        what_went_well: whatWentWell.trim(),
        what_to_improve: whatToImprove.trim() || "",
        recommend,
        contact_name: contactName.trim() || "",
        contact_email: contactEmail.trim() || "",

        // structured extras – keep empty for this visitor form
        house_rating: undefined,
        staff_rating: undefined,
        felt_safe: "",
        open_comments: "",
        contact_wanted: contactWanted,
      };

      const res = await postJson<MessageResponse & {
        feedback_id: string;
      }>("/api/v1/feedback", body);

      setMessage(res.message || "Thank you for your feedback.");

      // reset
      setWho("");
      setReason("");
      setOverallRating("");
      setWhatWentWell("");
      setWhatToImprove("");
      setRecommend(false);
      setContactName("");
      setContactEmail("");
      setContactWanted(false);
    } catch (err: any) {
      setError(err.message || "Something went wrong. Please try again.");
    } finally {
      setLoading(false);
    }
  }

  const homeDisplay = `${home.name} (${home.provider}) – QR Feedback`;

  return (
    <section className="vf-card">
      <header className="vf-header">
        <h1 className="vf-title">Share your feedback</h1>
        <p className="vf-subtitle">
          You’re giving feedback about <strong>{home.name}</strong>{" "}
          ({home.provider}).
        </p>
      </header>

      {message && <div className="vf-alert vf-alert-success">{message}</div>}
      {error && <div className="vf-alert vf-alert-error">{error}</div>}

      <form className="vf-form" onSubmit={handleSubmit}>
        {/* Home – set automatically */}
        <div className="field">
          <label className="field-label">
            Home
            <span className="field-optional"> – set automatically</span>
          </label>
          <input
            className="field-input"
            type="text"
            value={homeDisplay}
            disabled
            readOnly
          />
          <p className="field-helper">
            This is set automatically based on where you scanned the QR code.
          </p>
        </div>

        {/* Who are you? */}
        <div className="field">
          <label className="field-label">Who are you?</label>
          <select
            className="field-input"
            value={who}
            onChange={(e) => setWho(e.target.value as WhoOption | "")}
          >
            <option value="">Select one…</option>
            {WHO_OPTIONS.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
        </div>

        {/* What is this feedback about? */}
        <div className="field">
          <label className="field-label">
            What is this feedback about?
            <span className="field-optional"> (optional)</span>
          </label>
          <input
            className="field-input"
            type="text"
            placeholder="e.g. staff support, activities, communication…"
            value={reason}
            onChange={(e) => setReason(e.target.value)}
          />
        </div>

        {/* Overall rating */}
        <div className="field">
          <label className="field-label">
            Overall rating (1–5)
            <span className="field-optional"> (optional)</span>
          </label>
          <select
            className="field-input"
            value={overallRating}
            onChange={(e) => setOverallRating(e.target.value)}
          >
            <option value="">No rating</option>
            <option value="1">1 – Very poor</option>
            <option value="2">2 – Poor</option>
            <option value="3">3 – OK</option>
            <option value="4">4 – Good</option>
            <option value="5">5 – Excellent</option>
          </select>
        </div>

        {/* What went well? */}
        <div className="field">
          <label className="field-label">
            What went well? <span className="field-required">*</span>
          </label>
          <textarea
            className="field-textarea"
            placeholder="Tell us what worked well or made a positive difference."
            value={whatWentWell}
            onChange={(e) => setWhatWentWell(e.target.value)}
          />
        </div>

        {/* What could we improve? */}
        <div className="field">
          <label className="field-label">
            What could we improve?
            <span className="field-optional"> (optional)</span>
          </label>
          <textarea
            className="field-textarea"
            placeholder="If there’s anything that could be better, please share it here."
            value={whatToImprove}
            onChange={(e) => setWhatToImprove(e.target.value)}
          />
        </div>

        {/* Recommendation */}
        <div className="field">
          <label className="field-label">Recommendation</label>
          <label className="checkbox">
            <input
              type="checkbox"
              checked={recommend}
              onChange={(e) => setRecommend(e.target.checked)}
            />
            Would you recommend this home/service to others?
          </label>
        </div>

        {/* Name + email row */}
        <div className="vf-grid-2">
          <div className="field">
            <label className="field-label">
              Your name <span className="field-optional">(optional)</span>
            </label>
            <input
              className="field-input"
              type="text"
              placeholder="Only if you’re happy to share"
              value={contactName}
              onChange={(e) => setContactName(e.target.value)}
            />
          </div>

          <div className="field">
            <label className="field-label">
              Your email <span className="field-optional">(optional)</span>
            </label>
            <input
              className="field-input"
              type="email"
              placeholder="We’ll only use this to follow up if needed"
              value={contactEmail}
              onChange={(e) => setContactEmail(e.target.value)}
            />
          </div>
        </div>

        <div className="field">
          <label className="checkbox">
            <input
              type="checkbox"
              checked={contactWanted}
              onChange={(e) => setContactWanted(e.target.checked)}
            />
            I would like someone to contact me about this feedback.
          </label>
        </div>

        <div className="form-actions">
          <button className="btn-primary" type="submit" disabled={loading}>
            {loading ? "Submitting…" : "Submit feedback"}
          </button>
        </div>
      </form>
    </section>
  );
}
