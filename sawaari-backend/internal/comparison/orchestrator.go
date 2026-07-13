package comparison

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/sawaari/backend/internal/fare"
	"github.com/sawaari/backend/internal/geocoding"
	"github.com/sawaari/backend/internal/models"
	"github.com/sawaari/backend/internal/providers"
	"github.com/sawaari/backend/internal/routing"
)

// Orchestrator coordinates the comparison pipeline
type Orchestrator struct {
	geocoder *geocoding.Service
	router   *routing.Service
	fareEng  *fare.Engine
	providers []providers.Provider
}

// New creates a new comparison orchestrator
func New(
	geocoder *geocoding.Service,
	router *routing.Service,
	fareEng *fare.Engine,
	provs []providers.Provider,
) *Orchestrator {
	return &Orchestrator{
		geocoder:  geocoder,
		router:    router,
		fareEng:   fareEng,
		providers: provs,
	}
}

// Compare executes the full comparison pipeline
func (o *Orchestrator) Compare(ctx context.Context, req models.CompareRequest) (*models.CompareResponse, error) {
	start := time.Now()

	// Step 1: Geocode locations (with cache)
	fromLoc := req.FromLoc
	toLoc := req.ToLoc

	if fromLoc.Lat == 0 && fromLoc.Lng == 0 {
		result, err := o.geocoder.Geocode(ctx, req.From)
		if err != nil {
			return nil, err
		}
		fromLoc = result.Location
	}

	if toLoc.Lat == 0 && toLoc.Lng == 0 {
		result, err := o.geocoder.Geocode(ctx, req.To)
		if err != nil {
			return nil, err
		}
		toLoc = result.Location
	}

	// Step 2: Route calculation (parallel for different modes)
	var wg sync.WaitGroup
	var mu sync.Mutex

	routes := make(map[string]*models.Route)
	routeErrors := make(map[string]error)

	modes := []string{"car", "bicycle", "pedestrian"}
	for _, mode := range modes {
		wg.Add(1)
		go func(m string) {
			defer wg.Done()
			route, err := o.router.Route(ctx, fromLoc, toLoc, m)
			mu.Lock()
			if err != nil {
				routeErrors[m] = err
			} else {
				routes[m] = route
			}
			mu.Unlock()
		}(mode)
	}
	wg.Wait()

	// Get the best road route
	roadRoute := routes["car"]
	if roadRoute == nil {
		roadRoute = routes["bicycle"]
	}
	if roadRoute == nil {
		roadRoute = routes["pedestrian"]
	}

	// Step 3: Provider quotes (parallel)
	quoteChan := make(chan models.ProviderQuote, len(o.providers))
	var quoteWg sync.WaitGroup

	for _, p := range o.providers {
		quoteWg.Add(1)
		go func(prov providers.Provider) {
			defer quoteWg.Done()
			quote, _ := prov.Quote(ctx, fromLoc, toLoc, roadRoute.DistanceKm)
			quoteChan <- quote
		}(p)
	}

	go func() {
		quoteWg.Wait()
		close(quoteChan)
	}()

	// Step 4: Compute local fares (bus, metro, auto meter)
	localOptions := o.fareEng.ComputeLocalFares(fromLoc, toLoc, roadRoute, req.Prefs)

	// Step 5: Assemble and rank options
	var options []models.RideOption

	// Add local options
	options = append(options, localOptions...)

	// Add provider quotes
	for quote := range quoteChan {
		if quote.Available {
			opt := models.RideOption{
				ID:          quote.Provider + "_" + quote.Mode,
				Provider:    quote.Provider,
				Mode:        quote.Mode,
				DisplayName: o.providerDisplayName(quote.Provider, quote.Mode),
				Price: models.Price{
					Min:      quote.MinPrice * quote.Surge,
					Max:      quote.MaxPrice * quote.Surge,
					Currency: "INR",
				},
				ETA:         quote.ETA,
				DistanceKm:  roadRoute.DistanceKm,
				DeepLink:    o.providerDeepLink(quote.Provider, quote.Mode, fromLoc, toLoc),
				Bookable:    true,
			}
			options = append(options, opt)
		}
	}

	// Step 6: Rank options
	o.rankOptions(options, roadRoute.DurationMin)

	response := &models.CompareResponse{
		Options:   options,
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}

	_ = start // could log duration

	return response, nil
}

// rankOptions assigns badges based on price and ETA
func (o *Orchestrator) rankOptions(options []models.RideOption, baseETA int) {
	if len(options) == 0 {
		return
	}

	// Find min/max for normalization
	var minPrice, maxPrice float64 = 1e9, 0
	var minETA, maxETA int = 1e9, 0

	for _, opt := range options {
		if opt.Price.Min < minPrice {
			minPrice = opt.Price.Min
		}
		if opt.Price.Max > maxPrice {
			maxPrice = opt.Price.Max
		}
		if opt.ETA < minETA {
			minETA = opt.ETA
		}
		if opt.ETA > maxETA {
			maxETA = opt.ETA
		}
	}

	// Score each option
	type scored struct {
		idx    int
		score  float64
		price  float64
		eta    int
	}

	var scores []scored
	for i, opt := range options {
		normPrice := (opt.Price.Min - minPrice) / (maxPrice - minPrice + 0.001)
		normETA := float64(opt.ETA-minETA) / float64(maxETA-minETA+1)

		// Weighted score: 55% price, 45% time
		score := 0.55*normPrice + 0.45*normETA
		scores = append(scores, scored{i, score, opt.Price.Min, opt.ETA})
	}

	// Sort by score
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score < scores[j].score
	})

	// Assign badges
	for i, s := range scores {
		opt := &options[s.idx]
		opt.Badges = []string{}

		if i == 0 {
			opt.Badges = append(opt.Badges, "SMART_PICK")
		}
		if s.price == minPrice && maxPrice > minPrice {
			opt.Badges = append(opt.Badges, "CHEAPEST")
		}
		if s.eta == minETA && maxETA > minETA {
			opt.Badges = append(opt.Badges, "FASTEST")
		}
	}
}

func (o *Orchestrator) providerDisplayName(provider, mode string) string {
	switch provider {
	case "uber":
		switch mode {
		case "moto":
			return "Uber Moto"
		case "auto":
			return "Uber Auto"
		case "cab":
			return "Uber Go"
		}
	case "ola":
		switch mode {
		case "bike":
			return "Ola Bike"
		case "auto":
			return "Ola Auto"
		case "cab":
			return "Ola Prime"
		}
	case "rapido":
		return "Rapido " + mode
	}
	return provider + " " + mode
}

func (o *Orchestrator) providerDeepLink(provider, mode string, from, to models.Location) string {
	switch provider {
	case "uber":
		return "https://m.uber.com/ul/?action=setPickup&pickup[latitude]=" +
			fmt.Sprintf("%f", from.Lat) + "&pickup[longitude]=" +
			fmt.Sprintf("%f", from.Lng) + "&dropoff[latitude]=" +
			fmt.Sprintf("%f", to.Lat) + "&dropoff[longitude]=" +
			fmt.Sprintf("%f", to.Lng)
	case "ola":
		return "https://www.olacabs.com/book"
	}
	return ""
}
