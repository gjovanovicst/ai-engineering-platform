package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres/pgstore"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/orgs"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
)

// OrgStore implements orgs.Store.
type OrgStore struct {
	pool *pgxpool.Pool
	q    *pgstore.Queries
}

// NewOrgStore wires the store.
func NewOrgStore(pool *pgxpool.Pool) *OrgStore { return &OrgStore{pool: pool, q: pgstore.New(pool)} }

// Get returns one org.
func (s *OrgStore) Get(ctx context.Context, id string) (orgs.Org, error) {
	row, err := s.q.GetOrg(ctx, toUUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return orgs.Org{}, nil
		}
		return orgs.Org{}, err
	}
	return orgFromGetRow(row), nil
}

// UpdateName renames the org.
func (s *OrgStore) UpdateName(ctx context.Context, id, name string) (orgs.Org, error) {
	row, err := s.q.UpdateOrgName(ctx, pgstore.UpdateOrgNameParams{ID: toUUID(id), Name: name})
	if err != nil {
		return orgs.Org{}, err
	}
	return orgFromUpdateRow(row), nil
}

// Delete removes the org (cascades to its projects and runs).
func (s *OrgStore) Delete(ctx context.Context, id string) error {
	return s.q.DeleteOrg(ctx, toUUID(id))
}

// Audit appends an audit entry.
func (s *OrgStore) Audit(ctx context.Context, orgID, actorID, action, target string, metadata []byte) error {
	return db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return s.q.WithTx(tx).RecordOrgAudit(ctx, pgstore.RecordOrgAuditParams{
			OrgID: toUUID(orgID), ActorID: toUUID(actorID), Action: action, Target: target, Metadata: metadata,
		})
	})
}

// SetLegalHold toggles the org legal hold.
func (s *OrgStore) SetLegalHold(ctx context.Context, id string, hold bool) (orgs.Org, error) {
	row, err := s.q.SetOrgLegalHold(ctx, pgstore.SetOrgLegalHoldParams{ID: toUUID(id), LegalHold: hold})
	if err != nil {
		return orgs.Org{}, err
	}
	return orgFromSetLegalHoldRow(row), nil
}

func orgFromGetRow(row pgstore.GetOrgRow) orgs.Org {
	return orgs.Org{ID: uuidString(row.ID), Slug: row.Slug, Name: row.Name, MaxConcurrentRuns: int(row.MaxConcurrentRuns), LegalHold: row.LegalHold, Region: row.Region, CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time}
}

func orgFromUpdateRow(row pgstore.UpdateOrgNameRow) orgs.Org {
	return orgs.Org{ID: uuidString(row.ID), Slug: row.Slug, Name: row.Name, MaxConcurrentRuns: int(row.MaxConcurrentRuns), LegalHold: row.LegalHold, Region: row.Region, CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time}
}

func orgFromSetLegalHoldRow(row pgstore.SetOrgLegalHoldRow) orgs.Org {
	return orgs.Org{ID: uuidString(row.ID), Slug: row.Slug, Name: row.Name, MaxConcurrentRuns: int(row.MaxConcurrentRuns), LegalHold: row.LegalHold, Region: row.Region, CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time}
}

// SetRegion stores the org residency label.
func (s *OrgStore) SetRegion(ctx context.Context, id, region string) (orgs.Org, error) {
	row, err := s.q.SetOrgRegion(ctx, pgstore.SetOrgRegionParams{ID: toUUID(id), Region: region})
	if err != nil {
		return orgs.Org{}, err
	}
	return orgs.Org{ID: uuidString(row.ID), Slug: row.Slug, Name: row.Name, MaxConcurrentRuns: int(row.MaxConcurrentRuns), LegalHold: row.LegalHold, Region: row.Region, CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time}, nil
}
