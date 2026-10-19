// src/types.ts

export type OverallIndex = {
  index: number;
  previous_index?: number | null;
  change?: number | null;
};
// Child as returned from /v1/homes/{homeID}/children/onboard
export type Child = {
  id: number;
  home_id: number;
  external_ref?: string | null;
  initials: string;
  first_name?: string | null;
  last_name?: string | null;
  date_of_birth: string;
  legal_status?: string | null;
  admission_date?: string | null;
  gender?: string | null;
  active?: boolean;
  created_at?: string;
  updated_at?: string;
  baseline_status?: string;
};

export type GoalStatus = "active" | "paused" | "achieved" | "closed";

// Care-plan goals
export type ChildGoal = {
  id: number;
  child_id: number;
  goal_type: string; // e.g. "care_plan"
  title: string;
  domain?: string | null;
  description: string;
  start_date?: string | null;
  target_date?: string | null;
  status: GoalStatus | string;
  created_at: string;
  updated_at: string;

  // Psychological extras
  child_voice?: string | null;
  formulation_summary?: string | null;
  protective_factors?: string | null;
};

// Key-working targets
export type ChildTarget = {
  id: number;
  child_id: number;
  goal_id?: number | null;
  domain: string;
  short_label: string;
  description: string;
  status: GoalStatus | string;
  created_at: string;
  updated_at: string;

  // NEW psychological fields
  barriers?: string | null;
  supports_needed?: string | null;
  intervention_strategy?: string | null;
  review_date?: string | null;
  daily_observation_count: number;
  keyworking_session_count: number;
};

export type ChildDailyProgress = {
  id: number;
  child_id: number;
  recorded_by_staff_id: number;
  recorded_by_staff_name: string;
  observed_on: string;
  domain: string;
  target_id?: number | null;
  target_label?: string | null;
  wellbeing_score?: number | null;
  progress_score?: number | null;
  observation_note?: string | null;
  young_person_comment?: string | null;
  created_at: string;
};

export type MonthlyScoreStatistics = {
  count: number;
  distribution: Record<string, number>;
  mean?: number;
  median?: number;
  min?: number;
  max?: number;
  latest?: number;
};

export type MonthlyPerspectiveComparison = {
  period_month: string;
  domain: string;
  observation_count: number;
  wellbeing: MonthlyScoreStatistics;
  progress: MonthlyScoreStatistics;
  staff_descriptive_value?: number;
  staff_value_basis: string;
  young_person_rating?: number;
  young_person_source?: string;
  young_person_source_receipt_id?: number;
  young_person_submitted_at?: string;
  perspective_difference?: number;
  keyworking_session_count: number;
  comparison_status: string;
  persistent_review_suggested: boolean;
  review_suggestion?: string;
  rule_version: string;
};

export type MFHInformationStatus =
  | "known_confirmed"
  | "partly_known"
  | "awaiting_confirmation"
  | "not_known_yet";

export type MFHPlanStatus = "draft" | "submitted" | "approved" | "superseded" | "archived";

export type MFHPlanContent = {
  current_situation: {
    supervision_arrangement: string;
    free_time_arrangement: string;
    confirmed_legal_context: string;
    current_risk_context: string;
  };
  missing_history: {
    history_type: string;
    patterns: string;
    circumstances: string;
    what_helped_return: string;
    source: string;
    information_status: MFHInformationStatus | "";
  };
  people_places: Array<{
    kind: string;
    name_or_label: string;
    details: string;
    contact_or_route: string;
    source: string;
    information_status: MFHInformationStatus;
  }>;
  triggers: Array<{
    description: string;
    source: string;
    information_status: MFHInformationStatus;
  }>;
  operational_response: {
    immediate_checks: string;
    contact_attempts: string;
    local_search_expectations: string;
    police_policy_wording: string;
    information_for_police: string;
    safeguarding_considerations: string;
  };
  notifications: Array<{
    role: string;
    contact_name: string;
    contact_details: string;
    when_to_notify: string;
    source: string;
  }>;
  return_follow_up: {
    immediate_welfare_response: string;
    medical_safeguarding_check: string;
    calm_welcome_recovery_approach: string;
    return_conversation: string;
    independent_return_interview: string;
    professional_notifications: string;
    key_working_follow_up: string;
    consider_risk_review: boolean;
    consider_care_plan_review: boolean;
  };
};

