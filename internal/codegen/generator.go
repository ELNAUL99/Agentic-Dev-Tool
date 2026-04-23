package codegen

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/go-agent/go-agent/internal/config"
	"github.com/go-agent/go-agent/internal/models"
	"github.com/go-agent/go-agent/pkg/llm"
)

const systemPrompt = "You are a Go code generator. Generate complete, compilable Go files.\n" +
	"Always output with proper package declarations and imports.\n" +
	"Follow these rules:\n" +
	"1. Generate idiomatic Go code\n" +
	"2. Handle errors explicitly\n" +
	"3. Use context.Context for cancellation\n" +
	"4. Document all exported identifiers\n" +
	"5. Group imports properly (std lib, external, internal)\n" +
	"6. Prefer interfaces for testability"

// Generator handles code generation
type Generator struct {
	client llm.Client
	config *config.Config
}

// New creates a code generator
func New(client llm.Client, cfg *config.Config) *Generator {
	return &Generator{client: client, config: cfg}
}

// GenerateModule creates a complete Go module structure
func (g *Generator) GenerateModule(ctx context.Context, moduleName string, types []string) ([]models.CodeArtifact, error) {
	prompt := fmt.Sprintf(
		"Generate a Go module named %s with the following types:\n%s\n\n"+
			"Create:\n"+
			"1. go.mod\n"+
			"2. models.go - domain types\n"+
			"3. service.go - business logic\n"+
			"4. handler.go - HTTP handlers (using net/http or similar)\n"+
			"5. repository.go - data access interface\n"+
			"6. errors.go - custom errors\n\n"+
			"Output format for each file:\n"+
			"### file: path\n"+
			"```go\n"+
			"code\n"+
			"```\n",
		moduleName, strings.Join(types, ", "))

	resp, err := g.client.Complete(ctx, []models.PromptMessage{
		{Role: "user", Content: prompt},
	}, &llm.CompletionOptions{
		SystemPrompt: systemPrompt,
		Temperature:  0.2,
		MaxTokens:    g.config.LLM.MaxTokens,
	})
	if err != nil {
		return nil, err
	}

	return parseArtifacts(resp.Content), nil
}

// GenerateEndpoint creates a single REST endpoint
func (g *Generator) GenerateEndpoint(ctx context.Context, method, path string, requestType, responseType string) ([]models.CodeArtifact, error) {
	prompt := fmt.Sprintf(
		"Generate a Go HTTP handler for:\n"+
			"Method: %s\n"+
			"Path: %s\n"+
			"Request: %s\n"+
			"Response: %s\n\n"+
			"Include request validation, error handling, and a handler test.\n"+
			"Output in ### file: format with code blocks.",
		method, path, requestType, responseType)

	resp, err := g.client.Complete(ctx, []models.PromptMessage{
		{Role: "user", Content: prompt},
	}, &llm.CompletionOptions{
		SystemPrompt: systemPrompt,
		Temperature:  0.2,
	})
	if err != nil {
		return nil, err
	}

	return parseArtifacts(resp.Content), nil
}

// GenerateMigration creates a database migration file
func (g *Generator) GenerateMigration(ctx context.Context, tableName string, fields map[string]string) (models.CodeArtifact, error) {
	var fieldDefs []string
	for name, typ := range fields {
		fieldDefs = append(fieldDefs, fmt.Sprintf("%s %s", name, typ))
	}

	prompt := fmt.Sprintf(
		"Generate a Go struct and SQL migration for table %s with fields:\n"+
			"%s\n\n"+
			"Include gorm or sqlx tags if appropriate.",
		tableName, strings.Join(fieldDefs, "\n"))

	resp, err := g.client.Complete(ctx, []models.PromptMessage{
		{Role: "user", Content: prompt},
	}, &llm.CompletionOptions{
		SystemPrompt: systemPrompt,
		Temperature:  0.2,
	})
	if err != nil {
		return models.CodeArtifact{}, err
	}

	arts := parseArtifacts(resp.Content)
	if len(arts) == 0 {
		return models.CodeArtifact{}, fmt.Errorf("no artifacts generated")
	}
	return arts[0], nil
}

func parseArtifacts(content string) []models.CodeArtifact {
	var artifacts []models.CodeArtifact
	parts := strings.Split(content, "### file:")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		lines := strings.SplitN(part, "\n", 2)
		if len(lines) < 2 {
			continue
		}

		filePath := strings.TrimSpace(lines[0])
		code := extractCode(lines[1])

		if filePath == "" || code == "" {
			continue
		}

		lang := "go"
		ext := filepath.Ext(filePath)
		switch ext {
		case ".sql":
			lang = "sql"
		case ".yaml", ".yml":
			lang = "yaml"
		case ".md":
			lang = "markdown"
		case ".mod":
			lang = "go"
		}

		artifacts = append(artifacts, models.CodeArtifact{
			FilePath: filePath,
			Content:  code,
			Language: lang,
		})
	}

	return artifacts
}

func extractCode(content string) string {
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```") {
		idx := strings.Index(content[3:], "```")
		if idx != -1 {
			lines := strings.Split(content[3:idx+3], "\n")
			if len(lines) > 0 && !strings.Contains(lines[0], " ") {
				return strings.Join(lines[1:], "\n")
			}
			return strings.Join(lines, "\n")
		}
	}
	return content
}
