package storage

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"

	"therapeutic-engine/internal/models"
)

func TestSixDomainFrameworkDraftSubmitApproveAgainstPostgres(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()

	ctx := context.Background()
	var homeID, staffID, childID int64
	if err := tx.QueryRowContext(ctx, `INSERT INTO homes (name, external_ref) VALUES ('Synthetic Framework Test Home', 'SYNTHETIC-FRAMEWORK-TEST') RETURNING id`).Scan(&homeID); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRowContext(ctx, `INSERT INTO staff_users (email, display_name, external_auth_subject) VALUES ('synthetic-framework-test@example.invalid', 'Synthetic Manager', 'synthetic-framework-test') RETURNING id`).Scan(&staffID); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRowContext(ctx, `INSERT INTO children (home_id, initials, date_of_birth) VALUES ($1, 'SF', DATE '2012-01-01') RETURNING id`, homeID).Scan(&childID); err != nil {
		t.Fatal(err)
	}

	repo := &TxRepo{q: tx}
	baseline := &models.ChildOnboardingBaseline{
		ChildID: models.ChildID(childID), OnboardingPathway: "new_placement",
		ImmediateInformationStatus:  "partly_known",
		BackgroundInformationStatus: "awaiting_confirmation",
		SafetyInformationStatus:     "not_known_yet",
	}
	domains := make([]models.ChildOnboardingDomainState, 0, len(models.CanonicalProgressDomains))
	for _, domain := range models.CanonicalProgressDomains {
		domains = append(domains, models.ChildOnboardingDomainState{DomainKey: domain, InformationStatus: "not_known_yet"})
	}
	priorities := []models.ChildPriorityDomain{{DomainKey: models.CanonicalProgressDomains[0], FocusSummary: "Synthetic focus", HopedForChange: "Synthetic everyday change", SortOrder: 1}}
	sources := []models.ChildOnboardingFrameworkSource{{SourceType: "staff_observation", SourceName: "Synthetic source"}}

	if err := repo.SaveSixDomainFramework(ctx, baseline, domains, priorities, sources, "draft", staffID); err != nil {
		t.Fatalf("save draft: %v", err)
	}
	firstID := baseline.ID
	if baseline.FrameworkVersion != 1 || baseline.Status != "draft" {
		t.Fatalf("draft=%#v", baseline)
	}
	baseline.ImmediateInformation = "Updated synthetic information"
	if err := repo.SaveSixDomainFramework(ctx, baseline, domains, priorities, sources, "submitted", staffID); err != nil {
		t.Fatalf("resume and submit: %v", err)
	}
	if baseline.ID != firstID || baseline.Status != "submitted" {
		t.Fatalf("submitted=%#v", baseline)
	}
	approved, err := repo.ApproveLatestSixDomainFramework(ctx, models.ChildID(childID), "Synthetic Manager")
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if approved.Status != "approved" {
		t.Fatalf("approval status=%s", approved.Status)
	}

	for _, table := range []string{"child_risks", "child_care_plans", "child_goals", "child_targets"} {
		var count int
		if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM "+table+" WHERE child_id = $1", childID).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("framework created %d rows in %s", count, table)
		}
	}
}
