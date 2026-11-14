// Package agentplugins registers per-org agent images and selects one per project.
package agentplugins

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// Plugin is a registered agent image contract.
type Plugin struct {
	ID         string
	Name       string
	Version    string
	Image      string
	Command    string
	AgentTypes []string
	CreatedAt  time.Time
}

// Store persists plugins and project selection.
type Store interface {
	Create(ctx context.Context, orgID, actorID, name, version, image, command string, agentTypes []string) (Plugin, error)
	List(ctx context.Context, orgID string) ([]Plugin, error)
	SetProjectPlugin(ctx context.Context, orgID, projectID, pluginID string) error
	ForProject(ctx context.Context, orgID, projectID string) (Plugin, error)
	Audit(ctx context.Context, orgID, actorID, action, target string, metadata []byte) error
}

// ImageVerifier verifies an image before registration.
type ImageVerifier interface {
	Verify(ctx context.Context, image string) error
}

// Service exposes plugin use cases.
type Service struct {
	store     Store
	allowlist map[string]bool
	verifier  ImageVerifier
}

// New wires the service. An empty allowlist allows any digest (development).
func New(store Store, allowedDigests []string, verifier ImageVerifier) *Service {
	allow := make(map[string]bool, len(allowedDigests))
	for _, digest := range allowedDigests {
		digest = strings.TrimSpace(digest)
		if digest != "" {
			allow[digest] = true
		}
	}
	return &Service{store: store, allowlist: allow, verifier: verifier}
}

func imageDigest(image string) string {
	if i := strings.LastIndex(image, "@"); i >= 0 {
		return image[i+1:]
	}
	return ""
}

var agentTypes = map[string]bool{"planning": true, "development": true, "review": true}

// Create validates and registers a plugin.
func (s *Service) Create(ctx context.Context, orgID, actorID, name, version, image, command string, types []string) (Plugin, error) {
	name = strings.TrimSpace(name)
	version = strings.TrimSpace(version)
	image = strings.TrimSpace(image)
	command = strings.TrimSpace(command)
	if len(name) < 2 || len(name) > 60 || len(version) < 1 || len(version) > 40 {
		return Plugin{}, fmt.Errorf("%w: name (2..60) and version (1..40) are required", domain.ErrValidation)
	}
	if image == "" || command == "" {
		return Plugin{}, fmt.Errorf("%w: image and command are required", domain.ErrValidation)
	}
	if !strings.Contains(image, "@sha256:") {
		return Plugin{}, fmt.Errorf("%w: image must be pinned by digest", domain.ErrValidation)
	}
	if len(s.allowlist) > 0 && !s.allowlist[imageDigest(image)] {
		return Plugin{}, fmt.Errorf("%w: image digest is not in AIP_PLUGIN_ALLOWED_DIGESTS", domain.ErrValidation)
	}
	if s.verifier != nil {
		if err := s.verifier.Verify(ctx, image); err != nil {
			return Plugin{}, fmt.Errorf("%w: image signature verification failed: %w", domain.ErrValidation, err)
		}
	}
	if len(types) == 0 {
		types = []string{"planning", "development", "review"}
	}
	seen := map[string]bool{}
	clean := make([]string, 0, len(types))
	for _, agentType := range types {
		agentType = strings.ToLower(strings.TrimSpace(agentType))
		if !agentTypes[agentType] {
			return Plugin{}, fmt.Errorf("%w: agent type must be planning, development or review", domain.ErrValidation)
		}
		if !seen[agentType] {
			seen[agentType] = true
			clean = append(clean, agentType)
		}
	}
	plugin, err := s.store.Create(ctx, orgID, actorID, name, version, image, command, clean)
	if err != nil {
		return Plugin{}, err
	}
	_ = s.store.Audit(ctx, orgID, actorID, "agent_plugin.created", plugin.ID, []byte(`{}`))
	return plugin, nil
}

// List returns registered plugins.
func (s *Service) List(ctx context.Context, orgID string) ([]Plugin, error) {
	return s.store.List(ctx, orgID)
}

// Select sets or clears the project's plugin.
func (s *Service) Select(ctx context.Context, orgID, projectID, pluginID, actorID string) error {
	if pluginID == "" {
		return s.store.SetProjectPlugin(ctx, orgID, projectID, "")
	}
	plugins, err := s.store.List(ctx, orgID)
	if err != nil {
		return err
	}
	for _, plugin := range plugins {
		if plugin.ID == pluginID {
			if err := s.store.SetProjectPlugin(ctx, orgID, projectID, pluginID); err != nil {
				return err
			}
			_ = s.store.Audit(ctx, orgID, actorID, "agent_plugin.selected", projectID, []byte(`{}`))
			return nil
		}
	}
	return domain.ErrNotFound
}

// ForProject returns the selected plugin or ErrNotFound.
func (s *Service) ForProject(ctx context.Context, orgID, projectID string) (Plugin, error) {
	return s.store.ForProject(ctx, orgID, projectID)
}
