// Package attachments stores task files (wireframes, screenshots, specs).
package attachments

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"path/filepath"
	"strings"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// Meta is an attachment without its bytes.
type Meta struct {
	ID          string
	Filename    string
	ContentType string
	SizeBytes   int
	SHA256      string
	CreatedAt   string
}

// Attachment is metadata plus bytes.
type Attachment struct {
	Meta
	Data      []byte
	ObjectKey string
}

// CreateInput is one upload.
type CreateInput struct {
	Filename    string
	ContentType string
	Data        []byte
	ObjectKey   string
	SizeBytes   int
}

// ObjectStore stores attachment bytes outside Postgres (S3/MinIO).
type ObjectStore interface {
	Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error
	Get(ctx context.Context, key string) ([]byte, error)
}

// Store persists attachments.
type Store interface {
	Create(ctx context.Context, orgID, projectID, taskID, userID string, in CreateInput, sha string) (Meta, error)
	List(ctx context.Context, orgID, taskID string) ([]Meta, error)
	Get(ctx context.Context, orgID, taskID, id string) (Attachment, error)
}

// Service validates uploads.
// Scanner checks file bytes for malware; nil means scanning is disabled.
type Scanner interface {
	Scan(ctx context.Context, data []byte) error
}

type Service struct {
	store       Store
	scanner     Scanner
	objectStore ObjectStore
}

// New wires the service.
func New(store Store) *Service {
	return &Service{store: store}
}

// WithObjectStore moves attachment bytes to S3; metadata stays in Postgres.
func (s *Service) WithObjectStore(store ObjectStore) *Service {
	s.objectStore = store
	return s
}

// WithScanner enables malware scanning for uploads.
func (s *Service) WithScanner(scanner Scanner) *Service {
	s.scanner = scanner
	return s
}

const MaxBytes = 5 << 20 // 5 MiB

var allowed = map[string]bool{
	"image/png": true, "image/jpeg": true, "image/webp": true,
	"application/pdf": true, "text/plain": true,
}

// Upload validates and stores one attachment.
func (s *Service) Upload(ctx context.Context, orgID, projectID, taskID, userID, filename, contentType string, data []byte) (Meta, error) {
	filename = filepath.Base(strings.TrimSpace(filename))
	if filename == "" || filename == "." || len(filename) > 160 {
		return Meta{}, fmt.Errorf("%w: a filename is required (max 160)", domain.ErrValidation)
	}
	if len(data) == 0 {
		return Meta{}, fmt.Errorf("%w: the file is empty", domain.ErrValidation)
	}
	if len(data) > MaxBytes {
		return Meta{}, fmt.Errorf("%w: the file exceeds 5 MiB", domain.ErrValidation)
	}
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if parsed, _, err := mime.ParseMediaType(contentType); err == nil {
		contentType = parsed
	}
	if !allowed[contentType] {
		return Meta{}, fmt.Errorf("%w: unsupported file type %q (png, jpeg, webp, pdf, txt)", domain.ErrValidation, contentType)
	}
	if s.scanner != nil {
		if err := s.scanner.Scan(ctx, data); err != nil {
			return Meta{}, fmt.Errorf("%w: malware scan failed: %w", domain.ErrValidation, err)
		}
	}
	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])
	in := CreateInput{Filename: filename, ContentType: contentType, Data: data, SizeBytes: len(data)}
	if s.objectStore != nil {
		key := "attachments/" + orgID + "/" + sha
		if err := s.objectStore.Put(ctx, key, bytes.NewReader(data), int64(len(data)), contentType); err != nil {
			return Meta{}, fmt.Errorf("attachments: store object: %w", err)
		}
		in.Data = nil
		in.ObjectKey = key
	}
	return s.store.Create(ctx, orgID, projectID, taskID, userID, in, sha)
}

// List returns attachment metadata.
func (s *Service) List(ctx context.Context, orgID, taskID string) ([]Meta, error) {
	return s.store.List(ctx, orgID, taskID)
}

// Get returns one attachment with bytes.
func (s *Service) Get(ctx context.Context, orgID, taskID, id string) (Attachment, error) {
	attachment, err := s.store.Get(ctx, orgID, taskID, id)
	if err != nil {
		return Attachment{}, err
	}
	if len(attachment.Data) == 0 && attachment.ObjectKey != "" && s.objectStore != nil {
		data, gerr := s.objectStore.Get(ctx, attachment.ObjectKey)
		if gerr != nil {
			return Attachment{}, gerr
		}
		attachment.Data = data
	}
	return attachment, nil
}
