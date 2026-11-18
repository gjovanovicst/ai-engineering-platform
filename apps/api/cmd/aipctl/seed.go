package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
)

const (
	seedEmail           = "demo@acme.io"
	seedOrgSlug         = "acme"
	seedOrgName         = "Acme"
	seedProjectSlug     = "demo"
	seedDefaultPassword = "correct horse battery"
)

// seedSummary is what the seed reports; the password is never part of it.
type seedSummary struct {
	OrgID     string
	OrgName   string
	UserID    string
	UserEmail string
	ProjectID string
	Tasks     int
}

// demoTask is one seeded task.
type demoTask struct {
	title       string
	description string
	acceptance  []domain.AcceptanceCriterion
}

func demoTasks() []demoTask {
	return []demoTask{
		{
			title:       "Add a health endpoint",
			description: "Expose GET /health so a deploy can check liveness without touching the database.",
			acceptance: []domain.AcceptanceCriterion{{
				Given: "the API is running", When: "GET /health is called", Then: `it returns 200 with {"status":"ok"}`,
			}},
		},
		{
			title:       "Log requests as structured JSON",
			description: "Every HTTP request emits one slog line with method, path, status and duration.",
			acceptance: []domain.AcceptanceCriterion{{
				Given: "a request reaches the API", When: "the handler finishes", Then: "the log line carries method, path, status and duration_ms",
			}},
		},
		{
			title:       "Document the quickstart in the README",
			description: "The README shows the commands to run the stack locally and links the quickstart.",
			acceptance: []domain.AcceptanceCriterion{{
				Given: "a new developer", When: "they open the README", Then: "the quickstart commands and the documentation link are visible",
			}},
		},
	}
}

func runSeed() {
	dsn := os.Getenv("AIP_DB_DSN")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "AIP_DB_DSN is required (the application DSN)")
		os.Exit(2)
	}
	password := os.Getenv("AIP_SEED_PASSWORD")
	if password == "" {
		password = seedDefaultPassword
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "aipctl seed: connect: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	summary, err := seedDemoData(ctx, pool, password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "aipctl seed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("aipctl seed: demo data ready\n")
	fmt.Printf("  org     %s (%s) %s\n", summary.OrgName, seedOrgSlug, summary.OrgID)
	fmt.Printf("  owner   %s %s\n", summary.UserEmail, summary.UserID)
	fmt.Printf("  project %s %s\n", seedProjectSlug, summary.ProjectID)
	fmt.Printf("  tasks   %d ready\n", summary.Tasks)
	fmt.Printf("  login   %s / password from AIP_SEED_PASSWORD (local default in docs/quickstart.md)\n", summary.UserEmail)
}

// seedDemoData creates the demo org, owner, project and tasks. It is
// idempotent: a second run returns the existing rows.
func seedDemoData(ctx context.Context, pool *pgxpool.Pool, password string) (seedSummary, error) {
	accounts := postgres.NewAccountStore(pool)
	projects := postgres.NewProjectStore(pool)
	tasks := postgres.NewTaskStore(pool)

	summary := seedSummary{OrgName: seedOrgName, UserEmail: seedEmail}
	user, _, err := accounts.UserByEmail(ctx, seedEmail)
	switch {
	case err == nil:
		orgID, _, merr := accounts.PrimaryMembership(ctx, user.ID)
		if merr != nil {
			return summary, fmt.Errorf("find the demo org: %w", merr)
		}
		summary.OrgID = orgID
	case errors.Is(err, domain.ErrNotFound):
		hash, herr := auth.HashPassword(password, auth.DefaultParams())
		if herr != nil {
			return summary, fmt.Errorf("hash the demo password: %w", herr)
		}
		createdUser, createdOrg, cerr := accounts.CreateAccount(ctx, auth.CreateAccountParams{
			Email: seedEmail, Name: "Demo", PasswordHash: hash,
			OrgName: seedOrgName, OrgSlug: seedOrgSlug, Verified: true,
		})
		if cerr != nil {
			return summary, fmt.Errorf("create the demo account: %w", cerr)
		}
		user, summary.OrgID = createdUser, createdOrg.ID
	default:
		return summary, fmt.Errorf("look up the demo user: %w", err)
	}
	summary.UserID = user.ID

	project, err := findOrCreateDemoProject(ctx, projects, summary.OrgID)
	if err != nil {
		return summary, err
	}
	summary.ProjectID = project.ID

	existing, err := tasks.List(ctx, summary.OrgID, project.ID, "", "", 100)
	if err != nil {
		return summary, fmt.Errorf("list the demo tasks: %w", err)
	}
	if len(existing) > 0 {
		summary.Tasks = len(existing)
		return summary, nil
	}
	for _, spec := range demoTasks() {
		ref, rerr := tasks.NextTaskRef(ctx, summary.OrgID, project.ID)
		if rerr != nil {
			return summary, fmt.Errorf("allocate a task ref: %w", rerr)
		}
		created, cerr := tasks.Create(ctx, domain.Task{
			OrgID: summary.OrgID, ProjectID: project.ID,
			Ref: fmt.Sprintf("T-%d", ref), Title: spec.title, Description: spec.description,
			Acceptance: spec.acceptance, Priority: domain.PriorityMed, CreatedBy: user.ID,
		})
		if cerr != nil {
			return summary, fmt.Errorf("create the demo task %q: %w", spec.title, cerr)
		}
		if terr := created.Transition(domain.TaskReady, domain.TransitionOptions{Actor: domain.ActorHuman}); terr != nil {
			return summary, fmt.Errorf("move the demo task %q to ready: %w", spec.title, terr)
		}
		if _, uerr := tasks.UpdateStatus(ctx, created); uerr != nil {
			return summary, fmt.Errorf("save the demo task %q: %w", spec.title, uerr)
		}
		summary.Tasks++
	}
	return summary, nil
}

func findOrCreateDemoProject(ctx context.Context, projects *postgres.ProjectStore, orgID string) (domain.Project, error) {
	list, err := projects.List(ctx, orgID, 100)
	if err != nil {
		return domain.Project{}, fmt.Errorf("list projects: %w", err)
	}
	for _, p := range list {
		if p.Slug == seedProjectSlug {
			return p, nil
		}
	}
	created, err := projects.Create(ctx, domain.Project{
		OrgID: orgID, Name: "Demo", Slug: seedProjectSlug,
		Description: "The seeded demo project: three ready tasks.",
	})
	if err != nil {
		return domain.Project{}, fmt.Errorf("create the demo project: %w", err)
	}
	return created, nil
}
