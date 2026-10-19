// src/safeguarding.ts
import type { ChildDomainIndex, DomainIndex } from "./types";

export type SafeguardingLevel = "none" | "amber" | "red";

export interface SafeguardingFlag {
  level: SafeguardingLevel;
  headline: string;
  reasons: string[];
}

const getChange = (d?: DomainIndex): number | null => {
  if (!d) return null;
  if (d.change === null || d.change === undefined) return null;
  return d.change;
};

const maxLevel = (a: SafeguardingLevel, b: SafeguardingLevel): SafeguardingLevel => {
  const order: SafeguardingLevel[] = ["none", "amber", "red"];
  return order[Math.max(order.indexOf(a), order.indexOf(b))];
};

/**
 * Compute an early-warning / safeguarding flag from the existing
 * ChildDomainIndex payload.
 *
 * Assumes domain names like: "risk", "emotional", "behaviour", "education".
 */
export function computeSafeguardingFlag(
  idx: ChildDomainIndex | null | undefined
): SafeguardingFlag {
  if (!idx || !idx.domains || idx.domains.length === 0) {
    return { level: "none", headline: "", reasons: [] };
  }

  const findDomain = (name: string): DomainIndex | undefined =>
    idx.domains.find((d) => d.domain === name);

  const risk = findDomain("risk");
  const emotional = findDomain("emotional");
  const behaviour = findDomain("behaviour");
  const education = findDomain("education");

  let level: SafeguardingLevel = "none";
  const reasons: string[] = [];

  // --- Rule 1: Big deterioration in Risk / safety ---
  if (risk) {
    const change = getChange(risk);
    if (change !== null) {
      if (change <= -3) {
        level = "red";
        reasons.push(
          "Safety / risk scores have dropped significantly compared to the previous period."
        );
      } else if (change <= -1.5) {
        level = maxLevel(level, "amber");
        reasons.push(
          "Safety / risk scores have started to deteriorate compared to the previous period."
        );
      }
    }

    // Also flag if the current risk index is very low (0–10 scale)
    if (risk.index <= 3) {
      level = maxLevel(level, "amber");
      reasons.push("Safety / risk scores are currently in a low range.");
    }
  }

  // --- Rule 2: Emotional + Behaviour both deteriorating ---
  const emoChange = getChange(emotional);
  const behChange = getChange(behaviour);

  if (emoChange !== null && behChange !== null && emoChange <= -2 && behChange <= -2) {
    level = maxLevel(level, "amber");
    reasons.push(
      "Emotional wellbeing and behaviour / self-regulation have both deteriorated since the previous period."
    );
  }

  // --- Rule 3: Emotional + Behaviour + Education all soft-deteriorating => bump to red ---
  const eduChange = getChange(education);
  if (
    emoChange !== null &&
    behChange !== null &&
    eduChange !== null &&
    emoChange <= -2 &&
    behChange <= -2 &&
    eduChange <= -1.5
  ) {
    level = "red";
    reasons.push(
      "Emotional wellbeing, behaviour and daily learning / participation are all worsening – please review support and safety plans."
    );
  }

  if (level === "none") {
    return { level: "none", headline: "", reasons: [] };
  }

  const headline =
    level === "red"
      ? "Safeguarding trend alert – please review safety and support."
      : "Early warning – patterns worth reflecting on in the team.";

  return { level, headline, reasons };
}