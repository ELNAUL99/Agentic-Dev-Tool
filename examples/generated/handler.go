package restaurant

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// Handler provides HTTP handlers for restaurant endpoints
type Handler struct {
	service *Service
}

// NewHandler creates a restaurant HTTP handler
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers all restaurant routes on the given mux
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/restaurants", h.Search)
	mux.HandleFunc("GET /api/v1/restaurants/{id}", h.GetByID)
}

// Search handles restaurant search requests
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	filters, err := parseSearchFilters(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	results, total, err := h.service.Search(ctx, *filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "search failed")
		return
	}

	response := map[string]interface{}{
		"data":       results,
		"total":      total,
		"page":       filters.Page,
		"page_size":  filters.PageSize,
	}

	writeJSON(w, http.StatusOK, response)
}

// GetByID retrieves a single restaurant by ID
func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid restaurant id")
		return
	}

	restaurant, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "restaurant not found")
		return
	}

	writeJSON(w, http.StatusOK, restaurant)
}

func parseSearchFilters(r *http.Request) (*SearchFilters, error) {
	q := r.URL.Query()

	page, _ := strconv.Atoi(q.Get("page"))
	pageSize, _ := strconv.Atoi(q.Get("page_size"))
	if pageSize == 0 {
		pageSize = 20
	}

	minRating, _ := strconv.ParseFloat(q.Get("min_rating"), 64)
	lat, _ := strconv.ParseFloat(q.Get("lat"), 64)
	lng, _ := strconv.ParseFloat(q.Get("lng"), 64)
	radius, _ := strconv.ParseFloat(q.Get("radius_km"), 64)

	f := &SearchFilters{
		Query:     q.Get("q"),
		Cuisine:   q.Get("cuisine"),
		MinRating: minRating,
		Lat:       lat,
		Lng:       lng,
		RadiusKM:  radius,
		SortBy:    q.Get("sort_by"),
		Page:      page,
		PageSize:  pageSize,
	}

	if err := f.Validate(); err != nil {
		return nil, err
	}

	return f, nil
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
