// PicoClaw - Ultra-lightweight personal AI agent
// Inspired by and based on nanobot: https://github.com/HKUDS/nanobot
// License: MIT
//
// Copyright (c) 2026 PicoClaw contributors

package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mockUserIDProvider is a minimal mock for testing
type mockUserIDProvider struct {
	userID string
}

func (m *mockUserIDProvider) GetCurrentUserID() string {
	return m.userID
}

// TestMemoryReadTool_NoMemory verifies behavior when no memory exists
func TestMemoryReadTool_NoMemory(t *testing.T) {
	tmpDir := t.TempDir()
	cb := &mockUserIDProvider{userID: ""}
	tool := NewMemoryReadTool(tmpDir, cb)

	ctx := context.Background()
	result := tool.Execute(ctx, map[string]interface{}{})

	if result.IsError {
		t.Errorf("Expected success, got error: %s", result.ForLLM)
	}

	if !strings.Contains(result.ForLLM, "No memory data found") {
		t.Errorf("Expected 'No memory data found' message, got: %s", result.ForLLM)
	}
}

// TestMemoryReadTool_WithMemory verifies reading existing memory
func TestMemoryReadTool_WithMemory(t *testing.T) {
	tmpDir := t.TempDir()
	memoryDir := filepath.Join(tmpDir, "memory")
	os.MkdirAll(memoryDir, 0755)

	// Create long-term memory
	memoryContent := "# My Memory\n\nThis is important information."
	os.WriteFile(filepath.Join(memoryDir, "MEMORY.md"), []byte(memoryContent), 0644)

	// Create daily notes
	monthDir := filepath.Join(memoryDir, "202602") // February 2026
	os.MkdirAll(monthDir, 0755)
	dailyNote := "# 2026-02-20\n\nToday I learned something new."
	os.WriteFile(filepath.Join(monthDir, "20260220.md"), []byte(dailyNote), 0644)

	cb := &mockUserIDProvider{userID: ""}
	tool := NewMemoryReadTool(tmpDir, cb)

	ctx := context.Background()
	result := tool.Execute(ctx, map[string]interface{}{})

	if result.IsError {
		t.Errorf("Expected success, got error: %s", result.ForLLM)
	}

	if !strings.Contains(result.ForLLM, "My Memory") {
		t.Errorf("Expected memory content, got: %s", result.ForLLM)
	}

	if !strings.Contains(result.ForLLM, "Long-term Memory") {
		t.Errorf("Expected 'Long-term Memory' section, got: %s", result.ForLLM)
	}

	if !strings.Contains(result.ForLLM, "Recent Daily Notes") {
		t.Errorf("Expected 'Recent Daily Notes' section, got: %s", result.ForLLM)
	}
}

// TestMemoryReadTool_PerUser verifies per-user memory isolation
func TestMemoryReadTool_PerUser(t *testing.T) {
	tmpDir := t.TempDir()

	// Create memory for user1
	user1Dir := filepath.Join(tmpDir, "memory", "users", "user123")
	os.MkdirAll(user1Dir, 0755)
	user1Memory := "# User 123 Memory\n\nFavorite color: blue"
	os.WriteFile(filepath.Join(user1Dir, "MEMORY.md"), []byte(user1Memory), 0644)

	// Create memory for user2
	user2Dir := filepath.Join(tmpDir, "memory", "users", "user456")
	os.MkdirAll(user2Dir, 0755)
	user2Memory := "# User 456 Memory\n\nFavorite color: red"
	os.WriteFile(filepath.Join(user2Dir, "MEMORY.md"), []byte(user2Memory), 0644)

	// Test user1
	cb1 := &mockUserIDProvider{userID: "user123"}
	tool1 := NewMemoryReadTool(tmpDir, cb1)
	ctx := context.Background()
	result1 := tool1.Execute(ctx, map[string]interface{}{})

	if result1.IsError {
		t.Errorf("Expected success for user1, got error: %s", result1.ForLLM)
	}

	if !strings.Contains(result1.ForLLM, "blue") {
		t.Errorf("Expected user1's memory (blue), got: %s", result1.ForLLM)
	}

	if strings.Contains(result1.ForLLM, "red") {
		t.Errorf("User1 should not see user2's memory (red), got: %s", result1.ForLLM)
	}

	// Test user2
	cb2 := &mockUserIDProvider{userID: "user456"}
	tool2 := NewMemoryReadTool(tmpDir, cb2)
	result2 := tool2.Execute(ctx, map[string]interface{}{})

	if result2.IsError {
		t.Errorf("Expected success for user2, got error: %s", result2.ForLLM)
	}

	if !strings.Contains(result2.ForLLM, "red") {
		t.Errorf("Expected user2's memory (red), got: %s", result2.ForLLM)
	}

	if strings.Contains(result2.ForLLM, "blue") {
		t.Errorf("User2 should not see user1's memory (blue), got: %s", result2.ForLLM)
	}
}

