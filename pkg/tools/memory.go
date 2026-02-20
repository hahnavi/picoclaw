// PicoClaw - Ultra-lightweight personal AI agent
// Inspired by and based on nanobot: https://github.com/HKUDS/nanobot
// License: MIT
//
// Copyright (c) 2026 PicoClaw contributors

package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// UserIDProvider is an interface for getting the current user ID.
// This avoids import cycles between tools and agent packages.
type UserIDProvider interface {
	GetCurrentUserID() string
}

// MemoryReadTool reads the current user's memory (long-term memory and recent daily notes).
// For Discord users, this reads from workspace/memory/users/<USER_ID>/MEMORY.md
// For CLI mode, this reads from workspace/memory/MEMORY.md
type MemoryReadTool struct {
	userIDProvider UserIDProvider
	workspace      string
}

// NewMemoryReadTool creates a new memory_read tool.
func NewMemoryReadTool(workspace string, userIDProvider UserIDProvider) *MemoryReadTool {
	return &MemoryReadTool{
		userIDProvider: userIDProvider,
		workspace:      workspace,
	}
}

func (t *MemoryReadTool) Name() string {
	return "memory_read"
}

func (t *MemoryReadTool) Description() string {
	return "Read the current user's memory (long-term memory and recent daily notes). For Discord users, this reads per-user memory. For CLI mode, this reads shared memory."
}

func (t *MemoryReadTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

func (t *MemoryReadTool) Execute(ctx context.Context, args map[string]interface{}) *ToolResult {
	userID := ""
	if t.userIDProvider != nil {
		userID = t.userIDProvider.GetCurrentUserID()
	}

	var memoryDir string
	if userID != "" {
		memoryDir = filepath.Join(t.workspace, "memory", "users", userID)
	} else {
		memoryDir = filepath.Join(t.workspace, "memory")
	}

	// Read long-term memory
	var parts []string
	memoryFile := filepath.Join(memoryDir, "MEMORY.md")
	if data, err := os.ReadFile(memoryFile); err == nil {
		parts = append(parts, "## Long-term Memory\n\n"+string(data))
	}

	// Read recent daily notes (last 3 days)
	var notes []string
	for i := 0; i < 3; i++ {
		date := time.Now().AddDate(0, 0, -i)
		dateStr := date.Format("20060102") // YYYYMMDD
		monthDir := dateStr[:6]            // YYYYMM
		notePath := filepath.Join(memoryDir, monthDir, dateStr+".md")

		if data, err := os.ReadFile(notePath); err == nil {
			notes = append(notes, string(data))
		}
	}

	if len(notes) > 0 {
		parts = append(parts, "## Recent Daily Notes (Last 3 Days)\n\n"+strings.Join(notes, "\n\n---\n\n"))
	}

	if len(parts) == 0 {
		return NewToolResult("# Memory\n\nNo memory data found for this user.")
	}

	return NewToolResult("# Memory\n\n" + strings.Join(parts, "\n\n---\n\n"))
}

// MemoryWriteTool writes content to the current user's long-term memory file (MEMORY.md).
// For Discord users, this writes to workspace/memory/users/<USER_ID>/MEMORY.md
// For CLI mode, this writes to workspace/memory/MEMORY.md
type MemoryWriteTool struct {
	userIDProvider UserIDProvider
	workspace      string
}

// NewMemoryWriteTool creates a new memory_write tool.
func NewMemoryWriteTool(workspace string, userIDProvider UserIDProvider) *MemoryWriteTool {
	return &MemoryWriteTool{
		userIDProvider: userIDProvider,
		workspace:      workspace,
	}
}

func (t *MemoryWriteTool) Name() string {
	return "memory_write"
}

func (t *MemoryWriteTool) Description() string {
	return "Write content to the current user's long-term memory file (MEMORY.md). Use overwrite mode to replace the entire file, or append mode to add to the end. For Discord users, this writes to per-user memory. For CLI mode, this writes to shared memory."
}

func (t *MemoryWriteTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"content": map[string]interface{}{
				"type":        "string",
				"description": "Content to write to MEMORY.md",
			},
			"mode": map[string]interface{}{
				"type":        "string",
				"description": "Write mode: 'overwrite' to replace the entire file, 'append' to add to the end (default: 'overwrite')",
				"enum":        []string{"overwrite", "append"},
			},
		},
		"required": []string{"content"},
	}
}

