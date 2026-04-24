package testrunner

import (
	"context"
	"testing"
	"time"

	"github.com/go-agent/go-agent/internal/config"
	"github.com/go-agent/go-agent/internal/models"
)

func TestParseFailures(t *testing.T) {
	output := `=== RUN   TestAdd
--- FAIL: TestAdd (0.00s)
    main_test.go:10: expected 4, got 5
=== RUN   TestSubtract
--- PASS: TestSubtract (0.00s)
FAIL
exit status 1`

	failures := parseFailures(output)
	if len(failures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(failures))
	}

	if failures[0].TestName != "TestAdd" {
		t.Errorf("expected test name 'TestAdd', got %s", failures[0].TestName)
	}
	if failures[0].Message != "main_test.go:10: expected 4, got 5" {
		t.Errorf("unexpected message: %s", failures[0].Message)
	}
}

func TestParseCoverage(t *testing.T) {
	output := "coverage: 85.3% of statements\n"
	pct := parseCoverage(output)
	if pct != 85.3 {
		t.Errorf("expected coverage 85.3, got %f", pct)
	}

	output2 := "no coverage data\n"
	pct2 := parseCoverage(output2)
	if pct2 != 0 {
		t.Errorf("expected coverage 0 for no data, got %f", pct2)
	}
}

func TestRunnerLocal(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Docker.Enabled = false

	runner := New(cfg)
	artifacts := []models.CodeArtifact{
		{FilePath: "main.go", Language: "go", Content: "package main\n"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	results, err := runner.RunAll(ctx, artifacts)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
}
