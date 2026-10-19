import { useEffect, useRef, useState } from "react";
import { postJson } from "../config/api";
import EmojiRating from "./EmojiRating";

type Props = { contextToken: string };

const CANONICAL_DOMAINS = {
  emotional: "Emotional & Psychological Wellbeing",
  behaviour: "Behaviour & Self-Regulation",
  relationships: "Relationships & Connection",
  dailyLiving: "Daily Living, Learning & Participation",
  safety: "Safety, Risk & Coping with Stress",
  future: "Skills, Identity & Future",
} as const;

type DomainName = (typeof CANONICAL_DOMAINS)[keyof typeof CANONICAL_DOMAINS];

const RATING_STEPS: Array<{ type: "rating"; domain: DomainName; label: string }> = [
  { type: "rating", domain: CANONICAL_DOMAINS.emotional, label: "How have you been feeling this month?" },
  { type: "rating", domain: CANONICAL_DOMAINS.safety, label: "How safe have you felt this month?" },
  { type: "rating", domain: CANONICAL_DOMAINS.behaviour, label: "Do you feel staff listen to you?" },
  { type: "rating", domain: CANONICAL_DOMAINS.relationships, label: "How have things been with people around you?" },
  { type: "rating", domain: CANONICAL_DOMAINS.dailyLiving, label: "How have school, college or your plans for the future been going?" },
  { type: "rating", domain: CANONICAL_DOMAINS.future, label: "Do you feel you get a say in things that matter to you?" },
];

type VoiceField =
  | "q1_overall" | "q1_best" | "q1_hardest"
  | "q2_safe" | "q2_unsafe" | "q2_support_who"
  | "q3_listened" | "q3_trusted_adult" | "q3_change_support"
  | "q4_peers" | "q4_unkind" | "q4_help_with_peers"
  | "q5_enjoy" | "q5_wish_more" | "q5_house_change"
  | "q6_education" | "q6_education_help"
  | "q7_contact_feelings" | "q7_contact_change"
  | "q8_say_in_decisions" | "q8_change_example"
  | "q9_worries" | "q9_one_change"
  | "q10_what_tell_ofsted" | "q10_never_forget";

type Step =
  | (typeof RATING_STEPS)[number]
  | { type: "text"; key: "q1_overall"; label: string }
  | { type: "follow-up" };

// A future optional rotating question can be inserted before the follow-up
// without changing the six required ratings or the secure submission model.
const CORE_STEPS: Step[] = [
  ...RATING_STEPS,
  { type: "text", key: "q1_overall", label: "Is there anything you want us to know?" },
  { type: "follow-up" },
];

const emptyVoice = (): Record<VoiceField, string> => ({
  q1_overall: "", q1_best: "", q1_hardest: "",
  q2_safe: "", q2_unsafe: "", q2_support_who: "",
  q3_listened: "", q3_trusted_adult: "", q3_change_support: "",
  q4_peers: "", q4_unkind: "", q4_help_with_peers: "",
  q5_enjoy: "", q5_wish_more: "", q5_house_change: "",
  q6_education: "", q6_education_help: "",
  q7_contact_feelings: "", q7_contact_change: "",
  q8_say_in_decisions: "", q8_change_example: "",
  q9_worries: "", q9_one_change: "",
  q10_what_tell_ofsted: "", q10_never_forget: "",
});

