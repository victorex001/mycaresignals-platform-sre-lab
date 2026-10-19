// src/components/EmojiRating.tsx
import React from "react";

const EMOJIS = ["😡", "😕", "😐", "🙂", "🤩"];
const LABELS = ["Very difficult", "Difficult", "Mixed", "Good", "Really good"];

type Props = {
  label: string;
  supportingText?: string;
  value: number | null;
  onChange: (val: number) => void;
  required?: boolean;
};

const EmojiRating: React.FC<Props> = ({ label, supportingText, value, onChange, required }) => {
  return (
    <fieldset className="yv-rating-card" aria-required={required}>
      <legend className="yv-rating-question">
        {label} {required && <span className="required" aria-hidden="true">*</span>}
      </legend>
      <div className="yv-rating-meta">
        {supportingText && <span className="yv-rating-domain">{supportingText}</span>}
        {value !== null && (
          <span className="yv-rating-selection" aria-live="polite">
            Chosen: {value} · {LABELS[value]}
          </span>
        )}
      </div>
      <div className="emoji-row">
        {EMOJIS.map((emoji, idx) => {
          const score = idx;
          const active = value === score;
          return (
            <button
              key={score}
              type="button"
              className={`emoji-btn ${active ? "emoji-btn-active" : ""}`}
              onClick={() => onChange(score)}
              aria-label={`${score}, ${LABELS[score]}`}
              aria-pressed={active}
            >
              <span className="emoji" aria-hidden="true">{emoji}</span>
              <span className="emoji-score">{score}</span>
              <span className="emoji-meaning">{LABELS[score]}</span>
            </button>
          );
        })}
      </div>
    </fieldset>
  );
};

export default EmojiRating;
