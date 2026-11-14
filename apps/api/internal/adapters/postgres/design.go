package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres/pgstore"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/design"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
)

// DesignStore implements design.Store.
type DesignStore struct {
	pool *pgxpool.Pool
	q    *pgstore.Queries
}

// NewDesignStore wires the store.
func NewDesignStore(pool *pgxpool.Pool) *DesignStore {
	return &DesignStore{pool: pool, q: pgstore.New(pool)}
}

// ListTokens lists tokens.
func (s *DesignStore) ListTokens(ctx context.Context, orgID string) ([]design.Token, error) {
	var out []design.Token
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListDesignTokens(ctx, toUUID(orgID))
		if err != nil {
			return err
		}
		out = make([]design.Token, 0, len(rows))
		for _, row := range rows {
			out = append(out, design.Token{ID: uuidString(row.ID), Name: row.Name, Value: row.Value, Category: row.Category})
		}
		return nil
	})
	return out, err
}

// CreateToken inserts a token.
func (s *DesignStore) CreateToken(ctx context.Context, orgID string, t design.Token) (design.Token, error) {
	var out design.Token
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).CreateDesignToken(ctx, pgstore.CreateDesignTokenParams{
			OrgID: toUUID(orgID), Name: t.Name, Value: t.Value, Category: t.Category,
		})
		if err != nil {
			return mapUnique(err)
		}
		out = design.Token{ID: uuidString(row.ID), Name: row.Name, Value: row.Value, Category: row.Category}
		return nil
	})
	return out, err
}

// UpdateToken updates a token.
func (s *DesignStore) UpdateToken(ctx context.Context, orgID, id string, t design.Token) (design.Token, error) {
	var out design.Token
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).UpdateDesignToken(ctx, pgstore.UpdateDesignTokenParams{
			ID: toUUID(id), OrgID: toUUID(orgID), Value: t.Value, Category: t.Category,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = design.Token{ID: uuidString(row.ID), Name: row.Name, Value: row.Value, Category: row.Category}
		return nil
	})
	return out, err
}

// DeleteToken deletes a token.
func (s *DesignStore) DeleteToken(ctx context.Context, orgID, id string) error {
	return db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return s.q.WithTx(tx).DeleteDesignToken(ctx, pgstore.DeleteDesignTokenParams{ID: toUUID(id), OrgID: toUUID(orgID)})
	})
}

// CountTokens counts tokens.
func (s *DesignStore) CountTokens(ctx context.Context, orgID string) (int, error) {
	count := 0
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		n, err := s.q.WithTx(tx).CountDesignTokens(ctx, toUUID(orgID))
		count = int(n)
		return err
	})
	return count, err
}

// ListComponents lists components.
func (s *DesignStore) ListComponents(ctx context.Context, orgID string) ([]design.Component, error) {
	var out []design.Component
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListComponents(ctx, toUUID(orgID))
		if err != nil {
			return err
		}
		out = make([]design.Component, 0, len(rows))
		for _, row := range rows {
			out = append(out, componentFromRow(row))
		}
		return nil
	})
	return out, err
}

// CreateComponent inserts a component.
func (s *DesignStore) CreateComponent(ctx context.Context, orgID string, c design.Component) (design.Component, error) {
	var out design.Component
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).CreateComponent(ctx, pgstore.CreateComponentParams{
			OrgID: toUUID(orgID), Name: c.Name, Purpose: c.Purpose, Rules: c.Rules, Status: c.Status,
			Category: c.Category, Framework: c.Framework, Package: c.Package, ImportPath: c.ImportPath,
			Version: c.Version, Docs: c.Docs, Example: c.Example,
		})
		if err != nil {
			return mapUnique(err)
		}
		out = componentFromRow(row)
		return nil
	})
	return out, err
}

// UpdateComponent updates a component.
func (s *DesignStore) UpdateComponent(ctx context.Context, orgID, id string, c design.Component) (design.Component, error) {
	var out design.Component
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).UpdateComponent(ctx, pgstore.UpdateComponentParams{
			ID: toUUID(id), OrgID: toUUID(orgID), Purpose: c.Purpose, Rules: c.Rules, Status: c.Status,
			Category: c.Category, Framework: c.Framework, Package: c.Package, ImportPath: c.ImportPath,
			Version: c.Version, Docs: c.Docs, Example: c.Example,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = componentFromRow(row)
		return nil
	})
	return out, err
}

// DeleteComponent deletes a component.
func (s *DesignStore) DeleteComponent(ctx context.Context, orgID, id string) error {
	return db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return s.q.WithTx(tx).DeleteComponent(ctx, pgstore.DeleteComponentParams{ID: toUUID(id), OrgID: toUUID(orgID)})
	})
}

// CountComponents counts components.
func (s *DesignStore) CountComponents(ctx context.Context, orgID string) (int, error) {
	count := 0
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		n, err := s.q.WithTx(tx).CountComponents(ctx, toUUID(orgID))
		count = int(n)
		return err
	})
	return count, err
}

func componentFromRow(row pgstore.ComponentEntry) design.Component {
	return design.Component{
		ID: uuidString(row.ID), Name: row.Name, Purpose: row.Purpose, Rules: row.Rules, Status: row.Status,
		Category: row.Category, Framework: row.Framework, Package: row.Package, ImportPath: row.ImportPath,
		Version: row.Version, Docs: row.Docs, Example: row.Example,
	}
}

// ListForbiddenComponents returns import paths that must not be used.
func (s *DesignStore) ListForbiddenComponents(ctx context.Context, orgID string) ([]design.ForbiddenComponent, error) {
	var out []design.ForbiddenComponent
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListForbiddenComponents(ctx, toUUID(orgID))
		if err != nil {
			return err
		}
		for _, row := range rows {
			out = append(out, design.ForbiddenComponent{Name: row.Name, ImportPath: row.ImportPath})
		}
		return nil
	})
	return out, err
}
