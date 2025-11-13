package main

import (
	"encoding/json"
	"log"
	"math/rand/v2"
	"net/http"
	"os"
	"sync"
	"time"
)

type BonusRequest struct {
	User  string `json:"user"`
	Bonus int    `json:"bonus"`
}

type BonusRecord struct {
	User      string
	Bonus     int
	Timestamp time.Time
}

type UserPoints struct {
	User        string
	TotalPoints int
	Records     []BonusRecord
}

var (
	// Simulação de banco de dados de pontos
	userPoints = make(map[string]*UserPoints)
	mu         sync.RWMutex
)

func main() {
	http.HandleFunc("/bonus", registerBonusHandler)
	http.HandleFunc("/points", getPointsHandler)
	http.HandleFunc("/health", healthHandler)

	port := ":8083"
	log.Printf("Fidelity service starting on port %s", port)
	log.Fatal(http.ListenAndServe(port, nil))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

// Request 4: Fail (Crash, 0.02, _)
// 2% de chance de crashar (serviço para completamente)
func registerBonusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// FALHA: Crash com 2% de probabilidade
	if rand.Float64() < 0.02 {
		log.Println("[FAULT] Request 4: Crash fault triggered - Service shutting down")
		// Força o crash do serviço
		os.Exit(1)
	}

	var req BonusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.User == "" {
		respondError(w, "Missing required field: user", http.StatusBadRequest)
		return
	}

	if req.Bonus <= 0 {
		respondError(w, "Bonus must be greater than 0", http.StatusBadRequest)
		return
	}

	record := BonusRecord{
		User:      req.User,
		Bonus:     req.Bonus,
		Timestamp: time.Now(),
	}

	mu.Lock()
	if userPoints[req.User] == nil {
		userPoints[req.User] = &UserPoints{
			User:        req.User,
			TotalPoints: 0,
			Records:     make([]BonusRecord, 0),
		}
	}
	userPoints[req.User].TotalPoints += req.Bonus
	userPoints[req.User].Records = append(userPoints[req.User].Records, record)
	mu.Unlock()

	log.Printf("Bonus registered: user=%s, bonus=%d, total=%d",
		req.User, req.Bonus, userPoints[req.User].TotalPoints)

	response := map[string]interface{}{
		"success":      true,
		"user":         req.User,
		"bonus_added":  req.Bonus,
		"total_points": userPoints[req.User].TotalPoints,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func getPointsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user := r.URL.Query().Get("user")
	if user == "" {
		respondError(w, "Missing required parameter: user", http.StatusBadRequest)
		return
	}

	mu.RLock()
	points, exists := userPoints[user]
	mu.RUnlock()

	if !exists {
		response := map[string]interface{}{
			"user":         user,
			"total_points": 0,
			"records":      []BonusRecord{},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(points)
}

func respondError(w http.ResponseWriter, message string, statusCode int) {
	response := map[string]string{
		"error": message,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}
