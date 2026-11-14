// Package checksync polls git providers for CI results when webhooks are down.
package checksync

import (
	"context"
	"log/slog"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/checks"
)

// OpenPR is one open pull request to synchronise.
type OpenPR struct {
	ID       string
	OrgID    string
	RepoID   string
	RepoName string
	Provider string
	Number   int
}

// Check is a normalised provider check result.
type Check struct {
	Name       string
	Status     string
	Output     string
	DurationMS int
	CommitSHA  string
}

// Store lists open PRs.
type Store interface {
	ListOpen(ctx context.Context) ([]OpenPR, error)
}

// Recorder writes check results.
type Recorder interface {
	Record(ctx context.Context, orgID, repoID, prID, runID string, c checks.Check) (checks.Check, error)
}

// Lister is implemented per provider (GitHub first).
type Lister interface {
	ListChecks(ctx context.Context, orgID, repo string, number int) ([]Check, error)
}

// Syncer polls providers and records what it finds.
type Syncer struct {
	store   Store
	rec     Recorder
	listers map[string]Lister
	log     *slog.Logger
}

// New wires a syncer.
func New(store Store, rec Recorder, listers map[string]Lister, log *slog.Logger) *Syncer {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Syncer{store: store, rec: rec, listers: listers, log: log}
}

// Once syncs every open PR and returns how many checks were recorded.
func (s *Syncer) Once(ctx context.Context) (int, error) {
	prs, err := s.store.ListOpen(ctx)
	if err != nil {
		return 0, err
	}
	recorded := 0
	for _, pr := range prs {
		lister, ok := s.listers[pr.Provider]
		if !ok {
			continue
		}
		items, lerr := lister.ListChecks(ctx, pr.OrgID, pr.RepoName, pr.Number)
		if lerr != nil {
			s.log.Warn("checksync: list", slog.String("repo", pr.RepoName), slog.Int("pr", pr.Number), slog.Any("err", lerr))
			continue
		}
		for _, item := range items {
			if _, rerr := s.rec.Record(ctx, pr.OrgID, pr.RepoID, pr.ID, "", checks.Check{
				Name: item.Name, Status: item.Status, Output: item.Output,
				DurationMS: item.DurationMS, CommitSHA: item.CommitSHA,
			}); rerr != nil {
				s.log.Warn("checksync: record", slog.Any("err", rerr))
				continue
			}
			recorded++
		}
	}
	return recorded, nil
}

// Run polls immediately and then on the interval until ctx is done.
func (s *Syncer) Run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if n, err := s.Once(ctx); err != nil {
			s.log.Warn("checksync: pass", slog.Any("err", err))
		} else if n > 0 {
			s.log.Info("checksync: recorded", slog.Int("checks", n))
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