export default function YoungVoiceForm({ contextToken }: Props) {
  const [ratings, setRatings] = useState<Partial<Record<DomainName, number>>>({});
  const [voice, setVoice] = useState<Record<VoiceField, string>>(emptyVoice);
  const [wantsFollowUp, setWantsFollowUp] = useState<boolean | null>(null);
  const [followUpPreference, setFollowUpPreference] = useState("");
  const [stepIndex, setStepIndex] = useState(0);
  const [loading, setLoading] = useState(false);
  const [done, setDone] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const advanceTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const questionHeading = useRef<HTMLHeadingElement>(null);

  const currentStep = CORE_STEPS[stepIndex];
  const progress = ((stepIndex + 1) / CORE_STEPS.length) * 100;

  useEffect(() => {
    questionHeading.current?.focus();
  }, [stepIndex]);

  useEffect(() => () => {
    if (advanceTimer.current) clearTimeout(advanceTimer.current);
  }, []);

  const clearAdvanceTimer = () => {
    if (advanceTimer.current) clearTimeout(advanceTimer.current);
    advanceTimer.current = null;
  };

  const stepIsComplete = (step: Step) => {
    if (step.type === "rating") return ratings[step.domain] != null;
    if (step.type === "follow-up") return wantsFollowUp !== null;
    return true;
  };

  const next = () => {
    clearAdvanceTimer();
    setError(null);
    if (!stepIsComplete(currentStep)) return;
    setStepIndex((current) => Math.min(current + 1, CORE_STEPS.length - 1));
  };

  const back = () => {
    clearAdvanceTimer();
    setError(null);
    setStepIndex((current) => Math.max(current - 1, 0));
  };

  const chooseRating = (domain: DomainName, score: number) => {
    setRatings((current) => ({ ...current, [domain]: score }));
    setError(null);
    clearAdvanceTimer();
    advanceTimer.current = setTimeout(() => {
      setStepIndex((current) => current === stepIndex ? Math.min(current + 1, CORE_STEPS.length - 1) : current);
      advanceTimer.current = null;
    }, 700);
  };

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    if (currentStep.type !== "follow-up" || wantsFollowUp === null) return;
    setError(null);
    if (RATING_STEPS.some(({ domain }) => ratings[domain] == null)) {
      setError("A rating is missing. Go back and choose a face for each question before sending.");
      return;
    }
    setLoading(true);
    try {
      await postJson<{ receipt_id: string }>("/api/v1/young-voice", {
        context_token: contextToken,
        domain_ratings: ratings,
        ...voice,
        q9_want_talk: wantsFollowUp,
        q9_follow_up_preference: wantsFollowUp ? followUpPreference.trim() : "",
      });
      setDone(true);
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Something went wrong. Please try again.");
    } finally {
      setLoading(false);
    }
  }

  if (done) {
    return (
      <main className="yv-page">
        <section className="yv-card yv-thank-you">
          <div className="yv-celebration" aria-hidden="true">💙</div>
          <h1 className="yv-title">Thank you for telling us how things have been for you.</h1>
          <p className="yv-text">Your feedback has been sent.</p>
          {wantsFollowUp && <p className="yv-follow-up-confirmation">You’ve also asked for someone to talk to you.</p>}
        </section>
      </main>
    );
  }

  return (
    <main className="yv-page">
      <form className="yv-card" onSubmit={submit}>
        <header className="yv-header">
          <span className="yv-kicker">Your monthly voice</span>
          <div className="yv-progress-copy"><span>{stepIndex + 1} of {CORE_STEPS.length}</span><span>{Math.round(progress)}%</span></div>
          <div className="yv-progress-bar" role="progressbar" aria-valuemin={1} aria-valuemax={CORE_STEPS.length} aria-valuenow={stepIndex + 1} aria-label="Your progress">
            <div className="yv-progress-bar-inner" style={{ width: `${progress}%` }} />
          </div>
        </header>

        <section className="yv-slide" aria-live="polite">
          {currentStep.type === "rating" && (
            <div className="yv-question-panel">
              <h1 className="yv-question yv-visually-hidden" ref={questionHeading} tabIndex={-1}>{currentStep.label}</h1>
              <EmojiRating label={currentStep.label} value={ratings[currentStep.domain] ?? null} onChange={(score) => chooseRating(currentStep.domain, score)} required />
              <p className="yv-question-help yv-rating-help">Tap the face that feels closest. Your choice will move you on.</p>
            </div>
          )}

          {currentStep.type === "text" && (
            <div className="yv-question-panel">
              <h1 className="yv-question" ref={questionHeading} tabIndex={-1}>{currentStep.label}</h1>
              <p className="yv-question-help">You don’t have to write anything.</p>
              <label className="yv-visually-hidden" htmlFor={currentStep.key}>{currentStep.label}</label>
              <textarea id={currentStep.key} className="field-textarea yv-textarea" rows={6} value={voice[currentStep.key]} onChange={(event) => setVoice((current) => ({ ...current, [currentStep.key]: event.target.value }))} placeholder="Type here if you want to…" />
            </div>
          )}

          {currentStep.type === "follow-up" && (
            <div className="yv-question-panel">
              <h1 className="yv-question" ref={questionHeading} tabIndex={-1}>Would you like someone to talk to you about anything?</h1>
              <div className="yv-choice-row" role="group" aria-label="Would you like someone to talk to you?">
                <button type="button" className={`yv-choice-btn ${wantsFollowUp === true ? "yv-choice-btn-active" : ""}`} aria-pressed={wantsFollowUp === true} onClick={() => setWantsFollowUp(true)}>Yes</button>
                <button type="button" className={`yv-choice-btn ${wantsFollowUp === false ? "yv-choice-btn-active" : ""}`} aria-pressed={wantsFollowUp === false} onClick={() => setWantsFollowUp(false)}>No</button>
              </div>
              {wantsFollowUp && (
                <div className="yv-follow-up-reveal">
                  <label className="yv-follow-up-label" htmlFor="follow-up-preference">Who would you feel comfortable talking to?</label>
                  <span className="yv-question-help">This is optional.</span>
                  <input id="follow-up-preference" className="field-input yv-follow-up-input" value={followUpPreference} onChange={(event) => setFollowUpPreference(event.target.value)} placeholder="Type here if you want to…" />
                </div>
              )}
              {wantsFollowUp !== null && <p className="yv-ready-message">You’re ready to send your feedback.</p>}
            </div>
          )}

          {error && <div className="vf-alert vf-alert-error" role="alert">{error}</div>}
        </section>

        <footer className="yv-actions">
          <button type="button" className="yv-btn-secondary" onClick={back} disabled={stepIndex === 0}>Back</button>
          {currentStep.type === "follow-up" ? (
            <button type="submit" className="btn-primary yv-btn-primary" disabled={wantsFollowUp === null || loading}>{loading ? "Sending…" : "Send my feedback"}</button>
          ) : (
            <button type="button" className="btn-primary yv-btn-primary" onClick={next} disabled={!stepIsComplete(currentStep)}>Next</button>
          )}
        </footer>
      </form>
    </main>
  );
}
