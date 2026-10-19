ALTER TABLE child_goals
  ADD CONSTRAINT uq_child_goals_id_child_id UNIQUE (id, child_id);

ALTER TABLE child_targets
  ADD CONSTRAINT uq_child_targets_id_child_id UNIQUE (id, child_id);

CREATE TABLE child_risks (
  id BIGSERIAL PRIMARY KEY,
  child_id BIGINT NOT NULL REFERENCES children(id) ON DELETE CASCADE,
  title TEXT NOT NULL,
  category TEXT NOT NULL,
  domain TEXT,
  status TEXT NOT NULL DEFAULT 'active'
    CONSTRAINT chk_child_risks_status CHECK (status IN ('active', 'paused', 'closed')),
  nature_of_risk TEXT NOT NULL,
  who_is_at_risk TEXT,
  information_status TEXT NOT NULL
    CONSTRAINT chk_child_risks_information_status CHECK (information_status IN (
      'current_confirmed',
      'reported',
      'historical',
      'allegation_not_established',
      'awaiting_confirmation'
    )),
  source_note TEXT,
  initial_likelihood SMALLINT
    CONSTRAINT chk_child_risks_initial_likelihood CHECK (initial_likelihood BETWEEN 1 AND 5),
  initial_severity SMALLINT
    CONSTRAINT chk_child_risks_initial_severity CHECK (initial_severity BETWEEN 1 AND 5),
  residual_likelihood SMALLINT
    CONSTRAINT chk_child_risks_residual_likelihood CHECK (residual_likelihood BETWEEN 1 AND 5),
  residual_severity SMALLINT
    CONSTRAINT chk_child_risks_residual_severity CHECK (residual_severity BETWEEN 1 AND 5),
  CONSTRAINT chk_child_risks_initial_rating_pair
    CHECK ((initial_likelihood IS NULL) = (initial_severity IS NULL)),
  CONSTRAINT chk_child_risks_residual_rating_pair
    CHECK ((residual_likelihood IS NULL) = (residual_severity IS NULL)),
  management_approach TEXT NOT NULL
    CONSTRAINT chk_child_risks_management_approach CHECK (management_approach IN (
      'controls_only',
      'controls_and_developmental_work'
    )),
  proactive_strategies TEXT,
  reactive_strategies TEXT,
  effective_from DATE,
  last_reviewed_at TIMESTAMPTZ,
  next_review_date DATE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT uq_child_risks_id_child_id UNIQUE (id, child_id)
);

CREATE TABLE child_risk_goals (
  risk_id BIGINT NOT NULL,
  child_id BIGINT NOT NULL,
  goal_id BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (risk_id, goal_id),
  CONSTRAINT fk_child_risk_goals_risk_child
    FOREIGN KEY (risk_id, child_id) REFERENCES child_risks(id, child_id) ON DELETE CASCADE,
  CONSTRAINT fk_child_risk_goals_goal_child
    FOREIGN KEY (goal_id, child_id) REFERENCES child_goals(id, child_id) ON DELETE CASCADE
);

CREATE TABLE child_risk_targets (
  risk_id BIGINT NOT NULL,
  child_id BIGINT NOT NULL,
  target_id BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (risk_id, target_id),
  CONSTRAINT fk_child_risk_targets_risk_child
    FOREIGN KEY (risk_id, child_id) REFERENCES child_risks(id, child_id) ON DELETE CASCADE,
  CONSTRAINT fk_child_risk_targets_target_child
    FOREIGN KEY (target_id, child_id) REFERENCES child_targets(id, child_id) ON DELETE CASCADE
);

CREATE INDEX idx_child_risks_child_status
  ON child_risks (child_id, status, updated_at DESC);

CREATE INDEX idx_child_risks_next_review
  ON child_risks (child_id, next_review_date)
  WHERE status = 'active' AND next_review_date IS NOT NULL;

CREATE INDEX idx_child_risk_goals_goal
  ON child_risk_goals (goal_id);

CREATE INDEX idx_child_risk_targets_target
  ON child_risk_targets (target_id);
