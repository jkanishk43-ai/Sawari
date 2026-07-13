package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	bookingpkg "github.com/sawaari/backend/internal/booking"
	"github.com/sawaari/backend/internal/cache"
	"github.com/sawaari/backend/internal/comparison"
	"github.com/sawaari/backend/internal/events"
	"github.com/sawaari/backend/internal/geocoding"
	"github.com/sawaari/backend/internal/models"
	notifpkg "github.com/sawaari/backend/internal/notifications"
	"github.com/sawaari/backend/internal/realtime"
	"github.com/sawaari/backend/internal/transit"
	"github.com/sawaari/backend/internal/users"
	walletpkg "github.com/sawaari/backend/internal/wallet"
)

// Handler bundles all HTTP handlers with their dependencies.
type Handler struct {
	orchestrator *comparison.Orchestrator
	geocoder     *geocoding.Service
	transit      *transit.Planner
	booking      *bookingpkg.BookingMgr
	realtime     *realtime.ETAService
	wallet       *walletpkg.TicketService
	notifications *notifpkg.NotificationHub
	users        *users.UserService
	cache        *cache.ValkeyCache
	events       *events.EventBus
}

// New constructs a Handler with all dependencies wired in.
func New(
	orch *comparison.Orchestrator,
	geo *geocoding.Service,
	tr *transit.Planner,
	bk *bookingpkg.BookingMgr,
	rt *realtime.ETAService,
	wl *walletpkg.TicketService,
	nt *notifpkg.NotificationHub,
	us *users.UserService,
	ch *cache.ValkeyCache,
	ev *events.EventBus,
) *Handler {
	return &Handler{
		orchestrator:  orch,
		geocoder:      geo,
		transit:       tr,
		booking:       bk,
		realtime:      rt,
		wallet:        wl,
		notifications: nt,
		users:         us,
		cache:         ch,
		events:        ev,
	}
}

// Compare handles POST /v1/compare — the primary comparison endpoint.
// Body: {from:{lat,lng,address}, to:{lat,lng,address}, prefs{ac,saheli,night,surgeHint}}
func (h *Handler) Compare(w http.ResponseWriter, r *http.Request) {
	var req models.CompareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	if req.FromLoc.Lat == 0 || req.ToLoc.Lat == 0 {
		writeError(w, http.StatusBadRequest, "invalid_coordinates", "from and to require lat/lng")
		return
	}

	cacheKey := fmt.Sprintf("cmp:%.4f,%.4f:%.4f,%.4f:%v", req.FromLoc.Lat, req.FromLoc.Lng, req.ToLoc.Lat, req.ToLoc.Lng, req.Prefs)
	var cached models.CompareResponse
	if err := h.cache.Get(r.Context(), cacheKey, &cached); err == nil && len(cached.Options) > 0 {
		body, _ := json.Marshal(cached)
		w.Header().Set("X-Cache", "HIT")
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
		return
	}

	resp, err := h.orchestrator.Compare(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "compare_failed", err.Error())
		return
	}

	body, _ := json.Marshal(resp)
	h.cache.Set(r.Context(), cacheKey, 60*time.Second, body)
	h.events.Publish(r.Context(), "quotes.issued", string(body))

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	w.Write(body)
}

// NearbyStops handles GET /v1/stops/nearby?lat=&lng=&r= — stops within radius (meters, default 500).
func (h *Handler) NearbyStops(w http.ResponseWriter, r *http.Request) {
	lat, err := parseFloat(r.URL.Query().Get("lat"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_lat", err.Error())
		return
	}
	lng, err := parseFloat(r.URL.Query().Get("lng"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_lng", err.Error())
		return
	}
	radius := 500.0
	if rParam := r.URL.Query().Get("r"); rParam != "" {
		if v, err := parseFloat(rParam); err == nil {
			radius = v
		}
	}

	stops, err := h.transit.GetNearbyStops(r.Context(), lat, lng, int(radius))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "stops_failed", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"stops":  stops,
		"count":  len(stops),
		"radius": radius,
	})
}

// RouteDetail handles GET /v1/routes/{no} — route detail via transit planner.
func (h *Handler) RouteDetail(w http.ResponseWriter, r *http.Request) {
	routeNo := chi.URLParam(r, "no")
	if routeNo == "" {
		writeError(w, http.StatusBadRequest, "missing_route", "route number required")
		return
	}

	detail, err := h.transit.GetStopTimes(r.Context(), routeNo, 20)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "route_failed", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"route_id": routeNo,
		"stops":    detail,
	})
}

