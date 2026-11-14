package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/runs"
)

// OrgGate limits concurrent runs per organisation using advisory locks.
type OrgGate struct{ pool *pgxpool.Pool }

// NewOrgGate wires the gate.
func NewOrgGate(pool *pgxpool.Pool) *OrgGate { return &OrgGate{pool: pool} }

// Acquire claims a slot (0..slots-1) for the org. The returned release must be
// called exactly once.
func (g *OrgGate) Acquire(ctx context.Context, orgID string, slots int) (func(), bool, error) {
	if slots < 1 {
		slots = 1
	}
	conn, err := g.pool.Acquire(ctx)
	if err != nil {
		return nil, false, err
	}
	for slot := 0; slot < slots; slot++ {
		var ok bool
		if err := conn.QueryRow(ctx, "select pg_try_advisory_lock(hashtext($1), $2)", orgID, slot).Scan(&ok); err != nil {
			conn.Release()
			return nil, false, err
		}
		if ok {
			slotNum := slot
			release := func() {
				unlockCtx, cancel := context.WithTimeout(context.Background(), 5e9)
				defer cancel()
				if _, err := conn.Exec(unlockCtx, "select pg_advisory_unlock(hashtext($1), $2)", orgID, slotNum); err != nil {
					raw := conn.Hijack()
					_ = raw.Close(unlockCtx)
					return
				}
				conn.Release()
			}
			return release, true, nil
		}
	}
	conn.Release()
	return nil, false, nil
}

var _ runs.OrgGate = (*OrgGate)(nil)

func (g *OrgGate) String() string { return fmt.Sprintf("org-gate(%p)", g) }
