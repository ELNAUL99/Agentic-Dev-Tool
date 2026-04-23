package coder

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/go-agent/go-agent/internal/config"
	"github.com/go-agent/go-agent/internal/models"
	"github.com/go-agent/go-agent/pkg/llm"
)

const systemPrompt = "You are an expert Go backend developer. Generate clean, idiomatic Go code.\n" +
	"Rules:\n" +
	"1. Follow Go best practices and conventions\n" +
	"2. Include proper error handling\n" +
	"3. Add comments for exported functions\n" +
	"4. Use standard library where possible\n" +
	"5. Include context.Context as first parameter for async operations\n" +
	"6. Return (value, error) tuples\n" +
	"7. Write table-driven tests\n" +
	"8. Always include build tags and package declarations\n" +
	"9. Use meaningful variable names\n" +
	"10. Handle edge cases and nil checks"

// Agent generates and fixes code
type Agent struct {
	client llm.Client
	config *config.Config
}

// New creates a new coder agent
func New(client llm.Client, cfg *config.Config) *Agent {
	return &Agent{client: client, config: cfg}
}

// Generate produces code artifacts for a task
func (a *Agent) Generate(ctx context.Context, task models.Task, spec models.FeatureSpec, pluginCtx map[string]string) ([]models.CodeArtifact, error) {
	prompt := buildCodePrompt(task, spec, pluginCtx)

	opts := &llm.CompletionOptions{
		SystemPrompt: systemPrompt,
		Temperature:  0.2,
		MaxTokens:    a.config.LLM.MaxTokens,
	}

	resp, err := a.client.Complete(ctx, []models.PromptMessage{
		{Role: "user", Content: prompt},
	}, opts)
	if err != nil {
		return nil, fmt.Errorf("code generation failed: %w", err)
	}

	artifacts := parseCodeResponse(resp.Content, task)
	return artifacts, nil
}

// GenerateTests creates test files for existing code
func (a *Agent) GenerateTests(ctx context.Context, task models.Task, artifacts []models.CodeArtifact) ([]models.CodeArtifact, error) {
	var codeArtifacts []models.CodeArtifact
	for _, art := range artifacts {
		if !strings.HasSuffix(art.FilePath, "_test.go") && art.Language == "go" {
			codeArtifacts = append(codeArtifacts, art)
		}
	}

	if len(codeArtifacts) == 0 {
		return nil, fmt.Errorf("no Go code artifacts to test")
	}

	var results []models.CodeArtifact
	for _, art := range codeArtifacts {
		prompt := "Generate comprehensive Go tests for the following code.\n\n" +
			"Requirements:\n" +
			"- Use table-driven tests\n" +
			"- Test happy path and error cases\n" +
			"- Mock external dependencies\n" +
			"- Include benchmarks if appropriate\n" +
			"- Target 80%+ coverage\n\n" +
			fmt.Sprintf("Code (%s):\n%s\n\nGenerate only the test file content.", art.FilePath, art.Content)

		resp, err := a.client.Complete(ctx, []models.PromptMessage{
			{Role: "user", Content: prompt},
		}, &llm.CompletionOptions{
			SystemPrompt: systemPrompt,
			Temperature:  0.2,
		})
		if err != nil {
			continue
		}

		testPath := strings.TrimSuffix(art.FilePath, ".go") + "_test.go"
		results = append(results, models.CodeArtifact{
			FilePath:    testPath,
			Content:     extractCode(resp.Content),
			Language:    "go",
			Description: fmt.Sprintf("Tests for %s", filepath.Base(art.FilePath)),
		})
	}

	return results, nil
}

