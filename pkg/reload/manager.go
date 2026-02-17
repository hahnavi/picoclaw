// Package reload provides hot reload functionality for PicoClaw.
package reload

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/sipeed/picoclaw/pkg/agent"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/skills"
)

// ReloadableComponent is an interface for components that can be reloaded.
type ReloadableComponent interface {
	Reload() error
	Name() string
}

// ReloadResult represents the result of a reload operation.
type ReloadResult struct {
	Success   bool
	Component string
	Message   string
	Error     error
}

// ReloadManager manages hot reload of configuration, skills, and bootstrap files.
type ReloadManager struct {
	agentLoop      *agent.AgentLoop
	config         *config.Config
	configPath     string
	skillsLoader   *skills.SkillsLoader
	components     map[string]ReloadableComponent
	mu             sync.RWMutex
	msgBus         interface{} // *bus.MessageBus - use interface{} to avoid import cycle
	reloading      sync.Map    // Tracks which components are currently reloading
}

// NewReloadManager creates a new reload manager.
func NewReloadManager(agentLoop *agent.AgentLoop, cfg *config.Config, configPath string) *ReloadManager {
	workspace := cfg.WorkspacePath()
	stateDir := config.GetStateDir()
	globalSkillsDir := config.GetGlobalSkillsPath()
	builtinSkillsDir := config.GetBuiltinSkillsPath(stateDir)

	skillsLoader := skills.NewSkillsLoader(workspace, globalSkillsDir, builtinSkillsDir)

	return &ReloadManager{
		agentLoop:    agentLoop,
		config:       cfg,
		configPath:   configPath,
		skillsLoader: skillsLoader,
		components:   make(map[string]ReloadableComponent),
	}
}

// SetMessageBus sets the message bus for the reload manager.
// This is needed for tool recreation during reload.
func (rm *ReloadManager) SetMessageBus(msgBus interface{}) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.msgBus = msgBus
}

// RegisterComponent registers a reloadable component.
func (rm *ReloadManager) RegisterComponent(name string, component ReloadableComponent) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.components[name] = component
	logger.InfoC("reload", fmt.Sprintf("Registered reloadable component: %s", name))
}

// HandleEvent handles a watch event and performs the appropriate reload.
func (rm *ReloadManager) HandleEvent(ctx context.Context, event WatchEvent) ReloadResult {
	logger.InfoCF("reload", fmt.Sprintf("Handling %s event: %s", event.Type, event.Path),
		map[string]interface{}{
			"timestamp": event.Timestamp.Format(time.RFC3339),
		})

	switch event.Type {
	case WatchEventConfig:
		return rm.reloadConfig()
	case WatchEventSkill:
		return rm.reloadSkills()
	case WatchEventBootstrap:
		return rm.reloadBootstrap()
	default:
		return ReloadResult{
			Success: false,
			Message: fmt.Sprintf("Unknown event type: %d", event.Type),
			Error:   fmt.Errorf("unknown event type: %d", event.Type),
		}
	}
}

// reloadConfig reloads the configuration file.
func (rm *ReloadManager) reloadConfig() ReloadResult {
	// Check if already reloading
	if _, loading := rm.reloading.LoadOrStore("config", true); loading {
		return ReloadResult{
			Success: false,
			Message: "Config reload already in progress",
			Error:   fmt.Errorf("reload already in progress"),
		}
	}
	defer rm.reloading.Delete("config")

	logger.InfoC("reload", "Reloading configuration")

	// Load new config
	newConfig, err := config.LoadConfig(rm.configPath)
	if err != nil {
		logger.ErrorC("reload", fmt.Sprintf("Failed to load config: %v", err))
		return ReloadResult{
			Success: false,
			Component: "config",
			Message: "Failed to load configuration file",
			Error:   err,
		}
	}

	// Compare with current config to see what changed
	changedFields := rm.config.CompareHotReloadable(newConfig)

	// Update agent loop settings
	if len(changedFields) == 0 {
		logger.InfoC("reload", "No hot-reloadable fields changed")
		return ReloadResult{
			Success: true,
			Component: "config",
			Message: "No changes detected",
		}
	}

	logger.InfoC("reload", fmt.Sprintf("Config fields changed: %v", changedFields))

	// Apply changes to agent loop
	rm.applyConfigChanges(newConfig, changedFields)

	// Update stored config
	rm.mu.Lock()
	rm.config = newConfig
	rm.mu.Unlock()

	// Reload tools if tool config changed
	for _, field := range changedFields {
		if field == "tools.web" {
			if err := rm.reloadTools(); err != nil {
				logger.WarnC("reload", fmt.Sprintf("Failed to reload tools: %v", err))
			}
			break
		}
	}

	return ReloadResult{
		Success: true,
		Component: "config",
		Message: fmt.Sprintf("Reloaded config, changed fields: %v", changedFields),
	}
}

