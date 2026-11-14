// Package retention archives old run events and removes expired auth rows.
package retention

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/artifacts"
)

// Stats reports what one retention pass did.
type Stats struct {
	Sessions      int64
	Invites       int64
	Resets        int64
	Verifications int64
	ArchivedRuns  int
}

// Service runs retention with a privileged connection (RLS bypass is required
// to clean rows across tenants).
type Service struct {
	pool      *pgxpool.Pool
	artifacts *artifacts.Store
	days      int
	log       *slog.Logger
}

// New opens the admin pool. A nil artifacts store still allows the purge.
func New(ctx context.Context, adminDSN string, store *artifacts.Store, days int, log *slog.Logger) (*Service, error) {
	if adminDSN == "" {
		return nil, fmt.Errorf("retention: AIP_DB_ADMIN_DSN is required")
	}
	pool, err := pgxpool.New(ctx, adminDSN)
	if err != nil {
		return nil, fmt.Errorf("retention: pool: %w", err)
	}
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	if days <= 0 {
		days = 90
	}
	return &Service{pool: pool, artifacts: store, days: days, log: log}, nil
}

// Close releases the admin pool.
func (s *Service) Close() { s.pool.Close() }

// Run performs one pass. Events are archived, never deleted (append-only).
func (s *Service) Run(ctx context.Context) (Stats, error) {
	var stats Stats
	var err error
	if stats.Sessions, err = s.exec(ctx, `delete from sessions s
		where s.expires_at < now() - interval '30 days'
		  and not exists (
			select 1 from memberships m join orgs o on o.id = m.org_id
			where m.user_id = s.user_id and o.legal_hold
		  )`); err != nil {
		return stats, err
	}
	if stats.Invites, err = s.exec(ctx, `delete from invites i
		where i.expires_at < now() - interval '30 days'
		  and not exists (select 1 from orgs o where o.id = i.org_id and o.legal_hold)`); err != nil {
		return stats, err
	}
	if stats.Resets, err = s.exec(ctx, `delete from password_resets p
		where p.used_at is not null and p.used_at < now() - interval '30 days'
		  and not exists (
			select 1 from memberships m join orgs o on o.id = m.org_id
			where m.user_id = p.user_id and o.legal_hold
		  )`); err != nil {
		return stats, err
	}
	if stats.Verifications, err = s.exec(ctx, `delete from email_verifications e
		where e.used_at is not null and e.used_at < now() - interval '30 days'
		  and not exists (
			select 1 from memberships m join orgs o on o.id = m.org_id
			where m.user_id = e.user_id and o.legal_hold
		  )`); err != nil {
		return stats, err
	}

	if s.artifacts == nil {
		s.log.Info("retention: event archiving disabled (AIP_S3_ENDPOINT is empty)")
		return stats, nil
	}
	archived, err := s.archiveEvents(ctx)
	stats.ArchivedRuns = archived
	return stats, err
}

func (s *Service) exec(ctx context.Context, sql string) (int64, error) {
	tag, err := s.pool.Exec(ctx, sql)
	if err != nil {
		return 0, fmt.Errorf("retention: %w", err)
	}
	return tag.RowsAffected(), nil
}

// archiveEvents writes one gzipped NDJSON object per run (events/<org>/<run>).
func (s *Service) archiveEvents(ctx context.Context) (int, error) {
	rows, err := s.pool.Query(ctx, `
		select r.org_id::text, e.run_id::text, e.seq, e.type, e.actor, e.payload, e.created_at
		from run_events e
		join runs r on r.id = e.run_id
		join orgs o on o.id = r.org_id
		where e.created_at < now() - make_interval(days => $1)
		  and o.legal_hold = false
		order by r.org_id, e.run_id, e.seq`, s.days)
	if err != nil {
		return 0, fmt.Errorf("retention: query events: %w", err)
	}
	defer rows.Close()

	var (
		currentOrg, currentRun string
		buf                    bytes.Buffer
		gz                     *gzip.Writer
		runs                   int
	)
	flush := func() error {
		if currentRun == "" {
			return nil
		}
		if err := gz.Close(); err != nil {
			return err
		}
		key := fmt.Sprintf("events/%s/%s.ndjson.gz", currentOrg, currentRun)
		putCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
		if err := s.artifacts.Put(putCtx, key, bytes.NewReader(buf.Bytes()), int64(buf.Len()), "application/gzip"); err != nil {
			return err
		}
		runs++
		buf.Reset()
		currentRun = ""
		return nil
	}

	for rows.Next() {
		var org, run, typ, actor string
		var seq int64
		var payload []byte
		var createdAt time.Time
		if err := rows.Scan(&org, &run, &seq, &typ, &actor, &payload, &createdAt); err != nil {
			return runs, fmt.Errorf("retention: scan event: %w", err)
		}
		if org != currentOrg || run != currentRun {
			if err := flush(); err != nil {
				return runs, err
			}
			currentOrg = org
			currentRun = run
			gz = gzip.NewWriter(&buf)
		}
		line, err := json.Marshal(map[string]any{
			"seq": seq, "type": typ, "actor": actor, "payload": json.RawMessage(payload),
			"created_at": createdAt.UTC().Format(time.RFC3339),
		})
		if err != nil {
			return runs, err
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	if err := rows.Err(); err != nil {
		return runs, err
	}
	if err := flush(); err != nil {
		return runs, err
	}
	s.log.Info("retention: archived runs", slog.Int("runs", runs), slog.Int("after_days", s.days))
	return runs, nil
}
