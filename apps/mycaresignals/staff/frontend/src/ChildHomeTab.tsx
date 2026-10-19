import { useEffect, useState } from "react";
import { apiFetch } from "./api";
import type { Child, ChildCareOverview } from "./types";

type Props = {
  apiBase: string;
  homeName: string;
  child: Child;
  onBackToChildren: () => void;
  onOpenDailyProgress: () => void;
  onOpenProgress: () => void;
  onOpenFramework: () => void;
  onOpenKeywork: () => void;
  onOpenCarePlan: () => void;
  onOpenGoals: () => void;
  onOpenRisks: () => void;
  onOpenMissingFromHome: () => void;
  onOpenVoice: () => void;
  onOpenReviews: () => void;
  onOpenJourney: () => void;
};

type LauncherTile = {
  title: string;
  description: string;
  symbol: string;
  tone: string;
  onOpen: () => void;
};

const formatDate = (value?: string | null) => {
  if (!value) return null;
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return null;
  return date.toLocaleDateString(undefined, {
    day: "numeric",
    month: "short",
    year: "numeric",
  });
};

export default function ChildHomeTab({
  apiBase,
  homeName,
  child,
  onBackToChildren,
  onOpenDailyProgress,
  onOpenProgress,
  onOpenFramework,
  onOpenKeywork,
  onOpenCarePlan,
  onOpenGoals,
  onOpenRisks,
  onOpenMissingFromHome,
  onOpenVoice,
  onOpenReviews,
  onOpenJourney,
}: Props) {
  const [attentionCount, setAttentionCount] = useState<number | null>(null);
  const [canReview, setCanReview] = useState(false);

  useEffect(() => {
    let cancelled = false;

    void Promise.all([
      apiFetch(`${apiBase}/v1/children/${child.id}/care-overview`),
      apiFetch(`${apiBase}/v1/children/${child.id}/monthly-voice/review-access`),
    ])
      .then(async ([overviewResponse, reviewResponse]) => {
        if (!cancelled) setCanReview(reviewResponse.ok);
        if (!overviewResponse.ok) return;
        const overview = (await overviewResponse.json()) as ChildCareOverview;
        if (!cancelled) {
          setAttentionCount(
            Array.isArray(overview.needs_attention)
              ? overview.needs_attention.length
              : 0,
          );
        }
      })
      .catch(() => {
        if (!cancelled) {
          setAttentionCount(null);
          setCanReview(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [apiBase, child.id]);

  const displayName = child.first_name || child.initials || "Young person";
  const placementDate = formatDate(child.admission_date);

  const tiles: LauncherTile[] = [
    {
      title: "Initial Framework",
      description: "Understand strengths, needs and current priorities.",
      symbol: "◫",
      tone: "plum",
      onOpen: onOpenFramework,
    },
    {
      title: "Daily Progress",
      description: "Record what you noticed today.",
      symbol: "☀",
      tone: "coral",
      onOpen: onOpenDailyProgress,
    },
    {
      title: "Progress Over Time",
      description: "See patterns across staff observations, therapeutic work and the young person’s perspective.",
      symbol: "↗",
      tone: "mint",
      onOpen: onOpenProgress,
    },
    {
      title: "Key Working",
      description: "Record planned direct work.",
      symbol: "✎",
      tone: "amber",
      onOpen: onOpenKeywork,
    },
    {
      title: "Care Plan",
      description: "View and review the current Care Plan.",
      symbol: "▤",
      tone: "teal",
      onOpen: onOpenCarePlan,
    },
    {
      title: "Goals & Targets",
      description: "See what the team is working towards.",
      symbol: "◎",
      tone: "green",
      onOpen: onOpenGoals,
    },
    {
      title: "Risk & Safety",
      description: "View current risks and safety plans.",
      symbol: "◇",
      tone: "blue",
      onOpen: onOpenRisks,
    },
    {
      title: "Missing From Home",
      description: "Create and review the child-specific Missing From Home plan.",
      symbol: "⌂",
      tone: "rose",
      onOpen: onOpenMissingFromHome,
    },
    {
      title: "Young Person Voice",
      description: "View Monthly Voice and their perspective.",
      symbol: "◌",
      tone: "violet",
      onOpen: onOpenVoice,
    },
    ...(canReview
      ? [
          {
            title: "Reviews / Attention",
            description: "See what needs professional review.",
            symbol: "!",
            tone: "rose",
            onOpen: onOpenReviews,
          },
        ]
      : []),
    {
      title: "Child Journey",
      description: "View the full connected evidence history.",
      symbol: "→",
      tone: "navy",
      onOpen: onOpenJourney,
    },
  ];

  return (
    <section className="card child-home-page">
      <header className="child-home-header">
        <button
          type="button"
          className="text-button"
          onClick={onBackToChildren}
        >
          ← Back to children
        </button>
        <div>
          <p className="current-care-eyebrow">Child Home</p>
          <h2>{displayName}</h2>
          <p className="child-home-placement">
            {homeName}
            {placementDate && <> · Placement started {placementDate}</>}
          </p>
        </div>
      </header>

      <div className="child-home-prompt">
        <p className="current-care-eyebrow">Choose an area</p>
        <h3>What do you want to do?</h3>
      </div>

      <nav className="child-home-grid" aria-label={`Choose an area for ${displayName}`}>
        {tiles.map((tile) => (
          <button
            key={tile.title}
            type="button"
            className={`child-home-tile child-home-tile-${tile.tone}`}
            onClick={tile.onOpen}
          >
            <span className="child-home-tile-symbol" aria-hidden="true">
              {tile.symbol}
            </span>
            <span className="child-home-tile-copy">
              <strong>{tile.title}</strong>
              <span>{tile.description}</span>
            </span>
            {tile.title === "Reviews / Attention" && attentionCount !== null && (
              <span className="child-home-attention-count" aria-label={`${attentionCount} items need attention`}>
                {attentionCount}
              </span>
            )}
            <span className="child-home-tile-arrow" aria-hidden="true">→</span>
          </button>
        ))}
      </nav>
    </section>
  );
}
