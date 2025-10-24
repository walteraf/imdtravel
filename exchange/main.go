package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func main() {
	http.HandleFunc("/exchange", getExchangeRateHandler)
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

	// Gerar taxa de câmbio aleatória entre 5.0 e 6.0
	// 1 dólar = entre 5 e 6 reais
	exchangeRate := 5.0 + rand.Float64()*(6.0-5.0)

	log.Printf("Exchange rate generated: %.4f", exchangeRate)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(exchangeRate)
}