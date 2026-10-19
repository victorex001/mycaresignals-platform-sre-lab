import React from "react";
import ReactDOM from "react-dom/client";
import { AppRouter } from "./router";

import "./styles/global.css";
import "./styles/visitors.css";
import "./styles/youngVoice.css";

const el = document.getElementById("root");
if (!el) throw new Error("Missing #root in index.html");

class ErrorBoundary extends React.Component<
  { children: React.ReactNode },
  { err?: any }
> {
  state = { err: undefined as any };
  static getDerivedStateFromError(err: any) {
    return { err };
  }
  render() {
    if (this.state.err) {
      return (
        <pre style={{ padding: 16, whiteSpace: "pre-wrap", color: "crimson" }}>
          {String(this.state.err?.stack || this.state.err)}
        </pre>
      );
    }
    return this.props.children;
  }
}

ReactDOM.createRoot(el).render(
  <React.StrictMode>
    <ErrorBoundary>
      <AppRouter />
    </ErrorBoundary>
  </React.StrictMode>
);
