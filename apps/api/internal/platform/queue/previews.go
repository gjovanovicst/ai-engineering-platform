package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
)

// PreviewArgs starts or stops one preview environment.
type PreviewArgs struct {
	PreviewID string `json:"preview_id"`
	OrgID     string `json:"org_id"`
	Action    string `json:"action"` // start | stop
}

// Kind implements river.JobArgs.
func (PreviewArgs) Kind() string { return "preview_environment" }

// EnqueuePreview schedules a start (immediately) or stop (after ttl).
func EnqueuePreview(ctx context.Context, client *river.Client[pgx.Tx], previewID, orgID, action string, after time.Duration) error {
	opts := &river.InsertOpts{}
	if action == "start" {
		opts.UniqueOpts = river.UniqueOpts{ByArgs: true}
	}
	if after > 0 {
		opts.ScheduledAt = time.Now().Add(after)
	}
	if _, err := client.Insert(ctx, PreviewArgs{PreviewID: previewID, OrgID: orgID, Action: action}, opts); err != nil {
		return fmt.Errorf("queue: enqueue preview %s: %w", previewID, err)
	}
	return nil
}
