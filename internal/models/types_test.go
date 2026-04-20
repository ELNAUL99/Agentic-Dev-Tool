package models

import (
	"testing"
	"time"
)

func TestTaskStatus(t *testing.T) {
	task := Task{
		ID:     "task-1",
		Title:  "Test task",
		Type:   TaskTypeCode,
		Status: StatusPending,
	}

	if task.Status != StatusPending {
		t.Errorf("expected status pending, got %s", task.Status)
	}
}

func TestWorkflowState(t *testing.T) {
	state := WorkflowState{
		ID:        "wf-123",
		Status:    StatusRunning,
		StartedAt: time.Now(),
		Tasks: []Task{
			{ID: "t1", Type: TaskTypeCode, Status: StatusCompleted},
			{ID: "t2", Type: TaskTypeTest, Status: StatusPending},
		},
		Artifacts: []CodeArtifact{
			{FilePath: "main.go", Language: "go"},
		},
	}

	if len(state.Tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(state.Tasks))
	}
	if len(state.Artifacts) != 1 {
		t.Errorf("expected 1 artifact, got %d", len(state.Artifacts))
	}
}

func TestSeverityOrdering(t *testing.T) {
	severities := []Severity{SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow, SeverityInfo}
	for _, s := range severities {
		if s == "" {
			t.Error("severity should not be empty")
		}
	}
}

func TestMemoryEntry(t *testing.T) {
	entry := MemoryEntry{
		ID:      "mem-1",
		Type:    "workflow",
		Content: "Feature: Add search",
		Tags:    []string{"backend", "api"},
		Score:   0.85,
	}

	if entry.Score < 0 || entry.Score > 1 {
		t.Errorf("score should be between 0 and 1, got %f", entry.Score)
	}
}
