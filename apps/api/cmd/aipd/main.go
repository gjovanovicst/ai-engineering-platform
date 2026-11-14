// Command aipd is the HTTP API server.
package main

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/clamav"
	gitadapters "github.com/gjovanovicst/ai-engineering-platform/internal/adapters/git"
	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/mail"
	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres"
	"github.com/gjovanovicst/ai-engineering-platform/internal/agent"
	"github.com/gjovanovicst/ai-engineering-platform/internal/agent/sandbox"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/agentplugins"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/analytics"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/apikeys"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/attachments"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auditwitness"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/blueprints"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/checks"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/deployments"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/design"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/designstandards"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/directory"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/githubsetup"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/incidents"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/invites"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/mergequeue"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/notifications"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/oidc"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/orgcrypto"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/orgs"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/previews"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/projectaccess"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/projects"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/providers"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/restores"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/rules"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/runs"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/saml"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/scim"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/search"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/sso"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/tasks"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/usage"
	"github.com/gjovanovicst/ai-engineering-platform/internal/config"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
	"github.com/gjovanovicst/ai-engineering-platform/internal/httpapi"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/artifacts"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/events"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/figma"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/imageverify"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/kms"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/log"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/queue"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/ratelimit"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/replay"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/telemetry"
	"github.com/redis/go-redis/v9"
)

