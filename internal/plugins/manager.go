package plugins

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-agent/go-agent/internal/config"
	"github.com/go-agent/go-agent/internal/models"
)

// Manager handles plugin lifecycle
type Manager struct {
	config  *config.Config
	plugins map[string]Plugin
}

// Plugin is a tool extension for the agent
type Plugin interface {
	Name() string
	Description() string
	Execute(ctx context.Context, task models.Task, config map[string]interface{}) (map[string]string, error)
}

// New creates a plugin manager with built-in plugins
func New(cfg *config.Config) *Manager {
	m := &Manager{
		config:  cfg,
		plugins: make(map[string]Plugin),
	}

	// Register built-in plugins
	m.Register(&DBSchemaPlugin{})
	m.Register(&APIValidatorPlugin{})
	m.Register(&LoadTestPlugin{})

	return m
}

// Register adds a plugin
func (m *Manager) Register(p Plugin) {
	m.plugins[p.Name()] = p
}

// ExecuteRelevant runs plugins applicable to a task
func (m *Manager) ExecuteRelevant(ctx context.Context, task *models.Task) (map[string]string, error) {
	results := make(map[string]string)

	for _, name := range m.config.Plugins.Enabled {
		p, ok := m.plugins[name]
		if !ok {
			continue
		}

		cfg, configured := m.config.Plugins.Config[name]
		// The enabled list is the primary opt-in; per-plugin config only disables
		// a listed plugin when the user explicitly sets enabled=false.
		if configured && !cfg.Enabled {
			continue
		}

		// Check if plugin is relevant to task
		if !isRelevant(p, task) {
			continue
		}

		out, err := p.Execute(ctx, *task, cfg.Params)
		if err != nil {
			results[name+"_error"] = err.Error()
			continue
		}

		for k, v := range out {
			results[name+"_"+k] = v
		}
	}

	return results, nil
}

func isRelevant(p Plugin, task *models.Task) bool {
	name := strings.ToLower(p.Name())
	title := strings.ToLower(task.Title)
	desc := strings.ToLower(task.Description)

	switch {
	case strings.Contains(name, "db") || strings.Contains(name, "schema"):
		return strings.Contains(title, "database") ||
			strings.Contains(title, "model") ||
			strings.Contains(title, "schema") ||
			strings.Contains(title, "migration") ||
			strings.Contains(title, "table") ||
			strings.Contains(desc, "database") ||
			strings.Contains(desc, "schema")
	case strings.Contains(name, "api") || strings.Contains(name, "validator"):
		return strings.Contains(title, "api") ||
			strings.Contains(title, "endpoint") ||
			strings.Contains(title, "handler") ||
			strings.Contains(title, "route")
	case strings.Contains(name, "load") || strings.Contains(name, "perf"):
		return strings.Contains(title, "performance") ||
			strings.Contains(title, "load") ||
			strings.Contains(title, "benchmark")
	}

	return false
}

// --- Built-in Plugins ---

// DBSchemaPlugin generates database schema suggestions
type DBSchemaPlugin struct{}

func (p *DBSchemaPlugin) Name() string        { return "db-schema" }
func (p *DBSchemaPlugin) Description() string { return "Suggests database schema for models" }

func (p *DBSchemaPlugin) Execute(ctx context.Context, task models.Task, cfg map[string]interface{}) (map[string]string, error) {
	// Parse task description for entities
	entities := extractEntities(task.Description)

	var schema strings.Builder
	schema.WriteString("Suggested schema:\n\n")
	for _, entity := range entities {
		schema.WriteString(fmt.Sprintf("Table: %s\n", entity))
		schema.WriteString("  id: bigint primary key\n")
		schema.WriteString("  created_at: timestamp\n")
		schema.WriteString("  updated_at: timestamp\n\n")
	}

	return map[string]string{
		"schema":   schema.String(),
		"entities": strings.Join(entities, ", "),
	}, nil
}

// APIValidatorPlugin validates API design
type APIValidatorPlugin struct{}

func (p *APIValidatorPlugin) Name() string        { return "api-validator" }
func (p *APIValidatorPlugin) Description() string { return "Validates API design patterns" }

func (p *APIValidatorPlugin) Execute(ctx context.Context, task models.Task, cfg map[string]interface{}) (map[string]string, error) {
	issues := []string{}

	desc := strings.ToLower(task.Description)
	if !strings.Contains(desc, "validate") && !strings.Contains(desc, "validation") {
		issues = append(issues, "Consider adding input validation")
	}
	if !strings.Contains(desc, "auth") && !strings.Contains(desc, "authenticate") {
		issues = append(issues, "Consider authentication requirements")
	}
	if !strings.Contains(desc, "paginat") {
		issues = append(issues, "Consider pagination for list endpoints")
	}

	return map[string]string{
		"api_issues":     strings.Join(issues, "; "),
		"recommendation": "Follow REST conventions with proper status codes",
	}, nil
}

// LoadTestPlugin generates load test suggestions
type LoadTestPlugin struct{}

func (p *LoadTestPlugin) Name() string        { return "load-test" }
func (p *LoadTestPlugin) Description() string { return "Suggests load test scenarios" }

func (p *LoadTestPlugin) Execute(ctx context.Context, task models.Task, cfg map[string]interface{}) (map[string]string, error) {
	return map[string]string{
		"load_test_suggestion": "Consider testing with concurrent requests using k6 or vegeta",
		"benchmark_suggestion": "Add Go benchmarks for hot paths",
	}, nil
}

func extractEntities(description string) []string {
	var entities []string
	words := strings.Fields(description)
	for i, w := range words {
		w = strings.ToLower(strings.TrimRight(w, "s,.;:"))
		if (w == "model" || w == "entity" || w == "table") && i > 0 {
			entities = append(entities, words[i-1])
		}
	}
	if len(entities) == 0 {
		// Fallback: look for capitalized words
		for _, w := range words {
			w = strings.Trim(w, ".,;:")
			if len(w) > 2 && w[0] >= 'A' && w[0] <= 'Z' {
				entities = append(entities, w)
			}
		}
	}
	return entities
}

// LoadExternalPlugins loads plugins from the plugins directory
func (m *Manager) LoadExternalPlugins(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading plugins dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// Could load external plugin binaries here
			_ = filepath.Join(dir, entry.Name())
		}
	}

	return nil
}
