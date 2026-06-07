package tools

import (
	"context"
	"fmt"

	"intimate-agent/internal/memory"
	"intimate-agent/internal/models"
)

// MemoryTool provides read/write access to the user's memory store.
type MemoryTool struct {
	store *memory.Store
}

// NewMemoryTool creates a new MemoryTool.
func NewMemoryTool(store *memory.Store) *MemoryTool {
	return &MemoryTool{store: store}
}

func (t *MemoryTool) Name() string { return "memory_read_write" }

func (t *MemoryTool) Description() string {
	return "Read or write user memories. Actions: 'read' (get all active memories), 'read_category' (filter by category), 'write' (add a new memory)."
}

func (t *MemoryTool) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	action, _ := input["action"].(string)
	userID, _ := input["user_id"].(string)

	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	switch action {
	case "read":
		memories, err := t.store.GetActiveMemories(userID)
		if err != nil {
			return map[string]interface{}{"error": err.Error(), "memories": []interface{}{}}, nil
		}
		items := make([]interface{}, len(memories))
		for i, m := range memories {
			items[i] = map[string]interface{}{
				"id":        m.ID,
				"category":  string(m.Category),
				"key":       m.Key,
				"value":     m.Value,
				"source":    m.Source,
				"confidence": m.Confidence,
			}
		}
		return map[string]interface{}{"action": "read", "count": len(items), "memories": items}, nil

	case "read_category":
		category, _ := input["category"].(string)
		if category == "" {
			return nil, fmt.Errorf("category is required for read_category action")
		}
		memories, err := t.store.GetMemoriesByCategory(userID, models.MemoryCategory(category))
		if err != nil {
			return map[string]interface{}{"error": err.Error(), "memories": []interface{}{}}, nil
		}
		items := make([]interface{}, len(memories))
		for i, m := range memories {
			items[i] = map[string]interface{}{
				"id":    m.ID,
				"key":   m.Key,
				"value": m.Value,
			}
		}
		return map[string]interface{}{"action": "read_category", "category": category, "count": len(items), "memories": items}, nil

	default:
		return map[string]interface{}{"error": fmt.Sprintf("unknown action: %s", action)}, nil
	}
}
