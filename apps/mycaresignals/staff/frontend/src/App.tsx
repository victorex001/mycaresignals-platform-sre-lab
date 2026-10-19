// src/App.tsx
import React, { useEffect, useMemo, useState } from "react";
import "./App.css";

import { Routes, Route, Navigate, useLocation, useNavigate, useParams } from "react-router-dom";

import ChildrenTab from "./ChildrenTab";
import ChildHomeTab from "./ChildHomeTab";
import ChildDashboardTab from "./ChildDashboardTab";
import AboutTab from "./AboutTab";
import GoalsTargetsTab from "./GoalsTargetsTab";
import RiskSafetyTab from "./RiskSafetyTab";
import DailyScoresTab from "./DailyScoresTab";
import DailyProgressTab from "./DailyProgressTab";
import DailyProgressHistoryTab from "./DailyProgressHistoryTab";
import ProgressOverTimeTab from "./ProgressOverTimeTab";
import KeyworkNotesTab from "./KeyworkNotesTab";
import KeyworkHistoryTab from "./KeyworkHistoryTab";
import TherapeuticSummaryTab from "./TherapeuticSummaryTab";
import ChildVoiceTab from "./ChildVoiceTab";
import { OnboardBaselineForm } from "./OnboardBaselineForm";
import BaselineReportView from "./BaselineReportView";
import ChildReportView from "./ChildReportView";
import ChildTrendsTab from "./ChildTrendsTab";
import CarePlanTab from "./CarePlanTab";
import CarePlanSectionView from "./CarePlanSectionView";
import CarePlanReviewView from "./CarePlanReviewView";
import CarePlanHistoryView from "./CarePlanHistoryView";
import MissingFromHomePlanTab from "./MissingFromHomePlanTab";
import LegalFooter from "./LegalFooter";

import type { CarePlanSectionKey, Child } from "./types";
import logo from "./assets/mycaresignals-logo.png";
import {
  AUTH_CHANGED_EVENT,
  DEV_AUTH_CHANGED_EVENT,
  INVALID_DEV_TOKEN_MESSAGE,
  apiFetch,
  completeStaffSignInFromURL,
  hasProductionSession,
  clearDevToken,
  consumeDevAuthError,
  enterLocalManagerDevelopmentAccess,
  getDevToken,
  productionAuthEnabled,
  setDevToken,
  signOutStaff,
  startStaffSignIn,
} from "./api";

const API_BASE = import.meta.env.VITE_API_BASE_URL ?? "/api/te";
const DEMO_BUILD = import.meta.env.MODE === "demo";
const HOME_ID = 1;
const HOME_NAME = "Millbrow House";
const PROVIDER_NAME = "Care4EveryChild";

type OnboardChildPayload = {
  external_ref: string;
  initials: string;
  first_name: string;
  last_name: string;
  date_of_birth: string;
  legal_status: string;
  gender: string;
  admission_date?: string;
};

type InviteStakeholderResponse = {
  name: string;
  email: string;
  organisation: string;
  status: "sent" | "already_invited";
  already_invited: boolean;
  personal_message: string;
};

type StakeholderInviteHistoryItem = {
  name: string;
  organisation: string;
  email: string;
  status: "Invited" | "Account setup pending" | "Active" | string;
  invited_date: string;
};

type DemoStakeholderAccess = {
  demo_enabled?: boolean;
  is_admin?: boolean;
  can_invite?: boolean;
  can_submit_feedback?: boolean;
  can_view_feedback?: boolean;
};

type StakeholderProductFeedbackItem = {
  id: number;
  reviewer_name: string;
  reviewer_email: string;
  organisation: string;
  environment: string;
  page_path: string;
  overall_rating: number;
  most_useful: string;
  unclear_or_difficult: string;
  missing: string;
  anything_else: string;
  submitted_at: string;
};

const errorMessage = (caught: unknown, fallback: string) =>
  caught instanceof Error ? caught.message : fallback;

/**
 * Helper: load a child from the home list by ID (since your backend doesn’t currently have /v1/children/{id})
 */
function useChildFromRoute(apiBase: string, homeId: number) {
  const params = useParams();
  const childId = Number(params.childId || 0);

  const [child, setChild] = useState<Child | null>(null);
  const [loading, setLoading] = useState<boolean>(true);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    async function run() {
      if (!childId || Number.isNaN(childId)) {
        setLoading(false);
        setChild(null);
        return;
      }

      setLoading(true);
      setErr(null);

      try {
        const res = await apiFetch(`${apiBase}/v1/homes/${homeId}/children`);
        if (!res.ok) throw new Error((await res.text()) || "Failed to load children");
        const list = (await res.json()) as Child[];

        const found = list.find((c) => c.id === childId) || null;
        if (!cancelled) setChild(found);
        if (!cancelled && !found) setErr(`Child ${childId} not found in home ${homeId}`);
      } catch (e: unknown) {
        if (!cancelled) setErr(errorMessage(e, "Failed to load child"));
        if (!cancelled) setChild(null);
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    run();
    return () => {
      cancelled = true;
    };
  }, [apiBase, homeId, childId]);

  return { childId, child, loading, err };
}