// TestMemoryWriteTool_Overwrite verifies overwrite mode
func TestMemoryWriteTool_Overwrite(t *testing.T) {
	tmpDir := t.TempDir()
	memoryDir := filepath.Join(tmpDir, "memory")
	os.MkdirAll(memoryDir, 0755)

	// Write initial content
	memoryFile := filepath.Join(memoryDir, "MEMORY.md")
	os.WriteFile(memoryFile, []byte("old content"), 0644)

	cb := &mockUserIDProvider{userID: ""}
	tool := NewMemoryWriteTool(tmpDir, cb)

	ctx := context.Background()
	args := map[string]interface{}{
		"content": "new content",
		"mode":    "overwrite",
	}

	result := tool.Execute(ctx, args)

	if result.IsError {
		t.Errorf("Expected success, got error: %s", result.ForLLM)
	}

	// Verify file was overwritten
	data, err := os.ReadFile(memoryFile)
	if err != nil {
		t.Fatalf("Failed to read memory file: %v", err)
	}

	content := string(data)
	if content != "new content" {
		t.Errorf("Expected 'new content', got: %s", content)
	}
}

// TestMemoryWriteTool_Append verifies append mode
func TestMemoryWriteTool_Append(t *testing.T) {
	tmpDir := t.TempDir()
	memoryDir := filepath.Join(tmpDir, "memory")
	os.MkdirAll(memoryDir, 0755)

	// Write initial content
	memoryFile := filepath.Join(memoryDir, "MEMORY.md")
	os.WriteFile(memoryFile, []byte("existing content"), 0644)

	cb := &mockUserIDProvider{userID: ""}
	tool := NewMemoryWriteTool(tmpDir, cb)

	ctx := context.Background()
	args := map[string]interface{}{
		"content": "appended content",
		"mode":    "append",
	}

	result := tool.Execute(ctx, args)

	if result.IsError {
		t.Errorf("Expected success, got error: %s", result.ForLLM)
	}

	// Verify file was appended
	data, err := os.ReadFile(memoryFile)
	if err != nil {
		t.Fatalf("Failed to read memory file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "existing content") {
		t.Errorf("Expected existing content to remain, got: %s", content)
	}

	if !strings.Contains(content, "appended content") {
		t.Errorf("Expected appended content, got: %s", content)
	}
}

// TestMemoryWriteTool_PerUser verifies per-user write isolation
func TestMemoryWriteTool_PerUser(t *testing.T) {
	tmpDir := t.TempDir()

	cb1 := &mockUserIDProvider{userID: "user123"}
	tool1 := NewMemoryWriteTool(tmpDir, cb1)

	ctx := context.Background()
	args1 := map[string]interface{}{
		"content": "user 123 data",
	}

	result1 := tool1.Execute(ctx, args1)

	if result1.IsError {
		t.Errorf("Expected success for user1, got error: %s", result1.ForLLM)
	}

	// Verify user1's file exists
	user1File := filepath.Join(tmpDir, "memory", "users", "user123", "MEMORY.md")
	data, err := os.ReadFile(user1File)
	if err != nil {
		t.Fatalf("Failed to read user1 memory file: %v", err)
	}

	if string(data) != "user 123 data" {
		t.Errorf("Expected 'user 123 data', got: %s", string(data))
	}

	// Verify user2's file does not exist
	user2File := filepath.Join(tmpDir, "memory", "users", "user456", "MEMORY.md")
	if _, err := os.ReadFile(user2File); err == nil {
		t.Errorf("User2's file should not exist")
	}
}

// TestMemoryWriteTool_MissingContent verifies error handling for missing content
func TestMemoryWriteTool_MissingContent(t *testing.T) {
	tmpDir := t.TempDir()
	cb := &mockUserIDProvider{userID: ""}
	tool := NewMemoryWriteTool(tmpDir, cb)

	ctx := context.Background()
	args := map[string]interface{}{}

	result := tool.Execute(ctx, args)

	if !result.IsError {
		t.Errorf("Expected error when content is missing")
	}

	if !strings.Contains(result.ForLLM, "content is required") {
		t.Errorf("Expected 'content is required' message, got: %s", result.ForLLM)
	}
}

// TestMemoryAppendTool_CreateNew verifies creating a new daily note
func TestMemoryAppendTool_CreateNew(t *testing.T) {
	tmpDir := t.TempDir()
	memoryDir := filepath.Join(tmpDir, "memory")
	os.MkdirAll(memoryDir, 0755)

	cb := &mockUserIDProvider{userID: ""}
	tool := NewMemoryAppendTool(tmpDir, cb)

	ctx := context.Background()
	args := map[string]interface{}{
		"content": "today's note",
	}

	result := tool.Execute(ctx, args)

	if result.IsError {
		t.Errorf("Expected success, got error: %s", result.ForLLM)
	}

	// Verify daily note was created
	// The file path includes current date, so we need to check the directory structure
	monthDir := filepath.Join(memoryDir, "202602") // February 2026 (or current month)
	entries, err := os.ReadDir(monthDir)
	if err != nil {
		// Try different month directories
		entries, err = os.ReadDir(filepath.Join(memoryDir, "202601"))
		if err != nil {
			t.Fatalf("Failed to read month directory: %v", err)
		}
	}

	if len(entries) == 0 {
		t.Errorf("Expected daily note file to be created")
	}

	// Read the created file
	dailyFile := filepath.Join(monthDir, entries[0].Name())
	data, err := os.ReadFile(dailyFile)
	if err != nil {
		t.Fatalf("Failed to read daily note: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "today's note") {
		t.Errorf("Expected 'today's note' in daily file, got: %s", content)
	}
}

// TestMemoryAppendTool_Append verifies appending to existing daily note
func TestMemoryAppendTool_Append(t *testing.T) {
	tmpDir := t.TempDir()
	memoryDir := filepath.Join(tmpDir, "memory")

	// Create today's directory with existing note
	monthDir := filepath.Join(memoryDir, "202602")
	os.MkdirAll(monthDir, 0755)
	dailyFile := filepath.Join(monthDir, "20260220.md")
	os.WriteFile(dailyFile, []byte("# 2026-02-20\n\nexisting note"), 0644)

	cb := &mockUserIDProvider{userID: ""}
	tool := NewMemoryAppendTool(tmpDir, cb)

	ctx := context.Background()
	args := map[string]interface{}{
		"content": "appended note",
	}

	result := tool.Execute(ctx, args)

	if result.IsError {
		t.Errorf("Expected success, got error: %s", result.ForLLM)
	}

	// Verify content was appended
	data, err := os.ReadFile(dailyFile)
	if err != nil {
		t.Fatalf("Failed to read daily note: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "existing note") {
		t.Errorf("Expected existing note to remain, got: %s", content)
	}

	if !strings.Contains(content, "appended note") {
		t.Errorf("Expected appended note, got: %s", content)
	}
}

// TestMemoryAppendTool_PerUser verifies per-user daily notes
func TestMemoryAppendTool_PerUser(t *testing.T) {
	tmpDir := t.TempDir()

	cb1 := &mockUserIDProvider{userID: "user123"}
	tool1 := NewMemoryAppendTool(tmpDir, cb1)

	ctx := context.Background()
	args1 := map[string]interface{}{
		"content": "user 123 note",
	}

	result1 := tool1.Execute(ctx, args1)

	if result1.IsError {
		t.Errorf("Expected success for user1, got error: %s", result1.ForLLM)
	}

	// Verify user1's daily note exists
	user1MonthDir := filepath.Join(tmpDir, "memory", "users", "user123", "202602")
	entries, err := os.ReadDir(user1MonthDir)
	if err != nil {
		t.Fatalf("Failed to read user1 month directory: %v", err)
	}

	if len(entries) == 0 {
		t.Errorf("Expected daily note file for user1")
	}

	// Verify user2's directory does not exist
	user2MonthDir := filepath.Join(tmpDir, "memory", "users", "user456", "202602")
	if _, err := os.ReadDir(user2MonthDir); err == nil {
		t.Errorf("User2's daily note should not exist")
	}
}

// TestMemoryAppendTool_MissingContent verifies error handling for missing content
func TestMemoryAppendTool_MissingContent(t *testing.T) {
	tmpDir := t.TempDir()
	cb := &mockUserIDProvider{userID: ""}
	tool := NewMemoryAppendTool(tmpDir, cb)

	ctx := context.Background()
	args := map[string]interface{}{}

	result := tool.Execute(ctx, args)

	if !result.IsError {
		t.Errorf("Expected error when content is missing")
	}

	if !strings.Contains(result.ForLLM, "content is required") {
		t.Errorf("Expected 'content is required' message, got: %s", result.ForLLM)
	}
}

// TestMemoryTool_CLI_Mode verifies CLI mode uses shared memory (no user ID)
func TestMemoryTool_CLI_Mode(t *testing.T) {
	tmpDir := t.TempDir()

	// Create shared memory directory
	memoryDir := filepath.Join(tmpDir, "memory")
	os.MkdirAll(memoryDir, 0755)

	// Write to shared memory
	memoryFile := filepath.Join(memoryDir, "MEMORY.md")
	os.WriteFile(memoryFile, []byte("shared memory content"), 0644)

	// Test read
	cb := &mockUserIDProvider{userID: ""}
	readTool := NewMemoryReadTool(tmpDir, cb)
	ctx := context.Background()
	result := readTool.Execute(ctx, map[string]interface{}{})

	if result.IsError {
		t.Errorf("Expected success, got error: %s", result.ForLLM)
	}

	if !strings.Contains(result.ForLLM, "shared memory content") {
		t.Errorf("Expected shared memory content, got: %s", result.ForLLM)
	}

	// Test write
	writeTool := NewMemoryWriteTool(tmpDir, cb)
	args := map[string]interface{}{
		"content": "new shared content",
	}
	result = writeTool.Execute(ctx, args)

	if result.IsError {
		t.Errorf("Expected success, got error: %s", result.ForLLM)
	}

	// Verify it wrote to shared location, not users directory
	usersDir := filepath.Join(tmpDir, "memory", "users")
	if _, err := os.ReadDir(usersDir); err == nil {
		entries, _ := os.ReadDir(usersDir)
		if len(entries) > 0 {
			t.Errorf("Expected no user directories in CLI mode")
		}
	}

	// Verify shared file was updated
	data, err := os.ReadFile(memoryFile)
	if err != nil {
		t.Fatalf("Failed to read shared memory file: %v", err)
	}

	if string(data) != "new shared content" {
		t.Errorf("Expected 'new shared content', got: %s", string(data))
	}
}
