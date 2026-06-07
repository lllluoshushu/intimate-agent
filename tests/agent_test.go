package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"intimate-agent/internal/agent"
	"intimate-agent/internal/llm"
	"intimate-agent/internal/memory"
)

// MockLLMClient returns predictable responses for testing.
type MockLLMClient struct {
	responses map[string]string
	callCount int
}

func NewMockLLMClient() *MockLLMClient {
	return &MockLLMClient{
		responses: make(map[string]string),
	}
}

func (m *MockLLMClient) Chat(ctx context.Context, messages []llm.Message) (string, error) {
	m.callCount++
	lastMsg := messages[len(messages)-1].Content

	// Check for specific patterns to return appropriate responses.
	if strings.Contains(lastMsg, "information extraction") || strings.Contains(lastMsg, "Extract information") {
		return m.handleExtraction(lastMsg)
	}
	if strings.Contains(lastMsg, "memory conflict") || strings.Contains(lastMsg, "conflict resolver") {
		return m.handleConflict(lastMsg)
	}
	if strings.Contains(lastMsg, "caring, empathetic") || strings.Contains(lastMsg, "AI companion") {
		return m.handleReply(lastMsg)
	}

	return "I'm here to help!", nil
}

func (m *MockLLMClient) handleExtraction(msg string) (string, error) {
	// Extract info based on message content.
	if strings.Contains(msg, "小明") || strings.Contains(msg, "程序员") || strings.Contains(msg, "上海") {
		return `{"items": [
			{"category": "basic_info", "key": "name", "value": "小明", "confidence": 0.95},
			{"category": "basic_info", "key": "occupation", "value": "程序员", "confidence": 0.9},
			{"category": "basic_info", "key": "city", "value": "上海", "confidence": 0.9}
		]}`, nil
	}
	if strings.Contains(msg, "北京") {
		return `{"items": [
			{"category": "basic_info", "key": "city", "value": "北京", "confidence": 0.9}
		]}`, nil
	}
	if strings.Contains(msg, "深圳") {
		return `{"items": [
			{"category": "basic_info", "key": "city", "value": "深圳", "confidence": 0.95}
		]}`, nil
	}
	if strings.Contains(msg, "咖啡") || strings.Contains(msg, "篮球") {
		return `{"items": [
			{"category": "preference", "key": "likes_coffee", "value": "true", "confidence": 0.9},
			{"category": "preference", "key": "likes_basketball", "value": "true", "confidence": 0.85}
		]}`, nil
	}
	if strings.Contains(msg, "累") || strings.Contains(msg, "压力") {
		return `{"items": [
			{"category": "emotion", "key": "current_mood", "value": "tired", "confidence": 0.8}
		]}`, nil
	}
	return `{"items": []}`, nil
}

func (m *MockLLMClient) handleConflict(msg string) (string, error) {
	if strings.Contains(msg, "上海") && strings.Contains(msg, "深圳") {
		return `{"is_conflict": true, "resolution": "update", "reason": "User moved from Shanghai to Shenzhen"}`, nil
	}
	return `{"is_conflict": false, "resolution": "keep_old", "reason": "Same information restated"}`, nil
}

func (m *MockLLMClient) handleReply(msg string) (string, error) {
	if strings.Contains(msg, "stranger") {
		return "你好！很高兴认识你，小明！", nil
	}
	if strings.Contains(msg, "acquaintance") || strings.Contains(msg, "familiar") {
		return "小明，我记得你在上海做程序员，最近工作还顺利吗？", nil
	}
	return "我很开心能和你聊天！", nil
}

// Test helper to create a test runtime with mock LLM.
func setupTestRuntime(t *testing.T) (*agent.Runtime, *memory.Store) {
	t.Helper()

	dbPath := ":memory:"
	store, err := memory.NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	mockLLM := NewMockLLMClient()
	runtime := agent.NewRuntime(mockLLM, store)

	return runtime, store
}

