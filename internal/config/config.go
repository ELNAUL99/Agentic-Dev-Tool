package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config represents the main application configuration
type Config struct {
	LLM       LLMConfig       `json:"llm"`
	Agents    AgentsConfig    `json:"agents"`
	Plugins   PluginsConfig   `json:"plugins"`
	Git       GitConfig       `json:"git"`
	Docker    DockerConfig    `json:"docker"`
	Memory    MemoryConfig    `json:"memory"`
	Workflows WorkflowsConfig `json:"workflows"`
}

// LLMConfig holds LLM provider settings
type LLMConfig struct {
	Provider    string            `json:"provider"`
	Model       string            `json:"model"`
	APIKey      string            `json:"api_key"`
	BaseURL     string            `json:"base_url"`
	Temperature float64           `json:"temperature"`
	MaxTokens   int               `json:"max_tokens"`
	Timeout     int               `json:"timeout_seconds"`
	Headers     map[string]string `json:"headers,omitempty"`
}

// AgentsConfig configures the multi-agent system
type AgentsConfig struct {
	Planner  AgentRoleConfig `json:"planner"`
	Coder    AgentRoleConfig `json:"coder"`
	Reviewer AgentRoleConfig `json:"reviewer"`
}

// AgentRoleConfig configures a specific agent role
type AgentRoleConfig struct {
	Enabled      bool   `json:"enabled"`
	Model        string `json:"model,omitempty"`
	SystemPrompt string `json:"system_prompt,omitempty"`
	MaxRetries   int    `json:"max_retries"`
}

// PluginsConfig holds plugin settings
type PluginsConfig struct {
	Directory string                  `json:"directory"`
	Enabled   []string                `json:"enabled"`
	Config    map[string]PluginConfig `json:"config,omitempty"`
}

// PluginConfig is plugin-specific configuration
type PluginConfig struct {
	Enabled bool                   `json:"enabled"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

// GitConfig holds git integration settings
type GitConfig struct {
	Enabled      bool   `json:"enabled"`
	AutoCommit   bool   `json:"auto_commit"`
	BranchPrefix string `json:"branch_prefix"`
	CommitPrefix string `json:"commit_prefix"`
	RemoteName   string `json:"remote_name"`
}

// DockerConfig holds Docker sandbox settings
type DockerConfig struct {
	Enabled     bool     `json:"enabled"`
	Image       string   `json:"image"`
	Timeout     int      `json:"timeout_seconds"`
	MemoryLimit string   `json:"memory_limit"`
	CPULimit    string   `json:"cpu_limit"`
	NetworkMode string   `json:"network_mode"`
	Volumes     []string `json:"volumes,omitempty"`
	EnvVars     []string `json:"env_vars,omitempty"`
}

// MemoryConfig holds memory/persistence settings
type MemoryConfig struct {
	Enabled   bool   `json:"enabled"`
	StorePath string `json:"store_path"`
	MaxItems  int    `json:"max_items"`
	Embedding string `json:"embedding_model,omitempty"`
}

// WorkflowsConfig holds workflow settings
type WorkflowsConfig struct {
	DefaultsPath string `json:"defaults_path"`
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		LLM: LLMConfig{
			Provider:    "openai",
			Model:       "gpt-4o",
			Temperature: 0.2,
			MaxTokens:   4096,
			Timeout:     120,
		},
		Agents: AgentsConfig{
			Planner:  AgentRoleConfig{Enabled: true, MaxRetries: 3},
			Coder:    AgentRoleConfig{Enabled: true, MaxRetries: 3},
			Reviewer: AgentRoleConfig{Enabled: true, MaxRetries: 2},
		},
		Plugins: PluginsConfig{
			Directory: ".go-agent/plugins",
			Enabled:   []string{"db-schema", "api-validator"},
			Config:    make(map[string]PluginConfig),
		},
		Git: GitConfig{
			Enabled:      true,
			AutoCommit:   false,
			BranchPrefix: "agent/",
			CommitPrefix: "[agent] ",
			RemoteName:   "origin",
		},
		Docker: DockerConfig{
			Enabled:     true,
			Image:       "golang:1.23-alpine",
			Timeout:     300,
			MemoryLimit: "512m",
			CPULimit:    "1.0",
			NetworkMode: "none",
		},
		Memory: MemoryConfig{
			Enabled:   true,
			StorePath: ".go-agent/memory.db",
			MaxItems:  1000,
		},
		Workflows: WorkflowsConfig{
			DefaultsPath: "configs/workflows",
		},
	}
}

// LoadConfig reads configuration from a JSON file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	// Start from defaults so omitted nested fields keep their intended behavior,
	// while explicit JSON values such as false or 0 can still override them.
	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	return cfg, nil
}
