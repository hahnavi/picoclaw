// Package reload provides hot reload functionality for PicoClaw.
package reload

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
)

func TestNewFileWatcher(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Create a minimal config file
	cfg := config.DefaultConfig()
	if err := config.SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	watcherConfig := WatcherConfig{
		ConfigPath:     configPath,
		WorkspacePath:  tmpDir,
		WatchSkills:    true,
		WatchBootstrap: true,
	}

	watcher, err := NewFileWatcher(watcherConfig, 300*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}

	if watcher == nil {
		t.Fatal("Watcher is nil")
	}

	if err := watcher.Close(); err != nil {
		t.Fatalf("Failed to close watcher: %v", err)
	}
}

func TestFileWatcher_Start(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	cfg := config.DefaultConfig()
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

	// Create skills directory
	skillsDir := filepath.Join(tmpDir, "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("Failed to create skills directory: %v", err)
	}

	watcherConfig := WatcherConfig{
		ConfigPath:     configPath,
		WorkspacePath:  tmpDir,
		WatchSkills:    true,
		WatchBootstrap: true,
	}

	watcher, err := NewFileWatcher(watcherConfig, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := watcher.Start(ctx); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}
}

func TestFileWatcher_ConfigEvent(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	cfg := config.DefaultConfig()
	if err := config.SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	watcherConfig := WatcherConfig{
		ConfigPath:     configPath,
		WorkspacePath:  tmpDir,
		WatchSkills:    false,
		WatchBootstrap: false,
	}

	watcher, err := NewFileWatcher(watcherConfig, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := watcher.Start(ctx); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}

	// Modify config file
	go func() {
		time.Sleep(200 * time.Millisecond)
		cfg.Agents.Defaults.Model = "test-model"
		if err := config.SaveConfig(configPath, cfg); err != nil {
			t.Errorf("Failed to modify config: %v", err)
		}
	}()

	// Wait for event
	select {
	case event := <-watcher.Events():
		if event.Type != WatchEventConfig {
			t.Errorf("Expected WatchEventConfig, got %v", event.Type)
		}
		if event.Path != configPath {
			t.Errorf("Expected path %s, got %s", configPath, event.Path)
		}

	case <-time.After(2 * time.Second):
		t.Fatal("No event received within timeout")
	case <-ctx.Done():
		return
	}
}

func TestFileWatcher_BootstrapEvent(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	cfg := config.DefaultConfig()
	if err := config.SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	// Create IDENTITY.md
	identityPath := filepath.Join(tmpDir, "IDENTITY.md")
	if err := os.WriteFile(identityPath, []byte("# Test Identity\n"), 0644); err != nil {
		t.Fatalf("Failed to create IDENTITY.md: %v", err)
	}

	watcherConfig := WatcherConfig{
		ConfigPath:     configPath,
		WorkspacePath:  tmpDir,
		WatchSkills:    false,
		WatchBootstrap: true,
	}

	watcher, err := NewFileWatcher(watcherConfig, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := watcher.Start(ctx); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}

	// Modify IDENTITY.md
	go func() {
		time.Sleep(200 * time.Millisecond)
		if err := os.WriteFile(identityPath, []byte("# Modified Identity\n"), 0644); err != nil {
			t.Errorf("Failed to modify IDENTITY.md: %v", err)
		}
	}()

	// Wait for event
	select {
	case event := <-watcher.Events():
		if event.Type != WatchEventBootstrap {
			t.Errorf("Expected WatchEventBootstrap, got %v", event.Type)
		}
		if event.Path != identityPath {
			t.Errorf("Expected path %s, got %s", identityPath, event.Path)
		}

	case <-time.After(2 * time.Second):
		t.Fatal("No event received within timeout")
	case <-ctx.Done():
		return
	}
}

func TestFileWatcher_SkillEvent(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	cfg := config.DefaultConfig()
	if err := config.SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	// Create skills directory with a skill
	skillsDir := filepath.Join(tmpDir, "skills")
	testSkill := filepath.Join(skillsDir, "test-skill")
	if err := os.MkdirAll(testSkill, 0755); err != nil {
		t.Fatalf("Failed to create skill directory: %v", err)
	}

	skillFile := filepath.Join(testSkill, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte("# Test Skill\n"), 0644); err != nil {
		t.Fatalf("Failed to create SKILL.md: %v", err)
	}

	watcherConfig := WatcherConfig{
		ConfigPath:     configPath,
		WorkspacePath:  tmpDir,
		WatchSkills:    true,
		WatchBootstrap: false,
	}

	watcher, err := NewFileWatcher(watcherConfig, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := watcher.Start(ctx); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}

	// Modify SKILL.md
	go func() {
		time.Sleep(200 * time.Millisecond)
		if err := os.WriteFile(skillFile, []byte("# Modified Skill\n"), 0644); err != nil {
			t.Errorf("Failed to modify SKILL.md: %v", err)
		}
	}()

	// Wait for event
	select {
	case event := <-watcher.Events():
		if event.Type != WatchEventSkill {
			t.Errorf("Expected WatchEventSkill, got %v", event.Type)
		}

	case <-time.After(2 * time.Second):
		t.Fatal("No event received within timeout")
	case <-ctx.Done():
		return
	}
}

func TestFileWatcher_Debouncing(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	cfg := config.DefaultConfig()
	if err := config.SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	debounce := 300 * time.Millisecond
	watcherConfig := WatcherConfig{
		ConfigPath:     configPath,
		WorkspacePath:  tmpDir,
		WatchSkills:    false,
		WatchBootstrap: false,
	}

	watcher, err := NewFileWatcher(watcherConfig, debounce)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := watcher.Start(ctx); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}

	// Multiple rapid writes
	go func() {
		time.Sleep(100 * time.Millisecond)
		for i := 0; i < 5; i++ {
			cfg.Agents.Defaults.Model = "test-model"
			if err := config.SaveConfig(configPath, cfg); err != nil {
				t.Errorf("Failed to modify config: %v", err)
			}
			time.Sleep(50 * time.Millisecond)
		}
	}()

	// Should only receive one event after debounce period
	eventCount := 0
	timeout := time.After(2 * time.Second)
	for {
		select {
		case <-watcher.Events():
			eventCount++
		case <-timeout:
			if eventCount != 1 {
				t.Errorf("Expected 1 event after debouncing, got %d", eventCount)
			}
			return
		case <-ctx.Done():
			return
		}
	}
}

func TestWatchEventType_String(t *testing.T) {
	tests := []struct {
		name     string
		event    WatchEventType
		expected string
	}{
		{"Config", WatchEventConfig, "config"},
		{"Skill", WatchEventSkill, "skill"},
		{"Bootstrap", WatchEventBootstrap, "bootstrap"},
		{"Unknown", WatchEventType(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.event.String(); got != tt.expected {
				t.Errorf("WatchEventType.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}