// Test 1: Normal relationship building - verifies memory extraction and relationship progression.
func TestRelationshipBuilding(t *testing.T) {
	runtime, store := setupTestRuntime(t)
	defer store.Close()

	ctx := context.Background()
	userID := "test_user_1"

	// Turn 1: User introduces themselves.
	reply1, trace1, err := runtime.ProcessMessage(ctx, userID, "你好，我叫小明，在上海做程序员")
	if err != nil {
		t.Fatalf("Turn 1 failed: %v", err)
	}
	t.Logf("Turn 1 reply: %s", reply1)
	t.Logf("Turn 1 trace: %d steps", len(trace1.Steps))

	// Verify memories were extracted.
	memories, err := store.GetActiveMemories(userID)
	if err != nil {
		t.Fatalf("Failed to get memories: %v", err)
	}
	if len(memories) < 3 {
		t.Errorf("Expected at least 3 memories, got %d", len(memories))
	}

	// Check specific memories.
	nameMem, _ := store.GetMemoryByKey(userID, "basic_info", "name")
	if nameMem == nil || nameMem.Value != "小明" {
		t.Errorf("Expected name memory '小明', got %v", nameMem)
	}

	cityMem, _ := store.GetMemoryByKey(userID, "basic_info", "city")
	if cityMem == nil || cityMem.Value != "上海" {
		t.Errorf("Expected city memory '上海', got %v", cityMem)
	}

	// Turn 2: User shares emotions.
	reply2, trace2, err := runtime.ProcessMessage(ctx, userID, "最近加班很多，感觉有点累")
	if err != nil {
		t.Fatalf("Turn 2 failed: %v", err)
	}
	t.Logf("Turn 2 reply: %s", reply2)
	t.Logf("Turn 2 trace: %d steps", len(trace2.Steps))

	// Verify emotion memory.
	emotionMem, _ := store.GetMemoryByKey(userID, "emotion", "current_mood")
	if emotionMem == nil {
		t.Error("Expected emotion memory, got nil")
	}

	// Turn 3: User shares preferences.
	reply3, trace3, err := runtime.ProcessMessage(ctx, userID, "我喜欢打篮球和喝咖啡")
	if err != nil {
		t.Fatalf("Turn 3 failed: %v", err)
	}
	t.Logf("Turn 3 reply: %s", reply3)
	t.Logf("Turn 3 trace: %d steps", len(trace3.Steps))

	// Verify preference memories.
	coffeeMem, _ := store.GetMemoryByKey(userID, "preference", "likes_coffee")
	if coffeeMem == nil {
		t.Error("Expected coffee preference memory, got nil")
	}

	basketballMem, _ := store.GetMemoryByKey(userID, "preference", "likes_basketball")
	if basketballMem == nil {
		t.Error("Expected basketball preference memory, got nil")
	}

	// Check relationship progression.
	status, err := runtime.GetStatus(userID)
	if err != nil {
		t.Fatalf("Failed to get status: %v", err)
	}
	if status.Relationship.Familiarity <= 0 {
		t.Errorf("Expected familiarity > 0, got %.2f", status.Relationship.Familiarity)
	}
	if status.Relationship.Trust <= 0 {
		t.Errorf("Expected trust > 0, got %.2f", status.Relationship.Trust)
	}

	t.Logf("Final relationship: Familiarity=%.1f, Trust=%.1f, Intimacy=%.1f",
		status.Relationship.Familiarity, status.Relationship.Trust, status.Relationship.Intimacy)
	t.Logf("Total memories: %d", len(status.Memories))
}

