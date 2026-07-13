package routing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sawaari/backend/internal/models"
)

// ValhallaClient interfaces with Valhalla routing engine
type ValhallaClient struct {
	baseURL    string
	httpClient *http.Client
	apiKey     string
}

// Valhalla API request/response types
type valhallaRouteReq struct {
	Locations     []valhallaLoc `json:"locations"`
	Costing       string        `json:"costing"`
	CostingOptions json.RawMessage `json:"costing_options,omitempty"`
	Units         string        `json:"units,omitempty"`
	Language      string        `json:"language,omitempty"`
	DateTime      struct {
		Type int    `json:"type"` // 0=current, 1=departure, 2=arrival
		Value string `json:"value"`
	} `json:"date_time,omitempty"`
}

type valhallaLoc struct {
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
	Type   int     `json:"type"` // 0=break, 1=through, 2=via
	Side   string  `json:"side,omitempty"`
	Head   int     `json:"head,omitempty"`
	Rank   int     `json:"rank,omitempty"`
}

type valhallaRouteResp struct {
	trip struct {
		Locations []struct {
			Lat float64 `json:"lat"`
			Lng float64 `json:"lng"`
			Type string `json:"type"`
		} `json:"locations"`
		Summary struct {
			HasTime       bool    `json:"has_time"`
			HasDistance   bool    `json:"has_distance"`
			BaseTime      int     `json:"base_time"`
			TotalTime     int     `json:"total_time"`
			TotalDistance float64 `json:"total_distance"` // meters
		} `json:"summary"`
		Legs []struct {
			Summary struct {
				HasTime       bool    `json:"has_time"`
				HasDistance   bool    `json:"has_distance"`
				TotalTime     int     `json:"total_time"` // seconds
				TotalDistance float64 `json:"total_distance"` // meters
			} `json:"summary"`
			Shapes []string `json:"shape"`
			Maneuvers []struct {
				Type         int     `json:"type"`
				Instruction  string  `json:"instruction"`
				StreetName   string  `json:"street_name"`
				Length       float64 `json:"length"` // km
				Time         int     `json:"time"`   // seconds
				BeginShapeIndex int  `json:"begin_shape_index"`
				EndShapeIndex   int  `json:"end_shape_index"`
				Lat          float64 `json:"lat"`
				Lon          float64 `json:"lon"`
			} `json:"maneuvers"`
		} `json:"legs"`
		Status      int    `json:"status"`
		StatusMsg   string `json:"status_message"`
	} `json:"trip"`
}

// Maneuver types from Valhalla
const (
	ManeuverStart          = 1
	ManeuverDestination    = 2
	ManeuverTurnLeft       = 3
	ManeuverTurnRight      = 4
	ManeuverContinue       = 5
	ManeuverUturn          = 6
	ManeuverMerge          = 8
	ManeuverRampLeft       = 9
	ManeuverRampRight      = 10
	ManeuverStation        = 14
	ManeuverRoundabout     = 15
	ManeuverFerryEnter     = 16
	ManeuverFerryExit      = 17
	ManeuverTransitStart   = 30
	ManeuverTransitEnd     = 31
	ManeuverTransitRemain  = 32
	ManeuverTransitTransfer = 33
)

// Profile costing options
const (
	CostingAuto       = "auto"
	CostingAutoDist   = "auto_data_fix"
	CostingBicycle    = "bicycle"
	CostingBus        = "bus"
	CostingHov        = "hov"
	CostingMotorScooter = "motor_scooter"
	CostingMotorcycle = "motorcycle"
	CostingPedestrian = "pedestrian"
	CostingTruck      = "truck"
)

