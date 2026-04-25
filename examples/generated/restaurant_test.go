package restaurant

import (
	"context"
	"errors"
	"testing"
)

// mockRepository is a test double for Repository
type mockRepository struct {
	searchFunc func(ctx context.Context, filters SearchFilters) ([]Restaurant, int, error)
	getFunc    func(ctx context.Context, id int64) (*Restaurant, error)
}

func (m *mockRepository) Search(ctx context.Context, filters SearchFilters) ([]Restaurant, int, error) {
	if m.searchFunc != nil {
		return m.searchFunc(ctx, filters)
	}
	return nil, 0, nil
}

func (m *mockRepository) GetByID(ctx context.Context, id int64) (*Restaurant, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, id)
	}
	return nil, nil
}

func TestSearchFiltersValidate(t *testing.T) {
	tests := []struct {
		name    string
		filters SearchFilters
		wantErr bool
	}{
		{
			name:    "valid defaults",
			filters: SearchFilters{Page: 0, PageSize: 10},
		},
		{
			name:    "page size too large",
			filters: SearchFilters{PageSize: 200},
			wantErr: true,
		},
		{
			name:    "negative page",
			filters: SearchFilters{Page: -1},
			wantErr: true,
		},
		{
			name:    "rating too high",
			filters: SearchFilters{PageSize: 10, MinRating: 6},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.filters.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestServiceSearch(t *testing.T) {
	mock := &mockRepository{
		searchFunc: func(ctx context.Context, filters SearchFilters) ([]Restaurant, int, error) {
			return []Restaurant{
				{ID: 1, Name: "Sushi Place", Cuisine: "Japanese", Rating: 4.5},
				{ID: 2, Name: "Burger Joint", Cuisine: "American", Rating: 4.0},
			}, 2, nil
		},
	}

	svc := NewService(mock)
	ctx := context.Background()

	results, total, err := svc.Search(ctx, SearchFilters{Page: 0, PageSize: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 2 {
		t.Errorf("expected total 2, got %d", total)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestServiceSearchValidationError(t *testing.T) {
	mock := &mockRepository{}
	svc := NewService(mock)
	ctx := context.Background()

	_, _, err := svc.Search(ctx, SearchFilters{PageSize: 200})
	if err == nil {
		t.Fatal("expected error for invalid page size")
	}
}

func TestServiceSearchRepositoryError(t *testing.T) {
	mock := &mockRepository{
		searchFunc: func(ctx context.Context, filters SearchFilters) ([]Restaurant, int, error) {
			return nil, 0, errors.New("db connection failed")
		},
	}

	svc := NewService(mock)
	ctx := context.Background()

	_, _, err := svc.Search(ctx, SearchFilters{PageSize: 10})
	if err == nil {
		t.Fatal("expected error from repository")
	}
}

func TestSortResults(t *testing.T) {
	results := []Restaurant{
		{ID: 1, Name: "Bistro", Rating: 3.5},
		{ID: 2, Name: "Cafe", Rating: 4.5},
		{ID: 3, Name: "Diner", Rating: 4.0},
	}

	t.Run("by rating", func(t *testing.T) {
		sorted := sortResults(results, "rating")
		if sorted[0].ID != 2 {
			t.Errorf("expected highest rated first, got ID %d", sorted[0].ID)
		}
	})

	t.Run("by name", func(t *testing.T) {
		sorted := sortResults(results, "name")
		if sorted[0].Name != "Bistro" {
			t.Errorf("expected alphabetical first, got %s", sorted[0].Name)
		}
	})
}

func TestNewServiceNilRepo(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil repository")
		}
	}()
	NewService(nil)
}

func BenchmarkSortResults(b *testing.B) {
	results := []Restaurant{
		{ID: 1, Name: "A", Rating: 3.0},
		{ID: 2, Name: "B", Rating: 5.0},
		{ID: 3, Name: "C", Rating: 4.0},
	}

	for i := 0; i < b.N; i++ {
		sortResults(results, "rating")
	}
}
