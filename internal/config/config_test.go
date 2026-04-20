package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.LLM.Provider != "openai" {
		t.Errorf("expected provider 'openai', got %s", cfg.LLM.Provider)
	}
	if cfg.LLM.Model != "gpt-4o" {
		t.Errorf("expected model 'gpt-4o', got %s", cfg.LLM.Model)
	}
	if cfg.LLM.Temperature != 0.2 {
		t.Errorf("expected temperature 0.2, got %f", cfg.LLM.Temperature)
	}
	if cfg.Docker.Image != "golang:1.23-alpine" {
		t.Errorf("expected docker image 'golang:1.23-alpine', got %s", cfg.Docker.Image)
	}
	if cfg.Git.BranchPrefix != "agent/" {
		t.Errorf("expected branch prefix 'agent/', got %s", cfg.Git.BranchPrefix)
	}
}

func TestLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	cfg := DefaultConfig()
	cfg.LLM.Model = "gpt-4o-mini"
	data, _ := json.Marshal(cfg)
	os.WriteFile(configPath, data, 0644)

	loaded, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("load config failed: %v", err)
	}

	if loaded.LLM.Model != "gpt-4o-mini" {
		t.Errorf("expected model 'gpt-4o-mini', got %s", loaded.LLM.Model)
	}
	if loaded.LLM.Provider != "openai" {
		t.Errorf("expected provider 'openai', got %s", loaded.LLM.Provider)
	}
}

func TestLoadConfigMissingDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	minimal := map[string]interface{}{
		"llm": map[string]interface{}{
			"api_key": "test-key",
		},
	}
	data, _ := json.Marshal(minimal)
	os.WriteFile(configPath, data, 0644)

	loaded, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("load config failed: %v", err)
	}

	if loaded.LLM.Provider != "openai" {
		t.Errorf("expected default provider 'openai', got %s", loaded.LLM.Provider)
	}
	if loaded.LLM.Model != "gpt-4o" {
		t.Errorf("expected default model 'gpt-4o', got %s", loaded.LLM.Model)
	}
	if !loaded.Docker.Enabled {
		t.Error("expected default docker enabled setting to survive partial config")
	}
	if !loaded.Git.Enabled {
		t.Error("expected default git enabled setting to survive partial config")
	}
	if !loaded.Memory.Enabled {
		t.Error("expected default memory enabled setting to survive partial config")
	}
}

func TestLoadConfigAllowsExplicitFalseOverride(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	minimal := map[string]interface{}{
		"docker": map[string]interface{}{
			"enabled": false,
		},
	}
	data, _ := json.Marshal(minimal)
	os.WriteFile(configPath, data, 0644)

	loaded, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("load config failed: %v", err)
	}

	if loaded.Docker.Enabled {
		t.Error("expected explicit docker.enabled=false to override default")
	}
}
