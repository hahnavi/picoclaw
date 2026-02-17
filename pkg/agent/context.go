package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/providers"
	"github.com/sipeed/picoclaw/pkg/skills"
	"github.com/sipeed/picoclaw/pkg/tools"
)

type ContextBuilder struct {
	workspace       string
	skillsLoader    *skills.SkillsLoader
	memory          *MemoryStore
	tools           *tools.ToolRegistry // Direct reference to tool registry
	currentUserID   string             // Current user ID for memory operations
	bootstrapConfig BootstrapConfig    // Bootstrap truncation config
}

func getGlobalConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".picoclaw")
}

func NewContextBuilder(workspace string) *ContextBuilder {
	// builtin skills: skills directory in current project
	// Use the skills/ directory under the current working directory
	wd, _ := os.Getwd()
	builtinSkillsDir := filepath.Join(wd, "skills")
	globalSkillsDir := filepath.Join(getGlobalConfigDir(), "skills")

	return &ContextBuilder{
		workspace:    workspace,
		skillsLoader: skills.NewSkillsLoader(workspace, globalSkillsDir, builtinSkillsDir),
		memory:       NewMemoryStore(workspace),
	}
}

// SetToolsRegistry sets the tools registry for dynamic tool summary generation.
func (cb *ContextBuilder) SetToolsRegistry(registry *tools.ToolRegistry) {
	cb.tools = registry
}

// SetBootstrapConfig sets the bootstrap truncation configuration.
func (cb *ContextBuilder) SetBootstrapConfig(config BootstrapConfig) {
	cb.bootstrapConfig = config
}

// SetUserContext sets the current user ID for memory operations.
// All subsequent memory operations will use this user's memory store.
func (cb *ContextBuilder) SetUserContext(userID string) {
	cb.currentUserID = userID
}

// ClearUserContext clears the current user ID, reverting to shared memory.
func (cb *ContextBuilder) ClearUserContext() {
	cb.currentUserID = ""
}

// getUserMemoryContext returns memory context for the current user.
// If no user context is set, returns the shared memory context.
func (cb *ContextBuilder) getUserMemoryContext() string {
	return cb.memory.GetUserMemoryContext(cb.currentUserID)
}

