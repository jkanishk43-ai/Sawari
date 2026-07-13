package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sawaari/backend/internal/comparison"
	"github.com/sawaari/backend/internal/models"
)

// Handler contains HTTP handlers
type Handler struct {
	orchestrator *comparison.Orchestrator
}

// New creates a new handler
func New(orch *comparison.Orchestrator) *Handler {
	return &Handler{orchestrator: orch}
}

// ComparePrices handles GET/POST /api/compare
func (h *Handler) ComparePrices(w http.ResponseWriter, r *http.Request) {
	var req models.CompareRequest

	if r.Method == http.MethodPost {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
	} else {
		// GET with query params
		req.From = r.URL.Query().Get("from")
		req.To = r.URL.Query().Get("to")
	}

	if req.From == "" || req.To == "" {
		http.Error(w, "missing from or to parameter", http.StatusBadRequest)
		return
	}

	resp, err := h.orchestrator.Compare(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// Health handles GET /health
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
	})
}

// SetupRoutes configures all routes
func SetupRoutes(orch *comparison.Orchestrator) http.Handler {
	h := New(orch)
	r := chi.NewRouter()

	r.Get("/health", h.Health)
	r.Route("/api", func(r chi.Router) {
		r.Post("/compare", h.ComparePrices)
		r.Get("/compare", h.ComparePrices)
	})

	return r
}