// version is set via the linker flag: -X main.version=$(VERSION)
var version = "dev"

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", slog.Any("err", err))
		os.Exit(2)
	}

	logger := log.New(cfg.LogLevel, "aipd")
	shutdownTelemetry, err := telemetry.Init(context.Background(), telemetry.Config{
		Endpoint: cfg.OTELEndpoint, ServiceName: "aipd", Insecure: true,
	})
	if err != nil {
		logger.Error("telemetry", slog.Any("err", err))
		os.Exit(1)
	}
	defer func() { _ = shutdownTelemetry(context.Background()) }()
	logger.Info("starting", slog.String("config", cfg.Redacted()), slog.String("version", version))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DBDSN)
	if err != nil {
		logger.Error("database", slog.Any("err", err))
		os.Exit(1)
	}
	defer pool.Close()

	keyPEM, err := os.ReadFile(cfg.JWTKeyPath)
	if err != nil {
		logger.Error("jwt key is not readable", slog.String("path", cfg.JWTKeyPath),
			slog.String("hint", "run `make keygen` or set AIP_JWT_KEY"))
		os.Exit(1)
	}
	issuer, err := auth.NewIssuer(keyPEM, cfg.AccessTTL)
	if err != nil {
		logger.Error("jwt key", slog.Any("err", err))
		os.Exit(1)
	}

	sessionStore := postgres.NewSessionStore(pool)
	tokens, err := auth.NewService(sessionStore, issuer, cfg.RefreshTTL)
	if err != nil {
		logger.Error("token service", slog.Any("err", err))
		os.Exit(1)
	}
	tokens.WithIdleTTL(cfg.SessionIdleTTL)
	mailer := mail.NewSMTP(cfg.SMTPAddr, cfg.MailFrom)
	accounts, err := auth.NewAccounts(
		postgres.NewAccountStore(pool), tokens, mailer, cfg.WebURL, logger,
	)
	if err != nil {
		logger.Error("account service", slog.Any("err", err))
		os.Exit(1)
	}

	projectService, err := projects.New(postgres.NewProjectStore(pool))
	repoStore := postgres.NewRepoStore(pool)
	if err != nil {
		logger.Error("project service", slog.Any("err", err))
		os.Exit(1)
	}

	bus, err := events.New(ctx, cfg.RedisAddr)
	if err != nil {
		logger.Warn("redis unavailable; SSE falls back to database polling", slog.Any("err", err))
		bus = nil
	}
	runStore := postgres.NewRunStore(pool).WithPublisher(bus)
	taskService, err := tasks.New(postgres.NewTaskStore(pool))
	if err != nil {
		logger.Error("task service", slog.Any("err", err))
		os.Exit(1)
	}

	backend, err := agent.NewCommandBackend(sandbox.NewDocker(), cfg.AgentCommand, cfg.SandboxImage)
	if err != nil {
		logger.Error("agent backend", slog.Any("err", err))
		os.Exit(1)
	}
	runner := runs.NewRunner(backend, runStore, nil, nil, cfg.TestCommand, logger)
	queueClient, err := queue.NewClient(pool, nil, 1)
	if err != nil {
		logger.Error("queue client", slog.Any("err", err))
		os.Exit(1)
	}
	merger := &gitadapters.Publisher{
		Repos:    repoStore,
		Recorder: repoStore,
		Providers: func(provider, token, secret string) gitadapters.Provider {
			if provider == "gitlab" {
				p := gitadapters.NewGitLab(token, secret)
				p.BaseURL = cfg.GitLabAPIURL
				return p
			}
			p := gitadapters.NewGitHub(token, secret)
			p.BaseURL = cfg.GitHubAPIURL
			return p
		},
		Tokens: map[string]string{
			"github": cfg.GitHubToken,
			"gitlab": cfg.GitLabToken,
		},
		CloneBases: map[string]string{
			"github": cfg.GitHubCloneBase,
			"gitlab": cfg.GitLabCloneBase,
		},
	}
	checkService := checks.New(postgres.NewCheckStore(pool), logger)
	attachmentService := attachments.New(postgres.NewAttachmentStore(pool))
	var baselineObjects httpapi.ObjectStore
	if cfg.S3Endpoint != "" {
		objectStore, oerr := artifacts.New(context.Background(), cfg.S3Endpoint, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3Region, cfg.S3Bucket, cfg.S3UseSSL)
		if oerr != nil {
			logger.Error("attachments: object store", slog.Any("err", oerr))
		} else if objectStore != nil {
			attachmentService.WithObjectStore(objectStore)
			baselineObjects = objectStore
		}
	}
	if cfg.ClamAVAddr != "" {
		attachmentService.WithScanner(clamav.New(cfg.ClamAVAddr))
	}
	artifactStore := postgres.NewArtifactStore(pool)
	notificationService := notifications.New(postgres.NewNotificationStore(pool), logger)
	orgService := orgs.New(postgres.NewOrgStore(pool))
	designStandardService := designstandards.New(postgres.NewDesignStandardStore(pool))
	runService, err := runs.New(runStore, taskService, runner, queue.Enqueuer{Client: queueClient}, merger, logger)
	if err != nil {
		logger.Error("run service", slog.Any("err", err))
		os.Exit(1)
	}
	mergeQueueService := mergequeue.New(postgres.NewMergeQueueStore(pool), merger, logger).WithGate(checkService)
	runService.WithChangeWindow(projectService)
	auditWitnessService, awerr := auditwitness.New(postgres.NewAuditWitnessStore(pool), baselineObjects)
	if awerr != nil {
		logger.Error("audit witness", slog.Any("err", awerr))
		os.Exit(1)
	}
	apiKeyService, akerr := apikeys.New(postgres.NewAPIKeyStore(pool))
	if akerr != nil {
		logger.Error("api keys", slog.Any("err", akerr))
		os.Exit(1)
	}
	projectAccessService, paerr := projectaccess.New(postgres.NewProjectMemberStore(pool))
	if paerr != nil {
		logger.Error("project access", slog.Any("err", paerr))
		os.Exit(1)
	}
	previewService, perr := previews.New(postgres.NewPreviewStore(pool), nil)
	if perr != nil {
		logger.Error("previews", slog.Any("err", perr))
		os.Exit(1)
	}
	previewService.WithScheduler(previewScheduler{schedule: func(ctx context.Context, previewID, orgID, action string, after time.Duration) error {
		return queue.EnqueuePreview(ctx, queueClient, previewID, orgID, action, after)
	}})
	runService.WithMergeQueue(mergeQueueService)

	runService.WithChecks(checkService)
	runService.WithMaxActiveRuns(cfg.MaxActiveRuns)

	blueprintService, err := blueprints.New(postgres.NewBlueprintStore(pool),
		func(ctx context.Context, orgID, blueprintID, name, description string) (domain.Project, error) {
			return projectService.Create(ctx, projects.CreateInput{
				OrgID: orgID, BlueprintID: blueprintID, Name: name, Description: description,
			})
		},
		func(ctx context.Context, orgID, projectID, title, description string, priority domain.Priority, acceptance []domain.AcceptanceCriterion) (domain.Task, error) {
			return taskService.Create(ctx, tasks.CreateInput{
				OrgID: orgID, ProjectID: projectID, Title: title, Description: description,
				Priority: priority, Acceptance: acceptance,
			})
		}, logger)
	if err != nil {
		logger.Error("blueprint service", slog.Any("err", err))
		os.Exit(1)
	}

	inviteService, err := invites.New(postgres.NewInviteStore(pool), mailer, accounts, cfg.WebURL, logger)
	if err != nil {
		logger.Error("invite service", slog.Any("err", err))
		os.Exit(1)
	}

	searchService := search.New(postgres.NewSearchStore(pool))
	directoryService := directory.New(postgres.NewDirectoryStore(pool))
	deploymentService, err := deployments.New(postgres.NewDeploymentStore(pool), logger)
	if err != nil {
		logger.Error("deployment service", slog.Any("err", err))
		os.Exit(1)
	}
	rulesService, err := rules.New(postgres.NewRuleStore(pool), logger)
	if err != nil {
		logger.Error("rules service", slog.Any("err", err))
		os.Exit(1)
	}

	var deploymentKMS providers.KMS
	if client := kms.New(cfg.KMSURL, cfg.KMSToken); client != nil {
		deploymentKMS = client
	}
	providerCipher, err := providers.NewCipherWithKMS(cfg.SecretsKey, deploymentKMS)
	if err != nil {
		logger.Error("providers cipher", slog.Any("err", err))
		os.Exit(1)
	}
	providerService, err := providers.New(postgres.NewProviderStore(pool), providerCipher, logger)
	if err != nil {
		logger.Error("provider service", slog.Any("err", err))
		os.Exit(1)
	}
	orgCryptoStore := postgres.NewOrgCryptoStore(pool)
	orgCryptoService := orgcrypto.New(orgCryptoStore, providerCipher)
	secretsKey := cfg.SecretsKey
	providerService.WithCipherResolver(func(ctx context.Context, orgID string) (*providers.Cipher, error) {
		cc, cerr := orgCryptoStore.Get(ctx, orgID)
		if errors.Is(cerr, domain.ErrNotFound) {
			return providerCipher, nil
		}
		if cerr != nil {
			return nil, cerr
		}
		if cc.Status != "active" {
			return nil, fmt.Errorf("%w: BYOK is disabled for this organisation", providers.ErrCryptoUnavailable)
		}
		token := ""
		if len(cc.TokenEnc) > 0 {
			if providerCipher == nil {
				return nil, fmt.Errorf("%w: no deployment key to decrypt the KMS token", providers.ErrCryptoUnavailable)
			}
			token, cerr = providerCipher.Decrypt(cc.TokenEnc)
			if cerr != nil {
				return nil, cerr
			}
		}
		return providers.NewCipherWithKMSKey(secretsKey, kms.New(cc.KMSURL, token), cc.KeyID)
	})
	var appCipher githubsetup.Cipher
	if providerCipher != nil {
		appCipher = providerCipher
	}
	githubAppService := githubsetup.New(postgres.NewGithubAppStore(pool), appCipher, cfg.APIURL, cfg.WebURL, cfg.GitHubAPIURL)

	usageService := usage.New(postgres.NewUsageStore(pool), cfg.PriceInputPer1K, cfg.PriceOutputPer1K)
	designService := design.New(postgres.NewDesignStore(pool), logger)

	analyticsService, err := analytics.New(postgres.NewAnalyticsStore(pool), logger)
	if err != nil {
		logger.Error("analytics service", slog.Any("err", err))
		os.Exit(1)
	}

	oidcAuthorize, oidcToken, oidcUserinfo := cfg.OIDCSSOAuthorizeURL, cfg.OIDCSSOTokenURL, cfg.OIDCSSOUserInfoURL
	if cfg.OIDCSSOIssuer != "" && (oidcAuthorize == "" || oidcToken == "" || oidcUserinfo == "") {
		disc, derr := oidc.Discover(ctx, cfg.OIDCSSOIssuer)
		if derr != nil {
			logger.Warn("oidc discovery failed; SSO provider disabled", slog.Any("err", derr))
			oidcAuthorize, oidcToken, oidcUserinfo = "", "", ""
		} else {
			oidcAuthorize, oidcToken, oidcUserinfo = disc.AuthorizationEndpoint, disc.TokenEndpoint, disc.UserinfoEndpoint
		}
	}

	oidcService, err := oidc.New([]oidc.Provider{
		//nolint:gosec // provider endpoints, credentials come from config
		{
			Name: "github", AuthorizeURL: "https://github.com/login/oauth/authorize",
			TokenURL:    "https://github.com/login/oauth/access_token",
			UserInfoURL: "https://api.github.com/user",
			EmailsURL:   "https://api.github.com/user/emails",
			ClientID:    cfg.OIDCGitHubClientID, ClientSecret: cfg.OIDCGitHubClientSecret,
			RedirectURL: cfg.APIURL + "/api/v1/auth/oidc/github/callback",
			Scopes:      []string{"read:user", "user:email"},
		},
		//nolint:gosec // provider endpoints, credentials come from config
		{
			Name: "google", AuthorizeURL: "https://accounts.google.com/o/oauth2/v2/auth",
			TokenURL:    "https://oauth2.googleapis.com/token",
			UserInfoURL: "https://openidconnect.googleapis.com/v1/userinfo",
			ClientID:    cfg.OIDCGoogleClientID, ClientSecret: cfg.OIDCGoogleClientSecret,
			RedirectURL: cfg.APIURL + "/api/v1/auth/oidc/google/callback",
			Scopes:      []string{"openid", "email", "profile"},
		},
		{
			Name:         cfg.OIDCSSOName,
			AuthorizeURL: oidcAuthorize,
			TokenURL:     oidcToken,
			UserInfoURL:  oidcUserinfo,
			ClientID:     cfg.OIDCSSOClientID, ClientSecret: cfg.OIDCSSOClientSecret,
			RedirectURL: cfg.APIURL + "/api/v1/auth/oidc/" + cfg.OIDCSSOName + "/callback",
			Scopes:      []string{"openid", "email", "profile"},
		},
	}, accounts)
	if err != nil {
		logger.Error("oidc service", slog.Any("err", err))
		os.Exit(1)
	}

	loginLimiter := ratelimit.Allower(ratelimit.New(5, time.Minute))
	registerLimiter := ratelimit.Allower(ratelimit.New(3, time.Hour))
	resetLimiter := ratelimit.Allower(ratelimit.New(3, time.Hour))
	var redisClient *redis.Client
	var samlReplay replay.Cache = replay.NewMemory()
	if cfg.RedisAddr != "" {
		redisClient = redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
		samlReplay = replay.NewRedis(redisClient)
		loginLimiter = ratelimit.NewRedis(redisClient, 5, time.Minute, "aip:rl:login")
		registerLimiter = ratelimit.NewRedis(redisClient, 3, time.Hour, "aip:rl:register")
		resetLimiter = ratelimit.NewRedis(redisClient, 3, time.Hour, "aip:rl:reset")
	}
	roleMap, rerr := parseRoleMap(cfg.SAMLRoleMap)
	if rerr != nil {
		logger.Error("saml role map", slog.Any("err", rerr))
		os.Exit(1)
	}
	var samlService *saml.Service
	if cfg.SAMLIDPMetadata != "" {
		samlService, err = saml.New(ctx, saml.Config{
			BaseURL: cfg.APIURL, EntityID: cfg.SAMLEntityID, IDPMetadata: cfg.SAMLIDPMetadata,
			KeyFile: cfg.SAMLKeyFile, Certificate: cfg.SAMLCertFile, WebURL: cfg.WebURL,
		})
		if err != nil {
			logger.Error("saml service", slog.Any("err", err))
			os.Exit(1)
		}
	}

	limits := httpapi.RateLimits{
		Login:    loginLimiter,
		Register: registerLimiter,
		Reset:    resetLimiter,
	}

	auditKey, keyErr := parseEd25519(cfg.AuditEd25519Key)
	if keyErr != nil {
		logger.Error("audit key", slog.Any("err", keyErr))
		os.Exit(1)
	}
	allowedIPs, ipErr := parseAllowlist(cfg.IPAllowlist)
	if ipErr != nil {
		logger.Error("ip allowlist", slog.Any("err", ipErr))
		os.Exit(1)
	}
	srv := &http.Server{
		Addr: cfg.HTTPAddr,
		Handler: httpapi.NewRouter(httpapi.Deps{
			Version: version, StartedAt: time.Now(), Log: logger, DB: pool, Readiness: writablePool{pool: pool},
			AllowedOrigins: []string{cfg.WebURL},
			Auth:           accounts, Projects: projectService, Tasks: taskService, Runs: runService, Repos: repoStore, Analytics: analyticsService, Blueprints: blueprintService, Invites: inviteService, Search: searchService, Providers: providerService, Prices: usageService, Design: designService, Directory: directoryService, Rules: rulesService, Deployments: deploymentService, Checks: checkService, Artifacts: artifactStore, Notifications: notificationService, OIDC: oidcService, WebURL: cfg.WebURL,
			Issuer: issuer, Limits: limits, RateLimitPerMin: cfg.RateLimitPerMin, Events: bus, Usage: usageService, Orgs: orgService, DesignStandards: designStandardService, Attachments: attachmentService, GitHubApps: githubAppService, TestCommand: cfg.TestCommand, AgentBackend: cfg.AgentBackend, GitHubToken: cfg.GitHubToken, AllowedIPs: allowedIPs, GitLabToken: cfg.GitLabToken, AuditSigningKey: cfg.AuditSigningKey, AuditEd25519: auditKey, Figma: figma.New(cfg.FigmaAPIURL, cfg.FigmaToken), ApprovalReauthMinutes: cfg.ApprovalReauthMinutes, Objects: baselineObjects, MergeQueue: mergeQueueService, Previews: previewService, ProjectAccess: projectAccessService, APIKeys: apiKeyService, AuditWitness: auditWitnessService, Incidents: incidents.New(postgres.NewIncidentStore(pool)), SCIM: scim.New(postgres.NewSCIMStore(pool), inviteService), SAML: samlService, SAMLAccounts: accounts, SAMLReplay: samlReplay, SAMLAdminGroups: splitList(cfg.SAMLAdminGroups), SAMLRoleMap: roleMap, SSO: sso.New(postgres.NewSSOStore(pool), cfg.SSOAllowPrivateMetadata), OrgCrypto: orgCryptoService, Region: cfg.Region, OrgRegion: postgres.NewOrgRegion(pool), WALStats: postgres.NewWALStats(pool), BackupStats: postgres.NewBackupStats(pool), AgentPlugins: agentplugins.New(postgres.NewAgentPluginStore(pool), cfg.PluginAllowedDigests, imageverify.New(cfg.CosignBin, cfg.CosignIdentity, cfg.CosignIssuer, cfg.CosignKey)), Restores: restores.New(postgres.NewRestoreStore(pool)), GitHubAppConfigured: cfg.GitHubAppID > 0, GitHubAppWebhookSecret: cfg.GitHubAppWebhookSecret,
		}),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("http listening", slog.String("addr", cfg.HTTPAddr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		logger.Error("http server", slog.Any("err", err))
		os.Exit(1)
	case <-ctx.Done():
		logger.Info("shutdown requested")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown", slog.Any("err", err))
	}
	logger.Info("stopped")
}

type previewScheduler struct {
	schedule func(ctx context.Context, previewID, orgID, action string, after time.Duration) error
}

func (p previewScheduler) Schedule(ctx context.Context, previewID, orgID, action string, after time.Duration) error {
	return p.schedule(ctx, previewID, orgID, action, after)
}

func parseEd25519(raw string) (ed25519.PrivateKey, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("AIP_AUDIT_ED25519_KEY must be base64: %w", err)
	}
	switch len(data) {
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(data), nil
	case ed25519.PrivateKeySize:
		return ed25519.PrivateKey(data), nil
	default:
		return nil, fmt.Errorf("AIP_AUDIT_ED25519_KEY must be %d or %d bytes", ed25519.SeedSize, ed25519.PrivateKeySize)
	}
}

func parseAllowlist(raw string) ([]netip.Prefix, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var out []netip.Prefix
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(part)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR %q: %w", part, err)
		}
		out = append(out, prefix)
	}
	return out, nil
}

func splitList(raw string) map[string]bool {
	out := map[string]bool{}
	for _, value := range strings.Split(raw, ",") {
		if value = strings.TrimSpace(value); value != "" {
			out[value] = true
		}
	}
	return out
}

func parseRoleMap(raw string) (map[string]string, error) {
	out := map[string]string{}
	for _, pair := range strings.Split(raw, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("AIP_SAML_ROLE_MAP entry %q must be group:role", pair)
		}
		group, role := strings.TrimSpace(parts[0]), strings.ToLower(strings.TrimSpace(parts[1]))
		if group == "" || (role != "admin" && role != "member" && role != "viewer") {
			return nil, fmt.Errorf("AIP_SAML_ROLE_MAP entry %q has an invalid group/role", pair)
		}
		out[group] = role
	}
	return out, nil
}
