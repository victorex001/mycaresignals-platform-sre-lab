import React from "react";

type Props = {
  homeName: string;
  providerName: string;
};

const evidenceTypes = [
  ["Daily Progress", "Everyday staff observations."],
  ["Key Working", "Planned direct or therapeutic work."],
  ["Young Person Monthly Voice", "The young person’s own monthly perspective."],
  ["Risk & Safety", "Current risks, controls and professional Risk Reviews."],
  ["Care Plan Reviews", "Professional consideration of the wider evidence and whether the plan remains right for the young person."],
];

const workflowSteps = [
  {
    title: "When a young person arrives",
    items: [
      "Create their record.",
      "Complete the initial understanding/baseline.",
      "Begin their Care Plan.",
      "Identify current Risk & Safety arrangements.",
      "Agree Goals and SMART Targets.",
    ],
  },
  {
    title: "During everyday care",
    items: [
      "Record brief Daily Progress observations where relevant.",
      "Complete planned Key Working.",
      "Capture genuine young-person comments when they are expressed.",
    ],
  },
  {
    title: "Each month",
    items: [
      "The young person can complete their Monthly Voice.",
      "Staff and managers can see their perspective alongside the pattern of staff observations.",
    ],
  },
  {
    title: "When something needs attention",
    items: [
      "myCareSignals can surface review prompts, due reviews or differences in perspectives.",
      "Practitioners decide what action, if any, is required.",
    ],
  },
  {
    title: "At review",
    items: [
      "The team can consider Care Plan, Risk, Daily Progress, Key Working, Young Person Voice and other evidence together.",
    ],
  },
];

