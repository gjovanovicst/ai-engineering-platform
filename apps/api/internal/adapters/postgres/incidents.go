package postgres

import (
	"context"
	"errors"

	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres/pgstore"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/incidents"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
)

// IncidentStore implements incidents.Store.
type IncidentStore struct {
	pool *pgxpool.Pool
	q    *pgstore.Queries
}

// NewIncidentStore wires the store.
func NewIncidentStore(pool *pgxpool.Pool) *IncidentStore {
	return &IncidentStore{pool: pool, q: pgstore.New(pool)}
}

func (s *IncidentStore) Create(ctx context.Context, orgID, projectID, title, severity, userID string) (incidents.Incident, error) {
	var out incidents.Incident
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		var project *string
		if projectID != "" {
			project = &projectID
		}
		row, err := s.q.WithTx(tx).CreateIncident(ctx, pgstore.CreateIncidentParams{
			OrgID: toUUID(orgID), ProjectID: toOptionalUUID(project), Title: title, Severity: severity, CreatedBy: toOptionalUUID(&userID),
		})
		if err != nil {
			return err
		}
		out = incidentFromRow(row.ID, row.ProjectID, row.Title, row.Severity, row.Status, row.OpenedAt.Time, row.ResolvedAt)
		return nil
	})
	return out, err
}

func (s *IncidentStore) List(ctx context.Context, orgID string, limit int) ([]incidents.Incident, error) {
	var out []incidents.Incident
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListIncidents(ctx, pgstore.ListIncidentsParams{OrgID: toUUID(orgID), Limit: int32(limit)}) //nolint:gosec // clamped by the service
		if err != nil {
			return err
		}
		for _, row := range rows {
			out = append(out, incidentFromRow(row.ID, row.ProjectID, row.Title, row.Severity, row.Status, row.OpenedAt.Time, row.ResolvedAt))
		}
		return nil
	})
	return out, err
}

func (s *IncidentStore) Resolve(ctx context.Context, orgID, id string) (incidents.Incident, error) {
	var out incidents.Incident
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).ResolveIncident(ctx, pgstore.ResolveIncidentParams{ID: toUUID(id), OrgID: toUUID(orgID)})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = incidentFromRow(row.ID, row.ProjectID, row.Title, row.Severity, row.Status, row.OpenedAt.Time, row.ResolvedAt)
		return nil
	})
	return out, err
}

func (s *IncidentStore) Audit(ctx context.Context, orgID, actorID, action, target string, metadata []byte) error {
	return db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return s.q.WithTx(tx).RecordOrgAudit(ctx, pgstore.RecordOrgAuditParams{
			OrgID: toUUID(orgID), ActorID: toUUID(actorID), Action: action, Target: target, Metadata: metadata,
		})
	})
}

func toOptionalUUID(id *string) pgtype.UUID {
	if id == nil || *id == "" {
		return pgtype.UUID{}
	}
	return toUUID(*id)
}

func incidentFromRow(id, projectID pgtype.UUID, title, severity, status string, opened time.Time, resolved pgtype.Timestamptz) incidents.Incident {
	out := incidents.Incident{ID: uuidString(id), Title: title, Severity: severity, Status: status, OpenedAt: opened}
	if projectID.Valid {
		value := uuidString(projectID)
		out.ProjectID = &value
	}
	if resolved.Valid {
		value := resolved.Time
		out.ResolvedAt = &value
	}
	return out
}
