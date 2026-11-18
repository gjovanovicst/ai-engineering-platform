package runs

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"

	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"sort"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/gjovanovicst/ai-engineering-platform/internal/agent"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/acceptance"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/designcheck"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/monorepo"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/runartifacts"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/secretscan"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/visualdiff"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// ContextSource is one versioned input the agent received.
type ContextSource struct {
	Kind       string `json:"kind"`
	ID         string `json:"id"`
	Version    string `json:"version"`
	SHA256     string `json:"sha256,omitempty"`
	Visibility string `json:"visibility"`
}

// contextPrecedence is the conflict-resolution order (highest first).
var contextPrecedence = []string{"org_policy", "project_standards", "design_standard", "task_references", "repository", "model_defaults"}

// Runner executes a run through an agent backend and persists every step.
type Runner struct {
	backend           agent.Backend
	backendFor        func(ctx context.Context, orgID string) (agent.Backend, error)
	projectBackendFor func(ctx context.Context, orgID, projectID, agentType string) (agent.Backend, error)
	usage             UsageRecorder
	tests             TestRecorder
	artifacts         *runartifacts.Sink
	notifier          Notifier
	workspace         WorkspaceProvider
	priceIn           float64
	priceOut          float64
	store             Store
	enqueuer          Enqueuer
	publisher         Publisher
	proofVerifier     ProofVerifier
	headRunner        HeadRunner
	testCommand       string
	visuals           VisualBaselineReader
	embedder          Embedder
	log               *slog.Logger
}

// WorkspaceProvider clones the project repository into an archive that the
// sandbox extracts before the agent runs (implemented in adapters/git).
type WorkspaceProvider interface {
	Clone(ctx context.Context, orgID, projectID string) ([]byte, error)
}

// TestRecorder stores the test phase result (implemented in app/checks).
type TestRecorder interface {
	RecordRunTest(ctx context.Context, orgID, runID, name, status, output string) error
}

// UsageRecorder records token usage (implemented in app/usage).
type UsageRecorder interface {
	Record(ctx context.Context, run domain.Run, result agent.Result) error
}

// Publisher pushes the branch and opens a PR/MR (implemented in adapters/git).
type Publisher interface {
	Publish(ctx context.Context, run domain.Run, workspace []byte) (PullRequest, error)
}

// HeadRunner runs a command in the head workspace (e2e tests).
type HeadRunner interface {
	RunHead(ctx context.Context, archive []byte, command string) (exitCode int, output string, err error)
}

// ProofVerifier runs the acceptance tests against the base commit (implemented
// by the sandbox in the worker).
type ProofVerifier interface {
	VerifyBase(ctx context.Context, archive []byte, files []string, command string) (exitCode int, output string, err error)
}

// PullRequest is the PR/MR created for a run.
type PullRequest struct {
	ID     string
	Number int
	URL    string
}

// NewRunner wires the executor.
func NewRunner(backend agent.Backend, store Store, enqueuer Enqueuer, publisher Publisher, testCommand string, log *slog.Logger) *Runner {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Runner{backend: backend, store: store, enqueuer: enqueuer, publisher: publisher, testCommand: testCommand, log: log}
}

// WithBackendSelector chooses the backend per run (provider vs command).
func (r *Runner) WithBackendSelector(selector func(ctx context.Context, orgID string) (agent.Backend, error)) *Runner {
	r.backendFor = selector
	return r
}

// WithProjectBackendSelector prefers a project-selected custom agent.
func (r *Runner) WithProjectBackendSelector(selector func(ctx context.Context, orgID, projectID, agentType string) (agent.Backend, error)) *Runner {
	r.projectBackendFor = selector
	return r
}

// WithPrices sets the USD price per 1K tokens used for budget checks.
func (r *Runner) WithPrices(inputPer1K, outputPer1K float64) *Runner {
	r.priceIn, r.priceOut = inputPer1K, outputPer1K
	return r
}

// WithWorkspaceProvider attaches repository cloning.
func (r *Runner) WithWorkspaceProvider(provider WorkspaceProvider) *Runner {
	r.workspace = provider
	return r
}

// Notifier sends in-app notifications to the org's admins.
type Notifier interface {
	NotifyAdmins(ctx context.Context, orgID, kind, title, body, link string) error
}

// WithNotifier attaches the notification fan-out.
func (r *Runner) WithNotifier(notifier Notifier) *Runner {
	r.notifier = notifier
	return r
}

// WithArtifacts attaches the object-storage sink.
func (r *Runner) WithArtifacts(sink *runartifacts.Sink) *Runner {
	r.artifacts = sink
	return r
}

// WithHeadRunner attaches the head-workspace command runner (e2e).
// WithEmbedder enables semantic repository ranking (keyword map remains the
// fallback when the embedder is missing or fails).
func (r *Runner) WithEmbedder(embedder Embedder) *Runner {
	r.embedder = embedder
	return r
}

// VisualBaselineReader loads a stored screenshot baseline.
type VisualBaselineReader interface {
	Get(ctx context.Context, key string) ([]byte, error)
}

