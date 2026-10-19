// src/api/qrClient.ts
// Admin requests use the same QR CloudFront origin as the public forms.
import type { MessageResponse } from "@mycaresignals/shared/types/http";

export const QR_API_BASE = "";

export type VisitorWho =
  | "SocialWorker"
  | "Ofsted"
  | "Parent"
  | "Professional"
  | "Other";

export interface VisitorFeedbackPayload {
  context_token: string;
  who: VisitorWho;
  reason?: string;
  rating?: number;
  what_went_well: string;
  what_to_improve?: string;
  recommend: boolean;
  contact_name?: string;
  contact_email?: string;

  house_rating?: number;
  staff_rating?: number;
  felt_safe?: "yes" | "no" | "unsure";
  open_comments?: string;
  contact_wanted?: boolean;
}

export interface YoungVoicePayload {
  context_token: string;
  young_person_alias?: string;
  period_month?: string; // "YYYY-MM"

  q1_overall: string;
  q1_best?: string;
  q1_hardest?: string;

  q2_safe?: string;
  q2_unsafe?: string;
  q2_support_who?: string;

  q3_listened?: string;
  q3_trusted_adult?: string;
  q3_change_support?: string;

  q4_peers?: string;
  q4_unkind?: string;
  q4_help_with_peers?: string;

  q5_enjoy?: string;
  q5_wish_more?: string;
  q5_house_change?: string;

  q6_education?: string;
  q6_education_help?: string;

  q7_contact_feelings?: string;
  q7_contact_change?: string;

  q8_say_in_decisions?: string;
  q8_change_example?: string;

  q9_worries?: string;
  q9_one_change?: string;
  q9_want_talk?: boolean;
  q9_follow_up_preference?: string;

  q10_what_tell_ofsted?: string;
  q10_never_forget?: string;
}

export async function submitVisitorFeedback(payload: VisitorFeedbackPayload) {
  const res = await fetch(`${QR_API_BASE}/api/v1/feedback`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });

  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(data.error || "Failed to submit feedback");
  }
  return data as MessageResponse & { feedback_id: string };
}

export async function submitYoungVoice(payload: YoungVoicePayload) {
  const res = await fetch(`${QR_API_BASE}/api/v1/young-voice`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });

  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(data.error || "Failed to submit young person feedback");
  }
  return data as MessageResponse & {
    young_voice_id: string;
  };
}
