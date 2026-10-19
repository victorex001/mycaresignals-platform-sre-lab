export const CANONICAL_DOMAINS = [
  {
    key: "Emotional & Psychological Wellbeing",
    help: "Feelings, worries, confidence and emotional wellbeing.",
  },
  {
    key: "Behaviour & Self-Regulation",
    help: "Coping, regulation and responding safely when things feel difficult.",
  },
  {
    key: "Relationships & Connection",
    help: "Trust, belonging and relationships with family, peers and staff.",
  },
  {
    key: "Daily Living, Learning & Participation",
    help: "Routines, education, interests and taking part in daily life.",
  },
  {
    key: "Safety, Risk & Coping with Stress",
    help: "Known safety information, coping and support that needs human review.",
  },
  {
    key: "Skills, Identity & Future",
    help: "Identity, strengths, independence and hopes for the future.",
  },
] as const;

export type CanonicalDomain = (typeof CANONICAL_DOMAINS)[number]["key"];

export const INFORMATION_STATUSES = [
  { value: "known_confirmed", label: "Known / confirmed" },
  { value: "partly_known", label: "Partly known" },
  { value: "awaiting_confirmation", label: "Awaiting confirmation" },
  { value: "not_known_yet", label: "Not known yet" },
] as const;

export type InformationStatus = (typeof INFORMATION_STATUSES)[number]["value"];

export const FRAMEWORK_SOURCES = [
  { value: "local_authority_information", label: "Local Authority information" },
  { value: "existing_care_plan", label: "Existing Care Plan" },
  { value: "placement_plan", label: "Placement Plan" },
  { value: "existing_risk_assessment", label: "Existing Risk Assessment" },
  { value: "missing_from_home_plan", label: "Missing From Home Plan" },
  { value: "health_plan", label: "Health Plan" },
  { value: "education_pep", label: "Education / PEP" },
  { value: "behaviour_support_plan", label: "Behaviour / support plan" },
  { value: "contact_arrangements", label: "Contact arrangements" },
  { value: "berri_assessment", label: "BERRI assessment" },
  { value: "social_worker", label: "Social worker" },
  { value: "young_person", label: "Young person" },
  { value: "parent_family", label: "Parent / family" },
  { value: "school_education", label: "School / education" },
  { value: "health_professional", label: "Health professional" },
  { value: "staff_observation", label: "Staff observation" },
  { value: "other_professional_report", label: "Other professional report" },
  { value: "other", label: "Other" },
] as const;

export type FrameworkSource = (typeof FRAMEWORK_SOURCES)[number]["value"];

export const informationStatusLabel = (value: string) =>
  INFORMATION_STATUSES.find((item) => item.value === value)?.label ?? value;

export const frameworkSourceLabel = (value: string) =>
  FRAMEWORK_SOURCES.find((item) => item.value === value)?.label ?? value;
