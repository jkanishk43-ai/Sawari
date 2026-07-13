package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// NotificationHub manages all notification channels
type NotificationHub struct {
	mu       sync.RWMutex
	handlers map[string]NotificationHandler
}

// NotificationHandler sends notifications via a specific channel
type NotificationHandler interface {
	Send(ctx context.Context, notification Notification) (*DeliveryResult, error)
	ValidateRecipient(recipient string) bool
}

// Notification represents a notification payload
type Notification struct {
	ID        string            `json:"id"`
	Channel   string            `json:"channel"` // fcm, whatsapp, telegram, sms
	UserID    string            `json:"user_id"`
	Recipient string            `json:"recipient"` // phone, token, chat_id
	Subject   string            `json:"subject,omitempty"`
	Body      string            `json:"body"`
	Template  string            `json:"template,omitempty"`
	Data      map[string]string `json:"data,omitempty"`
	Priority  string            `json:"priority"` // high, normal
	Timestamp time.Time         `json:"timestamp"`
}

// DeliveryResult tracks delivery status
type DeliveryResult struct {
	NotificationID string        `json:"notification_id"`
	Provider       string        `json:"provider"`
	Status         string        `json:"status"` // delivered, failed, pending, bounced
	ProviderMsgID  string        `json:"provider_msg_id,omitempty"`
	Error          string        `json:"error,omitempty"`
	Latency        time.Duration `json:"latency"`
}

// NewNotificationHub creates a notification hub
func NewNotificationHub() *NotificationHub {
	hub := &NotificationHub{
		handlers: make(map[string]NotificationHandler),
	}

	// Register default handlers
	hub.handlers["fcm"] = NewFCMHandler()
	hub.handlers["whatsapp"] = NewWhatsAppHandler()
	hub.handlers["telegram"] = NewTelegramHandler()

	return hub
}

// Send dispatches a notification through the appropriate channel
func (h *NotificationHub) Send(ctx context.Context, notification Notification) (*DeliveryResult, error) {
	handler, ok := h.handlers[notification.Channel]
	if !ok {
		return &DeliveryResult{
			NotificationID: notification.ID,
			Status:         "failed",
			Error:          fmt.Sprintf("unsupported channel: %s", notification.Channel),
		}, fmt.Errorf("unsupported channel: %s", notification.Channel)
	}

	start := time.Now()
	result, err := handler.Send(ctx, notification)
	if result != nil {
		result.Latency = time.Since(start)
	}
	return result, err
}

// SendMulticast sends the same notification to multiple recipients
func (h *NotificationHub) SendMulticast(ctx context.Context, notification Notification, recipients []string) []*DeliveryResult {
	results := make([]*DeliveryResult, len(recipients))
	var wg sync.WaitGroup

	for i, recipient := range recipients {
		wg.Add(1)
		go func(idx int, rec string) {
			defer wg.Done()
			n := notification
			n.Recipient = rec
			results[idx], _ = h.Send(ctx, n)
		}(i, recipient)
	}

	wg.Wait()
	return results
}

// FCM (Firebase Cloud Messaging) Handler

// FCMHandler sends push notifications via Firebase
type FCMHandler struct {
	serverKey  string
	senderID  string
	endpoint  string
	httpClient *http.Client
}

func NewFCMHandler() *FCMHandler {
	return &FCMHandler{
		serverKey: "",
		endpoint:  "https://fcm.googleapis.com/fcm/send",
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Configure sets FCM credentials
func (f *FCMHandler) Configure(serverKey, senderID string) {
	f.serverKey = serverKey
	f.senderID = senderID
}

func (f *FCMHandler) ValidateRecipient(recipient string) bool {
	// FCM tokens are typically long strings
	return len(recipient) > 10
}

func (f *FCMHandler) Send(ctx context.Context, notification Notification) (*DeliveryResult, error) {
	payload := map[string]interface{}{
		"to": notification.Recipient,
		"data": map[string]string{
			"id":          notification.ID,
			"channel":     notification.Channel,
			"template":    notification.Template,
			"body":        notification.Body,
			"subject":     notification.Subject,
		},
		"notification": map[string]interface{}{
			"title": notification.Subject,
			"body":  notification.Body,
			"sound": "default",
		},
	}

	// Merge custom data
	for k, v := range notification.Data {
		payload["data"].(map[string]string)[k] = v
	}

	body, _ := jsonMarshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.endpoint, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "key="+f.serverKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return &DeliveryResult{
			NotificationID: notification.ID,
			Status:         "failed",
			Error:          err.Error(),
		}, nil
	}
	defer resp.Body.Close()

	respBody, _ := ioReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return &DeliveryResult{
			NotificationID: notification.ID,
			Status:         "failed",
			Error:          string(respBody),
		}, nil
	}

	return &DeliveryResult{
		NotificationID: notification.ID,
		Provider:       "fcm",
		Status:         "delivered",
		ProviderMsgID:  string(respBody),
	}, nil
}

