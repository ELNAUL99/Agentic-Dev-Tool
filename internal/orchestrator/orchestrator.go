package orchestrator

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-agent/go-agent/internal/agents/coder"
	"github.com/go-agent/go-agent/internal/agents/planner"
	"github.com/go-agent/go-agent/internal/agents/reviewer"
	"github.com/go-agent/go-agent/internal/codegen"
	"github.com/go-agent/go-agent/internal/config"
	"github.com/go-agent/go-agent/internal/git"
	"github.com/go-agent/go-agent/internal/memory"
	"github.com/go-agent/go-agent/internal/models"
	"github.com/go-agent/go-agent/internal/plugins"
	"github.com/go-agent/go-agent/internal/testrunner"
	"github.com/go-agent/go-agent/pkg/llm"
)

// ANSI colors (duplicated here to avoid import cycle)
const (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Cyan   = "\033[36m"
	White  = "\033[37m"
)

// Orchestrator manages the full agent workflow
type Orchestrator struct {
	config     *config.Config
	llm        llm.Client
	planner    *planner.Agent
	coder      *coder.Agent
	reviewer   *reviewer.Agent
	codegen    *codegen.Generator
	testRunner *testrunner.Runner
	git        *git.Integration
	memory     *memory.Store
	plugins    *plugins.Manager
}

// New creates a new workflow orchestrator
func New(cfg *config.Config) *Orchestrator {
	client := llm.NewHTTPClient(cfg.LLM)

	return &Orchestrator{
		config:     cfg,
		llm:        client,
		planner:    planner.New(client, cfg),
		coder:      coder.New(client, cfg),
		reviewer:   reviewer.New(client, cfg),
		codegen:    codegen.New(client, cfg),
		testRunner: testrunner.New(cfg),
		git:        git.New(cfg),
		memory:     memory.New(cfg),
		plugins:    plugins.New(cfg),
	}
}

