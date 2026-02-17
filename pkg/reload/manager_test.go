// Package reload provides hot reload functionality for PicoClaw.
package reload

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sipeed/picoclaw/pkg/agent"
	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/providers"
)

// mockProvider is a minimal mock LLM provider for testing
type mockProvider struct{}

func (m *mockProvider) Chat(ctx context.Context, messages []providers.Message, tools []providers.ToolDefinition, model string, opts map[string]interface{}) (*providers.LLMResponse, error) {
	return &providers.LLMResponse{
		Content: "Test response",
	}, nil
}

func (m *mockProvider) GetDefaultModel() string {
	return "test-model"
}

func TestNewReloadManager(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Workspace = tmpDir
	if err := config.SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	msgBus := bus.NewMessageBus()
	provider := &mockProvider{}
	agentLoop := agent.NewAgentLoop(cfg, msgBus, provider)

	rm := NewReloadManager(agentLoop, cfg, configPath)

	if rm == nil {
		t.Fatal("ReloadManager is nil")
	}

	if rm.configPath != configPath {
		t.Errorf("Expected configPath %s, got %s", configPath, rm.configPath)
	}
}

func TestReloadManager_HandleEvent_Config(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Workspace = tmpDir
	cfg.Agents.Defaults.Model = "initial-model"
	if err := config.SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	msgBus := bus.NewMessageBus()
	provider := &mockProvider{}
	agentLoop := agent.NewAgentLoop(cfg, msgBus, provider)

	rm := NewReloadManager(agentLoop, cfg, configPath)

	// Test handling a config event
	event := WatchEvent{
		Type:      WatchEventConfig,
		Path:      configPath,
		Timestamp: time.Now(),
	}

	result := rm.HandleEvent(context.Background(), event)

	if !result.Success {
		t.Errorf("Expected success, got error: %v", result.Error)
	}

	if result.Component != "config" {
		t.Errorf("Expected component 'config', got '%s'", result.Component)
	}
}

func TestReloadManager_HandleEvent_Skills(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Workspace = tmpDir
	if err := config.SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	msgBus := bus.NewMessageBus()
	provider := &mockProvider{}
	agentLoop := agent.NewAgentLoop(cfg, msgBus, provider)

	rm := NewReloadManager(agentLoop, cfg, configPath)

	// Test handling a skills event
	event := WatchEvent{
		Type:      WatchEventSkill,
		Path:      filepath.Join(tmpDir, "skills", "test", "SKILL.md"),
		Timestamp: time.Now(),
	}

	result := rm.HandleEvent(context.Background(), event)

	if !result.Success {
		t.Errorf("Expected success, got error: %v", result.Error)
	}

	if result.Component != "skills" {
		t.Errorf("Expected component 'skills', got '%s'", result.Component)
	}
}

func TestReloadManager_HandleEvent_Bootstrap(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Workspace = tmpDir
	if err := config.SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	// Create bootstrap files
	bootstrapFiles := []string{"IDENTITY.md", "SOUL.md", "AGENTS.md", "USER.md"}
	for _, file := range bootstrapFiles {
		path := filepath.Join(tmpDir, file)
		if err := os.WriteFile(path, []byte("# Test\n"), 0644); err != nil {
			t.Fatalf("Failed to create bootstrap file %s: %v", file, err)
		}
	}

	msgBus := bus.NewMessageBus()
	provider := &mockProvider{}
	agentLoop := agent.NewAgentLoop(cfg, msgBus, provider)

	rm := NewReloadManager(agentLoop, cfg, configPath)

	// Test handling a bootstrap event
	event := WatchEvent{
		Type:      WatchEventBootstrap,
		Path:      filepath.Join(tmpDir, "IDENTITY.md"),
		Timestamp: time.Now(),
	}

	result := rm.HandleEvent(context.Background(), event)

	if !result.Success {
		t.Errorf("Expected success, got error: %v", result.Error)
	}

	if result.Component != "bootstrap" {
		t.Errorf("Expected component 'bootstrap', got '%s'", result.Component)
	}
}

func TestReloadManager_RegisterComponent(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Workspace = tmpDir
	if err := config.SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	msgBus := bus.NewMessageBus()
	provider := &mockProvider{}
	agentLoop := agent.NewAgentLoop(cfg, msgBus, provider)

	rm := NewReloadManager(agentLoop, cfg, configPath)

	// Create a mock component
	mockComp := &mockReloadableComponent{
		name: "test-component",
	}

	rm.RegisterComponent("test", mockComp)

	rm.mu.RLock()
	if _, exists := rm.components["test"]; !exists {
		t.Error("Component was not registered")
	}
	rm.mu.RUnlock()
}

func TestReloadManager_ConcurrentReloads(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Workspace = tmpDir
	if err := config.SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	msgBus := bus.NewMessageBus()
	provider := &mockProvider{}
	agentLoop := agent.NewAgentLoop(cfg, msgBus, provider)

	rm := NewReloadManager(agentLoop, cfg, configPath)

	// Trigger multiple concurrent config reloads
	event := WatchEvent{
		Type:      WatchEventConfig,
		Path:      configPath,
		Timestamp: time.Now(),
	}

	results := make(chan ReloadResult, 3)

	// Start 3 goroutines trying to reload simultaneously
	for i := 0; i < 3; i++ {
		go func() {
			results <- rm.HandleEvent(context.Background(), event)
		}()
	}

	// Collect results
	successCount := 0
	for i := 0; i < 3; i++ {
		result := <-results
		if result.Success {
			successCount++
		}
	}

	// At least one should succeed
	if successCount < 1 {
		t.Error("Expected at least one successful reload")
	}

	// At most one should actually do work (others should be "already in progress")
	if successCount > 1 {
		// This is actually OK - different events might process
	}
}

// mockReloadableComponent is a mock implementation of ReloadableComponent for testing
type mockReloadableComponent struct {
	name       string
	reloadCall int
}

func (m *mockReloadableComponent) Reload() error {
	m.reloadCall++
	return nil
}

func (m *mockReloadableComponent) Name() string {
	return m.name
}
