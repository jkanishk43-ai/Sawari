package users

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"github.com/sawaari/backend/internal/cache"
)

// UserService manages user profiles, preferences, and trip history
type UserService struct {
	cache *cache.ValkeyCache
	users map[string]*User
	trips map[string][]TripRecord
	mu    sync.RWMutex
}

// User represents a user account
type User struct {
	ID           string     `json:"id"`
	Phone        string     `json:"phone"`
	PhoneHash    string     `json:"phone_hash,omitempty"` // For de-identified lookups
	DisplayName  string     `json:"display_name"`
	Email        string     `json:"email,omitempty"`
	PhotoURL     string     `json:"photo_url,omitempty"`
	SaheliFlag   bool       `json:"saheli_flag"`      // Women-only transport preference
	ACPreference bool       `json:"ac_preference"`
	NightMode   bool       `json:"night_mode"`
	Wheelchair  bool       `json:"wheelchair"`
	PreferredCurrency string `json:"preferred_currency"`
	Language    string     `json:"language"`
	TimeZone    string     `json:"timezone"`
	FCMTokens   []string   `json:"fcm_tokens"`
	TelegramID  string     `json:"telegram_id,omitempty"`
	WhatsAppID  string     `json:"whatsapp_id,omitempty"`
	SaheliCard  *SaheliCard `json:"saheli_card,omitempty"`
	Settings    *UserSettings `json:"settings"`
	CreatedAt   time.Time  `json:"created_at"`
	LastActive  time.Time  `json:"last_active"`
	Status      string     `json:"status"` // active, suspended, deleted
}

// SaheliCard tracks women's transport card details
type SaheliCard struct {
	CardNumber  string    `json:"card_number"`
	IssuedBy    string    `json:"issued_by"`    // DTC, DMRC
	IssuedAt    time.Time `json:"issued_at"`
	ValidUntil  time.Time `json:"valid_until"`
	Category    string    `json:"category"`     // student, working, senior
	LinkedID    string    `json:"linked_id,omitempty"`
	QRCode      string    `json:"qr_code,omitempty"`
}

// UserSettings stores user preferences
type UserSettings struct {
	DefaultPaymentMethod string   `json:"default_payment_method"`
	AutoBookMode        bool     `json:"auto_book_mode"`
	ShareTripWithTrusted bool     `json:"share_trip_with_trusted"`
	TrustedContacts     []string `json:"trusted_contacts"`
	NotificationPrefs   NotificationPrefs `json:"notification_prefs"`
	Theme               string   `json:"theme"`
	SavedPlaces         []SavedPlace `json:"saved_places"`
}

// NotificationPrefs
type NotificationPrefs struct {
	FCMEnabled       bool `json:"fcm_enabled"`
	WhatsAppEnabled  bool `json:"whatsapp_enabled"`
	TelegramEnabled  bool `json:"telegram_enabled"`
	FareAlerts       bool `json:"fare_alerts"`
	DisruptionAlerts bool `json:"disruption_alerts"`
	BookingConfirm   bool `json:"booking_confirm"`
}

// SavedPlace is a frequently used location
type SavedPlace struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Address string  `json:"address"`
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
	Tag     string  `json:"tag,omitempty"` // home, work, school
}

// TripRecord stores trip history
type TripRecord struct {
	ID              string        `json:"id"`
	UserID          string        `json:"user_id"`
	From            SavedPlace    `json:"from"`
	To              SavedPlace    `json:"to"`
	Provider        string        `json:"provider"`
	Mode            string        `json:"mode"`
	Fare            float64       `json:"fare"`
	Currency        string        `json:"currency"`
	DurationMinutes int           `json:"duration_minutes"`
	DistanceKm      float64       `json:"distance_km"`
	BookingID       string        `json:"booking_id,omitempty"`
	TicketID        string        `json:"ticket_id,omitempty"`
	StartedAt       time.Time     `json:"started_at"`
	EndedAt         time.Time     `json:"ended_at,omitempty"`
	Feedback        *TripFeedback `json:"feedback,omitempty"`
	Tags            []string      `json:"tags,omitempty"`
}

// TripFeedback stores user feedback for a trip
type TripFeedback struct {
	Rating      int    `json:"rating"`       // 1-5
	Comment     string `json:"comment,omitempty"`
	ActualFare  float64 `json:"actual_fare,omitempty"`
	Wheelchair  bool   `json:"wheelchair_used,omitempty"`
	RideSmoothness int `json:"ride_smoothness"` // 1-5
}

