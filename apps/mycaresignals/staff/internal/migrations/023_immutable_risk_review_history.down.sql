DROP TRIGGER IF EXISTS trg_child_risk_reviews_immutable ON child_risk_reviews;
DROP TABLE IF EXISTS child_risk_reviews;
DROP FUNCTION IF EXISTS prevent_child_risk_review_mutation();
