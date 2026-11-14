// Package queue wires River (PostgreSQL-backed jobs) for agent runs.
package queue

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

// RunArgs identifies one agent run job.
type RunArgs struct {
	RunID string `json:"run_id"`
	OrgID string `json:"org_id"`
}

// Kind implements river.JobArgs.
func (RunArgs) Kind() string { return "agent_run" }

// Migrate applies River's own migrations (idempotent). It runs from aipctl
// migrate so that deployments never start a worker against a missing schema.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return fmt.Errorf("queue: migrator: %w", err)
	}
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return fmt.Errorf("queue: migrate: %w", err)
	}
	return nil
}

// NewClient builds a River client. Workers may be empty for producers.
func NewClient(pool *pgxpool.Pool, workers *river.Workers, maxWorkers int) (*river.Client[pgx.Tx], error) {
	cfg := &river.Config{}
	if workers != nil {
		// River requires workers whenever queues are configured; producers
		// (aipd) only insert jobs and run with neither.
		cfg.Workers = workers
		cfg.Queues = map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: maxWorkers},
		}
	}
	client, err := river.NewClient(riverpgxv5.New(pool), cfg)
	if err != nil {
		return nil, fmt.Errorf("queue: client: %w", err)
	}
	return client, nil
}

// NewWorkers returns an empty worker registry.
func NewWorkers() *river.Workers { return river.NewWorkers() }

// Enqueue inserts a run job, unique per run id so retries cannot double-execute.
func Enqueue(ctx context.Context, client *river.Client[pgx.Tx], runID, orgID string) error {
	_, err := client.Insert(ctx, RunArgs{RunID: runID, OrgID: orgID}, &river.InsertOpts{
		UniqueOpts: river.UniqueOpts{ByArgs: true},
	})
	if err != nil {
		return fmt.Errorf("queue: enqueue run %s: %w", runID, err)
	}
	return nil
}

// Enqueuer adapts the River client to the runs.Enqueuer port.
type Enqueuer struct {
	Client *river.Client[pgx.Tx]
}

// Enqueue schedules a run job.
func (e Enqueuer) Enqueue(ctx context.Context, runID, orgID string) error {
	if e.Client == nil {
		return fmt.Errorf("queue: client is not configured")
	}
	return Enqueue(ctx, e.Client, runID, orgID)
}
