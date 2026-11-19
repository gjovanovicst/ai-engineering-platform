// Package httpapi contains HTTP routing and middleware.
// It moves to chi/v5 in E1-S7; until then the stdlib mux is sufficient.
package httpapi

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/netip"
	"strconv"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/saml"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/events"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/figma"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/metrics"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/replay"
)

// Pinger is implemented by anything that can verify its own liveness (the pool).
type Pinger interface {
	Ping(ctx context.Context) error
}

// Writable reports whether the database accepts writes (not a read-only replica).
type Writable interface {
	Writable(ctx context.Context) error
}

// Deps are the router dependencies.
type Deps struct {
	Version                string
	StartedAt              time.Time
	Log                    *slog.Logger
	DB                     Pinger
	Readiness              Writable
	AllowedOrigins         []string
	Auth                   AccountService
	Projects               ProjectService
	Tasks                  TaskService
	Runs                   RunService
	Repos                  RepoService
	RepoCredentials        RepoCredentialService
	Analytics              AnalyticsService
	Blueprints             BlueprintService
	WebURL                 string
	Invites                InviteService
	OIDC                   OIDCService
	Search                 SearchService
	Directory              DirectoryService
	Rules                  RulesService
	Deployments            DeploymentService
	Checks                 CheckService
	RateLimitPerMin        int
	Artifacts              ArtifactLister
	Notifications          NotificationService
	Events                 *events.Bus
	Usage                  UsageService
	Orgs                   OrgService
	DesignStandards        DesignStandardService
	Attachments            AttachmentService
	TestCommand            string
	AgentBackend           string
	GitHubAppConfigured    bool
	GitHubApps             GitHubAppService
	GitHubToken            string
	GitLabToken            string
	AuditSigningKey        string
	AuditEd25519           ed25519.PrivateKey
	Figma                  *figma.Client
	Incidents              IncidentService
	Assistant              AssistantService
	SCIM                   SCIMService
	SAML                   *saml.Service
	SAMLAccounts           SAMLAccounts
	SAMLReplay             replay.Cache
	SAMLAdminGroups        map[string]bool
	SAMLRoleMap            map[string]string
	SSO                    SSOService
	OrgCrypto              OrgCryptoService
	AgentPlugins           AgentPluginService
	Region                 string
	OrgRegion              OrgRegionReader
	WALStats               WALArchiveStats
	BackupStats            BackupStats
	Restores               RestoreService
	Objects                ObjectStore
	MergeQueue             MergeQueueReader
	Previews               PreviewService
	ProjectAccess          ProjectAccessService
	ApprovalReauthMinutes  int
	AllowedIPs             []netip.Prefix
	APIKeys                APIKeyService
	AuditWitness           AuditWitnessService
	GitHubAppWebhookSecret string
	Providers              ProviderService
	Prices                 PriceService
	Design                 DesignService
	Issuer                 *auth.Issuer
	Limits                 RateLimits
}