// NewValhalla creates a new Valhalla client
func NewValhalla(baseURL string, apiKey string) *ValhallaClient {
	if baseURL == "" {
		baseURL = "http://localhost:8002"
	}
	return &ValhallaClient{
		baseURL: baseURL,
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// RouteResult contains the computed route
type RouteResult struct {
	DistanceKm  float64
	DurationSec int
	Geometry     string
	Maneuvers    []Maneuver
	Summary      string
}

// Maneuver represents a turn-by-turn instruction
type Maneuver struct {
	Instruction string
	DistanceKm  float64
	DurationSec int
	Lat         float64
	Lon         float64
}

// Route computes a route between two points using specified profile
func (v *ValhallaClient) Route(ctx context.Context, from, to models.Location, profile string) (*RouteResult, error) {
	req := valhallaRouteReq{
		Locations: []valhallaLoc{
			{Lat: from.Lat, Lon: from.Lng, Type: 0}, // break (origin)
			{Lat: to.Lat, Lon: to.Lng, Type: 0},     // break (destination)
		},
		Costing: profile,
		Units:   "km",
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/route", v.baseURL)
	if v.apiKey != "" {
		url += "?api_key=" + v.apiKey
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "Sawaari/1.0")

	resp, err := v.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("Valhalla request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Valhalla returned %d: %s", resp.StatusCode, string(respBody))
	}

	var valhallaResp valhallaRouteResp
	if err := json.NewDecoder(resp.Body).Decode(&valhallaResp); err != nil {
		return nil, fmt.Errorf("failed to decode Valhalla response: %w", err)
	}

	if valhallaResp.trip.Status != 0 {
		return nil, fmt.Errorf("routing failed: %s", valhallaResp.trip.StatusMsg)
	}

	return v.parseResponse(&valhallaResp), nil
}

// RouteWithIntermediate adds waypoints for multi-stop routes
func (v *ValhallaClient) RouteWithIntermediate(ctx context.Context, locations []models.Location, profile string) (*RouteResult, error) {
	if len(locations) < 2 {
		return nil, fmt.Errorf("at least 2 locations required")
	}

	vlocs := make([]valhallaLoc, len(locations))
	for i, loc := range locations {
		locType := 0 // break
		if i > 0 && i < len(locations)-1 {
			locType = 2 // via
		}
		vlocs[i] = valhallaLoc{Lat: loc.Lat, Lon: loc.Lng, Type: locType}
	}

	req := valhallaRouteReq{
		Locations: vlocs,
		Costing:   profile,
		Units:     "km",
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/route", v.baseURL)
	if v.apiKey != "" {
		url += "?api_key=" + v.apiKey
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := v.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var valhallaResp valhallaRouteResp
	if err := json.NewDecoder(resp.Body).Decode(&valhallaResp); err != nil {
		return nil, err
	}

	return v.parseResponse(&valhallaResp), nil
}

// Isochrone computes reachability areas from a point
func (v *ValhallaClient) Isochrone(ctx context.Context, lat, lng float64, profile string, minutes []int) (*IsochroneResult, error) {
	req := map[string]interface{}{
		"locations": []valhallaLoc{
			{Lat: lat, Lon: lng, Type: 0},
		},
		"costing":     profile,
		"contours":    buildContours(minutes),
		"generalize":   0.0,
		"show_locations": false,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/isochrone", v.baseURL)
	if v.apiKey != "" {
		url += "?api_key=" + v.apiKey
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := v.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var isoResp struct {
		Type        string `json:"type"`
		Features    []struct {
			Type     string `json:"type"`
			Geometry struct {
				Type        string        `json:"type"`
				Coordinates json.RawMessage `json:"coordinates"`
			} `json:"geometry"`
			Properties struct {
				Contour  float64 `json:"contour"`
				Distance float64 `json:"distance"`
				Duration float64 `json:"duration"`
			} `json:"properties"`
		} `json:"features"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&isoResp); err != nil {
		return nil, err
	}

	result := &IsochroneResult{
		Contours: make([]Contour, len(isoResp.Features)),
	}

	for i, f := range isoResp.Features {
		result.Contours[i] = Contour{
			Minutes:   f.Properties.Duration / 60,
			DistanceKm: f.Properties.Distance / 1000,
			GeoJSON:   fmt.Sprintf(`{"type":"Feature","geometry":%s}`, f.Geometry.Coordinates),
		}
	}

	return result, nil
}

// IsochroneResult holds isochrone computation results
type IsochroneResult struct {
	Contours []Contour
}

// Contour represents a time/distance contour
type Contour struct {
	Minutes   float64
	DistanceKm float64
	GeoJSON   string
}

// Matrix computes a cost matrix between multiple origins and destinations
func (v *ValhallaClient) Matrix(ctx context.Context, sources, targets []models.Location, profile string) (*MatrixResult, error) {
	allLocs := append(sources, targets...)
	vlocs := make([]valhallaLoc, len(allLocs))
	for i, loc := range allLocs {
		vlocs[i] = valhallaLoc{Lat: loc.Lat, Lon: loc.Lng, Type: 1} // through
	}

	// Mark sources and targets
	sourceIndices := make([]int, len(sources))
	for i := range sources {
		sourceIndices[i] = i
	}
	targetIndices := make([]int, len(targets))
	for i := range targets {
		targetIndices[i] = len(sources) + i
	}

	req := map[string]interface{}{
		"locations":      vlocs,
		"costing":        profile,
		"units":          "km",
		"sources":        sourceIndices,
		"targets":        targetIndices,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/sources_to_targets", v.baseURL)
	if v.apiKey != "" {
		url += "?api_key=" + v.apiKey
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := v.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var matrixResp struct {
		sources_to_targets [][]struct {
			Distance float64 `json:"distance"` // km
			Time     int     `json:"time"`     // seconds
		} `json:"sources_to_targets"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&matrixResp); err != nil {
		return nil, err
	}

	result := &MatrixResult{
		Distances: make([][]float64, len(sources)),
		Times:     make([][]int, len(sources)),
	}

	for i, row := range matrixResp.sources_to_targets {
		result.Distances[i] = make([]float64, len(targets))
		result.Times[i] = make([]int, len(targets))
		for j, cell := range row {
			result.Distances[i][j] = cell.Distance
			result.Times[i][j] = cell.Time
		}
	}

	return result, nil
}

// MatrixResult contains origin-destination matrix
type MatrixResult struct {
	Distances [][]float64 // km
	Times     [][]int     // seconds
}

// HeightData retrieves elevation data along a route
func (v *ValhallaClient) HeightData(ctx context.Context, encodedPolyline string) ([]HeightPoint, error) {
	url := fmt.Sprintf("%s/height?encoded_polyline=%s", v.baseURL, encodedPolyline)
	if v.apiKey != "" {
		url += "&api_key=" + v.apiKey
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var heightResp struct {
		height []float64 `json:"height"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&heightResp); err != nil {
		return nil, err
	}

	points := make([]HeightPoint, len(heightResp.height))
	for i, h := range heightResp.height {
		points[i] = HeightPoint{Index: i, ElevationMeters: h}
	}

	return points, nil
}

// HeightPoint is a single elevation point
type HeightPoint struct {
	Index           int
	ElevationMeters float64
}

func (v *ValhallaClient) parseResponse(resp *valhallaRouteResp) *RouteResult {
	trip := resp.trip

	result := &RouteResult{
		DistanceKm:  trip.Summary.TotalDistance / 1000,
		DurationSec: trip.Summary.TotalTime,
		Maneuvers:   make([]Maneuver, 0),
	}

	// Extract encoded polyline from first leg
	if len(trip.Legs) > 0 && len(trip.Legs[0].Shapes) > 0 {
		result.Geometry = trip.Legs[0].Shapes[0]
	}

	// Collect maneuvers
	for _, leg := range trip.Legs {
		for _, m := range leg.Maneuvers {
			maneuver := Maneuver{
				Instruction: m.Instruction,
				DistanceKm:  m.Length,
				DurationSec: m.Time,
				Lat:         m.Lat,
				Lon:         m.Lon,
			}
			result.Maneuvers = append(result.Maneuvers, maneuver)
		}
	}

	// Build summary text
	if trip.Summary.HasTime && trip.Summary.HasDistance {
		dur := time.Duration(trip.Summary.TotalTime) * time.Second
		result.Summary = fmt.Sprintf("%.1f km in %v", trip.Summary.TotalDistance/1000, dur.Round(time.Minute))
	}

	return result
}

func buildContours(minutes []int) []map[string]interface{} {
	contours := make([]map[string]interface{}, len(minutes))
	for i, m := range minutes {
		contours[i] = map[string]interface{}{
			"time":     m * 60, // Valhalla uses seconds
			"distance": 0,
			"color":    fmt.Sprintf("#%x", i*50+50),
		}
	}
	return contours
}

// AutoRoute is a convenience wrapper for car routing
func (v *ValhallaClient) AutoRoute(ctx context.Context, from, to models.Location) (*models.Route, error) {
	result, err := v.Route(ctx, from, to, CostingAuto)
	if err != nil {
		return nil, err
	}
	return v.toModelRoute(from, to, result), nil
}

// MotorcycleRoute routes for motorcycles (Delhi bike-taxi profile)
func (v *ValhallaClient) MotorcycleRoute(ctx context.Context, from, to models.Location) (*models.Route, error) {
	result, err := v.Route(ctx, from, to, CostingMotorScooter)
	if err != nil {
		return nil, err
	}
	return v.toModelRoute(from, to, result), nil
}

// BicycleRoute routes for bicycles
func (v *ValhallaClient) BicycleRoute(ctx context.Context, from, to models.Location) (*models.Route, error) {
	result, err := v.Route(ctx, from, to, CostingBicycle)
	if err != nil {
		return nil, err
	}
	return v.toModelRoute(from, to, result), nil
}

// PedestrianRoute routes for walking
func (v *ValhallaClient) PedestrianRoute(ctx context.Context, from, to models.Location) (*models.Route, error) {
	result, err := v.Route(ctx, from, to, CostingPedestrian)
	if err != nil {
		return nil, err
	}
	return v.toModelRoute(from, to, result), nil
}

func (v *ValhallaClient) toModelRoute(from, to models.Location, result *RouteResult) *models.Route {
	steps := make([]models.RouteStep, 0, len(result.Maneuvers))
	for _, m := range result.Maneuvers {
		steps = append(steps, models.RouteStep{
			Mode:        "driving",
			Instruction: m.Instruction,
			DistanceKm:  m.DistanceKm,
			DurationMin: m.DurationSec / 60,
		})
	}

	return &models.Route{
		From:         from,
		To:           to,
		DistanceKm:   result.DistanceKm,
		DurationMin:  result.DurationSec / 60,
		Mode:         "car",
		Geometry:     result.Geometry,
		Steps:        steps,
	}
}
