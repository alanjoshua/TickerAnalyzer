package data

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
	"time"
)

type AlpacaResponse struct {
	Bars map[string][]struct {
		Close     float64 `json:"c"`
		Timestamp string  `json:"t"`
	} `json:"bars"`
}

type StockMetrics struct {
	CurrentPrice float64
	DailyDrift   float64
	DailyVol     float64
}

const BaseURL = "https://data.alpaca.markets/v2/stocks/"

func MakeRequestToAlpaca(url string, params map[string]string, alpacaData *AlpacaResponse) error {
	apiKey := strings.TrimSpace(os.Getenv("ALPACA_KEY"))
	apiSecret := strings.TrimSpace(os.Getenv("ALPACA_SECRET"))

	if apiKey == "" || apiSecret == "" {
		return fmt.Errorf("missing Alpaca credentials. Check your environment variables")
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	q := req.URL.Query()
	for key, value := range params {
		q.Add(key, value)
	}
	req.URL.RawQuery = q.Encode()

	req.Header.Add("APCA-API-KEY-ID", apiKey)
	req.Header.Add("APCA-API-SECRET-KEY", apiSecret)
	req.Header.Add("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		// Read the body payload Alpaca sent back
		bodyBytes, _ := io.ReadAll(resp.Body)
		// Return the exact message so we can see it on the frontend
		return fmt.Errorf("Alpaca HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse JSON
	if err := json.NewDecoder(resp.Body).Decode(&alpacaData); err != nil {
		return err
	}

	return nil
}

func FetchStockMetrics(ticker string) (StockMetrics, error) {
	cleanTicker := strings.ToUpper(strings.TrimSpace(ticker))
	end := time.Now().UTC()
	start := end.AddDate(-1, 0, 0)
	startStr := start.Format(time.RFC3339)
	endStr := end.Format(time.RFC3339)
	timeFrame := "1D"
	url := BaseURL + "bars"
	feed := "iex" // Only feed that can be used without subscription
	var alpacaData AlpacaResponse

	err := MakeRequestToAlpaca(url,
		map[string]string{"symbols": cleanTicker, "start": startStr, "end": endStr, "feed": feed, "timeframe": timeFrame},
		&alpacaData)
	if err != nil {
		return StockMetrics{}, err
	}

	stockData, exists := alpacaData.Bars[cleanTicker]
	if !exists {
		return StockMetrics{}, fmt.Errorf("Price history data is not available for the ticker %s", cleanTicker)
	}
	dailyReturns := make([]float64, 0, len(stockData))
	var sumReturns float64

	for i := 1; i < len(stockData); i++ {
		prevClose := stockData[i-1].Close
		curClose := stockData[i].Close
		// curReturn := (curClose - prevClose) / prevClose
		curReturn := math.Log(curClose / prevClose)
		dailyReturns = append(dailyReturns, curReturn)
		sumReturns += curReturn
	}
	meanReturn := sumReturns / float64(len(dailyReturns))

	var varianceSum float64
	for _, ret := range dailyReturns {
		varianceSum += math.Pow(ret-meanReturn, 2)
	}
	variance := varianceSum / float64(len(dailyReturns))
	dailyVol := math.Sqrt(variance)

	return StockMetrics{
		CurrentPrice: stockData[len(stockData)-1].Close,
		DailyDrift:   meanReturn,
		DailyVol:     dailyVol,
	}, nil
}