// WithVisualBaselines enables the built-in screenshot gate for visual commands.
func (r *Runner) WithVisualBaselines(reader VisualBaselineReader) *Runner {
	r.visuals = reader
	return r
}

func (r *Runner) WithHeadRunner(runner HeadRunner) *Runner {
	r.headRunner = runner
	return r
}

// WithProofVerifier attaches the base-SHA negative guard.
func (r *Runner) WithProofVerifier(verifier ProofVerifier) *Runner {
	r.proofVerifier = verifier
	return r
}

// WithTests attaches the test-result recorder.
func (r *Runner) WithTests(recorder TestRecorder) *Runner {
	r.tests = recorder
	return r
}

// WithUsageRecorder attaches usage accounting.
func (r *Runner) WithUsageRecorder(recorder UsageRecorder) *Runner {
	r.usage = recorder
	return r
}

var _ Executor = (*Runner)(nil)

// Execute marks the run running, streams backend events and records the final
// state. A canceled context leaves the status to the service (which already
// persisted the cancellation).
// watchCancellation cancels the run context when the API marks the run canceled.
func (r *Runner) watchCancellation(ctx context.Context, cancel context.CancelFunc, orgID, runID string) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			current, err := r.store.Get(context.WithoutCancel(ctx), orgID, runID)
			if err != nil {
				continue
			}
			if current.Status == domain.RunCanceled || current.CancelRequestedAt != nil {
				cancel()
				return
			}
		}
	}
}

// effectiveTestCommand prefers the project's test command and falls back to
// the global AIP_TEST_COMMAND.
func effectiveTestCommand(global, project string) string {
	if strings.TrimSpace(project) != "" {
		return project
	}
	return global
}

