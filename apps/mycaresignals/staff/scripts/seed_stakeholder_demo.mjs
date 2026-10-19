#!/usr/bin/env node

import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";

const API_BASE = process.env.API_BASE || "http://127.0.0.1:18082";
const DB_URL = process.env.POSTGRES_DSN || "postgres://appuser:secretpassword@127.0.0.1:5433/therapeutic_engine?sslmode=disable";

const FRONTEND_HOME_ID = 1;
const HOME_REF = "MCS-DEMO-HOME-V1";
const CHILD_REF = "MCS-DEMO-ALEX-V1-HOME-1";
const STAFF_SUBJECT = "local-demo-manager-v1";
const STAFF_EMAIL = "local-demo-manager@example.invalid";
const STAFF_NAME = "Synthetic Demo Manager";
const CHILD_FIRST = "Alex";
const CHILD_LAST = "Demo";
const CHILD_DOB = "2011-06-15";

const domains = [
  "Emotional & Psychological Wellbeing",
  "Behaviour & Self-Regulation",
  "Relationships & Connection",
  "Daily Living, Learning & Participation",
  "Safety, Risk & Coping with Stress",
  "Skills, Identity & Future",
];

const priorityDomains = [
  "Behaviour & Self-Regulation",
  "Relationships & Connection",
  "Safety, Risk & Coping with Stress",
];

const json = (value) => JSON.stringify(value);

