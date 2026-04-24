package testrunner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-agent/go-agent/internal/config"
	"github.com/go-agent/go-agent/internal/models"
)

// Runner executes tests in Docker or locally
type Runner struct {
	config *config.Config
}

// New creates a test runner
func New(cfg *config.Config) *Runner {
	return &Runner{config: cfg}
}

// RunAll runs all tests for given artifacts
func (r *Runner) RunAll(ctx context.Context, artifacts []models.CodeArtifact) ([]models.TestResult, error) {
	if r.config.Docker.Enabled {
		return r.runInDocker(ctx, artifacts)
	}
	return r.runLocal(ctx, artifacts)
}

// runLocal executes tests locally with `go test`
func (r *Runner) runLocal(ctx context.Context, artifacts []models.CodeArtifact) ([]models.TestResult, error) {
	// Group by directory
	dirs := make(map[string]bool)
	for _, art := range artifacts {
		if art.Language == "go" {
			dirs[filepath.Dir(art.FilePath)] = true
		}
	}

	var results []models.TestResult
	for dir := range dirs {
		if dir == "." {
			dir = ""
		}

		result := r.runGoTest(ctx, dir)
		results = append(results, result)
	}

	return results, nil
}

// runGoTest executes go test in a directory
func (r *Runner) runGoTest(ctx context.Context, dir string) models.TestResult {
	taskID := dir
	if taskID == "" {
		taskID = "root"
	}

	result := models.TestResult{
		TaskID: taskID,
		Passed: false,
	}

	// Check if there are test files
	pattern := filepath.Join(dir, "*_test.go")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		result.Passed = true // No tests to run = pass
		result.Stdout = "No tests found"
		return result
	}

	start := time.Now()
	cmd := exec.CommandContext(ctx, "go", "test", "-v", "-cover", "./...")
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	result.Duration = time.Since(start)
	result.Stdout = string(output)

	if err != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		}
		result.Failures = parseFailures(string(output))
	} else {
		result.Passed = true
		result.ExitCode = 0
		// Try to extract coverage
		result.Coverage = parseCoverage(string(output))
	}

	return result
}

// runInDocker executes tests in a Docker container
func (r *Runner) runInDocker(ctx context.Context, artifacts []models.CodeArtifact) ([]models.TestResult, error) {
	// Create temp dir with project files
	tmpDir, err := os.MkdirTemp("", "go-agent-test-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write all artifacts
	for _, art := range artifacts {
		path := filepath.Join(tmpDir, art.FilePath)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, []byte(art.Content), 0644); err != nil {
			return nil, err
		}
	}

	// Ensure go.mod exists
	goModPath := filepath.Join(tmpDir, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		goMod := `module testproject

go 1.23
`
		if err := os.WriteFile(goModPath, []byte(goMod), 0644); err != nil {
			return nil, err
		}
	}

	// Run docker
	image := r.config.Docker.Image
	timeout := r.config.Docker.Timeout
	if timeout == 0 {
		timeout = 300
	}

	args := []string{
		"run", "--rm",
		"-v", tmpDir + ":/workspace",
		"-w", "/workspace",
		"--network", r.config.Docker.NetworkMode,
		"--memory", r.config.Docker.MemoryLimit,
		"--cpus", r.config.Docker.CPULimit,
	}

	for _, vol := range r.config.Docker.Volumes {
		args = append(args, "-v", vol)
	}
	for _, env := range r.config.Docker.EnvVars {
		args = append(args, "-e", env)
	}

	// Add timeout
	dockerCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	args = append(args, image, "sh", "-c", "go mod tidy && go test -v -cover ./...")

	cmd := exec.CommandContext(dockerCtx, "docker", args...)
	output, err := cmd.CombinedOutput()

	result := models.TestResult{
		TaskID:   "docker-test",
		Passed:   err == nil,
		Stdout:   string(output),
		Duration: time.Duration(timeout) * time.Second,
	}

	if err != nil {
		result.ExitCode = 1
		result.Failures = parseFailures(string(output))
		if dockerCtx.Err() == context.DeadlineExceeded {
			result.Stderr = "Docker execution timed out"
		}
	} else {
		result.Coverage = parseCoverage(string(output))
	}

	return []models.TestResult{result}, nil
}

func parseFailures(output string) []models.TestFailure {
	var failures []models.TestFailure
	lines := strings.Split(output, "\n")

	for i, line := range lines {
		if strings.Contains(line, "FAIL:") || strings.Contains(line, "Error:") || strings.Contains(line, "panic:") {
			f := models.TestFailure{
				Message: line,
			}
			// Try to extract actual error from next line if available
			if i+1 < len(lines) {
				nextLine := strings.TrimSpace(lines[i+1])
				if strings.Contains(nextLine, ":") && !strings.HasPrefix(nextLine, "===") && !strings.HasPrefix(nextLine, "---") {
					f.Message = nextLine
				}
			}
			// Try to find test name from previous lines
			for j := i - 1; j >= 0 && j >= i-5; j-- {
				if strings.HasPrefix(lines[j], "=== RUN") {
					f.TestName = strings.TrimSpace(strings.TrimPrefix(lines[j], "=== RUN"))
					break
				}
				if strings.HasPrefix(lines[j], "--- FAIL:") {
					parts := strings.Split(lines[j], " ")
					if len(parts) >= 3 {
						f.TestName = parts[2]
					}
					break
				}
			}
			failures = append(failures, f)
		}
	}

	return failures
}

func parseCoverage(output string) float64 {
	// Parse "coverage: X.X% of statements"
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "coverage:") {
			var pct float64
			fmt.Sscanf(line, "coverage: %f%%", &pct)
			return pct
		}
	}
	return 0
}
