package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/caarlos0/env/v11"
)

// FlexibleStringSlice is a []string that also accepts JSON numbers,
// so allow_from can contain both "123" and 123.
type FlexibleStringSlice []string

func (f *FlexibleStringSlice) UnmarshalJSON(data []byte) error {
	// Try []string first
	var ss []string
	if err := json.Unmarshal(data, &ss); err == nil {
		*f = ss
		return nil
	}

	// Try []interface{} to handle mixed types
	var raw []interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	result := make([]string, 0, len(raw))
	for _, v := range raw {
		switch val := v.(type) {
		case string:
			result = append(result, val)
		case float64:
			result = append(result, fmt.Sprintf("%.0f", val))
		default:
			result = append(result, fmt.Sprintf("%v", val))
		}
	}
	*f = result
	return nil
}

type Config struct {
	Agents    AgentsConfig    `json:"agents"`
	Channels  ChannelsConfig  `json:"channels"`
	Providers ProvidersConfig `json:"providers"`
	Gateway   GatewayConfig   `json:"gateway"`
	Tools     ToolsConfig     `json:"tools"`
	Heartbeat HeartbeatConfig `json:"heartbeat"`
	Devices   DevicesConfig   `json:"devices"`
	mu        sync.RWMutex
}

type AgentsConfig struct {
	Defaults AgentDefaults `json:"defaults"`
}

// ContextPruningConfig holds configuration for TTL-based context pruning.
type ContextPruningConfig struct {
	Mode                 string  `json:"mode" env:"PICOCLAW_AGENTS_DEFAULTS_CONTEXT_PRUNING_MODE"`
	TTLMinutes           int     `json:"ttl_minutes" env:"PICOCLAW_AGENTS_DEFAULTS_CONTEXT_PRUNING_TTL_MINUTES"`
	KeepLastAssistants   int     `json:"keep_last_assistants" env:"PICOCLAW_AGENTS_DEFAULTS_CONTEXT_PRUNING_KEEP_LAST_ASSISTANTS"`
	SoftTrimRatio        float64 `json:"soft_trim_ratio" env:"PICOCLAW_AGENTS_DEFAULTS_CONTEXT_PRUNING_SOFT_TRIM_RATIO"`
	HardClearRatio       float64 `json:"hard_clear_ratio" env:"PICOCLAW_AGENTS_DEFAULTS_CONTEXT_PRUNING_HARD_CLEAR_RATIO"`
	MinPrunableToolChars int     `json:"min_prunable_tool_chars" env:"PICOCLAW_AGENTS_DEFAULTS_CONTEXT_PRUNING_MIN_PRUNABLE_TOOL_CHARS"`
}

type AgentDefaults struct {
	Workspace              string                 `json:"workspace" env:"PICOCLAW_AGENTS_DEFAULTS_WORKSPACE"`
	RestrictToWorkspace    bool                   `json:"restrict_to_workspace" env:"PICOCLAW_AGENTS_DEFAULTS_RESTRICT_TO_WORKSPACE"`
	Provider               string                 `json:"provider" env:"PICOCLAW_AGENTS_DEFAULTS_PROVIDER"`
	Model                  string                 `json:"model" env:"PICOCLAW_AGENTS_DEFAULTS_MODEL"`
	MaxTokens              int                    `json:"max_tokens" env:"PICOCLAW_AGENTS_DEFAULTS_MAX_TOKENS"`
	Temperature            float64                `json:"temperature" env:"PICOCLAW_AGENTS_DEFAULTS_TEMPERATURE"`
	MaxToolIterations      int                    `json:"max_tool_iterations" env:"PICOCLAW_AGENTS_DEFAULTS_MAX_TOOL_ITERATIONS"`
	BootstrapMaxChars      int                    `json:"bootstrap_max_chars" env:"PICOCLAW_AGENTS_DEFAULTS_BOOTSTRAP_MAX_CHARS"`
	BootstrapTotalMaxChars int                    `json:"bootstrap_total_max_chars" env:"PICOCLAW_AGENTS_DEFAULTS_BOOTSTRAP_TOTAL_MAX_CHARS"`
	ContextPruning         ContextPruningConfig   `json:"context_pruning"`
}

type ChannelsConfig struct {
	Discord DiscordConfig `json:"discord"`
}

