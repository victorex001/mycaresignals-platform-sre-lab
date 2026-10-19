-- Guided Missing From Home planning. This stores a versioned operational plan
-- only; it does not create or mutate risks, care plans, goals or targets.

CREATE TABLE child_missing_from_home_plans (
  id BIGSERIAL PRIMARY KEY,
  child_id BIGINT NOT NULL REFERENCES children(id) ON DELETE CASCADE,
  version INT NOT NULL CHECK (version > 0),
  status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN (
    'draft', 'submitted', 'approved', 'superseded', 'archived'
  )),
  predecessor_plan_id BIGINT REFERENCES child_missing_from_home_plans(id) ON DELETE RESTRICT,
  content JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(content) = 'object'),
  source_snapshots JSONB NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(source_snapshots) = 'array'),
  review_date DATE,
  created_by_staff_id BIGINT NOT NULL REFERENCES staff_users(id) ON DELETE RESTRICT,
  submitted_by_staff_id BIGINT REFERENCES staff_users(id) ON DELETE RESTRICT,
  approved_by_staff_id BIGINT REFERENCES staff_users(id) ON DELETE RESTRICT,
  archived_by_staff_id BIGINT REFERENCES staff_users(id) ON DELETE RESTRICT,
  submitted_at TIMESTAMPTZ,
  approved_at TIMESTAMPTZ,
  archived_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (child_id, version),
  CHECK (predecessor_plan_id IS NULL OR predecessor_plan_id <> id),
  CHECK ((status <> 'submitted') OR (submitted_by_staff_id IS NOT NULL AND submitted_at IS NOT NULL)),
  CHECK ((status NOT IN ('approved', 'superseded')) OR (approved_by_staff_id IS NOT NULL AND approved_at IS NOT NULL)),
  CHECK ((status <> 'archived') OR (archived_by_staff_id IS NOT NULL AND archived_at IS NOT NULL))
);

CREATE UNIQUE INDEX uq_child_mfh_open_plan
  ON child_missing_from_home_plans (child_id)
  WHERE status IN ('draft', 'submitted');

CREATE UNIQUE INDEX uq_child_mfh_approved_plan
  ON child_missing_from_home_plans (child_id)
  WHERE status = 'approved';

CREATE INDEX idx_child_mfh_history
  ON child_missing_from_home_plans (child_id, version DESC, id DESC);

CREATE FUNCTION prevent_approved_mfh_plan_rewrite() RETURNS trigger AS $$
BEGIN
  IF TG_OP = 'DELETE' AND OLD.status IN ('approved', 'superseded', 'archived') THEN
    RAISE EXCEPTION 'approved Missing From Home plan versions are immutable';
  END IF;
  IF TG_OP = 'UPDATE' AND OLD.status IN ('approved', 'superseded', 'archived') THEN
    IF NEW.child_id IS DISTINCT FROM OLD.child_id
      OR NEW.version IS DISTINCT FROM OLD.version
      OR NEW.predecessor_plan_id IS DISTINCT FROM OLD.predecessor_plan_id
      OR NEW.content IS DISTINCT FROM OLD.content
      OR NEW.source_snapshots IS DISTINCT FROM OLD.source_snapshots
      OR NEW.review_date IS DISTINCT FROM OLD.review_date
      OR NEW.created_by_staff_id IS DISTINCT FROM OLD.created_by_staff_id
      OR NEW.submitted_by_staff_id IS DISTINCT FROM OLD.submitted_by_staff_id
      OR NEW.approved_by_staff_id IS DISTINCT FROM OLD.approved_by_staff_id
      OR NEW.submitted_at IS DISTINCT FROM OLD.submitted_at
      OR NEW.approved_at IS DISTINCT FROM OLD.approved_at
      OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
      RAISE EXCEPTION 'approved Missing From Home plan versions are immutable';
    END IF;
    IF OLD.status = 'superseded' AND NEW.status <> OLD.status THEN
      RAISE EXCEPTION 'superseded Missing From Home plan versions are immutable';
    END IF;
    IF OLD.status = 'archived' AND NEW.status <> OLD.status THEN
      RAISE EXCEPTION 'archived Missing From Home plan versions are immutable';
    END IF;
    IF OLD.status = 'approved' AND NEW.status NOT IN ('approved', 'superseded', 'archived') THEN
      RAISE EXCEPTION 'invalid approved Missing From Home plan transition';
    END IF;
  END IF;
  IF TG_OP = 'DELETE' THEN
    RETURN OLD;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_mfh_approved_immutable
  BEFORE UPDATE OR DELETE ON child_missing_from_home_plans
  FOR EACH ROW EXECUTE FUNCTION prevent_approved_mfh_plan_rewrite();