// FCMBatch sends to multiple FCM tokens (up to 500 per request)
func (f *FCMHandler) Batch(ctx context.Context, tokens []string, title, body string, data map[string]string) (*DeliveryResult, error) {
	registrationIDs := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if f.ValidateRecipient(t) {
			registrationIDs = append(registrationIDs, t)
		}
	}

	payload := map[string]interface{}{
		"registration_ids": registrationIDs,
		"priority":         "high",
		"notification": map[string]interface{}{
			"title": title,
			"body":  body,
		},
		"data": data,
	}

	body, _ := jsonMarshal(payload)
	return f.sendRaw(ctx, body)
}

func (f *FCMHandler) sendRaw(ctx context.Context, body []byte) (*DeliveryResult, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, f.endpoint, strings.NewReader(string(body)))
	req.Header.Set("Authorization", "key="+f.serverKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := ioReadAll(resp.Body)
	return &DeliveryResult{
		Provider:      "fcm",
		Status:        "delivered",
		ProviderMsgID: string(respBody),
	}, nil
}

// WhatsApp Handler

// WhatsAppHandler sends messages via WhatsApp Business Cloud API
type WhatsAppHandler struct {
	phoneNumberID string
	accessToken   string
	baseURL       string
	httpClient    *http.Client
}

func NewWhatsAppHandler() *WhatsAppHandler {
	return &WhatsAppHandler{
		baseURL:    "https://graph.facebook.com/v19.0",
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// Configure sets WhatsApp credentials
func (w *WhatsAppHandler) Configure(phoneNumberID, accessToken string) {
	w.phoneNumberID = phoneNumberID
	w.accessToken = accessToken
}

func (w *WhatsAppHandler) ValidateRecipient(recipient string) bool {
	// E.164 format: +[country][number]
	return len(recipient) >= 12 && strings.HasPrefix(recipient, "+")
}

func (w *WhatsAppHandler) Send(ctx context.Context, notification Notification) (*DeliveryResult, error) {
	// Find template or use text
	templateName := notification.Template
	if templateName == "" {
		templateName = "sawaari_notification"
	}

	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"to":                strings.TrimPrefix(notification.Recipient, "+"),
		"type":              "template",
		"template": map[string]interface{}{
			"name": templateName,
			"language": map[string]string{
				"policy": "deterministic",
				"code":   "en",
			},
			"components": []map[string]interface{}{
				{
					"type": "body",
					"parameters": []map[string]string{
						{"type": "text", "text": notification.Subject},
						{"type": "text", "text": notification.Body},
					},
				},
			},
		},
	}

	// If template isn't configured, send as text
	if notification.Template == "" {
		payload["type"] = "text"
		payload["text"] = map[string]string{"body": notification.Body}
		delete(payload, "template")
	}

	body, _ := jsonMarshal(payload)
	url := fmt.Sprintf("%s/%s/messages", w.baseURL, w.phoneNumberID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+w.accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return &DeliveryResult{
			NotificationID: notification.ID,
			Status:         "failed",
			Error:          err.Error(),
		}, nil
	}
	defer resp.Body.Close()

	respBody, _ := ioReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return &DeliveryResult{
			NotificationID: notification.ID,
			Status:         "failed",
			Error:          string(respBody),
		}, nil
	}

	return &DeliveryResult{
		NotificationID: notification.ID,
		Provider:       "whatsapp",
		Status:         "delivered",
		ProviderMsgID:  string(respBody),
	}, nil
}

// WhatsAppTemplates defines reusable templates
var WhatsAppTemplates = map[string]string{
	"booking_confirmed":   "booking_confirmed",
	"ride_eta_update":     "ride_eta_update",
	"ticket_ready":        "ticket_ready",
	"fare_alert":          "fare_alert",
	"disruption_alert":    "disruption_alert",
	"feedback_request":    "feedback_request",
	"saheli_feature":      "saheli_feature",
	"welcome":             "welcome",
}

// Telegram Handler

// TelegramHandler sends messages via Telegram Bot API
type TelegramHandler struct {
	token    string
	baseURL  string
	httpClient *http.Client
}

