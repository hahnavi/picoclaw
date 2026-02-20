// PicoClaw - Ultra-lightweight personal AI agent
// Bootstrap file truncation to prevent large bootstrap files from consuming context

package agent

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sipeed/picoclaw/pkg/logger"
)

// SessionType defines the type of session for bootstrap filtering.
type SessionType string

const (
	// SessionTypeMain is a regular user session (full bootstrap)
	SessionTypeMain SessionType = "main"
	// SessionTypeCron is a scheduled task session (minimal bootstrap, no memory)
	SessionTypeCron SessionType = "cron"
	// SessionTypeSubagent is a subagent session (minimal bootstrap)
	SessionTypeSubagent SessionType = "subagent"
	// SessionTypeHeartbeat is a heartbeat session (HEARTBEAT.md only)
	SessionTypeHeartbeat SessionType = "heartbeat"
)

const (
	// DEFAULT_BOOTSTRAP_MAX_CHARS is the default maximum size for a single bootstrap file.
	DEFAULT_BOOTSTRAP_MAX_CHARS = 20_000

	// DEFAULT_BOOTSTRAP_TOTAL_MAX_CHARS is the default total size across all bootstrap files.
	DEFAULT_BOOTSTRAP_TOTAL_MAX_CHARS = 24_000

	// BOOTSTRAP_HEAD_RATIO is the fraction of content to preserve from the beginning.
	BOOTSTRAP_HEAD_RATIO = 0.70

	// BOOTSTRAP_TAIL_RATIO is the fraction of content to preserve from the end.
	BOOTSTRAP_TAIL_RATIO = 0.20
)

// BootstrapConfig holds configuration for bootstrap file truncation.
type BootstrapConfig struct {
	MaxChars       int // Per-file maximum
	TotalMaxChars  int // Total across all files
}

// DefaultBootstrapConfig returns the default bootstrap truncation configuration.
func DefaultBootstrapConfig() BootstrapConfig {
	return BootstrapConfig{
		MaxChars:      DEFAULT_BOOTSTRAP_MAX_CHARS,
		TotalMaxChars: DEFAULT_BOOTSTRAP_TOTAL_MAX_CHARS,
	}
}

// trimBootstrapContent truncates a bootstrap file's content while preserving
// the most important parts (head and tail).
// Returns detailed truncation information in the marker message.
func trimBootstrapContent(content string, filename string, maxChars int) string {
	if len(content) <= maxChars {
		return content
	}

	headSize := int(float64(maxChars) * BOOTSTRAP_HEAD_RATIO)
	tailSize := int(float64(maxChars) * BOOTSTRAP_TAIL_RATIO)

	// Ensure head and tail don't overlap
	if headSize+tailSize > len(content) {
		headSize = len(content) - tailSize - 100 // Leave room for truncation marker
		if headSize < 0 {
			headSize = 0
		}
	}

	head := content[:headSize]
	tail := ""
	if tailSize > 0 && len(content) > headSize {
		tailStart := len(content) - tailSize
		if tailStart < headSize {
			tailStart = headSize
		}
		tail = content[tailStart:]
	}

	// Enhanced truncation marker with file details
	marker := fmt.Sprintf(
		"\n\n[...truncated %s: kept %d+%d chars of %d, read %s for full content...]\n\n",
		filename, headSize, tailSize, len(content), filename,
	)

	logger.DebugCF("agent", "Bootstrap file truncated",
		map[string]interface{}{
			"filename":         filename,
			"original_chars":   len(content),
			"truncated_chars":  headSize + len(tail),
			"head_ratio":       BOOTSTRAP_HEAD_RATIO,
			"tail_ratio":       BOOTSTRAP_TAIL_RATIO,
			"max_limit":        maxChars,
		})

	if tail != "" {
		return head + marker + tail
	}
	return head + "\n[...truncated " + filename + "...]"
}

// getBootstrapFilesForSession returns the list of bootstrap files to load
// based on the session type.
func getBootstrapFilesForSession(sessionType SessionType) []string {
	switch sessionType {
	case SessionTypeMain:
		// Full bootstrap for main sessions
		return []string{
			"AGENTS.md",
			"SOUL.md",
			"TOOLS.md",
			"IDENTITY.md",
			"USER.md",
			"HEARTBEAT.md",
		}
	case SessionTypeCron, SessionTypeSubagent:
		// Minimal bootstrap for cron/subagent sessions (no MEMORY.md for security)
		return []string{
			"AGENTS.md",
			"TOOLS.md",
		}
	case SessionTypeHeartbeat:
		// Only HEARTBEAT.md for heartbeat sessions
		return []string{
			"HEARTBEAT.md",
		}
	default:
		// Default to full bootstrap
		return []string{
			"AGENTS.md",
			"SOUL.md",
			"TOOLS.md",
			"IDENTITY.md",
			"USER.md",
			"HEARTBEAT.md",
		}
	}
}

// LoadBootstrapFiles loads bootstrap files with truncation applied.
// Returns the concatenated content of all bootstrap files, respecting both
// per-file and total budget limits. Uses session-based filtering.
func LoadBootstrapFiles(workspace string, config BootstrapConfig) string {
	return LoadBootstrapFilesForSession(workspace, config, SessionTypeMain)
}

// LoadBootstrapFilesForSession loads bootstrap files with session-based filtering.
// This allows different bootstrap content for main sessions, cron tasks, and subagents.
func LoadBootstrapFilesForSession(workspace string, config BootstrapConfig, sessionType SessionType) string {
	bootstrapFiles := getBootstrapFilesForSession(sessionType)

	var result string
	totalUsed := 0
	perFileLimit := config.MaxChars

	// Track which files were loaded
	loadedFiles := make([]string, 0)

	for _, filename := range bootstrapFiles {
		filePath := filepath.Join(workspace, filename)
		data, err := os.ReadFile(filePath)
		if err != nil {
			// Log missing file but continue
			logger.DebugCF("agent", "Bootstrap file not found, skipping",
				map[string]interface{}{
					"filename": filename,
					"path":     filePath,
				})
			continue
		}

		content := string(data)

		// Check if this file alone exceeds per-file limit
		if len(content) > perFileLimit {
			content = trimBootstrapContent(content, filename, perFileLimit)
		}

		// Check if adding this file would exceed total budget
		if totalUsed+len(content) > config.TotalMaxChars {
			// Reduce this file to fit remaining budget
			remaining := config.TotalMaxChars - totalUsed
			if remaining > 500 { // Only add if we have meaningful space left
				content = trimBootstrapContent(content, filename, remaining)
			} else {
				logger.DebugCF("agent", "Bootstrap file skipped due to total budget limit",
					map[string]interface{}{
						"filename":        filename,
						"total_used":      totalUsed,
						"total_limit":     config.TotalMaxChars,
						"remaining_space": remaining,
					})
				continue
			}
		}

		result += fmt.Sprintf("## %s\n\n%s\n\n", filename, content)
		totalUsed += len(content)
		loadedFiles = append(loadedFiles, filename)
	}

	// Log summary
	if len(loadedFiles) > 0 {
		logger.DebugCF("agent", "Bootstrap files loaded",
			map[string]interface{}{
				"session_type":   sessionType,
				"files_loaded":   loadedFiles,
				"total_chars":    totalUsed,
				"total_limit":    config.TotalMaxChars,
				"per_file_limit": perFileLimit,
			})
	}

	return result
}
