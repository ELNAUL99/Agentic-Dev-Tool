package reviewer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-agent/go-agent/internal/config"
	"github.com/go-agent/go-agent/internal/models"
	"github.com/go-agent/go-agent/pkg/llm"
)

const systemPrompt = `You are a Senior Go Code Reviewer. Evaluate code for:
1. Correctness - does it work as intended?
2. Go idioms - does it follow Go best practices?
3. Error handling - are errors properly handled and wrapped?
4. Security - are there injection risks, race conditions, leaks?
5. Performance - are there obvious bottlenecks?
6. Testing - is the code testable? Are edge cases covered?
7. Documentation - are exported items documented?

Output a JSON review with score (0-100), issues, and suggestions.
Be strict but constructive. Only approve if score >= 80 and no critical issues.

JSON format:
{
  "score": 85,
  "approved": true,
  "summary": "brief summary",
  "issues": [
    {"severity": "high|medium|low|info", "file": "path", "line": 42, "message": "description", "category": "security|performance|style|correctness"}
  ],
  "suggestions": ["improvement 1", "improvement 2"]
}`

// Agent reviews generated code
type Agent struct {
	client llm.Client
	config *config.Config
}

// New creates a new reviewer agent
func New(client llm.Client, cfg *config.Config) *Agent {
	return &Agent{client: client, config: cfg}
}

// Review evaluates code artifacts for a task
func (a *Agent) Review(ctx context.Context, task *models.Task, artifacts []models.CodeArtifact) (*models.ReviewResult, error) {
	prompt := buildReviewPrompt(task, artifacts)

	resp, err := a.client.Complete(ctx, []models.PromptMessage{
		{Role: "user", Content: prompt},
	}, &llm.CompletionOptions{
		SystemPrompt: systemPrompt,
		Temperature:  0.2,
		JSONMode:     true,
	})
	if err != nil {
		return nil, fmt.Errorf("review failed: %w", err)
	}

	var result models.ReviewResult
	content := extractJSON(resp.Content)
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("parsing review (raw: %s): %w", resp.Content, err)
	}

	result.TaskID = task.ID

	// Auto-approve threshold
	if result.Score >= 80 && len(result.Issues) == 0 {
		result.Approved = true
	}

	return &result, nil
}

func buildReviewPrompt(task *models.Task, artifacts []models.CodeArtifact) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Review task: %s\n", task.Title))
	sb.WriteString(fmt.Sprintf("Description: %s\n\n", task.Description))

	sb.WriteString("Code to review:\n\n")
	for _, art := range artifacts {
		if strings.HasSuffix(art.FilePath, "_test.go") {
			continue
		}
		sb.WriteString(fmt.Sprintf("--- %s ---\n%s\n\n", art.FilePath, art.Content))
	}

	sb.WriteString("Return JSON review.")
	return sb.String()
}

func extractJSON(content string) string {
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start != -1 && end != -1 && end > start {
		return content[start : end+1]
	}
	return content
}
