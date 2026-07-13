package booking

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ONDCClient implements Beckn protocol for ride booking
type ONDCClient struct {
	baseURL       string
	subscriberID  string
	uniqueKeyID   string
	privateKey    string
	bapID         string
	bapURI        string
	networkPartID string
	registryURL   string
	httpClient    *http.Client
	lookupCache   map[string]NetworkParticipant
}

// NetworkParticipant represents an ONDC network participant
type NetworkParticipant struct {
	ID          string   `json:"subscriber_id"`
	URI         string   `json:"subscriber_url"`
	Type        string   `json:"type"` // BAP, BPP, BG
	Domain      string   `json:"domain"`
	Country     string   `json:"country"`
	City        []string `json:"city"`
	SigningKey  string   `json:"signing_public_key"`
	EncryptKey  string   `json:"encr_public_key"`
	ValidFrom  time.Time `json:"valid_from"`
	ValidUntil time.Time `json:"valid_until"`
	Status     string   `json:"status"`
}

// Beckn message structures
type BecknMessage struct {
	Context    *Context     `json:"context"`
	Message    *Message     `json:"message"`
	Error      *Error       `json:"error,omitempty"`
}

type Context struct {
	Domain       string `json:"domain"`
	Country      string `json:"country"`
	City         string `json:"city"`
	Action       string `json:"action"` // search, on_search, select, on_select, init, on_init, confirm, on_confirm, status, on_status
	Version      string `json:"version"`
	MessageID    string `json:"message_id"`
	TransactionID string `json:"transaction_id"`
	TransactionScope string `json:"transaction_scope"`
	TransactionID string `json:"bap_id"`
	BAPURI      string `json:"bap_uri"`
	BPPID       string `json:"bpp_id,omitempty"`
	BPPURI      string `json:"bpp_uri,omitempty"`
	Timestamp   string `json:"timestamp"`
	KeyID       string `json:"key_id,omitempty"`
}

type Message struct {
	// For /search
	Intent *Intent `json:"intent,omitempty"`
	// For /on_search
	Catalog *Catalog `json:"catalog,omitempty"`
	// For /select
	Order *Order `json:"order,omitempty"`
	// For /init
	// For /confirm
	// For /status
}

type Intent struct {
	Provider    *Provider   `json:"provider,omitempty"`
	Items       []Item     `json:"items,omitempty"`
	Fulfillment *Fulfillment `json:"fulfillment,omitempty"`
	Payment     *Payment   `json:"payment,omitempty"`
}

type Provider struct {
	ID   string `json:"id"`
	URI  string `json:"url,omitempty"`
	Name string `json:"name,omitempty"`
}

type Item struct {
	ID           string `json:"id"`
	Descriptor   `json:"descriptor,omitempty"`
	Price        `json:"price,omitempty"`
	Quantity     `json:"quantity,omitempty"`
	CategoryID   string `json:"category_id,omitempty"`
	ProviderID   string `json:"provider_id,omitempty"`
	FulfillmentID string `json:"fulfillment_id,omitempty"`
}

type Descriptor struct {
	Name        string `json:"name"`
	Code        string `json:"code,omitempty"`
	Symbol      string `json:"symbol,omitempty"`
	ShortDesc   string `json:"short_desc,omitempty"`
	LongDesc    string `json:"long_desc,omitempty"`
	Images      []string `json:"images,omitempty"`
}

type Price struct {
	Currency string `json:"currency"`
	Value    string `json:"value"`
	OfferedValue string `json:"offered_value,omitempty"`
	ListedValue  string `json:"listed_value,omitempty"`
}

type Quantity struct {
	Count int `json:"count,omitempty"`
}

type Fulfillment struct {
	ID           string       `json:"id,omitempty"`
	Type         string       `json:"type"`
	Tracking     bool         `json:"tracking"`
	Customer     *Customer    `json:"customer,omitempty"`
	State        *State       `json:"state,omitempty"`
	Start        *TimeInfo   `json:"start,omitempty"`
	End          *TimeInfo   `json:"end,omitempty"`
	Agent        *Agent       `json:"agent,omitempty"`
	Vehicle      *Vehicle     `json:"vehicle,omitempty"`
	Stops        []FulfillmentStop `json:"stops,omitempty"`
}

type Customer struct {
	Person    *Person `json:"person,omitempty"`
	Contact   *Contact `json:"contact,omitempty"`
}

type Person struct {
	Name  string `json:"name,omitempty"`
	Creds string `json:"creds,omitempty"`
}

