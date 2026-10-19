// internal/storage/checkins.go
package storage

import (
	"context"
	"time"
)

type DomainCheckIn struct {
	ID            int64     `db:"id"`
	HomeName      string    `db:"home_name"`
	ChildInitials *string   `db:"child_initials"`
	Date          time.Time `db:"date"`
	Context       *string   `db:"context"`

	Safety        *int `db:"safety"`
	Relationships *int `db:"relationships"`
	Health        *int `db:"health"`
	Education     *int `db:"education"`
	Identity      *int `db:"identity"`
	Coping        *int `db:"coping"`

	Notes     *string   `db:"notes"`
	CreatedAt time.Time `db:"created_at"`
}

func (r *Repo) InsertDomainCheckIn(ctx context.Context, c DomainCheckIn) (int64, error) {
	const q = `
        INSERT INTO domain_checkins
            (home_name, child_initials, date, context,
             safety, relationships, health, education, identity, coping,
             notes, created_at)
        VALUES
            ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11, NOW())
        RETURNING id;
    `
	var id int64
	err := r.db.QueryRowContext(ctx, q,
		c.HomeName, c.ChildInitials, c.Date, c.Context,
		c.Safety, c.Relationships, c.Health, c.Education, c.Identity, c.Coping,
		c.Notes,
	).Scan(&id)
	return id, err
}

func (r *Repo) ListDomainCheckInsForChild(
	ctx context.Context,
	homeName string,
	childInitials string,
	limit int,
) ([]DomainCheckIn, error) {
	if limit <= 0 {
		limit = 50 // sensible default
	}

	const q = `
		SELECT
			id, home_name, child_initials, date, context,
			safety, relationships, health, education, identity, coping,
			notes, created_at
		FROM domain_checkins
		WHERE home_name = $1
		  AND child_initials = $2
		ORDER BY date ASC
		LIMIT $3;
	`

	rows, err := r.db.QueryContext(ctx, q, homeName, childInitials, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DomainCheckIn
	for rows.Next() {
		var d DomainCheckIn
		if err := rows.Scan(
			&d.ID,
			&d.HomeName,
			&d.ChildInitials,
			&d.Date,
			&d.Context,
			&d.Safety,
			&d.Relationships,
			&d.Health,
			&d.Education,
			&d.Identity,
			&d.Coping,
			&d.Notes,
			&d.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// ListDomainCheckInsForChildID derives the legacy check-in keys from the
// canonical child/home relationship. Callers cannot choose another child's
// home name or initials through query parameters.
func (r *Repo) ListDomainCheckInsForChildID(
	ctx context.Context,
	childID int64,
	from *time.Time,
	to *time.Time,
) ([]DomainCheckIn, error) {
	const q = `
SELECT
	dc.id, dc.home_name, dc.child_initials, dc.date, dc.context,
	dc.safety, dc.relationships, dc.health, dc.education, dc.identity, dc.coping,
	dc.notes, dc.created_at
FROM children c
JOIN homes h ON h.id = c.home_id
JOIN domain_checkins dc
	ON dc.home_name = h.name
	AND dc.child_initials = c.initials
WHERE c.id = $1
	AND ($2::timestamptz IS NULL OR dc.date >= $2)
	AND ($3::timestamptz IS NULL OR dc.date <= $3)
ORDER BY dc.date ASC;
`
	rows, err := r.db.QueryContext(ctx, q, childID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]DomainCheckIn, 0)
	for rows.Next() {
		var d DomainCheckIn
		if err := rows.Scan(
			&d.ID,
			&d.HomeName,
			&d.ChildInitials,
			&d.Date,
			&d.Context,
			&d.Safety,
			&d.Relationships,
			&d.Health,
			&d.Education,
			&d.Identity,
			&d.Coping,
			&d.Notes,
			&d.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
