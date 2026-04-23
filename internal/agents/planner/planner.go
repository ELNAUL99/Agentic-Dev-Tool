package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-agent/go-agent/internal/config"
	"github.com/go-agent/go-agent/internal/models"
	"github.com/go-agent/go-agent/pkg/llm"
)

const systemPrompt = `You are a Planning Agent for a Go backend development system.
Your job is to break down feature specifications into concrete, ordered development tasks.

Rules:
1. Each task must be specific and implementable
2. Tasks should be ordered by dependency (infrastructure first, features second)
3. Include test tasks for each code task
4. Consider: error handling, validation, logging, and edge cases
5. Output as a JSON array of tasks

Task types: code, test, refactor, docs, config

JSON format:
[
  {
    "id": "task-1",
    "title": "short title",
    "description": "detailed description including acceptance criteria",
    "type": "code|test|refactor|docs|config",
    "priority": 1-10,
    "depends_on": ["task-id"],
    "files": ["expected/file.go"]
  }
]`

// Agent breaks down specs into tasks
type Agent struct {
	client llm.Client
	config *config.Config
}

// New creates a new planner agent
func New(client llm.Client, cfg *config.Config) *Agent {
	return &Agent{client: client, config: cfg}
}

// Plan converts a feature spec into a list of development tasks
func (a *Agent) Plan(ctx context.Context, spec models.FeatureSpec) ([]models.Task, error) {
	prompt := buildPlanningPrompt(spec)

	resp, err := a.client.Complete(ctx, []models.PromptMessage{
		{Role: "user", Content: prompt},
	}, &llm.CompletionOptions{
		SystemPrompt: systemPrompt,
		Temperature:  0.3,
		JSONMode:     true,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	var tasks []models.Task
	if err := json.Unmarshal([]byte(resp.Content), &tasks); err != nil {
		// Try extracting JSON from markdown code block
		content := extractJSON(resp.Content)
		if err2 := json.Unmarshal([]byte(content), &tasks); err2 != nil {
			return nil, fmt.Errorf("parsing tasks (raw: %s): %w", resp.Content, err)
		}
	}

	// Normalize and validate
	for i := range tasks {
		tasks[i].ID = fmt.Sprintf("%s-%d", spec.Title, i+1)
		if tasks[i].Status == "" {
			tasks[i].Status = models.StatusPending
		}
		if tasks[i].Type == "" {
			tasks[i].Type = models.TaskTypeCode
		}
	}

	return tasks, nil
}

func buildPlanningPrompt(spec models.FeatureSpec) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Feature: %s\n\n", spec.Title))
	sb.WriteString(fmt.Sprintf("Description: %s\n\n", spec.Description))

	if spec.Context != "" {
		sb.WriteString(fmt.Sprintf("Context:\n%s\n\n", spec.Context))
	}

	if len(spec.Constraints) > 0 {
		sb.WriteString("Constraints:\n")
		for _, c := range spec.Constraints {
			sb.WriteString(fmt.Sprintf("- %s\n", c))
		}
		sb.WriteString("\n")
	}

	if len(spec.Tags) > 0 {
		sb.WriteString(fmt.Sprintf("Tags: %s\n\n", strings.Join(spec.Tags, ", ")))
	}

	sb.WriteString("Generate a task breakdown as JSON. Include both implementation and test tasks.")
	return sb.String()
}

func extractJSON(content string) string {
	start := strings.Index(content, "[")
	end := strings.LastIndex(content, "]")
	if start != -1 && end != -1 && end > start {
		return content[start : end+1]
	}
	start = strings.Index(content, "{")
	end = strings.LastIndex(content, "}")
	if start != -1 && end != -1 && end > start {
		return content[start : end+1]
	}
	return content
}
