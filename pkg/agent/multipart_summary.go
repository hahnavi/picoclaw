// PicoClaw - Ultra-lightweight personal AI agent
// Multi-part summarization with adaptive chunking for large conversations

package agent

import (
	"context"
	"fmt"

	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/providers"
)

const (
	// BASE_CHUNK_RATIO is the base fraction of context window to use for each summary chunk.
	BASE_CHUNK_RATIO = 0.40

	// MIN_CHUNK_RATIO is the minimum fraction of context window for a chunk.
	MIN_CHUNK_RATIO = 0.15

	// SAFETY_MARGIN for token estimation.
	SUMMARY_SAFETY_MARGIN = 1.2

	// RESERVE_TOKENS_FLOOR is the minimum number of tokens to reserve for the summary itself.
	RESERVE_TOKENS_FLOOR = 20_000
)

// SummaryProvider is the interface that providers must implement for multi-part summarization.
type SummaryProvider interface {
	// Chat performs a chat completion request.
	Chat(ctx context.Context, messages []providers.Message, tools []providers.ToolDefinition, model string, options map[string]interface{}) (*providers.LLMResponse, error)
}

// ChunkInfo contains information about a message chunk.
type ChunkInfo struct {
	Messages    []providers.Message
	TargetTokens int
	ActualTokens int
}

// computeAdaptiveChunkRatio calculates the optimal chunk ratio based on message characteristics.
// Returns a value between MIN_CHUNK_RATIO and BASE_CHUNK_RATIO.
func computeAdaptiveChunkRatio(messages []providers.Message, contextWindow int) float64 {
	if len(messages) == 0 {
		return BASE_CHUNK_RATIO
	}

	// Calculate average message size
	totalChars := 0
	for _, m := range messages {
		totalChars += len(m.Content)
	}
	avgMessageSize := totalChars / len(messages)

	// Larger messages → smaller chunks (earlier summarization)
	// Smaller messages → larger chunks (more messages per summary)

	// Heuristic: if average message is > 2K chars, use smaller chunks
	if avgMessageSize > 2000 {
		return MIN_CHUNK_RATIO
	}
	if avgMessageSize > 1000 {
		return MIN_CHUNK_RATIO + (BASE_CHUNK_RATIO-MIN_CHUNK_RATIO)*0.5
	}

	return BASE_CHUNK_RATIO
}

// splitMessagesForSummary splits messages into chunks that fit within the target token count.
// Respects message boundaries and skips oversized messages.
func splitMessagesForSummary(messages []providers.Message, targetTokens int) []ChunkInfo {
	if len(messages) == 0 {
		return []ChunkInfo{}
	}

	// Target tokens to chars (4 chars per token heuristic)
	targetChars := targetTokens * 4

	var chunks []ChunkInfo
	currentChunk := make([]providers.Message, 0)
	currentChars := 0

	for _, msg := range messages {
		msgChars := len(msg.Content)

		// Skip oversized messages (> 50% of context window)
		if msgChars > targetChars/2 {
			logger.DebugCF("agent", "Skipping oversized message in summarization",
				map[string]interface{}{
					"message_chars": msgChars,
					"target_chars":  targetChars,
				})
			continue
		}

		// Check if adding this message would exceed target
		if currentChars+msgChars > targetChars && len(currentChunk) > 0 {
			// Finalize current chunk
			chunks = append(chunks, ChunkInfo{
				Messages:     currentChunk,
				TargetTokens: targetTokens,
				ActualTokens: currentChars / 4, // Convert back to tokens
			})

			// Start new chunk
			currentChunk = make([]providers.Message, 0)
			currentChars = 0
		}

		currentChunk = append(currentChunk, msg)
		currentChars += msgChars
	}

	// Add final chunk if not empty
	if len(currentChunk) > 0 {
		chunks = append(chunks, ChunkInfo{
			Messages:     currentChunk,
			TargetTokens: targetTokens,
			ActualTokens: currentChars / 4,
		})
	}

	return chunks
}

