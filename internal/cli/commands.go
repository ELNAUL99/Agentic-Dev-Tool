package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-agent/go-agent/internal/config"
	"github.com/go-agent/go-agent/internal/models"
	"github.com/go-agent/go-agent/internal/orchestrator"
)

// ANSI color codes
const (
	Reset   = "\033[0m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	White   = "\033[37m"
	Bold    = "\033[1m"
)

var cfgFile string
var cfg *config.Config

// Run parses CLI args and executes commands
func Run() error {
	if len(os.Args) < 2 {
		printUsage()
		return nil
	}

	// Parse global flags before command
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		if args[i] == "--config" || args[i] == "-c" {
			if i+1 < len(args) {
				cfgFile = args[i+1]
				args = append(args[:i], args[i+2:]...)
				break
			}
		}
	}

	if err := loadConfig(); err != nil {
		return err
	}

	if len(args) == 0 {
		printUsage()
		return nil
	}

	cmd := args[0]
	cmdArgs := args[1:]

	switch cmd {
	case "init":
		return runInit()
	case "create-feature":
		return runCreateFeature(cmdArgs)
	case "review":
		return runReview(cmdArgs)
	case "test":
		return runTest(cmdArgs)
	case "generate-tests":
		return runGenerateTests(cmdArgs)
	case "status":
		return showStatus(cmdArgs)
	case "approve":
		return runApprove(cmdArgs)
	case "reject":
		return runReject(cmdArgs)
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command: %s", cmd)
	}
}

func loadConfig() error {
	path := cfgFile
	if path == "" {
		path = ".go-agent/config.json"
	}

	var err error
	cfg, err = config.LoadConfig(path)
	if err != nil {
		cfg = config.DefaultConfig()
	}

	// Allow env overrides
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		cfg.LLM.APIKey = apiKey
	}
	if baseURL := os.Getenv("OPENAI_BASE_URL"); baseURL != "" {
		cfg.LLM.BaseURL = baseURL
	}
	if model := os.Getenv("OPENAI_MODEL"); model != "" {
		cfg.LLM.Model = model
	}

	return nil
}

func printUsage() {
	fmt.Println(Bold + Cyan + "go-agent" + Reset + " — AI-assisted backend code agent")
	fmt.Println()
	fmt.Println("Usage: go-agent [global flags] <command> [args...]")
	fmt.Println()
	fmt.Println("Global Flags:")
	fmt.Println("  --config, -c PATH    Config file path (default: .go-agent/config.json)")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  init                           Initialize configuration")
	fmt.Println("  create-feature [TITLE] [flags] Run full feature workflow")
	fmt.Println("    Flags:")
	fmt.Println("      -i, --interactive    Interactive mode (default: true)")
	fmt.Println("      --context FILES      Comma-separated context files")
	fmt.Println("      -t, --tags TAGS      Comma-separated tags")
	fmt.Println("  review WORKFLOW_ID             Review generated code")
	fmt.Println("  test WORKFLOW_ID               Run tests for workflow")
	fmt.Println("  generate-tests WORKFLOW_ID     Generate tests")
	fmt.Println("  status WORKFLOW_ID             Show workflow status")
	fmt.Println("  approve WORKFLOW_ID            Approve and commit")
	fmt.Println("  reject WORKFLOW_ID             Reject workflow")
	fmt.Println("  help                           Show this help")
}

func runInit() error {
	dir := ".go-agent"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	configPath := filepath.Join(dir, "config.json")
	if _, err := os.Stat(configPath); err == nil {
		fmt.Println(Yellow + "Config already exists at " + configPath + Reset)
		return nil
	}

	defaultCfg := config.DefaultConfig()
	data, _ := json.MarshalIndent(defaultCfg, "", "  ")
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return err
	}

	fmt.Println(Green + "Initialized go-agent at " + dir + "/" + Reset)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Set OPENAI_API_KEY environment variable")
	fmt.Println("  2. Customize " + configPath + " if needed")
	fmt.Println("  3. Run: go-agent create-feature \"Your feature description\"")
	return nil
}

func runCreateFeature(args []string) error {
	fs := flag.NewFlagSet("create-feature", flag.ContinueOnError)
	interactive := fs.Bool("interactive", true, "Interactive mode")
	interactiveShort := fs.Bool("i", true, "Interactive mode (short)")
	contextFiles := fs.String("context", "", "Comma-separated context files")
	tagsStr := fs.String("tags", "", "Comma-separated tags")
	tagsShort := fs.String("t", "", "Comma-separated tags (short)")
	fs.Parse(args)

	// Reconcile short and long flags
	if !*interactiveShort {
		*interactive = false
	}
	if *tagsShort != "" {
		*tagsStr = *tagsShort
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		return fmt.Errorf("feature title required")
	}

	title := strings.Join(remaining, " ")

	spec := models.FeatureSpec{
		Title:       title,
		Description: title,
	}

	if *tagsStr != "" {
		spec.Tags = strings.Split(*tagsStr, ",")
		for i := range spec.Tags {
			spec.Tags[i] = strings.TrimSpace(spec.Tags[i])
		}
	}

	if *contextFiles != "" {
		files := strings.Split(*contextFiles, ",")
		var parts []string
		for _, f := range files {
			f = strings.TrimSpace(f)
			data, err := os.ReadFile(f)
			if err != nil {
				fmt.Println(Yellow + "Warning: could not read " + f + ": " + err.Error() + Reset)
				continue
			}
			parts = append(parts, fmt.Sprintf("--- %s ---\n%s", f, string(data)))
		}
		spec.Context = strings.Join(parts, "\n\n")
	}

	fmt.Println(Cyan + "🚀 Starting feature workflow: " + title + Reset)
	fmt.Println("Interactive mode:", *interactive)

	orch := orchestrator.New(cfg)
	state, err := orch.RunFeatureWorkflow(context.Background(), spec, *interactive)
	if err != nil {
		fmt.Println(Red+"Workflow failed: "+err.Error()+Reset)
		return err
	}

	printWorkflowSummary(state)
	return nil
}

