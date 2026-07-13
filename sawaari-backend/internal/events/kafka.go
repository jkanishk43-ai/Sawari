package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
)

// EventBus provides Kafka-based event streaming
type EventBus struct {
	brokers   []string
	topic     string
	producer  *kafka.Writer
	consumers map[string]*kafka.Reader
	handlers  map[string][]EventHandler
	mu        sync.RWMutex
	connected bool
}

// EventHandler is a function that processes events
type EventHandler func(ctx context.Context, event Event) error

// Event is the base event structure
type Event struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Timestamp time.Time       `json:"timestamp"`
	Source    string          `json:"source"`
	Version   string          `json:"version"`
	Data      json.RawMessage `json:"data"`
}

// BusPositionEvent is published when a bus position is updated
type BusPositionEvent struct {
	VehicleID     string    `json:"vehicle_id"`
	TripID        string    `json:"trip_id"`
	RouteID       string    `json:"route_id"`
	RouteShortName string   `json:"route_short_name"`
	Lat           float64   `json:"lat"`
	Lng           float64   `json:"lng"`
	Bearing       float64   `json:"bearing"`
	Speed         float64   `json:"speed"` // m/s
	StopID        string    `json:"stop_id,omitempty"`
	StopSequence  int       `json:"stop_sequence,omitempty"`
	OccupancyStatus string  `json:"occupancy_status,omitempty"`
	Timestamp     time.Time `json:"timestamp"`
	H3Index       string    `json:"h3_index"`
}

// QuotesIssuedEvent is published when fare quotes are generated
type QuotesIssuedEvent struct {
	RequestID  string            `json:"request_id"`
	UserID     string            `json:"user_id,omitempty"`
	From       Location          `json:"from"`
	To         Location          `json:"to"`
	Quotes     []Quote           `json:"quotes"`
	GeneratedAt time.Time        `json:"generated_at"`
	ExpiresAt  time.Time        `json:"expires_at"`
}

// Quote represents a single quote
type Quote struct {
	ID           string          `json:"id"`
	Provider     string          `json:"provider"`
	Mode         string          `json:"mode"`
	FareMin      float64         `json:"fare_min"`
	FareMax      float64         `json:"fare_max"`
	ETA          int             `json:"eta_minutes"`
	Surge        float64         `json:"surge,omitempty"`
	Badges       []string        `json:"badges"`
	Breakdown    []LineItem      `json:"breakdown"`
	DeepLink     string          `json:"deeplink,omitempty"`
}

