package transit

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

// Planner provides transit trip planning via OpenTripPlanner 2
type Planner struct {
	baseURL    string
	httpClient *http.Client
	graphID    string
}

// OTP2 API response structures
type otpItinerary struct {
	duration    int           `json:"duration"`
	startTime   int64         `json:"startTime"`
	endTime     int64         `json:"endTime"`
	walkTime    int           `json:"walkTime"`
	transitTime int           `json:"transitTime"`
	waitTime    int           `json:"waitTime"`
	legs        []otpLeg      `json:"legs"`
	fare        otpFare       `json:"fare"`
}

type otpLeg struct {
	startTime        int64     `json:"startTime"`
	endTime          int64     `json:"endTime"`
	departureDelay   int       `json:"departureDelay"`
	arrivalDelay     int       `json:"arrivalDelay"`
	mode             string    `json:"mode"`
	route            string    `json:"route"`
	agencyName       string    `json:"agencyName"`
	agencyUrl        string    `json:"agencyUrl"`
	headsign         string    `json:"headsign"`
	realTime         bool      `json:"realTime"`
	distance         float64   `json:"distance"`
	transitLeg       bool      `json:"transitLeg"`
	from             otpStop   `json:"from"`
	To               otpStop   `json:"to"`
	legGeometry       otpGeom   `json:"legGeometry"`
	intermediateStops []otpStop `json:"intermediateStops"`
}

type otpStop struct {
	name        string       `json:"name"`
	stopId      string       `json:"stopId"`
	lat         float64      `json:"lat"`
	lon         float64      `json:"lon"`
	arrivalTime int64        `json:"arrivalTime,omitempty"`
	departureTime int64      `json:"departureTime,omitempty"`
}

type otpGeom struct {
	points    string  `json:"points"`
	length    int     `json:"length"`
}

type otpFare struct {
	fare           map[string]float64 `json:"fare"`
	totalFare      float64           `json:"totalFare"`
_details       interface{}         `json:"details,omitempty"`
}

type otpTripPlan struct {
	Error    interface{}   `json:"error"`
	ErrorMsg string        `json:"errorMessage"`
	plan     *otpItinerary `json:"plan"`
}