// NewRouter returns an HTTP handler with the infrastructure routes.
func NewRouter(d Deps) http.Handler {
	mux := http.NewServeMux()
	metricRegistry := metrics.New()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})

	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if d.DB == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{
				"status": "not ready", "reason": "database not configured",
			})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := d.DB.Ping(ctx); err != nil {
			d.Log.Warn("readyz: database ping failed", slog.Any("err", err))
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{
				"status": "not ready", "reason": "database unreachable",
			})
			return
		}
		if d.Readiness != nil {
			if err := d.Readiness.Writable(ctx); err != nil {
				d.Log.Warn("readyz: database is not writable", slog.Any("err", err))
				writeJSON(w, http.StatusServiceUnavailable, map[string]any{
					"status": "not ready", "reason": "database is not writable",
				})
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ready"})
	})

	assistantH := &assistantHandlers{svc: d.Assistant, issuer: d.Issuer, log: d.Log}
	mux.HandleFunc("POST /api/v1/orgs/{org}/assistant/messages", assistantH.send)
	mux.HandleFunc("GET /api/v1/orgs/{org}/assistant/conversations", assistantH.conversations)
	mux.HandleFunc("GET /api/v1/orgs/{org}/assistant/conversations/{conversation}", assistantH.messages)
	mux.HandleFunc("GET /api/v1/version", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"version": d.Version,
			"uptime":  time.Since(d.StartedAt).Round(time.Second).String(),
		})
	})

	if d.Auth != nil {
		newAuthHandlers(d.Auth, d.Issuer, d.Limits, d.Log).registerRoutes(mux)
	}
	if d.Projects != nil {
		newProjectHandlers(d.Projects, d.Repos, d.Issuer, d.TestCommand, d.AgentBackend, d.GitHubToken, d.GitLabToken, d.GitHubApps, d.Objects, d.MergeQueue, d.ProjectAccess, d.GitHubAppConfigured, d.Log).withRepoTokens(d.RepoCredentials).registerRoutes(mux)
	}
	if d.Tasks != nil {
		newTaskHandlers(d.Tasks, d.Projects, d.Issuer, d.Log).registerRoutes(mux)
	}
	if d.Runs != nil {
		newRunHandlers(d.Runs, d.Projects, d.Tasks, d.Issuer, d.Log, d.Events, d.ProjectAccess, time.Duration(d.ApprovalReauthMinutes)*time.Minute).registerRoutes(mux)
	}
	if d.Repos != nil {
		repos := newRepoHandlers(d.Repos, d.RepoCredentials, d.Projects, d.Issuer, d.Log)
		repos.registerRoutes(mux)
		if d.RepoCredentials != nil {
			repos.registerCredentialRoutes(mux)
		}
		newWebhookHandlers(d.Repos, d.Tasks, d.Checks, nil, d.Log).
			withAppSecret(d.GitHubAppWebhookSecret).
			withGitHubApps(d.GitHubApps).registerRoutes(mux)
	}
	if d.Analytics != nil {
		newAnalyticsHandlers(d.Analytics, d.Issuer, d.Log).registerRoutes(mux)
	}
	if d.Blueprints != nil {
		newBlueprintHandlers(d.Blueprints, d.Issuer, d.Log).registerRoutes(mux)
	}
	if d.Invites != nil {
		newInviteHandlers(d.Invites, d.Issuer, d.Log).registerRoutes(mux)
	}
	if d.OIDC != nil {
		newOIDCHandlers(d.OIDC, d.WebURL, d.Log).registerRoutes(mux)
	}
	if d.Search != nil {
		newSearchHandlers(d.Search, d.Issuer, d.Log).registerRoutes(mux)
	}
	if d.Directory != nil {
		newDirectoryHandlers(d.Directory, d.Issuer, d.Log, d.AuditSigningKey, d.AuditEd25519).registerRoutes(mux)
	}
	if d.Rules != nil {
		newRuleHandlers(d.Rules, d.Issuer, d.Log).registerRoutes(mux)
	}
	if d.Deployments != nil {
		newDeploymentHandlers(d.Deployments, d.Projects, d.Issuer, d.Log).registerRoutes(mux)
	}
	if d.Notifications != nil {
		newNotificationHandlers(d.Notifications, d.Issuer, d.Log).registerRoutes(mux)
	}
	if d.DesignStandards != nil {
		newDesignStandardHandlers(d.DesignStandards, d.Issuer, d.Log).registerRoutes(mux)
	}
	if d.Orgs != nil {
		newOrgHandlers(d.Orgs, d.Issuer, d.Log).registerRoutes(mux)
	}
	if d.Usage != nil {
		newUsageHandlers(d.Usage, d.Issuer, d.Log).registerRoutes(mux)
	}
	if d.Artifacts != nil {
		newArtifactHandlers(d.Artifacts, d.Issuer, d.Objects, d.Log).registerRoutes(mux)
	}
	if d.ProjectAccess != nil {
		(&projectMemberHandlers{svc: d.ProjectAccess, issuer: d.Issuer}).registerRoutes(mux)
	}
	if d.AgentPlugins != nil {
		(&agentPluginHandlers{svc: d.AgentPlugins, issuer: d.Issuer, log: d.Log}).registerRoutes(mux)
	}
	if d.Restores != nil {
		(&restoreHandlers{svc: d.Restores, issuer: d.Issuer, log: d.Log}).registerRoutes(mux)
	}
	if d.OrgCrypto != nil {
		(&orgCryptoHandlers{svc: d.OrgCrypto, issuer: d.Issuer, log: d.Log}).registerRoutes(mux)
	}
	if d.SAML != nil {
		(&samlHandlers{svc: d.SAML, accounts: d.SAMLAccounts, adminGroups: d.SAMLAdminGroups, roleMap: d.SAMLRoleMap, log: d.Log, replay: d.SAMLReplay}).registerRoutes(mux)
	}
	if d.SSO != nil {
		(&ssoHandlers{svc: d.SSO, issuer: d.Issuer, log: d.Log}).registerRoutes(mux)
	}
	if d.SCIM != nil {
		(&scimHandlers{svc: d.SCIM, issuer: d.Issuer, log: d.Log}).registerRoutes(mux)
	}
	if d.Incidents != nil {
		(&incidentHandlers{svc: d.Incidents, issuer: d.Issuer, log: d.Log}).registerRoutes(mux)
	}
	if d.AuditWitness != nil {
		(&auditWitnessHandlers{svc: d.AuditWitness, issuer: d.Issuer, log: d.Log}).registerRoutes(mux)
	}
	if d.APIKeys != nil {
		newAPIKeyHandlers(d.APIKeys, d.Issuer, d.Log).registerRoutes(mux)
	}
	if d.Previews != nil {
		newPreviewHandlers(d.Previews, d.Issuer, d.Log).registerRoutes(mux)
	}
	if d.GitHubApps != nil {
		newGithubAppHandlers(d.GitHubApps, d.Issuer, d.Log).registerRoutes(mux)
	}
	if d.Attachments != nil {
		newAttachmentHandlers(d.Attachments, d.Issuer, d.Log).registerRoutes(mux)
	}
	if d.Checks != nil {
		newCheckHandlers(d.Checks, d.Issuer, d.Log).registerRoutes(mux)
	}
	if d.Providers != nil {
		newProviderHandlers(d.Providers, d.Issuer, d.Log).registerRoutes(mux)
	}
	if d.Prices != nil {
		newPriceHandlers(d.Prices, d.Issuer, d.Log).registerRoutes(mux)
	}
	if d.Design != nil {
		newDesignHandlers(d.Design, d.Issuer, d.Log, d.Figma).registerRoutes(mux)
	}

	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		metricRegistry.Write(w, d.Version)
		if d.BackupStats != nil {
			if age, err := d.BackupStats.LastBaseBackupAge(r.Context()); err == nil {
				_, _ = fmt.Fprintf(w, "# HELP aip_base_backup_last_success_age_seconds Seconds since the last base backup\n# TYPE aip_base_backup_last_success_age_seconds gauge\naip_base_backup_last_success_age_seconds %.3f\n", age)
			}
		}
		if d.BackupStats != nil {
			if age, err := d.BackupStats.LastOffsiteAge(r.Context()); err == nil {
				_, _ = fmt.Fprintf(w, "# HELP aip_offsite_backup_last_success_age_seconds Seconds since the last offsite mirror\n# TYPE aip_offsite_backup_last_success_age_seconds gauge\naip_offsite_backup_last_success_age_seconds %.3f\n", age)
			}
			if age, err := d.BackupStats.LastRestoreAge(r.Context()); err == nil {
				_, _ = fmt.Fprintf(w, "# HELP aip_restore_last_success_age_seconds Seconds since the last successful restore\n# TYPE aip_restore_last_success_age_seconds gauge\naip_restore_last_success_age_seconds %.3f\n", age)
			}
		}
		if d.WALStats != nil {
			failed, archived, lastAge, err := d.WALStats.Stats(r.Context())
			if err == nil {
				_, _ = fmt.Fprintf(w, "# HELP aip_wal_archive_failed_total WAL archive failures\n# TYPE aip_wal_archive_failed_total counter\naip_wal_archive_failed_total %d\n", failed)
				_, _ = fmt.Fprintf(w, "# HELP aip_wal_archive_archived_total WAL segments archived\n# TYPE aip_wal_archive_archived_total counter\naip_wal_archive_archived_total %d\n", archived)
				_, _ = fmt.Fprintf(w, "# HELP aip_wal_archive_last_success_age_seconds Seconds since the last archived WAL\n# TYPE aip_wal_archive_last_success_age_seconds gauge\naip_wal_archive_last_success_age_seconds %.3f\n", lastAge)
			}
		}
	})

	var root http.Handler = mux
	if d.Region != "" && d.OrgRegion != nil {
		root = withRegionGuard(root, d.Issuer, d.Region, d.OrgRegion)
	}
	limited := withOrgRateLimit(root, d.Issuer, d.RateLimitPerMin)
	limited = withIPAllowlist(limited, d.AllowedIPs)
	limited = withAPIKeyAuth(limited, d.APIKeys)
	limited = withProjectRBAC(limited, d.Issuer, d.ProjectAccess)
	logged := withRequestLog(limited, d.Log)
	return withSecurityHeaders(withTracing(withCORS(withRecovery(withMetrics(logged, metricRegistry), d.Log), d.AllowedOrigins)))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// withMetrics records status and duration for every non-metrics request.
func withMetrics(next http.Handler, registry *metrics.Registry) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		registry.Observe(r.Method, strconv.Itoa(recorder.status), time.Since(start))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// Flush keeps Server-Sent Events working through the metrics middleware.
// Without it the run event stream would buffer silently and the UI would never
// see the events after the first buffer fills (for example await_approval).
func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
