package plugins

import (
	"context"
	"testing"

	"github.com/go-agent/go-agent/internal/config"
	"github.com/go-agent/go-agent/internal/models"
)

func TestDBSchemaPlugin(t *testing.T) {
	p := &DBSchemaPlugin{}
	if p.Name() != "db-schema" {
		t.Errorf("expected name 'db-schema', got %s", p.Name())
	}

	ctx := context.Background()
	task := models.Task{
		Title:       "Create user database model",
		Description: "Define user model table schema",
	}

	result, err := p.Execute(ctx, task, nil)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	if _, ok := result["schema"]; !ok {
		t.Error("expected schema in result")
	}
}

func TestAPIValidatorPlugin(t *testing.T) {
	p := &APIValidatorPlugin{}
	if p.Name() != "api-validator" {
		t.Errorf("expected name 'api-validator', got %s", p.Name())
	}

	ctx := context.Background()
	task := models.Task{
		Title:       "Add user API endpoint",
		Description: "Create REST endpoint for user management without pagination",
	}

	result, err := p.Execute(ctx, task, nil)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	if _, ok := result["api_issues"]; !ok {
		t.Error("expected api_issues in result")
	}
}

func TestLoadTestPlugin(t *testing.T) {
	p := &LoadTestPlugin{}
	if p.Name() != "load-test" {
		t.Errorf("expected name 'load-test', got %s", p.Name())
	}

	ctx := context.Background()
	task := models.Task{Title: "Add search endpoint"}
	result, err := p.Execute(ctx, task, nil)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	if _, ok := result["load_test_suggestion"]; !ok {
		t.Error("expected load_test_suggestion in result")
	}
}

func TestIsRelevant(t *testing.T) {
	dbPlugin := &DBSchemaPlugin{}
	apiPlugin := &APIValidatorPlugin{}
	loadPlugin := &LoadTestPlugin{}

	tests := []struct {
		plugin   Plugin
		task     models.Task
		expected bool
	}{
		{dbPlugin, models.Task{Title: "Create database model"}, true},
		{dbPlugin, models.Task{Title: "Add API endpoint"}, false},
		{apiPlugin, models.Task{Title: "Add REST endpoint"}, true},
		{apiPlugin, models.Task{Title: "Database migration"}, false},
		{loadPlugin, models.Task{Title: "Performance optimization"}, true},
		{loadPlugin, models.Task{Title: "Add login page"}, false},
	}

	for _, tt := range tests {
		got := isRelevant(tt.plugin, &tt.task)
		if got != tt.expected {
			t.Errorf("isRelevant(%s, %s) = %v, want %v", tt.plugin.Name(), tt.task.Title, got, tt.expected)
		}
	}
}

func TestManagerExecuteRelevant(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Plugins.Enabled = []string{"db-schema", "api-validator"}

	mgr := New(cfg)
	ctx := context.Background()
	task := &models.Task{Title: "Create user database model", Description: "Schema design"}

	results, err := mgr.ExecuteRelevant(ctx, task)
	if err != nil {
		t.Fatalf("execute relevant failed: %v", err)
	}

	if len(results) == 0 {
		t.Error("expected plugin results for db task")
	}
}

func TestManagerExplicitPluginDisable(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Plugins.Enabled = []string{"db-schema"}
	cfg.Plugins.Config["db-schema"] = config.PluginConfig{Enabled: false}

	mgr := New(cfg)
	ctx := context.Background()
	task := &models.Task{Title: "Create user database model", Description: "Schema design"}

	results, err := mgr.ExecuteRelevant(ctx, task)
	if err != nil {
		t.Fatalf("execute relevant failed: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected explicitly disabled plugin to be skipped, got %v", results)
	}
}
