package geocoding

import (
	"context"
	"sync"

	"github.com/sawaari/backend/internal/models"
)

// Service provides geocoding with caching
type Service struct {
	cache   map[string]*models.GeoCodeResult
	mu      sync.RWMutex
	nominatimURL string
}

// New creates a new geocoding service
func New() *Service {
	return &Service{
		cache:   make(map[string]*models.GeoCodeResult),
		nominatimURL: "https://nominatim.openstreetmap.org",
	}
}

// Geocode converts address to coordinates
func (s *Service) Geocode(ctx context.Context, address string) (*models.GeoCodeResult, error) {
	// Check cache
	s.mu.RLock()
	if cached, ok := s.cache[address]; ok {
		s.mu.RUnlock()
		return cached, nil
	}
	s.mu.RUnlock()

	// TODO: Call Nominatim API
	// For now, return mock data for Delhi
	result := &models.GeoCodeResult{
		Location: models.Location{
			Lat: 28.6139,
			Lng: 77.2090,
		},
		Address:  address,
		Name:    address,
		Accuracy: "geonear",
	}

	// Cache result
	s.mu.Lock()
	s.cache[address] = result
	s.mu.Unlock()

	return result, nil
}

// ReverseGeocode converts coordinates to address
func (s *Service) ReverseGeocode(ctx context.Context, lat, lng float64) (*models.GeoCodeResult, error) {
	// Check cache
	key := string(rune(int(lat*100))) + "," + string(rune(int(lng*100)))
	s.mu.RLock()
	if cached, ok := s.cache[key]; ok {
		s.mu.RUnlock()
		return cached, nil
	}
	s.mu.RUnlock()

	// TODO: Call Nominatim Reverse Geocoding API
	result := &models.GeoCodeResult{
		Location: models.Location{Lat: lat, Lng: lng},
		Address:  "Delhi, India",
		Name:    "Location",
		Accuracy: "geonear",
	}

	s.mu.Lock()
	s.cache[key] = result
	s.mu.Unlock()

	return result, nil
}