// NewUserService creates a new user service
func NewUserService(c *cache.ValkeyCache) *UserService {
	return &UserService{
		cache: c,
		users: make(map[string]*User),
		trips: make(map[string][]TripRecord),
	}
}

// CreateUser creates a new user profile
func (s *UserService) CreateUser(ctx context.Context, phone, displayName string) (*User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	phoneHash := hashPhone(phone)
	userID := generateUserID(phoneHash)

	// Check if user exists
	if _, exists := s.users[userID]; exists {
		return nil, fmt.Errorf("user already exists: %s", userID)
	}

	user := &User{
		ID:          userID,
		Phone:       phone,
		PhoneHash:   phoneHash,
		DisplayName: displayName,
		Status:      "active",
		Language:    "en",
		TimeZone:    "Asia/Kolkata",
		Settings: &UserSettings{
			Theme:     "system",
			FareAlerts: true,
			DisruptionAlerts: true,
			BookingConfirm: true,
			NotificationPrefs: NotificationPrefs{
				FCMEnabled:      true,
				WhatsAppEnabled: true,
				TelegramEnabled: false,
				FareAlerts:      true,
				DisruptionAlerts: true,
				BookingConfirm:  true,
			},
		},
		CreatedAt:  time.Now(),
		LastActive: time.Now(),
	}

	s.users[userID] = user
	s.trips[userID] = []TripRecord{}

	// Cache the user
	key := fmt.Sprintf("user:%s", userID)
	s.cache.Set(ctx, key, 24*time.Hour, user)

	return user, nil
}

// GetUser retrieves a user by ID
func (s *UserService) GetUser(ctx context.Context, userID string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check cache first
	key := fmt.Sprintf("user:%s", userID)
	var user User
	if err := s.cache.Get(ctx, key, &user); err == nil {
		return &user, nil
	}

	// Fallback to map
	userPtr, exists := s.users[userID]
	if !exists {
		return nil, fmt.Errorf("user not found: %s", userID)
	}

	return userPtr, nil
}

// GetUserByPhone retrieves a user by phone number
func (s *UserService) GetUserByPhone(ctx context.Context, phone string) (*User, error) {
	hash := hashPhone(phone)

	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, user := range s.users {
		if user.PhoneHash == hash {
			return user, nil
		}
	}

	return nil, fmt.Errorf("user not found for phone")
}

// UpdateUser updates user profile
func (s *UserService) UpdateUser(ctx context.Context, userID string, updates UserUpdate) (*User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.users[userID]
	if !exists {
		return nil, fmt.Errorf("user not found: %s", userID)
	}

	if updates.DisplayName != nil {
		user.DisplayName = *updates.DisplayName
	}
	if updates.Email != nil {
		user.Email = *updates.Email
	}
	if updates.PhotoURL != nil {
		user.PhotoURL = *updates.PhotoURL
	}
	if updates.SaheliFlag != nil {
		user.SaheliFlag = *updates.SaheliFlag
	}
	if updates.ACPreference != nil {
		user.ACPreference = *updates.ACPreference
	}
	if updates.NightMode != nil {
		user.NightMode = *updates.NightMode
	}
	if updates.Wheelchair != nil {
		user.Wheelchair = *updates.Wheelchair
	}
	if updates.Language != nil {
		user.Language = *updates.Language
	}
	if updates.Settings != nil {
		user.Settings = updates.Settings
	}

	user.LastActive = time.Now()

	// Update cache
	key := fmt.Sprintf("user:%s", userID)
	s.cache.Set(ctx, key, 24*time.Hour, user)

	return user, nil
}

// UserUpdate defines which fields can be updated
type UserUpdate struct {
	DisplayName   *string         `json:"display_name,omitempty"`
	Email         *string         `json:"email,omitempty"`
	PhotoURL      *string         `json:"photo_url,omitempty"`
	SaheliFlag    *bool           `json:"saheli_flag,omitempty"`
	ACPreference  *bool           `json:"ac_preference,omitempty"`
	NightMode     *bool           `json:"night_mode,omitempty"`
	Wheelchair    *bool           `json:"wheelchair,omitempty"`
	Language      *string         `json:"language,omitempty"`
	Settings      *UserSettings   `json:"settings,omitempty"`
}

// Prefs returns the user's preference set
func (u *User) Prefs() UserPrefs {
	return UserPrefs{
		Saheli:   u.SaheliFlag,
		AC:       u.ACPreference,
		Night:    u.NightMode,
		Wheelchair: u.Wheelchair,
	}
}

