package session

import (
	"sync"
	"time"

	"intimate-agent/internal/models"

	"github.com/google/uuid"
)

// Manager manages in-memory session state for all users.
type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*models.SessionState // keyed by userID
}

// NewManager creates a new session manager.
func NewManager() *Manager {
	return &Manager{
		sessions: make(map[string]*models.SessionState),
	}
}

// GetOrCreate returns the existing session for a user or creates a new one.
func (m *Manager) GetOrCreate(userID string) *models.SessionState {
	m.mu.RLock()
	if s, ok := m.sessions[userID]; ok {
		m.mu.RUnlock()
		return s
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock.
	if s, ok := m.sessions[userID]; ok {
		return s
	}

	s := &models.SessionState{
		SessionID: uuid.New().String(),
		UserID:    userID,
		Messages:  []models.Message{},
		Relationship: models.RelationshipState{
			Familiarity: 0,
			Trust:       0,
			Intimacy:    0,
		},
		TurnCount: 0,
	}
	m.sessions[userID] = s
	return s
}

// AddMessage appends a message to the user's session.
func (m *Manager) AddMessage(userID, role, content string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sessions[userID]
	if !ok {
		return
	}

	s.Messages = append(s.Messages, models.Message{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	})

	if role == "user" {
		s.TurnCount++
	}
}

// GetMessages returns the conversation history for a user.
func (m *Manager) GetMessages(userID string) []models.Message {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.sessions[userID]
	if !ok {
		return nil
	}
	msgs := make([]models.Message, len(s.Messages))
	copy(msgs, s.Messages)
	return msgs
}

// UpdateRelationship updates the relationship metrics for a user.
func (m *Manager) UpdateRelationship(userID string, rel models.RelationshipState) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sessions[userID]
	if !ok {
		return
	}
	s.Relationship = rel
}

// GetRelationship returns the current relationship state for a user.
func (m *Manager) GetRelationship(userID string) models.RelationshipState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.sessions[userID]
	if !ok {
		return models.RelationshipState{}
	}
	return s.Relationship
}

// GetSession returns a copy of the full session state.
func (m *Manager) GetSession(userID string) *models.SessionState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.sessions[userID]
	if !ok {
		return nil
	}
	cp := *s
	cp.Messages = make([]models.Message, len(s.Messages))
	copy(cp.Messages, s.Messages)
	return &cp
}
