// frontend/src/config/homeConfig.ts

export type HomeConfig = {
  slug: string;
  name: string;
  provider: string;
};

const HOMES: HomeConfig[] = [
  {
    slug: "millbrow-house",
    name: "Millbrow House",
    provider: "Care4EveryChild",
  },
  {
    slug: "test-home",
    name: "Test Home",
    provider: "Pathway Care Group",
  },
  // add more as you roll out
];

export function getHomeConfig(slug?: string): HomeConfig | undefined {
  if (!slug) return undefined;
  return HOMES.find((h) => h.slug === slug);
}