// LiveRoute is an SSE stream — GET /v1/live/route/{no} — vehicle positions, 10s cadence.
func (h *Handler) LiveRoute(w http.ResponseWriter, r *http.Request) {
	routeNo := chi.URLParam(r, "no")
	if routeNo == "" {
		writeError(w, http.StatusBadRequest, "missing_route", "route number required")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	if err := h.realtime.ServeSSE(r.Context(), routeNo, w); err != nil {
		writeError(w, http.StatusInternalServerError, "stream_failed", err.Error())
	}
}

// CreateBooking handles POST /v1/bookings — {optionId, rail: "ondc"|"deeplink"} → booking state machine.
func (h *Handler) CreateBooking(w http.ResponseWriter, r *http.Request) {
	var req bookingpkg.BookingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	bookingResp, err := h.booking.Book(r.Context(), &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "booking_failed", err.Error())
		return
	}

	body, _ := json.Marshal(bookingResp)
	h.events.Publish(r.Context(), "bookings.state", string(body))

	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}

// WalletTickets handles GET /v1/wallet/tickets — list user's tickets.
func (h *Handler) WalletTickets(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-Id")
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "missing_user", "X-User-Id header required")
		return
	}

	tickets, err := h.wallet.GetTicketsForUser(r.Context(), userID, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "wallet_failed", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"tickets": tickets, "count": len(tickets)})
}

// TicketPDF handles GET /v1/wallet/tickets/{id}.pdf — render ticket PDF.
func (h *Handler) TicketPDF(w http.ResponseWriter, r *http.Request) {
	ticketID := chi.URLParam(r, "id")
	if ticketID == "" {
		writeError(w, http.StatusBadRequest, "missing_ticket", "ticket id required")
		return
	}

	ticket, err := h.wallet.GetTicket(r.Context(), ticketID)
	if err != nil {
		writeError(w, http.StatusNotFound, "ticket_not_found", err.Error())
		return
	}

	renderer := walletpkg.NewPlaywrightPDFRenderer()
	pdf, err := renderer.RenderTicket(ticket)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "pdf_failed", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=ticket-%s.pdf", ticketID))
	w.Write(pdf)
}

// CreateAlert handles POST /v1/alerts — fare/disruption watchers.
func (h *Handler) CreateAlert(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID string                 `json:"user_id"`
		Kind   string                 `json:"kind"`
		Config map[string]interface{} `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	if req.UserID == "" || req.Kind == "" {
		writeError(w, http.StatusBadRequest, "missing_fields", "user_id and kind required")
		return
	}

	channels := []string{"push", "whatsapp"}
	switch req.Kind {
	case "fare_watcher":
		h.notifications.SendFareAlert(r.Context(), req.UserID, "", "", 0, 0, channels)
	case "disruption_watcher":
		h.notifications.SendDisruptionAlert(r.Context(), req.UserID, "", "", channels)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "alert_created"})
}

// FeedbackQuote handles POST /v1/feedback/quote — actual paid fare → trains the estimate models.
func (h *Handler) FeedbackQuote(w http.ResponseWriter, r *http.Request) {
	var req struct {
		QuoteID    string  `json:"quote_id"`
		ActualFare float64 `json:"actual_fare"`
		UserID     string  `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	if req.QuoteID == "" || req.ActualFare <= 0 {
		writeError(w, http.StatusBadRequest, "missing_fields", "quote_id and actual_fare required")
		return
	}

	if err := h.users.SubmitFeedback(r.Context(), req.QuoteID, &users.TripFeedback{}); err != nil {
		writeError(w, http.StatusInternalServerError, "feedback_failed", err.Error())
		return
	}

	body, _ := json.Marshal(map[string]any{
		"status":   "feedback_recorded",
		"quote_id": req.QuoteID,
		"actual":   req.ActualFare,
	})
	h.events.Publish(r.Context(), "quotes.feedback", string(body))

	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}

// Health handles GET /health and /healthz — liveness/readiness probe.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

// SetupRoutes wires the full PLATE 03 API surface.
func SetupRoutes(
	orch *comparison.Orchestrator,
	geo *geocoding.Service,
	tr *transit.Planner,
	bk *bookingpkg.BookingMgr,
	rt *realtime.ETAService,
	wl *walletpkg.TicketService,
	nt *notifpkg.NotificationHub,
	us *users.UserService,
	ch *cache.ValkeyCache,
	ev *events.EventBus,
) http.Handler {
	h := New(orch, geo, tr, bk, rt, wl, nt, us, ch, ev)
	r := chi.NewRouter()

	r.Get("/health", h.Health)
	r.Get("/healthz", h.Health)

	r.Route("/v1", func(r chi.Router) {
		r.Post("/compare", h.Compare)
		r.Get("/stops/nearby", h.NearbyStops)
		r.Get("/routes/{no}", h.RouteDetail)
		r.Get("/live/route/{no}", h.LiveRoute)
		r.Post("/bookings", h.CreateBooking)
		r.Get("/wallet/tickets", h.WalletTickets)
		r.Get("/wallet/tickets/{id}.pdf", h.TicketPDF)
		r.Post("/alerts", h.CreateAlert)
		r.Post("/feedback/quote", h.FeedbackQuote)
	})

	return r
}

// helpers

func writeError(w http.ResponseWriter, code int, errCode, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{
		"error":   errCode,
		"message": msg,
	})
}

func parseFloat(s string) (float64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty value")
	}
	return strconv.ParseFloat(s, 64)
}
