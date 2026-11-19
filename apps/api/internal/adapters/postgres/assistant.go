package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres/pgstore"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
)

// AssistantStore implements the assistant conversation store.
type AssistantStore struct {
	pool *pgxpool.Pool
	q    *pgstore.Queries
}

// NewAssistantStore wires the store.
func NewAssistantStore(pool *pgxpool.Pool) *AssistantStore {
	return &AssistantStore{pool: pool, q: pgstore.New(pool)}
}

func (s *AssistantStore) CreateConversation(ctx context.Context, c domain.AssistantConversation) (domain.AssistantConversation, error) {
	var out domain.AssistantConversation
	err := db.WithOrg(ctx, s.pool, c.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).CreateAssistantConversation(ctx, pgstore.CreateAssistantConversationParams{
			OrgID: toUUID(c.OrgID), UserID: toUUID(c.UserID), ProjectID: toOptionalUUID(nilIfEmpty(c.ProjectID)), Title: c.Title,
		})
		if err != nil {
			return err
		}
		out = assistantConversationFromRow(row)
		return nil
	})
	return out, err
}

func (s *AssistantStore) GetConversation(ctx context.Context, orgID, id string) (domain.AssistantConversation, error) {
	var out domain.AssistantConversation
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).GetAssistantConversation(ctx, pgstore.GetAssistantConversationParams{ID: toUUID(id), OrgID: toUUID(orgID)})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = assistantConversationFromRow(row)
		return nil
	})
	return out, err
}

func (s *AssistantStore) ListConversations(ctx context.Context, orgID, userID string, limit int) ([]domain.AssistantConversation, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	var out []domain.AssistantConversation
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListAssistantConversations(ctx, pgstore.ListAssistantConversationsParams{
			OrgID: toUUID(orgID), UserID: toUUID(userID), Limit: int32(limit), //nolint:gosec // limit is bounded to 50
		})
		if err != nil {
			return err
		}
		out = make([]domain.AssistantConversation, 0, len(rows))
		for _, row := range rows {
			out = append(out, assistantConversationFromRow(row))
		}
		return nil
	})
	return out, err
}

func (s *AssistantStore) AppendMessage(ctx context.Context, m domain.AssistantMessage) (domain.AssistantMessage, error) {
	var out domain.AssistantMessage
	err := db.WithOrg(ctx, s.pool, m.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).AppendAssistantMessage(ctx, pgstore.AppendAssistantMessageParams{
			ConversationID: toUUID(m.ConversationID), OrgID: toUUID(m.OrgID), Role: string(m.Role),
			Content: m.Content, ToolName: m.ToolName, ToolInput: m.ToolInput, ToolOutput: m.ToolOutput,
		})
		if err != nil {
			return err
		}
		if err := s.q.WithTx(tx).TouchAssistantConversation(ctx, pgstore.TouchAssistantConversationParams{ID: toUUID(m.ConversationID), OrgID: toUUID(m.OrgID)}); err != nil {
			return err
		}
		out = assistantMessageFromRow(row)
		return nil
	})
	return out, err
}

func (s *AssistantStore) ListMessages(ctx context.Context, orgID, conversationID string, limit int) ([]domain.AssistantMessage, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	var out []domain.AssistantMessage
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListAssistantMessages(ctx, pgstore.ListAssistantMessagesParams{
			ConversationID: toUUID(conversationID), OrgID: toUUID(orgID), Limit: int32(limit), //nolint:gosec // limit is bounded to 200
		})
		if err != nil {
			return err
		}
		out = make([]domain.AssistantMessage, 0, len(rows))
		for _, row := range rows {
			out = append(out, assistantMessageFromRow(row))
		}
		return nil
	})
	return out, err
}

func assistantConversationFromRow(row pgstore.AssistantConversation) domain.AssistantConversation {
	return domain.AssistantConversation{
		ID: uuidString(row.ID), OrgID: uuidString(row.OrgID), UserID: uuidString(row.UserID),
		ProjectID: uuidString(row.ProjectID), Title: row.Title,
		CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time,
	}
}

func assistantMessageFromRow(row pgstore.AssistantMessage) domain.AssistantMessage {
	return domain.AssistantMessage{
		ID: uuidString(row.ID), ConversationID: uuidString(row.ConversationID), OrgID: uuidString(row.OrgID),
		Role: domain.AssistantRole(row.Role), Content: row.Content, ToolName: row.ToolName,
		ToolInput: row.ToolInput, ToolOutput: row.ToolOutput, CreatedAt: row.CreatedAt.Time,
	}
}
