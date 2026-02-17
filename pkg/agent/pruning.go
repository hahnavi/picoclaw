// PicoClaw - Ultra-lightweight personal AI agent
// TTL-based context pruning to remove stale messages and old tool results

package agent

import (
	"time"

	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/providers"
)

// PruningMode defines the pruning strategy.
type PruningMode string

const (
	// PruningModeOff disables context pruning.
	PruningModeOff PruningMode = "off"

	// PruningModeCacheTTL enables TTL-based pruning similar to cache expiration.
	PruningModeCacheTTL PruningMode = "cache-ttl"
)

// PruningConfig holds configuration for context pruning.
type PruningConfig struct {
	Mode                 PruningMode // Pruning strategy
	TTL                  time.Duration
	KeepLastAssistants   int  // Number of recent assistant messages to preserve
	SoftTrimRatio        float64 // Fraction to trim softly (reduce content)
	HardClearRatio       float64 // Fraction to clear hard (remove entirely)
	MinPrunableToolChars int     // Minimum tool result size to consider pruning
}

// DefaultPruningConfig returns the default pruning configuration.
func DefaultPruningConfig() PruningConfig {
	return PruningConfig{
		Mode:                 PruningModeOff,
		TTL:                  1 * time.Hour,
		KeepLastAssistants:   4,
		SoftTrimRatio:        0.3,
		HardClearRatio:       0.5,
		MinPrunableToolChars: 1000,
	}
}

// PruningStats tracks statistics about pruning operations.
type PruningStats struct {
	MessagesRemoved   int
	ToolResultsRemoved int
	CharsSaved         int
}

// messageWithTimestamp extends Message with timestamp information for pruning.
// Since PicoClaw doesn't currently store timestamps in messages, we use
// position-based heuristics instead.
type messageWithTimestamp struct {
	Message   providers.Message
	Index     int // Position in history (used as proxy for age)
	Timestamp time.Time // Placeholder for future timestamp support
}

// pruneContextByTTL removes messages older than the configured TTL.
// Keeps the last N assistant messages to maintain conversation continuity.
func pruneContextByTTL(messages []providers.Message, config PruningConfig) ([]providers.Message, PruningStats) {
	stats := PruningStats{}

	if config.Mode != PruningModeCacheTTL {
		return messages, stats
	}

	if len(messages) == 0 {
		return messages, stats
	}

	// Since PicoClaw doesn't store message timestamps, we use position-based
	// heuristics: older messages are at the beginning of the list.
	// We keep recent messages and critical message types.

	// Keep last N assistant messages
	lastAssistantIndices := make([]int, 0)
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "assistant" {
			lastAssistantIndices = append([]int{i}, lastAssistantIndices...)
			if len(lastAssistantIndices) >= config.KeepLastAssistants {
				break
			}
		}
	}

	// Find the oldest assistant message we want to keep
	minKeepIndex := len(messages)
	if len(lastAssistantIndices) > 0 {
		minKeepIndex = lastAssistantIndices[0]
	}

	// Everything before minKeepIndex (except system messages) is pruneable
	// This is a simplification - with real timestamps we'd use config.TTL
	var pruned []providers.Message
	for i, msg := range messages {
		// Always keep system messages
		if msg.Role == "system" {
			pruned = append(pruned, msg)
			continue
		}

		// Keep messages after our cutoff point
		if i >= minKeepIndex {
			pruned = append(pruned, msg)
		} else {
			stats.MessagesRemoved++
			stats.CharsSaved += len(msg.Content)
		}
	}

	if stats.MessagesRemoved > 0 {
		logger.DebugCF("agent", "Context pruned by TTL",
			map[string]interface{}{
				"messages_removed": stats.MessagesRemoved,
				"chars_saved":      stats.CharsSaved,
				"remaining_count":  len(pruned),
			})
	}

	return pruned, stats
}

// pruneToolResults removes tool results that are below the minimum size threshold,
// prioritizing keeping recent results.
func pruneToolResults(messages []providers.Message, config PruningConfig) ([]providers.Message, PruningStats) {
	stats := PruningStats{}

	if config.Mode != PruningModeCacheTTL {
		return messages, stats
	}

	var pruned []providers.Message
	recentToolResults := make([]int, 0)

	// First pass: identify recent tool results (keep last few)
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "tool" {
			recentToolResults = append([]int{i}, recentToolResults...)
			if len(recentToolResults) >= 3 { // Keep last 3 tool results
				break
			}
		}
	}

	// Create a set of indices to keep
	recentToolSet := make(map[int]bool)
	for _, idx := range recentToolResults {
		recentToolSet[idx] = true
	}

	// Second pass: prune small tool results (except recent ones)
	for i, msg := range messages {
		if msg.Role == "tool" && !recentToolSet[i] {
			if len(msg.Content) < config.MinPrunableToolChars {
				stats.ToolResultsRemoved++
				stats.CharsSaved += len(msg.Content)
				continue // Skip this message
			}
		}
		pruned = append(pruned, msg)
	}

	if stats.ToolResultsRemoved > 0 {
		logger.DebugCF("agent", "Tool results pruned",
			map[string]interface{}{
				"tool_results_removed": stats.ToolResultsRemoved,
				"chars_saved":          stats.CharsSaved,
				"min_threshold":        config.MinPrunableToolChars,
			})
	}

	return pruned, stats
}

// ApplyPruning applies all pruning strategies to the message list.
func ApplyPruning(messages []providers.Message, config PruningConfig) ([]providers.Message, PruningStats) {
	totalStats := PruningStats{}

	if config.Mode == PruningModeOff {
		return messages, totalStats
	}

	// First, prune tool results
	messages, toolStats := pruneToolResults(messages, config)
	totalStats.ToolResultsRemoved += toolStats.ToolResultsRemoved
	totalStats.CharsSaved += toolStats.CharsSaved

	// Then, prune by TTL
	messages, ttlStats := pruneContextByTTL(messages, config)
	totalStats.MessagesRemoved += ttlStats.MessagesRemoved
	totalStats.CharsSaved += ttlStats.CharsSaved

	if totalStats.MessagesRemoved > 0 || totalStats.ToolResultsRemoved > 0 {
		logger.InfoCF("agent", "Pruning complete",
			map[string]interface{}{
				"messages_removed":     totalStats.MessagesRemoved,
				"tool_results_removed": totalStats.ToolResultsRemoved,
				"total_chars_saved":    totalStats.CharsSaved,
			})
	}

	return messages, totalStats
}
