import { BrowserRouter, Routes, Route } from "react-router-dom";
import VisitorFeedbackPage from "./pages/VisitorFeedbackPage";
import YoungVoicePage from "./pages/YoungVoicePage";
import ManagerFeedbackPage from "./pages/ManagerFeedbackPage";

const CHILDVOICE_HOST = "childvoice.mycaresignals.co.uk";
const VISITORFEEDBACK_HOST = "visitorfeedback.mycaresignals.co.uk";
const LOCAL_CHILDVOICE_HOST = "childvoice.localhost";
const LOCAL_VISITORFEEDBACK_HOST = "visitorfeedback.localhost";

type HostMode = "childvoice" | "visitorfeedback" | "unknown";

function hostMode(): HostMode {
  const hostname = window.location.hostname.toLowerCase();
  if (hostname === CHILDVOICE_HOST) return "childvoice";
  if (hostname === VISITORFEEDBACK_HOST) return "visitorfeedback";
  if (import.meta.env.DEV && hostname === LOCAL_CHILDVOICE_HOST) return "childvoice";
  if (import.meta.env.DEV && hostname === LOCAL_VISITORFEEDBACK_HOST) return "visitorfeedback";
  return "unknown";
}

function RootLanding() {
  const mode = hostMode();

  return (
    <div className="page page-center">
      <div className="vf-card">
        {mode === "visitorfeedback" && <p>Scan the home QR code to give visitor feedback.</p>}
        {mode === "childvoice" && <p>Scan the Young Person Voice QR code to share your monthly voice.</p>}
        {mode === "unknown" && <p>Invalid QR hostname. Please scan the QR code again.</p>}
      </div>
    </div>
  );
}

function NotFound() {
  return (
    <div className="page page-center">
      <div className="vf-card">
        <p>Invalid link. Please scan a QR code again.</p>
      </div>
    </div>
  );
}

export function AppRouter() {
  const mode = hostMode();

  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<RootLanding />} />

        {mode === "visitorfeedback" && (
          <Route path="/visit/:homeSlug" element={<VisitorFeedbackPage />} />
        )}
        {mode === "childvoice" && (
          <>
            <Route path="/young-voice" element={<YoungVoicePage />} />
            <Route path="/young-voice/:homeSlug" element={<YoungVoicePage />} />
          </>
        )}
        {(mode === "childvoice" || mode === "visitorfeedback") && (
          <Route path="/admin/feedback" element={<ManagerFeedbackPage />} />
        )}

        <Route path="*" element={<NotFound />} />
      </Routes>
    </BrowserRouter>
  );
}
