package runs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/riverqueue/river"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/attachments"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/design"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/designstandards"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/projects"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/rules"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/visualdiff"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/queue"
)

// RulesProvider supplies the published rules for a project.
type RulesProvider interface {
	PromptText(ctx context.Context, orgID, projectID string) (string, []rules.Ref, error)
}

// JobWorker executes agent_run jobs from River. The job is idempotent: a run
// that already finished (or no longer exists) is a no-op.
type JobWorker struct {
	river.WorkerDefaults[queue.RunArgs]
	store           Store
	tasks           TaskReader
	rules           RulesProvider
	runner          *Runner
	log             *slog.Logger
	projects        ProjectStandardReader
	designStandards DesignStandardReader
	components      ProjectComponentsReader
	forbidden       ForbiddenComponentsReader
	attachments     AttachmentReader
	orgGate         OrgGate
	orgSlots        int
	enqueuer        Enqueuer
}

// WithEnqueuer lets the worker re-queue a run after a deterministic remediation.
func (w *JobWorker) WithEnqueuer(enqueuer Enqueuer) *JobWorker {
	w.enqueuer = enqueuer
	return w
}

// OrgGate serializes/limits concurrent work per organisation.
type OrgGate interface {
	Acquire(ctx context.Context, orgID string, slots int) (release func(), ok bool, err error)
}

// AttachmentReader lists task attachments for the context manifest.
type AttachmentReader interface {
	List(ctx context.Context, orgID, taskID string) ([]attachments.Meta, error)
}

// ProjectComponentsReader lists the components a project allows.
type ProjectComponentsReader interface {
	ListComponents(ctx context.Context, orgID, projectID string) ([]projects.ProjectComponent, error)
}

// ForbiddenComponentsReader lists deprecated/forbidden import paths.
type ForbiddenComponentsReader interface {
	ListForbiddenComponents(ctx context.Context, orgID string) ([]design.ForbiddenComponent, error)
}

// ProjectStandardReader resolves the standard assigned to a project.
type ProjectStandardReader interface {
	DesignStandard(ctx context.Context, orgID, projectID string) (projects.ProjectStandard, error)
	E2ECommand(ctx context.Context, orgID, projectID string) (string, error)
	VisualCommand(ctx context.Context, orgID, projectID string) (string, error)
	TestCommand(ctx context.Context, orgID, projectID string) (string, error)
	MCPConfig(ctx context.Context, orgID, projectID string) (string, error)
	VisualPolicy(ctx context.Context, orgID, projectID string) (float64, string, error)
	Policy(ctx context.Context, orgID, projectID string) (int, []string, error)
	AllowedModels(ctx context.Context, orgID, projectID string) ([]string, error)
	AllowedTools(ctx context.Context, orgID, projectID string) ([]string, error)
	PluginChecks(ctx context.Context, orgID, projectID string) (string, error)
}

// DesignStandardReader loads a full design standard.
type DesignStandardReader interface {
	Get(ctx context.Context, orgID, id string) (designstandards.Standard, error)
}

// WithDesignStandard enables design-standard context injection.
func (w *JobWorker) WithDesignStandard(projects ProjectStandardReader, standards DesignStandardReader) *JobWorker {
	w.projects, w.designStandards = projects, standards
	return w
}

// WithComponents enables the project allowlist and forbidden-import checks.
func (w *JobWorker) WithComponents(components ProjectComponentsReader, forbidden ForbiddenComponentsReader) *JobWorker {
	w.components, w.forbidden = components, forbidden
	return w
}

// WithAttachments enables task attachment metadata in the prompt and manifest.
func (w *JobWorker) WithAttachments(reader AttachmentReader) *JobWorker {
	w.attachments = reader
	return w
}

// NewJobWorker wires the River worker.
func NewJobWorker(store Store, tasks TaskReader, ruleSet RulesProvider, runner *Runner, log *slog.Logger) *JobWorker {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &JobWorker{store: store, tasks: tasks, rules: ruleSet, runner: runner, log: log}
}

// Timeout raises River's default one minute; an agent run legitimately takes
// several minutes (model steps, tools, tests).
func (w *JobWorker) Timeout(*river.Job[queue.RunArgs]) time.Duration {
	return 30 * time.Minute
}

// Work implements river.Worker.
// WithOrgGate limits concurrent runs per organisation.
func (w *JobWorker) WithOrgGate(gate OrgGate, slots int) *JobWorker {
	w.orgGate = gate
	w.orgSlots = slots
	return w
}