func (r *Runner) Execute(ctx context.Context, run domain.Run, prompt string, contextRules []string, designRef string, forbiddenImports []string, criteria []string, sources []ContextSource, e2eCommand, visualCommand, mcpConfig, testCommand string, visualThreshold float64, visualMasks []visualdiff.Rect, policyMaxDiff int, forbiddenPaths []string, allowedModels []string, allowedTools []string, pluginChecks []PluginCheck) (domain.Run, error) {
	ctx, span := otel.Tracer("aip/runs").Start(ctx, "run.execute",
		trace.WithAttributes(
			attribute.String("run.id", run.ID),
			attribute.String("org.id", run.OrgID),
			attribute.String("agent.type", run.AgentType),
		))
	defer span.End()
	if span.SpanContext().IsValid() {
		run.TraceID = span.SpanContext().TraceID().String()
		_ = r.store.SetTraceID(ctx, run.OrgID, run.ID, run.TraceID)
	}

	runCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	// Cancellation is cooperative: cancel() from the API only works in-process,
	// the worker polls the run row instead.
	go r.watchCancellation(runCtx, cancel, run.OrgID, run.ID)

	now := time.Now().UTC()
	if err := run.Transition(domain.RunRunning, now); err != nil {
		return run, err
	}
	if _, err := r.store.UpdateStatus(runCtx, run); err != nil {
		return run, err
	}

	backend := r.backend
	if r.projectBackendFor != nil {
		selected, serr := r.projectBackendFor(runCtx, run.OrgID, run.ProjectID, run.AgentType)
		if serr != nil {
			r.log.Warn("runs: select project backend", slog.Any("err", serr))
		} else if selected != nil {
			backend = selected
		}
	}
	if backend == r.backend && r.backendFor != nil {
		selected, serr := r.backendFor(runCtx, run.OrgID)
		if serr != nil {
			r.log.Warn("runs: select backend", slog.Any("err", serr))
		} else if selected != nil {
			backend = selected
		}
	}

	if len(allowedModels) > 0 {
		model := run.Model
		if model == "" {
			if named, ok := backend.(interface{ ModelName() string }); ok {
				model = named.ModelName()
			}
		}
		allowed := false
		for _, candidate := range allowedModels {
			if candidate == model {
				allowed = true
			}
		}
		if !allowed {
			return r.fail(runCtx, run, "policy_violation", "model "+model+" is not in the project allowlist")
		}
	}

	var workspace []byte
	if r.workspace != nil {
		archive, werr := r.workspace.Clone(runCtx, run.OrgID, run.ProjectID)
		if werr != nil {
			r.log.Error("runs: clone workspace", slog.Any("err", werr))
			failed, ferr := r.fail(runCtx, run, "workspace_clone_failed", "could not clone the project repository")
			if ferr != nil {
				return run, ferr
			}
			return failed, nil
		}
		workspace = archive
	}

	if len(workspace) > 0 {
		files := tarFiles(workspace)
		repoSource := ContextSource{Kind: "repository", ID: run.ProjectID, Version: fmt.Sprintf("%d files", len(files)), SHA256: hashWorkspace(workspace), Visibility: "org"}
		allSources := append(append([]ContextSource{}, sources...), repoSource)
		manifest := buildContextManifest(run, files, contextRules, designRef, allSources)
		if contextMismatch(run.FrozenContextHash, manifest.hash) {
			var frozenSources []ContextSource
			_ = json.Unmarshal([]byte(run.FrozenContextSources), &frozenSources)
			_ = r.store.AppendEvent(runCtx, run.OrgID, run.ID, "context.changed", "system",
				map[string]any{"frozen": run.FrozenContextHash, "current": manifest.hash, "files": len(files),
					"changed_sources": sourceDelta(frozenSources, allSources)})
			return r.fail(runCtx, run, "context_changed", "the context changed after plan approval; request a new approval")
		}
		_ = r.store.SetContextHash(runCtx, run.OrgID, run.ID, manifest.hash)
		if aerr := r.artifacts.Upload(runCtx, run.OrgID, run.ID, "context_manifest", manifest.json); aerr != nil {
			r.log.Warn("runs: upload context manifest", slog.Any("err", aerr))
		}
		_ = r.store.AppendEvent(runCtx, run.OrgID, run.ID, "context.manifest", "system",
			map[string]any{"sha256": manifest.hash, "files": len(files), "rules": len(contextRules), "design": designRef, "sources": allSources, "precedence": contextPrecedence})
	}

	repoMap := RepoMap(workspace, prompt, 20)
	if r.embedder != nil {
		if semantic := SemanticRepoMap(runCtx, workspace, prompt, r.embedder, 20); len(semantic) > 0 {
			repoMap = semantic
		}
	}
	if len(repoMap) > 0 {
		prompt += "\n\n" + RepoMapPrompt(repoMap)
		_ = r.store.AppendEvent(runCtx, run.OrgID, run.ID, "context.repo_map", "system",
			map[string]any{"files": len(repoMap)})
		if symbols := Symbols(workspace, repoMap, 60); len(symbols) > 0 {
			prompt += "\n" + SymbolPrompt(symbols)
			_ = r.store.AppendEvent(runCtx, run.OrgID, run.ID, "context.symbols", "system",
				map[string]any{"files": len(symbols)})
		}
	}

	runAgent := func(agentPrompt string) (agent.Result, []string, error) {
		var agentLines []string
		res, runErr := backend.Run(runCtx, agent.Spec{
			RunID:       run.ID,
			Prompt:      agentPrompt,
			Model:       run.Model,
			Timeout:     30 * time.Minute,
			TestCommand: effectiveTestCommand(r.testCommand, testCommand),
			// Only development runs publish a branch.
			CaptureWorkspace: run.AgentType == string(domain.AgentDevelopment),
			Workspace:        workspace,
			ReadOnly:         run.AgentType != string(domain.AgentDevelopment),
			MCPConfig:        mcpConfig, AllowedTools: allowedTools,
			BudgetTokensIn:   run.Budget.TokensIn,
			BudgetTokensOut:  run.Budget.TokensOut,
			BudgetCostMicros: run.Budget.CostMicros,
			BudgetWallClock:  time.Duration(run.Budget.WallClockS) * time.Second,
			PriceInPer1K:     r.priceIn,
			PriceOutPer1K:    r.priceOut,
		}, func(ev agent.Event) error {
			if ev.Type == agent.EventText {
				agentLines = append(agentLines, ev.Text)
			}
			return r.store.AppendEvent(runCtx, run.OrgID, run.ID, string(ev.Type), "agent",
				map[string]any{"text": ev.Text})
		})
		return res, agentLines, runErr
	}

	result, lines, err := runAgent(prompt)
	if err == nil && canRetry(run, result) {
		reason := ""
		switch {
		case run.AgentType == string(domain.AgentPlanning) && extractPlan(lines) == nil:
			reason = "plan_missing"
		case run.AgentType == string(domain.AgentDevelopment) && len(criteria) > 0:
			if _, proofErr := acceptance.ParseProof(lines); proofErr != nil {
				reason = "proof_missing"
			}
		case run.AgentType == string(domain.AgentDevelopment) && result.TestExitCode != 0:
			switch classifyTestFailure(result.TestOutput) {
			case "infra":
				reason = "infra_test_failure"
			case "flaky":
				reason = "flaky_test_failure"
			}
		}
		if reason != "" {
			_ = r.store.AppendEvent(runCtx, run.OrgID, run.ID, "run.retry", "system",
				map[string]any{"reason": reason, "attempt": 2})
			firstIn, firstOut := result.InputTokens, result.OutputTokens
			retried, retriedLines, retryErr := runAgent(prompt + retryCorrection(reason))
			if retryErr == nil {
				retried.InputTokens += firstIn
				retried.OutputTokens += firstOut
				result, lines, err = retried, retriedLines, retryErr
				if retried.TestExitCode == 0 {
					_ = r.store.AppendEvent(runCtx, run.OrgID, run.ID, "test.flaky_recovered", "system",
						map[string]any{"reason": reason})
				}
			}
		}
	}

	if errors.Is(err, context.Canceled) || runCtx.Err() != nil {
		// The job can be interrupted (shutdown, cancel). Never leave the run in
		// running with no worker: record the terminal state here.
		if run.Status == domain.RunRunning {
			now := time.Now().UTC()
			if terr := run.Transition(domain.RunCanceled, now); terr == nil {
				_, _ = r.store.UpdateStatus(context.WithoutCancel(ctx), run)
				_ = r.store.AppendEvent(context.WithoutCancel(ctx), run.OrgID, run.ID, "run.canceled", "system",
					map[string]any{"reason": "worker stopped"})
			}
		}
		return run, nil
	}

	finished := time.Now().UTC()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "backend error")
		if r.usage != nil && (result.InputTokens > 0 || result.OutputTokens > 0) {
			if uerr := r.usage.Record(context.WithoutCancel(ctx), run, result); uerr != nil {
				r.log.Warn("runs: record usage", slog.Any("err", uerr))
			}
		}
		if errors.Is(err, agent.ErrBudgetExceeded) {
			return r.fail(ctx, run, "budget_exceeded", err.Error())
		}
		return r.fail(ctx, run, "backend_error", err.Error())
	}

	if r.usage != nil {
		if uerr := r.usage.Record(context.WithoutCancel(ctx), run, result); uerr != nil {
			r.log.Warn("runs: record usage", slog.Any("err", uerr))
		}
	}

	if run.AgentType == string(domain.AgentDevelopment) {
		if strings.TrimSpace(result.Diff) == "" {
			return r.fail(ctx, run, "no_changes", "the development agent produced no changes")
		}
		sum := sha256.Sum256([]byte(result.Diff))
		sha := hex.EncodeToString(sum[:])
		if err := r.store.SetDiff(context.WithoutCancel(ctx), run.OrgID, run.ID, []byte(result.Diff), sha); err != nil {
			return r.fail(ctx, run, "artifact_failed", err.Error())
		}
		if aerr := r.artifacts.Upload(ctx, run.OrgID, run.ID, "diff", []byte(result.Diff)); aerr != nil {
			r.log.Warn("runs: upload diff", slog.Any("err", aerr))
		}
		_ = r.store.AppendEvent(context.WithoutCancel(ctx), run.OrgID, run.ID, "diff.ready", "agent",
			map[string]any{"sha256": sha, "bytes": len(result.Diff)})

		if r.testCommand != "" && r.tests != nil {
			status := "passed"
			if result.TestExitCode != 0 {
				status = "failed"
			}
			if aerr := r.artifacts.Upload(ctx, run.OrgID, run.ID, "test_report", []byte(result.TestOutput)); aerr != nil {
				r.log.Warn("runs: upload test report", slog.Any("err", aerr))
			}
			if terr := r.tests.RecordRunTest(context.WithoutCancel(ctx), run.OrgID, run.ID, "test", status,
				truncate(result.TestOutput, 20000)); terr != nil {
				r.log.Warn("runs: record test result", slog.Any("err", terr))
			}
		}
		if r.testCommand != "" && result.TestExitCode != 0 {
			class := classifyTestFailure(result.TestOutput)
			_ = r.store.AppendEvent(context.WithoutCancel(ctx), run.OrgID, run.ID, "test.classified", "system",
				map[string]any{"class": class})
			code := "test_gate_failed"
			detail := truncate(result.TestOutput, 2000)
			switch class {
			case "infra":
				code = "test_infra"
			case "dependency":
				code = "dependency_missing"
				detail = "the offline sandbox cannot install a required dependency; " +
					"prepare it on the host and retry (see the dependency-preparation story)\n\n" +
					detail
			}
			return r.fail(ctx, run, code, detail)
		}
	}

	if run.AgentType == string(domain.AgentDevelopment) && e2eCommand != "" && r.headRunner != nil && len(result.Workspace) > 0 {
		exit, e2eOut, e2eErr := r.headRunner.RunHead(ctx, result.Workspace, e2eCommand)
		if e2eErr != nil {
			_ = r.store.AppendEvent(context.WithoutCancel(ctx), run.OrgID, run.ID, "e2e.unavailable", "system",
				map[string]any{"detail": e2eErr.Error()})
		} else {
			status := "passed"
			if exit != 0 {
				status = "failed"
			}
			if r.tests != nil {
				_ = r.tests.RecordRunTest(context.WithoutCancel(ctx), run.OrgID, run.ID, "e2e", status, truncate(e2eOut, 20000))
			}
			_ = r.store.AppendEvent(context.WithoutCancel(ctx), run.OrgID, run.ID, "e2e.completed", "system",
				map[string]any{"status": status, "exit": exit})
			if exit != 0 {
				return r.fail(ctx, run, "e2e_failed", truncate(e2eOut, 2000))
			}
		}
	}

	if run.AgentType == string(domain.AgentDevelopment) && visualCommand != "" && r.headRunner != nil && len(result.Workspace) > 0 {
		exit, visualOut, verr := r.headRunner.RunHead(ctx, result.Workspace, visualCommand)
		if verr != nil {
			_ = r.store.AppendEvent(context.WithoutCancel(ctx), run.OrgID, run.ID, "visual.unavailable", "system",
				map[string]any{"detail": verr.Error()})
		} else {
			status := "passed"
			if exit != 0 {
				status = "failed"
			}
			if r.tests != nil {
				_ = r.tests.RecordRunTest(context.WithoutCancel(ctx), run.OrgID, run.ID, "visual", status, truncate(visualOut, 20000))
			}
			_ = r.store.AppendEvent(context.WithoutCancel(ctx), run.OrgID, run.ID, "visual.completed", "system",
				map[string]any{"status": status, "exit": exit})
			if exit != 0 {
				return r.fail(ctx, run, "visual_failed", truncate(visualOut, 2000))
			}
			if r.visuals != nil && len(result.Workspace) > 0 {
				if detail, verr := r.checkVisualBaselines(ctx, run, result.Workspace, visualThreshold, visualMasks); verr != nil {
					return r.fail(ctx, run, "visual_failed", detail)
				}
			}
		}
	}

	for _, check := range pluginChecks {
		exit, output, perr := r.headRunner.RunHead(runCtx, result.Workspace, check.Command)
		status := "passed"
		if perr != nil || exit != 0 {
			status = "failed"
		}
		if r.tests != nil {
			_ = r.tests.RecordRunTest(context.WithoutCancel(ctx), run.OrgID, run.ID, "plugin:"+check.Name, status, truncate(output, 20000))
		}
		_ = r.store.AppendEvent(context.WithoutCancel(ctx), run.OrgID, run.ID, "plugin.completed", "system",
			map[string]any{"name": check.Name, "status": status, "exit": exit})
		if status == "failed" {
			detail := truncate(output, 2000)
			if perr != nil {
				detail = perr.Error()
			}
			return r.fail(runCtx, run, "plugin_failed", check.Name+": "+detail)
		}
	}

	if run.AgentType == string(domain.AgentDevelopment) && len(result.Diff) > 0 {
		if detail := policyViolation(result.Diff, policyMaxDiff, forbiddenPaths); detail != "" {
			return r.fail(runCtx, run, "policy_violation", detail)
		}
		affected := monorepo.Affected(result.Diff, monorepo.Manifests(workspace))
		_ = r.store.AppendEvent(context.WithoutCancel(ctx), run.OrgID, run.ID, "context.affected_packages", "system",
			map[string]any{"packages": affected})
	}
	if run.AgentType == string(domain.AgentDevelopment) {
		findings := secretscan.Scan(result.Diff)
		status := "passed"
		if len(findings) > 0 {
			status = "failed"
		}
		if r.tests != nil {
			_ = r.tests.RecordRunTest(context.WithoutCancel(ctx), run.OrgID, run.ID, "secrets", status, secretscan.Summary(findings))
		}
		_ = r.store.AppendEvent(context.WithoutCancel(ctx), run.OrgID, run.ID, "secrets.scanned", "system",
			map[string]any{"status": status, "findings": len(findings)})
		if len(findings) > 0 {
			return r.fail(ctx, run, "secret_detected", secretscan.Summary(findings))
		}
	}

	var acceptanceProof []byte
	if run.AgentType == string(domain.AgentDevelopment) && r.tests != nil {
		if warned := acceptance.TempTests(result.Diff); len(warned) > 0 {
			_ = r.store.AppendEvent(context.WithoutCancel(ctx), run.OrgID, run.ID, "acceptance.warning", "system",
				map[string]any{"detail": "temporary test files in the diff: " + strings.Join(warned, ", ")})
		}
		if len(criteria) > 0 {
			status := "failed"
			var detail string
			proof, perr := acceptance.ParseProof(lines)
			if perr != nil {
				detail = perr.Error()
			} else {
				checked := acceptance.ValidateProof(proof, criteria, result.Diff)
				detail = checked.Detail
				if checked.Passed {
					status = "passed"
					if r.proofVerifier != nil && r.testCommand != "" && len(workspace) > 0 && len(result.Workspace) > 0 {
						if files := acceptance.TestFiles(proof); len(files) > 0 {
							if baseArchive, aerr := ApplyTestFiles(workspace, result.Workspace, files); aerr != nil {
								_ = r.store.AppendEvent(context.WithoutCancel(ctx), run.OrgID, run.ID, "acceptance.base_unavailable", "system",
									map[string]any{"detail": aerr.Error()})
							} else if exit, out, verr := r.proofVerifier.VerifyBase(ctx, baseArchive, files, r.testCommand); verr != nil {
								_ = r.store.AppendEvent(context.WithoutCancel(ctx), run.OrgID, run.ID, "acceptance.base_unavailable", "system",
									map[string]any{"detail": verr.Error()})
							} else if exit == 0 {
								status = "failed"
								detail = "the acceptance test also passes on the base commit; it does not prove the change"
								_ = r.store.AppendEvent(context.WithoutCancel(ctx), run.OrgID, run.ID, "acceptance.base_failed", "system",
									map[string]any{"exit": exit, "output": truncate(out, 2000)})
							} else {
								_ = r.store.AppendEvent(context.WithoutCancel(ctx), run.OrgID, run.ID, "acceptance.base_verified", "system",
									map[string]any{"exit": exit})
							}
						}
					}
				}
				raw, _ := json.MarshalIndent(checked.Proof, "", "  ")
				acceptanceProof = raw
				if r.artifacts != nil {
					if aerr := r.artifacts.Upload(ctx, run.OrgID, run.ID, "acceptance_proof", raw); aerr != nil {
						r.log.Warn("runs: upload acceptance proof", slog.Any("err", aerr))
					}
				}
			}
			if terr := r.tests.RecordRunTest(context.WithoutCancel(ctx), run.OrgID, run.ID, "acceptance", status, truncate(detail, 20000)); terr != nil {
				r.log.Warn("runs: record acceptance", slog.Any("err", terr))
			}
			_ = r.store.AppendEvent(context.WithoutCancel(ctx), run.OrgID, run.ID, "acceptance.checked", "system",
				map[string]any{"status": status, "detail": truncate(detail, 2000)})
			if status == "passed" && len(acceptanceProof) > 0 {
				if serr := r.store.SetAcceptanceProof(context.WithoutCancel(ctx), run.OrgID, run.ID, acceptanceProof); serr != nil {
					r.log.Warn("runs: persist acceptance proof", slog.Any("err", serr))
				}
			}
			if status != "passed" {
				return r.fail(ctx, run, "acceptance_unproven", detail)
			}
		} else {
			ok, detail := acceptance.Check(result.Diff)
			status := "passed"
			if !ok {
				status = "failed"
			}
			if terr := r.tests.RecordRunTest(context.WithoutCancel(ctx), run.OrgID, run.ID, "acceptance", status, detail); terr != nil {
				r.log.Warn("runs: record acceptance", slog.Any("err", terr))
			}
			_ = r.store.AppendEvent(context.WithoutCancel(ctx), run.OrgID, run.ID, "acceptance.checked", "system",
				map[string]any{"status": status, "detail": truncate(detail, 2000)})
			if !ok {
				return r.fail(ctx, run, "acceptance_missing", detail)
			}
		}
	}

	if run.AgentType == string(domain.AgentDevelopment) && designRef != "" && r.tests != nil {
		violations := designcheck.Scan(result.Diff, forbiddenImports)
		status := "passed"
		if len(violations) > 0 {
			status = "failed"
		}
		output := designcheck.Summary(violations)
		if terr := r.tests.RecordRunTest(context.WithoutCancel(ctx), run.OrgID, run.ID, "design", status, output); terr != nil {
			r.log.Warn("runs: record design check", slog.Any("err", terr))
		}
		_ = r.store.AppendEvent(context.WithoutCancel(ctx), run.OrgID, run.ID, "design.compliance", "system",
			map[string]any{"status": status, "violations": len(violations), "detail": truncate(output, 2000)})
	}

	if run.AgentType == string(domain.AgentReview) {
		review := extractReview(lines)
		if review == nil {
			return r.fail(ctx, run, "review_missing", "the review agent did not emit an AIP_REVIEW line")
		}
		if aerr := r.artifacts.Upload(ctx, run.OrgID, run.ID, "review", review); aerr != nil {
			r.log.Warn("runs: upload review", slog.Any("err", aerr))
		}
		if err := r.store.SetReview(context.WithoutCancel(ctx), run.OrgID, run.ID, review); err != nil {
			return r.fail(ctx, run, "artifact_failed", err.Error())
		}
		_ = r.store.AppendEvent(context.WithoutCancel(ctx), run.OrgID, run.ID, "review.completed", "agent",
			map[string]any{"findings": countFindings(review)})
	}

	plan := extractPlan(lines)
	if run.AgentType == string(domain.AgentPlanning) {
		if plan == nil {
			run.FailureCode = "plan_missing"
			run.FailureDetail = "the planning agent did not emit an AIP_PLAN line"
			if terr := run.Transition(domain.RunFailed, finished); terr == nil {
				_, _ = r.store.UpdateStatus(context.WithoutCancel(ctx), run)
			}
			return run, nil
		}
		r.awaitPlanApproval(ctx, &run, plan)
		return run, nil
	}

	if terr := run.Transition(domain.RunSucceeded, finished); terr == nil {
		span.SetAttributes(attribute.String("run.status", string(domain.RunSucceeded)))
		updated, uerr := r.store.UpdateStatus(context.WithoutCancel(ctx), run)
		if uerr != nil {
			return run, uerr
		}
		if updated.AgentType == string(domain.AgentDevelopment) && r.publisher != nil && len(result.Workspace) > 0 {
			updated.AcceptanceProof = acceptanceProof
			pr, perr := r.publisher.Publish(context.WithoutCancel(ctx), updated, result.Workspace)
			if perr != nil {
				r.log.Warn("runs: publish branch", slog.Any("err", perr))
				_ = r.store.AppendEvent(context.WithoutCancel(ctx), run.OrgID, run.ID, "pr.failed", "system",
					map[string]any{"detail": truncate(perr.Error(), 1000)})
			} else if pr.ID != "" {
				_ = r.store.AppendEvent(context.WithoutCancel(ctx), run.OrgID, run.ID, "pr.opened", "system",
					map[string]any{"number": pr.Number, "url": pr.URL})
				approval, aerr := r.store.CreateApproval(context.WithoutCancel(ctx), run.OrgID, run.ID, pr.ID, "pr", time.Now().UTC().Add(7*24*time.Hour))
				if aerr != nil {
					r.log.Warn("runs: PR approval", slog.Any("err", aerr))
				} else {
					_ = r.store.AppendEvent(context.WithoutCancel(ctx), run.OrgID, run.ID, "approval.requested", "system",
						map[string]any{"approval_id": approval.ID, "type": "pr", "number": pr.Number})
					if r.notifier != nil {
						_ = r.notifier.NotifyAdmins(context.WithoutCancel(ctx), run.OrgID, "approval",
							fmt.Sprintf("Pull request #%d is ready", pr.Number),
							"Review the diff and approve the merge.", pr.URL)
					}
				}
			}
		}
		// The review agent starts only after the development run finished, so
		// the one-active-run-per-task index is free.
		if updated.AgentType == string(domain.AgentDevelopment) && r.enqueuer != nil {
			review, rerr := r.store.Create(context.WithoutCancel(ctx), domain.Run{
				OrgID: run.OrgID, ProjectID: run.ProjectID, TaskID: run.TaskID,
				AgentType: string(domain.AgentReview), Status: domain.RunQueued, Model: run.Model,
			})
			if rerr == nil {
				if eerr := r.enqueuer.Enqueue(context.WithoutCancel(ctx), review.ID, review.OrgID); eerr != nil {
					r.log.Error("runs: enqueue review", slog.Any("err", eerr))
				}
			} else {
				r.log.Error("runs: create review run", slog.Any("err", rerr))
			}
		}
		return updated, nil
	}
	return run, nil
}