// BookingsStateEvent is published when booking state changes
type BookingsStateEvent struct {
	BookingID     string        `json:"booking_id"`
	UserID        string        `json:"user_id"`
	QuoteID       string        `json:"quote_id"`
	Provider      string        `json:"provider"`
	Mode          string        `json:"mode"`
	Rail          string        `json:"rail"` // ondc, deeplink
	Status        BookingStatus `json:"status"`
	TransactionID string        `json:"transaction_id,omitempty"`
	DeepLink      string        `json:"deeplink,omitempty"`
	TrackingURL   string        `json:"tracking_url,omitempty"`
	Fare          float64       `json:"fare"`
	Driver        *DriverInfo   `json:"driver,omitempty"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

// BookingStatus represents booking state
type BookingStatus string

const (
	BookingPending   BookingStatus = "pending"
	BookingConfirmed BookingStatus = "confirmed"
	BookingAccepted  BookingStatus = "accepted"
	BookingArrived   BookingStatus = "arrived"
	BookingInProgress BookingStatus = "in_progress"
	BookingCompleted BookingStatus = "completed"
	BookingCancelled BookingStatus = "cancelled"
	BookingFailed    BookingStatus = "failed"
)

// AlertsFiredEvent is published when an alert is triggered
type AlertsFiredEvent struct {
	AlertID     string      `json:"alert_id"`
	UserID      string      `json:"user_id"`
	Type        AlertType   `json:"type"`
	Title       string      `json:"title"`
	Message     string      `json:"message"`
	Data        AlertData   `json:"data"`
	FiredAt     time.Time   `json:"fired_at"`
	Channels    []string    `json:"channels"` // fcm, whatsapp, telegram, email
}

// AlertType represents alert categories
type AlertType string

const (
	AlertFareDrop     AlertType = "fare_drop"
	AlertDisruption   AlertType = "disruption"
	AlertServiceStart AlertType = "service_start"
	AlertReminder     AlertType = "reminder"
	AlertPromo        AlertType = "promotion"
)

// Location for event data
type Location struct {
	Lat       float64 `json:"lat"`
	Lng       float64 `json:"lng"`
	Address   string  `json:"address,omitempty"`
	Name      string  `json:"name,omitempty"`
	StopID    string  `json:"stop_id,omitempty"`
}

// LineItem for fare breakdown
type LineItem struct {
	Name   string  `json:"name"`
	Amount float64 `json:"amount"`
}

// DriverInfo for booking events
type DriverInfo struct {
	Name         string `json:"name"`
	Phone        string `json:"phone"`
	PhotoURL     string `json:"photo_url,omitempty"`
	VehicleNumber string `json:"vehicle_number"`
	VehicleModel string `json:"vehicle_model,omitempty"`
}

// AlertData contains alert-specific data
type AlertData struct {
	RouteID      string   `json:"route_id,omitempty"`
	OldFare      float64  `json:"old_fare,omitempty"`
	NewFare      float64  `json:"new_fare,omitempty"`
	StopIDs      []string `json:"stop_ids,omitempty"`
	DelayMinutes int      `json:"delay_minutes,omitempty"`
}

// TariffsChangedEvent for tariff updates
type TariffsChangedEvent struct {
	TariffID     string    `json:"tariff_id"`
	Mode         string    `json:"mode"`
	Version      int       `json:"version"`
	EffectiveFrom time.Time `json:"effective_from"`
	Changes      []TariffChange `json:"changes"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TariffChange describes a single tariff modification
type TariffChange struct {
	Field string      `json:"field"` // base_fare, per_km, etc.
	Old   interface{} `json:"old_value"`
	New   interface{} `json:"new_value"`
}

// NewEventBus creates a new Kafka event bus
func NewEventBus(brokers []string, topic string) *EventBus {
	return &EventBus{
		brokers:   brokers,
		topic:     topic,
		consumers: make(map[string]*kafka.Reader),
		handlers:  make(map[string][]EventHandler),
	}
}

// Connect establishes connection to Kafka
func (eb *EventBus) Connect() error {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.producer = &kafka.Writer{
		Addr:         kafka.TCP(eb.brokers...),
		Topic:         eb.topic,
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond,
		RequiredAcks: kafka.RequireOne,
	}

	eb.connected = true
	log.Println("Connected to Kafka")
	return nil
}

// Publish sends an event to Kafka
func (eb *EventBus) Publish(ctx context.Context, eventType string, data interface{}) error {
	if !eb.connected {
		// Fallback: log locally
		log.Printf("EVENT (local): %s - %+v", eventType, data)
		return nil
	}

	eventID := fmt.Sprintf("%d", time.Now().UnixNano())
	event := Event{
		Type:      eventType,
		ID:        eventID,
		Timestamp: time.Now(),
		Source:    "sawaari-backend",
		Version:   "1.0",
	}

	eventData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}
	event.Data = eventData

	eventBytes, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	msg := kafka.Message{
		Key:   []byte(eventType),
		Value: eventBytes,
		Headers: []kafka.Header{
			{Key: "event_type", Value: []byte(eventType)},
			{Key: "source", Value: []byte("sawaari-backend")},
		},
	}

	if err := eb.producer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	return nil
}

// Subscribe registers a handler for an event type
func (eb *EventBus) Subscribe(eventType string, handler EventHandler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.handlers[eventType] = append(eb.handlers[eventType], handler)
}

// StartConsumer begins consuming events
func (eb *EventBus) StartConsumer(ctx context.Context, groupID string) error {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  eb.brokers,
		Topic:    eb.topic,
		GroupID:  groupID,
		MinBytes: 10e3,
		MaxBytes: 10e6,
	})

	eb.mu.Lock()
	eb.consumers[groupID] = reader
	eb.mu.Unlock()

	go func() {
		for {
			select {
			case <-ctx.Done():
				reader.Close()
				return
			default:
				msg, err := reader.ReadMessage(ctx)
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					log.Printf("Error reading message: %v", err)
					continue
				}

				eb.processMessage(ctx, msg)
			}
		}
	}()

	return nil
}

func (eb *EventBus) processMessage(ctx context.Context, msg kafka.Message) {
	var event Event
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		log.Printf("Failed to unmarshal event: %v", err)
		return
	}

	eb.mu.RLock()
	handlers := eb.handlers[event.Type]
	eb.mu.RUnlock()

	for _, handler := range handlers {
		if err := handler(ctx, event); err != nil {
			log.Printf("Handler error for %s: %v", event.Type, err)
		}
	}
}

// PublishBusPosition publishes a bus position update
func (eb *EventBus) PublishBusPosition(ctx context.Context, pos BusPositionEvent) error {
	return eb.Publish(ctx, "bus.positions", pos)
}

// PublishQuotesIssued publishes fare quotes
func (eb *EventBus) PublishQuotesIssued(ctx context.Context, quotes QuotesIssuedEvent) error {
	return eb.Publish(ctx, "quotes.issued", quotes)
}

// PublishQuotesFeedback publishes quote feedback
func (eb *EventBus) PublishQuotesFeedback(ctx context.Context, feedback QuoteFeedbackEvent) error {
	return eb.Publish(ctx, "quotes.feedback", feedback)
}

