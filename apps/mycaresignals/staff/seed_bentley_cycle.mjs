// seed_bentley_cycle.mjs
const API_BASE = process.env.API_BASE || "http://localhost:8081";
const HOME_ID = Number(process.env.HOME_ID || 1);

async function post(path, body) {
  const res = await fetch(`${API_BASE}${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    const txt = await res.text();
    throw new Error(`POST ${path} failed: ${res.status} ${txt}`);
  }
  return res.json();
}

function isoAtHour(daysAgo, hour = 18) {
  const d = new Date();
  d.setHours(0, 0, 0, 0);
  d.setDate(d.getDate() - daysAgo);
  d.setHours(hour, 0, 0, 0);
  return d.toISOString();
}

// map 1..30 → a gentle trend 0–4
function trend4(dayIndex, start = 1, step = 7) {
  const raw = start + Math.floor(dayIndex / step); // climbs every few days
  return Math.max(0, Math.min(4, raw)); // clamp 0–4
}

async function main() {
  console.log(`Using API_BASE=${API_BASE}`);
  console.log(`Using HOME_ID=${HOME_ID}\n`);

  // 1) Onboard Bentley
  console.log("==> Onboarding Bentley...");
  const child = await post(`/v1/homes/${HOME_ID}/children/onboard`, {
    external_ref: "MBH-BENTLEY-TEST",
    initials: "BK",
    first_name: "Bentley",
    last_name: "Kairanga",
    date_of_birth: "2011-05-10",
    admission_date: "2024-03-01",
    legal_status: "CLA – ICO",
    gender: "Male",
  });
  const CHILD_ID = child.id;
  console.log("Child:", child);
  console.log(`CHILD_ID = ${CHILD_ID}\n`);

  // 2) Goals
  console.log("==> Creating goals...");
  const goal1 = await post(`/v1/children/${CHILD_ID}/goals`, {
    goal_type: "care_plan",
    title: "Improve school attendance and engagement",
    description:
      "Bentley attends school more consistently and is able to settle into lessons with support.",
    start_date: "2025-01-01",
    target_date: null,
    status: "active",
  });
  const goal2 = await post(`/v1/children/${CHILD_ID}/goals`, {
    goal_type: "care_plan",
    title: "Reduce physical incidents and keep everyone safe",
    description:
      "Bentley uses safer responses when angry, with fewer physical incidents.",
    start_date: "2025-01-01",
    target_date: null,
    status: "active",
  });
  const goal3 = await post(`/v1/children/${CHILD_ID}/goals`, {
    goal_type: "care_plan",
    title: "Build a calmer morning routine",
    description:
      "Bentley can get up, complete basic hygiene and be ready for school with minimal prompts.",
    start_date: "2025-01-01",
    target_date: null,
    status: "active",
  });

  console.log("Goals:", goal1.id, goal2.id, goal3.id, "\n");

  // 3) Targets
  console.log("==> Creating targets...");
  const t1 = await post(`/v1/children/${CHILD_ID}/targets`, {
    goal_id: goal1.id,
    domain: "education",
    short_label: "Attends school at least 4 days each week",
    description:
      "4+ days present in school per week recorded on the home’s daily logs.",
  });

  const t2 = await post(`/v1/children/${CHILD_ID}/targets`, {
    goal_id: goal2.id,
    domain: "emotional_regulation",
    short_label: "Uses agreed coping strategy before escalation",
    description:
      "Uses a coping plan (time out, talking, sensory box) before shouting, throwing or leaving.",
  });

  const t3 = await post(`/v1/children/${CHILD_ID}/targets`, {
    goal_id: goal3.id,
    domain: "self_care",
    short_label: "Completes morning routine with no more than one prompt",
    description:
      "Up, washed, teeth brushed and dressed for school with 0–1 prompts.",
  });

  console.log("Targets:", t1.id, t2.id, t3.id, "\n");

  // 4) 30 days of sessions, scores 0–4 with gentle improvement
  console.log("==> Creating 30 daily sessions...\n");

  for (let daysAgo = 29; daysAgo >= 0; daysAgo--) {
    const dayIndex = 30 - daysAgo; // 1..30
    const session_at = isoAtHour(daysAgo, 18);

    const schoolProgress = trend4(dayIndex, 1, 5);
    const schoolWellbeing = trend4(dayIndex, 1, 6);

    const copingProgress = trend4(dayIndex, 0, 6);
    const copingWellbeing = trend4(dayIndex, 1, 7);

    const morningProgress = trend4(dayIndex, 0, 5);
    const morningWellbeing = trend4(dayIndex, 1, 7);

    console.log(
      `  - Session ${dayIndex}/30 on ${session_at} (school P=${schoolProgress}, coping P=${copingProgress}, morning P=${morningProgress})`
    );

    await post(`/v1/children/${CHILD_ID}/sessions`, {
      session_at,
      staff_name: "Victory (test)",
      notes: `Seeded test session day ${dayIndex} of 30.`,
      scores: [
        {
          target_id: t1.id,
          wellbeing_score: schoolWellbeing,
          progress_score: schoolProgress,
          comment: "School attendance focus – test data.",
        },
        {
          target_id: t2.id,
          wellbeing_score: copingWellbeing,
          progress_score: copingProgress,
          comment: "Coping strategies focus – test data.",
        },
        {
          target_id: t3.id,
          wellbeing_score: morningWellbeing,
          progress_score: morningProgress,
          comment: "Morning routine focus – test data.",
        },
      ],
    });
  }

  console.log("\n==> DONE");
  console.log(`Child ID: ${CHILD_ID}`);
  console.log(`Targets: ${t1.id}, ${t2.id}, ${t3.id}`);
  console.log("Open UI → Key-working & timeline to see 30 days of scores.");
}

main().catch((err) => {
  console.error("ERROR:", err);
  process.exit(1);
});
