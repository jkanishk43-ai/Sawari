package main

import (
	"log"
	"net/http"
	"os"

	"github.com/sawaari/backend/internal/comparison"
	"github.com/sawaari/backend/internal/fare"
	"github.com/sawaari/backend/internal/geocoding"
	"github.com/sawaari/backend/internal/handlers"
	"github.com/sawaari/backend/internal/providers"
	"github.com/sawaari/backend/internal/routing"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Initialize services
	geocoder := geocoding.New()
	router := routing.New()
	fareEngine := fare.New()

	// Initialize providers
	provs := []providers.Provider{
		providers.NewUber(
			os.Getenv("UBER_CLIENT_ID"),
			os.Getenv("UBER_CLIENT_SECRET"),
		),
		providers.NewOla(
			os.Getenv("OLA_API_KEY"),
		),
		providers.NewRapido(
			os.Getenv("RAPIDO_API_KEY"),
		),
	}

	// Create orchestrator
	orch := comparison.New(geocoder, router, fareEngine, provs)

	// Setup routes
	mux := handlers.SetupRoutes(orch)

	log.Printf("Server starting on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}
