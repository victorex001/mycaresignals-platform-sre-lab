import type {
  CarePlanInformationStatus,
  CarePlanSectionKey,
  ChildCarePlan,
  ChildCarePlanReview,
} from "./types";

export type CarePlanSectionConfig = {
  key: CarePlanSectionKey;
  title: string;
  description: string;
  domainTerms: string[];
};

export const CARE_PLAN_SECTIONS: CarePlanSectionConfig[] = [
  { key: "about_me", title: "About Me / What Matters", description: "What matters to the young person and what staff need to understand about them.", domainTerms: [] },
  { key: "safety_stability", title: "Safety & Stability", description: "Current safety, stability and the everyday approach that helps.", domainTerms: ["safety", "risk", "coping with stress"] },
  { key: "emotional_wellbeing", title: "Emotional Wellbeing", description: "Feelings, regulation, coping and psychological wellbeing.", domainTerms: ["emotional", "psychological", "behaviour", "self-regulation", "coping"] },
  { key: "health", title: "Health", description: "Physical health, healthcare and a healthy everyday environment.", domainTerms: ["health", "physical"] },
  { key: "education_future", title: "Education & Future", description: "Learning, participation, aspirations and preparation for the future.", domainTerms: ["education", "learning", "participation", "future"] },
  { key: "relationships_belonging", title: "Relationships & Belonging", description: "Trust, attachment, family, peers and a sense of belonging.", domainTerms: ["relationship", "connection", "belonging"] },
  { key: "independence_daily_living", title: "Independence & Daily Living", description: "Routines, practical skills, participation and growing independence.", domainTerms: ["daily living", "independence", "skills", "participation"] },
  { key: "identity_culture", title: "Identity & Culture", description: "Identity, culture, beliefs, strengths and sense of self.", domainTerms: ["identity", "culture", "skills", "future"] },
];

export const CARE_PLAN_INFORMATION_STATUSES: Array<{
  value: CarePlanInformationStatus;
  label: string;
}> = [
  { value: "known", label: "Known" },
  { value: "partly_known", label: "Partly known" },
  { value: "awaiting_confirmation", label: "Awaiting confirmation" },
  { value: "not_known_yet", label: "Not known yet" },
  { value: "not_applicable", label: "Not applicable" },
];

export const carePlanStatusLabel = (value: string) =>
  value.replaceAll("_", " ").replace(/^./, (letter) => letter.toUpperCase());

export const carePlanReviewOutcomeLabel = (review: ChildCarePlanReview) =>
  review.outcome === "continue" && review.review_note.startsWith("Needs more information.")
    ? "Needs more information"
    : carePlanStatusLabel(review.outcome);

export const carePlanDate = (value?: string | null) => {
  if (!value) return "Not set";
  const [year, month, day] = value.slice(0, 10).split("-").map(Number);
  return year && month && day
    ? new Date(year, month - 1, day).toLocaleDateString(undefined, { dateStyle: "medium" })
    : value;
};

export const carePlanSection = (plan: ChildCarePlan, key: CarePlanSectionKey) =>
  plan.sections.find((section) => section.section_key === key);

export const carePlanReadiness = (plan: ChildCarePlan) => {
  const understood = plan.sections.filter((section) =>
    ["known", "partly_known", "not_applicable"].includes(section.information_status),
  ).length;
  const needsInformation = plan.sections.filter((section) =>
    ["not_known_yet", "awaiting_confirmation"].includes(section.information_status),
  ).length;
  return { understood, needsInformation };
};

export const carePlanVersion = (plans: ChildCarePlan[], planID: number) =>
  Math.max(1, plans.findIndex((plan) => plan.id === planID) + 1);

export const carePlanDomainMatches = (
  domain: string | null | undefined,
  section: CarePlanSectionConfig,
) => {
  if (!domain || section.domainTerms.length === 0) return false;
  const normalized = domain.toLowerCase();
  return section.domainTerms.some((term) => normalized.includes(term));
};