type Contact struct {
	Phone string `json:"phone,omitempty"`
	Email string `json:"email,omitempty"`
}

type TimeInfo struct {
	Timestamp string `json:"timestamp,omitempty"`
	Time      *TimeRange `json:"time,omitempty"`
}

type TimeRange struct {
	Start string `json:"start,omitempty"`
	End   string `json:"end,omitempty"`
}

type State struct {
	Descriptor *Descriptor `json:"descriptor,omitempty"`
	UpdatedAt  string      `json:"updated_at,omitempty"`
}

type Agent struct {
	Name   string `json:"name,omitempty"`
	Phone  string `json:"phone,omitempty"`
	Image  string `json:"image,omitempty"`
}

type Vehicle struct {
	Category string `json:"category"` // AUTO, CAB, BUS, METRO
	Capacity int    `json:"capacity"`
	RegNo    string `json:"registration"`
}

type FulfillmentStop struct {
	ID        string     `json:"id"`
	Location   *Location `json:"location,omitempty"`
	Time       *TimeInfo `json:"time,omitempty"`
	State      *State    `json:"state,omitempty"`
	Type       string    `json:"type,omitempty"` // START, END, INTERMITTENT
}

type Location struct {
	ID    string `json:"id,omitempty"`
	GPS   `json:"gps"`
	Address string `json:"address,omitempty"`
	Name   string `json:"name,omitempty"`
	City   *CityInfo `json:"city,omitempty"`
	Country *CountryInfo `json:"country,omitempty"`
}

type GPS struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type CityInfo struct {
	Name string `json:"name"`
	Code string `json:"code"`
}

type CountryInfo struct {
	Name string `json:"name"`
	Code string `json:"code"`
}

type Payment struct {
	BuyerFees    []Fee `json:"buyer_fees,omitempty"`
	CollectedBy  string `json:"collected_by"`
	Type         string `json:"type"` // PRE-TRANSACT, ON-TRANSACT, POST-TRANSACT
	URI          string `json:"uri,omitempty"`
	TLMethod     string `json:"tl_method,omitempty"`
	Currency     string `json:"currency"`
}

type Fee struct {
	Descriptor `json:"descriptor,omitempty"`
	Amount     string `json:"amount"`
}

type Catalog struct {
	Providers []CatalogProvider `json:"providers,omitempty"`
}

type CatalogProvider struct {
	ID          string    `json:"id"`
	Descriptor  Descriptor `json:"descriptor"`
	Locations   []Location `json:"locations,omitempty"`
	Items       []Item    `json:"items,omitempty"`
	Fulfillments []FulfillmentType `json:"fulfillments,omitempty"`
	Categories []Category `json:"categories,omitempty"`
}

type FulfillmentType struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Start    *TimeInfo `json:"start,omitempty"`
	End      *TimeInfo `json:"end,omitempty"`
}

type Category struct {
	ID         string     `json:"id"`
	ParentID   string     `json:"parent_id,omitempty"`
	Descriptor Descriptor `json:"descriptor"`
}