const AboutTab: React.FC<Props> = ({ homeName, providerName }) => {
  return (
    <article className="card about-page">
      <header className="about-hero">
        <div className="about-hero-copy">
          <p className="current-care-eyebrow">About myCareSignals</p>
          <h2>A clear story of care and progress</h2>
          <p className="about-lead">
            A practical therapeutic care platform for residential
            children&apos;s homes that connects care planning, daily
            observations, key-working, young people&apos;s voice, risk and
            professional review into a clear story of care and progress over
            time.
          </p>
        </div>
        <dl className="about-context" aria-label="Current home and provider">
          <div>
            <dt>Home</dt>
            <dd>{homeName}</dd>
          </div>
          <div>
            <dt>Provider</dt>
            <dd>{providerName}</dd>
          </div>
        </dl>
      </header>

      <div className="about-content">
        <section className="about-section">
          <h3>Who myCareSignals is for</h3>
          <p>
            myCareSignals is designed for residential children&apos;s care
            teams working with young people who have individual care plans,
            changing needs and multi-agency involvement.
          </p>
          <p>It is particularly useful for:</p>
          <ul className="about-list">
            <li>Children&apos;s residential homes, including EBD, complex-needs and step-down placements.</li>
            <li>Short- and long-term placements where care needs to be reviewed over time.</li>
            <li>Key workers, senior staff and managers who need a clearer connection between the Care Plan and everyday practice.</li>
            <li>Responsible Individuals and leaders who need oversight of whether plans, reviews and evidence remain current.</li>
            <li>Providers who want a clearer story of care for young people, families, social workers, IROs, commissioners and inspectors without creating unnecessary recording for staff.</li>
          </ul>
        </section>

        <section className="about-section">
          <h3>What myCareSignals aims to do</h3>
          <div className="about-feature-list">
            <div className="about-feature">
              <h4>Keep the Care Plan connected to everyday care</h4>
              <p>Turn the Care Plan into clear domains, goals and SMART targets so staff understand what the team is working towards and why.</p>
            </div>
            <div className="about-feature">
              <h4>Make daily recording useful</h4>
              <p>Staff can record short Daily Progress observations against a care domain and, where relevant, an active target.</p>
              <p>Daily recording is deliberately lightweight: what was noticed, the young person&apos;s comment if they expressed one, and optional 0–4 wellbeing or progress ratings.</p>
            </div>
            <div className="about-feature">
              <h4>Make Key Working purposeful</h4>
              <p>Key Working remains a separate form of evidence for planned direct or therapeutic work.</p>
              <p>Sessions can connect to the young person&apos;s goals and targets, capture their voice and the worker&apos;s reflection, and retain a clear history for later review.</p>
            </div>
            <div className="about-feature">
              <h4>Bring the young person&apos;s own perspective into the picture</h4>
              <p>Young people can provide structured Monthly Voice feedback through a secure QR/link. Their monthly ratings and their own words remain separate from staff observations.</p>
              <p>This allows teams to see both:</p>
              <ul className="about-list">
                <li>what staff are observing</li>
                <li>how the young person says things feel for them</li>
              </ul>
            </div>
            <div className="about-feature">
              <h4>Surface differences that deserve professional curiosity</h4>
              <p>myCareSignals can identify simple, transparent patterns such as a persistent difference between staff Daily Progress observations and the young person&apos;s Monthly Voice.</p>
              <blockquote className="about-review-prompt">Persistent difference in perspectives — review suggested.</blockquote>
              <p>It does not decide what that difference means. Staff and professionals review the evidence and make the care decision.</p>
            </div>
            <div className="about-feature">
              <h4>Connect Risk &amp; Safety to the wider care journey</h4>
              <p>Risks, safety strategies and Risk Reviews remain clearly recorded while staying connected to the wider Care Plan.</p>
              <p>myCareSignals does not automatically create risks, alter risk ratings or make safeguarding decisions.</p>
            </div>
            <div className="about-feature">
              <h4>Support proper review rather than static paperwork</h4>
              <p>Care Plans and Risks can be reviewed over time, with review history preserved so teams can see what changed, why it changed and what evidence informed the decision.</p>
            </div>
            <div className="about-feature">
              <h4>Show the child&apos;s journey clearly</h4>
              <p>The Unified Child Journey brings together current priorities, Goals &amp; Targets, Care Plan information, Risk &amp; Safety, recent Daily Progress, Key Working, Young Person Voice and important review activity.</p>
              <p>The aim is not to create a single score for the child. It is to help professionals understand the evidence around them.</p>
            </div>
          </div>
        </section>

        <section className="about-section">
          <h3>Young Person Voice</h3>
          <p>Young Person Voice is intentionally different from staff Daily Progress. Staff Daily Progress is recorded frequently as part of everyday care. Young Person Monthly Voice is a monthly reflection from the young person themselves.</p>
          <p>Their ratings are not generated from their written comments and are not produced by AI. If a rating is shown, it is because the young person selected it themselves. Their written words remain available as direct voice evidence alongside the structured ratings.</p>
          <p>Where staff observations and the young person&apos;s own perspective differ over time, myCareSignals can surface that difference for human review rather than trying to determine who is “right”.</p>
        </section>

        <section className="about-section">
          <h3>Visitor and professional feedback</h3>
          <p>Home-level visitor, family, professional and inspector feedback is handled by the separate QR Feedback service.</p>
          <p>The QR service:</p>
          <ul className="about-list">
            <li>collects the feedback</li>
            <li>notifies management</li>
            <li>supports review, export and reporting</li>
          </ul>
          <p>It does not enter the individual child&apos;s therapeutic record or automatically create or change a Risk, Care Plan, Goal, Target, Daily Progress, Monthly Voice or perspective comparison.</p>
          <p>If a manager decides that child-care action is required, they use the appropriate Therapeutic Engine workflow separately.</p>
          <p>Young Person Monthly Voice remains securely linked to the child and is the QR-derived evidence that enters Therapeutic Engine.</p>
        </section>

        <section className="about-section">
          <h3>How the evidence fits together</h3>
          <p>myCareSignals keeps different forms of evidence distinct:</p>
          <dl className="about-evidence-list">
            {evidenceTypes.map(([title, description]) => (
              <div key={title}>
                <dt>{title}</dt>
                <dd>{description}</dd>
              </div>
            ))}
          </dl>
          <p>These records can inform one another without being collapsed into a single score.</p>
        </section>

        <section className="about-section">
          <h3>How this fits with other systems</h3>
          <p>myCareSignals can sit alongside existing systems such as ClearCare and local-authority systems, but it now supports much more than a simple outcomes tracker.</p>
          <p>It includes structured workflows for:</p>
          <ul className="about-list about-list-columns">
            <li>Care Plans</li>
            <li>Goals and SMART Targets</li>
            <li>Risk &amp; Safety</li>
            <li>Risk Reviews</li>
            <li>Daily Progress</li>
            <li>Key Working</li>
            <li>Key Working manager review</li>
            <li>Young Person Monthly Voice</li>
            <li>professional review history</li>
            <li>the Unified Child Journey</li>
          </ul>
          <p>Existing statutory, organisational or local-authority systems may still remain necessary for records and functions outside the scope of myCareSignals.</p>
          <p>The aim is not to duplicate everything already recorded elsewhere. It is to make the relationship between the plan, daily care, direct work, the young person&apos;s experience and professional review much clearer.</p>
        </section>

        <section className="about-section">
          <h3>How staff use it</h3>
          <div className="about-workflow">
            {workflowSteps.map((step) => (
              <div key={step.title}>
                <h4>{step.title}</h4>
                <ul className="about-list">
                  {step.items.map((item) => <li key={item}>{item}</li>)}
                </ul>
              </div>
            ))}
          </div>
        </section>

        <section className="about-section about-safety">
          <h3>What this tool is not</h3>
          <ul className="about-list">
            <li>It is not a diagnostic tool.</li>
            <li>It does not replace psychological, therapeutic or specialist professional judgement.</li>
            <li>It does not make safeguarding decisions.</li>
            <li>It does not automatically decide that a young person&apos;s risk has increased or decreased.</li>
            <li>It does not automatically change supervision, placement arrangements, Care Plans or Targets.</li>
            <li>It does not score staff performance.</li>
            <li>It does not reduce a young person to a single progress score.</li>
          </ul>
        </section>

        <section className="about-principle">
          <p className="current-care-eyebrow">The principle behind myCareSignals</p>
          <h3>Staff care, listen, observe and decide.</h3>
          <p>myCareSignals helps them ask useful questions, structure the record, connect evidence and review what is happening over time.</p>
          <p className="about-closing">“Staff record the small things. The platform helps tell the story.”</p>
        </section>
      </div>
    </article>
  );
};

export default AboutTab;