func (cb *ContextBuilder) getBotName() string {
	identityPath := filepath.Join(cb.workspace, "IDENTITY.md")
	data, err := os.ReadFile(identityPath)
	if err != nil {
		return "" // No fallback - triggers onboarding
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	// Find "## Name" section
	for i, line := range lines {
		if strings.Contains(line, "## Name") && i+1 < len(lines) {
			nameLine := strings.TrimSpace(lines[i+1])
			// Parse "BotName 🎭" format - extract first word
			parts := strings.Fields(nameLine)
			if len(parts) > 0 {
				return parts[0]
			}
		}
	}

	return "" // No fallback - triggers onboarding
}

// getIdentityField extracts a specific section value from IDENTITY.md
func (cb *ContextBuilder) getIdentityField(sectionName string) string {
	identityPath := filepath.Join(cb.workspace, "IDENTITY.md")
	data, err := os.ReadFile(identityPath)
	if err != nil {
		return ""
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	// Find the section (e.g., "## Creature", "## Vibe")
	for i, line := range lines {
		if strings.Contains(line, "## "+sectionName) && i+1 < len(lines) {
			value := strings.TrimSpace(lines[i+1])
			// Skip empty values or placeholder lines
			if value != "" && !strings.HasPrefix(value, "_(") && !strings.HasPrefix(value, "_(workspace") {
				return value
			}
		}
	}

	return ""
}

func (cb *ContextBuilder) getBotEmoji() string {
	identityPath := filepath.Join(cb.workspace, "IDENTITY.md")
	data, err := os.ReadFile(identityPath)
	if err != nil {
		return "🤖" // Default emoji for onboarding
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	// Find "## Name" section
	for i, line := range lines {
		if strings.Contains(line, "## Name") && i+1 < len(lines) {
			nameLine := strings.TrimSpace(lines[i+1])
			// Extract emoji (everything after the name)
			parts := strings.Fields(nameLine)
			if len(parts) > 1 {
				return strings.TrimPrefix(nameLine, parts[0]+" ")
			}
		}
	}

	return "🤖" // Default emoji for onboarding
}

func (cb *ContextBuilder) getIdentity() string {
	now := time.Now().Format("2006-01-02 15:04 (Monday)")
	workspacePath, _ := filepath.Abs(filepath.Join(cb.workspace))
	runtime := fmt.Sprintf("%s %s, Go %s", runtime.GOOS, runtime.GOARCH, runtime.Version())

	// Build tools section dynamically
	toolsSection := cb.buildToolsSection()

	// Read bot name and emoji dynamically
	botName := cb.getBotName()
	botEmoji := cb.getBotEmoji()

	// Read personality fields
	creature := cb.getIdentityField("Creature")
	vibe := cb.getIdentityField("Vibe")

	// Build personality description
	var personalityParts []string
	if creature != "" {
		personalityParts = append(personalityParts, fmt.Sprintf("**Creature:** %s", creature))
	}
	if vibe != "" {
		personalityParts = append(personalityParts, fmt.Sprintf("**Vibe:** %s", vibe))
	}
	personalitySection := ""
	if len(personalityParts) > 0 {
		personalitySection = "\n\n## Personality\n\n" + strings.Join(personalityParts, "\n")
	}

	// Determine memory path based on user context
	var memoryPathDisplay string
	if cb.currentUserID != "" {
		memoryPathDisplay = fmt.Sprintf("%s/memory/users/%s/MEMORY.md", workspacePath, cb.currentUserID)
	} else {
		memoryPathDisplay = fmt.Sprintf("%s/memory/MEMORY.md", workspacePath)
	}

	return fmt.Sprintf(`# %s %s

You are %s, a helpful AI assistant.%s

## Current Time
%s

## Runtime
%s

## Workspace
Your workspace is at: %s
- Memory: %s
- Daily Notes: %s/memory/YYYYMM/YYYYMMDD.md
- Skills: %s/skills/{skill-name}/SKILL.md

%s

## Important Rules

1. **ALWAYS use tools** - When you need to perform an action (schedule reminders, send messages, execute commands, etc.), you MUST call the appropriate tool. Do NOT just say you'll do it or pretend to do it.

2. **Be helpful and accurate** - When using tools, briefly explain what you're doing.

3. **Memory** - When remembering something, write to %s`,
		botName, botEmoji, strings.ToLower(botName), personalitySection,
		now, runtime, workspacePath, memoryPathDisplay, workspacePath, workspacePath, toolsSection, memoryPathDisplay)
}

func (cb *ContextBuilder) buildToolsSection() string {
	if cb.tools == nil {
		return ""
	}

	summaries := cb.tools.GetSummaries()
	if len(summaries) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Available Tools\n\n")
	sb.WriteString("**CRITICAL**: You MUST use tools to perform actions. Do NOT pretend to execute commands or schedule tasks.\n\n")
	sb.WriteString("You have access to the following tools:\n\n")
	for _, s := range summaries {
		sb.WriteString(s)
		sb.WriteString("\n")
	}

	return sb.String()
}

func (cb *ContextBuilder) BuildSystemPrompt() string {
	parts := []string{}

	// Core identity section
	parts = append(parts, cb.getIdentity())

	// Bootstrap files
	bootstrapContent := cb.LoadBootstrapFiles()
	if bootstrapContent != "" {
		parts = append(parts, bootstrapContent)
	}

	// Skills - show summary, AI can read full content with read_file tool
	skillsSummary := cb.skillsLoader.BuildSkillsSummary()
	if skillsSummary != "" {
		parts = append(parts, fmt.Sprintf(`# Skills

The following skills extend your capabilities. To use a skill, read its SKILL.md file using the read_file tool.

%s`, skillsSummary))
	}

	// Memory is NOT auto-injected - use the memory_get tool to load memory on demand
	// This reduces token usage when memory is not needed for the current request

	// Join with "---" separator
	return strings.Join(parts, "\n\n---\n\n")
}

func (cb *ContextBuilder) LoadBootstrapFiles() string {
	// Use default config if not set
	config := cb.bootstrapConfig
	if config.MaxChars == 0 {
		config = DefaultBootstrapConfig()
	}
	return LoadBootstrapFiles(cb.workspace, config)
}

func (cb *ContextBuilder) BuildMessages(history []providers.Message, summary string, currentMessage string, media []string, channel, chatID string) []providers.Message {
	messages := []providers.Message{}

	systemPrompt := cb.BuildSystemPrompt()

	// Add Current Session info if provided
	if channel != "" && chatID != "" {
		systemPrompt += fmt.Sprintf("\n\n## Current Session\nChannel: %s\nChat ID: %s", channel, chatID)
	}

	// Log system prompt summary for debugging (debug mode only)
	logger.DebugCF("agent", "System prompt built",
		map[string]interface{}{
			"total_chars":   len(systemPrompt),
			"total_lines":   strings.Count(systemPrompt, "\n") + 1,
			"section_count": strings.Count(systemPrompt, "\n\n---\n\n") + 1,
		})

	// Log preview of system prompt (avoid logging huge content)
	preview := systemPrompt
	if len(preview) > 500 {
		preview = preview[:500] + "... (truncated)"
	}
	logger.DebugCF("agent", "System prompt preview",
		map[string]interface{}{
			"preview": preview,
		})

	if summary != "" {
		systemPrompt += "\n\n## Summary of Previous Conversation\n\n" + summary
	}

	//This fix prevents the session memory from LLM failure due to elimination of toolu_IDs required from LLM
	// --- INICIO DEL FIX ---
	//Diegox-17
	for len(history) > 0 && (history[0].Role == "tool") {
		logger.DebugCF("agent", "Removing orphaned tool message from history to prevent LLM error",
			map[string]interface{}{"role": history[0].Role})
		history = history[1:]
	}
	//Diegox-17
	// --- FIN DEL FIX ---

	messages = append(messages, providers.Message{
		Role:    "system",
		Content: systemPrompt,
	})

	messages = append(messages, history...)

	messages = append(messages, providers.Message{
		Role:    "user",
		Content: currentMessage,
	})

	return messages
}

func (cb *ContextBuilder) AddToolResult(messages []providers.Message, toolCallID, toolName, result string) []providers.Message {
	messages = append(messages, providers.Message{
		Role:       "tool",
		Content:    result,
		ToolCallID: toolCallID,
	})
	return messages
}

func (cb *ContextBuilder) AddAssistantMessage(messages []providers.Message, content string, toolCalls []map[string]interface{}) []providers.Message {
	msg := providers.Message{
		Role:    "assistant",
		Content: content,
	}
	// Always add assistant message, whether or not it has tool calls
	messages = append(messages, msg)
	return messages
}

func (cb *ContextBuilder) loadSkills() string {
	allSkills := cb.skillsLoader.ListSkills()
	if len(allSkills) == 0 {
		return ""
	}

	var skillNames []string
	for _, s := range allSkills {
		skillNames = append(skillNames, s.Name)
	}

	content := cb.skillsLoader.LoadSkillsForContext(skillNames)
	if content == "" {
		return ""
	}

	return "# Skill Definitions\n\n" + content
}

// GetSkillsInfo returns information about loaded skills.
func (cb *ContextBuilder) GetSkillsInfo() map[string]interface{} {
	allSkills := cb.skillsLoader.ListSkills()
	skillNames := make([]string, 0, len(allSkills))
	for _, s := range allSkills {
		skillNames = append(skillNames, s.Name)
	}
	return map[string]interface{}{
		"total":     len(allSkills),
		"available": len(allSkills),
		"names":     skillNames,
	}
}