export type MFHReuseCandidate = {
  candidate_key: string;
  category: "current_situation" | "triggers" | "notifications" | string;
  source_type: string;
  source_id?: number;
  label: string;
  value: string;
  source_label: string;
  information_status: MFHInformationStatus;
  source_updated_at?: string;
};

export type MFHSourceSnapshot = MFHReuseCandidate & {
  snapshot_sha256: string;
  included: boolean;
  changed: boolean;
};

export type MissingFromHomePlan = {
  id: number;
  child_id: number;
  version: number;
  status: MFHPlanStatus;
  predecessor_plan_id?: number;
  content: MFHPlanContent;
  source_snapshots: MFHSourceSnapshot[];
  sources_changed: boolean;
  review_date?: string;
  created_by_staff_id: number;
  created_by_name: string;
  submitted_by_name?: string;
  approved_by_name?: string;
  archived_by_name?: string;
  submitted_at?: string;
  approved_at?: string;
  archived_at?: string;
  created_at: string;
  updated_at: string;
};

export type MFHWorkspace = {
  plan: MissingFromHomePlan | null;
  history: MissingFromHomePlan[];
  reuse_candidates: MFHReuseCandidate[];
  can_edit: boolean;
  can_approve: boolean;
};

export type RiskStatus = "active" | "paused" | "closed";
export type RiskInformationStatus =
  | "current_confirmed"
  | "reported"
  | "historical"
  | "allegation_not_established"
  | "awaiting_confirmation";
export type RiskManagementApproach = "controls_only" | "controls_and_developmental_work";

export type ChildRisk = {
  id: number;
  child_id: number;
  title: string;
  category: string;
  domain?: string | null;
  status: RiskStatus;
  nature_of_risk: string;
  who_is_at_risk?: string | null;
  information_status: RiskInformationStatus;
  source_note?: string | null;
  initial_likelihood?: number | null;
  initial_severity?: number | null;
  residual_likelihood?: number | null;
  residual_severity?: number | null;
  management_approach: RiskManagementApproach;
  proactive_strategies?: string | null;
  reactive_strategies?: string | null;
  effective_from?: string | null;
  last_reviewed_at?: string | null;
  next_review_date?: string | null;
  created_at: string;
  updated_at: string;
  goal_ids: number[];
  target_ids: number[];
};

export type RiskReviewOutcome = "continue" | "modify" | "close";

export type ChildRiskReview = {
  id: number;
  risk_id: number;
  child_id: number;
  reviewed_at: string;
  reviewer_staff_id: number;
  reviewer_display_name: string;
  previous_residual_likelihood?: number | null;
  previous_residual_severity?: number | null;
  reviewed_residual_likelihood: number;
  reviewed_residual_severity: number;
  review_note: string;
  outcome: RiskReviewOutcome;
  next_review_date?: string | null;
  created_at: string;
};

export type CarePlanStatus = "draft" | "active" | "archived";
export type CarePlanInformationStatus =
  | "known"
  | "partly_known"
  | "not_known_yet"
  | "awaiting_confirmation"
  | "not_applicable";
export type CarePlanSectionKey =
  | "about_me"
  | "safety_stability"
  | "emotional_wellbeing"
  | "health"
  | "education_future"
  | "relationships_belonging"
  | "independence_daily_living"
  | "identity_culture";

export type ChildCarePlanSection = {
  id: number;
  care_plan_id: number;
  section_key: CarePlanSectionKey;
  information_status: CarePlanInformationStatus;
  current_understanding?: string | null;
  strengths?: string | null;
  staff_support?: string | null;
  young_person_involvement_note?: string | null;
  created_at: string;
  updated_at: string;
};

export type ChildCarePlan = {
  id: number;
  child_id: number;
  status: CarePlanStatus;
  effective_from: string;
  review_date?: string | null;
  predecessor_care_plan_id?: number | null;
  created_by_staff_id: number;
  activated_by_staff_id?: number | null;
  activated_at?: string | null;
  archived_by_staff_id?: number | null;
  archived_at?: string | null;
  created_at: string;
  updated_at: string;
  sections: ChildCarePlanSection[];
};

