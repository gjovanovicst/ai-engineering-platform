package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres/pgstore"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auditwitness"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
)

// AuditWitnessStore implements auditwitness.Store.
type AuditWitnessStore struct {
	pool *pgxpool.Pool
	q    *pgstore.Queries
}

// NewAuditWitnessStore wires the store.
func NewAuditWitnessStore(pool *pgxpool.Pool) *AuditWitnessStore {
	return &AuditWitnessStore{pool: pool, q: pgstore.New(pool)}
}

func (s *AuditWitnessStore) Head(ctx context.Context, orgID string) (auditwitness.Head, error) {
	var head auditwitness.Head
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).AuditHead(ctx, toUUID(orgID))
		if err != nil {
			return err
		}
		head = auditwitness.Head{Rows: row.Rows, LastID: row.LastID, LastHash: row.LastHash}
		return nil
	})
	return head, err
}

func (s *AuditWitnessStore) HashAt(ctx context.Context, orgID string, id int64) (string, error) {
	var hash string
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).AuditHashAt(ctx, pgstore.AuditHashAtParams{OrgID: toUUID(orgID), ID: id})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		hash = row
		return nil
	})
	return hash, err
}

func (s *AuditWitnessStore) Insert(ctx context.Context, orgID, key string, head auditwitness.Head) (auditwitness.Witness, error) {
	var out auditwitness.Witness
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).InsertAuditWitness(ctx, pgstore.InsertAuditWitnessParams{
			OrgID: toUUID(orgID), ObjectKey: key, RowCount: head.Rows, LastID: head.LastID, LastHash: head.LastHash,
		})
		if err != nil {
			return err
		}
		out = auditwitness.Witness{ID: uuidString(row.ID), ObjectKey: row.ObjectKey, RowCount: row.RowCount, LastID: row.LastID, LastHash: row.LastHash, CreatedAt: row.CreatedAt.Time}
		return nil
	})
	return out, err
}

func (s *AuditWitnessStore) Latest(ctx context.Context, orgID string) (auditwitness.Witness, error) {
	var out auditwitness.Witness
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).LatestAuditWitness(ctx, toUUID(orgID))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = auditwitness.Witness{ID: uuidString(row.ID), ObjectKey: row.ObjectKey, RowCount: row.RowCount, LastID: row.LastID, LastHash: row.LastHash, CreatedAt: row.CreatedAt.Time}
		return nil
	})
	return out, err
}