// New creates a new OTP2 transit planner
func New(baseURL string, graphID string) *Planner {
	if baseURL == "" {
		baseURL = "http://localhost:8080/otp"
	}
	if graphID == "" {
		graphID = "default"
	}
	return &Planner{
		baseURL: baseURL,
		graphID: graphID,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// TripRequest defines parameters for a trip plan
type TripRequest struct {
	From        models.Location
	To          models.Location
	DepartAfter time.Time
	ArriveBy    bool
	Wheelchair  bool
	Bike        bool
	Modes       []string // BUS, TRAM, RAIL, SUBWAY, FERRY, WALK, BICYCLE
}

// TripPlan represents a complete trip plan
type TripPlan struct {
	Duration      time.Duration
	StartTime     time.Time
	EndTime       time.Time
	WalkTime      time.Duration
	TransitTime   time.Duration
	WaitTime      time.Duration
	Legs          []TripLeg
	TotalFare     float64
	Currency      string
	Transfers     int
}

// TripLeg represents a single leg of a trip
type TripLeg struct {
	StartTime      time.Time
	EndTime        time.Time
	Mode           string
	Route          string
	RouteShortName string
	HeadSign       string
	AgencyName     string
	AgencyURL      string
	FromStop       string
	FromStopID     string
	FromLat        float64
	FromLon        float64
	ToStop         string
	ToStopID       string
	ToLat          float64
	ToLon          float64
	DistanceMeters float64
	Geometry       string
	RealTime       bool
	IntermediateStops []Stop
}

// Stop represents a transit stop
type Stop struct {
	ID        string
	Name      string
	Lat       float64
	Lon       float64
	ArrTime   time.Time
	DepTime   time.Time
}

// PlanTrip queries OTP2 for transit itineraries
func (p *Planner) PlanTrip(ctx context.Context, req TripRequest) ([]TripPlan, error) {
	departTime := req.DepartAfter
	if departTime.IsZero() {
		departTime = time.Now()
	}

	// Build OTP2 API URL
	url := fmt.Sprintf("%s/routers/%s/plan",
		trimSlash(p.baseURL), p.graphID)

	// Build query parameters
	params := fmt.Sprintf("?fromPlace=%f,%f&toPlace=%f,%f&time=%s&arriveBy=%v&mode=WALK,TRANSIT",
		req.From.Lat, req.From.Lng, req.To.Lat, req.To.Lng,
		departTime.Format("15:04:05"), req.ArriveBy)

	if req.Wheelchair {
		params += "&wheelchair=true"
	}
	if req.Bike {
		params += "&bikeRental=true"
	}

	fullURL := url + params

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", "Sawaari/1.0")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("OTP2 request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OTP2 returned status %d: %s", resp.StatusCode, string(body))
	}

	var otpResp otpTripPlan
	if err := json.NewDecoder(resp.Body).Decode(&otpResp); err != nil {
		return nil, fmt.Errorf("failed to decode OTP2 response: %w", err)
	}

	if otpResp.Error != nil || otpResp.plan == nil {
		return nil, fmt.Errorf("no trip plan found: %s", otpResp.ErrorMsg)
	}

	// Convert OTP response to our model
	return []TripPlan{p.convertItinerary(otpResp.plan)}, nil
}

// GetNearbyStops finds transit stops near a location
func (p *Planner) GetNearbyStops(ctx context.Context, lat, lng float64, radiusMeters int) ([]Stop, error) {
	if radiusMeters == 0 {
		radiusMeters = 500
	}

	url := fmt.Sprintf("%s/routers/%s/index/stops?lat=%f&lon=%f&radius=%d",
		trimSlash(p.baseURL), p.graphID, lat, lng, radiusMeters)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OTP2 stops query failed: %d", resp.StatusCode)
	}

	var stopsResp []struct {
		ID   string `json:"id"`
		Lat  float64 `json:"lat"`
		Lon  float64 `json:"lon"`
		Name string `json:"name"`
		Dist int    `json:"distance"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&stopsResp); err != nil {
		return nil, err
	}

	stops := make([]Stop, 0, len(stopsResp))
	for _, s := range stopsResp {
		stops = append(stops, Stop{
			ID:   s.ID,
			Name: s.Name,
			Lat:  s.Lat,
			Lon:  s.Lon,
		})
	}

	return stops, nil
}

// GetStopTimes returns departure times for a stop
func (p *Planner) GetStopTimes(ctx context.Context, stopID string, limit int) ([]StopTime, error) {
	if limit == 0 {
		limit = 10
	}

	url := fmt.Sprintf("%s/routers/%s/index/stops/%s/SIMPLE/stoptimes",
		trimSlash(p.baseURL), p.graphID, stopID)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OTP2 stoptimes query failed: %d", resp.StatusCode)
	}

	var timesResp []struct {
		Date            int64  `json:"date"`
		Time            int    `json:"time"`
		RealtimeArrival int    `json:"realtimeArrival"`
		RealtimeDeparture int `json:"realtimeDeparture"`
		Headsign        string `json:"headsign"`
		Mode            string `json:"mode"`
		Route           string `json:"route"`
		TripID          string `json:"tripId"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&timesResp); err != nil {
		return nil, err
	}

	times := make([]StopTime, 0, limit)
	for i := 0; i < len(timesResp) && i < limit; i++ {
		t := timesResp[i]
		times = append(times, StopTime{
			TripID:       t.TripID,
			Route:        t.Route,
			HeadSign:     t.Headsign,
			Mode:         t.Mode,
			ScheduledArr:  time.Unix(t.Date+t.RealtimeArrival*60, 0),
			ScheduledDep: time.Unix(t.Date+t.RealtimeDeparture*60, 0),
		})
	}

	return times, nil
}

// StopTime represents departure information
type StopTime struct {
	TripID       string
	Route        string
	HeadSign     string
	Mode         string
	ScheduledArr time.Time
	ScheduledDep time.Time
	RealtimeArr  time.Time
	RealtimeDep  time.Time
	DelayMin     int
}

