// frontend/src/pages/CheckInPage.tsx
import { useParams } from "react-router-dom";
import { getHomeConfig } from "../config/homeConfig";

export default function CheckInPage() {
  const { homeSlug, childSlug } = useParams<{
    homeSlug: string;
    childSlug: string;
  }>();

  const home = getHomeConfig(homeSlug);

  if (!home) {
    return (
      <div style={{ color: "white", padding: "2rem" }}>
        Unknown home: <code>{homeSlug}</code>
      </div>
    );
  }
  if (!childSlug) {
    return (
      <div style={{ color: "white", padding: "2rem" }}>
        Missing child.
      </div>
    );
  }

  return (
    <div className="vf-page">
      <section className="vf-card">
        <h1 className="vf-title">Weekly check-in (coming soon)</h1>
        <p className="vf-subtitle">
          Home: <strong>{home.name}</strong> | Child:{" "}
          <strong>{childSlug}</strong>
        </p>
        <p style={{ marginTop: "1rem" }}>
          This page will show simple sliders linked to your domains and send
          those scores into the Therapeutic Engine.
        </p>
      </section>
    </div>
  );
}