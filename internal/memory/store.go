package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-agent/go-agent/internal/config"
	"github.com/go-agent/go-agent/internal/models"
)

// Store persists agent decisions and learnings
type Store struct {
	config *config.Config
	path   string
	items  []models.MemoryEntry
	loaded bool
}

// New creates a memory store
func New(cfg *config.Config) *Store {
	return &Store{
		config: cfg,
		path:   cfg.Memory.StorePath,
		items:  []models.MemoryEntry{},
	}
}

// Store saves a memory entry
func (s *Store) Store(ctx context.Context, entry models.MemoryEntry) error {
	if !s.config.Memory.Enabled {
		return nil
	}

	// Load persisted history before appending so a fresh Store instance does not
	// overwrite memories written by an earlier process.
	s.loadOnce()

	entry.ID = fmt.Sprintf("mem-%d", time.Now().UnixNano())
	entry.CreatedAt = time.Now()

	s.items = append(s.items, entry)

	// Trim if needed
	if len(s.items) > s.config.Memory.MaxItems {
		s.items = s.items[len(s.items)-s.config.Memory.MaxItems:]
	}

	return s.save()
}

// Retrieve finds relevant memories by tags or content similarity
func (s *Store) Retrieve(ctx context.Context, query string, tags []string) []models.MemoryEntry {
	if !s.config.Memory.Enabled {
		return nil
	}

	s.loadOnce()

	var results []models.MemoryEntry
	for _, item := range s.items {
		score := relevanceScore(item, query, tags)
		if score > 0.3 {
			item.Score = score
			results = append(results, item)
		}
	}

	// Sort by score descending (simple bubble sort for small sets)
	for i := range results {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	// Return top 10
	if len(results) > 10 {
		results = results[:10]
	}
	return results
}

// GetWorkflowHistory retrieves past workflows
func (s *Store) GetWorkflowHistory(ctx context.Context, limit int) []models.MemoryEntry {
	if !s.config.Memory.Enabled {
		return nil
	}

	s.loadOnce()

	var workflows []models.MemoryEntry
	for i := len(s.items) - 1; i >= 0; i-- {
		if s.items[i].Type == "workflow" {
			workflows = append(workflows, s.items[i])
			if len(workflows) >= limit {
				break
			}
		}
	}
	return workflows
}

func (s *Store) save() error {
	if s.path == "" {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s.items, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.path, data, 0644)
}

func (s *Store) loadOnce() {
	if s.loaded {
		return
	}
	s.loaded = true

	if s.path == "" {
		return
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		return
	}

	json.Unmarshal(data, &s.items)
}

func relevanceScore(item models.MemoryEntry, query string, tags []string) float64 {
	score := 0.0
	queryLower := strings.ToLower(query)

	// Content match
	if strings.Contains(strings.ToLower(item.Content), queryLower) {
		score += 0.5
	}
	if strings.Contains(strings.ToLower(item.Context), queryLower) {
		score += 0.3
	}

	// Tag match
	for _, t := range tags {
		for _, it := range item.Tags {
			if strings.EqualFold(t, it) {
				score += 0.2
			}
		}
	}

	// Type match for common queries
	if strings.Contains(queryLower, "workflow") && item.Type == "workflow" {
		score += 0.1
	}

	return score
}
