package domain

import (
	"strings"
	"time"
)

// ProjectStatus is the lifecycle of a project.
type ProjectStatus string

const (
	ProjectActive   ProjectStatus = "active"
	ProjectArchived ProjectStatus = "archived"
)

// Project is an instance of a blueprint: a repository, a backlog and the
// environments it deploys to.
type Project struct {
	ID                      string
	OrgID                   string
	Name                    string
	Slug                    string
	Description             string
	BlueprintID             string
	BlueprintVersion        string
	DesignStandardID        string
	DesignStandardVersion   int
	RequiredChecks          []string
	E2ECommand              string
	TestCommand             string
	VisualCommand           string
	MCPServers              string
	VisualThreshold         float64
	VisualMasks             string
	AgentPluginID           *string
	PolicyMaxDiffBytes      int
	PolicyForbiddenPaths    []string
	PolicyChangeWindow      string
	PolicyAllowedModels     []string
	PolicyAllowedTools      []string
	PolicyRequiredApprovals int
	PluginChecks            string
	Status                  ProjectStatus
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

// NewProject validates a new project. The blueprint is optional at creation
// time only for projects imported from an existing repository.
func NewProject(orgID, name, slug string) (*Project, error) {
	p := &Project{
		OrgID:  orgID,
		Name:   strings.TrimSpace(name),
		Slug:   strings.ToLower(strings.TrimSpace(slug)),
		Status: ProjectActive,
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return p, nil
}

// Validate enforces the project invariants.
func (p *Project) Validate() error {
	if p.OrgID == "" {
		return invalid("org_id", "must not be empty")
	}
	if strings.TrimSpace(p.Name) == "" {
		return invalid("name", "must not be empty")
	}
	if len(p.Name) > 120 {
		return invalid("name", "must be at most 120 characters")
	}
	if !slugPattern.MatchString(p.Slug) {
		return invalid("slug", "must be 3-40 chars of lowercase letters, digits or dashes")
	}
	switch p.Status {
	case ProjectActive, ProjectArchived:
	default:
		return invalid("status", "must be active or archived")
	}
	return nil
}
