package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// Response structure for health and readiness checks
type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Service   string    `json:"service"`
}

// healthCheckHandler handles the /health endpoint
func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now(),
		Service:   "opt360-portal-backend",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// readinessCheckHandler handles the /ready endpoint
func readinessCheckHandler(w http.ResponseWriter, r *http.Request) {
	// Add your readiness checks here (e.g., database connectivity, dependencies)
	// For now, we'll return ready status
	response := HealthResponse{
		Status:    "ready",
		Timestamp: time.Now(),
		Service:   "opt360-portal-backend",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func main() {
	// Register handlers
	http.HandleFunc("/health", healthCheckHandler)
	http.HandleFunc("/ready", readinessCheckHandler)

	// Start server
	port := ":8080"
	log.Printf("Starting server on port %s", port)
	log.Printf("Health check endpoint: http://localhost%s/health", port)
	log.Printf("Readiness check endpoint: http://localhost%s/ready", port)

	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