export type CarePlanReviewOutcome = "continue" | "revise" | "close";

export type CarePlanReviewEvidence = {
  evidence_since: string;
  active_goal_count: number;
  active_target_count: number;
  achieved_target_count: number;
  paused_target_count: number;
  targets_changed_count: number;
  targets_due_count: number;
  active_risk_count: number;
  risk_reviews_completed_count: number;
  keyworking_session_count: number;
  sections_not_known_count: number;
  sections_awaiting_confirmation_count: number;
};

export type ChildCarePlanReviewSection = {
  id: number;
  review_id: number;
  section_key: CarePlanSectionKey;
  information_status: CarePlanInformationStatus;
  current_understanding?: string | null;
  strengths?: string | null;
  staff_support?: string | null;
  young_person_involvement_note?: string | null;
  created_at: string;
};

export type ChildCarePlanReview = {
  id: number;
  care_plan_id: number;
  child_id: number;
  reviewed_at: string;
  reviewer_staff_id: number;
  reviewer_display_name: string;
  review_note: string;
  young_person_involvement_note?: string | null;
  outcome: CarePlanReviewOutcome;
  next_review_date?: string | null;
  successor_care_plan_id?: number | null;
  snapshot_status: CarePlanStatus;
  snapshot_effective_from: string;
  snapshot_review_date?: string | null;
  evidence: CarePlanReviewEvidence;
  created_at: string;
  sections: ChildCarePlanReviewSection[];
};

export type CareOverviewDomainPriority = {
  domain_key: string;
  label: string;
  focus_summary?: string;
  hoped_for_change?: string;
  active_goals: number;
  active_targets: number;
  active_risks: number;
  recent_keyworking: number;
  target_reviews_due: number;
};

export type CareOverviewTimelineEvent = {
  occurred_at: string;
  type: string;
  title: string;
  detail?: string;
  source: string;
};

export type FeedbackTriageStatus = "new" | "acknowledged" | "follow_up" | "reviewed";
export type FeedbackReviewOutcome = "acknowledge" | "no_further_action" | "follow_up_required" | "link_as_evidence" | "escalate_for_risk_review";

export type ExternalFeedbackReview = {
  id: number;
  receipt_id: number;
  home_id: number;
  reviewer_staff_id: number;
  reviewer_display_name: string;
  reviewed_at: string;
  outcome: FeedbackReviewOutcome;
  rationale: string;
  follow_up_required: boolean;
  child_id?: number;
  domain?: string;
  goal_id?: number;
  target_id?: number;
  risk_id?: number;
};

export type ExternalFeedbackReceipt = {
  id: number;
  intake_receipt_id: string;
  intake_type: "young_person_voice";
  source_type: "YoungPersonVoice";
  home_id: number;
  child_id?: number;
  received_at: string;
  requires_human_review: boolean;
  review_priority: "routine" | "priority" | "urgent";
  matched_rule_ids: string[];
  rule_version: string;
  raw_payload?: Record<string, unknown> | null;
  visitor_rating?: number;
  intake_context_id?: string;
  intake_purpose?: string;
  period_month?: string;
  domain_ratings?: Record<string, number>;
  follow_up_requested: boolean;
  follow_up_preference?: string;
  triage_status: FeedbackTriageStatus;
  reviews?: ExternalFeedbackReview[];
};

