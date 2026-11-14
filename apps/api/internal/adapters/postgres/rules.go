package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres/pgstore"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/rules"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
)

// RuleStore implements rules.Store.
type RuleStore struct {
	pool *pgxpool.Pool
	q    *pgstore.Queries
}

// NewRuleStore wires the store.
func NewRuleStore(pool *pgxpool.Pool) *RuleStore {
	return &RuleStore{pool: pool, q: pgstore.New(pool)}
}

func optionalUUID(id string) pgtype.UUID {
	if id == "" {
		return pgtype.UUID{}
	}
	return toUUID(id)
}

// CreateRule inserts a rule.
func (s *RuleStore) CreateRule(ctx context.Context, r rules.Rule) (rules.Rule, error) {
	var out rules.Rule
	err := db.WithOrg(ctx, s.pool, r.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).CreateRule(ctx, pgstore.CreateRuleParams{
			OrgID: toUUID(r.OrgID), ProjectID: optionalUUID(r.ProjectID), Level: r.Level,
			ScopeRef: nilIfEmpty(r.ScopeRef), Name: r.Name,
		})
		if err != nil {
			return mapUnique(err)
		}
		out = ruleFromRow(row)
		return nil
	})
	return out, err
}

// ListRules lists org rules, optionally for one project.
func (s *RuleStore) ListRules(ctx context.Context, orgID, projectID string, limit int) ([]rules.Rule, error) {
	var out []rules.Rule
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListRules(ctx, pgstore.ListRulesParams{
			OrgID: toUUID(orgID), Column2: optionalUUID(projectID),
			Limit: int32(limit), //nolint:gosec // clamped
		})
		if err != nil {
			return err
		}
		out = make([]rules.Rule, 0, len(rows))
		for _, row := range rows {
			out = append(out, ruleFromRow(row))
		}
		return nil
	})
	return out, err
}

// GetRule returns one rule.
func (s *RuleStore) GetRule(ctx context.Context, orgID, id string) (rules.Rule, error) {
	var out rules.Rule
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).GetRule(ctx, pgstore.GetRuleParams{ID: toUUID(id), OrgID: toUUID(orgID)})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = ruleFromRow(row)
		return nil
	})
	return out, err
}

// CreateVersion appends a version.
func (s *RuleStore) CreateVersion(ctx context.Context, orgID, ruleID, body string, sum []byte) (rules.Version, error) {
	var out rules.Version
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).CreateRuleVersion(ctx, pgstore.CreateRuleVersionParams{
			RuleID: toUUID(ruleID), Body: body, Sha256: sum,
		})
		if err != nil {
			return err
		}
		out = versionFromRow(row)
		return nil
	})
	return out, err
}

// ListVersions returns versions newest first.
func (s *RuleStore) ListVersions(ctx context.Context, orgID, ruleID string) ([]rules.Version, error) {
	var out []rules.Version
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListRuleVersions(ctx, toUUID(ruleID))
		if err != nil {
			return err
		}
		out = make([]rules.Version, 0, len(rows))
		for _, row := range rows {
			out = append(out, versionFromRow(row))
		}
		return nil
	})
	return out, err
}

// PublishVersion publishes a version.
func (s *RuleStore) PublishVersion(ctx context.Context, orgID, ruleID string, version int, userID string) (rules.Version, error) {
	var out rules.Version
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).PublishRuleVersion(ctx, pgstore.PublishRuleVersionParams{
			RuleID: toUUID(ruleID), Version: int32(version), PublishedBy: optionalUUID(userID), //nolint:gosec // small
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrInvalidTransition
			}
			return err
		}
		out = versionFromRow(row)
		return nil
	})
	return out, err
}

// Active returns published rules for the prompt.
func (s *RuleStore) Active(ctx context.Context, orgID, projectID string) ([]rules.Active, error) {
	var out []rules.Active
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ActiveRules(ctx, pgstore.ActiveRulesParams{
			OrgID: toUUID(orgID), ProjectID: optionalUUID(projectID),
		})
		if err != nil {
			return err
		}
		out = make([]rules.Active, 0, len(rows))
		for _, row := range rows {
			out = append(out, rules.Active{
				RuleID: uuidString(row.ID), Name: row.Name, Level: row.Level,
				Version: int(row.Version), Body: row.Body, SHA256: row.Sha256,
			})
		}
		return nil
	})
	return out, err
}

