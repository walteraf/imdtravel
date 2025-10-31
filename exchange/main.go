package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
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

	// rand.Intn(1001) gera um número entre [0, 1000]
	// 5000 + [0, 1000] = [5000, 6000]
	intValue := 5000 + rand.Intn(1001) // 1001 = (6000 - 5000 + 1)

	// Convertemos para float dividindo por 1000.0
	exchangeRate := float64(intValue) / 1000.0

	log.Printf("Exchange rate generated: %.4f", exchangeRate)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(exchangeRate)
}
