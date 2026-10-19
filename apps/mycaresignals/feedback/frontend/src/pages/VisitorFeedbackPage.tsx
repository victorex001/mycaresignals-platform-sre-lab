// frontend/src/pages/VisitorFeedbackPage.tsx
import { useParams } from "react-router-dom";
import { useSearchParams } from "react-router-dom";
import { getHomeConfig } from "../config/homeConfig";
import VisitorFeedbackForm from "../components/VisitorFeedbackForm";

export default function VisitorFeedbackPage() {
  const { homeSlug } = useParams<{ homeSlug: string }>();
  const [searchParams] = useSearchParams();
  const home = getHomeConfig(homeSlug);

  const contextToken = searchParams.get("context")?.trim() ?? "";

  if (!home || !contextToken) {
    return (
      <div className="page page-center">
        <div className="vf-card">
          <p>Invalid or expired feedback link. Please scan the QR code again.</p>
        </div>
      </div>
    );
  }

  return (
    <div className="page page-center">
      {/* 👇 ONLY the form – no left-hand title text */}
      <VisitorFeedbackForm home={home} contextToken={contextToken} />
    </div>
  );
}
