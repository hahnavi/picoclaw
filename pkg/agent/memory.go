// PicoClaw - Ultra-lightweight personal AI agent
// Inspired by and based on nanobot: https://github.com/HKUDS/nanobot
// License: MIT
//
// Copyright (c) 2026 PicoClaw contributors

package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MemoryStore manages persistent memory for the agent.
// - Long-term memory: memory/MEMORY.md
// - Daily notes: memory/YYYYMM/YYYYMMDD.md
type MemoryStore struct {
	workspace  string
	memoryDir  string
	memoryFile string
}

// NewMemoryStore creates a new MemoryStore with the given workspace path.
// It ensures the memory directory exists.
func NewMemoryStore(workspace string) *MemoryStore {
	memoryDir := filepath.Join(workspace, "memory")
	memoryFile := filepath.Join(memoryDir, "MEMORY.md")

	// Ensure memory directory exists
	os.MkdirAll(memoryDir, 0755)

	return &MemoryStore{
		workspace:  workspace,
		memoryDir:  memoryDir,
		memoryFile: memoryFile,
	}
}

// getTodayFile returns the path to today's daily note file (memory/YYYYMM/YYYYMMDD.md).
func (ms *MemoryStore) getTodayFile() string {
	today := time.Now().Format("20060102") // YYYYMMDD
	monthDir := today[:6]                  // YYYYMM
	filePath := filepath.Join(ms.memoryDir, monthDir, today+".md")
	return filePath
}

// ReadLongTerm reads the long-term memory (MEMORY.md).
// Returns empty string if the file doesn't exist.
func (ms *MemoryStore) ReadLongTerm() string {
	if data, err := os.ReadFile(ms.memoryFile); err == nil {
		return string(data)
	}
	return ""
}

// WriteLongTerm writes content to the long-term memory file (MEMORY.md).
func (ms *MemoryStore) WriteLongTerm(content string) error {
	return os.WriteFile(ms.memoryFile, []byte(content), 0644)
}

// ReadToday reads today's daily note.
// Returns empty string if the file doesn't exist.
func (ms *MemoryStore) ReadToday() string {
	todayFile := ms.getTodayFile()
	if data, err := os.ReadFile(todayFile); err == nil {
		return string(data)
	}
	return ""
}

// AppendToday appends content to today's daily note.
// If the file doesn't exist, it creates a new file with a date header.
func (ms *MemoryStore) AppendToday(content string) error {
	todayFile := ms.getTodayFile()

	// Ensure month directory exists
	monthDir := filepath.Dir(todayFile)
	os.MkdirAll(monthDir, 0755)

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

	return os.WriteFile(todayFile, []byte(newContent), 0644)
}

// GetRecentDailyNotes returns daily notes from the last N days.
// Contents are joined with "---" separator.
func (ms *MemoryStore) GetRecentDailyNotes(days int) string {
	var sb strings.Builder
	first := true

	for i := 0; i < days; i++ {
		date := time.Now().AddDate(0, 0, -i)
		dateStr := date.Format("20060102") // YYYYMMDD
		monthDir := dateStr[:6]            // YYYYMM
		filePath := filepath.Join(ms.memoryDir, monthDir, dateStr+".md")

		if data, err := os.ReadFile(filePath); err == nil {
			if !first {
				sb.WriteString("\n\n---\n\n")
			}
			sb.Write(data)
			first = false
		}
	}

	return sb.String()
}

// getUserMemoryDir returns the user-specific memory directory.
// If userID is empty, returns the base memory directory.
func (ms *MemoryStore) getUserMemoryDir(userID string) string {
	if userID == "" {
		return ms.memoryDir
	}
	return filepath.Join(ms.memoryDir, "users", userID)
}

// getUserMemoryFile returns the user-specific long-term memory file path.
// If userID is empty, returns the base memory file.
func (ms *MemoryStore) getUserMemoryFile(userID string) string {
	if userID == "" {
		return ms.memoryFile
	}
	return filepath.Join(ms.getUserMemoryDir(userID), "MEMORY.md")
}

