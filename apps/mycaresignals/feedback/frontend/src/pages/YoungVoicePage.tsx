// frontend/src/pages/YoungVoicePage.tsx
import { useSearchParams } from "react-router-dom";
import YoungVoiceForm from "../components/YoungVoiceForm";

export default function YoungVoicePage() {
  const [searchParams] = useSearchParams();

  const contextToken = searchParams.get("context")?.trim() ?? "";

  if (!contextToken) {
    return <div>Invalid or expired feedback link. Please scan the QR code again.</div>;
  }

  // Let YoungVoiceForm handle the page layout
  return <YoungVoiceForm contextToken={contextToken} />;
}