function TopNav({ access }: { access: DemoStakeholderAccess }) {
  const location = useLocation();
  const navigate = useNavigate();
  const canInviteStakeholders = access.can_invite === true;
  const canSubmitFeedback = access.can_submit_feedback === true;
  const demoEnabled = access.demo_enabled === true;
  const isAdmin = access.is_admin === true;

  const active = useMemo(() => {
    const p = location.pathname;
    if (p.startsWith("/admin")) return "admin";
    if (p.startsWith("/children")) return "children";
    if (p.startsWith("/onboard")) return "onboard";
    if (p.startsWith("/invite-stakeholder")) return "invite";
    if (p.startsWith("/feedback")) return "feedback";
    if (p.startsWith("/stakeholder-feedback")) return "stakeholder-feedback";
    return "about";
  }, [location.pathname]);

  return (
    <nav className="tab-row">
      {isAdmin && (
        <button
          className={`tab ${active === "admin" ? "tab-active" : ""}`}
          onClick={() => navigate("/admin")}
        >
          Admin
        </button>
      )}
      <button
        className={`tab ${active === "about" ? "tab-active" : ""}`}
        onClick={() => navigate("/about")}
      >
        About
      </button>
      <button
        className={`tab ${active === "children" ? "tab-active" : ""}`}
        onClick={() => navigate("/children")}
      >
        Children
      </button>
      {(!demoEnabled || canInviteStakeholders) && (
        <button
          className={`tab ${active === "onboard" ? "tab-active" : ""}`}
          onClick={() => navigate("/onboard")}
        >
          Add young person
        </button>
      )}
      {canSubmitFeedback && (
        <button
          className={`tab tab-subtle ${active === "feedback" ? "tab-active" : ""}`}
          onClick={() =>
            navigate("/feedback", {
              state: { returnTo: `${location.pathname}${location.search}${location.hash}` },
            })
          }
        >
          Give feedback
        </button>
      )}
    </nav>
  );
}

function DemoAdminPage() {
  const navigate = useNavigate();
  return (
    <div className="admin-workspace">
      <section className="card">
        <h1>Demo administration</h1>
        <p className="card-subtitle">
          Manage stakeholder access and review feedback submitted through the demo.
        </p>
        <div className="feedback-actions">
          <button type="button" className="btn btn-secondary" onClick={() => navigate("/children")}>
            Manage demo records
          </button>
          <button type="button" className="btn btn-secondary" onClick={() => navigate("/onboard")}>
            Add young person
          </button>
        </div>
      </section>
      <InviteStakeholderPage />
      <StakeholderFeedbackAdminPage />
    </div>
  );
}

function AdminOnly({ access, children }: { access: DemoStakeholderAccess; children: React.ReactNode }) {
  if (DEMO_BUILD && access.is_admin !== true) {
    return <Navigate to="/" replace />;
  }
  return <>{children}</>;
}

