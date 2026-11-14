package domain

import (
	"regexp"
	"strings"
	"time"
)

// Org is the tenant boundary. Every other aggregate belongs to exactly one org.
type Org struct {
	ID                string
	Slug              string
	Name              string
	MaxConcurrentRuns int
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,38}[a-z0-9]$`)

// NewOrg validates and normalizes a new organization.
func NewOrg(name, slug string) (*Org, error) {
	o := &Org{
		Name:              strings.TrimSpace(name),
		Slug:              strings.ToLower(strings.TrimSpace(slug)),
		MaxConcurrentRuns: 4,
	}
	if err := o.Validate(); err != nil {
		return nil, err
	}
	return o, nil
}

// Validate enforces the org invariants.
func (o *Org) Validate() error {
	if strings.TrimSpace(o.Name) == "" {
		return invalid("name", "must not be empty")
	}
	if len(o.Name) > 80 {
		return invalid("name", "must be at most 80 characters")
	}
	if !slugPattern.MatchString(o.Slug) {
		return invalid("slug", "must be 3-40 chars of lowercase letters, digits or dashes, starting and ending alphanumeric")
	}
	if o.MaxConcurrentRuns < 1 || o.MaxConcurrentRuns > 64 {
		return invalid("max_concurrent_runs", "must be between 1 and 64")
	}
	return nil
}
