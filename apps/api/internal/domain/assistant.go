package domain

import "time"

// AssistantRole is the author of one message in a conversation.
type AssistantRole string

const (
	AssistantRoleUser      AssistantRole = "user"
	AssistantRoleAssistant AssistantRole = "assistant"
	AssistantRoleTool      AssistantRole = "tool"
)

// AssistantConversation groups the messages of one chat.
type AssistantConversation struct {
	ID        string
	OrgID     string
	UserID    string
	ProjectID string
	Title     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// AssistantMessage is one turn: a user question, an assistant answer or a tool
// call with its input and output (the tool preview the UI shows).
type AssistantMessage struct {
	ID             string
	ConversationID string
	OrgID          string
	Role           AssistantRole
	Content        string
	ToolName       string
	ToolInput      []byte
	ToolOutput     []byte
	CreatedAt      time.Time
}