function InviteStakeholderPage() {
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [organisation, setOrganisation] = useState("");
  const [sending, setSending] = useState(false);
  const [copying, setCopying] = useState(false);
  const [successInvite, setSuccessInvite] = useState<InviteStakeholderResponse | null>(null);
  const [copyMessageStatus, setCopyMessageStatus] = useState<string | null>(null);
  const [personalMessage, setPersonalMessage] = useState("");
  const [history, setHistory] = useState<StakeholderInviteHistoryItem[]>([]);
  const [error, setError] = useState<string | null>(null);

  const loadHistory = async () => {
    const response = await apiFetch(`${API_BASE}/v1/demo/stakeholders/invites`);
    if (!response.ok) return;
    const payload = (await response.json()) as { items?: StakeholderInviteHistoryItem[] };
    setHistory(Array.isArray(payload.items) ? payload.items : []);
  };

  useEffect(() => {
    void loadHistory().catch(() => setHistory([]));
  }, []);

  const submitInvite = async (event: React.SyntheticEvent, resend = false, resendEmail = "") => {
    event.preventDefault();
    setSending(true);
    setSuccessInvite(null);
    setCopyMessageStatus(null);
    setError(null);
    try {
      const response = await apiFetch(`${API_BASE}/v1/demo/stakeholders/invite${resend ? "/resend" : ""}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          name: resend ? history.find((item) => item.email === resendEmail)?.name || resendEmail : name,
          email: resend ? resendEmail : email,
          organisation: resend ? history.find((item) => item.email === resendEmail)?.organisation || "" : organisation,
        }),
      });
      if (!response.ok) {
        const text = await response.text();
        throw new Error(text || "Invite could not be sent.");
      }
      const payload = (await response.json()) as InviteStakeholderResponse;
      if (!resend) {
        setName("");
        setEmail("");
        setOrganisation("");
      }
      setPersonalMessage(payload.personal_message);
      setSuccessInvite(payload);
      await loadHistory();
    } catch (caught) {
      setError(errorMessage(caught, "Invite could not be sent."));
    } finally {
      setSending(false);
    }
  };

  const copyMessage = async () => {
    if (!personalMessage.trim()) return;
    setCopying(true);
    setError(null);
    try {
      await navigator.clipboard.writeText(personalMessage);
      setCopyMessageStatus("Copied ✓");
    } catch (caught) {
      setError(errorMessage(caught, "Message could not be copied."));
    } finally {
      setCopying(false);
    }
  };

  return (
    <section className="card invite-card">
      <h2>Invite stakeholder</h2>
      <form className="invite-form" onSubmit={(event) => void submitInvite(event)}>
        <label className="form-group">
          <span className="label">Name</span>
          <input
            className="input"
            type="text"
            autoComplete="name"
            required
            value={name}
            onChange={(event) => setName(event.target.value)}
          />
        </label>
        <label className="form-group">
          <span className="label">Email</span>
          <input
            className="input"
            type="email"
            autoComplete="email"
            required
            value={email}
            onChange={(event) => setEmail(event.target.value)}
          />
        </label>
        <label className="form-group">
          <span className="label">Organisation <span className="field-hint">(optional)</span></span>
          <input
            className="input"
            type="text"
            autoComplete="organization"
            value={organisation}
            onChange={(event) => setOrganisation(event.target.value)}
          />
        </label>
        <button type="submit" className="btn btn-primary" disabled={sending}>
          {sending ? "Sending…" : "Send invite"}
        </button>
      </form>
      {successInvite && (
        <section className="invite-success-panel">
          <h3>Invite sent ✓</h3>
          <p>
            {successInvite.name.split(/\s+/)[0]} has been emailed a secure link to set up their
            myCareSignals demo access.
          </p>
        </section>
      )}
      {error && <p className="alert alert-warning">{error}</p>}
      {personalMessage && (
        <section className="invite-message-panel">
          <h3>Optional follow-up</h3>
          <p className="field-hint">
            Want to let {successInvite?.name.split(/\s+/)[0] || "them"} know personally? Copy this
            short message to send by LinkedIn, WhatsApp or email.
          </p>
          <textarea
            className="input"
            rows={5}
            value={personalMessage}
            onChange={(event) => setPersonalMessage(event.target.value)}
          />
          <div className="invite-follow-up-actions">
            <button type="button" className="btn btn-secondary" onClick={copyMessage} disabled={copying}>
              {copying ? "Copying…" : "Copy message"}
            </button>
            {copyMessageStatus && <span className="field-hint">{copyMessageStatus}</span>}
          </div>
        </section>
      )}
      <section className="invite-history">
        <h3>Stakeholder invites</h3>
        {history.length === 0 ? (
          <p className="field-hint">No stakeholder invites yet.</p>
        ) : (
          <div className="invite-history-list">
            {history.map((item) => (
              <article key={item.email} className="invite-history-row">
                <div>
                  <strong>{item.name}</strong>
                  <span>{item.organisation || "Organisation not recorded"}</span>
                </div>
                <div>
                  <span>{item.email}</span>
                  <span>{item.status}</span>
                </div>
                <div>
                  <span>{item.invited_date}</span>
                  {item.status !== "Active" && (
                    <button type="button" className="text-button" disabled={sending} onClick={(event) => void submitInvite(event, true, item.email)}>
                      Resend invite
                    </button>
                  )}
                </div>
              </article>
            ))}
          </div>
        )}
      </section>
    </section>
  );
}

function StakeholderProductFeedbackPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const returnTo =
    (location.state as { returnTo?: string } | null)?.returnTo ||
    "/about";
  const [overallRating, setOverallRating] = useState(0);
  const [mostUseful, setMostUseful] = useState("");
  const [unclearOrDifficult, setUnclearOrDifficult] = useState("");
  const [missing, setMissing] = useState("");
  const [anythingElse, setAnythingElse] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [submitted, setSubmitted] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submitFeedback = async (event: React.FormEvent) => {
    event.preventDefault();
    setError(null);
    if (overallRating < 1 || overallRating > 5) {
      setError("Please choose a rating from 1 to 5.");
      return;
    }
    setSubmitting(true);
    try {
      const response = await apiFetch(`${API_BASE}/v1/demo/stakeholders/product-feedback`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          overall_rating: overallRating,
          most_useful: mostUseful,
          unclear_or_difficult: unclearOrDifficult,
          missing,
          anything_else: anythingElse,
          page_path: returnTo,
        }),
      });
      if (!response.ok) {
        const text = await response.text();
        throw new Error(text || "Feedback could not be sent.");
      }
      setSubmitted(true);
    } catch (caught) {
      setError(errorMessage(caught, "Feedback could not be sent."));
    } finally {
      setSubmitting(false);
    }
  };

  if (submitted) {
    return (
      <section className="card feedback-card">
        <div className="feedback-success-panel">
          <h2>Thank you ✓</h2>
          <p>Your feedback has been sent to the myCareSignals team.</p>
        </div>
        <button type="button" className="btn btn-primary" onClick={() => navigate(returnTo)}>
          Return to what I was viewing
        </button>
      </section>
    );
  }

  return (
    <section className="card feedback-card">
      <h2>Share your feedback</h2>
      <p className="card-subtitle">
        Your feedback helps us improve myCareSignals. Please tell us what worked well and what could
        be clearer.
      </p>
      <form className="feedback-form" onSubmit={submitFeedback}>
        <fieldset className="feedback-rating-field">
          <legend>Overall, how useful does myCareSignals feel?</legend>
          <div className="feedback-rating-options" role="radiogroup" aria-label="Overall usefulness rating">
            {[1, 2, 3, 4, 5].map((rating) => (
              <button
                key={rating}
                type="button"
                className={`feedback-rating-button ${overallRating === rating ? "selected" : ""}`}
                onClick={() => setOverallRating(rating)}
                aria-pressed={overallRating === rating}
              >
                {rating}
              </button>
            ))}
          </div>
        </fieldset>
        <label className="form-group">
          <span className="label">What felt most useful? <span className="field-hint">(optional)</span></span>
          <textarea className="input" rows={4} value={mostUseful} onChange={(event) => setMostUseful(event.target.value)} />
        </label>
        <label className="form-group">
          <span className="label">Was anything unclear or difficult to use? <span className="field-hint">(optional)</span></span>
          <textarea className="input" rows={4} value={unclearOrDifficult} onChange={(event) => setUnclearOrDifficult(event.target.value)} />
        </label>
        <label className="form-group">
          <span className="label">What do you think is missing? <span className="field-hint">(optional)</span></span>
          <textarea className="input" rows={4} value={missing} onChange={(event) => setMissing(event.target.value)} />
        </label>
        <label className="form-group">
          <span className="label">Anything else you'd like us to know? <span className="field-hint">(optional)</span></span>
          <textarea className="input" rows={4} value={anythingElse} onChange={(event) => setAnythingElse(event.target.value)} />
        </label>
        {error && <p className="alert alert-warning">{error}</p>}
        <div className="feedback-actions">
          <button type="submit" className="btn btn-primary" disabled={submitting}>
            {submitting ? "Sending…" : "Send feedback"}
          </button>
          <button type="button" className="btn btn-secondary" onClick={() => navigate(returnTo)}>
            Cancel
          </button>
        </div>
      </form>
    </section>
  );
}

function StakeholderFeedbackAdminPage() {
  const [items, setItems] = useState<StakeholderProductFeedbackItem[]>([]);
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    async function loadFeedback() {
      setLoading(true);
      setError(null);
      try {
        const response = await apiFetch(`${API_BASE}/v1/demo/stakeholders/product-feedback`);
        if (!response.ok) throw new Error((await response.text()) || "Feedback could not be loaded.");
        const payload = (await response.json()) as { items?: StakeholderProductFeedbackItem[] };
        const nextItems = Array.isArray(payload.items) ? payload.items : [];
        if (!cancelled) {
          setItems(nextItems);
          setSelectedId(nextItems[0]?.id ?? null);
        }
      } catch (caught) {
        if (!cancelled) setError(errorMessage(caught, "Feedback could not be loaded."));
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    void loadFeedback();
    return () => {
      cancelled = true;
    };
  }, []);

  const selected = items.find((item) => item.id === selectedId) || items[0] || null;

  return (
    <section className="card stakeholder-feedback-admin">
      <h2>Stakeholder Feedback</h2>
      <p className="card-subtitle">
        Product feedback from demo reviewers. This is separate from child evidence and therapeutic
        records.
      </p>
      {loading && <p>Loading feedback…</p>}
      {error && <p className="alert alert-warning">{error}</p>}
      {!loading && !error && items.length === 0 && (
        <p className="field-hint">No stakeholder product feedback has been submitted yet.</p>
      )}
      {items.length > 0 && (
        <div className="stakeholder-feedback-workspace">
          <div className="stakeholder-feedback-list">
            {items.map((item) => (
              <button
                key={item.id}
                type="button"
                className={`stakeholder-feedback-row ${selected?.id === item.id ? "selected" : ""}`}
                onClick={() => setSelectedId(item.id)}
              >
                <strong>{item.reviewer_name || item.reviewer_email}</strong>
                <span>{item.organisation || "Organisation not recorded"}</span>
                <span>{new Date(item.submitted_at).toLocaleString()}</span>
                <span>Rating {item.overall_rating}/5</span>
              </button>
            ))}
          </div>
          {selected && (
            <article className="stakeholder-feedback-detail">
              <div className="feedback-detail-header">
                <div>
                  <h3>{selected.reviewer_name || selected.reviewer_email}</h3>
                  <p>{selected.reviewer_email}</p>
                </div>
                <span className="pill">Rating {selected.overall_rating}/5</span>
              </div>
              <dl className="feedback-detail-list">
                <div>
                  <dt>Organisation</dt>
                  <dd>{selected.organisation || "Not recorded"}</dd>
                </div>
                <div>
                  <dt>Date</dt>
                  <dd>{new Date(selected.submitted_at).toLocaleString()}</dd>
                </div>
                <div>
                  <dt>Page/screen</dt>
                  <dd>{selected.page_path}</dd>
                </div>
                <div>
                  <dt>What felt most useful?</dt>
                  <dd>{selected.most_useful || "No response"}</dd>
                </div>
                <div>
                  <dt>Was anything unclear or difficult?</dt>
                  <dd>{selected.unclear_or_difficult || "No response"}</dd>
                </div>
                <div>
                  <dt>What is missing?</dt>
                  <dd>{selected.missing || "No response"}</dd>
                </div>
                <div>
                  <dt>Anything else?</dt>
                  <dd>{selected.anything_else || "No response"}</dd>
                </div>
              </dl>
            </article>
          )}
        </div>
      )}
    </section>
  );
}

function ChildrenPage() {
  const navigate = useNavigate();

  return (
    <ChildrenTab
      apiBase={API_BASE}
      homeId={HOME_ID}
      homeName={HOME_NAME}
      providerName={PROVIDER_NAME}
      selectedChild={null}
      onOpenChildDashboard={(child) => navigate(`/children/${child.id}/dashboard`)}
      onCreateChild={() => navigate("/onboard")}
      onContinueBaseline={(child) => navigate(`/children/${child.id}/baseline-report`)}
    />
  );
}

function OnboardPage() {
  const navigate = useNavigate();
  const params = useParams();
  const childId = Number(params.childId || 0);

  const [onboardStep, setOnboardStep] = useState<0 | 1 | 2>(0);

  const [externalRef, setExternalRef] = useState("");
  const [initials, setInitials] = useState("");
  const [firstName, setFirstName] = useState("");
  const [lastName, setLastName] = useState("");
  const [dob, setDob] = useState("");
  const [legalStatus, setLegalStatus] = useState("");
  const [admissionDate, setAdmissionDate] = useState("");
  const [gender, setGender] = useState("");

  const [loadingOnboard, setLoadingOnboard] = useState(false);
  const [onboardError, setOnboardError] = useState<string | null>(null);

  const [selectedChild, setSelectedChild] = useState<Child | null>(null);

  // If user came to /children/:childId/onboard, load the child so baseline step works on refresh
  const { child, loading, err } = useChildFromRoute(API_BASE, HOME_ID);

  useEffect(() => {
    if (childId && child && child.id === childId) {
      setSelectedChild(child);
      setOnboardStep(2);
    }
  }, [childId, child]);

  const handleOnboardSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setOnboardError(null);

    if (!initials.trim() || !dob) {
      setOnboardError("Initials and date of birth are required.");
      return;
    }

    setLoadingOnboard(true);
    try {
      const payload: OnboardChildPayload = {
        external_ref: externalRef || "",
        initials: initials.trim(),
        first_name: firstName.trim() || "",
        last_name: lastName.trim() || "",
        date_of_birth: dob,
        legal_status: legalStatus.trim() || "",
        gender: gender.trim() || "",
      };
      if (admissionDate) payload.admission_date = admissionDate;

      const res = await apiFetch(`${API_BASE}/v1/homes/${HOME_ID}/children/onboard`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });

      if (!res.ok) {
        const txt = await res.text();
        throw new Error(txt || `Onboard failed (${res.status})`);
      }

      const data: Child = await res.json();
      setSelectedChild(data);
      setOnboardStep(2);

      // put child in URL so refresh keeps you here
      navigate(`/children/${data.id}/onboard`);
    } catch (err: unknown) {
      setOnboardError(errorMessage(err, "Something went wrong"));
    } finally {
      setLoadingOnboard(false);
    }
  };

  const renderOnboardView = () => (
    <section className="card">
      <div className="card-header">
        <div>
          <h2>Add young person</h2>
          <p className="card-subtitle">
            {onboardStep === 0
              ? "Choose the right starting point. Both routes use the same six-domain framework."
              : onboardStep === 1
              ? "Create or confirm a child record for this home."
              : "Build a human-led starting framework of strengths, needs and priorities."}
          </p>
        </div>
      </div>

      {onboardError && <div className="alert alert-error">{onboardError}</div>}

      {onboardStep === 0 && (
        <div className="framework-choice-grid onboard-entry-choices">
          <button type="button" className="framework-choice" onClick={() => setOnboardStep(1)}>
            <strong>New placement</strong>
            <span>Create the young person’s record, then begin from admission information.</span>
          </button>
          <button type="button" className="framework-choice" onClick={() => navigate("/children")}>
            <strong>Existing young person</strong>
            <span>Choose someone already listed, then open Initial Framework from Child Home.</span>
          </button>
        </div>
      )}

      {onboardStep === 1 && (
        <form className="form-grid" onSubmit={handleOnboardSubmit}>
          <div className="summary">
            <h3>Placement basics</h3>
            <p>
              Capture the minimum information needed to create a record for this
              child in MyCareSignals. This will be linked to goals, key-working
              and the therapeutic timeline.
            </p>
          </div>

          <div className="form-row-2">
            <div className="form-group">
              <label className="label">
                Care / LA reference <span className="label-pill">Optional</span>
              </label>
              <input
                className="input"
                value={externalRef}
                onChange={(e) => setExternalRef(e.target.value)}
                placeholder="e.g. Local authority ID"
              />
            </div>

            <div className="form-group">
              <label className="label">
                Initials <span className="required">*</span>
              </label>
              <input
                className="input"
                value={initials}
                onChange={(e) => setInitials(e.target.value.toUpperCase())}
                placeholder="e.g. BK"
              />
            </div>
          </div>

          <div className="form-row-2">
            <div className="form-group">
              <label className="label">First name</label>
              <input
                className="input"
                value={firstName}
                onChange={(e) => setFirstName(e.target.value)}
                placeholder="Optional – for internal use only"
              />
            </div>
            <div className="form-group">
              <label className="label">Last name</label>
              <input
                className="input"
                value={lastName}
                onChange={(e) => setLastName(e.target.value)}
                placeholder="Optional – for internal use only"
              />
            </div>
          </div>

          <div className="form-row-2">
            <div className="form-group">
              <label className="label">
                Date of birth <span className="required">*</span>
              </label>
              <input className="input" type="date" value={dob} onChange={(e) => setDob(e.target.value)} />
            </div>

            <div className="form-group">
              <label className="label">Gender</label>
              <input
                className="input"
                value={gender}
                onChange={(e) => setGender(e.target.value)}
                placeholder="As recorded in placement documentation"
              />
            </div>
          </div>

          <div className="form-row-2">
            <div className="form-group">
              <label className="label">Legal status</label>
              <input
                className="input"
                value={legalStatus}
                onChange={(e) => setLegalStatus(e.target.value)}
                placeholder="e.g. Section 20, ICO, Full Care Order"
              />
            </div>

            <div className="form-group">
              <label className="label">Admission date to this home</label>
              <input
                className="input"
                type="date"
                value={admissionDate}
                onChange={(e) => setAdmissionDate(e.target.value)}
              />
            </div>
          </div>

          <div className="field-hint">
            This creates the child record only. No goals, care plans or risks are created automatically.
          </div>

          <div>
            <button type="submit" className="btn btn-primary" disabled={loadingOnboard}>
              {loadingOnboard ? "Creating record..." : "Continue to initial framework"}
            </button>
          </div>
        </form>
      )}

      {onboardStep === 2 && (selectedChild || child) && (
        <OnboardBaselineForm
          apiBase={API_BASE}
          homeId={HOME_ID}
          child={(selectedChild ?? child)!}
          pathway="new_placement"
          onFinished={() => navigate(`/children/${(selectedChild ?? child)!.id}/dashboard`)}
          onBack={() => {
            setOnboardStep(1);
            navigate("/onboard");
          }}
        />
      )}

      {onboardStep === 2 && childId && loading && <p className="field-hint">Loading child…</p>}
      {onboardStep === 2 && childId && err && <div className="alert alert-error">{err}</div>}
    </section>
  );

  return renderOnboardView();
}

function ChildPage({ section }: { section:
  | "dashboard"
  | "journey"
  | "goals"
  | "care_plan"
  | "care_plan_section"
  | "care_plan_review"
  | "care_plan_history"
  | "missing_from_home"
  | "risks"
  | "daily_progress"
  | "daily_progress_history"
  | "progress_over_time"
  | "daily_scores"
  | "keywork_notes"
  | "keywork_history"
  | "summary"
  | "child_voice"
  | "framework"
  | "framework_awaiting"
  | "baseline_report"
  | "child_report"
  | "scores_trends";
}) {
  const navigate = useNavigate();
  const location = useLocation();
  const routeParams = useParams();
  const { childId, child, loading, err } = useChildFromRoute(API_BASE, HOME_ID);

  if (loading) return <p className="field-hint">Loading child…</p>;
  if (err) return <div className="alert alert-error">{err}</div>;
  if (!child || !childId) return <Navigate to="/children" replace />;

  const goto = (path: string) => navigate(`/children/${child.id}/${path}`);

  if (section === "dashboard") {
    return (
      <ChildHomeTab
        apiBase={API_BASE}
        homeName={HOME_NAME}
        child={child}
        onBackToChildren={() => navigate("/children")}
        onOpenFramework={() => goto("framework")}
        onOpenDailyProgress={() => goto("daily-progress")}
        onOpenProgress={() => goto("progress")}
        onOpenKeywork={() => goto("keywork-notes")}
        onOpenCarePlan={() => goto("care-plan")}
        onOpenGoals={() => goto("goals")}
        onOpenRisks={() => goto("risks")}
        onOpenMissingFromHome={() => goto("missing-from-home")}
        onOpenVoice={() => goto("child-voice")}
        onOpenReviews={() => goto("journey?focus=attention")}
        onOpenJourney={() => goto("journey")}
      />
    );
  }

  if (section === "journey") {
    return (
      <ChildDashboardTab
        apiBase={API_BASE}
        homeName={HOME_NAME}
        child={child}
        onBackToHome={() => goto("dashboard")}
        onOpenPlan={() => goto("goals")}
        onOpenCarePlan={() => goto("care-plan")}
        onOpenRisks={() => goto("risks")}
        onOpenDailyProgress={() => goto("daily-progress")}
        onOpenKeyworkNotes={() => goto("keywork-notes")}
        onOpenKeyworkHistory={() => goto("keywork-history")}
        onOpenSummary={() => goto("summary")}
        onOpenChildVoice={() => goto("child-voice")}
        onOpenBaselineReport={() => goto("baseline-report")}
        onOpenChildReport={() => goto("report")}
        onOpenScoresTrends={() => goto("trends")}
        onOpenFeedbackReview={() => goto("child-voice")}
        focusAttention={new URLSearchParams(location.search).get("focus") === "attention"}
      />
    );
  }

  if (section === "goals") {
    return (
      <GoalsTargetsTab
        apiBase={API_BASE}
        childId={child.id}
        childInitials={child.initials}
        onBackToDashboard={() => goto("dashboard")}
        onOpenKeywork={() => goto("keywork-notes")}
        onOpenRisks={() => goto("risks")}
      />
    );
  }

  if (section === "care_plan") {
    return (
      <CarePlanTab
        apiBase={API_BASE}
        child={child}
        onBackToDashboard={() => goto("dashboard")}
        onOpenSection={(sectionKey) => goto(`care-plan/sections/${sectionKey}`)}
        onOpenReview={() => goto("care-plan/review")}
        onOpenHistory={() => goto("care-plan/history")}
      />
    );
  }

  if (section === "care_plan_section") {
    return (
      <CarePlanSectionView
        apiBase={API_BASE}
        child={child}
        sectionKey={(routeParams.sectionKey ?? "") as CarePlanSectionKey}
        onBackToPlan={() => goto("care-plan")}
        onOpenBaseline={() => goto("baseline-report")}
      />
    );
  }

  if (section === "care_plan_review") {
    return (
      <CarePlanReviewView
        apiBase={API_BASE}
        child={child}
        onBackToPlan={() => goto("care-plan")}
        onOpenGoals={() => goto("goals")}
        onOpenRisks={() => goto("risks")}
        onOpenKeywork={() => goto("keywork-history")}
        onOpenDailyProgress={() => goto("daily-progress/history")}
        onOpenVoice={() => goto("child-voice")}
        onOpenHistory={() => goto("care-plan/history")}
      />
    );
  }

  if (section === "care_plan_history") {
    return <CarePlanHistoryView apiBase={API_BASE} child={child} onBackToPlan={() => goto("care-plan")} />;
  }

  if (section === "risks") {
    return <RiskSafetyTab apiBase={API_BASE} childId={child.id} childInitials={child.initials} onBackToDashboard={() => goto("dashboard")} onOpenGoals={() => goto("goals")} />;
  }

  if (section === "missing_from_home") {
    return (
      <MissingFromHomePlanTab
        apiBase={API_BASE}
        child={child}
        onBackToDashboard={() => goto("dashboard")}
        onOpenRisks={() => goto("risks")}
        onOpenKeywork={() => goto("keywork-notes")}
      />
    );
  }

  if (section === "daily_scores") {
    return <DailyScoresTab apiBase={API_BASE} childId={child.id} childInitials={child.initials} />;
  }

  if (section === "daily_progress") {
    return (
      <DailyProgressTab
        apiBase={API_BASE}
        childId={child.id}
        childInitials={child.initials}
        onBackToDashboard={() => goto("dashboard")}
        onOpenProgress={() => goto("progress")}
        onOpenHistory={() => goto("daily-progress/history")}
      />
    );
  }

  if (section === "daily_progress_history") {
    return (
      <DailyProgressHistoryTab
        apiBase={API_BASE}
        childId={child.id}
        childInitials={child.initials}
        onBackToDashboard={() => goto("dashboard")}
        onOpenDailyProgress={() => goto("daily-progress")}
      />
    );
  }

  if (section === "progress_over_time") {
    return (
      <ProgressOverTimeTab
        apiBase={API_BASE}
        childId={child.id}
        childInitials={child.initials}
        onBackToDashboard={() => goto("dashboard")}
        onOpenDailyProgress={() => goto("daily-progress")}
        onOpenDailyHistory={() => goto("daily-progress/history")}
        onOpenKeywork={() => goto("keywork-history")}
        onOpenMonthlyVoice={(receiptId) => navigate(`/children/${child.id}/child-voice?receipt=${receiptId}`)}
      />
    );
  }

  if (section === "keywork_notes") {
    return (
      <KeyworkNotesTab
        apiBase={API_BASE}
        childId={child.id}
        childInitials={child.initials}
        onBackToDashboard={() => goto("dashboard")}
        onOpenHistory={() => goto("keywork-history")}
      />
    );
  }

  if (section === "keywork_history") {
    return (
      <KeyworkHistoryTab
        apiBase={API_BASE}
        childId={child.id}
        childInitials={child.initials}
        onBackToKeywork={() => goto("keywork-notes")}
        onBackToDashboard={() => goto("dashboard")}
      />
    );
  }

  if (section === "summary") {
    return <TherapeuticSummaryTab apiBase={API_BASE} childId={child.id} childInitials={child.initials} />;
  }

  if (section === "child_voice") {
    return (
      <ChildVoiceTab
        apiBase={API_BASE}
        homeName={HOME_NAME}
        child={child}
        onBackToDashboard={() => goto("dashboard")}
      />
    );
  }

  if (section === "framework") {
    return (
      <OnboardBaselineForm
        apiBase={API_BASE}
        homeId={HOME_ID}
        child={child}
        pathway="existing_young_person"
        onFinished={() => goto("dashboard")}
        onBack={() => goto("dashboard")}
        onOpenAwaitingInformation={() => goto("framework/awaiting-information")}
        onOpenLegacyBaseline={() => goto("baseline-report")}
        onOpenGoals={() => goto("goals")}
        onOpenCarePlan={() => goto("care-plan")}
        onOpenRisks={() => goto("risks")}
      />
    );
  }

  if (section === "framework_awaiting") {
    return (
      <OnboardBaselineForm
        apiBase={API_BASE}
        homeId={HOME_ID}
        child={child}
        pathway="existing_young_person"
        focusedView="awaiting_information"
        onFinished={() => goto("dashboard")}
        onBack={() => goto("framework")}
        onOpenLegacyBaseline={() => goto("baseline-report")}
        onOpenGoals={() => goto("goals")}
        onOpenCarePlan={() => goto("care-plan")}
        onOpenRisks={() => goto("risks")}
      />
    );
  }

  if (section === "baseline_report") {
    return <BaselineReportView apiBase={API_BASE} child={child} onBack={() => goto("dashboard")} />;
  }

  if (section === "child_report") {
    return (
      <ChildReportView
        apiBase={API_BASE}
        homeName={HOME_NAME}
        providerName={PROVIDER_NAME}
        child={child}
        onBack={() => goto("dashboard")}
      />
    );
  }

  // scores_trends
  return <ChildTrendsTab apiBase={API_BASE} childId={child.id} childInitials={child.initials} />;
}

function DevelopmentAccessGate({ onGranted }: { onGranted: () => void }) {
  const [token, setToken] = useState("");
  const [error, setError] = useState<string | null>(() =>
    consumeDevAuthError(),
  );
  const [starting, setStarting] = useState(false);

  const enterLocalManager = async () => {
    setError(null);
    setStarting(true);
    try {
      await enterLocalManagerDevelopmentAccess();
      onGranted();
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Local manager access could not be started.");
    } finally {
      setStarting(false);
    }
  };

  const handleSubmit = (event: React.FormEvent) => {
    event.preventDefault();
    setError(null);
    if (!token.trim()) {
      setError("Enter a development JWT to continue.");
      return;
    }
    if (!setDevToken(token)) {
      setError(INVALID_DEV_TOKEN_MESSAGE);
      return;
    }
    setToken("");
    onGranted();
  };

  return (
    <main className="layout-single">
      <section className="card" style={{ maxWidth: "36rem", margin: "3rem auto" }}>
        <div className="card-header">
          <div>
            <h2>Development access</h2>
            <p className="card-subtitle">
              Local development only. This is not the finished staff login.
            </p>
          </div>
        </div>

        {error && <div className="alert alert-error">{error}</div>}

        <button type="button" className="btn btn-primary" onClick={enterLocalManager} disabled={starting}>
          {starting ? "Entering…" : "Enter as local manager"}
        </button>

        <details className="dev-access-manual">
          <summary>Manual token entry</summary>
          <form className="form-grid" onSubmit={handleSubmit}>
            <div className="form-group">
              <label className="label" htmlFor="development-access-token">
                Development JWT
              </label>
              <input
                id="development-access-token"
                type="password"
                className="input"
                value={token}
                onChange={(event) => setToken(event.target.value)}
                autoComplete="off"
                spellCheck={false}
              />
            </div>
            <button type="submit" className="btn btn-secondary">
              Save manual token
            </button>
          </form>
        </details>
      </section>
    </main>
  );
}

function StaffSignInGate({ onGranted }: { onGranted: () => void }) {
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [starting, setStarting] = useState(false);

  useEffect(() => {
    let cancelled = false;
    completeStaffSignInFromURL()
      .then((signedIn) => {
        if (cancelled) return;
        if (signedIn) onGranted();
      })
      .catch((caught) => {
        if (!cancelled) setError(errorMessage(caught, "Staff sign in could not be completed."));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [onGranted]);

  const signIn = async () => {
    setError(null);
    setStarting(true);
    try {
      await startStaffSignIn();
    } catch (caught) {
      setStarting(false);
      setError(errorMessage(caught, "Staff sign in is not configured."));
    }
  };

  return (
    <main className="layout-single">
      <section className="card" style={{ maxWidth: "32rem", margin: "4rem auto", textAlign: "center" }}>
        <div className="card-header" style={{ justifyContent: "center" }}>
          <div>
            <h1>myCareSignals</h1>
            <p className="card-subtitle">Staff sign in</p>
          </div>
        </div>
        {loading && <p>Checking your session…</p>}
        {error && <div className="alert alert-error">{error}</div>}
        <button type="button" className="btn btn-primary" onClick={signIn} disabled={loading || starting}>
          {starting ? "Opening sign in…" : "Sign in"}
        </button>
      </section>
    </main>
  );
}

const App: React.FC = () => {
  const [hasDevToken, setHasDevToken] = useState(() => getDevToken() !== null);
  const [hasStaffSession, setHasStaffSession] = useState(() => hasProductionSession());
  const [demoAccess, setDemoAccess] = useState<DemoStakeholderAccess>({});
  const [accessResolved, setAccessResolved] = useState(false);
  const location = useLocation();
  const navigate = useNavigate();

  useEffect(() => {
    if (!import.meta.env.DEV) return;
    const syncTokenState = () => setHasDevToken(getDevToken() !== null);
    window.addEventListener(DEV_AUTH_CHANGED_EVENT, syncTokenState);
    return () => window.removeEventListener(DEV_AUTH_CHANGED_EVENT, syncTokenState);
  }, []);

  useEffect(() => {
    const authenticated = productionAuthEnabled() ? hasStaffSession : !import.meta.env.DEV || hasDevToken;
    if (!authenticated) {
      setAccessResolved(false);
      setDemoAccess({});
      return;
    }
    let cancelled = false;
    async function loadAccess() {
      try {
        const response = await apiFetch(`${API_BASE}/v1/demo/stakeholders/access`);
        if (!response.ok) {
          if (!cancelled) setDemoAccess({});
          return;
        }
        const payload = (await response.json()) as DemoStakeholderAccess;
        if (!cancelled) setDemoAccess(payload);
      } catch {
        if (!cancelled) setDemoAccess({});
      } finally {
        if (!cancelled) setAccessResolved(true);
      }
    }
    void loadAccess();
    return () => {
      cancelled = true;
    };
  }, [hasDevToken, hasStaffSession]);

  useEffect(() => {
    if (!accessResolved || demoAccess.demo_enabled !== true) return;
    const adminOnlyPath =
      location.pathname === "/admin" ||
      location.pathname === "/invite-stakeholder" ||
      location.pathname === "/stakeholder-feedback" ||
      location.pathname === "/onboard";
    if (demoAccess.is_admin === true && location.pathname === "/") {
      navigate("/admin", { replace: true });
    } else if (demoAccess.is_admin !== true && adminOnlyPath) {
      navigate("/", { replace: true });
    }
  }, [accessResolved, demoAccess.demo_enabled, demoAccess.is_admin, location.pathname, navigate]);

  useEffect(() => {
    if (!productionAuthEnabled()) return;
    const syncSessionState = () => setHasStaffSession(hasProductionSession());
    window.addEventListener(AUTH_CHANGED_EVENT, syncSessionState);
    return () => window.removeEventListener(AUTH_CHANGED_EVENT, syncSessionState);
  }, []);

  if (productionAuthEnabled() && !hasStaffSession) {
    return <StaffSignInGate onGranted={() => setHasStaffSession(true)} />;
  }

  if (import.meta.env.DEV && !hasDevToken) {
    return <DevelopmentAccessGate onGranted={() => setHasDevToken(true)} />;
  }

  if (!accessResolved) {
    return (
      <main className="layout-single">
        <section className="card" style={{ maxWidth: "32rem", margin: "4rem auto", textAlign: "center" }}>
          <p>Loading your demo access…</p>
        </section>
      </main>
    );
  }

  return (
    <div className="app-shell">
      <header className="app-header">
        <div className="app-logo-only">
          <img src={logo} alt="myCareSignals" className="app-logo-image" />
        </div>
        <div className="app-meta">
          <div className="pill">Home: {HOME_NAME}</div>
          <div className="pill-muted">Environment: Prototype · Local</div>
          {import.meta.env.DEV && hasDevToken && (
            <button
              type="button"
              className="btn btn-ghost btn-xs"
              onClick={clearDevToken}
            >
              Clear development access
            </button>
          )}
          {productionAuthEnabled() && hasStaffSession && (
            <button
              type="button"
              className="btn btn-ghost btn-xs"
              onClick={signOutStaff}
            >
              Sign out
            </button>
          )}
        </div>
      </header>

      <TopNav access={demoAccess} />

      <main className="layout-single">
        <Routes>
          <Route path="/" element={<AboutTab homeName={HOME_NAME} providerName={PROVIDER_NAME} />} />

          <Route path="/admin" element={<AdminOnly access={demoAccess}><DemoAdminPage /></AdminOnly>} />

          <Route path="/about" element={<AboutTab homeName={HOME_NAME} providerName={PROVIDER_NAME} />} />
          <Route path="/children" element={<ChildrenPage />} />
          <Route path="/onboard" element={<AdminOnly access={demoAccess}><OnboardPage /></AdminOnly>} />
          <Route path="/invite-stakeholder" element={<AdminOnly access={demoAccess}><Navigate to="/admin" replace /></AdminOnly>} />
          <Route path="/feedback" element={<StakeholderProductFeedbackPage />} />
          <Route path="/stakeholder-feedback" element={<AdminOnly access={demoAccess}><Navigate to="/admin" replace /></AdminOnly>} />
          <Route path="/children/:childId/onboard" element={<AdminOnly access={demoAccess}><OnboardPage /></AdminOnly>} />

          <Route path="/children/:childId/dashboard" element={<ChildPage section="dashboard" />} />
          <Route path="/children/:childId/journey" element={<ChildPage section="journey" />} />
          <Route path="/children/:childId/goals" element={<ChildPage section="goals" />} />
          <Route path="/children/:childId/care-plan" element={<ChildPage section="care_plan" />} />
          <Route path="/children/:childId/care-plan/sections/:sectionKey" element={<ChildPage section="care_plan_section" />} />
          <Route path="/children/:childId/care-plan/review" element={<ChildPage section="care_plan_review" />} />
          <Route path="/children/:childId/care-plan/history" element={<ChildPage section="care_plan_history" />} />
          <Route path="/children/:childId/risks" element={<ChildPage section="risks" />} />
          <Route path="/children/:childId/missing-from-home" element={<ChildPage section="missing_from_home" />} />
          <Route path="/children/:childId/daily-progress" element={<ChildPage section="daily_progress" />} />
          <Route path="/children/:childId/daily-progress/history" element={<ChildPage section="daily_progress_history" />} />
          <Route path="/children/:childId/progress" element={<ChildPage section="progress_over_time" />} />
          <Route path="/children/:childId/daily-scores" element={<ChildPage section="daily_scores" />} />
          <Route path="/children/:childId/keywork-notes" element={<ChildPage section="keywork_notes" />} />
          <Route path="/children/:childId/keywork-history" element={<ChildPage section="keywork_history" />} />
          <Route path="/children/:childId/summary" element={<ChildPage section="summary" />} />
          <Route path="/children/:childId/child-voice" element={<ChildPage section="child_voice" />} />
          <Route path="/children/:childId/framework" element={<ChildPage section="framework" />} />
          <Route path="/children/:childId/framework/awaiting-information" element={<ChildPage section="framework_awaiting" />} />
          <Route path="/children/:childId/baseline-report" element={<ChildPage section="baseline_report" />} />
          <Route path="/children/:childId/report" element={<ChildPage section="child_report" />} />
          <Route path="/children/:childId/trends" element={<ChildPage section="scores_trends" />} />

          <Route path="*" element={<Navigate to="/about" replace />} />
        </Routes>
      </main>

      <LegalFooter />
    </div>
  );
};

export default App;