// UserPrefs is the ride-preference subset
type UserPrefs struct {
	Saheli     bool
	AC         bool
	Night      bool
	Wheelchair bool
}

// AddSavedPlace saves a frequently used location
func (s *UserService) AddSavedPlace(ctx context.Context, userID string, place SavedPlace) error {
	user, err := s.GetUser(ctx, userID)
	if err != nil {
		return err
	}

	place.ID = generatePlaceID()
	user.Settings.SavedPlaces = append(user.Settings.SavedPlaces, place)

	return s.UpdateUser(ctx, userID, UserUpdate{
		Settings: user.Settings,
	})
}

// RecordTrip adds a trip to the user's history
func (s *UserService) RecordTrip(ctx context.Context, trip *TripRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	userTrips := s.trips[trip.UserID]
	trip.ID = generateTripID()
	trip.StartedAt = time.Now()
	trip.EndedAt = time.Now()

	userTrips = append(userTrips, *trip)
	s.trips[trip.UserID] = userTrips

	// Cache recent trip
	key := fmt.Sprintf("trip:%s", trip.ID)
	s.cache.Set(ctx, key, 7*24*time.Hour, trip)

	return nil
}

// GetTripHistory returns the user's recent trips
func (s *UserService) GetTripHistory(ctx context.Context, userID string, limit int) ([]TripRecord, error) {
	if limit == 0 {
		limit = 20
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	userTrips := s.trips[userID]
	if userTrips == nil {
		return []TripRecord{}, nil
	}

	// Return most recent trips
	start := len(userTrips) - limit
	if start < 0 {
		start = 0
	}
	if start >= len(userTrips) {
		return []TripRecord{}, nil
	}

	result := userTrips[start:]
	return result, nil
}

// SubmitFeedback records feedback for a trip
func (s *UserService) SubmitFeedback(ctx context.Context, tripID string, feedback *TripFeedback) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Find the trip
	for userID, trips := range s.trips {
		for i := range trips {
			if trips[i].ID == tripID {
				trips[i].Feedback = feedback
				s.trips[userID] = trips

				// Cache feedback
				key := fmt.Sprintf("feedback:%s", tripID)
				s.cache.Set(context.Background(), key, 7*24*time.Hour, feedback)

				return nil
			}
		}
	}

	return fmt.Errorf("trip not found: %s", tripID)
}

// GetSaheliStats returns women's transport usage statistics
func (s *UserService) GetSaheliStats(ctx context.Context, userID string) (*SaheliStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &SaheliStats{
		UserID: userID,
	}

	userTrips := s.trips[userID]
	for _, trip := range userTrips {
		if trip.Mode == "bus" || trip.Mode == "metro" {
			stats.FreeRides++
		}
		if trip.StartedAt.Before(time.Now().Add(-30 * 24 * time.Hour)) {
			stats.LastUsedSaheli = &trip.StartedAt
		}
	}

	stats.TotalRides = len(userTrips)
	stats.SavingsINR = float64(stats.FreeRides) * 25 // avg ₹25 per ride

	return stats, nil
}

// SaheliStats tracks women's transport usage
type SaheliStats struct {
	UserID           string    `json:"user_id"`
	TotalRides       int       `json:"total_rides"`
	FreeRides        int       `json:"free_rides"`
	SavingsINR       float64   `json:"savings_inr"`
	LastUsedSaheli   *time.Time `json:"last_used_saheli,omitempty"`
	CardStatus       string    `json:"card_status"` // not_issued, active, expired
}

// GetUserPreferences returns quick preferences for ride comparison
func (s *UserService) GetUserPreferences(ctx context.Context, userID string) (*UserPrefs, error) {
	user, err := s.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	prefs := user.Prefs()
	return &prefs, nil
}

func hashPhone(phone string) string {
	hash := sha256.Sum256([]byte(phone))
	return base64.StdEncoding.EncodeToString(hash[:])
}

func generateUserID(phoneHash string) string {
	hash := sha256.Sum256([]byte("user:" + phoneHash))
	return "usr_" + base64.URLEncoding.EncodeToString(hash[:8])
}

func generatePlaceID() string {
	return fmt.Sprintf("plc_%d", time.Now().UnixNano())
}

func generateTripID() string {
	return fmt.Sprintf("trip_%d", time.Now().UnixNano())
}
