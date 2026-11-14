package httpapi

import "context"

// WALArchiveStats exposes PostgreSQL archiver counters.
type WALArchiveStats interface {
	Stats(ctx context.Context) (failed, archived int64, lastAge float64, err error)
}
