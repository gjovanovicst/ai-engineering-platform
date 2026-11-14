package previews

import (
	"context"

	"github.com/riverqueue/river"

	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/queue"
)

// Worker processes preview start/stop jobs.
type Worker struct {
	river.WorkerDefaults[queue.PreviewArgs]
	Service *Service
}

// Work implements river.Worker.
func (w Worker) Work(ctx context.Context, job *river.Job[queue.PreviewArgs]) error {
	if job.Args.Action == "stop" {
		return w.Service.Destroy(ctx, job.Args.OrgID, job.Args.PreviewID)
	}
	return w.Service.Start(ctx, job.Args.OrgID, job.Args.PreviewID)
}
