package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres/pgstore"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/attachments"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
)

// AttachmentStore implements attachments.Store.
type AttachmentStore struct {
	pool *pgxpool.Pool
	q    *pgstore.Queries
}

// NewAttachmentStore wires the store.
func NewAttachmentStore(pool *pgxpool.Pool) *AttachmentStore {
	return &AttachmentStore{pool: pool, q: pgstore.New(pool)}
}

// Create stores an attachment, checking the task belongs to the project.
func (s *AttachmentStore) Create(ctx context.Context, orgID, projectID, taskID, userID string, in attachments.CreateInput, sha string) (attachments.Meta, error) {
	var out attachments.Meta
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).CreateTaskAttachment(ctx, pgstore.CreateTaskAttachmentParams{
			OrgID: toUUID(orgID), TaskID: toUUID(taskID), Filename: in.Filename,
			ContentType: in.ContentType, SizeBytes: int32(in.SizeBytes), Sha256: sha, //nolint:gosec // the service caps data at 5 MiB
			Data: in.Data, UploadedBy: toUUID(userID), ProjectID: toUUID(projectID), ObjectKey: in.ObjectKey,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = metaFromRow(row)
		return nil
	})
	return out, err
}

// List returns attachment metadata for a task.
func (s *AttachmentStore) List(ctx context.Context, orgID, taskID string) ([]attachments.Meta, error) {
	var out []attachments.Meta
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListTaskAttachments(ctx, pgstore.ListTaskAttachmentsParams{
			OrgID: toUUID(orgID), TaskID: toUUID(taskID),
		})
		if err != nil {
			return err
		}
		for _, row := range rows {
			out = append(out, attachments.Meta{
				ID: uuidString(row.ID), Filename: row.Filename, ContentType: row.ContentType,
				SizeBytes: int(row.SizeBytes), SHA256: row.Sha256, CreatedAt: row.CreatedAt.Time.Format(time.RFC3339),
			})
		}
		return nil
	})
	return out, err
}

// Get returns one attachment with bytes.
func (s *AttachmentStore) Get(ctx context.Context, orgID, taskID, id string) (attachments.Attachment, error) {
	var out attachments.Attachment
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).GetTaskAttachment(ctx, pgstore.GetTaskAttachmentParams{
			ID: toUUID(id), OrgID: toUUID(orgID), TaskID: toUUID(taskID),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = attachments.Attachment{
			Meta: attachments.Meta{
				ID: uuidString(row.ID), Filename: row.Filename, ContentType: row.ContentType,
				SizeBytes: int(row.SizeBytes), SHA256: row.Sha256, CreatedAt: row.CreatedAt.Time.Format(time.RFC3339),
			},
			Data: row.Data, ObjectKey: row.ObjectKey,
		}
		return nil
	})
	return out, err
}

func metaFromRow(row pgstore.CreateTaskAttachmentRow) attachments.Meta {
	return attachments.Meta{
		ID: uuidString(row.ID), Filename: row.Filename, ContentType: row.ContentType,
		SizeBytes: int(row.SizeBytes), SHA256: row.Sha256, CreatedAt: row.CreatedAt.Time.Format(time.RFC3339),
	}
}