// applyConfigChanges applies configuration changes to the agent loop.
func (rm *ReloadManager) applyConfigChanges(newConfig *config.Config, changedFields []string) {
	for _, field := range changedFields {
		switch field {
		case "model":
			rm.agentLoop.UpdateModel(newConfig.Agents.Defaults.Model)
			logger.InfoC("reload", fmt.Sprintf("Updated model to: %s", newConfig.Agents.Defaults.Model))
		case "max_tokens":
			rm.agentLoop.UpdateContextWindow(newConfig.Agents.Defaults.MaxTokens)
			logger.InfoC("reload", fmt.Sprintf("Updated max_tokens to: %d", newConfig.Agents.Defaults.MaxTokens))
		case "temperature":
			// Temperature is used per-call, not stored in agent loop
			logger.InfoC("reload", fmt.Sprintf("Temperature changed to: %f", newConfig.Agents.Defaults.Temperature))
		case "bootstrap_max_chars", "bootstrap_total_max_chars":
			rm.agentLoop.UpdateBootstrapConfig(agent.BootstrapConfig{
				MaxChars:      newConfig.Agents.Defaults.BootstrapMaxChars,
				TotalMaxChars: newConfig.Agents.Defaults.BootstrapTotalMaxChars,
			})
			logger.InfoC("reload", "Updated bootstrap configuration")
		case "context_pruning":
			rm.agentLoop.UpdatePruningConfig(agent.PruningConfig{
				Mode:                 agent.PruningMode(newConfig.Agents.Defaults.ContextPruning.Mode),
				TTL:                  time.Duration(newConfig.Agents.Defaults.ContextPruning.TTLMinutes) * time.Minute,
				KeepLastAssistants:   newConfig.Agents.Defaults.ContextPruning.KeepLastAssistants,
				SoftTrimRatio:        newConfig.Agents.Defaults.ContextPruning.SoftTrimRatio,
				HardClearRatio:       newConfig.Agents.Defaults.ContextPruning.HardClearRatio,
				MinPrunableToolChars: newConfig.Agents.Defaults.ContextPruning.MinPrunableToolChars,
			})
			logger.InfoC("reload", "Updated context pruning configuration")
		}
	}
}

// reloadTools recreates the tool registry with new configuration.
func (rm *ReloadManager) reloadTools() error {
	logger.InfoC("reload", "Reloading tools with new configuration")

	rm.mu.RLock()
	cfg := rm.config
	msgBus := rm.msgBus
	rm.mu.RUnlock()

	if msgBus == nil {
		return fmt.Errorf("message bus not set")
	}

	// The ReloadTools method on AgentLoop will recreate tools with the new config
	return rm.agentLoop.ReloadTools(cfg)
}

// reloadSkills reloads the skills loader and updates the context builder.
func (rm *ReloadManager) reloadSkills() ReloadResult {
	// Check if already reloading
	if _, loading := rm.reloading.LoadOrStore("skills", true); loading {
		return ReloadResult{
			Success: false,
			Message: "Skills reload already in progress",
			Error:   fmt.Errorf("reload already in progress"),
		}
	}
	defer rm.reloading.Delete("skills")

	logger.InfoC("reload", "Reloading skills")

	// Reload skills summary in context builder
	if err := rm.agentLoop.ReloadSkillsSummary(); err != nil {
		logger.ErrorC("reload", fmt.Sprintf("Failed to reload skills: %v", err))
		return ReloadResult{
			Success: false,
			Component: "skills",
			Message: "Failed to reload skills summary",
			Error:   err,
		}
	}

	// Get updated skills info
	skillsInfo := rm.agentLoop.GetSkillsInfo()
	total, _ := skillsInfo["total"].(int)
	available, _ := skillsInfo["available"].(int)

	return ReloadResult{
		Success: true,
		Component: "skills",
		Message: fmt.Sprintf("Skills reloaded (%d available, %d total)", available, total),
	}
}

// reloadBootstrap invalidates the bootstrap file cache.
func (rm *ReloadManager) reloadBootstrap() ReloadResult {
	// Check if already reloading
	if _, loading := rm.reloading.LoadOrStore("bootstrap", true); loading {
		return ReloadResult{
			Success: false,
			Message: "Bootstrap reload already in progress",
			Error:   fmt.Errorf("reload already in progress"),
		}
	}
	defer rm.reloading.Delete("bootstrap")

	logger.InfoC("reload", "Invalidating bootstrap cache")

	// Invalidate bootstrap cache in context builder
	rm.agentLoop.InvalidateBootstrapCache()

	// Get the filename for logging
	filename := filepath.Base(rm.configPath)

	return ReloadResult{
		Success: true,
		Component: "bootstrap",
		Message: fmt.Sprintf("Bootstrap cache invalidated (changed: %s)", filename),
	}
}
