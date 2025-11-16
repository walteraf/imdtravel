package main

import (
	"encoding/json"
	"log"
	"math/rand/v2"
	"net/http"
	"sync"
	"time"
)

var (
	faultR2Mutex   sync.Mutex
	faultR2Active  bool
	faultR2EndTime time.Time
)

func main() {
	http.HandleFunc("/convert", getExchangeRateHandler)
	http.HandleFunc("/health", healthHandler)

	port := ":8082"
	log.Printf("Exchange service starting on port %s", port)
	log.Fatal(http.ListenAndServe(port, nil))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

func getExchangeRateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	const (
		probR2     = 0.1
		durationR2 = 5 * time.Second
	)

	faultR2Mutex.Lock()
	now := time.Now()

	if faultR2Active && now.Before(faultR2EndTime) {
		log.Println("[FAULT] Request 2: Error STATE active. Returning HTTP 500.")
		faultR2Mutex.Unlock()
		http.Error(w, "Internal Server Error (Simulated Fault State)", http.StatusInternalServerError)
		return

	} else {
		faultR2Active = false

		if rand.Float64() < probR2 {
			log.Println("[FAULT] Request 2: Error fault TRIGGERED. State active for 5s.")
			faultR2Active = true
			faultR2EndTime = now.Add(durationR2)

			faultR2Mutex.Unlock()
			http.Error(w, "Internal Server Error (Simulated Fault State)", http.StatusInternalServerError)
			return
		}
	}
	faultR2Mutex.Unlock()

	intValue := 5000 + rand.IntN(1001)
	exchangeRate := float64(intValue) / 1000.0

	log.Printf("Exchange rate generated: %.4f", exchangeRate)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(exchangeRate)
}
