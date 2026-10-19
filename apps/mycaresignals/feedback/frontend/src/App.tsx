// // src/App.tsx
// import React, { useState } from "react";
// import VisitorFeedbackForm from "./components/VisitorFeedbackForm";
// import YoungPersonMonthlyForm from "./components/YoungVoiceForm";
// import DomainCheckInForm from "./pages/CheckInPage";

// type View = "visitor" | "young" | "checkin";

// const App: React.FC = () => {
//   const [view, setView] = useState<View>("visitor");

//   return (
//     <div className="app-shell">
//       <header className="app-header">
//         <h1 className="brand">myCareSignals – Feedback</h1>
//         <p className="brand-subtitle">
//           QR feedback and daily check-ins where children&apos;s voices shape care.
//         </p>

//         <nav className="nav-tabs">
//           <button
//             className={`nav-tab ${view === "visitor" ? "nav-tab-active" : ""}`}
//             onClick={() => setView("visitor")}
//             type="button"
//           >
//             Visitor feedback
//           </button>

//           <button
//             className={`nav-tab ${view === "young" ? "nav-tab-active" : ""}`}
//             onClick={() => setView("young")}
//             type="button"
//           >
//             Young person monthly
//           </button>

//           <button
//             className={`nav-tab ${view === "checkin" ? "nav-tab-active" : ""}`}
//             onClick={() => setView("checkin")}
//             type="button"
//           >
//             Daily domain check-in
//           </button>
//         </nav>
//       </header>

//       <main className="app-main">
//         {view === "visitor" && <VisitorFeedbackForm />}
//         {view === "young" && <YoungPersonMonthlyForm />}
//         {view === "checkin" && <DomainCheckInForm />}
//       </main>
//     </div>
//   );
// };

// export default App;


import React from "react";
import { AppRouter } from "./router";

const App: React.FC = () => {
  return (
    <div className="app-shell">
      <header className="app-header">
        <h1 className="brand">myCareSignals – Feedback</h1>
        <p className="brand-subtitle">
          Secure feedback where young people&apos;s voices shape care.
        </p>
      </header>

      <main className="app-main">
        <AppRouter />
      </main>
    </div>
  );
};

export default App;
