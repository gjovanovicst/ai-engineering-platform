// Command aip-worker executes agent runs and background jobs (River).
package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	gitadapters "github.com/gjovanovicst/ai-engineering-platform/internal/adapters/git"
	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/mail"
	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres"
	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/previewdocker"
	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/s3cache"
	"github.com/gjovanovicst/ai-engineering-platform/internal/agent"
	"github.com/gjovanovicst/ai-engineering-platform/internal/agent/sandbox"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/checks"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/checksync"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/embeddings"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/githubsetup"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/notifications"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/previews"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/providers"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/rules"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/runartifacts"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/runs"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/tasks"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/usage"
	"github.com/gjovanovicst/ai-engineering-platform/internal/config"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/artifacts"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/events"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/githubapp"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/log"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/queue"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/retention"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/siem"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/telemetry"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/workermetrics"
)

var version = "dev"

// s3Objects adapts the platform artifact store to runartifacts.ObjectStore.
type s3Objects struct{ store *artifacts.Store }

func (s s3Objects) Put(ctx context.Context, key string, body []byte, contentType string) error {
	if s.store == nil {
		return nil
	}
	return s.store.Put(ctx, key, bytes.NewReader(body), int64(len(body)), contentType)
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", slog.Any("err", err))
		os.Exit(2)
	}

	logger := log.New(cfg.LogLevel, "aip-worker")
	shutdownTelemetry, err := telemetry.Init(context.Background(), telemetry.Config{
		Endpoint: cfg.OTELEndpoint, ServiceName: "aip-worker", Insecure: true,
	})
	if err != nil {
		logger.Error("telemetry", slog.Any("err", err))
		os.Exit(1)
	}
	defer func() { _ = shutdownTelemetry(context.Background()) }()
	logger.Info("starting", slog.String("version", version), slog.String("sandbox", cfg.SandboxDriver))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DBDSN)
	if err != nil {
		logger.Error("database", slog.Any("err", err))
		os.Exit(1)
	}
	if cfg.SIEMWebhookURL != "" {
		go siem.Run(ctx, pool, &siem.Sender{URL: cfg.SIEMWebhookURL, Secret: []byte(cfg.SIEMWebhookSecret)}, logger)
	}

	if cfg.WorkerMetricsAddr != "" {
		metricsHandler := &workermetrics.Handler{Queue: pgQueue{pool: pool}, Started: time.Now()}
		metricsServer := &http.Server{Addr: cfg.WorkerMetricsAddr, Handler: metricsHandler, ReadHeaderTimeout: 5 * time.Second}
		go func() {
			<-ctx.Done()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = metricsServer.Shutdown(shutdownCtx)
		}()
		go func() {
			if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Warn("worker metrics", slog.Any("err", err))
			}
		}()
	}

	defer pool.Close()

	var driver sandbox.Sandbox
	switch cfg.SandboxDriver {
	case "k8s", "kubernetes":
		driver, err = sandbox.NewKubernetes(cfg.SandboxNamespace)
		if err != nil {
			logger.Error("kubernetes sandbox", slog.Any("err", err))
			os.Exit(1)
		}
	default:
		driver = sandbox.NewDocker()
	}
	command := cfg.AgentCommand
	if preset := agent.AdapterCommand(cfg.AgentAdapter); preset != "" {
		command = preset
	}
	backend, err := agent.NewCommandBackend(driver, command, cfg.SandboxImage)
	if err != nil {
		logger.Error("agent backend", slog.Any("err", err))
		os.Exit(1)
	}

	bus, err := events.New(ctx, cfg.RedisAddr)
	if err != nil {
		logger.Warn("redis unavailable; live logs fall back to polling", slog.Any("err", err))
		bus = nil
	}
	runStore := postgres.NewRunStore(pool).WithPublisher(bus)
	taskService, err := tasks.New(postgres.NewTaskStore(pool))
	if err != nil {
		logger.Error("task service", slog.Any("err", err))
		os.Exit(1)
	}
	workers := queue.NewWorkers()
	maxWorkers := cfg.WorkerMaxWorkers
	if maxWorkers < 1 {
		maxWorkers = 1
	}
	client, err := queue.NewClient(pool, workers, maxWorkers)
	if err != nil {
		logger.Error("queue client", slog.Any("err", err))
		os.Exit(1)
	}
	repoStore := postgres.NewRepoStore(pool)
	publisher := &gitadapters.Publisher{
		Repos:    repoStore,
		Recorder: repoStore,
		Tasks:    taskService,
		WebURL:   cfg.WebURL,
		Pusher:   gitadapters.HostPusher{},
		Providers: func(provider, token, secret string) gitadapters.Provider {
			if provider == "gitlab" {
				p := gitadapters.NewGitLab(token, secret)
				p.BaseURL = cfg.GitLabAPIURL
				return p
			}
			if provider == "gitea" {
				p := gitadapters.NewGitea(token, []byte(secret))
				p.BaseURL = cfg.GiteaAPIURL
				return p
			}
			if provider == "bitbucket" {
				p := gitadapters.NewBitbucket(token, []byte(secret))
				p.BaseURL = cfg.BitbucketAPIURL
				return p
			}
			p := gitadapters.NewGitHub(token, secret)
			p.BaseURL = cfg.GitHubAPIURL
			return p
		},
		Tokens: map[string]string{
			"github":    cfg.GitHubToken,
			"gitlab":    cfg.GitLabToken,
			"gitea":     cfg.GiteaToken,
			"bitbucket": cfg.BitbucketToken,
		},
		CloneBases: map[string]string{
			"github":    cfg.GitHubCloneBase,
			"gitlab":    cfg.GitLabCloneBase,
			"gitea":     cfg.GiteaCloneBase,
			"bitbucket": cfg.BitbucketCloneBase,
		},
	}
	providerCipher, err := providers.NewCipher(cfg.SecretsKey)
	if err != nil {
		logger.Error("provider cipher", slog.Any("err", err))
		os.Exit(1)
	}
	var appTokenCipher githubsetup.Cipher
	if providerCipher != nil {
		appTokenCipher = providerCipher
	}
	githubAppSetup := githubsetup.New(postgres.NewGithubAppStore(pool), appTokenCipher, cfg.APIURL, cfg.WebURL, cfg.GitHubAPIURL)
	orgTokenFunc := func(ctx context.Context, orgID, provider string) (string, error) {
		if provider != "github" {
			return "", nil
		}
		return githubAppSetup.InstallationToken(ctx, orgID)
	}

	providerService, err := providers.New(postgres.NewProviderStore(pool), providerCipher, logger)
	if err != nil {
		logger.Error("provider service", slog.Any("err", err))
		os.Exit(1)
	}
	usageService := usage.New(postgres.NewUsageStore(pool), cfg.PriceInputPer1K, cfg.PriceOutputPer1K)
	checkService := checks.New(postgres.NewCheckStore(pool), logger)
	gitChecks := gitadapters.NewGitHub(cfg.GitHubToken, "")
	gitChecks.BaseURL = cfg.GitHubAPIURL
	checkSyncer := checksync.New(postgres.NewPRSyncStore(pool), checkService,
		map[string]checksync.Lister{"github": gitCheckLister{static: gitChecks, appToken: githubAppSetup.InstallationToken}}, logger)
	go checkSyncer.Run(ctx, time.Minute)
	notificationService := notifications.New(postgres.NewNotificationStore(pool), logger).
		WithEmailer(mail.NewSMTP(cfg.SMTPAddr, cfg.MailFrom), cfg.WebURL)

	if cfg.GitHubAppID > 0 && cfg.GitHubAppKeyPath != "" {
		app, aerr := githubapp.New(cfg.GitHubAppID, cfg.GitHubAppInstall, cfg.GitHubAppKeyPath, cfg.GitHubAPIURL)
		if aerr != nil {
			logger.Warn("github app disabled; falling back to the static token", slog.Any("err", aerr))
		} else {
			publisher.TokenFunc = func(ctx context.Context, provider string) (string, error) {
				if provider == "github" {
					return app.Token(ctx)
				}
				return cfg.GitLabToken, nil
			}
			gitChecks.Token = app.Token
			logger.Info("github app tokens enabled", slog.Int64("app_id", cfg.GitHubAppID))
		}
	}

	publisher.OrgTokenFunc = orgTokenFunc
	cloner := &gitadapters.Cloner{
		Repos:        repoStore,
		Tokens:       publisher.Tokens,
		TokenFunc:    publisher.TokenFunc,
		OrgTokenFunc: orgTokenFunc,
		CloneBases:   publisher.CloneBases,
	}

	objectStore, oerr := artifacts.New(ctx, cfg.S3Endpoint, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3Region, cfg.S3Bucket, cfg.S3UseSSL)
	if oerr != nil {
		logger.Warn("artifact store unavailable; artifacts stay in the database", slog.Any("err", oerr))
		objectStore = nil
	}
	artifactSink := runartifacts.New(s3Objects{store: objectStore}, postgres.NewArtifactStore(pool), logger)

	pluginStore := postgres.NewAgentPluginStore(pool)
	proofVerifier := &sandboxProofVerifier{sandbox: driver, image: cfg.SandboxImage}
	runner := runs.NewRunner(backend, runStore, queue.Enqueuer{Client: client}, publisherAdapter{publisher}, cfg.TestCommand, logger).
		WithProofVerifier(proofVerifier).
		WithHeadRunner(proofVerifier).
		WithUsageRecorder(usageService).
		WithTests(checkService).
		WithArtifacts(artifactSink).
		WithNotifier(notificationService).
		WithWorkspaceProvider(cloner).
		WithProjectBackendSelector(func(ctx context.Context, orgID, projectID, agentType string) (agent.Backend, error) {
			plugin, perr := pluginStore.ForProject(ctx, orgID, projectID)
			if errors.Is(perr, domain.ErrNotFound) {
				return nil, nil
			}
			if perr != nil {
				return nil, perr
			}
			for _, allowed := range plugin.AgentTypes {
				if allowed == agentType {
					return agent.NewCommandBackend(driver, plugin.Command, plugin.Image)
				}
			}
			return nil, nil
		}).
		WithPrices(cfg.PriceInputPer1K, cfg.PriceOutputPer1K)
	if objectStore != nil {
		runner = runner.WithVisualBaselines(objectStore)
	}
	if client := embeddings.New(cfg.EmbeddingsURL, cfg.EmbeddingsKey, cfg.EmbeddingsModel); client != nil {
		var embedder runs.Embedder = client
		if objectStore != nil {
			embedder = embeddings.CachedClient{Client: client, Cache: s3cache.New(objectStore)}
		}
		runner = runner.WithEmbedder(embedder)
	}
	if cfg.AgentBackend == "tools" {
		runner = runner.WithBackendSelector(func(ctx context.Context, orgID string) (agent.Backend, error) {
			provider, key, perr := providerService.Default(ctx, orgID)
			if perr != nil {
				logger.Warn("provider unavailable; using the command backend", slog.Any("err", perr))
				return nil, nil
			}
			if key == "" {
				logger.Info("provider has no key; using the command backend")
				return nil, nil
			}
			if provider.Kind == "anthropic" {
				return &agent.AnthropicBackend{
					Sandbox: driver, Image: cfg.SandboxImage,
					BaseURL: provider.BaseURL, APIKey: key, Model: provider.Model, MaxSteps: 12,
					MCPConfig: cfg.MCPConfig,
				}, nil
			}
			return &agent.ToolLoopBackend{
				Sandbox: driver, Image: cfg.SandboxImage,
				BaseURL: provider.BaseURL, APIKey: key, Model: provider.Model, MaxSteps: 12,
				MCPConfig: cfg.MCPConfig,
			}, nil
		})
	}
	if cfg.AgentBackend == "provider" {
		runner = runner.WithBackendSelector(func(ctx context.Context, orgID string) (agent.Backend, error) {
			provider, key, perr := providerService.Default(ctx, orgID)
			if perr != nil || key == "" {
				return nil, nil // fall back to the configured command
			}
			if provider.Kind == "anthropic" {
				return &agent.AnthropicBackend{
					Sandbox: driver, Image: cfg.SandboxImage,
					BaseURL: provider.BaseURL, APIKey: key, Model: provider.Model, MaxSteps: 12,
					MCPConfig: cfg.MCPConfig,
				}, nil
			}
			httpBackend := agent.NewHTTPBackend(provider.BaseURL, key, provider.Model)
			httpBackend.MaxAttempts = cfg.LLMMaxAttempts
			return httpBackend, nil
		})
	}
	ruleService, err := rules.New(postgres.NewRuleStore(pool), logger)
	if err != nil {
		logger.Error("rule service", slog.Any("err", err))
		os.Exit(1)
	}
	projectStore := postgres.NewProjectStore(pool)
	designStandardStore := postgres.NewDesignStandardStore(pool)
	attachmentStore := postgres.NewAttachmentStore(pool)
	designStore := postgres.NewDesignStore(pool)
	previewService, perr := previews.New(postgres.NewPreviewStore(pool), previewdocker.New())
	if perr != nil {
		logger.Error("previews", slog.Any("err", perr))
		os.Exit(1)
	}
	river.AddWorker(workers, previews.Worker{Service: previewService})
	river.AddWorker(workers, runs.NewJobWorker(runStore, taskService, ruleService, runner, logger).
		WithDesignStandard(projectStore, designStandardStore).
		WithComponents(projectStore, designStore).
		WithAttachments(attachmentStore).
		WithOrgGate(postgres.NewOrgGate(pool), cfg.OrgMaxConcurrentRuns))
	if err := client.Start(ctx); err != nil {
		logger.Error("queue start", slog.Any("err", err))
		os.Exit(1)
	}
	logger.Info("worker started")

	if cfg.AdminDSN != "" {
		go func() {
			store, aerr := artifacts.New(ctx, cfg.S3Endpoint, cfg.S3AccessKey, cfg.S3SecretKey,
				cfg.S3Region, cfg.S3Bucket, cfg.S3UseSSL)
			if aerr != nil {
				logger.Warn("retention: artifact store unavailable, archiving disabled", slog.Any("err", aerr))
				store = nil
			}
			svc, rerr := retention.New(ctx, cfg.AdminDSN, store, cfg.RetentionDays, logger)
			if rerr != nil {
				logger.Warn("retention disabled", slog.Any("err", rerr))
				return
			}
			defer svc.Close()
			run := func() {
				stats, err := svc.Run(ctx)
				if err != nil {
					logger.Warn("retention run", slog.Any("err", err))
					return
				}
				logger.Info("retention done", slog.Any("stats", stats))
			}
			run()
			ticker := time.NewTicker(time.Duration(cfg.RetentionHours) * time.Hour)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					run()
				}
			}
		}()
	}

	// Expire stale approvals even when nobody opens the approvals list.
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		sweep := func() {
			rows, err := pool.Query(ctx, "SELECT id::text FROM orgs")
			if err != nil {
				logger.Warn("approval sweep: list orgs", slog.Any("err", err))
				return
			}
			defer rows.Close()
			for rows.Next() {
				var orgID string
				if err := rows.Scan(&orgID); err != nil {
					continue
				}
				expired, err := runStore.ExpireApprovals(ctx, orgID)
				if err != nil {
					logger.Warn("approval sweep", slog.String("org", orgID), slog.Any("err", err))
					continue
				}
				for _, approval := range expired {
					_ = runStore.AppendEvent(ctx, orgID, approval.RunID, "approval.expired", "system",
						map[string]any{"approval_id": approval.ID, "type": approval.Type})
				}
			}
		}
		sweep()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sweep()
			}
		}
	}()

	<-ctx.Done()
	logger.Info("shutdown requested")

	shutdown, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := client.Stop(shutdown); err != nil {
		logger.Error("queue stop", slog.Any("err", err))
	}
	logger.Info("stopped")
}

