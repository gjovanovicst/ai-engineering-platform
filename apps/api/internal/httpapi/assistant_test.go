package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/assistant"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

type mockAssistant struct {
	out assistant.SendOutput
	err error
}

func (m *mockAssistant) Send(context.Context, assistant.SendInput) (assistant.SendOutput, error) {
	return m.out, m.err
}

func (m *mockAssistant) ListConversations(context.Context, string, string, int) ([]domain.AssistantConversation, error) {
	return nil, nil
}

func (m *mockAssistant) ListMessages(context.Context, string, string, int) ([]domain.AssistantMessage, error) {
	return nil, nil
}

func assistantReq(t *testing.T, body string, accept string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orgs/o/assistant/messages", strings.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), apiKeyContextKey{}, &auth.Claims{OrgID: "o", Subject: "u"}))
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	return req
}

func TestAssistantSendJSON(t *testing.T) {
	t.Parallel()
	h := &assistantHandlers{svc: &mockAssistant{out: assistant.SendOutput{ConversationID: "c1", Text: "The run failed because the test gate failed."}}, log: slog.Default()}
	rec := httptest.NewRecorder()
	h.send(rec, assistantReq(t, `{"prompt":"why did it fail?"}`, ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "test gate failed") || !strings.Contains(rec.Body.String(), `"conversation_id":"c1"`) {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestAssistantStreamsEvents(t *testing.T) {
	t.Parallel()
	h := &assistantHandlers{svc: &mockAssistant{out: assistant.SendOutput{ConversationID: "c1", Text: "I will create the task.", PendingTool: "create_task"}}, log: slog.Default()}
	rec := httptest.NewRecorder()
	h.send(rec, assistantReq(t, `{"prompt":"draft it"}`, "text/event-stream"))
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("expected an event stream, got %q", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "event: pending") || !strings.Contains(body, "event: done") {
		t.Fatalf("expected the pending and done events, got: %s", body)
	}
}
