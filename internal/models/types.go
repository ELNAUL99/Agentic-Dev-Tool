package models

import "time"

// Task represents a unit of work generated from a spec
type Task struct {
	ID          string   `json:"id" yaml:"id"`
	Title       string   `json:"title" yaml:"title"`
	Description string   `json:"description" yaml:"description"`
	Type        TaskType `json:"type" yaml:"type"`
	Status      Status   `json:"status" yaml:"status"`
	Priority    int      `json:"priority" yaml:"priority"`
	DependsOn   []string `json:"depends_on,omitempty" yaml:"depends_on,omitempty"`
	Files       []string `json:"files,omitempty" yaml:"files,omitempty"`
	Tests       []string `json:"tests,omitempty" yaml:"tests,omitempty"`
	Output      string   `json:"output,omitempty" yaml:"output,omitempty"`
}

// TaskType defines the category of work
type TaskType string

const (
	TaskTypeCode      TaskType = "code"
	TaskTypeTest      TaskType = "test"
	TaskTypeRefactor  TaskType = "refactor"
	TaskTypeDocs      TaskType = "docs"
	TaskTypeConfig    TaskType = "config"
)

// Status represents the state of a workflow item
type Status string

const (
	StatusPending    Status = "pending"
	StatusRunning    Status = "running"
	StatusCompleted  Status = "completed"
	StatusFailed     Status = "failed"
	StatusApproved   Status = "approved"
	StatusRejected   Status = "rejected"
)

// FeatureSpec is the user input describing what to build
type FeatureSpec struct {
	Title       string            `json:"title" yaml:"title"`
	Description string            `json:"description" yaml:"description"`
	Context     string            `json:"context,omitempty" yaml:"context,omitempty"`
	Tags        []string          `json:"tags,omitempty" yaml:"tags,omitempty"`
	Constraints []string          `json:"constraints,omitempty" yaml:"constraints,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// CodeArtifact represents generated code
type CodeArtifact struct {
	FilePath    string `json:"file_path" yaml:"file_path"`
	Content     string `json:"content" yaml:"content"`
	Language    string `json:"language" yaml:"language"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

// TestResult holds the outcome of test execution
type TestResult struct {
	TaskID      string        `json:"task_id"`
	Passed      bool          `json:"passed"`
	ExitCode    int           `json:"exit_code"`
	Stdout      string        `json:"stdout,omitempty"`
	Stderr      string        `json:"stderr,omitempty"`
	Duration    time.Duration `json:"duration"`
	Failures    []TestFailure `json:"failures,omitempty"`
	Coverage    float64       `json:"coverage,omitempty"`
}

// TestFailure details a single test failure
type TestFailure struct {
	TestName string `json:"test_name"`
	Message  string `json:"message"`
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
}

// ReviewResult holds code review output
type ReviewResult struct {
	TaskID      string        `json:"task_id"`
	Score       int           `json:"score"` // 0-100
	Issues      []ReviewIssue `json:"issues,omitempty"`
	Suggestions []string      `json:"suggestions,omitempty"`
	Approved    bool          `json:"approved"`
	Summary     string        `json:"summary"`
}

// ReviewIssue represents a single issue found during review
type ReviewIssue struct {
	Severity Severity `json:"severity"`
	File     string   `json:"file,omitempty"`
	Line     int      `json:"line,omitempty"`
	Message  string   `json:"message"`
	Category string   `json:"category"`
}

// Severity levels for review issues
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
	SeverityInfo     Severity = "info"
)

// WorkflowState tracks the overall workflow execution
type WorkflowState struct {
	ID        string    `json:"id"`
	Spec      FeatureSpec `json:"spec"`
	Tasks     []Task    `json:"tasks"`
	Status    Status    `json:"status"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
	Artifacts []CodeArtifact `json:"artifacts,omitempty"`
	Results   []TestResult   `json:"results,omitempty"`
	Reviews   []ReviewResult `json:"reviews,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// PRSummary is the final output for human review
type PRSummary struct {
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Branch      string         `json:"branch"`
	Files       []FileChange   `json:"files"`
	Tests       []TestResult   `json:"tests"`
	Review      ReviewResult   `json:"review"`
	Checklist   []string       `json:"checklist"`
}

// FileChange represents a changed file in a PR
type FileChange struct {
	Path      string `json:"path"`
	Operation string `json:"operation"` // add, modify, delete
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

// MemoryEntry represents a stored decision or learning
type MemoryEntry struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Content   string    `json:"content"`
	Context   string    `json:"context,omitempty"`
	Tags      []string  `json:"tags,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	Score     float64   `json:"score,omitempty"` // relevance score
}

// PromptMessage represents a message in an LLM conversation
type PromptMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// LLMResponse is the parsed response from an LLM
type LLMResponse struct {
	Content      string `json:"content"`
	FinishReason string `json:"finish_reason"`
	Usage        struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}
