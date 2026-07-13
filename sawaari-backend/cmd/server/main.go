package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	bookingpkg "github.com/sawaari/backend/internal/booking"
	"github.com/sawaari/backend/internal/cache"
	"github.com/sawaari/backend/internal/comparison"
	"github.com/sawaari/backend/internal/events"
	"github.com/sawaari/backend/internal/fare"
	"github.com/sawaari/backend/internal/geocoding"
	"github.com/sawaari/backend/internal/handlers"
	notifpkg "github.com/sawaari/backend/internal/notifications"
	"github.com/sawaari/backend/internal/providers"
	"github.com/sawaari/backend/internal/realtime"
	"github.com/sawaari/backend/internal/routing"
	"github.com/sawaari/backend/internal/transit"
	"github.com/sawaari/backend/internal/users"
	walletpkg "github.com/sawaari/backend/internal/wallet"
)

func main() {
	port := envOr("PORT", "8080")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// --- DATA PLANE ---
	log.Println("[startup] data plane")
	valkeyCache := cache.NewValkeyCache()

	kafkaBrokers := splitCSV(os.Getenv("KAFKA_BROKERS"), []string{"localhost:9092"})
	eventBus := events.NewEventBus(kafkaBrokers, "sawaari")
	if err := eventBus.Connect(); err != nil {
		log.Printf("[warn] kafka connect: %v (continuing without bus)", err)
	}
	defer eventBus.Close()

	// --- CORE SERVICES ---
	log.Println("[startup] core services")
	geocoder := geocoding.New()
	router := routing.New()
	fareEngine := fare.New()

	otpBase := envOr("OTP2_BASE_URL", "http://localhost:8080/otp")
	transitPlanner := transit.New(otpBase, "default")

	bapID := envOr("ONDC_BAP_ID", "onbc_sawaari.bap@bap.builder")
	bapURI := envOr("ONDC_BAP_URI", "https://api.sawaari.app")
	bookingSvc := bookingpkg.NewBookingService(bapID, bapURI, os.Getenv("ONDC_PRIVATE_KEY"))

	realtimeSvc := realtime.NewETAService(valkeyCache, envOr("GTFS_RT_VEHICLE_URL", "https://otd.delhi.gov.in/api/release/gtfs-rt/feed/vehicle-positions?key=OTD_KEY"))
	go realtimeSvc.Start()
	defer realtimeSvc.Stop()

	walletSvc := walletpkg.NewTicketService(valkeyCache)

	notifHub := notifpkg.NewNotificationHub()

	userSvc := users.NewUserService(valkeyCache)

	// --- PROVIDERS ---
	log.Println("[startup] provider adapters")
	provs := []providers.Provider{
		providers.NewUber(os.Getenv("UBER_CLIENT_ID"), os.Getenv("UBER_CLIENT_SECRET")),
		providers.NewOla(os.Getenv("OLA_API_KEY")),
		providers.NewRapido(os.Getenv("RAPIDO_API_KEY")),
	}

	// --- ORCHESTRATOR ---
	log.Println("[startup] comparison orchestrator")
	orch := comparison.New(geocoder, router, fareEngine, provs)

	// --- ROUTES ---
	mux := handlers.SetupRoutes(orch, geocoder, transitPlanner, bookingSvc, realtimeSvc, walletSvc, notifHub, userSvc, valkeyCache, eventBus)

	// --- HTTP SERVER ---
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("[shutdown] signal received")
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer shutCancel()
		_ = srv.Shutdown(shutCtx)
		cancel()
	}()

	log.Printf("[startup] listening on :%s", port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func splitCSV(s string, fallback []string) []string {
	if s == "" {
		return fallback
	}
	out := []string{}
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}