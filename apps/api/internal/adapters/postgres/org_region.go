package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
)

// OrgRegion reads an organisation's data-plane region.
type OrgRegion struct{ pool *pgxpool.Pool }

// NewOrgRegion wires the reader.
func NewOrgRegion(pool *pgxpool.Pool) *OrgRegion { return &OrgRegion{pool: pool} }

// Region implements httpapi.OrgRegionReader.
func (o *OrgRegion) Region(ctx context.Context, orgID string) (string, error) {
	var region string
	err := db.WithOrg(ctx, o.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, "SELECT region FROM orgs WHERE id = $1", toUUID(orgID)).Scan(&region)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrNotFound
	}
	return region, err
}
