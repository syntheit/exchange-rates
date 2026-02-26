package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

// 1. Structs for the APIs (Strict Typing)
type DolarRate struct {
	Casa   string  `json:"casa"`
	Compra float64 `json:"compra"`
	Venta  float64 `json:"venta"`
}

type BoliviaRate struct {
	Compra             float64 `json:"compra"`
	Venta              float64 `json:"venta"`
	Casa               string  `json:"casa"`
	Nombre             string  `json:"nombre"`
	Moneda             string  `json:"moneda"`
	FechaActualizacion string  `json:"fechaActualizacion"`
}

type WorldRates struct {
	Result          string             `json:"result"`
	ConversionRates map[string]float64 `json:"conversion_rates"`
}

type CoinCapResponse struct {
	Data []struct {
		Symbol   string `json:"symbol"`
		PriceUsd string `json:"priceUsd"`
	} `json:"data"`
}

// 2. The Final Output Schema
type RatesData struct {
	UpdatedAt   string             `json:"updatedAt"`
	Base        string             `json:"base"`
	Rates       map[string]float64 `json:"rates"`
	CryptoRates map[string]float64 `json:"cryptoRates"`
}

func main() {
	// --- Step 1: Fetch Argentina (Blue/MEP) ---
	fmt.Println("Fetching DolarApi...")
	arsBody, err := fetch("https://dolarapi.com/v1/dolares")
	if err != nil {
		panic(err)
	}
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

	// --- Step 1.5: Fetch Bolivia (Official and Blue) ---
	fmt.Println("Fetching Bolivia DolarApi...")
	boliviaBody, err := fetch("https://bo.dolarapi.com/v1/dolares")
	if err != nil {
		panic(err)
	}
	var boliviaData []DolarRate
	if err := json.Unmarshal(boliviaBody, &boliviaData); err != nil {
		panic(err)
	}

	// Extract the specific rates we care about with "Expat Math" Logic
	for _, rate := range boliviaData {
		switch rate.Casa {
		case "oficial":
			// Official: Calculate the Midrate (Compra + Venta) / 2
			ratesMap["BOB_OFFICIAL"] = (rate.Compra + rate.Venta) / 2
		case "binance":
			// Blue (Binance/Crypto): Use the Sell Rate (Venta) only since Compra is null
			ratesMap["BOB_BLUE"] = rate.Venta
		}
	}

	// --- Step 2: Fetch World (EUR, BRL, etc.) ---
	fmt.Println("Fetching World Rates...")
	apiKey := os.Getenv("EXCHANGE_KEY")
	if apiKey == "" {
		panic("Missing EXCHANGE_KEY environment variable")
	}
	worldBody, err := fetch(fmt.Sprintf("https://v6.exchangerate-api.com/v6/%s/latest/USD", apiKey))
	if err != nil {
		panic(err)
	}
	var worldData WorldRates
	if err := json.Unmarshal(worldBody, &worldData); err != nil {
		panic(err)
	}

	// --- Step 2.5: Fetch Crypto (CoinCap) ---
	fmt.Println("Fetching CoinCap Crypto Rates...")

	cryptoRates := make(map[string]float64)
	targetSymbols := map[string]bool{
		"BTC":  true,
		"ETH":  true,
		"BNB":  true,
		"SOL":  true,
		"XRP":  true,
		"ADA":  true,
		"AVAX": true,
		"DOT":  true,
		"LINK": true,
		"NEAR": true,
		"APT":  true,
		"SUI":  true,
		"TON":  true,
		"POL":  true,
		"UNI":  true,
		"AAVE": true,
		"MKR":  true,
		"INJ":  true,
		"RNDR": true,
		"LTC":  true,
		"BCH":  true,
		"ETC":  true,
		"USDC": true,
		"DAI":  true,
		"FDUSD": true,
		"DOGE": true,
		"SHIB": true,
		"PEPE": true,
		"WIF":  true,
	}

	coinCapBody, err := fetch("https://api.coincap.io/v2/assets?limit=500")
	if err != nil {
		fmt.Printf("Failed to fetch CoinCap rates: %v\n", err)
		// Don't panic here, just skip crypto if it fails
	} else {
		var coinCapData CoinCapResponse
		if err := json.Unmarshal(coinCapBody, &coinCapData); err != nil {
			fmt.Printf("Error unmarshaling CoinCap data: %v\n", err)
			fmt.Printf("Response body: %s\n", string(coinCapBody))
			panic(err)
		}

		for _, p := range coinCapData.Data {
			if targetSymbols[p.Symbol] {
				price, err := strconv.ParseFloat(p.PriceUsd, 64)
				if err != nil {
					continue
				}
				cryptoRates[p.Symbol] = price
			}
		}
	}

	// --- Step 3: Merge & Overwrite ---
	// Start with the world rates
	finalRates := worldData.ConversionRates
	if finalRates == nil {
		finalRates = make(map[string]float64)
	}

	// Remove exchangerate-api's ARS and BOB rates
	delete(finalRates, "ARS")
	delete(finalRates, "BOB")

	// Inject our custom Argentina rates
	for key, value := range ratesMap {
		finalRates[key] = value
	}

	payload := RatesData{
		UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
		Base:        "USD",
		Rates:       finalRates,
		CryptoRates: cryptoRates,
	}

	// --- Step 4: Save to Disk ---
	file, _ := json.MarshalIndent(payload, "", "  ")
	_ = os.WriteFile("rates.json", file, 0644)

	fmt.Println("Oracle Update Complete: rates.json saved.")
}

// Helper: HTTP GET
func fetch(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}
