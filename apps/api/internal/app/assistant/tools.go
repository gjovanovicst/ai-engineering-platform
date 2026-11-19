package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/projects"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/runs"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/tasks"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

type tool struct {
	name        string
	description string
	schema      json.RawMessage
	readOnly    bool
	preview     func(input json.RawMessage) string
	run         func(ctx context.Context, orgID string, input json.RawMessage) (any, error)
}

func (s *Service) tools() map[string]tool {
	return map[string]tool{
		"list_projects": {
			name:        "list_projects",
			description: "List the projects in the workspace (id, name, slug, status).",
			schema:      json.RawMessage(`{"type":"object","properties":{}}`),
			readOnly:    true,
			run: func(ctx context.Context, orgID string, _ json.RawMessage) (any, error) {
				projects, err := s.platform.ListProjects(ctx, orgID, 50)
				if err != nil {
					return nil, err
				}
				out := make([]map[string]string, 0, len(projects))
				for _, p := range projects {
					out = append(out, map[string]string{"id": p.ID, "name": p.Name, "slug": p.Slug, "status": string(p.Status)})
				}
				return out, nil
			},
		},
		"create_project": {
			name:        "create_project",
			description: "Draft a project in the workspace. The human must confirm before it is created.",
			schema:      json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"},"slug":{"type":"string"},"description":{"type":"string"}},"required":["name"]}`),
			readOnly:    false,
			preview: func(input json.RawMessage) string {
				var args struct{ Name string }
				_ = json.Unmarshal(input, &args)
				return fmt.Sprintf("I will create the project %q. Confirm to create it.", args.Name)
			},
			run: func(ctx context.Context, orgID string, input json.RawMessage) (any, error) {
				var args struct{ Name, Slug, Description string }
				if err := json.Unmarshal(input, &args); err != nil {
					return nil, err
				}
				project, err := s.platform.CreateProject(ctx, projects.CreateInput{OrgID: orgID, Name: args.Name, Slug: args.Slug, Description: args.Description})
				if err != nil {
					return nil, err
				}
				return map[string]string{"id": project.ID, "name": project.Name, "slug": project.Slug}, nil
			},
		},
		"list_tasks": {
			name:        "list_tasks",
			description: "List the tasks of a project (ref, title, status, priority, acceptance criteria).",
			schema:      json.RawMessage(`{"type":"object","properties":{"project_id":{"type":"string"}},"required":["project_id"]}`),
			readOnly:    true,
			run: func(ctx context.Context, orgID string, input json.RawMessage) (any, error) {
				var args struct {
					ProjectID string `json:"project_id"`
				}
				if err := json.Unmarshal(input, &args); err != nil {
					return nil, err
				}
				tasks, err := s.platform.ListTasks(ctx, orgID, args.ProjectID)
				if err != nil {
					return nil, err
				}
				out := make([]map[string]any, 0, len(tasks))
				for _, t := range tasks {
					out = append(out, map[string]any{"ref": t.Ref, "title": t.Title, "status": string(t.Status), "priority": string(t.Priority), "acceptance": t.Acceptance})
				}
				return out, nil
			},
		},
		"get_run": {
			name:        "get_run",
			description: "Read one run: the status, the agent type, the failure and the recent events.",
			schema:      json.RawMessage(`{"type":"object","properties":{"run_id":{"type":"string"}},"required":["run_id"]}`),
			readOnly:    true,
			run: func(ctx context.Context, orgID string, input json.RawMessage) (any, error) {
				var args struct {
					RunID string `json:"run_id"`
				}
				if err := json.Unmarshal(input, &args); err != nil {
					return nil, err
				}
				run, err := s.platform.GetRun(ctx, orgID, args.RunID)
				if err != nil {
					return nil, err
				}
				return map[string]any{
					"id": run.ID, "project_id": run.ProjectID, "task_id": run.TaskID,
					"agent_type": run.AgentType, "status": string(run.Status), "phase": string(run.CurrentPhase),
					"plan": truncate(string(run.Plan), 4000), "review": truncate(string(run.Review), 4000),
				}, nil
			},
		},
		"list_runs": {
			name:        "list_runs",
			description: "List the recent runs in the workspace (id, project, agent type, status).",
			schema:      json.RawMessage(`{"type":"object","properties":{}}`),
			readOnly:    true,
			run: func(ctx context.Context, orgID string, _ json.RawMessage) (any, error) {
				runs, err := s.platform.ListRuns(ctx, orgID)
				if err != nil {
					return nil, err
				}
				out := make([]map[string]string, 0, len(runs))
				for i, r := range runs {
					if i >= 20 {
						break
					}
					out = append(out, map[string]string{"id": r.ID, "project_id": r.ProjectID, "agent_type": r.AgentType, "status": string(r.Status)})
				}
				return out, nil
			},
		},
		"create_task": {
			name:        "create_task",
			description: "Draft a task in a project. The human must confirm before it is created.",
			schema:      json.RawMessage(`{"type":"object","properties":{"project_id":{"type":"string"},"title":{"type":"string"},"description":{"type":"string"},"acceptance":{"type":"array","items":{"type":"object","properties":{"given":{"type":"string"},"when":{"type":"string"},"then":{"type":"string"}}}}},"required":["project_id","title"]}`),
			readOnly:    false,
			preview: func(input json.RawMessage) string {
				var args struct{ ProjectID, Title, Description string }
				_ = json.Unmarshal(input, &args)
				return fmt.Sprintf("I will create the task %q in project %s. Confirm to create it.", args.Title, args.ProjectID)
			},
			run: func(ctx context.Context, orgID string, input json.RawMessage) (any, error) {
				var args struct {
					ProjectID   string                               `json:"project_id"`
					Title       string                               `json:"title"`
					Description string                               `json:"description"`
					Acceptance  []struct{ Given, When, Then string } `json:"acceptance"`
				}
				if err := json.Unmarshal(input, &args); err != nil {
					return nil, err
				}
				acceptance := make([]domain.AcceptanceCriterion, 0, len(args.Acceptance))
				for _, ac := range args.Acceptance {
					acceptance = append(acceptance, domain.AcceptanceCriterion{Given: ac.Given, When: ac.When, Then: ac.Then})
				}
				task, err := s.platform.CreateTask(ctx, tasks.CreateInput{OrgID: orgID, ProjectID: args.ProjectID, Title: args.Title, Description: args.Description, Acceptance: acceptance})
				if err != nil {
					return nil, err
				}
				return map[string]string{"ref": task.Ref, "title": task.Title, "status": string(task.Status)}, nil
			},
		},
		"retry_run": {
			name:        "retry_run",
			description: "Re-queue a failed run. The human must confirm.",
			schema:      json.RawMessage(`{"type":"object","properties":{"run_id":{"type":"string"}},"required":["run_id"]}`),
			readOnly:    false,
			preview:     func(input json.RawMessage) string { return "I will retry the failed run. Confirm to retry." },
			run: func(ctx context.Context, orgID string, input json.RawMessage) (any, error) {
				var args struct {
					RunID string `json:"run_id"`
				}
				if err := json.Unmarshal(input, &args); err != nil {
					return nil, err
				}
				next, err := s.platform.RetryRun(ctx, runs.RetryInput{OrgID: orgID, RunID: args.RunID})
				if err != nil {
					return nil, err
				}
				return map[string]string{"id": next.ID, "status": string(next.Status)}, nil
			},
		},
		"cancel_run": {
			name:        "cancel_run",
			description: "Cancel a running run. The human must confirm.",
			schema:      json.RawMessage(`{"type":"object","properties":{"run_id":{"type":"string"}},"required":["run_id"]}`),
			readOnly:    false,
			preview:     func(input json.RawMessage) string { return "I will cancel the run. Confirm to cancel." },
			run: func(ctx context.Context, orgID string, input json.RawMessage) (any, error) {
				var args struct {
					RunID string `json:"run_id"`
				}
				if err := json.Unmarshal(input, &args); err != nil {
					return nil, err
				}
				run, err := s.platform.CancelRun(ctx, orgID, args.RunID)
				if err != nil {
					return nil, err
				}
				return map[string]string{"id": run.ID, "status": string(run.Status)}, nil
			},
		},
	}
}

// Limiter is a small per-user sliding window.
type Limiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	hits   map[string][]time.Time
}

// NewLimiter allows limit requests per window per user.
func NewLimiter(limit int, window time.Duration) *Limiter {
	return &Limiter{limit: limit, window: window, hits: map[string][]time.Time{}}
}

// Allow reports whether the user may send one more request.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := time.Now().Add(-l.window)
	kept := l.hits[key][:0]
	for _, hit := range l.hits[key] {
		if hit.After(cutoff) {
			kept = append(kept, hit)
		}
	}
	if len(kept) >= l.limit {
		l.hits[key] = kept
		return false
	}
	l.hits[key] = append(kept, time.Now())
	return true
}
