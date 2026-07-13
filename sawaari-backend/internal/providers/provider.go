package providers

import (
	"context"

	"github.com/sawaari/backend/internal/models"
)

// Provider interface for ride providers
type Provider interface {
	Name() string
	Quote(ctx context.Context, from, to models.Location, distanceKm float64) (models.ProviderQuote, error)
	Book(ctx context.Context, req models.BookingRequest, from, to models.Location) (*models.BookingResponse, error)
}

// Uber provider
type Uber struct {
	clientID     string
	clientSecret string
}

// NewUber creates a new Uber provider
func NewUber(clientID, clientSecret string) *Uber {
	return &Uber{clientID: clientID, clientSecret: clientSecret}
}

func (u *Uber) Name() string { return "uber" }

// Quote returns a quote from Uber
func (u *Uber) Quote(ctx context.Context, from, to models.Location, distanceKm float64) (models.ProviderQuote, error) {
	// TODO: Call Uber API
	// Mock implementation
	baseFare := 40.0
	perKm := 8.0

	return models.ProviderQuote{
		Provider: "uber",
		Mode:     "cab",
		MinPrice: baseFare + (perKm * distanceKm * 0.8),
		MaxPrice: baseFare + (perKm * distanceKm * 1.2),
		ETA:      5,
		Available: true,
	}, nil
}

func (u *Uber) Book(ctx context.Context, req models.BookingRequest, from, to models.Location) (*models.BookingResponse, error) {
	return &models.BookingResponse{
		Status:   "deeplink",
		DeepLink: "https://m.uber.com/ul/",
	}, nil
}

// Ola provider
type Ola struct {
	apiKey string
}

// NewOla creates a new Ola provider
func NewOla(apiKey string) *Ola {
	return &Ola{apiKey: apiKey}
}

func (o *Ola) Name() string { return "ola" }

func (o *Ola) Quote(ctx context.Context, from, to models.Location, distanceKm float64) (models.ProviderQuote, error) {
	// TODO: Call Ola API
	baseFare := 35.0
	perKm := 9.0

	return models.ProviderQuote{
		Provider: "ola",
		Mode:     "cab",
		MinPrice: baseFare + (perKm * distanceKm * 0.85),
		MaxPrice: baseFare + (perKm * distanceKm * 1.15),
		ETA:      7,
		Available: true,
	}, nil
}

func (o *Ola) Book(ctx context.Context, req models.BookingRequest, from, to models.Location) (*models.BookingResponse, error) {
	return &models.BookingResponse{
		Status:   "deeplink",
		DeepLink: "https://www.olacabs.com/book",
	}, nil
}

// Rapido provider
type Rapido struct {
	apiKey string
}

// NewRapido creates a new Rapido provider
func NewRapido(apiKey string) *Rapido {
	return &Rapido{apiKey: apiKey}
}

func (r *Rapido) Name() string { return "rapido" }

func (r *Rapido) Quote(ctx context.Context, from, to models.Location, distanceKm float64) (models.ProviderQuote, error) {
	baseFare := 15.0
	perKm := 6.0

	return models.ProviderQuote{
		Provider: "rapido",
		Mode:     "bike",
		MinPrice: baseFare + (perKm * distanceKm * 0.9),
		MaxPrice: baseFare + (perKm * distanceKm * 1.1),
		ETA:      4,
		Available: true,
	}, nil
}

func (r *Rapido) Book(ctx context.Context, req models.BookingRequest, from, to models.Location) (*models.BookingResponse, error) {
	return &models.BookingResponse{
		Status:   "deeplink",
		DeepLink: "https://www.rapido.bike/",
	}, nil
}