func ruleFromRow(row pgstore.Rule) rules.Rule {
	r := rules.Rule{
		ID: uuidString(row.ID), OrgID: uuidString(row.OrgID), Level: row.Level,
		Name: row.Name, CreatedAt: row.CreatedAt.Time,
	}
	if row.ProjectID.Valid {
		r.ProjectID = uuidString(row.ProjectID)
	}
	if row.ScopeRef != nil {
		r.ScopeRef = *row.ScopeRef
	}
	return r
}

func versionFromRow(row pgstore.RuleVersion) rules.Version {
	v := rules.Version{
		ID: uuidString(row.ID), RuleID: uuidString(row.RuleID), Version: int(row.Version),
		Body: row.Body, SHA256: row.Sha256, CreatedAt: row.CreatedAt.Time,
	}
	if row.PublishedAt.Valid {
		t := row.PublishedAt.Time
		v.PublishedAt = &t
	}
	if row.PublishedBy.Valid {
		v.PublishedBy = uuidString(row.PublishedBy)
	}
	return v
}

// GetVersion returns one rule version by id.
func (s *RuleStore) GetVersion(ctx context.Context, orgID, id string) (rules.Version, error) {
	var out rules.Version
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).GetRuleVersion(ctx, toUUID(id))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = versionFromRow(row)
		return nil
	})
	return out, err
}

// CreatePublishApproval inserts a rule_publish approval.
func (s *RuleStore) CreatePublishApproval(ctx context.Context, orgID, versionID, requestedBy string, expires time.Time) (rules.PublishRequest, error) {
	var out rules.PublishRequest
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		version, err := s.q.WithTx(tx).GetRuleVersion(ctx, toUUID(versionID))
		if err != nil {
			return err
		}
		row, err := s.q.WithTx(tx).CreateRulePublishApproval(ctx, pgstore.CreateRulePublishApprovalParams{
			OrgID: toUUID(orgID), RuleVersionID: toUUID(versionID),
			RequestedBy: toUUID(requestedBy), ExpiresAt: pgtype.Timestamptz{Time: expires, Valid: true},
		})
		if err != nil {
			return err
		}
		out = rules.PublishRequest{
			ID: uuidString(row.ID), RuleID: uuidString(version.RuleID), Version: int(version.Version),
			RequestedBy: requestedBy,
		}
		return nil
	})
	return out, err
}

// PendingPublishes lists pending rule_publish approvals.
func (s *RuleStore) PendingPublishes(ctx context.Context, orgID string) ([]rules.PublishRequest, error) {
	var out []rules.PublishRequest
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListPendingRuleApprovals(ctx, toUUID(orgID))
		if err != nil {
			return err
		}
		for _, row := range rows {
			out = append(out, rules.PublishRequest{
				ID: uuidString(row.ID), RuleID: uuidString(row.RuleID), RuleName: row.RuleName,
				Version: int(row.Version),
			})
		}
		return nil
	})
	return out, err
}

// GetPublishApproval returns one rule_publish approval.
func (s *RuleStore) GetPublishApproval(ctx context.Context, orgID, id string) (rules.PublishRequest, error) {
	var out rules.PublishRequest
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).GetRuleApproval(ctx, pgstore.GetRuleApprovalParams{ID: toUUID(id), OrgID: toUUID(orgID)})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out.ID = uuidString(row.ID)
		if row.RequestedBy.Valid {
			out.RequestedBy = uuidString(row.RequestedBy)
		}
		if row.RuleVersionID.Valid {
			version, verr := s.q.WithTx(tx).GetRuleVersion(ctx, row.RuleVersionID)
			if verr != nil {
				return verr
			}
			out.RuleID = uuidString(version.RuleID)
			out.Version = int(version.Version)
		}
		return nil
	})
	return out, err
}

// DecidePublishApproval marks the approval decided.
func (s *RuleStore) DecidePublishApproval(ctx context.Context, orgID, id, status, userID, reason string) error {
	return db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		_, err := s.q.WithTx(tx).DecideRuleApproval(ctx, pgstore.DecideRuleApprovalParams{
			ID: toUUID(id), OrgID: toUUID(orgID), Status: status,
			DecidedBy: toUUID(userID), Reason: nilIfEmpty(reason),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrInvalidTransition
			}
			return err
		}
		return nil
	})
}
