package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"
)

func main() {
	name := os.Getenv("BACKEND_NAME")
	if name == "" {
		name = "backend-unknown"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "9001"
	}

	latencyMs := 0
	if v := os.Getenv("LATENCY_MS"); v != "" {
		if ms, err := strconv.Atoi(v); err == nil {
			latencyMs = ms
		}
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if latencyMs > 0 {
			// Add jitter: 50-150% of configured latency.
			jitter := float64(latencyMs) * (0.5 + rand.Float64())
			time.Sleep(time.Duration(jitter) * time.Millisecond)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"backend":   name,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"message":   fmt.Sprintf("Hello from %s!", name),
		})
	})

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"backend": name,
		})
	})

	addr := ":" + port
	log.Printf("%s starting on %s (latency=%dms)", name, addr, latencyMs)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
