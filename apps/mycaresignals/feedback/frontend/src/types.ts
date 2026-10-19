// frontend/src/types.ts

import type { MessageResponse } from "@mycaresignals/shared/types/http";

// Mirror of Go: models.FeedbackIn
export type WhoOption =
  | "SocialWorker"
  | "Ofsted"
  | "Parent"
  | "Professional"
  | "Other";

export interface FeedbackIn {
  source: string;
  who: WhoOption;
  reason: string;
  rating: number | null;
  what_went_well: string;
  what_to_improve: string;
  recommend: boolean;
  contact_name: string;
  contact_email: string;

  house_rating: number | null;
  staff_rating: number | null;
  felt_safe: "yes" | "no" | "unsure" | "";
  open_comments: string;
  contact_wanted: boolean;
}

export interface FeedbackResponse extends MessageResponse {
  feedback_id: string;
}
