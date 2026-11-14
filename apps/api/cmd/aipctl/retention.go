package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/artifacts"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/retention"
)

// runRetention performs one retention pass (archive old events, purge expired
// auth rows). Intended for a CronJob or a manual run.
func runRetention() {
	adminDSN := os.Getenv("AIP_DB_ADMIN_DSN")
	if adminDSN == "" {
		adminDSN = os.Getenv("AIP_DB_MIGRATOR_DSN")
	}
	if adminDSN == "" {
		fmt.Fprintln(os.Stderr, "AIP_DB_ADMIN_DSN (or AIP_DB_MIGRATOR_DSN) is required")
		os.Exit(2)
	}
	days := 90
	if raw := os.Getenv("AIP_RETENTION_DAYS"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			days = n
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	store, err := artifacts.New(ctx,
		os.Getenv("AIP_S3_ENDPOINT"), os.Getenv("AIP_S3_ACCESS_KEY"), os.Getenv("AIP_S3_SECRET_KEY"),
		os.Getenv("AIP_S3_REGION"), os.Getenv("AIP_S3_BUCKET"), os.Getenv("AIP_S3_USE_SSL") == "true")
	if err != nil {
		fmt.Fprintf(os.Stderr, "artifacts: %v\n", err)
		os.Exit(1)
	}
	svc, err := retention.New(ctx, adminDSN, store, days, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "retention: %v\n", err)
		os.Exit(1)
	}
	defer svc.Close()

	stats, err := svc.Run(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "retention run: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("sessions=%d invites=%d resets=%d verifications=%d archived_runs=%d\n",
		stats.Sessions, stats.Invites, stats.Resets, stats.Verifications, stats.ArchivedRuns)
}
