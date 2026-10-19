import { useMemo, useState } from "react";

const API_BASE_URL = import.meta.env.VITE_API_URL || "http://localhost:8080";
const COMPANY_NAME = "Care4EveryChild";

// Same home slug mapping idea
const HOME_NAMES = {
  millbrow: "Millbrow House",
  // oakwood: "Oakwood House",
};

// How many days from the end of the month the form is OPEN.
// Example: 5 = only open on the last 5 days of each month.
const OPEN_DAYS_FROM_END = 5;

function useQueryParams() {
  return useMemo(() => {
    const params = new URLSearchParams(window.location.search);
    const homeSlug = params.get("home") || "";
    const homeName = HOME_NAMES[homeSlug] || "";
    const yp = params.get("yp") || ""; // optional initials / alias

    return { homeSlug, homeName, yp };
  }, []);
}

// Check if today is within the "end of month" window
function isEndOfMonthWindow() {
  const now = new Date();
  const year = now.getFullYear();
  const month = now.getMonth(); // 0–11
  const today = now.getDate();

  const lastDay = new Date(year, month + 1, 0).getDate(); // 0 => last day of prev month
  return today > lastDay - OPEN_DAYS_FROM_END;
}

function YoungVoiceApp() {
  const { homeName, yp } = useQueryParams();

  const [form, setForm] = useState({
    young_person_alias: yp || "",

    q_overall_month: "",
    q_best_thing: "",
    q_unsafe_or_worried: "",
    q_listened_to: "",
    q_peers: "",
    q_change_home: "",
    q_education_help: "",
    q_message_ofsted: "",
    q_never_forget: "",

    want_talk: false,
    follow_up_preference: "",
  });

  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState(null); // { id }

  const updateField = (field) => (e) => {
    const value =
      e.target.type === "checkbox" ? e.target.checked : e.target.value;
    setForm((prev) => ({ ...prev, [field]: value }));
  };

  // Period month: default to current month YYYY-MM (same as before)
  const currentPeriodMonth = (() => {
    const now = new Date();
    const m = String(now.getMonth() + 1).padStart(2, "0");
    return `${now.getFullYear()}-${m}`;
  })();

  const handleSubmit = async (e) => {
    e.preventDefault();
    setError("");
    setSubmitting(true);

    if (!homeName) {
      setError("Internal error: home is missing from the link.");
      setSubmitting(false);
      return;
    }

    if (!form.q_overall_month.trim()) {
      setError("Please tell us how this month has felt living here.");
      setSubmitting(false);
      return;
    }

    const payload = {
      home: homeName,
      young_person_alias: form.young_person_alias.trim(),
      period_month: currentPeriodMonth,

      q_overall_month: form.q_overall_month.trim(),
      q_best_thing: form.q_best_thing.trim(),
      q_unsafe_or_worried: form.q_unsafe_or_worried.trim(),
      q_listened_to: form.q_listened_to.trim(),
      q_peers: form.q_peers.trim(),
      q_change_home: form.q_change_home.trim(),
      q_education_help: form.q_education_help.trim(),
      q_message_ofsted: form.q_message_ofsted.trim(),
      q_never_forget: form.q_never_forget.trim(),

      want_talk: form.want_talk,
      follow_up_preference: form.follow_up_preference.trim(),
    };

    try {
      const res = await fetch(`${API_BASE_URL}/api/v1/young-voice`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });

      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        throw new Error(body.error || `Request failed with ${res.status}`);
      }

      const data = await res.json();
      setSuccess({ id: data.young_voice_id });
    } catch (err) {
      console.error(err);
      setError(err.message || "Something went wrong submitting your views.");
    } finally {
      setSubmitting(false);
    }
  };

  // 1) If link is broken / missing home
  if (!homeName) {
    return (
      <div className="app-root">
        <div className="card">
          <h1>Invalid or incomplete link</h1>
          <p>
            This check-in link is missing the home information. Please speak to
            staff or contact <strong>{COMPANY_NAME}</strong>.
          </p>
        </div>
      </div>
    );
  }

  // 2) Only allow submission at end of month
  const open = isEndOfMonthWindow();
  if (!open) {
    return (
      <div className="app-root">
        <div className="card">
          <h1>Monthly check-in not open yet 😊</h1>
          <p className="subtitle">
            This form is only available at the end of each month.
          </p>
          <p>
            Please come back in the last few days of the month to share how
            things have been living at{" "}
            <strong>
              {homeName} ({COMPANY_NAME})
            </strong>
            .
          </p>
        </div>
      </div>
    );
  }

  // 3) Success screen
  if (success) {
    return (
      <div className="app-root">
        <div className="card">
          <h1>Thank you for sharing 💙</h1>
          <p>
            Your monthly check-in about{" "}
            <strong>
              {homeName} ({COMPANY_NAME})
            </strong>{" "}
            has been saved.
          </p>
          <p>Your reference ID:</p>
          <pre>{success.id}</pre>
          <p>
            An adult in the home will read this and follow up where needed. You
            can now close this page.
          </p>
        </div>
      </div>
    );
  }

  // 4) Main form (shorter)
  return (
    <div className="app-root">
      <div className="card">
        <h1>Your monthly check-in</h1>
        <p className="subtitle">
          This is your space to say how things really feel living at{" "}
          <strong>
            {homeName} ({COMPANY_NAME})
          </strong>
          . There are no right or wrong answers.
        </p>

        {error && <div className="error-banner">{error}</div>}

        <form onSubmit={handleSubmit} className="form-grid">
          <label className="field">
            <span>Your name or initials (optional)</span>
            <input
              type="text"
              value={form.young_person_alias}
              onChange={updateField("young_person_alias")}
              placeholder="You can leave this blank if you prefer."
            />
          </label>

          {/* 1. Overall month */}
          <label className="field full">
            <span>How has this month felt living here?</span>
            <textarea
              rows={3}
              value={form.q_overall_month}
              onChange={updateField("q_overall_month")}
              placeholder="Tell us in your own words what this month has been like for you."
              required
            />
          </label>

          {/* 2. Best thing */}
          <label className="field full">
            <span>What has been the BEST thing about this month?</span>
            <textarea
              rows={2}
              value={form.q_best_thing}
              onChange={updateField("q_best_thing")}
              placeholder="A moment, a person, an activity – anything that stands out."
            />
          </label>

          {/* 3. Unsafe / worried */}
          <label className="field full">
            <span>
              Has anything this month made you feel unsafe, scared or very
              worried?
            </span>
            <textarea
              rows={3}
              value={form.q_unsafe_or_worried}
              onChange={updateField("q_unsafe_or_worried")}
              placeholder="Only share what you feel comfortable sharing. You can mention people, places or situations."
            />
          </label>

          {/* 4. Adults listening */}
          <label className="field full">
            <span>
              Do you feel adults here listen to you and take you seriously?
            </span>
            <textarea
              rows={3}
              value={form.q_listened_to}
              onChange={updateField("q_listened_to")}
              placeholder="You can tell us about a time this month when an adult listened (or didn’t listen) to you."
            />
          </label>

          {/* 5. Other young people */}
          <label className="field full">
            <span>How are things with the other young people in the home?</span>
            <textarea
              rows={3}
              value={form.q_peers}
              onChange={updateField("q_peers")}
              placeholder="Friends, arguments, people you get on with or don’t get on with."
            />
          </label>

          {/* 6. Change about home */}
          <label className="field full">
            <span>
              If you could change ONE thing about the home or routines, what
              would it be?
            </span>
            <textarea
              rows={2}
              value={form.q_change_home}
              onChange={updateField("q_change_home")}
            />
          </label>

          {/* 7. Education help */}
          <label className="field full">
            <span>
              Is there anything the home could do to help you more with school,
              college or training?
            </span>
            <textarea
              rows={2}
              value={form.q_education_help}
              onChange={updateField("q_education_help")}
            />
          </label>

          {/* 8. Message to Ofsted */}
          <label className="field full">
            <span>
              If Ofsted or a visitor asked “What is this home really like?”,
              what would you tell them?
            </span>
            <textarea
              rows={3}
              value={form.q_message_ofsted}
              onChange={updateField("q_message_ofsted")}
            />
          </label>

          {/* 9. Never forget */}
          <label className="field full">
            <span>
              Is there anything you’d like adults to NEVER forget about you or
              your story?
            </span>
            <textarea
              rows={3}
              value={form.q_never_forget}
              onChange={updateField("q_never_forget")}
            />
          </label>

          {/* 10. Follow-up */}
          <label className="field checkbox">
            <input
              type="checkbox"
              checked={form.want_talk}
              onChange={updateField("want_talk")}
            />
            <span>
              I would like someone to talk to me about what I’ve written.
            </span>
          </label>

          <label className="field full">
            <span>If yes, who would you like to talk to?</span>
            <textarea
              rows={2}
              value={form.follow_up_preference}
              onChange={updateField("follow_up_preference")}
              placeholder="For example: key worker, a specific staff member, social worker…"
            />
          </label>

          <button type="submit" className="submit-btn" disabled={submitting}>
            {submitting ? "Sending..." : "Submit my check-in"}
          </button>
        </form>
      </div>
    </div>
  );
}

export default YoungVoiceApp;
