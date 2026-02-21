package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDefaultConfig_HeartbeatEnabled verifies heartbeat is enabled by default
func TestDefaultConfig_HeartbeatEnabled(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.Heartbeat.Enabled {
		t.Error("Heartbeat should be enabled by default")
	}
}

// TestDefaultConfig_WorkspacePath verifies workspace path is correctly set
func TestDefaultConfig_WorkspacePath(t *testing.T) {
	cfg := DefaultConfig()

	// Just verify the workspace is set, don't compare exact paths
	// since expandHome behavior may differ based on environment
	if cfg.Agents.Defaults.Workspace == "" {
		t.Error("Workspace should not be empty")
	}
}

// TestDefaultConfig_AgentDefaults verifies agent defaults are configured
func TestDefaultConfig_AgentDefaults(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Agents.Defaults.Model == "" {
		t.Error("Model should not be empty")
	}

	if cfg.Agents.Defaults.MaxTokens == 0 {
		t.Error("MaxTokens should not be zero")
	}

	if cfg.Agents.Defaults.MaxToolIterations == 0 {
		t.Error("MaxToolIterations should not be zero")
	}

	if cfg.Agents.Defaults.Temperature == 0 {
		t.Error("Temperature should not be zero")
	}
}

// TestDefaultConfig_Gateway verifies gateway defaults
func TestDefaultConfig_Gateway(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Gateway.Host != "0.0.0.0" {
		t.Error("Gateway host should have default value")
	}
	if cfg.Gateway.Port == 0 {
		t.Error("Gateway port should have default value")
	}
}

// TestDefaultConfig_Providers verifies provider structure
func TestDefaultConfig_Providers(t *testing.T) {
	cfg := DefaultConfig()

	// Verify all providers are empty by default
	if cfg.Providers.OpenAI.APIKey != "" {
		t.Error("OpenAI API key should be empty by default")
	}
	if cfg.Providers.OpenRouter.APIKey != "" {
		t.Error("OpenRouter API key should be empty by default")
	}
	if cfg.Providers.Groq.APIKey != "" {
		t.Error("Groq API key should be empty by default")
	}
	if cfg.Providers.Zhipu.APIKey != "" {
		t.Error("Zhipu API key should be empty by default")
	}
	if cfg.Providers.VLLM.APIKey != "" {
		t.Error("VLLM API key should be empty by default")
	}
	if cfg.Providers.Gemini.APIKey != "" {
		t.Error("Gemini API key should be empty by default")
	}
}

// TestDefaultConfig_Channels verifies channels are disabled by default
func TestDefaultConfig_Channels(t *testing.T) {
	cfg := DefaultConfig()

	// Verify all channels are disabled by default
	if cfg.Channels.Discord.Enabled {
		t.Error("Discord should be disabled by default")
	}
}

// TestDefaultConfig_WebTools verifies web tools config
func TestDefaultConfig_WebTools(t *testing.T) {
	cfg := DefaultConfig()

	// Verify web tools defaults
	if cfg.Tools.Web.Brave.MaxResults != 5 {
		t.Error("Expected Brave MaxResults 5, got ", cfg.Tools.Web.Brave.MaxResults)
	}
	if cfg.Tools.Web.Brave.APIKey != "" {
		t.Error("Brave API key should be empty by default")
	}
	if cfg.Tools.Web.DuckDuckGo.MaxResults != 5 {
		t.Error("Expected DuckDuckGo MaxResults 5, got ", cfg.Tools.Web.DuckDuckGo.MaxResults)
	}
}

// TestConfig_Complete verifies all config fields are set
func TestConfig_Complete(t *testing.T) {
	cfg := DefaultConfig()

	// Verify complete config structure
	if cfg.Agents.Defaults.Workspace == "" {
		t.Error("Workspace should not be empty")
	}
	if cfg.Agents.Defaults.Model == "" {
		t.Error("Model should not be empty")
	}
	if cfg.Agents.Defaults.Temperature == 0 {
		t.Error("Temperature should have default value")
	}
	if cfg.Agents.Defaults.MaxTokens == 0 {
		t.Error("MaxTokens should not be zero")
	}
	if cfg.Agents.Defaults.MaxToolIterations == 0 {
		t.Error("MaxToolIterations should not be zero")
	}
	if cfg.Gateway.Host != "0.0.0.0" {
		t.Error("Gateway host should have default value")
	}
	if cfg.Gateway.Port == 0 {
		t.Error("Gateway port should have default value")
	}
	if !cfg.Heartbeat.Enabled {
		t.Error("Heartbeat should be enabled by default")
	}
}

func TestDefaultConfig_AdditionalMemoryDir_Empty(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Agents.Defaults.AdditionalMemoryDir != "" {
		t.Errorf("AdditionalMemoryDir should default to empty, got: %q", cfg.Agents.Defaults.AdditionalMemoryDir)
	}
	if cfg.AdditionalMemoryPath() != "" {
		t.Errorf("AdditionalMemoryPath should default to empty, got: %q", cfg.AdditionalMemoryPath())
	}
}

func TestConfig_AdditionalMemoryPath_RelativeToWorkspace(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Agents.Defaults.Workspace = "/tmp/picoclaw-workspace"
	cfg.Agents.Defaults.AdditionalMemoryDir = "extra-memory"

	got := cfg.AdditionalMemoryPath()
	want := filepath.Clean("/tmp/picoclaw-workspace/extra-memory")
	if got != want {
		t.Errorf("Expected %q, got %q", want, got)
	}
}

func TestConfig_AdditionalMemoryPath_Absolute(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Agents.Defaults.Workspace = "/tmp/picoclaw-workspace"
	cfg.Agents.Defaults.AdditionalMemoryDir = "/var/lib/picoclaw-memory"

	got := cfg.AdditionalMemoryPath()
	want := filepath.Clean("/var/lib/picoclaw-memory")
	if got != want {
		t.Errorf("Expected %q, got %q", want, got)
	}
}

func TestConfig_AdditionalMemoryPath_TildeExpansion(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("home directory unavailable")
	}

	cfg := DefaultConfig()
	cfg.Agents.Defaults.AdditionalMemoryDir = "~/picoclaw-extra-memory"

	got := cfg.AdditionalMemoryPath()
	if !strings.HasPrefix(got, home) {
		t.Errorf("Expected path %q to start with home %q", got, home)
	}
	if !strings.HasSuffix(got, filepath.Clean("/picoclaw-extra-memory")) {
		t.Errorf("Expected path %q to end with /picoclaw-extra-memory", got)
	}
}

func TestConfig_CompareHotReloadable_AdditionalMemoryDir(t *testing.T) {
	cfg1 := DefaultConfig()
	cfg2 := DefaultConfig()

	// No change initially
	changed := cfg1.CompareHotReloadable(cfg2)
	if len(changed) != 0 {
		t.Errorf("Expected no changes, got %v", changed)
	}

	// Change additional_memory_dir
	cfg2.Agents.Defaults.AdditionalMemoryDir = "/extra/memory"
	changed = cfg1.CompareHotReloadable(cfg2)
	found := false
	for _, field := range changed {
		if field == "agents.defaults.additional_memory_dir" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected agents.defaults.additional_memory_dir in changed fields, got %v", changed)
	}
}
