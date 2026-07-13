package models

import "time"

// Location represents a geographic point
type Location struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// CompareRequest is the request to compare rides
type CompareRequest struct {
	From      string   `json:"from"`
	To        string   `json:"to"`
	FromLoc   Location `json:"from_loc"`
	ToLoc     Location `json:"to_loc"`
	Prefs     Prefs   `json:"prefs"`
}

// Prefs contains user preferences
type Prefs struct {
	AC       bool `json:"ac"`
	Saheli   bool `json:"saheli"`   // Women-only transport
	Night    bool `json:"night"`     // Night travel
	Surge    bool `json:"surge"`     // Allow surge pricing
	Wheelchair bool `json:"wheelchair"`
}

// CompareResponse contains all available ride options
type CompareResponse struct {
	Options   []RideOption `json:"options"`
	ExpiresAt time.Time    `json:"expires_at"`
}

// RideOption represents a single ride option
type RideOption struct {
	ID          string   `json:"id"`
	Provider    string   `json:"provider"`
	Mode        string   `json:"mode"`         // bus, metro, auto, cab, bike
	DisplayName string   `json:"display_name"`
	Price       Price    `json:"price"`
	ETA         int      `json:"eta_minutes"`
	DistanceKm  float64  `json:"distance_km"`
	Badges      []string `json:"badges"`       // CHEAPEST, FASTEST, SMART_PICK
	Reliability float64  `json:"reliability"` // 0-1, cancellation rate
	DeepLink    string   `json:"deeplink,omitempty"`
	Bookable    bool     `json:"bookable"`     // Can book directly
}

// Price contains fare information
type Price struct {
	Min       float64 `json:"min"`
	Max       float64 `json:"max"`
	Currency  string  `json:"currency"`
	Breakdown []LineItem `json:"breakdown,omitempty"`
}

// LineItem is a single component of the fare
type LineItem struct {
	Name  string  `json:"name"`
	Amount float64 `json:"amount"`
}

// GeoCodeResult represents geocoding output
type GeoCodeResult struct {
	Location Location `json:"location"`
	Address  string   `json:"address"`
	Name     string   `json:"name"`
	Accuracy string   `json:"accuracy"` // exact, interpolated, geonear
}

// Route represents a computed route
type Route struct {
	From      Location `json:"from"`
	To        Location `json:"to"`
	DistanceKm float64 `json:"distance_km"`
	DurationMin int   `json:"duration_min"`
	Mode      string   `json:"mode"`
	Geometry  string   `json:"geometry,omitempty"` // polyline
	Steps     []RouteStep `json:"steps,omitempty"`
}

// RouteStep is a single segment of a route
type RouteStep struct {
	Mode        string   `json:"mode"`
	Instruction string   `json:"instruction"`
	DistanceKm  float64  `json:"distance_km"`
	DurationMin int      `json:"duration_min"`
}

// ProviderQuote is raw quote from a provider
type ProviderQuote struct {
	Provider string  `json:"provider"`
	Mode     string  `json:"mode"`
	MinPrice float64 `json:"min_price"`
	MaxPrice float64 `json:"max_price"`
	ETA      int     `json:"eta_minutes"`
	Available bool   `json:"available"`
	Surge    float64 `json:"surge,omitempty"`
	Error    string  `json:"error,omitempty"`
}

// BookingRequest to book a ride
type BookingRequest struct {
	OptionID string `json:"option_id"`
	Rail     string `json:"rail"` // ondc, deeplink
}

// BookingResponse from booking
type BookingResponse struct {
	Status    string `json:"status"`
	BookingID string `json:"booking_id"`
	DeepLink  string `json:"deeplink,omitempty"`
	Message   string `json:"message,omitempty"`
}
