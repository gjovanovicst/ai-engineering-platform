package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/assistant"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// AssistantService is the assistant the HTTP layer depends on.
type AssistantService interface {
	Send(ctx context.Context, in assistant.SendInput) (assistant.SendOutput, error)
	ListConversations(ctx context.Context, orgID, userID string, limit int) ([]domain.AssistantConversation, error)
	ListMessages(ctx context.Context, orgID, conversationID string, limit int) ([]domain.AssistantMessage, error)
	DeleteConversation(ctx context.Context, orgID, userID, id string) error
	DeleteConversations(ctx context.Context, orgID, userID string) error
}

type assistantHandlers struct {
	svc    AssistantService
	issuer *auth.Issuer
	log    *slog.Logger
}

type assistantMessageInput struct {
	ConversationID string   `json:"conversation_id"`
	Prompt         string   `json:"prompt"`
	Confirm        bool     `json:"confirm"`
	Context        string   `json:"context"`
	Images         []string `json:"images"`
}

// send answers one message. With the SSE Accept header it streams the text and
// the tool preview as events; otherwise it returns one JSON object.
func (h *assistantHandlers) send(w http.ResponseWriter, r *http.Request) {
	claims, ok := bearerClaims(h.issuer, r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "missing or invalid access token")
		return
	}
	if org := r.PathValue("org"); org != "" && org != claims.OrgID {
		writeProblem(w, http.StatusNotFound, "not_found", "resource not found")
		return
	}
	var input assistantMessageInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&input); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid", "the body is not valid JSON")
		return
	}
	if strings.TrimSpace(input.Prompt) == "" {
		writeProblem(w, http.StatusBadRequest, "invalid", "the prompt is empty")
		return
	}
	out, err := h.svc.Send(r.Context(), assistant.SendInput{
		OrgID: claims.OrgID, UserID: claims.Subject, ConversationID: input.ConversationID, Prompt: input.Prompt, Confirm: input.Confirm, Context: input.Context, Images: input.Images,
	})
	if err != nil {
		if errors.Is(err, domain.ErrValidation) || errors.Is(err, domain.ErrNotFound) {
			writeProblem(w, http.StatusBadRequest, "assistant", err.Error())
			return
		}
		h.log.Error("assistant.send", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "the assistant could not answer")
		return
	}
	if strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
		h.stream(w, out)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"conversation_id": out.ConversationID, "text": out.Text, "pending_tool": out.PendingTool, "pending_input": out.PendingInput})
}

// stream pushes the answer as Server-Sent Events so the panel can render it
// while the model/tool loop is still running.
func (h *assistantHandlers) stream(w http.ResponseWriter, out assistant.SendOutput) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeProblem(w, http.StatusInternalServerError, "internal", "streaming is not supported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("retry: 3000\n\n"))
	emit := func(event string, payload map[string]any) {
		data, _ := json.Marshal(payload)
		_, _ = w.Write([]byte("event: " + event + "\ndata: " + string(data) + "\n\n"))
	}
	if out.PendingTool != "" {
		emit("pending", map[string]any{"pending_tool": out.PendingTool, "text": out.Text, "pending_input": out.PendingInput, "conversation_id": out.ConversationID})
	} else {
		emit("text", map[string]any{"text": out.Text, "conversation_id": out.ConversationID})
	}
	emit("done", map[string]any{"conversation_id": out.ConversationID})
	flusher.Flush()
}

// conversations lists the user's recent conversations.
func (h *assistantHandlers) conversations(w http.ResponseWriter, r *http.Request) {
	claims, ok := bearerClaims(h.issuer, r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "missing or invalid access token")
		return
	}
	if org := r.PathValue("org"); org != claims.OrgID {
		writeProblem(w, http.StatusNotFound, "not_found", "resource not found")
		return
	}
	list, err := h.svc.ListConversations(r.Context(), claims.OrgID, claims.Subject, 20)
	if err != nil {
		h.log.Error("assistant.conversations", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not load the conversations")
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, c := range list {
		out = append(out, map[string]any{"id": c.ID, "title": c.Title, "updated_at": c.UpdatedAt.UTC().Format(time.RFC3339)})
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

// messages returns one conversation's messages.
func (h *assistantHandlers) messages(w http.ResponseWriter, r *http.Request) {
	claims, ok := bearerClaims(h.issuer, r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "missing or invalid access token")
		return
	}
	if org := r.PathValue("org"); org != claims.OrgID {
		writeProblem(w, http.StatusNotFound, "not_found", "resource not found")
		return
	}
	list, err := h.svc.ListMessages(r.Context(), claims.OrgID, r.PathValue("conversation"), 100)
	if err != nil {
		h.log.Error("assistant.messages", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not load the messages")
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, m := range list {
		out = append(out, map[string]any{
			"id": m.ID, "role": string(m.Role), "content": m.Content, "tool_name": m.ToolName,
			"created_at": m.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

// deleteConversation removes one conversation.
func (h *assistantHandlers) deleteConversation(w http.ResponseWriter, r *http.Request) {
	claims, ok := bearerClaims(h.issuer, r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "missing or invalid access token")
		return
	}
	if org := r.PathValue("org"); org != claims.OrgID {
		writeProblem(w, http.StatusNotFound, "not_found", "resource not found")
		return
	}
	if err := h.svc.DeleteConversation(r.Context(), claims.OrgID, claims.Subject, r.PathValue("conversation")); err != nil {
		h.log.Error("assistant.delete", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not delete the conversation")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// deleteConversations removes all of the user's conversations.
func (h *assistantHandlers) deleteConversations(w http.ResponseWriter, r *http.Request) {
	claims, ok := bearerClaims(h.issuer, r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "missing or invalid access token")
		return
	}
	if org := r.PathValue("org"); org != claims.OrgID {
		writeProblem(w, http.StatusNotFound, "not_found", "resource not found")
		return
	}
	if err := h.svc.DeleteConversations(r.Context(), claims.OrgID, claims.Subject); err != nil {
		h.log.Error("assistant.clear", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not clear the conversations")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
