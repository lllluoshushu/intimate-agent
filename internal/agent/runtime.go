package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"intimate-agent/internal/llm"
	"intimate-agent/internal/memory"
	"intimate-agent/internal/models"
	"intimate-agent/internal/session"
	"intimate-agent/internal/tools"
)

const maxRetries = 2

// Runtime is the core agent orchestrator.
type Runtime struct {
	llm         llm.Client
	store       *memory.Store
	resolver    *memory.ConflictResolver
	sessions    *session.Manager
	relEngine   *session.RelationshipEngine
	toolReg     *tools.Registry
	extractTool *tools.ExtractionTool
	memTool     *tools.MemoryTool
}

// NewRuntime creates a fully wired agent runtime.
func NewRuntime(llmClient llm.Client, store *memory.Store) *Runtime {
	resolver := memory.NewConflictResolver(store, llmClient)
	sessMgr := session.NewManager()
	relEngine := session.NewRelationshipEngine()

	extractTool := tools.NewExtractionTool(llmClient)
	memTool := tools.NewMemoryTool(store)

	toolReg := tools.NewRegistry()
	toolReg.Register(extractTool)
	toolReg.Register(memTool)

	return &Runtime{
		llm:         llmClient,
		store:       store,
		resolver:    resolver,
		sessions:    sessMgr,
		relEngine:   relEngine,
		toolReg:     toolReg,
		extractTool: extractTool,
		memTool:     memTool,
	}
}

