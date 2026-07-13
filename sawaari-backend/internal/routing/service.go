package routing

import (
	"context"
	"math"

	"github.com/sawaari/backend/internal/models"
)

// Service provides routing calculations
type Service struct {
	osrmURL string
}

// New creates a new routing service
func New() *Service {
	return &Service{
		osrmURL: "https://router.project-osrm.org",
	}
}

// Route calculates route between two points
func (s *Service) Route(ctx context.Context, from, to models.Location, mode string) (*models.Route, error) {
	distance := haversine(from.Lat, from.Lng, to.Lat, to.Lng)

	var duration int
	switch mode {
	case "car":
		duration = int(distance/25*60) + 5 // ~25 km/h avg in Delhi with traffic
	case "bicycle":
		duration = int(distance/15*60) + 2
	case "pedestrian":
		duration = int(distance/5*60) + 1
	default:
		duration = int(distance/20*60) + 3
	}

	return &models.Route{
		From:        from,
		To:          to,
		DistanceKm:  math.Round(distance*100) / 100,
		DurationMin: duration,
		Mode:        mode,
	}, nil
}

// haversine calculates distance in km between two points
func haversine(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadius = 6371.0

	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	deltaLat := (lat2 - lat1) * math.Pi / 180
	deltaLng := (lng2 - lng1) * math.Pi / 180

	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(deltaLng/2)*math.Sin(deltaLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadius * c
}