func runReview(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("workflow ID required")
	}
	fmt.Println(Cyan + "🔍 Running code review for workflow " + args[0] + "..." + Reset)
	orch := orchestrator.New(cfg)
	review, err := orch.RunReview(context.Background(), args[0])
	if err != nil {
		return err
	}

	fmt.Printf("\n📊 Review Score: %d/100\n", review.Score)
	if review.Approved {
		fmt.Println(Green + "✅ Code approved" + Reset)
	} else {
		fmt.Println(Yellow + "⚠️  Code needs improvements" + Reset)
	}

	if len(review.Issues) > 0 {
		fmt.Println("\n📝 Issues found:")
		for _, issue := range review.Issues {
			color := Yellow
			switch issue.Severity {
			case models.SeverityCritical, models.SeverityHigh:
				color = Red
			}
			fmt.Printf("  %s[%s] %s (line %d): %s%s\n", color, issue.Severity, issue.File, issue.Line, issue.Message, Reset)
		}
	}

	if len(review.Suggestions) > 0 {
		fmt.Println("\n💡 Suggestions:")
		for _, s := range review.Suggestions {
			fmt.Printf("  %s• %s%s\n", Cyan, s, Reset)
		}
	}

	return nil
}

func runTest(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("workflow ID required")
	}
	fmt.Println(Cyan + "🧪 Running tests for workflow " + args[0] + "..." + Reset)
	orch := orchestrator.New(cfg)
	results, err := orch.RunTests(context.Background(), args[0])
	if err != nil {
		return err
	}

	for _, r := range results {
		if r.Passed {
			fmt.Printf("  %s✅ %s - PASSED (%.2fs)%s\n", Green, r.TaskID, r.Duration.Seconds(), Reset)
		} else {
			fmt.Printf("  %s❌ %s - FAILED (%.2fs)%s\n", Red, r.TaskID, r.Duration.Seconds(), Reset)
			if r.Stderr != "" {
				fmt.Printf("     %s%s%s\n", Red, r.Stderr, Reset)
			}
		}
	}
	return nil
}

func runGenerateTests(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("workflow ID required")
	}
	fmt.Println(Cyan + "🧪 Generating tests for workflow " + args[0] + "..." + Reset)
	orch := orchestrator.New(cfg)
	return orch.GenerateTests(context.Background(), args[0])
}

func showStatus(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("workflow ID required")
	}
	state, err := orchestrator.LoadState(args[0])
	if err != nil {
		return err
	}

	fmt.Println(Cyan + "📋 Workflow: " + state.Spec.Title + Reset)
	fmt.Println("Status:", state.Status)
	fmt.Println("Started:", state.StartedAt.Format("2006-01-02 15:04:05"))
	if state.EndedAt != nil {
		fmt.Println("Ended:", state.EndedAt.Format("2006-01-02 15:04:05"))
	}

	fmt.Printf("\nTasks (%d):\n", len(state.Tasks))
	for _, t := range state.Tasks {
		color := White
		switch t.Status {
		case models.StatusCompleted:
			color = Green
		case models.StatusFailed:
			color = Red
		case models.StatusRunning:
			color = Yellow
		}
		fmt.Printf("  %s[%s] %s: %s%s\n", color, t.Status, t.Type, t.Title, Reset)
	}

	if len(state.Artifacts) > 0 {
		fmt.Printf("\nArtifacts (%d):\n", len(state.Artifacts))
		for _, a := range state.Artifacts {
			fmt.Printf("  📄 %s (%s)\n", a.FilePath, a.Language)
		}
	}

	return nil
}

func runApprove(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("workflow ID required")
	}
	fmt.Println(Cyan + "✅ Approving workflow " + args[0] + "..." + Reset)
	orch := orchestrator.New(cfg)
	return orch.Approve(context.Background(), args[0])
}

func runReject(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("workflow ID required")
	}
	fmt.Println(Yellow + "❌ Rejecting workflow " + args[0] + "..." + Reset)
	orch := orchestrator.New(cfg)
	return orch.Reject(args[0])
}

func printWorkflowSummary(state *models.WorkflowState) {
	fmt.Println(Green + "\n✅ Workflow completed: " + state.ID + Reset)
	fmt.Println("\n📊 Summary:")
	fmt.Printf("  Tasks: %d total\n", len(state.Tasks))

	completed := 0
	for _, t := range state.Tasks {
		if t.Status == models.StatusCompleted || t.Status == models.StatusApproved {
			completed++
		}
	}
	fmt.Printf("  Completed: %d/%d\n", completed, len(state.Tasks))
	fmt.Printf("  Artifacts: %d\n", len(state.Artifacts))
	fmt.Printf("  Test runs: %d\n", len(state.Results))

	if len(state.Artifacts) > 0 {
		fmt.Println("\n📁 Generated files:")
		for _, a := range state.Artifacts {
			fmt.Println(Cyan + "  • " + a.FilePath + Reset)
		}
	}
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
