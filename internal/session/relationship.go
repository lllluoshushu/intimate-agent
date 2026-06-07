package session

import (
	"math"

	"intimate-agent/internal/models"
)

// RelationshipEngine manages relationship metric updates based on conversation events.
type RelationshipEngine struct {
	// Weights for different signals.
	infoWeight    float64
	emotionWeight float64
	turnWeight    float64
	conflictWeight float64
}

// NewRelationshipEngine creates a new RelationshipEngine with default weights.
func NewRelationshipEngine() *RelationshipEngine {
	return &RelationshipEngine{
		infoWeight:     5.0,
		emotionWeight:  3.0,
		turnWeight:     1.5,
		conflictWeight: 2.0,
	}
}

// UpdateInput contains the signals for updating relationship metrics.
type UpdateInput struct {
	// NumInfoItems is how many new info items were extracted this turn.
	NumInfoItems int
	// HasEmotion indicates if emotional content was detected.
	HasEmotion bool
	// ConflictCount is how many memory conflicts were resolved this turn.
	ConflictCount int
	// TurnCount is the total number of user turns so far.
	TurnCount int
}

// Update calculates new relationship metrics based on the input signals.
func (re *RelationshipEngine) Update(current models.RelationshipState, input UpdateInput) models.RelationshipState {
	result := current

	// Familiarity increases with information gathered.
	familiarityDelta := float64(input.NumInfoItems) * re.infoWeight
	result.Familiarity = clamp(result.Familiarity + familiarityDelta, 0, 100)

	// Trust increases when user shares emotions and over time.
	trustDelta := re.turnWeight
	if input.HasEmotion {
		trustDelta += re.emotionWeight
	}
	result.Trust = clamp(result.Trust + trustDelta, 0, 100)

	// Intimacy increases gradually with turns, and more when emotions are shared.
	intimacyDelta := re.turnWeight * 0.5
	if input.HasEmotion {
		intimacyDelta += re.emotionWeight * 0.8
	}
	// Sharing info also slightly increases intimacy.
	intimacyDelta += float64(input.NumInfoItems) * 0.5
	result.Intimacy = clamp(result.Intimacy + intimacyDelta, 0, 100)

	return result
}

// GetRelationshipLabel returns a human-readable label for the relationship level.
func GetRelationshipLabel(rel models.RelationshipState) string {
	avg := (rel.Familiarity + rel.Trust + rel.Intimacy) / 3.0
	switch {
	case avg < 10:
		return "stranger"
	case avg < 30:
		return "acquaintance"
	case avg < 50:
		return "familiar"
	case avg < 70:
		return "close"
	default:
		return "intimate"
	}
}

// GetToneHint returns a tone adjustment hint based on relationship state.
func GetToneHint(rel models.RelationshipState) string {
	label := GetRelationshipLabel(rel)
	switch label {
	case "stranger":
		return "Be polite and formal. Introduce yourself warmly."
	case "acquaintance":
		return "Be friendly and curious. Show interest in the user."
	case "familiar":
		return "Be warm and casual. Reference shared knowledge naturally."
	case "close":
		return "Be caring and personal. Use a gentle, supportive tone."
	case "intimate":
		return "Be deeply empathetic and personal. Show genuine care and use warm, familiar language."
	default:
		return "Be friendly and helpful."
	}
}

func clamp(val, min, max float64) float64 {
	return math.Max(min, math.Min(max, val))
}
