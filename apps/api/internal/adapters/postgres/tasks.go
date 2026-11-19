package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres/pgstore"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
)

// TaskStore implements tasks.Store on PostgreSQL with tenant RLS.
type TaskStore struct {
	pool *pgxpool.Pool
	q    *pgstore.Queries
}

// NewTaskStore wires the store to a pool.
func NewTaskStore(pool *pgxpool.Pool) *TaskStore {
	return &TaskStore{pool: pool, q: pgstore.New(pool)}
}

// Create inserts a task.
func (s *TaskStore) Create(ctx context.Context, t domain.Task) (domain.Task, error) {
	acceptance, err := json.Marshal(t.Acceptance)
	if err != nil {
		return domain.Task{}, err
	}
	tags := t.Tags
	if tags == nil {
		tags = []string{}
	}
	depends := make([]pgtype.UUID, 0, len(t.DependsOn))
	for _, id := range t.DependsOn {
		depends = append(depends, toUUID(id))
	}
	var out domain.Task
	err = db.WithOrg(ctx, s.pool, t.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).CreateTask(ctx, pgstore.CreateTaskParams{
			OrgID:       toUUID(t.OrgID),
			ProjectID:   toUUID(t.ProjectID),
			Ref:         t.Ref,
			Title:       t.Title,
			Description: t.Description,
			Acceptance:  acceptance,
			Priority:    string(t.Priority),
			Tags:        tags,
			CreatedBy:   toUUID(t.CreatedBy),
			DependsOn:   depends,
			Files:       nonNil(t.Files),
			OutOfScope:  nonNil(t.OutOfScope),
			TestPlan:    nonNil(t.TestPlan),
			Rollback:    nonNil(t.Rollback),
		})
		if err != nil {
			return mapUnique(err)
		}
		out, err = taskFromRow(row)
		return err
	})
	return out, err
}

// Get returns a task by id.
func (s *TaskStore) Get(ctx context.Context, orgID, id string) (domain.Task, error) {
	return s.one(ctx, orgID, func(ctx context.Context, q *pgstore.Queries) (pgstore.Task, error) {
		return q.GetTask(ctx, pgstore.GetTaskParams{ID: toUUID(id), OrgID: toUUID(orgID)})
	})
}

// GetByRef returns a task by its human reference within a project.
func (s *TaskStore) GetByRef(ctx context.Context, orgID, projectID, ref string) (domain.Task, error) {
	return s.one(ctx, orgID, func(ctx context.Context, q *pgstore.Queries) (pgstore.Task, error) {
		return q.GetTaskByRef(ctx, pgstore.GetTaskByRefParams{
			OrgID: toUUID(orgID), ProjectID: toUUID(projectID), Ref: ref,
		})
	})
}

func (s *TaskStore) one(ctx context.Context, orgID string, query func(context.Context, *pgstore.Queries) (pgstore.Task, error)) (domain.Task, error) {
	var out domain.Task
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := query(ctx, s.q.WithTx(tx))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out, err = taskFromRow(row)
		return err
	})
	return out, err
}

// List returns tasks of a project, newest first.
func (s *TaskStore) List(ctx context.Context, orgID, projectID, status, assignee string, limit int) ([]domain.Task, error) {
	var out []domain.Task
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		params := pgstore.ListTasksParams{
			OrgID:     toUUID(orgID),
			ProjectID: toUUID(projectID),
			Limit:     int32(limit), //nolint:gosec // clamped by the service
		}
		if status != "" {
			params.Status = &status
		}
		if assignee != "" {
			params.Assignee = toUUID(assignee)
		}
		rows, err := s.q.WithTx(tx).ListTasks(ctx, params)
		if err != nil {
			return err
		}
		out = make([]domain.Task, 0, len(rows))
		for _, row := range rows {
			t, err := taskFromRow(row)
			if err != nil {
				return err
			}
			out = append(out, t)
		}
		return nil
	})
	return out, err
}

