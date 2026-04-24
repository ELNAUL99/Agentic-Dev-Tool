package git

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/go-agent/go-agent/internal/config"
)

// Integration handles Git operations
type Integration struct {
	config *config.Config
}

// New creates a git integration
func New(cfg *config.Config) *Integration {
	return &Integration{config: cfg}
}

// IsRepo checks if current directory is a git repository
func (g *Integration) IsRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	err := cmd.Run()
	return err == nil
}

// Status returns current repository status
func (g *Integration) Status(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "status", "--short")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git status: %w", err)
	}
	return string(out), nil
}

// CurrentBranch returns the current branch name
func (g *Integration) CurrentBranch(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git branch: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// CreateBranch creates and checks out a new branch
func (g *Integration) CreateBranch(ctx context.Context, name string) error {
	base, err := g.CurrentBranch(ctx)
	if err != nil {
		base = "main"
	}

	// Pull latest from base
	cmd := exec.CommandContext(ctx, "git", "pull", g.config.Git.RemoteName, base)
	cmd.CombinedOutput()

	// Create branch
	cmd = exec.CommandContext(ctx, "git", "checkout", "-b", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git checkout -b %s: %w\n%s", name, err, string(out))
	}
	return nil
}

// Add stages files
func (g *Integration) Add(ctx context.Context, paths ...string) error {
	args := append([]string{"add"}, paths...)
	cmd := exec.CommandContext(ctx, "git", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git add: %w\n%s", err, string(out))
	}
	return nil
}

// Commit creates a commit
func (g *Integration) Commit(ctx context.Context, message string) error {
	cmd := exec.CommandContext(ctx, "git", "commit", "-m", message)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Check if nothing to commit
		if strings.Contains(string(out), "nothing to commit") {
			return nil
		}
		return fmt.Errorf("git commit: %w\n%s", err, string(out))
	}
	return nil
}

// Push pushes current branch to remote
func (g *Integration) Push(ctx context.Context) error {
	branch, err := g.CurrentBranch(ctx)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "git", "push", "-u", g.config.Git.RemoteName, branch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git push: %w\n%s", err, string(out))
	}
	return nil
}

// Diff shows diff of staged changes
func (g *Integration) Diff(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", "--cached")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git diff: %w", err)
	}
	return string(out), nil
}

// Log returns recent commit history
func (g *Integration) Log(ctx context.Context, n int) ([]Commit, error) {
	cmd := exec.CommandContext(ctx, "git", "log", fmt.Sprintf("-%d", n), "--oneline", "--format=%H|%s|%an|%at")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}

	var commits []Commit
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			continue
		}

		var ts time.Time
		var sec int64
		if _, err := fmt.Sscanf(parts[3], "%d", &sec); err == nil {
			ts = time.Unix(sec, 0)
		}

		commits = append(commits, Commit{
			Hash:    parts[0],
			Message: parts[1],
			Author:  parts[2],
			Time:    ts,
		})
	}

	return commits, nil
}

// Commit represents a git commit
type Commit struct {
	Hash    string
	Message string
	Author  string
	Time    time.Time
}