// ProcessMessage runs the full agent pipeline for a user message.
func (r *Runtime) ProcessMessage(ctx context.Context, userID, userMessage string) (string, models.ExecutionTrace, error) {
	trace := models.ExecutionTrace{}
	stepNum := 0

	// Ensure user profile exists.
	profile, err := r.store.GetOrCreateProfile(userID)
	if err != nil {
		return "", trace, fmt.Errorf("get/create profile: %w", err)
	}
	_ = profile

	// Get or create session.
	sess := r.sessions.GetOrCreate(userID)

	// Record user message.
	r.sessions.AddMessage(userID, "user", userMessage)

	// --- Step 1: Read existing memories ---
	stepNum++
	t0 := time.Now()
	readResult, err := r.executeToolWithRetry(ctx, "memory_read_write", map[string]interface{}{
		"action":  "read",
		"user_id": userID,
	})
	memStatus := "success"
	if err != nil {
		memStatus = "fallback"
		readResult = map[string]interface{}{"memories": []interface{}{}, "error": err.Error()}
	}
	trace.Steps = append(trace.Steps, models.TraceStep{
		Step:   stepNum,
		Name:   "Step1: Read existing memories",
		Input:  fmt.Sprintf("user_id=%s", userID),
		Output: fmt.Sprintf("Found %v memories", readResult["count"]),
		Status: memStatus,
		Latency: time.Since(t0),
	})

	// --- Step 2: Extract info from user message ---
	stepNum++
	t1 := time.Now()
	extractResult, err := r.executeToolWithRetry(ctx, "info_extraction", map[string]interface{}{
		"message": userMessage,
		"source":  userMessage,
	})
	extractStatus := "success"
	if err != nil {
		extractStatus = "fallback"
		extractResult = map[string]interface{}{"items": []interface{}{}, "count": 0}
	}
	trace.Steps = append(trace.Steps, models.TraceStep{
		Step:   stepNum,
		Name:   "Step2: Extract user info",
		Input:  fmt.Sprintf("message='%s'", truncate(userMessage, 50)),
		Output: fmt.Sprintf("Extracted %v items", extractResult["count"]),
		Status: extractStatus,
		Latency: time.Since(t1),
	})

	// --- Step 3: Resolve conflicts and update memory ---
	stepNum++
	t2 := time.Now()
	var extractedInfos []memory.ExtractedInfo
	if rawItems, ok := extractResult["items"]; ok {
		if itemsList, ok := rawItems.([]interface{}); ok {
			for _, item := range itemsList {
				if m, ok := item.(map[string]interface{}); ok {
					info := memory.ExtractedInfo{
						Category:   getString(m, "category"),
						Key:        getString(m, "key"),
						Value:      getString(m, "value"),
						Confidence: getFloat(m, "confidence"),
						Source:     getString(m, "source"),
					}
					extractedInfos = append(extractedInfos, info)
				}
			}
		}
	}

	stored, conflicts, err := r.resolver.ResolveAndStore(ctx, userID, extractedInfos)
	conflictStatus := "success"
	conflictOutput := fmt.Sprintf("Stored %d memories", len(stored))
	if len(conflicts) > 0 {
		conflictOutput += fmt.Sprintf(", %d conflicts resolved", len(conflicts))
	}
	if err != nil {
		conflictStatus = "error"
		conflictOutput = fmt.Sprintf("Error: %v", err)
	}
	trace.Steps = append(trace.Steps, models.TraceStep{
		Step:   stepNum,
		Name:   "Step3: Resolve conflicts & update memory",
		Input:  fmt.Sprintf("%d extracted items", len(extractedInfos)),
		Output: conflictOutput,
		Status: conflictStatus,
		Latency: time.Since(t2),
	})

	// --- Step 4: Update relationship state ---
	stepNum++
	t3 := time.Now()
	hasEmotion := false
	for _, info := range extractedInfos {
		if info.Category == "emotion" {
			hasEmotion = true
			break
		}
	}
	newRel := r.relEngine.Update(sess.Relationship, session.UpdateInput{
		NumInfoItems:  len(stored),
		HasEmotion:    hasEmotion,
		ConflictCount: len(conflicts),
		TurnCount:     sess.TurnCount,
	})
	r.sessions.UpdateRelationship(userID, newRel)
	relLabel := session.GetRelationshipLabel(newRel)
	trace.Steps = append(trace.Steps, models.TraceStep{
		Step:   stepNum,
		Name:   "Step4: Update relationship state",
		Input:  fmt.Sprintf("info_items=%d, has_emotion=%v", len(stored), hasEmotion),
		Output: fmt.Sprintf("Familiarity=%.1f, Trust=%.1f, Intimacy=%.1f (%s)", newRel.Familiarity, newRel.Trust, newRel.Intimacy, relLabel),
		Status: "success",
		Latency: time.Since(t3),
	})

	// --- Step 5: Generate reply ---
	stepNum++
	t4 := time.Now()
	reply, err := r.generateReply(ctx, userID, userMessage, stored, conflicts, newRel, sess)
	replyStatus := "success"
	if err != nil {
		replyStatus = "fallback"
		reply = r.fallbackReply(newRel)
	}
	trace.Steps = append(trace.Steps, models.TraceStep{
		Step:   stepNum,
		Name:   "Step5: Generate reply",
		Input:  fmt.Sprintf("relationship=%s, memories=%d", relLabel, len(stored)),
		Output: fmt.Sprintf("Reply length=%d chars", len(reply)),
		Status: replyStatus,
		Latency: time.Since(t4),
	})

	// Record assistant message.
	r.sessions.AddMessage(userID, "assistant", reply)

	return reply, trace, nil
}

