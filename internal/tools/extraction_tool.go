package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"intimate-agent/internal/llm"
	"intimate-agent/internal/memory"
)

// ExtractionTool uses the LLM to extract structured information from user messages.
type ExtractionTool struct {
	llmClient llm.Client
}

// NewExtractionTool creates a new ExtractionTool.
func NewExtractionTool(llmClient llm.Client) *ExtractionTool {
	return &ExtractionTool{llmClient: llmClient}
}

func (t *ExtractionTool) Name() string { return "info_extraction" }

func (t *ExtractionTool) Description() string {
	return "Extract structured user information from a message. Returns categorized info items (basic_info, preference, emotion, event, relationship_pref)."
}

type extractionResult struct {
	Items []memory.ExtractedInfo `json:"items"`
}

func (t *ExtractionTool) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	message, _ := input["message"].(string)
	source, _ := input["source"].(string)
	if source == "" {
		source = message
	}

	if message == "" {
		return map[string]interface{}{"items": []interface{}{}, "count": 0}, nil
	}

	prompt := fmt.Sprintf(`You are an information extraction system. Analyze the following user message and extract any personal information the user reveals.

Extract information into these categories:
- basic_info: name, age, occupation, city, or other demographic info
- preference: things the user likes or dislikes
- emotion: current emotional state (happy, anxious, tired, stressed, etc.)
- event: important life events (exam, moving, breakup, interview, etc.)
- relationship_pref: how the user wants the AI to behave (rational, gentle, humorous, etc.)

For each piece of information, provide:
- category: one of the above
- key: a short identifier (e.g., "city", "likes_coffee", "current_mood")
- value: the extracted value
- confidence: 0.0 to 1.0

User message: "%s"

Respond in this exact JSON format and nothing else:
{"items": [{"category": "basic_info", "key": "city", "value": "Shanghai", "confidence": 0.9}]}

If no information is found, return: {"items": []}`, message)

	messages := []llm.Message{
		{Role: "user", Content: prompt},
	}

	reply, err := t.llmClient.Chat(ctx, messages)
	if err != nil {
		// Fallback: return empty extraction on LLM failure.
		return map[string]interface{}{
			"items":  []interface{}{},
			"count":  0,
			"error":  fmt.Sprintf("LLM extraction failed: %v", err),
			"status": "fallback",
		}, nil
	}

	reply = strings.TrimSpace(reply)
	if idx := strings.Index(reply, "{"); idx >= 0 {
		reply = reply[idx:]
	}
	if idx := strings.LastIndex(reply, "}"); idx >= 0 {
		reply = reply[:idx+1]
	}

	var result extractionResult
	if err := json.Unmarshal([]byte(reply), &result); err != nil {
		return map[string]interface{}{
			"items":  []interface{}{},
			"count":  0,
			"error":  fmt.Sprintf("parse extraction result: %v", err),
			"status": "fallback",
		}, nil
	}

	// Set source for each item.
	for i := range result.Items {
		result.Items[i].Source = source
	}

	items := make([]interface{}, len(result.Items))
	for i, item := range result.Items {
		items[i] = map[string]interface{}{
			"category":   item.Category,
			"key":        item.Key,
			"value":      item.Value,
			"confidence": item.Confidence,
			"source":     item.Source,
		}
	}

	return map[string]interface{}{
		"items":  items,
		"count":  len(items),
		"status": "success",
		"raw":    result.Items,
	}, nil
}
