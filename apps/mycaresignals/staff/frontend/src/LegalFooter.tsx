import React from "react";

async function downloadFile(path: string, filename: string) {
  const url = new URL(path, window.location.origin).toString();
  const res = await fetch(url);
  if (!res.ok) throw new Error(`Download failed: ${res.status}`);
  const blob = await res.blob();

  const a = document.createElement("a");
  a.href = URL.createObjectURL(blob);
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(a.href);
}

const LegalFooter: React.FC = () => {
  const year = new Date().getFullYear();

  return (
    <footer className="app-footer">
      <div className="app-footer-links">
        <button
          type="button"
          className="link-like"
          onClick={() =>
            downloadFile(
              "/legal/myCareSignals_Privacy_Policy_v1.pdf",
              "myCareSignals_Privacy_Policy_v1.pdf"
            )
          }
        >
          Privacy Policy
        </button>

        <button
          type="button"
          className="link-like"
          onClick={() =>
            downloadFile(
              "/legal/myCareSignals_Cookies_Policy_v1.pdf",
              "myCareSignals_Cookies_Policy_v1.pdf"
            )
          }
        >
          Cookies Policy
        </button>

        <button
          type="button"
          className="link-like"
          onClick={() =>
            downloadFile(
              "/legal/myCareSignals_Terms_Of_Use_v1.pdf",
              "myCareSignals_Terms_Of_Use_v1.pdf"
            )
          }
        >
          Terms &amp; Conditions
        </button>
      </div>

      <div className="app-footer-meta">© {year} myCareSignals. All rights reserved.</div>
    </footer>
  );
};

export default LegalFooter;