// LoadGTFS triggers a GTFS data reload in OTP2
func (p *Planner) LoadGTFS(ctx context.Context, gtfsURL string) error {
	url := fmt.Sprintf("%s/routers/%s/import",
		trimSlash(p.baseURL), p.graphID)

	payload := map[string]string{
		"feedId":   "delhi_gtfs",
		"url":      gtfsURL,
		"stripWhiteSpace": "true",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GTFS load failed: %d - %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// RouterInfo returns OTP2 router configuration
func (p *Planner) RouterInfo(ctx context.Context) (*RouterConfig, error) {
	url := fmt.Sprintf("%s/routers/%s",
		trimSlash(p.baseURL), p.graphID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var info struct {
		config struct {
			TimeZone          string   `json:"timeZone"`
			GeocoderClass     string   `json:"geocoderClass"`
			StopAreaMode      bool     `json:"stopAreaMode"`
			WalkSpeed         float64  `json:"walkSpeed"`
			BikeSpeed         float64  `json:"bikeSpeed"`
			CarSpeed          float64  `json:"carSpeed"`
			TransferPenalty   int      `json:"transferPenalty"`
			WalkingBoardCost  int      `json:"walkingBoardCost"`
			NumberOfItineraries int    `json:"numItineraries"`
		} `json:"config"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}

	return &RouterConfig{
		TimeZone:          info.config.TimeZone,
		WalkSpeedMps:      info.config.WalkSpeed,
		BikeSpeedMps:      info.config.BikeSpeed,
		CarSpeedMps:       info.config.CarSpeed,
		TransferPenaltySec: info.config.TransferPenalty,
		MaxItineraries:   info.config.NumberOfItineraries,
	}, nil
}

// RouterConfig holds OTP router configuration
type RouterConfig struct {
	TimeZone          string
	WalkSpeedMps      float64
	BikeSpeedMps      float64
	CarSpeedMps       float64
	TransferPenaltySec int
	MaxItineraries    int
}

func (p *Planner) convertItinerary(it *otpItinerary) TripPlan {
	plan := TripPlan{
		Duration:    time.Duration(it.duration) * time.Second,
		StartTime:  time.UnixMilli(it.startTime),
		EndTime:    time.UnixMilli(it.endTime),
		WalkTime:   time.Duration(it.walkTime) * time.Second,
		TransitTime: time.Duration(it.transitTime) * time.Second,
		WaitTime:   time.Duration(it.waitTime) * time.Second,
		Legs:       make([]TripLeg, 0, len(it.legs)),
		Currency:   "INR",
	}

	for i, leg := range it.legs {
		tripLeg := TripLeg{
			StartTime:      time.UnixMilli(leg.startTime),
			EndTime:        time.UnixMilli(leg.endTime),
			Mode:           leg.mode,
			HeadSign:       leg.headsign,
			AgencyName:     leg.agencyName,
			AgencyURL:      leg.agencyUrl,
			FromStop:       leg.from.name,
			FromStopID:     leg.from.stopId,
			FromLat:        leg.from.lat,
			FromLon:        leg.from.lon,
			ToStop:         leg.To.name,
			ToStopID:       leg.To.stopId,
			ToLat:          leg.To.lat,
			ToLon:          leg.To.lon,
			DistanceMeters: leg.distance,
			Geometry:       leg.legGeometry.points,
			RealTime:       leg.realTime,
		}

		// Extract route info from mode-specific fields
		if leg.route != "" {
			tripLeg.Route = leg.route
		}

		// Convert intermediate stops
		for _, is := range leg.intermediateStops {
			tripLeg.IntermediateStops = append(tripLeg.IntermediateStops, Stop{
				ID:   is.stopId,
				Name: is.name,
				Lat:  is.lat,
				Lon:  is.lon,
			})
		}

		if leg.transitLeg {
			plan.Transfers++
		}

		plan.Legs = append(plan.Legs, tripLeg)
	}

	// Count unique transit legs for fare
	if it.fare.totalFare > 0 {
		plan.TotalFare = it.fare.totalFare
	}

	// Adjust transfers (subtract 1 if > 0)
	if plan.Transfers > 0 {
		plan.Transfers--
	}

	return plan
}

// ToModels converts a TripPlan to comparison models
func (p *TripPlan) ToModels() models.RideOption {
	eta := int(p.Duration.Minutes())
	return models.RideOption{
		ID:          fmt.Sprintf("transit_%d", p.StartTime.Unix()),
		Provider:    "OTP2",
		Mode:        p.PrimaryMode(),
		DisplayName: p.DisplayName(),
		Price: models.Price{
			Min:      p.TotalFare,
			Max:      p.TotalFare,
			Currency: p.Currency,
		},
		ETA:         eta,
		DistanceKm:  p.TotalDistanceMeters() / 1000,
		Badges:      []string{},
		Reliability: 0.92,
		Bookable:    p.CanBook(),
	}
}

// PrimaryMode returns the dominant transit mode
func (p *TripPlan) PrimaryMode() string {
	modeCounts := make(map[string]int)
	for _, leg := range p.Legs {
		if leg.Mode != "WALK" {
			modeCounts[leg.Mode]++
		}
	}

	dominant := "BUS"
	maxCount := 0
	for mode, count := range modeCounts {
		if count > maxCount {
			maxCount = count
			dominant = mode
		}
	}
	return dominant
}

// DisplayName generates a human-readable trip name
func (p *TripPlan) DisplayName() string {
	if len(p.Legs) == 0 {
		return "Transit Trip"
	}

	mode := p.PrimaryMode()
	firstTransit := ""
	for _, leg := range p.Legs {
		if leg.Mode != "WALK" && leg.Route != "" {
			firstTransit = leg.Route
			break
		}
	}

	if firstTransit != "" {
		return fmt.Sprintf("%s via %s", mode, firstTransit)
	}
	return mode
}

// TotalDistanceMeters sums all leg distances
func (p *TripPlan) TotalDistanceMeters() float64 {
	var total float64
	for _, leg := range p.Legs {
		total += leg.DistanceMeters
	}
	return total
}

// CanBook returns whether this trip can be booked via ONDC
func (p *TripPlan) CanBook() bool {
	// Metro trips can be booked via ONDC QR tickets
	for _, leg := range p.Legs {
		if leg.Mode == "SUBWAY" || leg.Mode == "METRO_RAIL" {
			return true
		}
	}
	return false
}

func trimSlash(s string) string {
	s = bytes.TrimSuffix([]byte(s), []byte("/"))
	return string(s)
}