// QuoteFeedbackEvent is published when user provides fare feedback
type QuoteFeedbackEvent struct {
	QuoteID    string    `json:"quote_id"`
	UserID     string    `json:"user_id"`
	ActualFare float64   `json:"actual_fare"`
	Surge      float64   `json:"surge_used"`
	Provider   string    `json:"provider"`
	Mode       string    `json:"mode"`
	Delta      float64   `json:"delta"` // actual - quoted
	DeltaPct   float64   `json:"delta_pct"`
	Feedback   string    `json:"feedback,omitempty"` // thumbs_up, thumbs_down
	Timestamp  time.Time `json:"timestamp"`
}

// PublishBookingState publishes booking state change
func (eb *EventBus) PublishBookingState(ctx context.Context, booking BookingsStateEvent) error {
	return eb.Publish(ctx, "bookings.state", booking)
}

// PublishAlert publishes an alert
func (eb *EventBus) PublishAlert(ctx context.Context, alert AlertsFiredEvent) error {
	return eb.Publish(ctx, "alerts.fired", alert)
}

// PublishTariffsChanged publishes tariff changes
func (eb *EventBus) PublishTariffsChanged(ctx context.Context, tariffs TariffsChangedEvent) error {
	return eb.Publish(ctx, "tariffs.changed", tariffs)
}

// EventPublisher provides typed event publishing
type EventPublisher struct {
	bus *EventBus
}

// NewEventPublisher creates a typed event publisher
func NewEventPublisher(bus *EventBus) *EventPublisher {
	return &EventPublisher{bus: bus}
}

// PublishPosition publishes bus position
func (p *EventPublisher) PublishPosition(ctx context.Context, pos BusPositionEvent) error {
	return p.bus.PublishBusPosition(ctx, pos)
}

// PublishQuotes publishes quotes
func (p *EventPublisher) PublishQuotes(ctx context.Context, quotes QuotesIssuedEvent) error {
	return p.bus.PublishQuotesIssued(ctx, quotes)
}

// PublishFeedback publishes quote feedback
func (p *EventPublisher) PublishFeedback(ctx context.Context, feedback QuoteFeedbackEvent) error {
	return p.bus.PublishQuotesFeedback(ctx, feedback)
}

// PublishBooking publishes booking state
func (p *EventPublisher) PublishBooking(ctx context.Context, booking BookingsStateEvent) error {
	return p.bus.PublishBookingState(ctx, booking)
}

// PublishAlert publishes alert
func (p *EventPublisher) PublishAlert(ctx context.Context, alert AlertsFiredEvent) error {
	return p.bus.PublishAlert(ctx, alert)
}

// EventSubscriber provides typed event subscription
type EventSubscriber struct {
	bus *EventBus
}

// NewEventSubscriber creates a typed event subscriber
func NewEventSubscriber(bus *EventBus) *EventSubscriber {
	return &EventSubscriber{bus: bus}
}

// SubscribePositions subscribes to bus position events
func (s *EventSubscriber) SubscribePositions(handler func(ctx context.Context, pos BusPositionEvent) error) {
	s.bus.Subscribe("bus.positions", func(ctx context.Context, event Event) error {
		var pos BusPositionEvent
		if err := json.Unmarshal(event.Data, &pos); err != nil {
			return err
		}
		return handler(ctx, pos)
	})
}

// SubscribeQuotes subscribes to quotes.issued events
func (s *EventSubscriber) SubscribeQuotes(handler func(ctx context.Context, quotes QuotesIssuedEvent) error) {
	s.bus.Subscribe("quotes.issued", func(ctx context.Context, event Event) error {
		var quotes QuotesIssuedEvent
		if err := json.Unmarshal(event.Data, &quotes); err != nil {
			return err
		}
		return handler(ctx, quotes)
	})
}

// SubscribeBookings subscribes to bookings.state events
func (s *EventSubscriber) SubscribeBookings(handler func(ctx context.Context, booking BookingsStateEvent) error) {
	s.bus.Subscribe("bookings.state", func(ctx context.Context, event Event) error {
		var booking BookingsStateEvent
		if err := json.Unmarshal(event.Data, &booking); err != nil {
			return err
		}
		return handler(ctx, booking)
	})
}

// SubscribeAlerts subscribes to alerts.fired events
func (s *EventSubscriber) SubscribeAlerts(handler func(ctx context.Context, alert AlertsFiredEvent) error) {
	s.bus.Subscribe("alerts.fired", func(ctx context.Context, event Event) error {
		var alert AlertsFiredEvent
		if err := json.Unmarshal(event.Data, &alert); err != nil {
			return err
		}
		return handler(ctx, alert)
	})
}

// Close closes all consumers and producers
func (eb *EventBus) Close() error {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if eb.producer != nil {
		eb.producer.Close()
	}

	for _, reader := range eb.consumers {
		reader.Close()
	}

	eb.connected = false
	return nil
}