function stableUUID(input) {
  const hex = createHash("sha256").update(input).digest("hex");
  const variant = ((parseInt(hex.slice(16, 18), 16) & 0x3f) | 0x80).toString(16);
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-4${hex.slice(13, 16)}-${variant}${hex.slice(18, 20)}-${hex.slice(20, 32)}`;
}

function jwtPayload(token) {
  const [, payload] = String(token).split(".");
  return JSON.parse(Buffer.from(payload, "base64url").toString("utf8"));
}

function psql(sql, variables = {}) {
  const args = [DB_URL, "-X", "-q", "-t", "-A", "-v", "ON_ERROR_STOP=1"];
  for (const [key, value] of Object.entries(variables)) {
    args.push("-v", `${key}=${value}`);
  }
  return execFileSync("psql", args, { input: sql, encoding: "utf8" }).trim();
}

function psqlJSON(sql) {
  const result = psql(`SELECT COALESCE(json_agg(row_to_json(result)), '[]'::json) FROM (${sql}) result`);
  return JSON.parse(result || "[]");
}

async function api(path, { method = "GET", body, token } = {}) {
  const response = await fetch(`${API_BASE}${path}`, {
    method,
    headers: {
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...(body ? { "Content-Type": "application/json" } : {}),
    },
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!response.ok) {
    throw new Error(`${method} ${path} failed: ${response.status} ${await response.text()}`);
  }
  if (response.status === 204) return null;
  return response.json();
}

async function localManagerToken() {
  const response = await api("/internal/dev/local-manager-token", { method: "POST" });
  if (!response.token || String(response.token).split(".").length !== 3) {
    throw new Error("local development manager token was not issued");
  }
  return response.token;
}

function ensureBaseRecords() {
  const homeID = psql(`
UPDATE homes
SET name = 'myCareSignals Demo Home',
    external_ref = :'home_ref'
WHERE id = :'home_id'::bigint;
SELECT id FROM homes WHERE id = :'home_id'::bigint`, { home_ref: HOME_REF, home_id: FRONTEND_HOME_ID });

  const staffID = psql(`
WITH staff AS (
  INSERT INTO staff_users (email, display_name, external_auth_subject, status)
  VALUES (:'email', :'name', :'subject', 'active')
  ON CONFLICT (email) DO UPDATE
    SET display_name = EXCLUDED.display_name,
        external_auth_subject = EXCLUDED.external_auth_subject,
        status = 'active'
  RETURNING id
), role AS (
  INSERT INTO staff_home_roles (staff_user_id, home_id, role, active)
  SELECT id, :'home_id'::bigint, 'manager', TRUE FROM staff
  ON CONFLICT (staff_user_id, home_id, role) DO UPDATE SET active = TRUE
)
SELECT id FROM staff`, { email: STAFF_EMAIL, name: STAFF_NAME, subject: STAFF_SUBJECT, home_id: homeID });

  const childID = psql(`
WITH child AS (
  INSERT INTO children (
    home_id, external_ref, initials, first_name, last_name,
    date_of_birth, admission_date, legal_status, gender, active
  )
  SELECT :'home_id'::bigint, :'child_ref', 'AD', :'first_name', :'last_name',
    :'dob'::date, '2026-04-20'::date, 'Synthetic development placement', 'Not specified', TRUE
  WHERE NOT EXISTS (SELECT 1 FROM children WHERE external_ref = :'child_ref')
  RETURNING id
)
SELECT id FROM child
UNION ALL
SELECT id FROM children WHERE external_ref = :'child_ref'
LIMIT 1`, { home_id: homeID, child_ref: CHILD_REF, first_name: CHILD_FIRST, last_name: CHILD_LAST, dob: CHILD_DOB });

  return { homeID: Number(homeID), staffID: Number(staffID), childID: Number(childID) };
}

function ensureTokenManagerCanAccessDemoHome(token, homeID) {
  const subject = jwtPayload(token).sub;
  if (!subject) throw new Error("development manager token did not include a subject");
  psql(`
INSERT INTO staff_home_roles (staff_user_id, home_id, role, active)
SELECT id, :'home_id'::bigint, 'manager', TRUE
FROM staff_users
WHERE external_auth_subject = :'subject'
ON CONFLICT (staff_user_id, home_id, role) DO UPDATE SET active = TRUE`, { home_id: homeID, subject });
}

function seedFramework({ childID, staffID }) {
  const existing = Number(psql("SELECT count(*) FROM child_onboarding_baselines WHERE child_id = :'child_id'::bigint AND framework_model = 'six_domain_v1'", { child_id: childID }) || "0");
  if (existing > 0) return;

  psql(`
WITH baseline AS (
  INSERT INTO child_onboarding_baselines (
    child_id, status, framework_model, framework_version, onboarding_pathway,
    immediate_information, immediate_information_status,
    summary_of_history, background_information_status,
    known_safety_information, safety_information_status,
    next_review_date, lead_reviewer, created_by_staff_id,
    approved_by, approved_at
  )
  VALUES (
    :'child_id'::bigint, 'approved', 'six_domain_v1', 1, 'new_placement',
    'Clear explanations, one staff member taking the lead, and time to settle after changes.',
    'known_confirmed',
    'Synthetic stakeholder demo: strengths include cooking, football, and a positive key-worker relationship.',
    'known_confirmed',
    'Previous missing-from-home behaviour has been reported when Alex is upset; this is recorded for human review.',
    'known_confirmed',
    '2026-09-30'::date, 'Synthetic Demo Manager', :'staff_id'::bigint,
    'Synthetic Demo Manager', '2026-08-31T09:00:00Z'
  )
  RETURNING id
), domain_data(domain_key, active, reason, current_understanding, strengths, support_needs, what_helps, current_support, yp_view, info_status, source, details, source_date) AS (
  VALUES
    ('Emotional & Psychological Wellbeing', FALSE, '', 'Alex can reflect well once settled; distress may increase after unexpected contact changes.', 'Can talk reflectively after regulation.', 'Time and calm follow-up help emotions settle.', 'Predictable explanation and space.', 'Key worker offers check-ins after difficult moments.', 'Prefers calm conversations after space.', 'known_confirmed', 'staff_observation', 'Synthetic staff handover', '2026-05-01'::date),
    ('Behaviour & Self-Regulation', TRUE, 'Current priority because early coping steps reduce escalation.', 'Alex may escalate when overwhelmed but has started using agreed words to ask for space.', 'Can use agreed words with trusted staff.', 'Support to notice early signs and use coping strategy.', 'One lead staff member, low-demand language, time away from busy spaces.', 'Daily staff prompts and key-working practice.', 'Wants staff to notice when space is needed.', 'known_confirmed', 'staff_observation', 'Synthetic key-worker summary', '2026-05-08'::date),
    ('Relationships & Connection', TRUE, 'Current priority because family-contact changes can affect trust and communication.', 'Changes to family-contact arrangements can cause distress and withdrawal.', 'Positive relationship with key worker.', 'Clear explanations before and after changes.', 'Consistent lead staff member and planning check-in.', 'Weekly planning check-in with trusted staff.', 'Can share one view when the conversation feels predictable.', 'known_confirmed', 'young_person', 'Synthetic young-person planning note', '2026-06-01'::date),
    ('Daily Living, Learning & Participation', FALSE, '', 'Enjoys cooking and football; one education detail is still awaiting confirmation.', 'Cooking and football are motivating activities.', 'Education timetable detail awaiting confirmation.', 'Use interests to support routine and participation.', 'Staff support cooking activity and football attendance.', '', 'awaiting_confirmation', 'school_education', 'Synthetic education contact note', '2026-06-05'::date),
    ('Safety, Risk & Coping with Stress', TRUE, 'Current priority because missing-from-home planning is needed after upset.', 'Previous missing-from-home behaviour has occurred when upset; calm return and follow-up help.', 'Can return to communal areas after calming.', 'Support plan for upset following contact changes.', 'Immediate calm checks, known-place thinking, and warm return.', 'MFH plan and risk review remain human-led.', 'Wants staff to be calm if they return after time away.', 'known_confirmed', 'existing_risk_assessment', 'Synthetic risk planning note', '2026-06-10'::date),
    ('Skills, Identity & Future', FALSE, '', 'Non-urgent information about future independence preferences is not known yet.', 'Enjoys preparing simple meals.', 'Future independence preferences to explore later.', 'Build from cooking strengths.', 'Offer low-pressure cooking opportunities.', '', 'not_known_yet', 'staff_observation', 'Synthetic future-planning note', '2026-06-20'::date)
), inserted_domains AS (
  INSERT INTO child_onboarding_domain_states (
    baseline_id, domain_key, active, reason, current_understanding,
    strengths_protective_factors, support_needs, what_helps, current_support,
    young_person_view, information_status, information_source, source_details, source_date
  )
  SELECT baseline.id, domain_key, active, NULLIF(reason, ''), current_understanding,
    strengths, support_needs, what_helps, current_support, yp_view, info_status,
    source, details, source_date
  FROM baseline, domain_data
), priority_data(domain_key, focus, hoped, sort_order) AS (
  VALUES
    ('Behaviour & Self-Regulation', 'Use early coping steps before escalation.', 'Alex asks for space before reacting and can rejoin when calmer.', 1),
    ('Relationships & Connection', 'Keep contact-related communication predictable.', 'Alex shares one view during the weekly planning check-in.', 2),
    ('Safety, Risk & Coping with Stress', 'Keep missing-from-home planning calm and specific.', 'Staff follow the agreed response and Alex receives calm return support.', 3)
), inserted_priorities AS (
  INSERT INTO child_priority_domains (baseline_id, domain_key, focus_summary, hoped_for_change, sort_order)
  SELECT baseline.id, domain_key, focus, hoped, sort_order FROM baseline, priority_data
)
INSERT INTO child_onboarding_framework_sources (
  baseline_id, source_type, source_name, source_date, notes
)
SELECT baseline.id, source_type, source_name, source_date, notes
FROM baseline,
(VALUES
  ('staff_observation', 'Synthetic staff handover', '2026-05-01'::date, 'Strengths, regulation support and return routines. Status: known / confirmed.'),
  ('young_person', 'Synthetic planning conversation', '2026-06-01'::date, 'Views about space, calm follow-up and planning check-ins. Status: known / confirmed.'),
  ('school_education', 'Synthetic education note', '2026-06-05'::date, 'One education detail remains awaiting confirmation.')
) AS sources(source_type, source_name, source_date, notes)`, { child_id: childID, staff_id: staffID });
}

async function ensureGoalAndTarget(token, childID, goal, target) {
  const goals = await api(`/v1/children/${childID}/goals`, { token });
  let existingGoal = goals.find((item) => item.title === goal.title);
  if (!existingGoal) {
    existingGoal = await api(`/v1/children/${childID}/goals`, { method: "POST", token, body: goal });
  }
  const targets = await api(`/v1/children/${childID}/targets`, { token });
  let existingTarget = targets.find((item) => item.short_label === target.short_label);
  if (!existingTarget) {
    existingTarget = await api(`/v1/children/${childID}/targets`, {
      method: "POST",
      token,
      body: { ...target, goal_id: existingGoal.id },
    });
  }
  return existingTarget;
}

async function seedGoalsTargets(token, childID) {
  const targetA = await ensureGoalAndTarget(token, childID, {
    goal_type: "care_plan",
    title: "Use coping strategy before escalation",
    domain: "Safety, Risk & Coping with Stress",
    description: "Alex uses an agreed coping step before distress escalates.",
    start_date: "2026-05-01",
    target_date: "2026-10-31",
    status: "active",
    child_voice: "I want staff to give me space before it gets too much.",
  }, {
    domain: "Safety, Risk & Coping with Stress",
    short_label: "Use coping strategy before escalation",
    description: "Use the agreed calm-space routine before leaving or reacting.",
    status: "active",
    supports_needed: "One staff member calmly prompts the agreed strategy.",
    barriers: "Unexpected contact changes can make it harder to pause.",
    intervention_strategy: "Low-demand reminder, space, then calm follow-up.",
    review_date: "2026-09-30",
  });
  const targetB = await ensureGoalAndTarget(token, childID, {
    goal_type: "care_plan",
    title: "Ask for space before escalation",
    domain: "Behaviour & Self-Regulation",
    description: "Alex practises asking for space before distress escalates.",
    start_date: "2026-05-01",
    target_date: "2026-10-31",
    status: "active",
  }, {
    domain: "Behaviour & Self-Regulation",
    short_label: "Ask for space before escalation",
    description: "Use agreed words to ask for space before reacting.",
    status: "active",
    supports_needed: "Staff recognise early signs and respond consistently.",
    barriers: "Busy rooms and sudden changes can increase pressure.",
    intervention_strategy: "Offer one clear choice and reduce verbal demands.",
    review_date: "2026-09-30",
  });
  const targetC = await ensureGoalAndTarget(token, childID, {
    goal_type: "care_plan",
    title: "Share one view during the weekly planning check-in",
    domain: "Relationships & Connection",
    description: "Alex contributes one view in the weekly planning conversation.",
    start_date: "2026-05-01",
    target_date: "2026-10-31",
    status: "active",
  }, {
    domain: "Relationships & Connection",
    short_label: "Share one view during the weekly planning check-in",
    description: "Share one view with a trusted staff member during weekly planning.",
    status: "active",
    supports_needed: "Predictable questions and enough time to answer.",
    barriers: "Can disengage if plans feel sudden.",
    intervention_strategy: "Use the same weekly structure and write down choices.",
    review_date: "2026-09-30",
  });
  return { targetA, targetB, targetC };
}

async function seedRisk(token, childID, targetA) {
  const risks = await api(`/v1/children/${childID}/risks`, { token });
  const existing = risks.find((risk) => risk.title === "Missing From Home");
  if (existing) return existing;
  return api(`/v1/children/${childID}/risks`, {
    method: "POST",
    token,
    body: {
      title: "Missing From Home",
      category: "missing_from_home",
      domain: "Safety, Risk & Coping with Stress",
      status: "active",
      nature_of_risk: "Synthetic concern: Alex has previously left or considered leaving when upset after contact-related changes.",
      who_is_at_risk: "Alex",
      information_status: "current_confirmed",
      source_note: "Synthetic risk planning note, June 2026.",
      initial_likelihood: 3,
      initial_severity: 4,
      residual_likelihood: 2,
      residual_severity: 3,
      management_approach: "controls_and_developmental_work",
      proactive_strategies: "Explain changes early, identify one lead staff member, offer calm space, and use agreed words.",
      reactive_strategies: "Complete immediate welfare checks, consider known safe places, contact relevant professionals, and provide calm return support.",
      effective_from: "2026-06-10",
      next_review_date: "2026-09-30",
      target_ids: [targetA.id],
    },
  });
}

async function seedCarePlan(token, childID) {
  const current = await api(`/v1/children/${childID}/care-plans`, { token });
  if (current.plan) return current.plan;
  const created = await api(`/v1/children/${childID}/care-plans`, {
    method: "POST",
    token,
    body: { effective_from: "2026-05-01", review_date: "2026-09-30" },
  });
  const plan = created.plan;
  const sections = {
    about_me: ["known", "Alex enjoys cooking and football and can reflect well once settled.", "Cooking, football, humour and a trusted key-worker relationship.", "Use interests as low-pressure ways to connect.", "Alex has said space and calm explanations help."],
    emotional_wellbeing: ["partly_known", "Distress can increase when contact arrangements change suddenly.", "Can settle and reflect after time.", "Explain changes clearly and follow up calmly.", "Would rather talk after having space first."],
    safety_stability: ["known", "A Missing From Home risk and plan are in place for upset following contact changes.", "Can return to communal space after calming.", "Follow the MFH plan without duplicating it inside the care plan.", "Wants staff to stay calm if they are worried."],
    relationships_belonging: ["known", "A consistent key-worker relationship is protective.", "Can share one view in a predictable check-in.", "Keep weekly planning check-in consistent.", "Trusted staff member helps conversations feel safer."],
    education_future: ["awaiting_confirmation", "Education timetable detail is awaiting confirmation from a synthetic education contact.", "Cooking and football support participation.", "Use known interests while education detail is confirmed.", ""],
  };
  for (const [key, [information_status, current_understanding, strengths, staff_support, young_person_involvement_note]] of Object.entries(sections)) {
    await api(`/v1/children/${childID}/care-plans/${plan.id}/sections/${key}`, {
      method: "PUT",
      token,
      body: { information_status, current_understanding, strengths, staff_support, young_person_involvement_note },
    });
  }
  await api(`/v1/children/${childID}/care-plans/${plan.id}/activate`, { method: "POST", token });
  return (await api(`/v1/children/${childID}/care-plans`, { token })).plan;
}

async function seedMFHPlan(token, childID) {
  const workspace = await api(`/v1/children/${childID}/missing-from-home-plans`, { token });
  if (workspace.plan) return workspace.plan;
  const created = await api(`/v1/children/${childID}/missing-from-home-plans`, { method: "POST", token });
  const plan = created.plan;
  const draftWorkspace = await api(`/v1/children/${childID}/missing-from-home-plans`, { token });
  const content = {
    current_situation: {
      supervision_arrangement: "Synthetic demo: ordinary home supervision with extra calm check-ins after contact-related changes.",
      free_time_arrangement: "Free time is planned with one staff member clearly taking the lead.",
      confirmed_legal_context: "Synthetic development placeholder; no real legal details.",
      current_risk_context: "Active Missing From Home risk exists; this plan describes the operational response only.",
    },
    missing_history: {
      history_type: "previous_episodes",
      patterns: "Previous behaviour has been linked to feeling overwhelmed after changes.",
      circumstances: "Family/contact changes are a known synthetic trigger.",
      what_helped_return: "A calm message, time, and a familiar staff member helped re-engagement.",
      source: "Synthetic risk planning note",
      information_status: "known_confirmed",
    },
    people_places: [
      { kind: "family_contact", name_or_label: "Family-contact area category", details: "Synthetic place category only; no real address.", contact_or_route: "Follow local policy and known safe-place checks.", source: "Synthetic framework source", information_status: "partly_known" },
      { kind: "known_location", name_or_label: "Football pitch category", details: "Alex may seek familiar outdoor places.", contact_or_route: "Check agreed local area categories.", source: "Synthetic staff handover", information_status: "awaiting_confirmation" },
    ],
    triggers: [
      { description: "Unexpected change to family-contact arrangements.", source: "Synthetic framework source", information_status: "known_confirmed" },
      { description: "Feeling overwhelmed in a busy communal space.", source: "Synthetic staff observation", information_status: "partly_known" },
    ],
    operational_response: {
      immediate_checks: "Check immediate safety, room, garden and agreed calm spaces first.",
      contact_attempts: "One staff member sends calm, brief messages and avoids repeated pressure.",
      local_search_expectations: "Use local policy and agreed synthetic place categories.",
      police_policy_wording: "Follow home policy and professional judgement if Alex cannot be located.",
      information_for_police: "Provide only relevant synthetic description and known risk context in this demo.",
      safeguarding_considerations: "Consider emotional state, contact-change trigger and return support.",
    },
    notifications: [
      { role: "Manager", contact_name: "Duty manager", contact_details: "Internal demo contact only", when_to_notify: "Follow local missing-from-home policy.", source: "Synthetic notification plan" },
      { role: "Social worker", contact_name: "Allocated worker placeholder", contact_details: "Synthetic contact route only", when_to_notify: "As required by policy.", source: "Synthetic notification plan" },
    ],
    return_follow_up: {
      immediate_welfare_response: "Offer food, drink, warmth and calm reassurance.",
      medical_safeguarding_check: "Consider health and safeguarding checks according to policy.",
      calm_welcome_recovery_approach: "Keep the first response calm and non-punitive.",
      return_conversation: "Offer a short conversation after Alex has had time to settle.",
      independent_return_interview: "Arrange where policy requires.",
      professional_notifications: "Update relevant professionals with factual information.",
      key_working_follow_up: "Use the next key-working session to review coping words and contact-change planning.",
      consider_risk_review: true,
      consider_care_plan_review: true,
    },
  };
  await api(`/v1/children/${childID}/missing-from-home-plans/${plan.id}`, {
    method: "PUT",
    token,
    body: {
      content,
      source_decisions: (draftWorkspace.reuse_candidates || []).map((candidate) => ({ candidate_key: candidate.candidate_key, included: true })),
      review_date: "2026-09-30",
    },
  });
  await api(`/v1/children/${childID}/missing-from-home-plans/${plan.id}/submit`, { method: "POST", token });
  await api(`/v1/children/${childID}/missing-from-home-plans/${plan.id}/approve`, { method: "POST", token });
  return (await api(`/v1/children/${childID}/missing-from-home-plans`, { token })).plan;
}

async function seedDailyProgress(token, childID, targets) {
  const existing = Number(psql("SELECT count(*) FROM child_daily_progress WHERE child_id = :'child_id'::bigint", { child_id: childID }) || "0");
  if (existing >= 14) return;
  const rows = [
    ["2026-05-06", targets.targetB.id, 2, 2, "Alex accepted one prompt to step away from a busy room."],
    ["2026-05-17", targets.targetB.id, 3, 2, "Alex needed repeated staff support before settling."],
    ["2026-05-28", targets.targetB.id, 2, 2, "Alex used agreed words after staff reminded them."],
    ["2026-06-05", targets.targetB.id, 3, 3, "Alex asked for space before reacting."],
    ["2026-06-18", targets.targetB.id, 2, 3, "Alex returned to the communal area after calming."],
    ["2026-06-27", targets.targetC.id, 3, 3, "Alex shared one view during planning with a trusted staff member."],
    ["2026-07-08", targets.targetB.id, 2, 3, "Alex asked for space independently after a change was explained."],
    ["2026-07-19", targets.targetB.id, 1, 3, "Alex was unsettled but accepted one lead staff member supporting them."],
    ["2026-07-26", targets.targetA.id, 2, 3, "Alex used the agreed calm-space routine before leaving the area."],
    ["2026-08-04", targets.targetB.id, 2, 3, "Alex used agreed words before escalation."],
    ["2026-08-16", targets.targetB.id, 3, 3, "Alex returned after time away and reflected briefly."],
    ["2026-08-24", targets.targetB.id, 2, 3, "Alex needed a second prompt but then chose space."],
    ["2026-08-28", targets.targetC.id, 3, 3, "Alex named one preference for the next weekly plan."],
    ["2026-08-30", targets.targetA.id, 2, 3, "Alex accepted calm follow-up after becoming upset."],
  ];
  for (const [observed_on, target_id, wellbeing_score, progress_score, observation_note] of rows) {
    const count = Number(psql("SELECT count(*) FROM child_daily_progress WHERE child_id = :'child_id'::bigint AND observed_on = :'observed_on'::date AND target_id = :'target_id'::bigint", { child_id: childID, observed_on, target_id }) || "0");
    if (count > 0) continue;
    await api(`/v1/children/${childID}/daily-progress`, {
      method: "POST",
      token,
      body: { observed_on, target_id, domain: "", wellbeing_score, progress_score, observation_note, young_person_comment: "" },
    });
  }
}

async function seedKeyWorking(token, childID, targets) {
  const existing = Number(psql("SELECT count(*) FROM keyworking_sessions WHERE child_id = :'child_id'::bigint AND notes_summary LIKE 'Synthetic stakeholder demo:%'", { child_id: childID }) || "0");
  if (existing >= 5) return;
  const rows = [
    ["2026-06-12T10:00:00Z", "MCS-DEMO-KW-2026-06-12", targets.targetB.id, "Recognising early signs and asking for space."],
    ["2026-07-03T10:00:00Z", "MCS-DEMO-KW-2026-07-03", targets.targetB.id, "Practised the agreed words before escalation."],
    ["2026-07-24T10:00:00Z", "MCS-DEMO-KW-2026-07-24", targets.targetA.id, "Reflected after an upset and identified calm-space support."],
    ["2026-08-09T10:00:00Z", "MCS-DEMO-KW-2026-08-09", targets.targetB.id, "Reviewed asking for space after contact-related change."],
    ["2026-08-23T10:00:00Z", "MCS-DEMO-KW-2026-08-23", targets.targetC.id, "Shared one view for weekly planning."],
  ];
  for (const [session_at, external_ref, target_id, notes] of rows) {
    const count = Number(psql("SELECT count(*) FROM keyworking_sessions WHERE child_id = :'child_id'::bigint AND session_date = (:'session_at'::timestamptz)::date AND target_id = :'target_id'::bigint", { child_id: childID, session_at, target_id }) || "0");
    if (count > 0) continue;
    await api(`/v1/children/${childID}/sessions`, {
      method: "POST",
      token,
      body: {
        session_at,
        session_type: "keywork",
        external_ref,
        notes: `Synthetic stakeholder demo: ${notes}`,
        primary_domain: target_id === targets.targetC.id ? "Relationships & Connection" : "Behaviour & Self-Regulation",
        target_id,
        young_person_presentation: "Synthetic demo note: Alex engaged after a predictable start.",
        young_person_voice_note: "Synthetic demo note: Alex preferred calm wording and time.",
        key_worker_reflection: "Staff will keep explanations clear and use one lead worker.",
        signed_by_key_worker: true,
        scores: [{ target_id, wellbeing_score: 2, progress_score: 3, comment: "Synthetic key-working score." }],
      },
    });
  }
}

function seedMonthlyVoice({ childID, homeID, staffID }) {
  const ratings = {
    "2026-05-01": 2,
    "2026-06-01": 2,
    "2026-07-01": 1,
    "2026-08-01": 1,
  };
  for (const [month, behaviourRating] of Object.entries(ratings)) {
    const allRatings = Object.fromEntries(domains.map((domain) => [domain, domain === "Behaviour & Self-Regulation" ? behaviourRating : 2]));
    psql(`
WITH receipt AS (
  INSERT INTO external_feedback_receipts (
    intake_receipt_id, intake_type, source_type, home_id, child_id, received_at,
    requires_human_review, review_priority, matched_rule_ids, rule_version,
    raw_payload, intake_context_id, intake_purpose, period_month, domain_ratings,
    follow_up_requested, triage_status
  )
  VALUES (
    :'receipt_id', 'young_person_voice', 'YoungPersonVoice', :'home_id'::bigint, :'child_id'::bigint,
    (:'month'::date + interval '12 hours'), TRUE, 'routine', '[]'::jsonb, 'stakeholder-demo-v1',
    :'raw_payload'::jsonb, :'intake_context_id'::uuid, 'young_person_monthly_voice', :'month'::date, :'domain_ratings'::jsonb,
    FALSE, 'reviewed'
  )
  ON CONFLICT (intake_receipt_id) DO NOTHING
  RETURNING id, intake_context_id
), chosen AS (
  SELECT id, intake_context_id FROM receipt
  UNION ALL
  SELECT id, intake_context_id FROM external_feedback_receipts WHERE intake_receipt_id = :'receipt_id'
  LIMIT 1
), review AS (
  INSERT INTO external_feedback_reviews (
    receipt_id, home_id, reviewer_staff_id, reviewed_at, outcome, rationale,
    follow_up_required, child_id, domain
  )
  SELECT id, :'home_id'::bigint, :'staff_id'::bigint, (:'month'::date + interval '1 day'), 'link_as_evidence',
    'Synthetic stakeholder demo monthly voice linked as evidence.', FALSE, :'child_id'::bigint,
    'Behaviour & Self-Regulation'
  FROM chosen
  ON CONFLICT DO NOTHING
  RETURNING id, receipt_id
), chosen_review AS (
  SELECT id, receipt_id FROM review
  UNION ALL
  SELECT id, receipt_id FROM external_feedback_reviews
  WHERE receipt_id = (SELECT id FROM chosen)
    AND child_id = :'child_id'::bigint
    AND outcome = 'link_as_evidence'
  LIMIT 1
)
INSERT INTO child_monthly_voice_ratings (
  child_id, period_month, domain, young_person_rating,
  source_receipt_id, source_review_id, source_context_id, submitted_at
)
SELECT :'child_id'::bigint, :'month'::date, item.key, (item.value)::smallint,
  chosen.id, NULL, chosen.intake_context_id, (:'month'::date + interval '12 hours')
FROM chosen, jsonb_each_text(:'domain_ratings'::jsonb) AS item
ON CONFLICT (child_id, domain, period_month) DO NOTHING`, {
      receipt_id: stableUUID(`stakeholder-demo-voice-${CHILD_REF}-${month}`),
      intake_context_id: stableUUID(`stakeholder-demo-context-${CHILD_REF}-${month}`),
      home_id: homeID,
      child_id: childID,
      staff_id: staffID,
      month,
      raw_payload: json({ synthetic: true, demo: "stakeholder-story-v1", month }),
      domain_ratings: json(allRatings),
    });
  }
}

async function main() {
  const { homeID, staffID, childID } = ensureBaseRecords();
  const token = await localManagerToken();
  ensureTokenManagerCanAccessDemoHome(token, homeID);
  seedFramework({ childID, staffID });
  const targets = await seedGoalsTargets(token, childID);
  await seedRisk(token, childID, targets.targetA);
  await seedCarePlan(token, childID);
  await seedMFHPlan(token, childID);
  await seedDailyProgress(token, childID, targets);
  await seedKeyWorking(token, childID, targets);
  seedMonthlyVoice({ childID, homeID, staffID });

  const comparison = await api(`/v1/children/${childID}/daily-progress/monthly?from=2026-05&to=2026-08`, { token });
  const behaviour = comparison.items.filter((item) => item.domain === "Behaviour & Self-Regulation");
  const july = behaviour.find((item) => item.period_month === "2026-07");
  const august = behaviour.find((item) => item.period_month === "2026-08");
  const carePlan = await api(`/v1/children/${childID}/care-plans`, { token });
  const reviewEvidence = carePlan.plan
    ? await api(`/v1/children/${childID}/care-plans/${carePlan.plan.id}/review-evidence`, { token })
    : null;
  const summary = {
    homeID,
    childID,
    childHomeURL: `http://127.0.0.1:4174/children/${childID}`,
    july: july ? { staff: july.progress.median, voice: july.young_person_rating, difference: july.perspective_difference, prompt: july.persistent_review_suggested } : null,
    august: august ? { staff: august.progress.median, voice: august.young_person_rating, difference: august.perspective_difference, prompt: august.persistent_review_suggested } : null,
    reviewEvidence,
    counts: psqlJSON(`
      SELECT
        (SELECT count(*) FROM child_priority_domains p JOIN child_onboarding_baselines b ON b.id = p.baseline_id WHERE b.child_id = ${childID}) AS priority_domains,
        (SELECT count(*) FROM child_targets WHERE child_id = ${childID} AND status = 'active') AS active_targets,
        (SELECT count(*) FROM child_daily_progress WHERE child_id = ${childID}) AS daily_progress,
        (SELECT count(*) FROM keyworking_sessions WHERE child_id = ${childID}) AS keyworking,
        (SELECT count(*) FROM child_monthly_voice_ratings WHERE child_id = ${childID} AND period_month BETWEEN DATE '2026-05-01' AND DATE '2026-08-01') AS monthly_voice,
        (SELECT count(*) FROM child_risks WHERE child_id = ${childID} AND status = 'active') AS active_risks,
        (SELECT count(*) FROM child_missing_from_home_plans WHERE child_id = ${childID} AND status = 'approved') AS approved_mfh,
        (SELECT count(*) FROM child_care_plans WHERE child_id = ${childID} AND status = 'active') AS active_care_plan
    `)[0],
  };
  console.log(JSON.stringify(summary, null, 2));
}

main().catch((error) => {
  console.error(error.message);
  process.exit(1);
});