// Count returns how many tasks the project has (used for ref generation).
func (s *TaskStore) Count(ctx context.Context, orgID, projectID string) (int, error) {
	var n int64
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		n, err = s.q.WithTx(tx).CountTasks(ctx, toUUID(projectID))
		return err
	})
	return int(n), err
}

// UpdateStatus persists the state-machine result.
func (s *TaskStore) UpdateStatus(ctx context.Context, t domain.Task) (domain.Task, error) {
	var out domain.Task
	err := db.WithOrg(ctx, s.pool, t.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).UpdateTaskStatus(ctx, pgstore.UpdateTaskStatusParams{
			ID:            toUUID(t.ID),
			OrgID:         toUUID(t.OrgID),
			Status:        string(t.Status),
			BlockedReason: nilIfEmpty(t.BlockedReason),
			ReadyAt:       timePtr(t.ReadyAt),
			DoneAt:        timePtr(t.DoneAt),
		})
		if err != nil {
			return err
		}
		out, err = taskFromRow(row)
		return err
	})
	return out, err
}

func taskFromRow(row pgstore.Task) (domain.Task, error) {
	t := domain.Task{
		ID:          uuidString(row.ID),
		OrgID:       uuidString(row.OrgID),
		ProjectID:   uuidString(row.ProjectID),
		Ref:         row.Ref,
		Title:       row.Title,
		Description: row.Description,
		Status:      domain.TaskStatus(row.Status),
		Priority:    domain.Priority(row.Priority),
		Tags:        row.Tags,
		Files:       row.Files,
		OutOfScope:  row.OutOfScope,
		TestPlan:    row.TestPlan,
		Rollback:    row.Rollback,
		DependsOn:   uuidsToStrings(row.DependsOn),
		AssigneeID:  uuidString(row.AssigneeID),
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}
	if len(row.Acceptance) > 0 {
		if err := json.Unmarshal(row.Acceptance, &t.Acceptance); err != nil {
			return domain.Task{}, err
		}
	}
	if row.BlockedReason != nil {
		t.BlockedReason = *row.BlockedReason
	}
	if row.ReadyAt.Valid {
		v := row.ReadyAt.Time
		t.ReadyAt = &v
	}
	if row.DoneAt.Valid {
		v := row.DoneAt.Time
		t.DoneAt = &v
	}
	return t, nil
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func timePtr(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

// Update edits the mutable task fields.
func (s *TaskStore) Update(ctx context.Context, task domain.Task) (domain.Task, error) {
	acceptance, err := json.Marshal(task.Acceptance)
	if err != nil {
		return domain.Task{}, err
	}
	depends := make([]pgtype.UUID, 0, len(task.DependsOn))
	for _, id := range task.DependsOn {
		depends = append(depends, toUUID(id))
	}
	var out domain.Task
	err = db.WithOrg(ctx, s.pool, task.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).UpdateTask(ctx, pgstore.UpdateTaskParams{
			ID: toUUID(task.ID), OrgID: toUUID(task.OrgID), Title: task.Title,
			Description: task.Description, Priority: string(task.Priority),
			AssigneeID: optionalUUID(task.AssigneeID), Tags: task.Tags,
			Acceptance: acceptance, DependsOn: depends,
			Files: nonNil(task.Files), OutOfScope: nonNil(task.OutOfScope),
			TestPlan: nonNil(task.TestPlan), Rollback: nonNil(task.Rollback),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		mapped, err := taskFromRow(row)
		if err != nil {
			return err
		}
		out = mapped
		return nil
	})
	return out, err
}

// NextTaskRef atomically increments the project's task sequence.
func (s *TaskStore) NextTaskRef(ctx context.Context, orgID, projectID string) (int64, error) {
	var seq int64
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		n, err := s.q.WithTx(tx).NextTaskRef(ctx, pgstore.NextTaskRefParams{
			ID: toUUID(projectID), OrgID: toUUID(orgID),
		})
		seq = n
		return err
	})
	return seq, err
}

func uuidsToStrings(ids []pgtype.UUID) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, uuidString(id))
	}
	return out
}

// nonNil keeps the NOT NULL text[] columns valid for empty input.
func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