// publisherAdapter maps the git publisher's PR type onto runs.PullRequest.
type publisherAdapter struct {
	inner *gitadapters.Publisher
}

func (a publisherAdapter) Publish(ctx context.Context, run domain.Run, workspace []byte) (runs.PullRequest, error) {
	pr, err := a.inner.Publish(ctx, run, workspace)
	if err != nil {
		return runs.PullRequest{}, err
	}
	return runs.PullRequest{ID: pr.ID, Number: pr.Number, URL: pr.URL}, nil
}

// gitCheckLister adapts the GitHub client to checksync.
type gitCheckLister struct {
	static   *gitadapters.GitHub
	appToken func(ctx context.Context, orgID string) (string, error)
}

func (l gitCheckLister) ListChecks(ctx context.Context, orgID, repo string, number int) ([]checksync.Check, error) {
	client := l.static
	if l.appToken != nil {
		if token, err := l.appToken(ctx, orgID); err == nil && token != "" {
			client = gitadapters.NewGitHub(token, "")
			client.BaseURL = l.static.BaseURL
		}
	}
	events, err := client.ListChecks(ctx, repo, number)
	if err != nil {
		return nil, err
	}
	out := make([]checksync.Check, 0, len(events))
	for _, event := range events {
		out = append(out, checksync.Check{
			Name: event.CheckName, Status: event.CheckStatus, Output: event.CheckOutput,
			DurationMS: event.CheckDurationMS, CommitSHA: event.CommitSHA,
		})
	}
	return out, nil
}

type pgQueue struct{ pool *pgxpool.Pool }

func (q pgQueue) Runs(ctx context.Context) (map[string]int64, error) {
	rows, err := q.pool.Query(ctx, `SELECT status, count FROM aip_run_status_counts()`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		out[status] = count
	}
	return out, rows.Err()
}

func (q pgQueue) Stats(ctx context.Context) (int64, int64, float64, error) {
	var available, running int64
	var oldest float64
	err := q.pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE state = 'available')::bigint,
		       count(*) FILTER (WHERE state = 'running')::bigint,
		       COALESCE(EXTRACT(EPOCH FROM (now() - min(scheduled_at) FILTER (WHERE state = 'available'))), 0)::float8
		FROM river_job`).Scan(&available, &running, &oldest)
	return available, running, oldest, err
}