type DiscordConfig struct {
	Enabled        bool                         `json:"enabled" env:"PICOCLAW_CHANNELS_DISCORD_ENABLED"`
	Token          string                       `json:"token" env:"PICOCLAW_CHANNELS_DISCORD_TOKEN"`
	AllowFrom      FlexibleStringSlice          `json:"allow_from" env:"PICOCLAW_CHANNELS_DISCORD_ALLOW_FROM"`
	RequireMention bool                         `json:"require_mention" env:"PICOCLAW_CHANNELS_DISCORD_REQUIRE_MENTION"`
	ReplyToMode    string                       `json:"reply_to_mode" env:"PICOCLAW_CHANNELS_DISCORD_REPLY_TO_MODE"` // "off", "first", "all"
	DMPolicy       string                       `json:"dm_policy" env:"PICOCLAW_CHANNELS_DISCORD_DM_POLICY"`       // "open", "allowlist", "disabled"
	Guilds         map[string]DiscordGuildConfig `json:"guilds"`
}

type DiscordGuildConfig struct {
	RequireMention bool                           `json:"require_mention"`
	Channels       map[string]DiscordChannelConfig `json:"channels"`
}

type DiscordChannelConfig struct {
	Allow         bool     `json:"allow"`
	RequireMention bool    `json:"require_mention"`
	Users         []string `json:"users"`
	Roles         []string `json:"roles"`
}

type HeartbeatConfig struct {
	Enabled  bool `json:"enabled" env:"PICOCLAW_HEARTBEAT_ENABLED"`
	Interval int  `json:"interval" env:"PICOCLAW_HEARTBEAT_INTERVAL"` // minutes, min 5
}

type DevicesConfig struct {
	Enabled    bool `json:"enabled" env:"PICOCLAW_DEVICES_ENABLED"`
	MonitorUSB bool `json:"monitor_usb" env:"PICOCLAW_DEVICES_MONITOR_USB"`
}

type ProvidersConfig struct {
	OpenAI        ProviderConfig `json:"openai"`
	OpenRouter    ProviderConfig `json:"openrouter"`
	Groq          ProviderConfig `json:"groq"`
	Zhipu         ProviderConfig `json:"zhipu"`
	VLLM          ProviderConfig `json:"vllm"`
	Gemini        ProviderConfig `json:"gemini"`
	Nvidia        ProviderConfig `json:"nvidia"`
	Ollama        ProviderConfig `json:"ollama"`
	Moonshot      ProviderConfig `json:"moonshot"`
	ShengSuanYun  ProviderConfig `json:"shengsuanyun"`
	DeepSeek      ProviderConfig `json:"deepseek"`
	GitHubCopilot ProviderConfig `json:"github_copilot"`
}

type ProviderConfig struct {
	APIKey      string `json:"api_key" env:"PICOCLAW_PROVIDERS_{{.Name}}_API_KEY"`
	APIBase     string `json:"api_base" env:"PICOCLAW_PROVIDERS_{{.Name}}_API_BASE"`
	Proxy       string `json:"proxy,omitempty" env:"PICOCLAW_PROVIDERS_{{.Name}}_PROXY"`
	AuthMethod  string `json:"auth_method,omitempty" env:"PICOCLAW_PROVIDERS_{{.Name}}_AUTH_METHOD"`
	ConnectMode string `json:"connect_mode,omitempty" env:"PICOCLAW_PROVIDERS_{{.Name}}_CONNECT_MODE"` //only for Github Copilot, `stdio` or `grpc`
}

type GatewayConfig struct {
	Host string `json:"host" env:"PICOCLAW_GATEWAY_HOST"`
	Port int    `json:"port" env:"PICOCLAW_GATEWAY_PORT"`
}

type BraveConfig struct {
	Enabled    bool   `json:"enabled" env:"PICOCLAW_TOOLS_WEB_BRAVE_ENABLED"`
	APIKey     string `json:"api_key" env:"PICOCLAW_TOOLS_WEB_BRAVE_API_KEY"`
	MaxResults int    `json:"max_results" env:"PICOCLAW_TOOLS_WEB_BRAVE_MAX_RESULTS"`
}

type DuckDuckGoConfig struct {
	Enabled    bool `json:"enabled" env:"PICOCLAW_TOOLS_WEB_DUCKDUCKGO_ENABLED"`
	MaxResults int  `json:"max_results" env:"PICOCLAW_TOOLS_WEB_DUCKDUCKGO_MAX_RESULTS"`
}

type PerplexityConfig struct {
	Enabled    bool   `json:"enabled" env:"PICOCLAW_TOOLS_WEB_PERPLEXITY_ENABLED"`
	APIKey     string `json:"api_key" env:"PICOCLAW_TOOLS_WEB_PERPLEXITY_API_KEY"`
	MaxResults int    `json:"max_results" env:"PICOCLAW_TOOLS_WEB_PERPLEXITY_MAX_RESULTS"`
}

type WebToolsConfig struct {
	Brave      BraveConfig      `json:"brave"`
	DuckDuckGo DuckDuckGoConfig `json:"duckduckgo"`
	Perplexity PerplexityConfig `json:"perplexity"`
}