func (w *JobWorker) Work(ctx context.Context, job *river.Job[queue.RunArgs]) error {
	run, err := w.store.Get(ctx, job.Args.OrgID, job.Args.RunID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			w.log.Warn("runs: job for a missing run", slog.String("run_id", job.Args.RunID))
			return nil
		}
		return err
	}
	if run.Status.Terminal() {
		return nil
	}
	if w.orgGate != nil {
		release, ok, gerr := w.orgGate.Acquire(ctx, run.OrgID, w.orgSlots)
		if gerr != nil {
			return gerr
		}
		if !ok {
			return river.JobSnooze(10 * time.Second)
		}
		defer release()
	}

	task, err := w.tasks.Get(ctx, run.OrgID, run.ProjectID, run.TaskID)
	if err != nil {
		return err
	}
	prompt := BuildPrompt(task)
	switch run.AgentType {
	case string(domain.AgentPlanning):
		prompt += "\nYou are the PLANNING agent. Do NOT modify files (write_file is not available).\n" +
			"Investigate with read-only tools if needed, then finish with exactly one line:\n" +
			"AIP_PLAN: {\"steps\":[\"...\"],\"files\":[\"...\"],\"risks\":[\"...\"]}\n"
	case string(domain.AgentReview):
		prompt += reviewPrompt(w.developmentDiff(ctx, run)) +
			"Finish with exactly one line:\n" +
			"AIP_REVIEW: {\"findings\":[{\"severity\":\"major|minor|nit\",\"body\":\"...\"}]}\n"
	case string(domain.AgentDevelopment):
		prompt += "\nYou are the DEVELOPMENT agent. Implement the task inside the repository with " +
			"write_file and verify it with run_tests. Execute the whole approved plan; do not stop " +
			"after one file or one step. If a Go module is missing, call fetch_module with its path " +
			"and version (for example golang.org/x/crypto v0.36.0): the worker host downloads it " +
			"into the offline module cache, then run `go mod tidy` and retry. Never reimplement a " +
			"library or write cryptography yourself; if fetch_module fails, stop and report it in " +
			"your final message. When everything is implemented and the tests pass, finish " +
			"with the AIP_PROOF line described below.\n"
	}
	criteria := make([]string, 0, len(task.Acceptance))
	if run.AgentType == string(domain.AgentDevelopment) && len(task.Acceptance) > 0 {
		prompt += "\n\nACCEPTANCE CRITERIA (every one needs evidence):\n"
		for i, ac := range task.Acceptance {
			id := fmt.Sprintf("AC%d", i+1)
			criteria = append(criteria, id)
			prompt += fmt.Sprintf("- %s: Given %s When %s Then %s\n", id, ac.Given, ac.When, ac.Then)
		}
		prompt += "\nFinish with exactly one line mapping every criterion:\n" +
			"AIP_PROOF: {\"evidence\":[{\"criterion\":\"AC1\",\"kind\":\"automated\",\"test\":\"TestName\",\"files\":[\"path_test.go\"]}]}\n" +
			"Use kind \"manual\" with a reason only when automation is not practical. " +
			"Do not weaken tests to make them pass.\n"
	}
	sources := []ContextSource{{
		Kind: "task", ID: run.TaskID, Version: task.Ref,
		SHA256:     hashBytes([]byte(task.Title + "\n" + task.Description + "\n" + fmt.Sprint(task.Acceptance))),
		Visibility: "org",
	}}
	var contextRules []string
	if w.rules != nil {
		rulesText, refs, rerr := w.rules.PromptText(ctx, run.OrgID, run.ProjectID)
		if rerr != nil {
			w.log.Warn("runs: load rules", slog.Any("err", rerr))
		} else if rulesText != "" {
			prompt += "\n\n" + rulesText
			for _, ref := range refs {
				contextRules = append(contextRules, fmt.Sprintf("%s@%d", ref.RuleID, ref.Version))
				sources = append(sources, ContextSource{Kind: "rule", ID: ref.RuleID, Version: fmt.Sprintf("v%d", ref.Version), Visibility: "org"})
			}
			_ = w.store.AppendEvent(ctx, run.OrgID, run.ID, "rules.snapshot", "system",
				map[string]any{"rules": refs})
		}
	}
	if len(run.Plan) > 0 {
		prompt += "\nApproved plan:\n" + string(run.Plan) + "\n"
	}
	// A fix run keeps the approved plan and fixes the review findings (or the
	// failed CI check log) on the existing pull request; the runner clones its
	// branch.
	if run.FixOfRunID != "" {
		if run.FixKind == "troubleshoot" && len(run.FixContext) > 0 {
			prompt += "\nThe run failed. Diagnose the root cause and fix it on the existing branch; do not re-plan and do not rewrite unrelated code. The failure:\n" + string(run.FixContext) + "\n"
		} else if run.FixKind == "ci" && len(run.FixContext) > 0 {
			prompt += "\nThe CI check failed on the existing pull request. Fix the build; " +
				"do not re-plan and do not rewrite unrelated code. The check log:\n" + string(run.FixContext) + "\n"
		} else if run.FixKind == "review" && len(run.FixContext) > 0 {
			prompt += "\nThe human selected these review findings to fix on the existing pull request; " +
				"do not re-plan and do not rewrite unrelated code:\n" + string(run.FixContext) + "\n"
		} else if findings, ferr := w.store.LatestReviewForTask(ctx, run.OrgID, run.TaskID); ferr == nil && len(findings) > 0 {
			prompt += "\nThe review found these issues. Fix every blocker on the existing pull request; " +
				"do not re-plan and do not rewrite unrelated code:\n" + string(findings) + "\n"
		}
	}

	designRef := ""
	if w.projects != nil && w.designStandards != nil {
		if standard, serr := w.projects.DesignStandard(ctx, run.OrgID, run.ProjectID); serr == nil {
			if full, derr := w.designStandards.Get(ctx, run.OrgID, standard.ID); derr == nil {
				prompt += "\n\n" + DesignPrompt(standard.Name, standard.Version, standard.SHA256, full.Spec)
				designRef = fmt.Sprintf("%s@%d", standard.ID, standard.Version)
				sources = append(sources, ContextSource{Kind: "design_standard", ID: standard.ID, Version: fmt.Sprintf("v%d", standard.Version), SHA256: standard.SHA256, Visibility: "org"})
				_ = w.store.AppendEvent(ctx, run.OrgID, run.ID, "design.snapshot", "system",
					map[string]any{"id": standard.ID, "name": standard.Name, "version": standard.Version, "sha256": standard.SHA256})
			}
		}
	}

	forbiddenImports := make([]string, 0)
	if w.components != nil {
		if items, cerr := w.components.ListComponents(ctx, run.OrgID, run.ProjectID); cerr == nil && len(items) > 0 {
			prompt += "\nAPPROVED COMPONENTS (project allowlist):\n"
			componentText := ""
			for _, item := range items {
				prompt += fmt.Sprintf("- %s (%s %s)\n", item.Name, item.Package, item.ImportPath)
				componentText += item.Name + "|" + item.Package + "|" + item.ImportPath + "\n"
			}
			sources = append(sources, ContextSource{Kind: "component_registry", ID: run.ProjectID, Version: fmt.Sprintf("%d components", len(items)), SHA256: hashBytes([]byte(componentText)), Visibility: "org"})
		}
	}
	if w.forbidden != nil {
		if items, ferr := w.forbidden.ListForbiddenComponents(ctx, run.OrgID); ferr == nil {
			for _, item := range items {
				forbiddenImports = append(forbiddenImports, item.ImportPath)
			}
		}
	}

	if w.attachments != nil {
		if items, aerr := w.attachments.List(ctx, run.OrgID, run.TaskID); aerr == nil && len(items) > 0 {
			attachmentText := ""
			prompt += "\nATTACHMENTS (metadata only; a reviewer can open them in the task):\n"
			for _, item := range items {
				prompt += fmt.Sprintf("- %s (%s, %d bytes)\n", item.Filename, item.ContentType, item.SizeBytes)
				attachmentText += item.Filename + "|" + item.SHA256 + "\n"
			}
			sources = append(sources, ContextSource{Kind: "attachment", ID: run.TaskID, Version: fmt.Sprintf("%d files", len(items)), SHA256: hashBytes([]byte(attachmentText)), Visibility: "org"})
		}
	}

	e2eCommand := ""
	testCommand := ""
	visualCommand := ""
	mcpConfig := ""
	visualThreshold := 0.01
	var visualMasks []visualdiff.Rect
	policyMaxDiff := 0
	var forbiddenPaths []string
	var allowedModels []string
	var allowedTools []string
	var pluginChecks []PluginCheck
	if w.projects != nil {
		if cmd, err := w.projects.E2ECommand(ctx, run.OrgID, run.ProjectID); err == nil {
			e2eCommand = cmd
		}
		if cmd, err := w.projects.TestCommand(ctx, run.OrgID, run.ProjectID); err == nil {
			testCommand = cmd
		}
		if cmd, err := w.projects.VisualCommand(ctx, run.OrgID, run.ProjectID); err == nil {
			visualCommand = cmd
		}
		if cfg, err := w.projects.MCPConfig(ctx, run.OrgID, run.ProjectID); err == nil {
			mcpConfig = cfg
		}
		if threshold, masks, err := w.projects.VisualPolicy(ctx, run.OrgID, run.ProjectID); err == nil {
			visualThreshold = threshold
			_ = json.Unmarshal([]byte(masks), &visualMasks)
		}
		if maxBytes, paths, err := w.projects.Policy(ctx, run.OrgID, run.ProjectID); err == nil {
			policyMaxDiff = maxBytes
			forbiddenPaths = paths
		}
		if models, err := w.projects.AllowedModels(ctx, run.OrgID, run.ProjectID); err == nil {
			allowedModels = models
		}
		if tools, err := w.projects.AllowedTools(ctx, run.OrgID, run.ProjectID); err == nil {
			allowedTools = tools
		}
		if raw, err := w.projects.PluginChecks(ctx, run.OrgID, run.ProjectID); err == nil {
			pluginChecks, _ = ParsePluginChecks(raw)
		}
	}
	final, err := w.runner.Execute(ctx, run, prompt, contextRules, designRef, forbiddenImports, criteria, sources, e2eCommand, visualCommand, mcpConfig, testCommand, visualThreshold, visualMasks, policyMaxDiff, forbiddenPaths, allowedModels, allowedTools, pluginChecks)
	if err != nil {
		w.log.Error("runs: execute", slog.Any("err", err))
		return nil
	}
	// A failed run must not leave the task in planning, in progress or review:
	// park it in blocked (one transition away from ready) with the reason, so
	// the board is truthful and a retry is one click away.
	if final.Status == domain.RunFailed {
		w.blockTaskAfterFailure(ctx, final)
		w.maybeRemediate(ctx, final)
	}

	// ASDLC §2: a successful development run moves the task to review.
	if final.AgentType == string(domain.AgentDevelopment) && final.Status == domain.RunSucceeded {
		if task, terr := w.tasks.Get(ctx, run.OrgID, run.ProjectID, run.TaskID); terr == nil {
			if terr := task.Transition(domain.TaskReview, domain.TransitionOptions{
				Actor: domain.ActorSystem, TestsGreen: true,
			}); terr == nil {
				_, _ = w.tasks.UpdateStatus(ctx, task)
			}
		}
	}
	return nil
}

