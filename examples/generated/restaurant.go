package restaurant

import (
	"context"
	"fmt"
	"strings"
)

// Restaurant represents a dining establishment
type Restaurant struct {
	ID       int64   `json:"id"`
	Name     string  `json:"name"`
	Cuisine  string  `json:"cuisine"`
	Lat      float64 `json:"lat"`
	Lng      float64 `json:"lng"`
	Rating   float64 `json:"rating"`
	Address  string  `json:"address"`
}

// SearchFilters defines search parameters
type SearchFilters struct {
	Query      string
	Cuisine    string
	MinRating  float64
	Lat        float64
	Lng        float64
	RadiusKM   float64
	SortBy     string // "distance", "rating", "name"
	Page       int
	PageSize   int
}

// Validate checks search filter constraints
func (f *SearchFilters) Validate() error {
	if f.PageSize <= 0 || f.PageSize > 100 {
		return fmt.Errorf("page_size must be between 1 and 100")
	}
	if f.Page < 0 {
		return fmt.Errorf("page must be non-negative")
	}
	if f.MinRating < 0 || f.MinRating > 5 {
		return fmt.Errorf("rating must be between 0 and 5")
	}
	return nil
}

// Repository defines data access operations
//go:generate mockgen -source=restaurant.go -destination=mocks/repository_mock.go -package=mocks

type Repository interface {
	Search(ctx context.Context, filters SearchFilters) ([]Restaurant, int, error)
	GetByID(ctx context.Context, id int64) (*Restaurant, error)
}

// Service provides business logic for restaurant operations
type Service struct {
	repo Repository
}

// NewService creates a restaurant service
func NewService(repo Repository) *Service {
	if repo == nil {
		panic("repository cannot be nil")
	}
	return &Service{repo: repo}
}

// Search finds restaurants matching the given filters
func (s *Service) Search(ctx context.Context, filters SearchFilters) ([]Restaurant, int, error) {
	if err := filters.Validate(); err != nil {
		return nil, 0, fmt.Errorf("invalid filters: %w", err)
	}

	results, total, err := s.repo.Search(ctx, filters)
	if err != nil {
		return nil, 0, fmt.Errorf("search failed: %w", err)
	}

	// Apply sorting if not handled by repository
	results = sortResults(results, filters.SortBy)

	return results, total, nil
}

// GetByID retrieves a restaurant by its ID
func (s *Service) GetByID(ctx context.Context, id int64) (*Restaurant, error) {
	restaurant, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get restaurant by id: %w", err)
	}
	return restaurant, nil
}
func sortResults(results []Restaurant, sortBy string) []Restaurant {
	switch strings.ToLower(sortBy) {
	case "rating":
		// Sort by rating descending
		for i := range results {
			for j := i + 1; j < len(results); j++ {
				if results[j].Rating > results[i].Rating {
					results[i], results[j] = results[j], results[i]
				}
			}
		}
	case "name":
		// Sort by name ascending
		for i := range results {
			for j := i + 1; j < len(results); j++ {
				if strings.ToLower(results[j].Name) < strings.ToLower(results[i].Name) {
					results[i], results[j] = results[j], results[i]
				}
			}
		}
	}
	return results
}
