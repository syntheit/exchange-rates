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

type KrakenResponse struct {
	Error []string `json:"error"`
	Result map[string]struct {
		C []string `json:"c"`
	} `json:"result"`
}

type RatesData struct {
	UpdatedAt   string             `json:"updatedAt"`
	Base        string             `json:"base"`
	Rates       map[string]float64 `json:"rates"`
	CryptoRates map[string]float64 `json:"cryptoRates"`
}

func main() {
	fmt.Println("Fetching DolarApi...")
	arsBody, err := fetch("https://dolarapi.com/v1/dolares")
	if err != nil {
		panic(err)
	}
	var arsData []DolarRate
	if err := json.Unmarshal(arsBody, &arsData); err != nil {
		panic(err)
	}

	ratesMap := make(map[string]float64)
	for _, rate := range arsData {
		switch rate.Casa {
		case "blue":
			ratesMap["ARS_BLUE"] = (rate.Compra + rate.Venta) / 2
		case "oficial":
			ratesMap["ARS_OFFICIAL"] = (rate.Compra + rate.Venta) / 2
		case "cripto":
			ratesMap["ARS_CRYPTO"] = (rate.Compra + rate.Venta) / 2
		case "bolsa":
			ratesMap["ARS_MEP"] = rate.Venta
		}
	}

	if ratesMap["ARS_MEP"] == 0 {
		ratesMap["ARS_MEP"] = ratesMap["ARS_BLUE"]
		fmt.Println("MEP rate was 0, falling back to Blue rate")
	}

	fmt.Println("Fetching Bolivia DolarApi...")
	boliviaBody, err := fetch("https://bo.dolarapi.com/v1/dolares")
	if err != nil {
		panic(err)
	}
	var boliviaData []DolarRate
	if err := json.Unmarshal(boliviaBody, &boliviaData); err != nil {
		panic(err)
	}

	for _, rate := range boliviaData {
		switch rate.Casa {
		case "oficial":
			ratesMap["BOB_OFFICIAL"] = (rate.Compra + rate.Venta) / 2
		case "binance":
			ratesMap["BOB_BLUE"] = rate.Venta
		}
	}

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

	fmt.Println("Fetching Kraken Crypto Rates...")

	cryptoRates := make(map[string]float64)
	krakenMap := map[string]string{
		"XXBTZUSD": "BTC",
		"XETHZUSD": "ETH",
		"BNBUSD":   "BNB",
		"SOLUSD":   "SOL",
		"XXRPZUSD": "XRP",
		"ADAUSD":   "ADA",
		"AVAXUSD":  "AVAX",
		"DOTUSD":   "DOT",
		"LINKUSD":  "LINK",
		"NEARUSD":  "NEAR",
		"APTUSD":   "APT",
		"SUIUSD":   "SUI",
		"TONUSD":   "TON",
		"POLUSD":   "POL",
		"UNIUSD":   "UNI",
		"AAVEUSD":  "AAVE",
		"INJUSD":   "INJ",
		"RENDERUSD": "RNDR",
		"XLTCZUSD": "LTC",
		"BCHUSD":   "BCH",
		"XETCZUSD": "ETC",
		"USDCUSD":  "USDC",
		"DAIUSD":   "DAI",
		"XDGUSD":   "DOGE",
		"SHIBUSD":  "SHIB",
		"PEPEUSD":  "PEPE",
		"WIFUSD":   "WIF",
	}

	pairs := ""
	for k := range krakenMap {
		if pairs != "" {
			pairs += ","
		}
		pairs += k
	}

	krakenBody, err := fetch("https://api.kraken.com/0/public/Ticker?pair=" + pairs)
	if err != nil {
		fmt.Printf("Failed to fetch Kraken rates: %v\n", err)
	} else {
		var krakenData KrakenResponse
		if err := json.Unmarshal(krakenBody, &krakenData); err != nil {
			fmt.Printf("Error unmarshaling Kraken data: %v\n", err)
			panic(err)
		}

		for kName, data := range krakenData.Result {
			symbol, ok := krakenMap[kName]
			if !ok {
				continue
			}
			if len(data.C) > 0 {
				price, err := strconv.ParseFloat(data.C[0], 64)
				if err == nil {
					cryptoRates[symbol] = price
				}
			}
		}
	}

	finalRates := worldData.ConversionRates
	if finalRates == nil {
		finalRates = make(map[string]float64)
	}

	delete(finalRates, "ARS")
	delete(finalRates, "BOB")

	for key, value := range ratesMap {
		finalRates[key] = value
	}

	payload := RatesData{
		UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
		Base:        "USD",
		Rates:       finalRates,
		CryptoRates: cryptoRates,
	}

	file, _ := json.MarshalIndent(payload, "", "  ")
	_ = os.WriteFile("rates.json", file, 0644)

	fmt.Println("Oracle Update Complete: rates.json saved.")
}

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
