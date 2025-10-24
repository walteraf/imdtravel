package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Flight struct {
	Flight string  `json:"flight"`
	Day    string  `json:"day"`
	Value  float64 `json:"value"`
}

type SellRequest struct {
	Flight string `json:"flight"`
	Day    string `json:"day"`
}

type SellResponse struct {
	ID string `json:"id"`
}

type Transaction struct {
	ID     string
	Flight string
	Day    string
	Date   time.Time
}

var (
	// Simulação de banco de dados de voos
	flights = map[string]Flight{
		"AA123-2025-11-15": {Flight: "AA123", Day: "2025-11-15", Value: 500.00},
		"AA123-2025-11-20": {Flight: "AA123", Day: "2025-11-20", Value: 550.00},
		"BA456-2025-11-15": {Flight: "BA456", Day: "2025-11-15", Value: 750.00},
		"BA456-2025-12-01": {Flight: "BA456", Day: "2025-12-01", Value: 800.00},
		"LA789-2025-11-25": {Flight: "LA789", Day: "2025-11-25", Value: 450.00},
		"LA789-2025-12-10": {Flight: "LA789", Day: "2025-12-10", Value: 480.00},
		"UA999-2025-11-30": {Flight: "UA999", Day: "2025-11-30", Value: 920.00},
		"DL555-2025-12-05": {Flight: "DL555", Day: "2025-12-05", Value: 680.00},
	}
	
	// Armazena transações realizadas
	transactions = make(map[string]Transaction)
	mu           sync.RWMutex
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func main() {
	http.HandleFunc("/flight", getFlightHandler)
	http.HandleFunc("/sell", sellTicketHandler)
	http.HandleFunc("/health", healthHandler)

	port := ":8081"
	log.Printf("AirlinesHub service starting on port %s", port)
	log.Fatal(http.ListenAndServe(port, nil))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

func getFlightHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	flightNumber := r.URL.Query().Get("flight")
	day := r.URL.Query().Get("day")

	if flightNumber == "" || day == "" {
		respondError(w, "Missing required parameters: flight and day", http.StatusBadRequest)
		return
	}

	key := flightNumber + "-" + day
	
	mu.RLock()
	flight, exists := flights[key]
	mu.RUnlock()

	if !exists {
		respondError(w, "Flight not found", http.StatusNotFound)
		return
	}

	log.Printf("Flight query: %s on %s - Value: $%.2f", flightNumber, day, flight.Value)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(flight)
}

func sellTicketHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SellRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Flight == "" || req.Day == "" {
		respondError(w, "Missing required fields: flight and day", http.StatusBadRequest)
		return
	}

	// Verificar se o voo existe
	key := req.Flight + "-" + req.Day
	mu.RLock()
	_, exists := flights[key]
	mu.RUnlock()

	if !exists {
		respondError(w, "Flight not found", http.StatusNotFound)
		return
	}

	// Gerar ID único para a transação
	transactionID := uuid.New().String()

	transaction := Transaction{
		ID:     transactionID,
		Flight: req.Flight,
		Day:    req.Day,
		Date:   time.Now(),
	}

	mu.Lock()
	transactions[transactionID] = transaction
	mu.Unlock()

	log.Printf("Ticket sold: transaction_id=%s, flight=%s, day=%s", transactionID, req.Flight, req.Day)

	response := SellResponse{
		ID: transactionID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func respondError(w http.ResponseWriter, message string, statusCode int) {
	response := map[string]string{
		"error": message,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}