type Order struct {
	ID              string       `json:"id,omitempty"`
	Provider        *Provider   `json:"provider,omitempty"`
	Items           []Item      `json:"items,omitempty"`
	Fulfillment     *Fulfillment `json:"fulfillment,omitempty"`
	Payment         *Payment     `json:"payment,omitempty"`
	QuotedFare      float64     `json:"quote,omitempty"`
	State           string       `json:"state,omitempty"`
	CreatedAt       string       `json:"created_at,omitempty"`
	UpdatedAt       string       `json:"updated_at,omitempty"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// BookingService manages ride bookings
type BookingService struct {
	ondc       *ONDCClient
	deeplinkBuilder *DeeplinkBuilder
	ticketService *TicketService
}

// BookingRequest initiates a booking
type BookingRequest struct {
	OptionID   string          `json:"option_id"`
	Rail       string          `json:"rail"` // "ondc" or "deeplink"
	From       LocationInput  `json:"from"`
	To         LocationInput  `json:"to"`
	Customer   CustomerInput  `json:"customer"`
	Schedule   *ScheduleInput `json:"schedule,omitempty"`
	Payment    *PaymentInput  `json:"payment,omitempty"`
}

type LocationInput struct {
	Lat       float64 `json:"lat"`
	Lng       float64 `json:"lng"`
	Address   string  `json:"address,omitempty"`
	Landmark  string  `json:"landmark,omitempty"`
	Name      string  `json:"name,omitempty"`
}

type CustomerInput struct {
	Name     string `json:"name"`
	Phone    string `json:"phone"`
	Email    string `json:"email,omitempty"`
}

type ScheduleInput struct {
	PickupTime string `json:"pickup_time"` // RFC3339
}

type PaymentInput struct {
	Method  string `json:"method"` // UPI, CARD, WALLET
	UPIID   string `json:"upi_id,omitempty"`
}

// BookingResponse is the result of a booking attempt
type BookingResponse struct {
	BookingID     string      `json:"booking_id"`
	Status        string      `json:"status"` // pending, confirmed, failed, deeplink
	DeepLink      string      `json:"deeplink,omitempty"`
	TransactionID string      `json:"transaction_id,omitempty"`
	OrderID       string      `json:"order_id,omitempty"`
	Message       string      `json:"message,omitempty"`
	ETAMinutes    int         `json:"eta_minutes,omitempty"`
	Fare          float64     `json:"fare,omitempty"`
	FareCurrency  string      `json:"fare_currency,omitempty"`
}

// TicketService handles ticket generation
type TicketService struct {
	ticketURL  string
	httpClient *http.Client
}

// NewONDCClient creates a new ONDC BAP client
func NewONDCClient(bapID, bapURI, privateKey string) *ONDCClient {
	return &ONDCClient{
		baseURL:     "https://api.ondc.org",
		bapID:       bapID,
		bapURI:      bapURI,
		privateKey:  privateKey,
		registryURL: "https://registry.ondc.org/lookup",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		lookupCache: make(map[string]NetworkParticipant),
	}
}

// LookupBPP discovers BPPs by domain
func (c *ONDCClient) LookupBPP(domain string) ([]NetworkParticipant, error) {
	if cached, ok := c.lookupCache[domain]; ok {
		return []NetworkParticipant{cached}, nil
	}

	url := fmt.Sprintf("%s/lookup?type=BPP&domain=%s&country=IND&city=std:011",
		c.registryURL, domain)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry lookup failed: %d", resp.StatusCode)
	}

	var result []NetworkParticipant
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result) > 0 {
		c.lookupCache[domain] = result[0]
	}

	return result, nil
}

// Search sends a ride intent to the network
func (c *ONDCClient) Search(ctx context.Context, req *BookingRequest) (*BecknMessage, error) {
	msgID := generateMsgID()
	txnID := generateTxnID()

	becknCtx := &Context{
		Domain:           "nic2004:60232", // Urban mobility domain
		Country:          "IND",
		City:             "std:011",
		Action:           "search",
		Version:          "2.0.0",
		MessageID:        msgID,
		TransactionID:    txnID,
		TransactionScope: "BAP",
		BAPURI:          c.bapURI,
		Timestamp:        time.Now().Format(time.RFC3339),
	}

	intent := &Intent{
		Provider: &Provider{
			ID: "namma-yatri",
		},
		Fulfillment: &Fulfillment{
			Type:     "DRIVER",
			Tracking: false,
			Start: &TimeInfo{
				Location: &Location{
					GPS: GPS{Lat: req.From.Lat, Lng: req.From.Lng},
				},
			},
			End: &TimeInfo{
				Location: &Location{
					GPS: GPS{Lat: req.To.Lat, Lng: req.To.Lng},
				},
			},
		},
	}

	msg := &Message{Intent: intent}
	becknMsg := &BecknMessage{Context: becknCtx, Message: msg}

	// Find BPPs
	bpps, err := c.LookupBPP("nic2004:60232")
	if err != nil || len(bpps) == 0 {
		return nil, fmt.Errorf("no BPPs found: %w", err)
	}

	// Send to first available BPP
	bpp := bpps[0]
	becknCtx.BPPID = bpp.ID
	becknCtx.BPPURI = bpp.URI

	return c.send(ctx, bpp.URI+"/search", becknMsg)
}

// Select confirms vehicle selection
func (c *ONDCClient) Select(ctx context.Context, txnID, bppURI string, itemID, fulfillmentID string) (*BecknMessage, error) {
	msgID := generateMsgID()

	becknCtx := &Context{
		Domain:           "nic2004:60232",
		Country:          "IND",
		City:             "std:011",
		Action:           "select",
		Version:          "2.0.0",
		MessageID:        msgID,
		TransactionID:    txnID,
		TransactionScope: "BAP",
		BAPURI:          c.bapURI,
		BPPURI:          bppURI,
		Timestamp:        time.Now().Format(time.RFC3339),
	}

	msg := &Message{
		Order: &Order{
			Items: []Item{{ID: itemID}},
			Fulfillment: &Fulfillment{ID: fulfillmentID},
		},
	}

	return c.send(ctx, bppURI+"/select", &BecknMessage{Context: becknCtx, Message: msg})
}

// Init initializes the order
func (c *ONDCClient) Init(ctx context.Context, txnID, bppURI string, order *Order) (*BecknMessage, error) {
	msgID := generateMsgID()

	becknCtx := &Context{
		Domain:           "nic2004:60232",
		Country:          "IND",
		City:             "std:011",
		Action:           "init",
		Version:          "2.0.0",
		MessageID:        msgID,
		TransactionID:    txnID,
		TransactionScope: "BAP",
		BAPURI:          c.bapURI,
		BPPURI:          bppURI,
		Timestamp:        time.Now().Format(time.RFC3339),
	}

	return c.send(ctx, bppURI+"/init", &BecknMessage{Context: becknCtx, Message: &Message{Order: order}})
}

// Confirm finalizes the booking
func (c *ONDCClient) Confirm(ctx context.Context, txnID, bppURI string, order *Order) (*BecknMessage, error) {
	msgID := generateMsgID()

	becknCtx := &Context{
		Domain:           "nic2004:60232",
		Country:          "IND",
		City:             "std:011",
		Action:           "confirm",
		Version:          "2.0.0",
		MessageID:        msgID,
		TransactionID:    txnID,
		TransactionScope: "BAP",
		BAPURI:          c.bapURI,
		BPPURI:          bppURI,
		Timestamp:        time.Now().Format(time.RFC3339),
	}

	return c.send(ctx, bppURI+"/confirm", &BecknMessage{Context: becknCtx, Message: &Message{Order: order}})
}

// Status checks booking status
func (c *ONDCClient) Status(ctx context.Context, txnID, bppURI, orderID string) (*BecknMessage, error) {
	msgID := generateMsgID()

	becknCtx := &Context{
		Domain:           "nic2004:60232",
		Country:          "IND",
		City:             "std:011",
		Action:           "status",
		Version:          "2.0.0",
		MessageID:        msgID,
		TransactionID:    txnID,
		TransactionScope: "BAP",
		BAPURI:          c.bapURI,
		BPPURI:          bppURI,
		Timestamp:        time.Now().Format(time.RFC3339),
	}

	return c.send(ctx, bppURI+"/status", &BecknMessage{Context: becknCtx})
}

// Cancel cancels a booking
func (c *ONDCClient) Cancel(ctx context.Context, txnID, bppURI, orderID, reason string) (*BecknMessage, error) {
	msgID := generateMsgID()

	becknCtx := &Context{
		Domain:           "nic2004:60232",
		Country:          "IND",
		City:             "std:011",
		Action:           "cancel",
		Version:          "2.0.0",
		MessageID:        msgID,
		TransactionID:    txnID,
		TransactionScope: "BAP",
		BAPURI:          c.bapURI,
		BPPURI:          bppURI,
		Timestamp:        time.Now().Format(time.RFC3339),
	}

	return c.send(ctx, bppURI+"/cancel", &BecknMessage{Context: becknCtx})
}

func (c *ONDCClient) send(ctx context.Context, url string, msg *BecknMessage) (*BecknMessage, error) {
	body, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-ONDC-Message-Id", msg.Context.MessageID)
	req.Header.Set("X-ONDC-Transaction-Id", msg.Context.TransactionID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ONDC request failed: %d - %s", resp.StatusCode, string(respBody))
	}

	var result BecknMessage
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// DeeplinkBuilder generates provider deeplinks
type DeeplinkBuilder struct{}

// NewDeeplinkBuilder creates a new builder
func NewDeeplinkBuilder() *DeeplinkBuilder {
	return &DeeplinkBuilder{}
}

// Uber generates Uber deeplink
func (b *DeeplinkBuilder) Uber(fromLat, fromLng, toLat, toLng float64, productCategory string) string {
	return fmt.Sprintf(
		"https://m.uber.com/ul/?action=setPickup&pickup[latitude]=%.6f&pickup[longitude]=%.6f&dropoff[latitude]=%.6f&dropoff[longitude]=%.6f",
		fromLat, fromLng, toLat, toLng,
	)
}

// UberProduct returns URL for specific Uber product
func (b *DeeplinkBuilder) UberProduct(fromLat, fromLng, toLat, toLng float64, productID string) string {
	return fmt.Sprintf(
		"https://m.uber.com/ul/?action=setPickup&pickup[latitude]=%.6f&pickup[longitude]=%.6f&dropoff[latitude]=%.6f&dropoff[longitude]=%.6f&product_id=%s",
		fromLat, fromLng, toLat, toLng, productID,
	)
}

// Ola generates Ola deeplink
func (b *DeeplinkBuilder) Ola(fromLat, fromLng float64, category string) string {
	return fmt.Sprintf(
		"https://www.olacabs.com/book?lat=%f&lng=%f&category=%s",
		fromLat, fromLng, category,
	)
}

// Rapido generates Rapido deeplink
func (b *DeeplinkBuilder) Rapido(fromLat, fromLng, toLat, toLng float64, service string) string {
	return fmt.Sprintf(
		"https://www.rapido.bike/?src_lat=%f&src_lng=%f&dst_lat=%f&dst_lng=%f&service=%s",
		fromLat, fromLng, toLat, toLng, service,
	)
}

// DMRC generates DMRC QR ticket URL
func (b *DeeplinkBuilder) DMRC(fromStation, toStation string) string {
	return fmt.Sprintf(
		"https://www.delhimetrorail.com/qr-ticket?from=%s&to=%s",
		fromStation, toStation,
	)
}

// PhonePe generates payment deep link
func (b *DeeplinkBuilder) PhonePe(merchantID, amount int64, callbackURL string) string {
	return fmt.Sprintf(
		"phonepe://pay?merchantId=%s&amount=%d&callbackUrl=%s",
		merchantID, amount, callbackURL,
	)
}

// GPay generates Google Pay request
func (b *DeeplinkBuilder) GPay(merchantID string, upiID string, amount float64, name string) string {
	// UPI deep link format
	return fmt.Sprintf(
		"gpay://pay?pa=%s&pn=%s&am=%.2f&cu=INR",
		upiID, name, amount,
	)
}

// BookingService manages the full booking flow
type BookingMgr struct {
	ondc            *ONDCClient
	deeplinkBuilder *DeeplinkBuilder
	ticketService   *TicketService
}

// NewBookingService creates a new booking service
func NewBookingService(bapID, bapURI, privateKey string) *BookingMgr {
	return &BookingMgr{
		ondc:            NewONDCClient(bapID, bapURI, privateKey),
		deeplinkBuilder: NewDeeplinkBuilder(),
		ticketService:   NewTicketService(),
	}
}

// Book attempts to book via the specified rail
func (s *BookingMgr) Book(ctx context.Context, req *BookingRequest) (*BookingResponse, error) {
	switch strings.ToLower(req.Rail) {
	case "ondc":
		return s.bookONDC(ctx, req)
	case "deeplink":
		return s.bookDeeplink(ctx, req)
	default:
		return nil, fmt.Errorf("unsupported rail: %s", req.Rail)
	}
}

func (s *BookingMgr) bookONDC(ctx context.Context, req *BookingRequest) (*BookingResponse, error) {
	bookingID := generateBookingID()

	// Step 1: Search
	searchResp, err := s.ondc.Search(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	txnID := searchResp.Context.TransactionID
	bppURI := searchResp.Context.BPPURI

	if searchResp.Error != nil {
		return &BookingResponse{
			BookingID: bookingID,
			Status:    "failed",
			Message:   searchResp.Error.Message,
		}, nil
	}

	// Step 2: Select (pick first available)
	catalog := searchResp.Message.Catalog
	if catalog == nil || len(catalog.Providers) == 0 {
		return nil, fmt.Errorf("no providers available")
	}

	provider := catalog.Providers[0]
	var itemID, fulfillmentID string
	if len(provider.Items) > 0 {
		itemID = provider.Items[0].ID
	}
	if len(provider.Fulfillments) > 0 {
		fulfillmentID = provider.Fulfillments[0].ID
	}

	_, err = s.ondc.Select(ctx, txnID, bppURI, itemID, fulfillmentID)
	if err != nil {
		return nil, fmt.Errorf("select failed: %w", err)
	}

	// Step 3: Init
	order := &Order{
		Provider: &Provider{ID: provider.ID},
		Items:    []Item{{ID: itemID}},
		Fulfillment: &Fulfillment{
			ID:   fulfillmentID,
			Type: "DRIVER",
			Start: &TimeInfo{
				Location: &Location{
					GPS: GPS{Lat: req.From.Lat, Lng: req.From.Lng},
				},
			},
			End: &TimeInfo{
				Location: &Location{
					GPS: GPS{Lat: req.To.Lat, Lng: req.To.Lng},
				},
			},
			Customer: &Customer{
				Person: &Person{Name: req.Customer.Name},
				Contact: &Contact{Phone: req.Customer.Phone},
			},
		},
		Payment: &Payment{
			CollectedBy: "BAP",
			Type:       "PRE-TRANSACT",
			Currency:   "INR",
		},
	}

	initResp, err := s.ondc.Init(ctx, txnID, bppURI, order)
	if err != nil {
		return nil, fmt.Errorf("init failed: %w", err)
	}

	// Step 4: Confirm
	order.ID = generateBookingID()
	confirmResp, err := s.ondc.Confirm(ctx, txnID, bppURI, order)
	if err != nil {
		return nil, fmt.Errorf("confirm failed: %w", err)
	}

	if confirmResp.Error != nil {
		return &BookingResponse{
			BookingID:     bookingID,
			TransactionID: txnID,
			Status:       "failed",
			Message:      confirmResp.Error.Message,
		}, nil
	}

	return &BookingResponse{
		BookingID:     bookingID,
		Status:        "confirmed",
		TransactionID: txnID,
		OrderID:       order.ID,
		Message:       "Booking confirmed via ONDC",
	}, nil
}

func (s *BookingMgr) bookDeeplink(ctx context.Context, req *BookingRequest) (*BookingResponse, error) {
	bookingID := generateBookingID()

	// Generate deeplink based on option
	var deeplink string
	switch {
	case strings.Contains(req.OptionID, "uber"):
		deeplink = s.deeplinkBuilder.Uber(req.From.Lat, req.From.Lng, req.To.Lat, req.To.Lng, "auto")
	case strings.Contains(req.OptionID, "ola"):
		deeplink = s.deeplinkBuilder.Ola(req.From.Lat, req.From.Lng, "auto")
	case strings.Contains(req.OptionID, "rapido"):
		deeplink = s.deeplinkBuilder.Rapido(req.From.Lat, req.From.Lng, req.To.Lat, req.To.Lng, "auto")
	case strings.Contains(req.OptionID, "metro"):
		deeplink = s.deeplinkBuilder.DMRC("", "")
	default:
		deeplink = s.deeplinkBuilder.Uber(req.From.Lat, req.From.Lng, req.To.Lat, req.To.Lng, "")
	}

	return &BookingResponse{
		BookingID: bookingID,
		Status:   "deeplink",
		DeepLink: deeplink,
		Message:  "Redirecting to provider app",
	}, nil
}

// GetBookingStatus retrieves the current status
func (s *BookingMgr) GetBookingStatus(ctx context.Context, txnID, bppURI, orderID string) (*BookingResponse, error) {
	statusResp, err := s.ondc.Status(ctx, txnID, bppURI, orderID)
	if err != nil {
		return nil, err
	}

	var status string
	if statusResp.Message != nil && statusResp.Message.Order != nil {
		status = statusResp.Message.Order.State
	}

	return &BookingResponse{
		TransactionID: txnID,
		OrderID:       orderID,
		Status:        status,
	}, nil
}

// CancelBooking cancels an existing booking
func (s *BookingMgr) CancelBooking(ctx context.Context, txnID, bppURI, orderID, reason string) (*BookingResponse, error) {
	cancelResp, err := s.ondc.Cancel(ctx, txnID, bppURI, orderID, reason)
	if err != nil {
		return nil, err
	}

	if cancelResp.Error != nil {
		return &BookingResponse{
			BookingID: orderID,
			Status:    "failed",
			Message:   cancelResp.Error.Message,
		}, nil
	}

	return &BookingResponse{
		BookingID: orderID,
		Status:   "cancelled",
		Message:  "Booking cancelled successfully",
	}, nil
}

func NewTicketService() *TicketService {
	return &TicketService{
		ticketURL:  "https://api.delhimetrorail.com/tickets",
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func generateMsgID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)[:21]
}

func generateTxnID() string {
	b := make([]byte, 16)
	rand.Read(b)
	hash := sha256.Sum256(b)
	return fmt.Sprintf("txn:%s", base64.URLEncoding.EncodeToString(hash[:])[:16])
}

func generateBookingID() string {
	id := uuid.New()
	return fmt.Sprintf("SAW-%s", strings.ToUpper(id.String()[:8]))
}
