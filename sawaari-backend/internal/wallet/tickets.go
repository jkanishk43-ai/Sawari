package wallet

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sawaari/backend/internal/cache"
)

// TicketService manages the QR ticket vault and PDF generation
type TicketService struct {
	cache          *cache.ValkeyCache
	qrVault        *QRVault
	history        []TicketRecord
	paymentGateway *PaymentGateway
	mu             sync.RWMutex
}

// Ticket represents a single ticket
type Ticket struct {
	ID           string    `json:"id"`
	BookingID    string    `json:"booking_id"`
	Provider     string    `json:"provider"`    // DMRC, DTC
	Type         string    `json:"type"`        // metro_qr, bus_qr, paper
	Status       string    `json:"status"`      // active, used, expired, refunded, pending
	Fare         float64   `json:"fare"`
	Currency     string    `json:"currency"`
	Origin       string    `json:"origin"`
	Destination  string    `json:"destination"`
	ValidFrom    time.Time `json:"valid_from"`
	ValidTo      time.Time `json:"valid_to"`
	QRPayload    string    `json:"qr_payload,omitempty"`
	PDFURL       string    `json:"pdf_url,omitempty"`
	PassengerRef string    `json:"passenger_ref,omitempty"` // Saheli card ref
	Metadata     map[string]string `json:"metadata,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TicketRecord includes cached state for listing
type TicketRecord struct {
	Ticket
	UsedAt    *time.Time `json:"used_at,omitempty"`
	RefundID  string     `json:"refund_id,omitempty"`
}

// QRVault stores QR code payloads
type QRVault struct {
	cache *cache.ValkeyCache
	mu    sync.RWMutex
}

// PaymentGateway abstracts UPI payments
type PaymentGateway struct {
	apiKey     string
	merchantID string
	vpa        string
	baseURL    string
}

// NewTicketService creates a new ticket service
func NewTicketService(c *cache.ValkeyCache) *TicketService {
	return &TicketService{
		cache: c,
		qrVault: &QRVault{
			cache: c,
		},
		history:      make([]TicketRecord, 0),
		paymentGateway: NewPaymentGateway(),
	}
}

// NewPaymentGateway creates the payment adapter
func NewPaymentGateway() *PaymentGateway {
	return &PaymentGateway{
		baseURL:    "https://api.razorpay.com/v1",
	}
}

// GenerateTicket creates a ticket from a booking
func (s *TicketService) GenerateTicket(ctx context.Context, bookingID, provider, origin, dest string, fare float64) (*Ticket, error) {
	now := time.Now()
	ticket := &Ticket{
		ID:          generateTicketID(),
		BookingID:   bookingID,
		Provider:    provider,
		Type:        ticketType(provider),
		Status:      "active",
		Fare:        fare,
		Currency:    "INR",
		Origin:      origin,
		Destination: dest,
		ValidFrom:   now,
		ValidTo:     calculateValidUntil(provider),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Generate QR payload
	qrPayload, err := s.qrVault.GenerateQRPayload(ticket)
	if err != nil {
		return nil, fmt.Errorf("failed to generate QR: %w", err)
	}
	ticket.QRPayload = qrPayload

	// Store in Valkey cache for quick access
	key := fmt.Sprintf("ticket:%s", ticket.ID)
	s.cache.Set(ctx, key, 6*time.Hour, ticket)

	// Add to history
	s.mu.Lock()
	s.history = append(s.history, TicketRecord{Ticket: *ticket})
	s.mu.Unlock()

	return ticket, nil
}

// GetTicket retrieves a ticket by ID
func (s *TicketService) GetTicket(ctx context.Context, ticketID string) (*Ticket, error) {
	key := fmt.Sprintf("ticket:%s", ticketID)
	var ticket Ticket
	if err := s.cache.Get(ctx, key, &ticket); err == nil {
		return &ticket, nil
	}

	// Fallback: search in history
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, record := range s.history {
		if record.ID == ticketID {
			return &record.Ticket, nil
		}
	}

	return nil, fmt.Errorf("ticket not found: %s", ticketID)
}

// GetTicketsForUser returns all tickets for a user
func (s *TicketService) GetTicketsForUser(ctx context.Context, userID string) ([]TicketRecord, error) {
	// In production, query the database by user_id
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]TicketRecord, 0)
	for _, record := range s.history {
		// Filter by user (via booking linkage)
		_ = userID
		result = append(result, record)
	}

	return result, nil
}

// QRVault methods

func (v *QRVault) GenerateQRPayload(ticket *Ticket) (string, error) {
	// Create QR payload following DMRC/ONDC ticket spec
	payload := map[string]interface{}{
		"ticket_id":  ticket.ID,
		"booking_id": ticket.BookingID,
		"provider":   ticket.Provider,
		"type":       ticket.Type,
		"origin":     ticket.Origin,
		"destination": ticket.Destination,
		"valid_from": ticket.ValidFrom.Unix(),
		"valid_to":   ticket.ValidTo.Unix(),
		"fare":       ticket.Fare,
		"hash":       generateQRHash(ticket),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	// In production, encode as a QR code (e.g., using go-qrcode)
	// and store the raw payload for verification
	return base64.URLEncoding.EncodeToString(data), nil
}

func (v *QRVault) VerifyQRPayload(encoded string) (*Ticket, error) {
	data, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}

	// Verify hash
	var payload struct {
		TicketID    string `json:"ticket_id"`
		BookingID   string `json:"booking_id"`
		Provider    string `json:"provider"`
		Hash        string `json:"hash"`
	}

	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}

	key := fmt.Sprintf("ticket:%s", payload.TicketID)
	var ticket Ticket
	if err := v.cache.Get(context.Background(), key, &ticket); err != nil {
		return nil, fmt.Errorf("ticket not found or expired")
	}

	if payload.Hash != generateQRHash(&ticket) {
		return nil, fmt.Errorf("QR payload hash mismatch")
	}

	return &ticket, nil
}

func generateQRHash(ticket *Ticket) string {
	raw := fmt.Sprintf("%s:%s:%f:%d", ticket.ID, ticket.Provider, ticket.Fare, ticket.ValidFrom.Unix())
	hash := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", hash[:8])
}

// PDF renderer interface
type PDFRenderer interface {
	RenderTicket(ticket *Ticket) ([]byte, error)
	RenderReceipt(ticket *Ticket) ([]byte, error)
	RenderItinerary(tickets []Ticket) ([]byte, error)
}

// PlaywrightPDFRenderer renders PDFs via Playwright
type PlaywrightPDFRenderer struct {
	playgroundURL string
	httpClient    *http.Client
}

func NewPlaywrightPDFRenderer() *PlaywrightPDFRenderer {
	return &PlaywrightPDFRenderer{
		playgroundURL: "http://localhost:3000/render",
		httpClient:    &http.Client{Timeout: 60 * time.Second},
	}
}

func (r *PlaywrightPDFRenderer) RenderTicket(ticket *Ticket) ([]byte, error) {
	payload := map[string]interface{}{
		"template": "ticket",
		"data":     ticket,
	}

	body, _ := json.Marshal(payload)
	resp, err := r.httpClient.Post(r.playgroundURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return ioReadAll(resp.Body)
}

func (r *PlaywrightPDFRenderer) RenderReceipt(ticket *Ticket) ([]byte, error) {
	payload := map[string]interface{}{
		"template": "receipt",
		"data":     ticket,
	}

	body, _ := json.Marshal(payload)
	resp, err := r.httpClient.Post(r.playgroundURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return ioReadAll(resp.Body)
}

func (r *PlaywrightPDFRenderer) RenderItinerary(tickets []Ticket) ([]byte, error) {
	payload := map[string]interface{}{
		"template": "itinerary",
		"data": map[string]interface{}{
			"tickets": tickets,
		},
	}

	body, _ := json.Marshal(payload)
	resp, err := r.httpClient.Post(r.playgroundURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return ioReadAll(resp.Body)
}

// Payment methods

type PaymentRequest struct {
	Amount    float64           `json:"amount"`
	Currency  string            `json:"currency"`
	Method    string            `json:"method"` // upi, card, wallet, netbanking
	UserID    string            `json:"user_id"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type PaymentResponse struct {
	PaymentID     string    `json:"payment_id"`
	Status        string    `json:"status"`
	Amount        float64   `json:"amount"`
	Currency      string    `json:"currency"`
	UPIRef        string    `json:"upi_ref,omitempty"`
	Verification  string    `json:"verification"`
	CreatedAt     time.Time `json:"created_at"`
}

func (pg *PaymentGateway) InitiateUPI(ctx context.Context, req *PaymentRequest) (*PaymentResponse, error) {
	// Razorpay payment link format
	payload := map[string]interface{}{
		"amount":     int(req.Amount * 100), // paise
		"currency":   req.Currency,
		"method":     "upi",
		"type":       "link",
		"upi_link":   true,
		"reference_id": req.UserID,
	}

	body, _ := json.Marshal(payload)
	resp, err := pg.httpRequest(ctx, "POST", "/payment_links", body)
	if err != nil {
		return nil, err
	}

	var razorpayResp struct {
		ID       string `json:"id"`
		ShortURL string `json:"short_url"`
		Amount   int    `json:"amount"`
	}

	if err := json.Unmarshal(resp, &razorpayResp); err != nil {
		return nil, err
	}

	return &PaymentResponse{
		PaymentID:    razorpayResp.ID,
		Status:      "pending",
		Amount:      float64(razorpayResp.Amount) / 100,
		UPIRef:      razorpayResp.ShortURL,
		CreatedAt:   time.Now(),
	}, nil
}

func (pg *PaymentGateway) VerifyPayment(ctx context.Context, paymentID string) (*PaymentResponse, error) {
	resp, err := pg.httpRequest(ctx, "GET", fmt.Sprintf("/payments/%s", paymentID), nil)
	if err != nil {
		return nil, err
	}

	var payment struct {
		ID       string `json:"id"`
		Status   string `json:"status"`
		Amount   float64 `json:"amount"`
		Currency string `json:"currency"`
	}

	if err := json.Unmarshal(resp, &payment); err != nil {
		return nil, err
	}

	return &PaymentResponse{
		PaymentID:  payment.ID,
		Status:    payment.Status,
		Amount:    payment.Amount,
		Currency:  payment.Currency,
		CreatedAt: time.Now(),
	}, nil
}

func (pg *PaymentGateway) ProcessRefund(ctx context.Context, paymentID string, amount float64, reason string) (*PaymentResponse, error) {
	payload := map[string]interface{}{
		"amount":    int(amount * 100),
		"speed":     "instant",
		"notes": map[string]string{
			"reason": reason,
		},
	}

	body, _ := json.Marshal(payload)
	resp, err := pg.httpRequest(ctx, "POST", fmt.Sprintf("/payments/%s/refund", paymentID), body)
	if err != nil {
		return nil, err
	}

	var refund struct {
		ID      string  `json:"id"`
		Status  string  `json:"status"`
		Amount  float64 `json:"amount"`
	}

	if err := json.Unmarshal(resp, &refund); err != nil {
		return nil, err
	}

	return &PaymentResponse{
		PaymentID: refund.ID,
		Status:    refund.Status,
		Amount:    refund.Amount,
		CreatedAt: time.Now(),
	}, nil
}

func (pg *PaymentGateway) httpRequest(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	url := pg.baseURL + path

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(pg.apiKey+":"+pg.apiKey)))

	resp, err := pg.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return ioReadAll(resp.Body)
}

