package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// 1. Structs for the APIs (Strict Typing)
type DolarRate struct {
	Casa   string  `json:"casa"`
	Compra float64 `json:"compra"`
	Venta  float64 `json:"venta"`
}

type WorldRates struct {
	Result          string             `json:"result"`
	ConversionRates map[string]float64 `json:"conversion_rates"`
}

// 2. The Final Output Schema
type OraclePayload struct {
	UpdatedAt string             `json:"updatedAt"`
	Base      string             `json:"base"`
	Rates     map[string]float64 `json:"rates"`
}

func main() {
	// --- Step 1: Fetch Argentina (Blue/MEP) ---
	fmt.Println("Fetching DolarApi...")
	arsBody := fetch("https://dolarapi.com/v1/dolares")
	var arsData []DolarRate
	if err := json.Unmarshal(arsBody, &arsData); err != nil {
		panic(err)
	}

	// Extract the specific rates we care about with "Expat Math" Logic
	ratesMap := make(map[string]float64)
	for _, rate := range arsData {
		switch rate.Casa {
		case "blue":
			// Blue: Calculate the Midrate (Compra + Venta) / 2
			ratesMap["ARS_BLUE"] = (rate.Compra + rate.Venta) / 2
		case "oficial":
			// Official: Calculate the Midrate (Compra + Venta) / 2
			ratesMap["ARS_OFFICIAL"] = (rate.Compra + rate.Venta) / 2
		case "cripto":
			// Crypto: Calculate the Midrate (Compra + Venta) / 2
			ratesMap["ARS_CRYPTO"] = (rate.Compra + rate.Venta) / 2
		case "bolsa":
			// MEP/Card (bolsa): Use the Sell Rate (Venta) only
			ratesMap["ARS_MEP"] = rate.Venta
		}
	}

	// Safety check: If MEP rate is 0, fallback to Blue rate
	if ratesMap["ARS_MEP"] == 0 {
		ratesMap["ARS_MEP"] = ratesMap["ARS_BLUE"]
		fmt.Println("MEP rate was 0, falling back to Blue rate")
	}

	// --- Step 2: Fetch World (EUR, BRL, etc.) ---
	fmt.Println("Fetching World Rates...")
	apiKey := os.Getenv("EXCHANGE_KEY")
	if apiKey == "" {
		panic("Missing EXCHANGE_KEY environment variable")
	}
	worldBody := fetch(fmt.Sprintf("https://v6.exchangerate-api.com/v6/%s/latest/USD", apiKey))
	var worldData WorldRates
	if err := json.Unmarshal(worldBody, &worldData); err != nil {
		panic(err)
	}

	// --- Step 3: Merge & Overwrite ---
	// Start with the world rates
	finalRates := worldData.ConversionRates

	// Remove exchangerate-api's ARS rate
	delete(finalRates, "ARS")

	// Inject our custom Argentina rates
	for key, value := range ratesMap {
		finalRates[key] = value
	}

	payload := OraclePayload{
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		Base:      "USD",
		Rates:     finalRates,
	}

	// --- Step 4: Save to Disk ---
	file, _ := json.MarshalIndent(payload, "", "  ")
	_ = os.WriteFile("rates.json", file, 0644)

	fmt.Println("Oracle Update Complete: rates.json saved.")
}

// Helper: HTTP GET
func fetch(url string) []byte {
	resp, err := http.Get(url)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return body
}