func (t *MemoryWriteTool) Execute(ctx context.Context, args map[string]interface{}) *ToolResult {
	content, ok := args["content"].(string)
	if !ok {
		return ErrorResult("content is required")
	}

	userID := ""
	if t.userIDProvider != nil {
		userID = t.userIDProvider.GetCurrentUserID()
	}

	var memoryFile string
	var memoryDir string
	if userID != "" {
		memoryDir = filepath.Join(t.workspace, "memory", "users", userID)
		memoryFile = filepath.Join(memoryDir, "MEMORY.md")
	} else {
		memoryDir = filepath.Join(t.workspace, "memory")
		memoryFile = filepath.Join(memoryDir, "MEMORY.md")
	}

	// Ensure directory exists
	if err := os.MkdirAll(memoryDir, 0755); err != nil {
		return ErrorResult(fmt.Sprintf("failed to create memory directory: %v", err))
	}

	mode, _ := args["mode"].(string)
	if mode == "" {
		mode = "overwrite"
	}

	var dataToWrite []byte
	if mode == "append" {
		// Read existing content and append
		existingContent := ""
		if data, err := os.ReadFile(memoryFile); err == nil {
			existingContent = string(data)
		}
		dataToWrite = []byte(existingContent + "\n" + content)
	} else {
		// Overwrite mode
		dataToWrite = []byte(content)
	}

	if err := os.WriteFile(memoryFile, dataToWrite, 0644); err != nil {
		return ErrorResult(fmt.Sprintf("failed to write memory file: %v", err))
	}

	userInfo := "shared"
	if userID != "" {
		userInfo = fmt.Sprintf("user %s", userID)
	}
	return SilentResult(fmt.Sprintf("Memory updated for %s (mode: %s)", userInfo, mode))
}

// MemoryAppendTool appends content to the current user's today daily note.
// For Discord users, this appends to workspace/memory/users/<USER_ID>/YYYYMM/YYYYMMDD.md
// For CLI mode, this appends to workspace/memory/YYYYMM/YYYYMMDD.md
type MemoryAppendTool struct {
	userIDProvider UserIDProvider
	workspace      string
}

// NewMemoryAppendTool creates a new memory_append tool.
func NewMemoryAppendTool(workspace string, userIDProvider UserIDProvider) *MemoryAppendTool {
	return &MemoryAppendTool{
		userIDProvider: userIDProvider,
		workspace:      workspace,
	}
}

func (t *MemoryAppendTool) Name() string {
	return "memory_append"
}

func (t *MemoryAppendTool) Description() string {
	return "Append content to the current user's today daily note. Creates a new file with a date header if it doesn't exist. For Discord users, this writes to per-user daily notes. For CLI mode, this writes to shared daily notes."
}

func (t *MemoryAppendTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"content": map[string]interface{}{
				"type":        "string",
				"description": "Content to append to today's daily note",
			},
		},
		"required": []string{"content"},
	}
}

func (t *MemoryAppendTool) Execute(ctx context.Context, args map[string]interface{}) *ToolResult {
	content, ok := args["content"].(string)
	if !ok {
		return ErrorResult("content is required")
	}

	userID := ""
	if t.userIDProvider != nil {
		userID = t.userIDProvider.GetCurrentUserID()
	}

	var baseDir string
	if userID != "" {
		baseDir = filepath.Join(t.workspace, "memory", "users", userID)
	} else {
		baseDir = filepath.Join(t.workspace, "memory")
	}

	today := time.Now().Format("20060102") // YYYYMMDD
	monthDir := today[:6]                  // YYYYMM
	todayFile := filepath.Join(baseDir, monthDir, today+".md")

	// Ensure month directory exists
	monthPath := filepath.Join(baseDir, monthDir)
	if err := os.MkdirAll(monthPath, 0755); err != nil {
		return ErrorResult(fmt.Sprintf("failed to create month directory: %v", err))
	}

	var existingContent string
	if data, err := os.ReadFile(todayFile); err == nil {
		existingContent = string(data)
	}

	var newContent string
	if existingContent == "" {
		// Add header for new day
		header := fmt.Sprintf("# %s\n\n", time.Now().Format("2006-01-02"))
		newContent = header + content
	} else {
		// Append to existing content
		newContent = existingContent + "\n" + content
	}

	if err := os.WriteFile(todayFile, []byte(newContent), 0644); err != nil {
		return ErrorResult(fmt.Sprintf("failed to write daily note: %v", err))
	}

	userInfo := "shared"
	if userID != "" {
		userInfo = fmt.Sprintf("user %s", userID)
	}
	return SilentResult(fmt.Sprintf("Daily note updated for %s", userInfo))
}
