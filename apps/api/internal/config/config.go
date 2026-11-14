// Package config loads configuration from the environment and validates it at startup.
// All variables use the AIP_ prefix; invalid configuration is a fatal
// error at startup, never a silent degradation.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

func getBool(key string, def bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return def
	}
	return value
}

func getInt(key string, def int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return n
}

// Config is the complete process configuration.
type Config struct {
	Env      string
	HTTPAddr string
	BaseURL  string
	WebURL   string
	LogLevel string

	DBDSN         string
	DBMigratorDSN string
	RedisAddr     string

	// JWTKeyPath points at the Ed25519 private key (PEM).
	JWTKeyPath            string
	AccessTTL             time.Duration
	RefreshTTL            time.Duration
	SessionIdleTTL        time.Duration
	ApprovalReauthMinutes int
	IPAllowlist           string
	SIEMWebhookURL        string
	SIEMWebhookSecret     string

	SMTPAddr string
	MailFrom string

	SandboxDriver           string
	SandboxImage            string
	SandboxNamespace        string
	AgentCommand            string
	TestCommand             string
	GitHubToken             string
	GitLabToken             string
	GiteaToken              string
	GiteaAPIURL             string
	GiteaCloneBase          string
	BitbucketToken          string
	BitbucketAPIURL         string
	BitbucketCloneBase      string
	GitHubAPIURL            string
	GitHubCloneBase         string
	GitLabAPIURL            string
	GitLabCloneBase         string
	SecretsKey              string
	AgentBackend            string
	AgentAdapter            string
	MCPConfig               string
	ClamAVAddr              string
	EmbeddingsURL           string
	EmbeddingsModel         string
	EmbeddingsKey           string
	WorkerMetricsAddr       string
	WorkerMaxWorkers        int
	OrgMaxConcurrentRuns    int
	PluginAllowedDigests    []string
	CosignBin               string
	CosignIdentity          string
	CosignIssuer            string
	CosignKey               string
	Region                  string
	AuditSigningKey         string
	AuditEd25519Key         string
	FigmaToken              string
	FigmaAPIURL             string
	KMSURL                  string
	SSOAllowPrivateMetadata bool
	SAMLIDPMetadata         string
	SAMLEntityID            string
	SAMLKeyFile             string
	SAMLCertFile            string
	SAMLAdminGroups         string
	SAMLRoleMap             string
	KMSToken                string
	PriceInputPer1K         float64
	PriceOutputPer1K        float64
	LLMMaxAttempts          int
	MaxActiveRuns           int
	RateLimitPerMin         int
	S3Endpoint              string
	S3Bucket                string
	S3AccessKey             string
	S3SecretKey             string
	S3Region                string
	S3UseSSL                bool
	AdminDSN                string
	GitHubAppID             int64
	GitHubAppKeyPath        string
	GitHubAppInstall        int64
	GitHubAppWebhookSecret  string
	RetentionDays           int
	RetentionHours          int
	OTELEndpoint            string

	// OIDC providers; empty client credentials disable the provider.
	APIURL                 string
	OIDCGitHubClientID     string
	OIDCGitHubClientSecret string
	OIDCGoogleClientID     string
	OIDCGoogleClientSecret string
	OIDCSSOName            string
	OIDCSSOIssuer          string
	OIDCSSOAuthorizeURL    string
	OIDCSSOTokenURL        string
	OIDCSSOUserInfoURL     string
	OIDCSSOClientID        string
	OIDCSSOClientSecret    string
}

