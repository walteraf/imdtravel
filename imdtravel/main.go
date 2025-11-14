package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"sync"
	"time"
)

// Request 0: Adicionado parâmetro ft (fault tolerance)
type BuyTicketRequest struct {
	Flight string `json:"flight"`
	Day    string `json:"day"`
	User   string `json:"user"`
	FT     bool   `json:"ft"` // Parâmetro de tolerância a falhas
}

type BuyTicketResponse struct {
	Success       bool    `json:"success"`
	Message       string  `json:"message,omitempty"`
	Error         string  `json:"error,omitempty"`
	TransactionID string  `json:"transaction_id,omitempty"`
	Flight        string  `json:"flight,omitempty"`
	Day           string  `json:"day,omitempty"`
	ValueUSD      float64 `json:"value_usd,omitempty"`
	ValueBRL      float64 `json:"value_brl,omitempty"`
	ExchangeRate  float64 `json:"exchange_rate,omitempty"`
	BonusPoints   int     `json:"bonus_points,omitempty"`
	BonusStatus   string  `json:"bonus_status,omitempty"` // Indica se bônus foi processado ou está pendente
}

type FlightResponse struct {
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

type BonusRequest struct {
	User  string `json:"user"`
	Bonus int    `json:"bonus"`
}

// Request 4: Estrutura para fila de bonificações pendentes
type PendingBonus struct {
	User        string
	Bonus       int
	Attempts    int
	LastAttempt time.Time
	CreatedAt   time.Time
}

var (
	airlinesHubURL = getEnv("AIRLINESHUB_URL", "http://localhost:8081")
	exchangeURL    = getEnv("EXCHANGE_URL", "http://localhost:8082")
	fidelityURL    = getEnv("FIDELITY_URL", "http://localhost:8083")

	// Request 4: Fila de bonificações pendentes
	pendingBonuses   = make(map[string]*PendingBonus) // key: user_timestamp
	pendingBonusesMu sync.RWMutex
)

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func main() {
	http.HandleFunc("/buyTicket", buyTicketHandler)
	http.HandleFunc("/health", healthHandler)

	// Request 4: Iniciar goroutine para processar bonificações pendentes
	go processPendingBonuses()

	port := ":8080"
	log.Printf("IMDTravel service starting on port %s", port)
	log.Fatal(http.ListenAndServe(port, nil))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

func buyTicketHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req BuyTicketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validar campos obrigatórios
	if req.Flight == "" || req.Day == "" || req.User == "" {
		respondError(w, "Missing required fields: flight, day, user", http.StatusBadRequest)
		return
	}

	log.Printf("Processing ticket purchase: flight=%s, day=%s, user=%s, ft=%v", 
		req.Flight, req.Day, req.User, req.FT)

	// Request 1: Consultar voo no AirlinesHub
	flight, err := getFlightInfo(req.Flight, req.Day)
	if err != nil {
		log.Printf("Error getting flight info: %v", err)
		respondError(w, fmt.Sprintf("Failed to get flight info: %v", err), http.StatusInternalServerError)
		return
	}

	// Request 2: Consultar taxa de câmbio (com timeout de 1s)
	exchangeRate, err := getExchangeRate()
	if err != nil {
		log.Printf("Error getting exchange rate: %v", err)
		respondError(w, fmt.Sprintf("Failed to get exchange rate: %v", err), http.StatusInternalServerError)
		return
	}

	// Calcular valor em reais
	valueBRL := flight.Value * exchangeRate

	// Request 3: Registrar venda no AirlinesHub
	transactionID, err := sellTicket(req.Flight, req.Day)
	if err != nil {
		log.Printf("Error selling ticket: %v", err)
		respondError(w, fmt.Sprintf("Failed to sell ticket: %v", err), http.StatusInternalServerError)
		return
	}

	// Request 4: Registrar bônus no Fidelity
	bonusPoints := int(math.Round(flight.Value))
	bonusStatus := "processed"

	if req.FT {
		// COM TOLERÂNCIA A FALHAS: Não impedir venda se Fidelity falhar
		if err := registerBonusWithRetry(req.User, bonusPoints, 3); err != nil {
			log.Printf("Warning: Failed to register bonus immediately: %v", err)
			log.Printf("[FAULT TOLERANCE] Adding bonus to pending queue")
			
			// Adicionar à fila de pendentes
			addPendingBonus(req.User, bonusPoints)
			bonusStatus = "pending"
		}
	} else {
		// SEM TOLERÂNCIA A FALHAS: Venda falha se Fidelity falhar
		if err := registerBonus(req.User, bonusPoints); err != nil {
			log.Printf("Error registering bonus: %v", err)
			respondError(w, fmt.Sprintf("Failed to register bonus: %v", err), http.StatusInternalServerError)
			return
		}
	}

	// Resposta de sucesso
	response := BuyTicketResponse{
		Success:       true,
		Message:       "Ticket purchased successfully",
		TransactionID: transactionID,
		Flight:        req.Flight,
		Day:           req.Day,
		ValueUSD:      flight.Value,
		ValueBRL:      valueBRL,
		ExchangeRate:  exchangeRate,
		BonusPoints:   bonusPoints,
		BonusStatus:   bonusStatus,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)

	log.Printf("Purchase completed: transaction_id=%s, bonus_status=%s", transactionID, bonusStatus)
}

func getFlightInfo(flight, day string) (*FlightResponse, error) {
	url := fmt.Sprintf("%s/flight?flight=%s&day=%s", airlinesHubURL, flight, day)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("service returned status %d: %s", resp.StatusCode, string(body))
	}

	var flightResp FlightResponse
	if err := json.NewDecoder(resp.Body).Decode(&flightResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &flightResp, nil
}

func getExchangeRate() (float64, error) {
	url := fmt.Sprintf("%s/convert", exchangeURL)

	client := &http.Client{Timeout: 1 * time.Second}
	start := time.Now()

	resp, err := client.Get(url)
	elapsed := time.Since(start)

	if err != nil {
		return 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Validar timeout de 1 segundo
	if elapsed > 1*time.Second {
		return 0, fmt.Errorf("exchange service timeout exceeded 1s (took %v)", elapsed)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("service returned status %d: %s", resp.StatusCode, string(body))
	}

	var rate float64
	if err := json.NewDecoder(resp.Body).Decode(&rate); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	return rate, nil
}

func sellTicket(flight, day string) (string, error) {
	url := fmt.Sprintf("%s/sell", airlinesHubURL)

	reqBody := SellRequest{
		Flight: flight,
		Day:    day,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("service returned status %d: %s", resp.StatusCode, string(body))
	}

	var sellResp SellResponse
	if err := json.NewDecoder(resp.Body).Decode(&sellResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return sellResp.ID, nil
}

// Request 4: Função básica de registro de bônus (sem retry)
func registerBonus(user string, bonus int) error {
	url := fmt.Sprintf("%s/bonus", fidelityURL)

	reqBody := BonusRequest{
		User:  user,
		Bonus: bonus,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("service returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Request 4: Função de registro com retry imediato
func registerBonusWithRetry(user string, bonus int, maxRetries int) error {
	var lastErr error
	
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := registerBonus(user, bonus)
		if err == nil {
			if attempt > 1 {
				log.Printf("[FAULT TOLERANCE] Bonus registered after %d attempts", attempt)
			}
			return nil
		}
		
		lastErr = err
		log.Printf("[FAULT TOLERANCE] Bonus registration attempt %d/%d failed: %v", 
			attempt, maxRetries, err)
		
		if attempt < maxRetries {
			// Backoff exponencial: 100ms, 200ms, 400ms...
			backoff := time.Duration(100*attempt) * time.Millisecond
			time.Sleep(backoff)
		}
	}
	
	return fmt.Errorf("all %d retry attempts failed: %w", maxRetries, lastErr)
}

// Request 4: Adicionar bônus à fila de pendentes
func addPendingBonus(user string, bonus int) {
	key := fmt.Sprintf("%s_%d", user, time.Now().UnixNano())
	
	pending := &PendingBonus{
		User:        user,
		Bonus:       bonus,
		Attempts:    0,
		LastAttempt: time.Time{},
		CreatedAt:   time.Now(),
	}
	
	pendingBonusesMu.Lock()
	pendingBonuses[key] = pending
	pendingBonusesMu.Unlock()
	
	log.Printf("[PENDING QUEUE] Added bonus for user %s: %d points (total pending: %d)", 
		user, bonus, len(pendingBonuses))
}

// Request 4: Processar fila de bonificações pendentes em background
func processPendingBonuses() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	log.Println("[PENDING QUEUE] Background processor started")
	
	for range ticker.C {
		pendingBonusesMu.Lock()
		if len(pendingBonuses) == 0 {
			pendingBonusesMu.Unlock()
			continue
		}
		
		log.Printf("[PENDING QUEUE] Processing %d pending bonuses", len(pendingBonuses))
		
		// Copiar chaves para processar sem manter lock
		keys := make([]string, 0, len(pendingBonuses))
		for key := range pendingBonuses {
			keys = append(keys, key)
		}
		pendingBonusesMu.Unlock()
		
		// Processar cada bonificação pendente
		for _, key := range keys {
			pendingBonusesMu.RLock()
			pending, exists := pendingBonuses[key]
			pendingBonusesMu.RUnlock()
			
			if !exists {
				continue
			}
			
			// Limitar tentativas (máximo 20 tentativas)
			if pending.Attempts >= 20 {
				log.Printf("[PENDING QUEUE] Max attempts reached for %s, removing from queue", key)
				pendingBonusesMu.Lock()
				delete(pendingBonuses, key)
				pendingBonusesMu.Unlock()
				continue
			}
			
			// Tentar processar
			pending.Attempts++
			pending.LastAttempt = time.Now()
			
			err := registerBonus(pending.User, pending.Bonus)
			if err == nil {
				log.Printf("[PENDING QUEUE] Successfully processed bonus for user %s after %d attempts", 
					pending.User, pending.Attempts)
				pendingBonusesMu.Lock()
				delete(pendingBonuses, key)
				pendingBonusesMu.Unlock()
			} else {
				log.Printf("[PENDING QUEUE] Attempt %d failed for user %s: %v", 
					pending.Attempts, pending.User, err)
			}
		}
	}
}

func respondError(w http.ResponseWriter, message string, statusCode int) {
	response := BuyTicketResponse{
		Success: false,
		Error:   message,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}