// fail records a failed run and returns it.
func (r *Runner) fail(ctx context.Context, run domain.Run, code, detail string) (domain.Run, error) {
	run.FailureCode = code
	run.FailureDetail = detail
	if r.notifier != nil {
		_ = r.notifier.NotifyAdmins(context.WithoutCancel(ctx), run.OrgID, "run",
			"Run failed: "+code, detail, fmt.Sprintf("/projects/%s/runs/%s", run.ProjectID, run.ID))
	}
	if err := run.Transition(domain.RunFailed, time.Now().UTC()); err != nil {
		return run, err
	}
	updated, err := r.store.UpdateStatus(context.WithoutCancel(ctx), run)
	if err != nil {
		return run, err
	}
	return updated, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// planPrefix marks the structured plan in the agent output.
const planPrefix = "AIP_PLAN:"

// extractPlan finds the last structured plan line emitted by the agent.
func extractPlan(lines []string) []byte {
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, planPrefix) {
			raw := strings.TrimSpace(strings.TrimPrefix(line, planPrefix))
			if json.Valid([]byte(raw)) {
				return []byte(raw)
			}
		}
	}
	return nil
}

// awaitPlanApproval stores the plan and parks the run for a human decision.
func (r *Runner) awaitPlanApproval(ctx context.Context, run *domain.Run, plan []byte) {
	bg := context.WithoutCancel(ctx)
	sum := sha256.Sum256(plan)
	sha := hex.EncodeToString(sum[:])

	if err := r.store.SetPlan(bg, run.OrgID, run.ID, plan, sha); err != nil {
		r.log.Error("runs: store plan", slog.Any("err", err))
		return
	}
	if err := run.Transition(domain.RunAwaitingApproval, time.Now().UTC()); err != nil {
		r.log.Error("runs: await approval", slog.Any("err", err))
		return
	}
	if _, err := r.store.UpdateStatus(bg, *run); err != nil {
		r.log.Error("runs: mark awaiting approval", slog.Any("err", err))
		return
	}
	approval, err := r.store.CreateApproval(bg, run.OrgID, run.ID, "", "plan", time.Now().UTC().Add(72*time.Hour))
	if err != nil {
		r.log.Error("runs: create approval", slog.Any("err", err))
		return
	}
	if aerr := r.artifacts.Upload(bg, run.OrgID, run.ID, "plan", plan); aerr != nil {
		r.log.Warn("runs: upload plan", slog.Any("err", aerr))
	}
	if r.notifier != nil {
		link := fmt.Sprintf("/projects/%s/runs/%s", run.ProjectID, run.ID)
		if nerr := r.notifier.NotifyAdmins(bg, run.OrgID, "approval", "Plan ready for approval",
			"Run "+run.ID[:8]+" is waiting for a human decision.", link); nerr != nil {
			r.log.Warn("runs: notify plan", slog.Any("err", nerr))
		}
	}
	_ = r.store.AppendEvent(bg, run.OrgID, run.ID, "approval.requested", "system",
		map[string]any{"approval_id": approval.ID, "type": approval.Type})
}

