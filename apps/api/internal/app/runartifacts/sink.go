// Package runartifacts uploads run artifacts to object storage and records them.
package runartifacts

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// Artifact is one recorded object.
type Artifact struct {
	Kind      string
	BucketKey string
	SHA256    []byte
	Bytes     int64
	CreatedAt time.Time
}

// Store records artifact metadata.
type Store interface {
	Upsert(ctx context.Context, orgID, runID, kind, key string, sha []byte, size int64) (Artifact, error)
	ListForRun(ctx context.Context, orgID, runID string) ([]Artifact, error)
}

// ObjectStore is the S3 Put surface (nil means "not configured").
type ObjectStore interface {
	Put(ctx context.Context, key string, body []byte, contentType string) error
}

// Sink uploads objects and records their metadata. Without an object store it is
// a no-op: the database keeps the artifact as before.
type Sink struct {
	objects ObjectStore
	store   Store
	log     *slog.Logger
}

// New wires the sink.
func New(objects ObjectStore, store Store, log *slog.Logger) *Sink {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Sink{objects: objects, store: store, log: log}
}

var contentTypes = map[string]string{
	"plan": "application/json", "diff": "text/x-diff", "test_report": "text/plain", "review": "application/json",
}

// Upload stores one artifact and upserts its row. Errors are returned so the
// caller can log them without failing the run.
func (s *Sink) Upload(ctx context.Context, orgID, runID, kind string, data []byte) error {
	if s == nil || s.objects == nil || s.store == nil || len(data) == 0 {
		return nil
	}
	sum := sha256.Sum256(data)
	key := fmt.Sprintf("runs/%s/%s/%s", orgID, runID, kind)
	contentType := contentTypes[kind]
	if contentType == "" && strings.HasPrefix(kind, "visual/") {
		contentType = "image/png"
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	putCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if err := s.objects.Put(putCtx, key, bytes.Clone(data), contentType); err != nil {
		return err
	}
	_, err := s.store.Upsert(ctx, orgID, runID, kind, key, sum[:], int64(len(data)))
	return err
}
