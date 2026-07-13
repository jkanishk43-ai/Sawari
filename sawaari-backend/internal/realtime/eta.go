package realtime

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sawaari/backend/internal/cache"
)

// ETAService processes GTFS-RT feeds and computes real-time arrival predictions
type ETAService struct {
	cache        *cache.ValkeyCache
	vehicleStore *VehicleStore
	stopRegistry *StopRegistry
	baseURL     string
	pollInterval time.Duration
	httpClient   *http.Client

	// GTFS-RT protobuf parsing
	feedURL   string
	subscriber chan<- VehiclePosition

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// VehiclePosition represents a vehicle's current state
type VehiclePosition struct {
	VehicleID    string    `json:"vehicle_id"`
	TripID       string    `json:"trip_id"`
	RouteID      string    `json:"route_id"`
	RouteShortName string  `json:"route_short_name"`
	DirectionID  int       `json:"direction_id"`
	Latitude     float64   `json:"latitude"`
	Longitude    float64   `json:"longitude"`
	Bearing      float64   `json:"bearing"`
	Speed        float64   `json:"speed"` // m/s
	StopID       string    `json:"stop_id,omitempty"`
	StopSequence int       `json:"stop_sequence,omitempty"`
	VehicleLabel string    `json:"vehicle_label,omitempty"`
	LicensePlate string    `json:"license_plate,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
	H3Index      string    `json:"h3_index"`
}

// StopETA represents predicted arrival at a stop
type StopETA struct {
	StopID        string    `json:"stop_id"`
	StopName      string    `json:"stop_name"`
	TripID        string    `json:"trip_id"`
	RouteID       string    `json:"route_id"`
	RouteShortName string   `json:"route_short_name"`
	DirectionID   int       `json:"direction_id"`
	VehicleID     string    `json:"vehicle_id"`
	ScheduledArr  time.Time `json:"scheduled_arrival"`
	PredictedArr  time.Time `json:"predicted_arrival"`
	DelaySeconds  int       `json:"delay_seconds"`
	HeadwaySec    int       `json:"headway_seconds,omitempty"`
	DistanceMeters float64  `json:"distance_meters"`
	Occupancy     string    `json:"occupancy,omitempty"` // EMPTY, MANY_SEATS, FEW_SEATS, STANDING, FULL
}

// RoutePosition represents real-time vehicle positions on a route
type RoutePosition struct {
	RouteID        string             `json:"route_id"`
	RouteShortName string             `json:"route_short_name"`
	VehicleCount   int                `json:"vehicle_count"`
	Positions      []VehiclePosition  `json:"positions"`
	LastUpdated    time.Time          `json:"last_updated"`
}

// NewETAService creates a new ETA computation service
func NewETAService(cache *cache.ValkeyCache, feedURL string) *ETAService {
	ctx, cancel := context.WithCancel(context.Background())
	return &ETAService{
		cache:         cache,
		vehicleStore:  NewVehicleStore(),
		stopRegistry:  NewStopRegistry(),
		feedURL:       feedURL,
		baseURL:       "https://otd.delhi.gov.in",
		pollInterval:  10 * time.Second,
		httpClient:    &http.Client{Timeout: 30 * time.Second},
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start begins polling the GTFS-RT feed
func (s *ETAService) Start() {
	s.wg.Add(1)
	go s.pollLoop()
}

// Stop halts the polling loop
func (s *ETAService) Stop() {
	s.cancel()
	s.wg.Wait()
}

func (s *ETAService) pollLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	// Initial fetch
	s.fetchAndProcess()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.fetchAndProcess()
		}
	}
}

func (s *ETAService) fetchAndProcess() {
	ctx, cancel := context.WithTimeout(s.ctx, 25*time.Second)
	defer cancel()

	positions, err := s.fetchVehiclePositions(ctx)
	if err != nil {
		log.Printf("ETA: failed to fetch vehicle positions: %v", err)
		return
	}

	s.processPositions(positions)
}

func (s *ETAService) fetchVehiclePositions(ctx context.Context) ([]VehiclePosition, error) {
	// Build the GTFS-RT vehicle positions URL
	url := s.baseURL + "/gtfs-rt/vehiclepositions.pb"
	if s.feedURL != "" {
		url = s.feedURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/octet-stream")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("feed returned status %d", resp.StatusCode)
	}

	// Parse GTFS-RT protobuf
	return s.parseGTFSRT(resp.Body)
}

// parseGTFSRT decodes a GTFS-RT VehiclePositions feed
func (s *ETAService) parseGTFSRT(r io.Reader) ([]VehiclePosition, error) {
	// Read all bytes for manual protobuf parsing
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	// GTFS-RT VehiclePositions message structure:
	// FeedMessage { header: FeedHeader, entity: [FeedEntity] }
	// Each FeedEntity has id, vehicle: VehiclePosition

	var positions []VehiclePosition
	offset := 0

	for offset < len(data) {
		// Read field tag and length
		if offset >= len(data) {
			break
		}

		// Field 1 (Vehicle): tag = 0x0A (1 << 3 | 2 = length-delimited)
		// We need to find the vehicle entities in the feed
		tag := data[offset]
		if tag == 0 {
			offset++
			continue
		}

		fieldNum := int(tag >> 3)
		wireType := int(tag & 0x07)

		if fieldNum == 1 && wireType == 2 { // Vehicle field
			// Read length
			offset++
			length, consumed := readVarint(data[offset:])
			offset += consumed

			vehicleData := data[offset : offset+int(length)]
			pos, err := s.parseVehicleEntity(vehicleData)
			if err == nil && pos.VehicleID != "" {
				// Compute H3 index
				pos.H3Index = h3Index(pos.Latitude, pos.Longitude, 9)
				positions = append(positions, pos)
			}
			offset += int(length)
		} else {
			// Skip this field
			offset++
			switch wireType {
			case 0: // Varint
				_, consumed := readVarint(data[offset:])
				offset += consumed
			case 2: // Length-delimited
				length, consumed := readVarint(data[offset:])
				offset += consumed + int(length)
			case 5: // 32-bit
				offset += 4
			default:
				_, consumed := readVarint(data[offset:])
				offset += consumed
			}
		}
	}

	return positions, nil
}

func (s *ETAService) parseVehicleEntity(data []byte) (VehiclePosition, error) {
	var pos VehiclePosition
	offset := 0

	for offset < len(data) {
		if offset >= len(data) {
			break
		}

		tag := data[offset]
		fieldNum := int(tag >> 3)
		wireType := int(tag & 0x07)
		offset++

		switch fieldNum {
		case 1: // Vehicle descriptor (nested)
			length, consumed := readVarint(data[offset:])
			offset += consumed
			s.parseVehicleDescriptor(data[offset:offset+int(length)], &pos)
			offset += int(length)

		case 2: // Trip descriptor
			length, consumed := readVarint(data[offset:])
			offset += consumed
			s.parseTripDescriptor(data[offset:offset+int(length)], &pos)
			offset += int(length)

		case 3: // Position
			length, consumed := readVarint(data[offset:])
			offset += consumed
			s.parsePosition(data[offset:offset+int(length)], &pos)
			offset += int(length)

		case 4: // Current stop sequence
			val, consumed := readVarint(data[offset:])
			offset += consumed
			pos.StopSequence = int(val)

		case 5: // Stop ID
			length, consumed := readVarint(data[offset:])
			offset += consumed
			pos.StopID = string(data[offset : offset+int(length)])
			offset += int(length)

		case 6: // Current status (enum)
			val, consumed := readVarint(data[offset:])
			offset += consumed
			// 0=INCOMING_AT, 1=STOPPED_AT, 2=IN_TRANSIT_TO

		case 7: // Timestamp
			val, consumed := readVarint(data[offset:])
			offset += consumed
			pos.Timestamp = time.Unix(int64(val), 0)

		default:
			// Skip unknown field
			switch wireType {
			case 0:
				_, consumed := readVarint(data[offset:])
				offset += consumed
			case 2:
				length, consumed := readVarint(data[offset:])
				offset += consumed + int(length)
			case 5:
				offset += 4
			default:
				_, consumed := readVarint(data[offset:])
				offset += consumed
			}
		}
	}

	return pos, nil
}

func (s *ETAService) parseVehicleDescriptor(data []byte, pos *VehiclePosition) {
	offset := 0
	for offset < len(data) {
		tag := data[offset]
		fieldNum := int(tag >> 3)
		wireType := int(tag & 0x07)
		offset++

		switch fieldNum {
		case 1: // ID
			length, consumed := readVarint(data[offset:])
			offset += consumed
			pos.VehicleID = string(data[offset : offset+int(length)])
			offset += int(length)
		case 2: // Label
			length, consumed := readVarint(data[offset:])
			offset += consumed
			pos.VehicleLabel = string(data[offset : offset+int(length)])
			offset += int(length)
		case 3: // License plate
			length, consumed := readVarint(data[offset:])
			offset += consumed
			pos.LicensePlate = string(data[offset : offset+int(length)])
			offset += int(length)
		default:
			switch wireType {
			case 0:
				_, consumed := readVarint(data[offset:])
				offset += consumed
			case 2:
				length, consumed := readVarint(data[offset:])
				offset += consumed + int(length)
			default:
				_, consumed := readVarint(data[offset:])
				offset += consumed
			}
		}
	}
}

func (s *ETAService) parseTripDescriptor(data []byte, pos *VehiclePosition) {
	offset := 0
	for offset < len(data) {
		tag := data[offset]
		fieldNum := int(tag >> 3)
		wireType := int(tag & 0x07)
		offset++

		switch fieldNum {
		case 1: // Trip ID
			length, consumed := readVarint(data[offset:])
			offset += consumed
			pos.TripID = string(data[offset : offset+int(length)])
			offset += int(length)
		case 2: // Route ID
			length, consumed := readVarint(data[offset:])
			offset += consumed
			pos.RouteID = string(data[offset : offset+int(length)])
			offset += int(length)
		case 3: // Direction ID
			val, consumed := readVarint(data[offset:])
			offset += consumed
			pos.DirectionID = int(val)
		case 5: // Route short name
			length, consumed := readVarint(data[offset:])
			offset += consumed
			pos.RouteShortName = string(data[offset : offset+int(length)])
			offset += int(length)
		default:
			switch wireType {
			case 0:
				_, consumed := readVarint(data[offset:])
				offset += consumed
			case 2:
				length, consumed := readVarint(data[offset:])
				offset += consumed + int(length)
			default:
				_, consumed := readVarint(data[offset:])
				offset += consumed
			}
		}
	}
}

func (s *ETAService) parsePosition(data []byte, pos *VehiclePosition) {
	offset := 0
	for offset < len(data) {
		tag := data[offset]
		fieldNum := int(tag >> 3)
		wireType := int(tag & 0x07)
		offset++

		switch fieldNum {
		case 1: // Latitude (float)
			bits := binary.LittleEndian.Uint32(data[offset : offset+4])
			pos.Latitude = math.Float32frombits(bits)
			offset += 4
		case 2: // Longitude (float)
			bits := binary.LittleEndian.Uint32(data[offset : offset+4])
			pos.Longitude = math.Float32frombits(bits)
			offset += 4
		case 3: // Bearing (float)
			bits := binary.LittleEndian.Uint32(data[offset : offset+4])
			pos.Bearing = float64(math.Float32frombits(bits))
			offset += 4
		case 4: // Speed (float)
			bits := binary.LittleEndian.Uint32(data[offset : offset+4])
			pos.Speed = float64(math.Float32frombits(bits))
			offset += 4
		default:
			switch wireType {
			case 0:
				_, consumed := readVarint(data[offset:])
				offset += consumed
			case 2:
				length, consumed := readVarint(data[offset:])
				offset += consumed + int(length)
			case 5:
				offset += 4
			default:
				_, consumed := readVarint(data[offset:])
				offset += consumed
			}
		}
	}
}

// H3 index computation (simplified version - in production use uber/h3 library)
func h3Index(lat, lng float64, resolution int) string {
	// This is a simplified placeholder
	// In production, use github.com/uber/h3-go oruber/h3-go
	return fmt.Sprintf("%08x%08x", int(lat*1e7), int(lng*1e7))
}

func readVarint(data []byte) (uint64, int) {
	var result uint64
	var shift uint
	consumed := 0

	for i, b := range data {
		consumed = i + 1
		result |= uint64(b&0x7F) << shift
		if b&0x80 == 0 {
			break
		}
		shift += 7
		if shift >= 64 {
			break
		}
	}

	return result, consumed
}

func (s *ETAService) processPositions(positions []VehiclePosition) {
	now := time.Now()

	for _, pos := range positions {
		// Store in memory
		s.vehicleStore.Update(pos)

		// Cache for quick access
		key := fmt.Sprintf("vehicle:%s", pos.VehicleID)
		s.cache.Valkey.Set(s.ctx, key, 30*time.Second, pos)

		// Publish to Kafka (if configured)
		s.publishPosition(pos)
	}

	// Update route summaries
	routes := s.vehicleStore.RouteSummary()
	for routeID, positions := range routes {
		key := fmt.Sprintf("route:%s:positions", routeID)
		s.cache.Valkey.Set(s.ctx, key, 60*time.Second, RoutePosition{
			RouteID:        routeID,
			VehicleCount:   len(positions),
			Positions:      positions,
			LastUpdated:    now,
		})
	}
}

// GetETAsForStop returns predicted arrivals for a stop
func (s *ETAService) GetETAsForStop(ctx context.Context, stopID string) ([]StopETA, error) {
	// Get stop location
	stop := s.stopRegistry.Get(stopID)
	if stop == nil {
		return nil, fmt.Errorf("stop not found: %s", stopID)
	}

	// Get vehicles on routes that serve this stop
	candidates := s.findNearbyVehicles(stop.Lat, stop.Lng, 5*time.Minute)

	var etas []StopETA
	for _, pos := range candidates {
		eta := s.computeETA(pos, stop)
		etas = append(etas, eta)
	}

	// Sort by predicted arrival time
	sort.Slice(etas, func(i, j int) bool {
		return etas[i].PredictedArr.Before(etas[j].PredictedArr)
	})

	// Limit results
	if len(etas) > 20 {
		etas = etas[:20]
	}

	return etas, nil
}

func (s *ETAService) findNearbyVehicles(lat, lng float64, maxAge time.Duration) []VehiclePosition {
	vehicles := s.vehicleStore.GetAll()
	var nearby []VehiclePosition
	cutoff := time.Now().Add(-maxAge)

	for _, pos := range vehicles {
		if pos.Timestamp.Before(cutoff) {
			continue
		}
		// Simple distance check (in production use proper geo distance)
		dist := haversine(lat, lng, pos.Latitude, pos.Longitude)
		if dist < 10 { // within 10km
			nearby = append(nearby, pos)
		}
	}

	return nearby
}

func (s *ETAService) computeETA(vehicle VehiclePosition, stop *StopInfo) StopETA {
	distance := haversine(vehicle.Latitude, vehicle.Longitude, stop.Lat, stop.Lng) * 1000 // meters

	var eta time.Duration
	if vehicle.Speed > 0.5 { // Moving
		eta = time.Duration(distance/vehicle.Speed) * time.Second
	} else {
		// Estimate based on historical speed (5 m/s = 18 km/h)
		eta = time.Duration(distance/5) * time.Second
	}

	predictedArr := time.Now().Add(eta)

	// Compute delay (mock - in production compare with schedule)
	delaySec := int(eta.Seconds() / 60) // Simplified

	return StopETA{
		StopID:        stop.ID,
		StopName:      stop.Name,
		TripID:        vehicle.TripID,
		RouteID:       vehicle.RouteID,
		RouteShortName: vehicle.RouteShortName,
		DirectionID:   vehicle.DirectionID,
		VehicleID:     vehicle.VehicleID,
		DistanceMeters: distance,
		PredictedArr:  predictedArr,
		DelaySeconds:  delaySec,
	}
}

// GetRoutePositions returns live vehicle positions for a route
func (s *ETAService) GetRoutePositions(ctx context.Context, routeID string) (*RoutePosition, error) {
	// Check cache first
	key := fmt.Sprintf("route:%s:positions", routeID)
	var positions RoutePosition
	if err := s.cache.Valkey.Get(ctx, key, &positions); err == nil {
		return &positions, nil
	}

	// Fall back to in-memory store
	vehicles := s.vehicleStore.GetByRoute(routeID)
	if vehicles == nil {
		return nil, fmt.Errorf("no vehicles found for route: %s", routeID)
	}

	return &RoutePosition{
		RouteID:       routeID,
		VehicleCount:  len(vehicles),
		Positions:     vehicles,
		LastUpdated:   time.Now(),
	}, nil
}

func (s *ETAService) publishPosition(pos VehiclePosition) {
	if s.subscriber == nil {
		return
	}
	select {
	case s.subscriber <- pos:
	default:
		// Channel full, skip
	}
}

// SubscribePositions returns a channel of vehicle positions
func (s *ETAService) SubscribePositions() <-chan VehiclePosition {
	ch := make(chan VehiclePosition, 100)
	s.subscriber = ch
	return ch
}

// VehicleStore holds current vehicle state
type VehicleStore struct {
	mu        sync.RWMutex
	vehicles  map[string]VehiclePosition
	routeIdx  map[string][]VehiclePosition
}

func NewVehicleStore() *VehicleStore {
	return &VehicleStore{
		vehicles: make(map[string]VehiclePosition),
		routeIdx: make(map[string][]VehiclePosition),
	}
}

func (vs *VehicleStore) Update(pos VehiclePosition) {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	oldPos, exists := vs.vehicles[pos.VehicleID]
	if exists {
		// Remove from old route index
		if oldPos.RouteID != "" {
			vs.removeFromRouteIdx(oldPos)
		}
	}

	vs.vehicles[pos.VehicleID] = pos

	// Add to new route index
	if pos.RouteID != "" {
		vs.routeIdx[pos.RouteID] = append(vs.routeIdx[pos.RouteID], pos)
	}
}

func (vs *VehicleStore) removeFromRouteIdx(pos VehiclePosition) {
	if vehicles, ok := vs.routeIdx[pos.RouteID]; ok {
		filtered := make([]VehiclePosition, 0, len(vehicles))
		for _, v := range vehicles {
			if v.VehicleID != pos.VehicleID {
				filtered = append(filtered, v)
			}
		}
		vs.routeIdx[pos.RouteID] = filtered
	}
}

func (vs *VehicleStore) Get(vehicleID string) (VehiclePosition, bool) {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	pos, ok := vs.vehicles[vehicleID]
	return pos, ok
}

func (vs *VehicleStore) GetAll() []VehiclePosition {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	result := make([]VehiclePosition, 0, len(vs.vehicles))
	for _, v := range vs.vehicles {
		result = append(result, v)
	}
	return result
}

func (vs *VehicleStore) GetByRoute(routeID string) []VehiclePosition {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	result := make([]VehiclePosition, len(vs.routeIdx[routeID]))
	copy(result, vs.routeIdx[routeID])
	return result
}

func (vs *VehicleStore) RouteSummary() map[string][]VehiclePosition {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	result := make(map[string][]VehiclePosition)
	for k, v := range vs.routeIdx {
		cp := make([]VehiclePosition, len(v))
		copy(cp, v)
		result[k] = cp
	}
	return result
}

// StopRegistry maps stop IDs to their locations
type StopRegistry struct {
	mu    sync.RWMutex
	stops map[string]*StopInfo
}

type StopInfo struct {
	ID     string
	Name   string
	Lat    float64
	Lon    float64
	Routes []string
}

func NewStopRegistry() *StopRegistry {
	return &StopRegistry{
		stops: make(map[string]*StopInfo),
	}
}

func (sr *StopRegistry) Register(info StopInfo) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	sr.stops[info.ID] = &info
}

func (sr *StopRegistry) Get(stopID string) *StopInfo {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	return sr.stops[stopID]
}

func (sr *StopRegistry) LoadFromGTFS(ctx context.Context, stopsURL string) error {
	resp, err := http.Get(stopsURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch stops: %d", resp.StatusCode)
	}

	// Parse CSV
	reader := csv.NewReader(resp.Body)
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	_, err = reader.Read() // Skip header
	if err != nil {
		return err
	}

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		if len(record) < 6 {
			continue
		}

		stopID := record[0]
		stopName := record[2]
		lat, _ := strconv.ParseFloat(record[4], 64)
		lng, _ := strconv.ParseFloat(record[5], 64)

		sr.Register(StopInfo{
			ID:   stopID,
			Name: stopName,
			Lat:  lat,
			Lon:  lng,
		})
	}

	return nil
}

// haversine calculates distance in meters
func haversine(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadius = 6371000 // meters

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

// ETAResponse is the API response format
type ETAResponse struct {
	StopID     string     `json:"stop_id"`
	StopName   string     `json:"stop_name"`
	GeneratedAt time.Time  `json:"generated_at"`
	ETAs       []StopETA  `json:"etas"`
}

// ServeSSE streams ETAs as Server-Sent Events
func (s *ETAService) ServeSSE(ctx context.Context, routeID string, w io.Writer) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("response writer doesn't support flushing")
	}

	headers := w.Header()
	headers.Set("Content-Type", "text/event-stream")
	headers.Set("Cache-Control", "no-cache")
	headers.Set("Connection", "keep-alive")
	headers.Set("Access-Control-Allow-Origin", "*")

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	// Send initial positions
	s.sendPositions(ctx, routeID, w)
	flusher.Flush()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			s.sendPositions(ctx, routeID, w)
			flusher.Flush()
		}
	}
}

func (s *ETAService) sendPositions(ctx context.Context, routeID string, w io.Writer) {
	positions, err := s.GetRoutePositions(ctx, routeID)
	if err != nil {
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", err.Error())
		return
	}

	data, _ := json.Marshal(positions)
	fmt.Fprintf(w, "event: positions\ndata: %s\n\n", string(data))
}

// CSV export for GTFS-RT analysis
func ExportPositionsCSV(w io.Writer, positions []VehiclePosition) error {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Header
	writer.Write([]string{
		"vehicle_id", "trip_id", "route_id", "latitude", "longitude",
		"bearing", "speed", "stop_id", "timestamp",
	})

	for _, p := range positions {
		writer.Write([]string{
			p.VehicleID,
			p.TripID,
			p.RouteID,
			strconv.FormatFloat(p.Latitude, 'f', 6, 64),
			strconv.FormatFloat(p.Longitude, 'f', 6, 64),
			strconv.FormatFloat(p.Bearing, 'f', 2, 64),
			strconv.FormatFloat(p.Speed, 'f', 2, 64),
			p.StopID,
			p.Timestamp.Format(time.RFC3339),
		})
	}

	return nil
}