// blockTaskAfterFailure parks the run's task in blocked with the failure code.
// A terminal task (done, canceled) or one that cannot transition is skipped.
func (w *JobWorker) blockTaskAfterFailure(ctx context.Context, run domain.Run) {
	task, err := w.tasks.Get(ctx, run.OrgID, run.ProjectID, run.TaskID)
	if err != nil || task.Status.Terminal() || task.Status == domain.TaskBlocked {
		return
	}
	reason := "run failed"
	if run.FailureCode != "" {
		reason += ": " + run.FailureCode
	}
	if run.FailureDetail != "" {
		reason += " — " + truncate(run.FailureDetail, 300)
	}
	if terr := task.Transition(domain.TaskBlocked, domain.TransitionOptions{
		Actor: domain.ActorSystem, Reason: reason,
	}); terr != nil {
		return
	}
	if _, uerr := w.tasks.UpdateStatus(ctx, task); uerr != nil {
		w.log.Warn("runs: block task after failure", slog.Any("err", uerr))
		return
	}
	_ = w.store.AppendEvent(ctx, run.OrgID, run.ID, "task.blocked", "system", map[string]any{"reason": reason})
}

// maybeRemediate retries a failed run once when the failure is an environment
// anomaly with a mechanical fix. A second failure is the human's call: the run
// carries FixKind "auto", so it is never remediated again.
func (w *JobWorker) maybeRemediate(ctx context.Context, failed domain.Run) {
	if w.enqueuer == nil || w.store == nil || w.tasks == nil {
		return
	}
	if failed.FixKind == "auto" || len(failed.Plan) == 0 {
		return
	}
	action, ok := Remediable(failed.FailureCode)
	if !ok {
		return
	}
	task, err := w.tasks.Get(ctx, failed.OrgID, failed.ProjectID, failed.TaskID)
	if err != nil {
		return
	}
	if task.Status.Terminal() {
		return
	}
	if err := retryTask(ctx, &task); err != nil {
		return
	}
	created, err := w.store.Create(ctx, domain.Run{
		OrgID: failed.OrgID, ProjectID: failed.ProjectID, TaskID: failed.TaskID,
		AgentType: string(domain.AgentDevelopment), Status: domain.RunQueued,
		Model: failed.Model, Budget: failed.Budget, FixOfRunID: failed.ID, FixKind: "auto",
	})
	if err != nil {
		return // e.g. a run is already active
	}
	if _, serr := w.tasks.UpdateStatus(ctx, task); serr != nil {
		w.log.Warn("runs: remediation task status", slog.Any("err", serr))
	}
	if err := w.store.SetPlan(ctx, failed.OrgID, created.ID, failed.Plan, ""); err != nil {
		w.log.Warn("runs: remediation plan", slog.Any("err", err))
	}
	_ = w.store.AppendEvent(ctx, created.OrgID, created.ID, "run.auto_remediated", "system",
		map[string]any{"from_run": failed.ID, "action": action, "failure": failed.FailureCode})
	if err := w.enqueuer.Enqueue(ctx, created.ID, created.OrgID); err != nil {
		w.log.Warn("runs: remediation enqueue", slog.Any("err", err))
	}
}