// generateReply builds the LLM prompt with memory context and generates a reply.
func (r *Runtime) generateReply(ctx context.Context, userID, userMessage string, storedMemories []models.MemoryItem, conflicts []string, rel models.RelationshipState, sess *models.SessionState) (string, error) {
	// Build memory context string.
	memContext := r.buildMemoryContext(userID)

	// Build conversation history.
	history := r.buildHistory(sess)

	// Get tone hint.
	toneHint := session.GetToneHint(rel)

	// Build conflict note.
	conflictNote := ""
	if len(conflicts) > 0 {
		conflictNote = "\n\n[Memory conflicts resolved this turn: " + strings.Join(conflicts, "; ") + "]"
	}

	systemPrompt := fmt.Sprintf(`You are a caring, empathetic AI companion who builds a genuine relationship with the user over time. You remember what the user tells you and reference it naturally in conversation.

Relationship Level: %s (%.0f%% familiarity, %.0f%% trust, %.0f%% intimacy)
Tone Guidance: %s

What you know about the user:
%s%s

Guidelines:
- Reference past information naturally, don't repeat it mechanically
- If there were memory conflicts, acknowledge the update warmly
- Match your tone to the relationship level
- Be genuine, not performative
- Keep replies concise but warm (1-3 sentences usually)`,
		relLabel(rel), rel.Familiarity, rel.Trust, rel.Intimacy,
		toneHint, memContext, conflictNote)

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
	}
	messages = append(messages, history...)
	messages = append(messages, llm.Message{Role: "user", Content: userMessage})

	reply, err := r.llm.Chat(ctx, messages)
	if err != nil {
		return "", err
	}

	return reply, nil
}

func (r *Runtime) buildMemoryContext(userID string) string {
	memories, err := r.store.GetActiveMemories(userID)
	if err != nil || len(memories) == 0 {
		return "(No memories yet - this is a new conversation)"
	}

	var lines []string
	for _, m := range memories {
		lines = append(lines, fmt.Sprintf("- [%s] %s: %s", m.Category, m.Key, m.Value))
	}
	return strings.Join(lines, "\n")
}

func (r *Runtime) buildHistory(sess *models.SessionState) []llm.Message {
	// Use last 10 messages to avoid context overflow.
	msgs := sess.Messages
	if len(msgs) > 10 {
		msgs = msgs[len(msgs)-10:]
	}

	var history []llm.Message
	for _, m := range msgs {
		history = append(history, llm.Message{Role: m.Role, Content: m.Content})
	}
	return history
}

func (r *Runtime) fallbackReply(rel models.RelationshipState) string {
	label := relLabel(rel)
	switch label {
	case "stranger", "acquaintance":
		return "抱歉，我刚才走神了。你能再说一次吗？"
	case "familiar", "close":
		return "哎呀，我刚才没听清，能再说一遍吗？"
	default:
		return "对不起亲爱的，我刚才没跟上，能再告诉我一次吗？"
	}
}

func (r *Runtime) executeToolWithRetry(ctx context.Context, toolName string, input map[string]interface{}) (map[string]interface{}, error) {
	var lastErr error
	for i := 0; i <= maxRetries; i++ {
		tool, ok := r.toolReg.Get(toolName)
		if !ok {
			return nil, fmt.Errorf("tool not found: %s", toolName)
		}
		result, err := tool.Execute(ctx, input)
		if err == nil {
			return result, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("tool %s failed after %d retries: %w", toolName, maxRetries, lastErr)
}

func relLabel(rel models.RelationshipState) string {
	return session.GetRelationshipLabel(rel)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// GetStatus returns the current status for a user.
func (r *Runtime) GetStatus(userID string) (*models.StatusResponse, error) {
	profile, err := r.store.GetOrCreateProfile(userID)
	if err != nil {
		return nil, err
	}

	memories, err := r.store.GetActiveMemories(userID)
	if err != nil {
		return nil, err
	}
	if memories == nil {
		memories = []models.MemoryItem{}
	}

	rel := r.sessions.GetRelationship(userID)

	return &models.StatusResponse{
		UserID:       userID,
		Profile:      profile,
		Memories:     memories,
		Relationship: rel,
	}, nil
}

// GetSession returns a copy of the user's current session state.
func (r *Runtime) GetSession(userID string) models.SessionState {
	sess := r.sessions.GetSession(userID)
	if sess == nil {
		return models.SessionState{}
	}
	return *sess
}

// getString extracts a string value from a map.
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// getFloat extracts a float64 value from a map.
func getFloat(m map[string]interface{}, key string) float64 {
	if v, ok := m[key]; ok {
		if f, ok := v.(float64); ok {
			return f
		}
	}
	return 0.0
}