export type ChildCareOverview = {
  child: {
    display_name: string;
    initials: string;
    home_name: string;
    admission_date?: string | null;
  };
  baseline: {
    status: string;
    created_at?: string | null;
    approved_at?: string | null;
  };
  current_priorities: CareOverviewDomainPriority[];
  recent_daily_progress: Array<{
    observed_on: string;
    domain: string;
    target_label?: string;
    wellbeing_score?: number | null;
    progress_score?: number | null;
    observation_note?: string;
    recorded_by_staff_name: string;
  }>;
  daily_progress_review_suggested: boolean;
  goals_targets: {
    active_goals: number;
    active_targets: number;
    targets_due: number;
    targets_overdue: number;
    achieved_recently: number;
    goals: Array<{ title: string; domain?: string; status: string }>;
    targets: Array<{
      label: string;
      goal_title?: string;
      domain?: string;
      review_date?: string | null;
      status: string;
    }>;
  };
  risk_safety: {
    active_risks: number;
    recorded_low: number;
    recorded_medium: number;
    recorded_high: number;
    recorded_not_rated: number;
    reviews_due: number;
    recent_completed_reviews: number;
    developmental_work_missing_links: number;
    items: Array<{
      title: string;
      domain?: string;
      recorded_rating?: number | null;
      recorded_band?: string;
      next_review_date?: string | null;
    }>;
  };
  care_plan: {
    exists: boolean;
    status?: string;
    version?: number;
    effective_from?: string | null;
    review_date?: string | null;
    sections_not_known: number;
    sections_awaiting_confirmation: number;
    latest_review_outcome?: string;
    latest_reviewed_at?: string | null;
    successor_draft_exists: boolean;
    successor_draft_version?: number;
  };
  recent_keyworking: Array<{
    session_at: string;
    target_label?: string;
    goal_title?: string;
    domain?: string;
    signed_by_key_worker: boolean;
  }>;
  child_voice: {
    recent_entry_count: number;
    latest_date?: string | null;
  };
  needs_attention: Array<{
    kind: string;
    label: string;
    due_date?: string | null;
    priority: number;
    source: string;
  }>;
  timeline: CareOverviewTimelineEvent[];
};

// Logged key-working sessions
export type KeyworkSession = {
  id: number;
  child_id: number;
  session_at: string; // ISO representation of the persisted session date
  session_type: string;
  staff_name: string;
  notes: string;
  primary_domain?: string | null;
  target_id?: number | null;
  young_person_presentation?: string | null;
  young_person_voice_note?: string | null;
  key_worker_reflection?: string | null;
  signed_by_key_worker: boolean;
  key_worker_signed_at?: string | null;
  signed_by_manager: boolean;
  manager_signed_at?: string | null;
  manager_review_decision?: "approved" | "returned_for_review" | null;
  manager_review_comment?: string | null;
  manager_reviewer_name?: string | null;
  can_manager_review: boolean;
  scores: Array<{
    target_id: number;
    wellbeing_score?: number | null;
    progress_score?: number | null;
    comment?: string;
  }>;
  created_at: string;
};

export type KeyworkManagerReview = {
  session_id: number;
  decision: "approved" | "returned_for_review";
  comment?: string | null;
  reviewer_name: string;
  reviewed_at: string;
};

// Flattened timeline view of scores
export type TimelineItem = {
  session_id: number;
  session_at: string;
  staff_name: string;
  target_id: number;
  target_name: string;
  target_domain: string;
  wellbeing_score: number;
  progress_score: number;
  comment: string;
};

// Domain index for one domain (e.g. emotional / education)
export type DomainIndex = {
  domain: string;
  average_wellbeing: number;
  average_progress: number;
  index: number;
  previous_index?: number | null;
  change?: number | null;
};

// Child-level domain index payload
export type ChildDomainIndex = {
  child_id: number;
  window_days: number;
  overall: OverallIndex;
  domains: DomainIndex[];
};

export type PsychologicalReview = {
  id: number;
  child_id: number;
  window_days: number;
  review_from: string;         // ISO date
  review_to: string;           // ISO date
  formulation: string;
  protective_factors: string;
  recommendations: string;
  created_at: string;
  created_by: string;
};

// --- Child + analytics types ---

// export interface Child {
//   id: number;
//   home_id: number;
//   external_ref?: string | null;
//   initials: string;
//   first_name?: string | null;
//   last_name?: string | null;
//   date_of_birth?: string | null;
//   admission_date?: string | null;
//   legal_status?: string | null;
//   gender?: string | null;
// }

// // One domain (behaviour, emotional, safety_risk, etc.)
// export interface DomainIndex {
//   domain: string;
//   average_wellbeing: number;
//   average_progress: number;
//   index: number;
//   previous_index?: number | null;
//   change?: number | null;
// }

// export interface OverallIndex {
//   index: number;
//   previous_index?: number | null;
//   change?: number | null;
// }

// // Full payload from GET /v1/children/{id}/domain_index
// export interface ChildDomainIndex {
//   child_id: number;
//   window_days: number;
//   overall: OverallIndex;
//   domains: DomainIndex[];
// }