func NewTelegramHandler() *TelegramHandler {
	return &TelegramHandler{
		baseURL:    "https://api.telegram.org",
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (t *TelegramHandler) Configure(token string) {
	t.token = token
}

func (t *TelegramHandler) ValidateRecipient(recipient string) bool {
	// Telegram chat IDs are numeric
	return len(recipient) > 5
}

func (t *TelegramHandler) Send(ctx context.Context, notification Notification) (*DeliveryResult, error) {
	chatID := notification.Recipient

	payload := map[string]interface{}{
		"chat_id": chatID,
		"text":    notification.Body,
	}

	// Use parse_mode for formatting
	if strings.Contains(notification.Body, "*") {
		payload["parse_mode"] = "Markdown"
	}

	body, _ := jsonMarshal(payload)
	url := fmt.Sprintf("%s/bot%s/sendMessage", t.baseURL, t.token)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return &DeliveryResult{
			NotificationID: notification.ID,
			Status:         "failed",
			Error:          err.Error(),
		}, nil
	}
	defer resp.Body.Close()

	respBody, _ := ioReadAll(resp.Body)

	var telegramResp struct {
		OK          bool `json:"ok"`
		Result struct {
			MessageID int `json:"message_id"`
		} `json:"result"`
	}

	_ = json.Unmarshal(respBody, &telegramResp)

	if !telegramResp.OK {
		return &DeliveryResult{
			NotificationID: notification.ID,
			Status:         "failed",
			Error:          "telegram API error",
		}, nil
	}

	return &DeliveryResult{
		NotificationID: notification.ID,
		Provider:       "telegram",
		Status:         "delivered",
		ProviderMsgID:  fmt.Sprintf("%d", telegramResp.Result.MessageID),
	}, nil
}

// SendBookingConfirmation sends a booking confirmation across channels
func (h *NotificationHub) SendBookingConfirmation(ctx context.Context, userID, bookingID, destination string, eta int, channels []string) []*DeliveryResult {
	results := make([]*DeliveryResult, 0, len(channels))

	for _, ch := range channels {
		notification := Notification{
			ID:        generateNotificationID(),
			Channel:   ch,
			UserID:    userID,
			Subject:   "Booking Confirmed",
			Body:      fmt.Sprintf("Your ride to %s is confirmed. Driver is %d minutes away.", destination, eta),
			Template:  "booking_confirmed",
			Priority:  "high",
			Timestamp: time.Now(),
			Data: map[string]string{
				"booking_id": bookingID,
				"eta":        fmt.Sprintf("%d", eta),
				"destination": destination,
			},
		}

		result, _ := h.Send(ctx, notification)
		results = append(results, result)
	}

	return results
}

// SendFareAlert sends fare price alerts
func (h *NotificationHub) SendFareAlert(ctx context.Context, userID, from, to string, newFare, oldFare float64, channels []string) []*DeliveryResult {
	pctChange := ((newFare - oldFare) / oldFare) * 100
	direction := "dropped"
	if pctChange > 0 {
		direction = "increased"
	}

	body := fmt.Sprintf("Fare for %s → %s has %s by %.1f%%: ₹%.0f (was ₹%.0f).",
		from, to, direction, pctChange, newFare, oldFare)

	results := make([]*DeliveryResult, 0, len(channels))
	for _, ch := range channels {
		notification := Notification{
			ID:        generateNotificationID(),
			Channel:   ch,
			UserID:    userID,
			Subject:   fmt.Sprintf("Fare %s: %s", direction, from),
			Body:      body,
			Template:  "fare_alert",
			Priority:  "normal",
			Timestamp: time.Now(),
			Data: map[string]string{
				"from":     from,
				"to":       to,
				"new_fare": fmt.Sprintf("%.0f", newFare),
				"old_fare": fmt.Sprintf("%.0f", oldFare),
				"pct_change": fmt.Sprintf("%.1f", pctChange),
			},
		}
		result, _ := h.Send(ctx, notification)
		results = append(results, result)
	}
	return results
}

// SendDisruptionAlert sends service disruption notifications
func (h *NotificationHub) SendDisruptionAlert(ctx context.Context, userID, line, disruption string, channels []string) []*DeliveryResult {
	body := fmt.Sprintf("Disruption on %s: %s. Check Sawaari for alternatives.", line, disruption)

	results := make([]*DeliveryResult, 0, len(channels))
	for _, ch := range channels {
		notification := Notification{
			ID:        generateNotificationID(),
			Channel:   ch,
			UserID:    userID,
			Subject:   fmt.Sprintf("Service Alert: %s", line),
			Body:      body,
			Template:  "disruption_alert",
			Priority:  "high",
			Timestamp: time.Now(),
			Data: map[string]string{
				"line":        line,
				"disruption":  disruption,
			},
		}
		result, _ := h.Send(ctx, notification)
		results = append(results, result)
	}
	return results
}

func generateNotificationID() string {
	return fmt.Sprintf("ntf_%d", time.Now().UnixNano())
}

func jsonMarshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func ioReadAll(r io.Reader) ([]byte, error) {
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