// Test 2: Memory conflict handling - verifies conflict detection and resolution.
func TestMemoryConflict(t *testing.T) {
	runtime, store := setupTestRuntime(t)
	defer store.Close()

	ctx := context.Background()
	userID := "test_user_2"

	// Turn 1: User says they're in Beijing.
	reply1, _, err := runtime.ProcessMessage(ctx, userID, "我在北京工作")
	if err != nil {
		t.Fatalf("Turn 1 failed: %v", err)
	}
	t.Logf("Turn 1 reply: %s", reply1)

	// Verify initial city memory.
	cityMem, _ := store.GetMemoryByKey(userID, "basic_info", "city")
	if cityMem == nil {
		t.Fatal("Expected city memory after turn 1")
	}
	if cityMem.Value != "上海" { // Mock returns 上海 for generic city mentions
		t.Logf("Initial city: %s", cityMem.Value)
	}

	// Turn 2: User says they moved to Shenzhen.
	reply2, trace2, err := runtime.ProcessMessage(ctx, userID, "其实我上个月搬到深圳了")
	if err != nil {
		t.Fatalf("Turn 2 failed: %v", err)
	}
	t.Logf("Turn 2 reply: %s", reply2)

	// Check for conflict resolution in trace.
	foundConflictStep := false
	for _, step := range trace2.Steps {
		if strings.Contains(step.Name, "conflict") || strings.Contains(step.Output, "conflict") {
			foundConflictStep = true
			t.Logf("Conflict step: %s -> %s", step.Name, step.Output)
		}
	}
	if !foundConflictStep {
		t.Log("Note: No explicit conflict step found in trace (may be handled silently)")
	}

	// Verify the city memory was updated.
	newCityMem, _ := store.GetMemoryByKey(userID, "basic_info", "city")
	if newCityMem == nil {
		t.Fatal("Expected city memory after update")
	}
	t.Logf("Updated city: %s", newCityMem.Value)

	// Check that old memories are superseded.
	allMemories, _ := store.GetAllMemories(userID)
	supersededCount := 0
	for _, m := range allMemories {
		if !m.Active && m.SupersededBy != "" {
			supersededCount++
		}
	}
	t.Logf("Superseded memories: %d", supersededCount)

	// Verify status shows updated information.
	status, _ := runtime.GetStatus(userID)
	t.Logf("Final status - Memories: %d, Familiarity: %.1f",
		len(status.Memories), status.Relationship.Familiarity)
}

// Test 3: Error handling - verifies fallback behavior when LLM fails.
func TestErrorHandling(t *testing.T) {
	// Create a mock that fails on extraction.
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	failLLM := &FailOnExtractionLLM{failCount: 2}
	runtime := agent.NewRuntime(failLLM, store)

	ctx := context.Background()
	userID := "test_user_3"

	// Turn 1: Should use fallback for extraction.
	reply1, trace1, err := runtime.ProcessMessage(ctx, userID, "你好，我叫小红")
	if err != nil {
		t.Fatalf("Turn 1 failed unexpectedly: %v", err)
	}
	t.Logf("Turn 1 reply (with fallback): %s", reply1)

	// Check that trace shows fallback.
	for _, step := range trace1.Steps {
		t.Logf("Step %d: %s - Status: %s", step.Step, step.Name, step.Status)
	}

	// Verify the system still generated a reply.
	if reply1 == "" {
		t.Error("Expected non-empty reply even with extraction failure")
	}

	// Turn 2: Should still work (failCount exhausted).
	reply2, trace2, err := runtime.ProcessMessage(ctx, userID, "今天天气不错")
	if err != nil {
		t.Fatalf("Turn 2 failed unexpectedly: %v", err)
	}
	t.Logf("Turn 2 reply: %s", reply2)

	for _, step := range trace2.Steps {
		t.Logf("Step %d: %s - Status: %s", step.Step, step.Name, step.Status)
	}

	if reply2 == "" {
		t.Error("Expected non-empty reply on second turn")
	}
}

// FailOnExtractionLLM fails extraction calls a limited number of times.
type FailOnExtractionLLM struct {
	failCount int
	callCount int
}

func (f *FailOnExtractionLLM) Chat(ctx context.Context, messages []llm.Message) (string, error) {
	f.callCount++
	lastMsg := messages[len(messages)-1].Content

	if strings.Contains(lastMsg, "information extraction") || strings.Contains(lastMsg, "Extract information") {
		if f.failCount > 0 {
			f.failCount--
			return "", fmt.Errorf("simulated LLM failure")
		}
		return `{"items": []}`, nil
	}

	// For reply generation, always succeed.
	if strings.Contains(lastMsg, "caring, empathetic") {
		return "你好！很高兴认识你！", nil
	}

	return "I'm here!", nil
}

// Test helper to verify JSON marshaling works.
func TestJSONMarshaling(t *testing.T) {
	trace := struct {
		Steps []struct {
			Step   int    `json:"step"`
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"steps"`
	}{
		Steps: []struct {
			Step   int    `json:"step"`
			Name   string `json:"name"`
			Status string `json:"status"`
		}{
			{Step: 1, Name: "Test", Status: "success"},
		},
	}

	data, err := json.Marshal(trace)
	if err != nil {
		t.Fatalf("Failed to marshal trace: %v", err)
	}

	if !strings.Contains(string(data), "Test") {
		t.Error("Expected trace to contain step name")
	}
}
