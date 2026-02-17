// PicoClaw - Ultra-lightweight personal AI agent
// Context window validation to prevent misconfiguration

package agent

import (
	"github.com/sipeed/picoclaw/pkg/logger"
)

const (
	// CONTEXT_WINDOW_WARN_BELOW is the threshold below which a warning is logged.
	CONTEXT_WINDOW_WARN_BELOW = 32_000

	// CONTEXT_WINDOW_HARD_MIN is the absolute minimum context window size.
	CONTEXT_WINDOW_HARD_MIN = 16_000
)

// ContextWindowGuardResult contains the result of context window validation.
type ContextWindowGuardResult struct {
	IsBelowRecommended bool
	IsBelowMinimum     bool
	ContextWindow      int
	RecommendedMin     int
	HardMin            int
}

// EvaluateContextWindowGuard checks if the context window is within acceptable bounds.
// Returns a result with warnings and recommendations.
func EvaluateContextWindowGuard(contextWindow int) ContextWindowGuardResult {
	result := ContextWindowGuardResult{
		ContextWindow:  contextWindow,
		RecommendedMin: CONTEXT_WINDOW_WARN_BELOW,
		HardMin:        CONTEXT_WINDOW_HARD_MIN,
	}

	result.IsBelowMinimum = contextWindow < CONTEXT_WINDOW_HARD_MIN
	result.IsBelowRecommended = contextWindow < CONTEXT_WINDOW_WARN_BELOW

	if result.IsBelowMinimum {
		logger.WarnCF("agent", "Context window is below hard minimum - performance will be severely degraded",
			map[string]interface{}{
				"context_window":    contextWindow,
				"hard_minimum":      CONTEXT_WINDOW_HARD_MIN,
				"recommended_min":   CONTEXT_WINDOW_WARN_BELOW,
			})
	} else if result.IsBelowRecommended {
		logger.WarnCF("agent", "Context window is below recommended minimum",
			map[string]interface{}{
				"context_window":    contextWindow,
				"recommended_min":   CONTEXT_WINDOW_WARN_BELOW,
				"note":             "Consider using a model with at least 32K context window for better performance",
			})
	} else {
		logger.DebugCF("agent", "Context window validated",
			map[string]interface{}{
				"context_window": contextWindow,
				"status":         "ok",
			})
	}

	return result
}

// ShouldBlockContextWindow returns true if the context window is too small to function.
func ShouldBlockContextWindow(contextWindow int) bool {
	return contextWindow < CONTEXT_WINDOW_HARD_MIN
}

// ShouldWarnContextWindow returns true if the context window is below recommended minimum.
func ShouldWarnContextWindow(contextWindow int) bool {
	return contextWindow < CONTEXT_WINDOW_WARN_BELOW
}
