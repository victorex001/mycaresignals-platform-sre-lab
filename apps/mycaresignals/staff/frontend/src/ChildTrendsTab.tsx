// src/ChildTrendsTab.tsx
import React from "react";
import DailyScoresTab from "./DailyScoresTab";

type Props = {
  apiBase: string;
  childId: number;
  childInitials: string;
};

const ChildTrendsTab: React.FC<Props> = (props) => {
  return (
    <DailyScoresTab
      {...props}
      defaultInsightsTab="trends"
    />
  );
};

export default ChildTrendsTab;