// Load reads the environment and returns the configuration or an error listing every problem.
func Load() (*Config, error) {
	var problems []string

	getFloat := func(key string, def float64) float64 {
		raw := strings.TrimSpace(os.Getenv(key))
		if raw == "" {
			return def
		}
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return def
		}
		return value
	}
	get := func(key, def string) string {
		if v, ok := os.LookupEnv(key); ok && v != "" {
			return v
		}
		return def
	}
	require := func(key string) string {
		v := os.Getenv(key)
		if v == "" {
			problems = append(problems, key+" is required")
		}
		return v
	}
	duration := func(key string, def time.Duration) time.Duration {
		raw := os.Getenv(key)
		if raw == "" {
			return def
		}
		d, err := time.ParseDuration(raw)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: invalid duration %q", key, raw))
			return def
		}
		return d
	}

	cfg := &Config{
		Env:      get("AIP_ENV", "local"),
		HTTPAddr: get("AIP_HTTP_ADDR", ":8080"),
		BaseURL:  get("AIP_BASE_URL", "http://localhost:8080"),
		WebURL:   get("AIP_WEB_URL", "http://localhost:3000"),
		LogLevel: get("AIP_LOG_LEVEL", "info"),

		DBDSN:         require("AIP_DB_DSN"),
		DBMigratorDSN: os.Getenv("AIP_DB_MIGRATOR_DSN"),
		RedisAddr:     get("AIP_REDIS_ADDR", "localhost:6379"),

		JWTKeyPath:            get("AIP_JWT_KEY", "jwt.pem"),
		AccessTTL:             duration("AIP_ACCESS_TTL", 15*time.Minute),
		RefreshTTL:            duration("AIP_REFRESH_TTL", 720*time.Hour),
		SessionIdleTTL:        duration("AIP_SESSION_IDLE_TTL", 168*time.Hour),
		ApprovalReauthMinutes: getInt("AIP_APPROVAL_REAUTH_MINUTES", 15),
		IPAllowlist:           get("AIP_IP_ALLOWLIST", ""),
		SIEMWebhookURL:        get("AIP_SIEM_WEBHOOK_URL", ""),
		SIEMWebhookSecret:     get("AIP_SIEM_WEBHOOK_SECRET", ""),

		SMTPAddr: get("AIP_SMTP_ADDR", "localhost:1025"),
		MailFrom: get("AIP_MAIL_FROM", "no-reply@aip.local"),

		SandboxDriver:        get("AIP_SANDBOX_DRIVER", "docker"),
		SandboxImage:         get("AIP_SANDBOX_IMAGE", "aip/sandbox:go1.25-node22"),
		AgentCommand:         get("AIP_AGENT_COMMAND", "echo 'agent backend not configured'"),
		TestCommand:          get("AIP_TEST_COMMAND", ""),
		GitHubToken:          get("AIP_GITHUB_TOKEN", get("GITHUB_TOKEN", "")),
		GitLabToken:          get("AIP_GITLAB_TOKEN", get("GITLAB_TOKEN", "")),
		GitHubAPIURL:         get("AIP_GITHUB_API_URL", "https://api.github.com"),
		GitHubCloneBase:      get("AIP_GITHUB_CLONE_BASE", "https://github.com"),
		GitLabAPIURL:         get("AIP_GITLAB_API_URL", "https://gitlab.com"),
		GiteaToken:           get("AIP_GITEA_TOKEN", ""),
		GiteaAPIURL:          get("AIP_GITEA_API_URL", "https://gitea.com/api/v1"),
		GiteaCloneBase:       get("AIP_GITEA_CLONE_BASE", "https://gitea.com"),
		BitbucketToken:       get("AIP_BITBUCKET_TOKEN", ""),
		BitbucketAPIURL:      get("AIP_BITBUCKET_API_URL", "https://api.bitbucket.org/2.0"),
		BitbucketCloneBase:   get("AIP_BITBUCKET_CLONE_BASE", "https://bitbucket.org"),
		GitLabCloneBase:      get("AIP_GITLAB_CLONE_BASE", "https://gitlab.com"),
		SecretsKey:           get("AIP_SECRETS_KEY", ""),
		AgentBackend:         get("AIP_AGENT_BACKEND", "command"),
		AgentAdapter:         get("AIP_AGENT_ADAPTER", ""),
		MCPConfig:            get("AIP_MCP_SERVERS", ""),
		ClamAVAddr:           get("AIP_CLAMAV_ADDR", ""),
		EmbeddingsURL:        get("AIP_EMBEDDINGS_URL", ""),
		EmbeddingsModel:      get("AIP_EMBEDDINGS_MODEL", "text-embedding-3-small"),
		EmbeddingsKey:        get("AIP_EMBEDDINGS_KEY", ""),
		WorkerMetricsAddr:    get("AIP_WORKER_METRICS_ADDR", ":9090"),
		WorkerMaxWorkers:     getInt("AIP_WORKER_MAX_WORKERS", 4),
		OrgMaxConcurrentRuns: getInt("AIP_ORG_MAX_CONCURRENT_RUNS", 2),
		PluginAllowedDigests: strings.Split(os.Getenv("AIP_PLUGIN_ALLOWED_DIGESTS"), ","),
		CosignBin: func() string {
			if v := os.Getenv("AIP_COSIGN_BIN"); v != "" {
				return v
			}
			return "cosign"
		}(),
		CosignIdentity:          os.Getenv("AIP_COSIGN_IDENTITY"),
		CosignIssuer:            os.Getenv("AIP_COSIGN_ISSUER"),
		CosignKey:               os.Getenv("AIP_COSIGN_KEY"),
		Region:                  os.Getenv("AIP_REGION"),
		AuditSigningKey:         get("AIP_AUDIT_SIGNING_KEY", ""),
		AuditEd25519Key:         get("AIP_AUDIT_ED25519_KEY", ""),
		FigmaToken:              get("AIP_FIGMA_TOKEN", ""),
		FigmaAPIURL:             get("AIP_FIGMA_API_URL", ""),
		KMSURL:                  get("AIP_KMS_URL", ""),
		SSOAllowPrivateMetadata: getBool("AIP_SSO_ALLOW_PRIVATE_METADATA", false),
		SAMLIDPMetadata:         get("AIP_SAML_IDP_METADATA", ""),
		SAMLEntityID:            get("AIP_SAML_ENTITY_ID", "aip-saml"),
		SAMLKeyFile:             get("AIP_SAML_KEY_FILE", ""),
		SAMLCertFile:            get("AIP_SAML_CERT_FILE", ""),
		SAMLAdminGroups:         get("AIP_SAML_ADMIN_GROUPS", ""),
		SAMLRoleMap:             get("AIP_SAML_ROLE_MAP", ""),
		KMSToken:                get("AIP_KMS_TOKEN", ""),
		PriceInputPer1K:         getFloat("AIP_PRICE_INPUT_PER_1K", 0),
		PriceOutputPer1K:        getFloat("AIP_PRICE_OUTPUT_PER_1K", 0),
		LLMMaxAttempts:          3,
		MaxActiveRuns:           5,
		RateLimitPerMin:         600,
		S3Endpoint:              get("AIP_S3_ENDPOINT", ""),
		S3Bucket:                get("AIP_S3_BUCKET", "aip-artifacts"),
		S3AccessKey:             get("AIP_S3_ACCESS_KEY", "minio"),
		S3SecretKey:             get("AIP_S3_SECRET_KEY", "minio123"),
		S3Region:                get("AIP_S3_REGION", "eu-central-1"),
		S3UseSSL:                get("AIP_S3_USE_SSL", "false") == "true",
		AdminDSN:                get("AIP_DB_ADMIN_DSN", ""),
		GitHubAppKeyPath:        get("AIP_GITHUB_APP_KEY_PATH", ""),
		GitHubAppWebhookSecret:  get("AIP_GITHUB_APP_WEBHOOK_SECRET", ""),
		RetentionDays:           90,
		RetentionHours:          24,
		OTELEndpoint:            get("AIP_OTEL_ENDPOINT", ""),
		APIURL:                  get("AIP_API_URL", "http://localhost:8080"),
		OIDCGitHubClientID:      get("AIP_OIDC_GITHUB_CLIENT_ID", ""),
		OIDCGitHubClientSecret:  get("AIP_OIDC_GITHUB_CLIENT_SECRET", ""),
		OIDCGoogleClientID:      get("AIP_OIDC_GOOGLE_CLIENT_ID", ""),
		OIDCGoogleClientSecret:  get("AIP_OIDC_GOOGLE_CLIENT_SECRET", ""),
		OIDCSSOName:             get("AIP_OIDC_SSO_NAME", "sso"),
		OIDCSSOIssuer:           get("AIP_OIDC_SSO_ISSUER", ""),
		OIDCSSOAuthorizeURL:     get("AIP_OIDC_SSO_AUTHORIZE_URL", ""),
		OIDCSSOTokenURL:         get("AIP_OIDC_SSO_TOKEN_URL", ""),
		OIDCSSOUserInfoURL:      get("AIP_OIDC_SSO_USERINFO_URL", ""),
		OIDCSSOClientID:         get("AIP_OIDC_SSO_CLIENT_ID", ""),
		OIDCSSOClientSecret:     get("AIP_OIDC_SSO_CLIENT_SECRET", ""),
	}

	if raw := strings.TrimSpace(os.Getenv("AIP_MAX_ACTIVE_RUNS")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 100 {
			cfg.MaxActiveRuns = n
		}
	}
	if raw := strings.TrimSpace(os.Getenv("AIP_RATE_LIMIT_PER_MINUTE")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 100000 {
			cfg.RateLimitPerMin = n
		}
	}
	if raw := strings.TrimSpace(os.Getenv("AIP_GITHUB_APP_ID")); raw != "" {
		if n, err := strconv.ParseInt(raw, 10, 64); err == nil {
			cfg.GitHubAppID = n
		}
	}
	if raw := strings.TrimSpace(os.Getenv("AIP_GITHUB_APP_INSTALLATION_ID")); raw != "" {
		if n, err := strconv.ParseInt(raw, 10, 64); err == nil {
			cfg.GitHubAppInstall = n
		}
	}
	if raw := strings.TrimSpace(os.Getenv("AIP_RETENTION_DAYS")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			cfg.RetentionDays = n
		}
	}
	if raw := strings.TrimSpace(os.Getenv("AIP_RETENTION_INTERVAL_HOURS")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			cfg.RetentionHours = n
		}
	}
	if raw := strings.TrimSpace(os.Getenv("AIP_LLM_MAX_ATTEMPTS")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 10 {
			cfg.LLMMaxAttempts = n
		}
	}
	if cfg.Env == "prod" && cfg.JWTKeyPath == "jwt.pem" {
		problems = append(problems, "AIP_JWT_KEY must be set explicitly in prod")
	}
	if _, err := strconv.Atoi(strings.TrimPrefix(cfg.HTTPAddr, ":")); err != nil {
		problems = append(problems, fmt.Sprintf("AIP_HTTP_ADDR: invalid address %q", cfg.HTTPAddr))
	}
	if raw := strings.TrimSpace(os.Getenv("AIP_SANDBOX_EGRESS_ALLOW")); raw != "" {
		problems = append(problems, "AIP_SANDBOX_EGRESS_ALLOW is not implemented; remove it (the sandbox network is disabled)")
	}

	if len(problems) > 0 {
		return nil, errors.New("invalid configuration:\n  - " + strings.Join(problems, "\n  - "))
	}
	return cfg, nil
}

// Redacted returns a safe representation for logging.
func (c *Config) Redacted() string {
	mask := func(v string) string {
		if v == "" {
			return "(empty)"
		}
		return "***"
	}
	return fmt.Sprintf("env=%s http=%s web=%s db=%s redis=%s jwt=%s",
		c.Env, c.HTTPAddr, c.WebURL, mask(c.DBDSN), c.RedisAddr, c.JWTKeyPath)
}
