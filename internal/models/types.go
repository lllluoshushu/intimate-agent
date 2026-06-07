package models

import "time"

// UserProfile stores basic information about the user.
type UserProfile struct {
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	Age       int       `json:"age"`
	Occupation string   `json:"occupation"`
	City      string    `json:"city"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// MemoryCategory defines the type of memory item.
type MemoryCategory string

const (
	MemBasicInfo   MemoryCategory = "basic_info"
	MemPreference  MemoryCategory = "preference"
	MemEmotion     MemoryCategory = "emotion"
	MemEvent       MemoryCategory = "event"
	MemRelPref     MemoryCategory = "relationship_pref"
)

// MemoryItem represents a single piece of structured memory about the user.
type MemoryItem struct {
	ID           string         `json:"id"`
	UserID       string         `json:"user_id"`
	Category     MemoryCategory `json:"category"`
	Key          string         `json:"key"`
	Value        string         `json:"value"`
	Source       string         `json:"source"`
	Confidence   float64        `json:"confidence"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	SupersededBy string         `json:"superseded_by,omitempty"`
	Active       bool           `json:"active"`
}

// RelationshipState tracks the evolving relationship metrics.
type RelationshipState struct {
	Familiarity float64 `json:"familiarity"` // 0-100: how much the AI knows about the user
	Trust       float64 `json:"trust"`       // 0-100: how open the user has been
	Intimacy    float64 `json:"intimacy"`    // 0-100: closeness of interaction style
}

// Message represents a single turn in the conversation.
type Message struct {
	Role      string    `json:"role"` // "user" or "assistant"
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// SessionState holds the current conversation state for a user.
type SessionState struct {
	SessionID    string           `json:"session_id"`
	UserID       string           `json:"user_id"`
	Messages     []Message        `json:"messages"`
	Relationship RelationshipState `json:"relationship"`
	TurnCount    int              `json:"turn_count"`
}

// TraceStep records a single step in the agent's execution.
type TraceStep struct {
	Step    int           `json:"step"`
	Name    string        `json:"name"`
	Input   string        `json:"input"`
	Output  string        `json:"output"`
	Status  string        `json:"status"` // "success", "fallback", "error"
	Latency time.Duration `json:"latency_ms"`
}

// ExecutionTrace holds the full execution trace for a single request.
type ExecutionTrace struct {
	Steps []TraceStep `json:"steps"`
}

// ChatRequest is the incoming HTTP request body.
type ChatRequest struct {
	UserID  string `json:"user_id"`
	Message string `json:"message"`
}

// ChatResponse is the outgoing HTTP response body.
type ChatResponse struct {
	Reply   string          `json:"reply"`
	Trace   ExecutionTrace  `json:"trace"`
	Session SessionState    `json:"session"`
}

// StatusResponse shows the user's current memory and relationship state.
type StatusResponse struct {
	UserID      string           `json:"user_id"`
	Profile     UserProfile      `json:"profile"`
	Memories    []MemoryItem     `json:"memories"`
	Relationship RelationshipState `json:"relationship"`
}