// SummarizeMultipart performs multi-part summarization with adaptive chunking.
// Splits the conversation into chunks, summarizes each, and merges the results.
func SummarizeMultipart(
	ctx context.Context,
	provider SummaryProvider,
	messages []providers.Message,
	existingSummary string,
	model string,
	contextWindow int,
) (string, error) {
	if len(messages) == 0 {
		return existingSummary, nil
	}

	// Calculate adaptive chunk ratio
	chunkRatio := computeAdaptiveChunkRatio(messages, contextWindow)

	// Calculate target tokens per chunk
	targetTokens := int(float64(contextWindow) * chunkRatio)
	targetTokens = int(float64(targetTokens) / SUMMARY_SAFETY_MARGIN)

	// Ensure we have enough space for the summary itself
	if targetTokens < contextWindow-RESERVE_TOKENS_FLOOR {
		targetTokens = contextWindow - RESERVE_TOKENS_FLOOR
	}

	logger.InfoCF("agent", "Multi-part summarization starting",
		map[string]interface{}{
			"total_messages":   len(messages),
			"context_window":   contextWindow,
			"chunk_ratio":      chunkRatio,
			"target_per_chunk": targetTokens,
		})

	// Split into chunks
	chunks := splitMessagesForSummary(messages, targetTokens)

	logger.InfoCF("agent", "Split into chunks",
		map[string]interface{}{
			"num_chunks": len(chunks),
		})

	// Summarize each chunk
	var summaries []string
	for i, chunk := range chunks {
		logger.DebugCF("agent", fmt.Sprintf("Summarizing chunk %d/%d", i+1, len(chunks)),
			map[string]interface{}{
				"messages":      len(chunk.Messages),
				"target_tokens": chunk.TargetTokens,
				"actual_tokens": chunk.ActualTokens,
			})

		summary, err := summarizeChunk(ctx, provider, chunk.Messages, existingSummary, model)
		if err != nil {
			logger.WarnCF("agent", fmt.Sprintf("Failed to summarize chunk %d, skipping", i+1),
				map[string]interface{}{
					"error": err.Error(),
				})
			continue
		}

		summaries = append(summaries, summary)
	}

	// Merge summaries
	if len(summaries) == 0 {
		return existingSummary, fmt.Errorf("all chunks failed to summarize")
	}

	if len(summaries) == 1 {
		return summaries[0], nil
	}

	// Merge multiple summaries
	return mergeSummaries(ctx, provider, summaries, model)
}

// summarizeChunk summarizes a single chunk of messages.
func summarizeChunk(
	ctx context.Context,
	provider SummaryProvider,
	messages []providers.Message,
	existingSummary string,
	model string,
) (string, error) {
	prompt := "Provide a concise summary of this conversation segment, preserving core context and key points.\n"
	if existingSummary != "" {
		prompt += "Existing context: " + existingSummary + "\n"
	}
	prompt += "\nCONVERSATION:\n"
	for _, m := range messages {
		prompt += fmt.Sprintf("%s: %s\n", m.Role, m.Content)
	}

	response, err := provider.Chat(ctx, []providers.Message{{Role: "user", Content: prompt}}, nil, model, map[string]interface{}{
		"max_tokens":  1024,
		"temperature": 0.3,
	})
	if err != nil {
		return "", err
	}

	return response.Content, nil
}

// mergeSummaries merges multiple summaries into a single cohesive summary.
func mergeSummaries(
	ctx context.Context,
	provider SummaryProvider,
	summaries []string,
	model string,
) (string, error) {
	mergePrompt := "Merge these conversation summaries into one cohesive summary that preserves the full conversation flow:\n\n"
	for i, s := range summaries {
		mergePrompt += fmt.Sprintf("PART %d:\n%s\n\n", i+1, s)
	}

	response, err := provider.Chat(ctx, []providers.Message{{Role: "user", Content: mergePrompt}}, nil, model, map[string]interface{}{
		"max_tokens":  2048,
		"temperature": 0.3,
	})
	if err != nil {
		// Fallback: concatenate summaries
		return fmt.Sprintf("%s\n\n[Note: Failed to merge summaries, concatenated instead]\n\n%s",
			summaries[0], summaries[1]), nil
	}

	return response.Content, nil
}

// EstimateChunkTokens estimates the token count for a set of messages.
func EstimateChunkTokens(messages []providers.Message) int {
	totalChars := 0
	for _, m := range messages {
		totalChars += len(m.Content)
	}
	// Apply safety margin
	estimated := (totalChars * 2 / 5)
	return int(float64(estimated) * SUMMARY_SAFETY_MARGIN)
}