type CronToolsConfig struct {
	ExecTimeoutMinutes int `json:"exec_timeout_minutes" env:"PICOCLAW_TOOLS_CRON_EXEC_TIMEOUT_MINUTES"` // 0 means no timeout
}

type ToolsConfig struct {
	Web  WebToolsConfig  `json:"web"`
	Cron CronToolsConfig `json:"cron"`
}

func DefaultConfig() *Config {
	return &Config{
		Agents: AgentsConfig{
			Defaults: AgentDefaults{
				Workspace:              GetDefaultWorkspace(),
				RestrictToWorkspace:    true,
				Provider:               "",
				Model:                  "glm-4.7",
				MaxTokens:              8192,
				Temperature:            0.7,
				MaxToolIterations:      20,
				BootstrapMaxChars:      20000,
				BootstrapTotalMaxChars: 24000,
				ContextPruning: ContextPruningConfig{
					Mode:                 "off",
					TTLMinutes:           60,
					KeepLastAssistants:   4,
					SoftTrimRatio:        0.3,
					HardClearRatio:       0.5,
					MinPrunableToolChars: 1000,
				},
			},
		},
		Channels: ChannelsConfig{
			Discord: DiscordConfig{
				Enabled:        false,
				Token:          "",
				AllowFrom:      FlexibleStringSlice{},
				RequireMention: false,
				ReplyToMode:    "first",
				DMPolicy:       "allowlist",
				Guilds:         make(map[string]DiscordGuildConfig),
			},
		},
		Providers: ProvidersConfig{
			OpenAI:       ProviderConfig{},
			OpenRouter:   ProviderConfig{},
			Groq:         ProviderConfig{},
			Zhipu:        ProviderConfig{},
			VLLM:         ProviderConfig{},
			Gemini:       ProviderConfig{},
			Nvidia:       ProviderConfig{},
			Moonshot:     ProviderConfig{},
			ShengSuanYun: ProviderConfig{},
		},
		Gateway: GatewayConfig{
			Host: "0.0.0.0",
			Port: 18790,
		},
		Tools: ToolsConfig{
			Web: WebToolsConfig{
				Brave: BraveConfig{
					Enabled:    false,
					APIKey:     "",
					MaxResults: 5,
				},
				DuckDuckGo: DuckDuckGoConfig{
					Enabled:    true,
					MaxResults: 5,
				},
				Perplexity: PerplexityConfig{
					Enabled:    false,
					APIKey:     "",
					MaxResults: 5,
				},
			},
			Cron: CronToolsConfig{
				ExecTimeoutMinutes: 5, // default 5 minutes for LLM operations
			},
		},
		Heartbeat: HeartbeatConfig{
			Enabled:  true,
			Interval: 30, // default 30 minutes
		},
		Devices: DevicesConfig{
			Enabled:    false,
			MonitorUSB: true,
		},
	}
}

func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	if err := env.Parse(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func SaveConfig(path string, cfg *Config) error {
	cfg.mu.RLock()
	defer cfg.mu.RUnlock()

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func (c *Config) WorkspacePath() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return expandHome(c.Agents.Defaults.Workspace)
}

func (c *Config) GetAPIKey() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.Providers.OpenRouter.APIKey != "" {
		return c.Providers.OpenRouter.APIKey
	}
	if c.Providers.OpenAI.APIKey != "" {
		return c.Providers.OpenAI.APIKey
	}
	if c.Providers.Gemini.APIKey != "" {
		return c.Providers.Gemini.APIKey
	}
	if c.Providers.Zhipu.APIKey != "" {
		return c.Providers.Zhipu.APIKey
	}
	if c.Providers.Groq.APIKey != "" {
		return c.Providers.Groq.APIKey
	}
	if c.Providers.VLLM.APIKey != "" {
		return c.Providers.VLLM.APIKey
	}
	if c.Providers.ShengSuanYun.APIKey != "" {
		return c.Providers.ShengSuanYun.APIKey
	}
	return ""
}

func (c *Config) GetAPIBase() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.Providers.OpenRouter.APIKey != "" {
		if c.Providers.OpenRouter.APIBase != "" {
			return c.Providers.OpenRouter.APIBase
		}
		return "https://openrouter.ai/api/v1"
	}
	if c.Providers.Zhipu.APIKey != "" {
		return c.Providers.Zhipu.APIBase
	}
	if c.Providers.VLLM.APIKey != "" && c.Providers.VLLM.APIBase != "" {
		return c.Providers.VLLM.APIBase
	}
	return ""
}

func expandHome(path string) string {
	if path == "" {
		return path
	}
	if path[0] == '~' {
		home, _ := os.UserHomeDir()
		if len(path) > 1 && path[1] == '/' {
			return home + path[1:]
		}
		return home
	}
	return path
}