// RunFeatureWorkflow executes the complete feature development workflow
func (o *Orchestrator) RunFeatureWorkflow(ctx context.Context, spec models.FeatureSpec, interactive bool) (*models.WorkflowState, error) {
	state := &models.WorkflowState{
		ID:        fmt.Sprintf("wf-%d", time.Now().Unix()),
		Spec:      spec,
		Status:    models.StatusRunning,
		StartedAt: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	if err := saveState(state); err != nil {
		return nil, err
	}

	// --- Phase 1: Planning ---
	fmt.Println(Cyan + "\n[Phase 1/5] Planning..." + Reset)
	tasks, err := o.planner.Plan(ctx, spec)
	if err != nil {
		state.Status = models.StatusFailed
		saveState(state)
		return nil, fmt.Errorf("planning failed: %w", err)
	}
	state.Tasks = tasks
	fmt.Printf("  Generated %d tasks\n", len(tasks))

	if interactive {
		fmt.Println("\nGenerated plan:")
		for i, t := range tasks {
			fmt.Printf("  %d. [%s] %s\n", i+1, t.Type, t.Title)
		}
		if !confirm("Approve this plan?") {
			state.Status = models.StatusRejected
			saveState(state)
			return state, fmt.Errorf("plan rejected by user")
		}
	}

	// --- Phase 2: Code Generation ---
	fmt.Println(Cyan + "\n[Phase 2/5] Generating code..." + Reset)
	for i := range state.Tasks {
		task := &state.Tasks[i]
		if task.Type != models.TaskTypeCode && task.Type != models.TaskTypeConfig {
			continue
		}

		task.Status = models.StatusRunning
		saveState(state)

		pluginCtx, err := o.plugins.ExecuteRelevant(ctx, task)
		if err != nil {
			fmt.Println(Yellow + "  Plugin warning: " + err.Error() + Reset)
		}

		artifacts, err := o.coder.Generate(ctx, *task, spec, pluginCtx)
		if err != nil {
			fmt.Println(Red + "  Task " + task.ID + " failed: " + err.Error() + Reset)
			task.Status = models.StatusFailed
			continue
		}

		for _, art := range artifacts {
			if err := writeArtifact(art); err != nil {
				fmt.Println(Red + "  Failed to write " + art.FilePath + ": " + err.Error() + Reset)
				continue
			}
			state.Artifacts = append(state.Artifacts, art)
			fmt.Println(Green + "  ✓ " + art.FilePath + Reset)
		}

		task.Status = models.StatusCompleted
		saveState(state)

		if interactive && i < len(state.Tasks)-1 {
			if !confirm(fmt.Sprintf("Continue to next task (%s)?", state.Tasks[i+1].Title)) {
				state.Status = models.StatusPending
				saveState(state)
				return state, nil
			}
		}
	}

	// --- Phase 3: Test Generation & Execution ---
	fmt.Println(Cyan + "\n[Phase 3/5] Generating and running tests..." + Reset)
	for i := range state.Tasks {
		task := &state.Tasks[i]
		if task.Type != models.TaskTypeCode {
			continue
		}

		testArtifacts, err := o.coder.GenerateTests(ctx, *task, state.Artifacts)
		if err != nil {
			fmt.Println(Yellow + "  Could not generate tests for " + task.ID + ": " + err.Error() + Reset)
			continue
		}

		for _, art := range testArtifacts {
			if err := writeArtifact(art); err != nil {
				fmt.Println(Red + "  Failed to write test " + art.FilePath + ": " + err.Error() + Reset)
				continue
			}
			state.Artifacts = append(state.Artifacts, art)
			fmt.Println(Green + "  ✓ Test: " + art.FilePath + Reset)
		}
	}

	results, err := o.testRunner.RunAll(ctx, state.Artifacts)
	if err != nil {
		fmt.Println(Yellow + "  Test runner error: " + err.Error() + Reset)
	}
	state.Results = results

	// Auto-fix loop
	fixAttempts := 0
	maxFixes := 3
	for _, r := range results {
		if r.Passed {
			continue
		}
		fixAttempts++
		if fixAttempts > maxFixes {
			fmt.Println(Yellow + "  Max fix attempts reached" + Reset)
			break
		}

		fmt.Println(Yellow + "  Auto-fixing failures in " + r.TaskID + "..." + Reset)
		fixed, err := o.coder.FixTests(ctx, r, state.Artifacts)
		if err != nil {
			fmt.Println(Red + "  Auto-fix failed: " + err.Error() + Reset)
			continue
		}

		for _, art := range fixed {
			if err := writeArtifact(art); err != nil {
				continue
			}
			newResults, _ := o.testRunner.RunAll(ctx, state.Artifacts)
			results = newResults
		}
	}
	state.Results = results

	// --- Phase 4: Code Review ---
	fmt.Println(Cyan + "\n[Phase 4/5] Code review..." + Reset)
	for _, task := range state.Tasks {
		if task.Type != models.TaskTypeCode {
			continue
		}

		review, err := o.reviewer.Review(ctx, &task, state.Artifacts)
		if err != nil {
			fmt.Println(Yellow + "  Review error: " + err.Error() + Reset)
			continue
		}
		state.Reviews = append(state.Reviews, *review)

		fmt.Printf("  Score: %d/100\n", review.Score)
		if !review.Approved {
			fmt.Printf("  Issues found: %d\n", len(review.Issues))
		}
	}

	if interactive {
		fmt.Println("\nReview complete. Options:")
		fmt.Println("  [a]pprove - approve and commit")
		fmt.Println("  [r]efine  - request improvements")
		fmt.Println("  [s]kip    - leave as-is")

		choice := prompt("Choice")
		switch choice {
		case "a", "approve":
			// Continue to Phase 5
		case "r", "refine":
			feedback := prompt("What needs improvement?")
			refined, err := o.coder.Refine(ctx, state.Tasks, state.Artifacts, feedback)
			if err != nil {
				return state, err
			}
			for _, art := range refined {
				writeArtifact(art)
				state.Artifacts = append(state.Artifacts, art)
			}
		default:
			fmt.Println("Skipping approval")
		}
	}

	// --- Phase 5: Git Integration & PR Summary ---
	fmt.Println(Cyan + "\n[Phase 5/5] Git integration & PR summary..." + Reset)

	if o.config.Git.Enabled {
		branch := o.config.Git.BranchPrefix + state.ID
		if err := o.git.CreateBranch(ctx, branch); err != nil {
			fmt.Println(Yellow + "  Git branch error: " + err.Error() + Reset)
		} else {
			fmt.Println(Green + "  Created branch: " + branch + Reset)
		}

		for _, art := range state.Artifacts {
			o.git.Add(ctx, art.FilePath)
		}

		msg := o.config.Git.CommitPrefix + spec.Title
		if err := o.git.Commit(ctx, msg); err != nil {
			fmt.Println(Yellow + "  Git commit error: " + err.Error() + Reset)
		} else {
			fmt.Println(Green + "  Committed: " + msg + Reset)
		}
	}

	summary, err := o.generatePRSummary(ctx, state)
	if err != nil {
		fmt.Println(Yellow + "  PR summary error: " + err.Error() + Reset)
	} else {
		fmt.Println(Green + "\n📋 PR Summary:\n" + summary + Reset)
	}

	if o.config.Memory.Enabled {
		o.memory.Store(ctx, models.MemoryEntry{
			Type:    "workflow",
			Content: fmt.Sprintf("Feature: %s\nTasks: %d\nArtifacts: %d", spec.Title, len(state.Tasks), len(state.Artifacts)),
			Context: state.ID,
			Tags:    spec.Tags,
		})
	}

	state.Status = models.StatusCompleted
	now := time.Now()
	state.EndedAt = &now
	saveState(state)

	return state, nil
}

// RunReview triggers a code review for existing workflow
func (o *Orchestrator) RunReview(ctx context.Context, workflowID string) (*models.ReviewResult, error) {
	state, err := LoadState(workflowID)
	if err != nil {
		return nil, err
	}

	var lastReview *models.ReviewResult
	for _, task := range state.Tasks {
		if task.Type != models.TaskTypeCode {
			continue
		}
		review, err := o.reviewer.Review(ctx, &task, state.Artifacts)
		if err != nil {
			continue
		}
		lastReview = review
	}

	if lastReview == nil {
		return nil, fmt.Errorf("no code tasks to review")
	}
	return lastReview, nil
}

// RunTests executes tests for a workflow
func (o *Orchestrator) RunTests(ctx context.Context, workflowID string) ([]models.TestResult, error) {
	state, err := LoadState(workflowID)
	if err != nil {
		return nil, err
	}
	return o.testRunner.RunAll(ctx, state.Artifacts)
}

// GenerateTests generates tests for existing workflow code
func (o *Orchestrator) GenerateTests(ctx context.Context, workflowID string) error {
	state, err := LoadState(workflowID)
	if err != nil {
		return err
	}

	for i := range state.Tasks {
		task := &state.Tasks[i]
		if task.Type != models.TaskTypeCode {
			continue
		}

		tests, err := o.coder.GenerateTests(ctx, *task, state.Artifacts)
		if err != nil {
			continue
		}

		for _, art := range tests {
			writeArtifact(art)
			state.Artifacts = append(state.Artifacts, art)
		}
	}

	return saveState(state)
}

// Approve finalizes and commits the workflow
func (o *Orchestrator) Approve(ctx context.Context, workflowID string) error {
	state, err := LoadState(workflowID)
	if err != nil {
		return err
	}

	state.Status = models.StatusApproved
	if err := saveState(state); err != nil {
		return err
	}

	if o.config.Git.Enabled && o.config.Git.AutoCommit {
		branch := o.config.Git.BranchPrefix + workflowID
		msg := o.config.Git.CommitPrefix + "Final: " + state.Spec.Title
		if err := o.git.Commit(ctx, msg); err != nil {
			return err
		}
		fmt.Println(Green + "Committed to branch: " + branch + Reset)
	}

	return nil
}

// Reject marks the workflow as rejected
func (o *Orchestrator) Reject(workflowID string) error {
	state, err := LoadState(workflowID)
	if err != nil {
		return err
	}
	state.Status = models.StatusRejected
	return saveState(state)
}

func (o *Orchestrator) generatePRSummary(ctx context.Context, state *models.WorkflowState) (string, error) {
	var changes []models.FileChange
	for _, a := range state.Artifacts {
		changes = append(changes, models.FileChange{
			Path:      a.FilePath,
			Operation: "add",
			Additions: len(a.Content),
		})
	}

	var totalScore int
	if len(state.Reviews) > 0 {
		for _, r := range state.Reviews {
			totalScore += r.Score
		}
		totalScore /= len(state.Reviews)
	}

	passed := 0
	for _, r := range state.Results {
		if r.Passed {
			passed++
		}
	}

	return fmt.Sprintf(`## %s

### Changes
%s

### Test Results
%d/%d tests passed

### Code Review
Score: %d/100

### Checklist
- [x] Code generated
- [x] Tests written
- [%s] Tests passing
- [x] Code reviewed
`,
		state.Spec.Title,
		formatFileChanges(changes),
		passed, len(state.Results),
		totalScore,
		func() string {
			if passed == len(state.Results) && len(state.Results) > 0 {
				return "x"
			}
			return " "
		}(),
	), nil
}

func formatFileChanges(changes []models.FileChange) string {
	var out string
	for _, c := range changes {
		out += fmt.Sprintf("- `%s` (%s, +%d)\n", c.Path, c.Operation, c.Additions)
	}
	return out
}

// --- persistence helpers ---

const stateDir = ".go-agent/workflows"

func statePath(id string) string {
	return filepath.Join(stateDir, id+".json")
}

func saveState(state *models.WorkflowState) error {
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(statePath(state.ID), data, 0644)
}

// LoadState retrieves a workflow state by ID
func LoadState(id string) (*models.WorkflowState, error) {
	data, err := os.ReadFile(statePath(id))
	if err != nil {
		return nil, fmt.Errorf("loading state %s: %w", id, err)
	}
	var state models.WorkflowState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parsing state: %w", err)
	}
	return &state, nil
}

func writeArtifact(art models.CodeArtifact) error {
	dir := filepath.Dir(art.FilePath)
	if dir != "." && dir != "/" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return os.WriteFile(art.FilePath, []byte(art.Content), 0644)
}

func confirm(prompt string) bool {
	fmt.Printf("%s [y/N]: ", prompt)
	reader := bufio.NewReader(os.Stdin)
	resp, _ := reader.ReadString('\n')
	resp = strings.TrimSpace(strings.ToLower(resp))
	return resp == "y" || resp == "yes"
}

func prompt(label string) string {
	fmt.Printf("%s: ", label)
	reader := bufio.NewReader(os.Stdin)
	resp, _ := reader.ReadString('\n')
	return strings.TrimSpace(resp)
}