// getUserTodayFile returns the path to today's daily note file for a user.
// Format: memory/users/<userID>/YYYYMM/YYYYMMDD.md
// If userID is empty, returns the base today file.
func (ms *MemoryStore) getUserTodayFile(userID string) string {
	if userID == "" {
		return ms.getTodayFile()
	}
	today := time.Now().Format("20060102") // YYYYMMDD
	monthDir := today[:6]                  // YYYYMM
	filePath := filepath.Join(ms.getUserMemoryDir(userID), monthDir, today+".md")
	return filePath
}

// ReadUserLongTerm reads the long-term memory for a specific user.
// If userID is empty, reads from the base memory file.
// Returns empty string if the file doesn't exist.
func (ms *MemoryStore) ReadUserLongTerm(userID string) string {
	memoryFile := ms.getUserMemoryFile(userID)
	if data, err := os.ReadFile(memoryFile); err == nil {
		return string(data)
	}
	return ""
}

// WriteUserLongTerm writes content to the user's long-term memory file.
// If userID is empty, writes to the base memory file.
// Creates the user memory directory if it doesn't exist.
func (ms *MemoryStore) WriteUserLongTerm(userID string, content string) error {
	memoryFile := ms.getUserMemoryFile(userID)

	// Ensure directory exists
	if userID != "" {
		userDir := ms.getUserMemoryDir(userID)
		if err := os.MkdirAll(userDir, 0755); err != nil {
			return err
		}
	}

	return os.WriteFile(memoryFile, []byte(content), 0644)
}

// ReadUserToday reads today's daily note for a specific user.
// If userID is empty, reads from the base today file.
// Returns empty string if the file doesn't exist.
func (ms *MemoryStore) ReadUserToday(userID string) string {
	todayFile := ms.getUserTodayFile(userID)
	if data, err := os.ReadFile(todayFile); err == nil {
		return string(data)
	}
	return ""
}

// AppendUserToday appends content to the user's daily note.
// If userID is empty, appends to the base today file.
// If the file doesn't exist, it creates a new file with a date header.
func (ms *MemoryStore) AppendUserToday(userID string, content string) error {
	todayFile := ms.getUserTodayFile(userID)

	// Ensure month directory exists
	monthDir := filepath.Dir(todayFile)
	if err := os.MkdirAll(monthDir, 0755); err != nil {
		return err
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

	return os.WriteFile(todayFile, []byte(newContent), 0644)
}

// GetUserMemoryContext returns formatted memory context for a specific user.
// If userID is empty, returns the base memory context.
// Includes long-term memory and recent daily notes.
func (ms *MemoryStore) GetUserMemoryContext(userID string) string {
	// Long-term memory
	longTerm := ms.ReadUserLongTerm(userID)
	recentNotes := ms.GetRecentDailyNotesForUser(userID, 3)

	if longTerm == "" && recentNotes == "" {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("# Memory\n\n")

	if longTerm != "" {
		sb.WriteString("## Long-term Memory\n\n")
		sb.WriteString(longTerm)
	}

	if recentNotes != "" {
		if longTerm != "" {
			sb.WriteString("\n\n---\n\n")
		}
		sb.WriteString("## Recent Daily Notes\n\n")
		sb.WriteString(recentNotes)
	}

	return sb.String()
}

// GetRecentDailyNotesForUser returns daily notes from the last N days for a specific user.
// If userID is empty, returns notes from the base directory.
// Contents are joined with "---" separator.
func (ms *MemoryStore) GetRecentDailyNotesForUser(userID string, days int) string {
	var sb strings.Builder
	first := true
	baseDir := ms.getUserMemoryDir(userID)

	for i := 0; i < days; i++ {
		date := time.Now().AddDate(0, 0, -i)
		dateStr := date.Format("20060102") // YYYYMMDD
		monthDir := dateStr[:6]            // YYYYMM
		filePath := filepath.Join(baseDir, monthDir, dateStr+".md")

		if data, err := os.ReadFile(filePath); err == nil {
			if !first {
				sb.WriteString("\n\n---\n\n")
			}
			sb.Write(data)
			first = false
		}
	}

	return sb.String()
}

// GetMemoryContext returns formatted memory context for the agent prompt.
// Includes long-term memory and recent daily notes.
func (ms *MemoryStore) GetMemoryContext() string {
	return ms.GetUserMemoryContext("")
}