// extractReview finds the structured review emitted by the review agent.
func extractReview(lines []string) []byte {
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "AIP_REVIEW:") {
			raw := strings.TrimSpace(strings.TrimPrefix(line, "AIP_REVIEW:"))
			if json.Valid([]byte(raw)) {
				return []byte(raw)
			}
		}
	}
	return nil
}

// countFindings reports how many findings the review JSON contains.
func countFindings(raw []byte) int {
	var payload struct {
		Findings []struct {
			Severity string `json:"severity"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return 0
	}
	return len(payload.Findings)
}

type contextManifest struct {
	json        []byte          `json:"-"`
	hash        string          `json:"-"`
	TaskID      string          `json:"task_id"`
	AgentType   string          `json:"agent_type"`
	Model       string          `json:"model"`
	Files       []string        `json:"files"`
	Rules       []string        `json:"rules"`
	Design      string          `json:"design,omitempty"`
	Sources     []ContextSource `json:"sources,omitempty"`
	Precedence  []string        `json:"precedence"`
	GeneratedAt string          `json:"generated_at"`
}

// buildContextManifest hashes a canonical manifest of the task, the repo files
// and the rule versions. The same inputs always produce the same bytes.
func buildContextManifest(run domain.Run, files, rules []string, designRef string, sources []ContextSource) contextManifest {
	sortedFiles := append([]string(nil), files...)
	sort.Strings(sortedFiles)
	sortedRules := append([]string(nil), rules...)
	sort.Strings(sortedRules)
	sortedSources := append([]ContextSource(nil), sources...)
	sort.Slice(sortedSources, func(i, j int) bool {
		if sortedSources[i].Kind == sortedSources[j].Kind {
			return sortedSources[i].ID < sortedSources[j].ID
		}
		return sortedSources[i].Kind < sortedSources[j].Kind
	})
	m := contextManifest{
		TaskID: run.TaskID, AgentType: run.AgentType, Model: run.Model,
		Files: sortedFiles, Rules: sortedRules, Design: designRef,
		Sources: sortedSources, Precedence: contextPrecedence,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}
	canonical := m
	canonical.GeneratedAt = ""
	// The approved context is the task/repo/rules identity; the agent role
	// (planning vs development) must not change it.
	canonical.AgentType = ""
	raw, _ := json.Marshal(canonical)
	sum := sha256.Sum256(raw)
	m.json = raw
	m.hash = hex.EncodeToString(sum[:])
	return m
}

// tarFiles lists regular file paths in an uncompressed tar workspace.
func tarFiles(archive []byte) []string {
	out := make([]string, 0)
	reader := tar.NewReader(bytes.NewReader(archive))
	for {
		header, err := reader.Next()
		if err != nil {
			break
		}
		name := strings.TrimPrefix(header.Name, "./")
		if header.Typeflag == tar.TypeReg && !strings.HasPrefix(name, ".git/") && name != ".git" {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// hashWorkspace hashes sorted file paths and contents, ignoring tar metadata
// (mtime, ordering) so two clones of the same tree have the same identity.
func hashWorkspace(archive []byte) string {
	type entry struct{ path, hash string }
	entries := make([]entry, 0)
	reader := tar.NewReader(bytes.NewReader(archive))
	for {
		header, err := reader.Next()
		if err != nil {
			break
		}
		name := strings.TrimPrefix(header.Name, "./")
		if header.Typeflag != tar.TypeReg || strings.HasPrefix(name, ".git/") || name == ".git" {
			continue
		}
		content, err := io.ReadAll(reader)
		if err != nil {
			continue
		}
		entries = append(entries, entry{path: name, hash: hashBytes(content)})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].path < entries[j].path })
	sum := sha256.New()
	for _, e := range entries {
		_, _ = io.WriteString(sum, e.path+"\x00"+e.hash+"\n")
	}
	return hex.EncodeToString(sum.Sum(nil))
}

// hashBytes returns the SHA-256 of data as hex.
func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// contextMismatch reports whether an approved context was frozen and differs
// from the context the development run would use.
func contextMismatch(frozen, current string) bool {
	return frozen != "" && frozen != current
}

// sourceDelta names the sources whose identity or content changed.
func sourceDelta(frozen, current []ContextSource) []string {
	key := func(s ContextSource) string { return s.Kind + ":" + s.ID }
	old := map[string]string{}
	for _, source := range frozen {
		old[key(source)] = source.SHA256 + "@" + source.Version
	}
	var changed []string
	for _, source := range current {
		if was, ok := old[key(source)]; !ok || was != source.SHA256+"@"+source.Version {
			changed = append(changed, source.Kind)
		}
		delete(old, key(source))
	}
	for kind := range old {
		changed = append(changed, kind)
	}
	sort.Strings(changed)
	return changed
}
