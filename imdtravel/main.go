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
	"time"
)

type BuyTicketRequest struct {
	Flight string `json:"flight"`
	Day    string `json:"day"`
	User   string `json:"user"`
	FT     bool   `json:"ft,omitempty"`
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

var (
	airlinesHubURL = getEnv("AIRLINESHUB_URL", "http://localhost:8081")
	exchangeURL    = getEnv("EXCHANGE_URL", "http://localhost:8082")
	fidelityURL    = getEnv("FIDELITY_URL", "http://localhost:8083")
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

	log.Printf("Processing ticket purchase: flight=%s, day=%s, user=%s, ft=%t", req.Flight, req.Day, req.User, req.FT)

	// Request 1: Consultar voo no AirlinesHub
	flight, err := getFlightInfo(req.Flight, req.Day, req.FT)
	if err != nil {
		log.Printf("Error getting flight info: %v", err)
		respondError(w, fmt.Sprintf("Failed to get flight info: %v", err), http.StatusInternalServerError)
		return
	}

	// Request 2: Consultar taxa de câmbio (com timeout de 1s)
	exchangeRate, err := getExchangeRate(req.FT)
	if err != nil {
		log.Printf("Error getting exchange rate: %v", err)
		respondError(w, fmt.Sprintf("Failed to get exchange rate: %v", err), http.StatusInternalServerError)
		return
	}

	// Calcular valor em reais
	valueBRL := flight.Value * exchangeRate

	// Request 3: Registrar venda no AirlinesHub
	transactionID, err := sellTicket(req.Flight, req.Day, req.FT)
	if err != nil {
		log.Printf("Error selling ticket: %v", err)
		respondError(w, fmt.Sprintf("Failed to sell ticket: %v", err), http.StatusInternalServerError)
		return
	}

	// Request 4: Registrar bônus no Fidelity
	bonusPoints := int(math.Round(flight.Value))
	if err := registerBonus(req.User, bonusPoints, req.FT); err != nil {
		log.Printf("Error registering bonus: %v", err)
		respondError(w, fmt.Sprintf("Failed to register bonus: %v", err), http.StatusInternalServerError)
		return
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
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)

	log.Printf("Purchase completed successfully: transaction_id=%s", transactionID)
}

func getFlightInfo(flight, day string, ft bool) (*FlightResponse, error) {
	url := fmt.Sprintf("%s/flight?flight=%s&day=%s", airlinesHubURL, flight, day)
	
	client := &http.Client{Timeout: 5 * time.Second} 
	resp, err := client.Get(url)
	if err != nil {
		// --- Falha na Tentativa 1 (Erro de Rede/Timeout) ---
		log.Printf("[R1] Attempt 1 failed (timeout/net error): %v", err)

		// Se FT (Tolerância a Falhas) estiver DESLIGADO, falha imediatamente.
		if !ft {
			return nil, fmt.Errorf("request failed: %w", err)
		}

		// --- FT LIGADO: Iniciar Lógica de Retry ---
		log.Println("[FT R1] FT is ON: Using Retry Strategy (3 more attempts).")
		
		const maxRetries = 4
		var lastErr = err

		// Começa o loop a partir da segunda tentativa
		for attempt := 2; attempt <= maxRetries; attempt++ {
			time.Sleep(500 * time.Millisecond) // Espera antes de tentar de novo
			log.Printf("[FT R1] Attempt %d/%d...", attempt, maxRetries)
			
			resp, err := client.Get(url)
			if err != nil {
				// Erro de rede/timeout
				lastErr = fmt.Errorf("attempt %d: request failed (timeout/net error): %w", attempt, err)
				log.Println(lastErr)
				continue
			}
			// Se o status não for OK
			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				return nil, fmt.Errorf("service returned non-OK status %d: %s", resp.StatusCode, string(body))
			}
			// Sucesso na retentativa
			var flightResp FlightResponse
			if err := json.NewDecoder(resp.Body).Decode(&flightResp); err != nil {
				resp.Body.Close()
				return nil, fmt.Errorf("attempt %d: failed to decode response: %w", attempt, err)
			}
			resp.Body.Close()
			log.Printf("[FT R1] Success on attempt %d", attempt)
			return &flightResp, nil
		}

		return nil, fmt.Errorf("all retries failed for Request 1: %w", lastErr)
	}

	// --- Sucesso na Tentativa 1 (err == nil) ---
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		defer resp.Body.Close()
		return nil, fmt.Errorf("service returned status %d: %s", resp.StatusCode, string(body))
	}

	var flightResp FlightResponse
	if err := json.NewDecoder(resp.Body).Decode(&flightResp); err != nil {
		defer resp.Body.Close()
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	defer resp.Body.Close()
	return &flightResp, nil
}

func getExchangeRate(ft bool) (float64, error) {
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

func sellTicket(flight, day string, ft bool) (string, error) {
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

func registerBonus(user string, bonus int, ft bool) error {
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

func respondError(w http.ResponseWriter, message string, statusCode int) {
	response := BuyTicketResponse{
		Success: false,
		Error:   message,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}
