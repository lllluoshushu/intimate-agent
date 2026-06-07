package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"intimate-agent/internal/llm"
	"intimate-agent/internal/models"

	"github.com/google/uuid"
)

// ConflictResolver handles detection and resolution of memory conflicts.
type ConflictResolver struct {
	store *Store
	llm   llm.Client
}

// NewConflictResolver creates a new ConflictResolver.
func NewConflictResolver(store *Store, llmClient llm.Client) *ConflictResolver {
	return &ConflictResolver{store: store, llm: llmClient}
}

// ExtractedInfo represents information extracted from a user message.
type ExtractedInfo struct {
	Category   string  `json:"category"`
	Key        string  `json:"key"`
	Value      string  `json:"value"`
	Source     string  `json:"source"`
	Confidence float64 `json:"confidence"`
}

// ResolveAndStore takes extracted info items and resolves conflicts with existing memories.
// Returns the list of memory items that were created or updated, and any conflict descriptions.
func (cr *ConflictResolver) ResolveAndStore(ctx context.Context, userID string, extracted []ExtractedInfo) ([]models.MemoryItem, []string, error) {
	var stored []models.MemoryItem
	var conflicts []string

	for _, info := range extracted {
		cat := models.MemoryCategory(info.Category)
		existing, err := cr.store.GetMemoryByKey(userID, cat, info.Key)
		if err != nil {
			return stored, conflicts, fmt.Errorf("lookup memory: %w", err)
		}

		now := time.Now()
		newItem := models.MemoryItem{
			ID:         uuid.New().String(),
			UserID:     userID,
			Category:   cat,
			Key:        info.Key,
			Value:      info.Value,
			Source:     info.Source,
			Confidence: info.Confidence,
			CreatedAt:  now,
			UpdatedAt:  now,
			Active:     true,
		}

		if existing == nil {
			if err := cr.store.AddMemory(newItem); err != nil {
				return stored, conflicts, fmt.Errorf("add memory: %w", err)
			}
			stored = append(stored, newItem)
			continue
		}

		if existing.Value == info.Value {
			continue
		}

		isConflict, resolution, err := cr.resolveConflict(ctx, existing, &info)
		if err != nil {
			isConflict = true
			if info.Confidence >= existing.Confidence {
				resolution = "update"
			} else {
				resolution = "keep_old"
			}
		}

		if !isConflict {
			continue
		}

		switch resolution {
		case "update":
			if err := cr.store.SupersedeMemory(existing.ID, newItem.ID); err != nil {
				return stored, conflicts, fmt.Errorf("supersede memory: %w", err)
			}
			if err := cr.store.AddMemory(newItem); err != nil {
				return stored, conflicts, fmt.Errorf("add replacement memory: %w", err)
			}
			conflicts = append(conflicts, fmt.Sprintf(
				"Updated %s/%s: '%s' -> '%s'", info.Category, info.Key, existing.Value, info.Value,
			))
			stored = append(stored, newItem)
		case "keep_old":
			conflicts = append(conflicts, fmt.Sprintf(
				"Kept existing %s/%s: '%s' (new value '%s' had lower confidence)",
				info.Category, info.Key, existing.Value, info.Value,
			))
		case "ask_user":
			newItem.Source = "[UNSURE] " + newItem.Source
			if err := cr.store.AddMemory(newItem); err != nil {
				return stored, conflicts, fmt.Errorf("add uncertain memory: %w", err)
			}
			conflicts = append(conflicts, fmt.Sprintf(
				"Uncertain update for %s/%s: '%s' vs '%s' - kept both, will ask user",
				info.Category, info.Key, existing.Value, info.Value,
			))
			stored = append(stored, newItem)
		}
	}

	return stored, conflicts, nil
}

type conflictResult struct {
	IsConflict bool   `json:"is_conflict"`
	Resolution string `json:"resolution"`
	Reason     string `json:"reason"`
}

func (cr *ConflictResolver) resolveConflict(ctx context.Context, existing *models.MemoryItem, newInfo *ExtractedInfo) (bool, string, error) {
	prompt := fmt.Sprintf(`You are a memory conflict resolver. A user previously said something and now says something new about the same topic.

Previous memory:
- Category: %s
- Key: %s
- Value: %s
- Source: "%s"

New information:
- Value: %s
- Source: "%s"

Determine:
1. Is this a real conflict (the user is updating/correcting their info) or just a restatement?
2. If it's a conflict, what should the resolution be?

Respond in this exact JSON format and nothing else:
{"is_conflict": true, "resolution": "update", "reason": "brief explanation"}`, existing.Category, existing.Key, existing.Value, existing.Source, newInfo.Value, newInfo.Source)

	messages := []llm.Message{
		{Role: "user", Content: prompt},
	}

	reply, err := cr.llm.Chat(ctx, messages)
	if err != nil {
		return false, "", err
	}

	reply = strings.TrimSpace(reply)
	if idx := strings.Index(reply, "{"); idx >= 0 {
		reply = reply[idx:]
	}
	if idx := strings.LastIndex(reply, "}"); idx >= 0 {
		reply = reply[:idx+1]
	}

	var result conflictResult
	if err := json.Unmarshal([]byte(reply), &result); err != nil {
		return false, "", fmt.Errorf("parse conflict result: %w", err)
	}

	return result.IsConflict, result.Resolution, nil
}