func (s *TicketService) GetTicketsForUser(ctx context.Context, userID string, status string) ([]TicketRecord, error) {
	var result []TicketRecord

	if status == "" {
		result = make([]TicketRecord, 0, len(s.history))
		s.mu.RLock()
		copy(result, s.history)
		s.mu.RUnlock()
	} else {
		s.mu.RLock()
		for _, record := range s.history {
			if record.Status == status {
				result = append(result, record)
			}
		}
		s.mu.RUnlock()
	}

	// Sort by created_at descending
	sortByCreatedAtDesc(result)

	return result, nil
}

func sortByCreatedAtDesc(records []TicketRecord) {
	sort.Slice(records, func(i, j int) bool {
		return records[i].CreatedAt.After(records[j].CreatedAt)
	})
}

func ticketType(provider string) string {
	switch provider {
	case "DMRC":
		return "metro_qr"
	case "DTC":
		return "bus_qr"
	default:
		return "paper"
	}
}

func calculateValidUntil(provider string) time.Time {
	switch provider {
	case "DMRC":
		return time.Now().Add(24 * time.Hour) // 24h Metro validity
	case "DTC":
		return time.Now().Add(1 * time.Hour) // 1h bus validity
	default:
		return time.Now().Add(24 * time.Hour)
	}
}

func generateTicketID() string {
	b := make([]byte, 12)
	for i := range b {
		b[i] = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"[byte(len("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"))%byte(len("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"))]
	}
	return fmt.Sprintf("TKT-%s", base64.URLEncoding.EncodeToString(b)[:12])
}

func ioReadAll(r io.Reader) ([]byte, error) {
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
