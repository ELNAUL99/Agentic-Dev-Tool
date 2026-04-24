package memory

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-agent/go-agent/internal/config"
	"github.com/go-agent/go-agent/internal/models"
)

func TestStoreAndRetrieve(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "memory.db")

	cfg := config.DefaultConfig()
	cfg.Memory.Enabled = true
	cfg.Memory.StorePath = storePath
	cfg.Memory.MaxItems = 100

	store := New(cfg)
	ctx := context.Background()

	entry := models.MemoryEntry{
		Type:    "workflow",
		Content: "Feature: Add search",
		Context: "wf-123",
		Tags:    []string{"backend", "api"},
	}

	if err := store.Store(ctx, entry); err != nil {
		t.Fatalf("store failed: %v", err)
	}

	results := store.Retrieve(ctx, "search", []string{"backend"})
	if len(results) == 0 {
		t.Error("expected to retrieve stored entry")
	}

	if len(results) > 0 && results[0].Content != "Feature: Add search" {
		t.Errorf("expected content 'Feature: Add search', got %s", results[0].Content)
	}
}

func TestRelevanceScore(t *testing.T) {
	item := models.MemoryEntry{
		Content: "Generate user authentication middleware",
		Context: "project-x",
		Tags:    []string{"auth", "middleware"},
	}

	score := relevanceScore(item, "authentication middleware", []string{"auth"})
	if score < 0.5 {
		t.Errorf("expected high score for matching query, got %f", score)
	}

	score = relevanceScore(item, "database", []string{"db"})
	if score > 0.3 {
		t.Errorf("expected low score for unrelated query, got %f", score)
	}
}

func TestTrimOnMaxItems(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "memory.db")

	cfg := config.DefaultConfig()
	cfg.Memory.Enabled = true
	cfg.Memory.StorePath = storePath
	cfg.Memory.MaxItems = 3

	store := New(cfg)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		store.Store(ctx, models.MemoryEntry{Type: "test", Content: "entry"})
	}

	if len(store.items) > 3 {
		t.Errorf("expected items trimmed to 3, got %d", len(store.items))
	}
}

func TestDisabledMemory(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Memory.Enabled = false

	store := New(cfg)
	ctx := context.Background()

	entry := models.MemoryEntry{Type: "workflow", Content: "test"}
	err := store.Store(ctx, entry)
	if err != nil {
		t.Errorf("disabled store should not error: %v", err)
	}

	results := store.Retrieve(ctx, "test", nil)
	if len(results) != 0 {
		t.Error("disabled store should return empty results")
	}
}

func TestPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "memory.db")

	cfg := config.DefaultConfig()
	cfg.Memory.Enabled = true
	cfg.Memory.StorePath = storePath

	store1 := New(cfg)
	ctx := context.Background()
	store1.Store(ctx, models.MemoryEntry{Type: "workflow", Content: "data"})

	// Verify file exists
	if _, err := os.Stat(storePath); os.IsNotExist(err) {
		t.Fatal("memory file should exist after store")
	}

	// Create new store instance pointing to same file
	store2 := New(cfg)
	results := store2.Retrieve(ctx, "data", nil)
	if len(results) == 0 {
		t.Error("expected to load persisted data")
	}
}

func TestStorePreservesExistingFileEntries(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "memory.db")

	cfg := config.DefaultConfig()
	cfg.Memory.Enabled = true
	cfg.Memory.StorePath = storePath

	ctx := context.Background()
	store1 := New(cfg)
	if err := store1.Store(ctx, models.MemoryEntry{Type: "workflow", Content: "first"}); err != nil {
		t.Fatalf("first store failed: %v", err)
	}

	store2 := New(cfg)
	if err := store2.Store(ctx, models.MemoryEntry{Type: "workflow", Content: "second"}); err != nil {
		t.Fatalf("second store failed: %v", err)
	}

	store3 := New(cfg)
	history := store3.GetWorkflowHistory(ctx, 10)
	if len(history) != 2 {
		t.Fatalf("expected both persisted entries, got %d", len(history))
	}
}