// FixTests attempts to fix failing tests based on error output
func (a *Agent) FixTests(ctx context.Context, result models.TestResult, artifacts []models.CodeArtifact) ([]models.CodeArtifact, error) {
	if result.Passed || len(result.Failures) == 0 {
		return nil, nil
	}

	var targetArtifact *models.CodeArtifact
	for i := range artifacts {
		if strings.Contains(artifacts[i].FilePath, result.TaskID) ||
			(filepath.Base(artifacts[i].FilePath) == result.TaskID) {
			targetArtifact = &artifacts[i]
			break
		}
	}

	if targetArtifact == nil {
		return nil, fmt.Errorf("could not find artifact for task %s", result.TaskID)
	}

	var failureDesc strings.Builder
	for _, f := range result.Failures {
		failureDesc.WriteString(fmt.Sprintf("Test: %s\nError: %s\nFile: %s:%d\n\n",
			f.TestName, f.Message, f.File, f.Line))
	}

	prompt := fmt.Sprintf("Fix the following Go code so all tests pass.\n\nTest Failures:\n%s\n\nCurrent Code:\n%s\n\nProvide the corrected code only.",
		failureDesc.String(), targetArtifact.Content)

	resp, err := a.client.Complete(ctx, []models.PromptMessage{
		{Role: "user", Content: prompt},
	}, &llm.CompletionOptions{
		SystemPrompt: systemPrompt,
		Temperature:  0.2,
	})
	if err != nil {
		return nil, err
	}

	return []models.CodeArtifact{{
		FilePath:    targetArtifact.FilePath,
		Content:     extractCode(resp.Content),
		Language:    "go",
		Description: targetArtifact.Description,
	}}, nil
}

// Refine improves code based on human feedback
func (a *Agent) Refine(ctx context.Context, tasks []models.Task, artifacts []models.CodeArtifact, feedback string) ([]models.CodeArtifact, error) {
	var sb strings.Builder
	sb.WriteString("Refine the following Go backend code based on feedback.\n\n")
	sb.WriteString(fmt.Sprintf("Feedback: %s\n\n", feedback))
	sb.WriteString("Current code:\n\n")

	for _, art := range artifacts {
		if strings.HasSuffix(art.FilePath, "_test.go") {
			continue
		}
		sb.WriteString(fmt.Sprintf("--- %s ---\n%s\n\n", art.FilePath, art.Content))
	}

	sb.WriteString("Provide the complete updated files.")

	resp, err := a.client.Complete(ctx, []models.PromptMessage{
		{Role: "user", Content: sb.String()},
	}, &llm.CompletionOptions{
		SystemPrompt: systemPrompt,
		Temperature:  0.3,
	})
	if err != nil {
		return nil, err
	}

	return parseCodeResponse(resp.Content, models.Task{Title: "refined"}), nil
}

func buildCodePrompt(task models.Task, spec models.FeatureSpec, pluginCtx map[string]string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Task: %s\n", task.Title))
	sb.WriteString(fmt.Sprintf("Description: %s\n\n", task.Description))

	if spec.Context != "" {
		sb.WriteString(fmt.Sprintf("Project Context:\n%s\n\n", spec.Context))
	}

	if len(task.DependsOn) > 0 {
		sb.WriteString(fmt.Sprintf("Dependencies: %v\n\n", task.DependsOn))
	}

	if len(pluginCtx) > 0 {
		sb.WriteString("Plugin Context:\n")
		for k, v := range pluginCtx {
			sb.WriteString(fmt.Sprintf("%s: %s\n", k, v))
		}
		sb.WriteString("\n")
	}

	if len(task.Files) > 0 {
		sb.WriteString(fmt.Sprintf("Expected files: %v\n\n", task.Files))
	}

	sb.WriteString("Generate Go code for this task. Output each file in this format:\n\n")
	sb.WriteString("### file: path/to/file.go\n")
	sb.WriteString("```go\n")
	sb.WriteString("package ...\n")
	sb.WriteString("// code\n")
	sb.WriteString("```\n\n")
	sb.WriteString("### file: path/to/file_test.go\n")
	sb.WriteString("```go\n")
	sb.WriteString("package ...\n")
	sb.WriteString("// test code\n")
	sb.WriteString("```\n")

	return sb.String()
}

func parseCodeResponse(content string, task models.Task) []models.CodeArtifact {
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
		if strings.HasSuffix(filePath, ".yaml") || strings.HasSuffix(filePath, ".yml") {
			lang = "yaml"
		} else if strings.HasSuffix(filePath, ".md") {
			lang = "markdown"
		}

		artifacts = append(artifacts, models.CodeArtifact{
			FilePath:    filePath,
			Content:     code,
			Language:    lang,
			Description: task.Title,
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
