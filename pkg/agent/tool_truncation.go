// PicoClaw - Ultra-lightweight personal AI agent
// Tool result truncation to prevent large file reads from consuming context

package agent

import (
	"strings"

	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/utils"
)

const (
	// MAX_TOOL_RESULT_CONTEXT_SHARE is the maximum fraction of context window
	// that a single tool result can consume (30%).
	MAX_TOOL_RESULT_CONTEXT_SHARE = 0.3

	// HARD_MAX_TOOL_RESULT_CHARS is the absolute maximum size for any tool result.
	// This prevents runaway memory usage even on large context models.
	HARD_MAX_TOOL_RESULT_CHARS = 400_000

	// MIN_TOOL_RESULT_CHARS is the minimum content to preserve when truncating.
	// This ensures at least some context is kept.
	MIN_TOOL_RESULT_CHARS = 2_000
)

// calculateMaxToolResultChars calculates the maximum allowed size for a tool result
// based on the context window size. Returns the limit in characters.
func calculateMaxToolResultChars(contextWindowTokens int) int {
	// Convert tokens to chars using 4 chars/token heuristic
	maxTokens := float64(contextWindowTokens) * MAX_TOOL_RESULT_CONTEXT_SHARE
	maxChars := int(maxTokens * 4)

	// Cap at hard maximum
	if maxChars > HARD_MAX_TOOL_RESULT_CHARS {
		return HARD_MAX_TOOL_RESULT_CHARS
	}

	// Ensure at least minimum
	if maxChars < MIN_TOOL_RESULT_CHARS {
		return MIN_TOOL_RESULT_CHARS
	}

	return maxChars
}

// truncateToolResultText truncates a tool result to fit within maxChars while
// preserving as much useful information as possible.
// Preserves the beginning and tries to truncate at newline boundaries.
func truncateToolResultText(text string, maxChars int) string {
	if len(text) <= maxChars {
		return text
	}

	// Preserve at least the minimum
	if maxChars <= MIN_TOOL_RESULT_CHARS {
		return utils.Truncate(text, maxChars)
	}

	// Try to find a clean break point (newline) near the limit
	// Look backwards from maxChars for a newline
	truncationPoint := maxChars
	searchStart := maxChars - 200 // Search up to 200 chars back
	if searchStart < 0 {
		searchStart = 0
	}

	// Find the last newline in our search range
	lastNewline := strings.LastIndex(text[searchStart:truncationPoint], "\n")
	if lastNewline != -1 {
		truncationPoint = searchStart + lastNewline + 1 // Keep the newline
	}

	result := text[:truncationPoint]
	truncatedCount := len(text) - truncationPoint

	logger.DebugCF("agent", "Tool result truncated",
		map[string]interface{}{
			"original_chars":   len(text),
			"truncated_chars":  truncationPoint,
			"dropped_chars":    truncatedCount,
			"max_limit":        maxChars,
		})

	return result + "\n[...truncated...]"
}

// TruncateToolResult truncates a tool result based on the context window size.
// This is a convenience function that combines calculateMaxToolResultChars
// and truncateToolResultText.
func TruncateToolResult(result string, contextWindowTokens int) string {
	maxChars := calculateMaxToolResultChars(contextWindowTokens)
	return truncateToolResultText(result, maxChars)
}
