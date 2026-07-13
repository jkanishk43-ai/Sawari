package fare

import (
	"time"

	"github.com/sawaari/backend/internal/models"
)

// Engine computes local fares for regulated transport
type Engine struct {
	tariffs map[string]*Tariff
}

// Tariff defines fare rules for a transport mode
type Tariff struct {
	Mode       string
	BaseFare  float64
	PerKm     float64
	PerMin    float64
	MinFare   float64
	MaxFare   float64
	Currency   string
	SaheliFree bool
}

// New creates a new fare engine with tariffs
func New() *Engine {
	return &Engine{
		tariffs: map[string]*Tariff{
			"bus": {
				Mode:       "bus",
				BaseFare:   5,
				PerKm:      1.5,
				PerMin:     0,
				MinFare:    5,
				MaxFare:    50,
				Currency:   "INR",
				SaheliFree: true,
			},
			"metro": {
				Mode:       "metro",
				BaseFare:   8,
				PerKm:      0.5,
				PerMin:     0,
				MinFare:    8,
				MaxFare:    60,
				Currency:   "INR",
				SaheliFree: true,
			},
			"auto": {
				Mode:       "auto",
				BaseFare:   25,
				PerKm:      10.5, // Rs per km
				PerMin:     1.25, // Rs per minute waiting
				MinFare:    25,
				MaxFare:    300,
				Currency:   "INR",
				SaheliFree: false,
			},
			"meter_auto": {
				Mode:       "meter_auto",
				BaseFare:   25,
				PerKm:      10.5,
				PerMin:     1.25,
				MinFare:    25,
				MaxFare:    250,
				Currency:   "INR",
				SaheliFree: false,
			},
			"e_rickshaw": {
				Mode:       "e_rickshaw",
				BaseFare:   10,
				PerKm:      6,
				PerMin:     0.5,
				MinFare:    10,
				MaxFare:    50,
				Currency:   "INR",
				SaheliFree: false,
			},
		},
	}
}

// ComputeLocalFares calculates fares for regulated transport modes
func (e *Engine) ComputeLocalFares(from, to models.Location, route *models.Route, prefs models.Prefs) []models.RideOption {
	var options []models.RideOption

	km := route.DistanceKm
	eta := route.DurationMin

	// Bus option
	if busTariff, ok := e.tariffs["bus"]; ok {
		fare := e.calculateFare(busTariff, km)
		opt := models.RideOption{
			ID:          "dtc_bus",
			Provider:    "DTC",
			Mode:        "bus",
			DisplayName: "DTC Bus",
			Price: models.Price{
				Min:      fare,
				Max:      fare,
				Currency: "INR",
				Breakdown: []models.LineItem{
					{"Base fare", busTariff.BaseFare},
					{"Distance charge", busTariff.PerKm * km},
				},
			},
			ETA:        eta + 10, // +10 min for wait
			DistanceKm: km,
			Bookable:   false,
			Reliability: 0.75,
		}
		if prefs.Saheli && busTariff.SaheliFree {
			opt.Price.Min = 0
			opt.Price.Max = 0
			opt.Badges = append(opt.Badges, "FREE_SAHELI")
		}
		options = append(options, opt)
	}

	// Metro option
	if metroTariff, ok := e.tariffs["metro"]; ok {
		fare := e.calculateFare(metroTariff, km)
		opt := models.RideOption{
			ID:          "dmrc_metro",
			Provider:    "DMRC",
			Mode:        "metro",
			DisplayName: "Delhi Metro",
			Price: models.Price{
				Min:      fare,
				Max:      fare,
				Currency: "INR",
				Breakdown: []models.LineItem{
					{"Metro fare", fare},
				},
			},
			ETA:        eta + 5, // +5 min for walking to station
			DistanceKm: km,
			Bookable:   true, // ONDC QR tickets
			Reliability: 0.95,
		}
		if prefs.Saheli && metroTariff.SaheliFree {
			opt.Price.Min = 0
			opt.Price.Max = 0
			opt.Badges = append(opt.Badges, "FREE_SAHELI")
		}
		options = append(options, opt)
	}

	// Auto option (meter)
	if autoTariff, ok := e.tariffs["auto"]; ok {
		fare := e.calculateFare(autoTariff, km)
		opt := models.RideOption{
			ID:          "meter_auto",
			Provider:    "Meter",
			Mode:        "auto",
			DisplayName: "Meter Auto",
			Price: models.Price{
				Min:      autoTariff.MinFare,
				Max:      fare,
				Currency: "INR",
				Breakdown: []models.LineItem{
					{"Flag fall", autoTariff.BaseFare},
					{"Per km", autoTariff.PerKm * km},
					{"Night charges (10PM-6AM)", e.nightCharge(autoTariff, prefs)},
				},
			},
			ETA:        eta + 3,
			DistanceKm: km,
			Bookable:   false, // Street hail
			Reliability: 0.70,
		}
		if prefs.Saheli {
			opt.Badges = append(opt.Badges, "SAHELI_GATE")
		}
		options = append(options, opt)
	}

	// E-rickshaw for short distances
	if km < 5 {
		if erTariff, ok := e.tariffs["e_rickshaw"]; ok {
			fare := e.calculateFare(erTariff, km)
			opt := models.RideOption{
				ID:          "e_rickshaw",
				Provider:    "E-Rickshaw",
				Mode:        "e_rickshaw",
				DisplayName: "E-Rickshaw",
				Price: models.Price{
					Min:      fare,
					Max:      fare,
					Currency: "INR",
				},
				ETA:        eta + 2,
				DistanceKm: km,
				Bookable:   false,
				Reliability: 0.65,
			}
			options = append(options, opt)
		}
	}

	return options
}

func (e *Engine) calculateFare(t *Tariff, km float64) float64 {
	fare := t.BaseFare + (t.PerKm * km)
	if fare < t.MinFare {
		fare = t.MinFare
	}
	if fare > t.MaxFare {
		fare = t.MaxFare
	}
	return fare
}

func (e *Engine) nightCharge(t *Tariff, prefs models.Prefs) float64 {
	if !prefs.Night {
		return 0
	}

	hour := time.Now().Hour()
	if hour >= 22 || hour < 6 {
		// 25% night surcharge
		return t.BaseFare * 0.25
	}
	return 0
}
