package quant

import (
	"TickerAnalyzer/data"
	"fmt"
	"math"
	"strings"
	"time"
)

func CalculateBeta(tickerData []float64, marketData []float64, tickerReturnsMean float64, marketReturnsMean float64) (float64, error) {

	// calclulate covariance
	if len(tickerData) != len(marketData) {
		return 0, fmt.Errorf("The ticker and Market data should have the same number of entries")
	}

	var covarianceSum float64
	var marketVarianceSum float64
	for i := 0; i < len(tickerData); i++ {
		market_returnDiff := marketData[i] - marketReturnsMean
		marketVarianceSum += math.Pow(market_returnDiff, 2)
		covarianceSum += ((tickerData[i] - tickerReturnsMean) * (market_returnDiff))
	}

	return covarianceSum / marketVarianceSum, nil
}

func CalculateBetaFromTicker(ticker string, market string, timeFrame string) (float64, error) {
	ticker = strings.ToUpper(strings.TrimSpace(ticker))
	market = strings.ToUpper(strings.TrimSpace(market))
	url := data.BaseURL + "bars"
	end := time.Now().UTC()
	var start time.Time

	// Hard coded this just because I know the Yandex/nbis fiasco, would need to implement a system to handle this in the future
	if ticker != "NBIS" {
		start = end.AddDate(-5, 0, 0)
	} else {
		start = time.Date(2024, time.October, 10, 0, 0, 0, 0, time.UTC)
	}

	startStr := start.Format(time.RFC3339)
	endStr := end.Format(time.RFC3339)
	feed := "iex" // Only feed that can be used without subscription
	var alpacaData data.AlpacaResponse

	err := data.MakeRequestToAlpaca(url,
		map[string]string{"symbols": ticker + "," + market, "start": startStr, "end": endStr, "feed": feed, "timeframe": timeFrame},
		&alpacaData)
	if err != nil {
		return 0, err
	}

	if len(alpacaData.Bars) < 2 {
		return 0, fmt.Errorf("Some data was not available")
	}

	tickerData, exists := alpacaData.Bars[ticker]
	if !exists {
		return 0, fmt.Errorf("Ticker data not available")
	}
	marketData, exists := alpacaData.Bars[market]
	if !exists {
		return 0, fmt.Errorf("Market data not available")
	}

	minLen := min(len(tickerData), len(marketData))
	alignedTickerData := make([]float64, 0, minLen)
	alignedMarketData := make([]float64, 0, minLen)

	// map ticker data
	ticker_map := make(map[string]float64, len(tickerData))
	for _, dat := range tickerData {
		ticker_map[dat.Timestamp] = dat.Close
	}

	// We need to align the data
	for i := 0; i < len(marketData); i++ {
		marDat := marketData[i]
		tDat, exists := ticker_map[marDat.Timestamp]
		if exists {
			alignedTickerData = append(alignedTickerData, tDat)
			alignedMarketData = append(alignedMarketData, marDat.Close)
		}
	}

	tickerReturns := make([]float64, 0, len(alignedTickerData))
	marketReturns := make([]float64, 0, len(alignedTickerData))
	var tickerReturnsMean float64
	var marketReturnsMean float64
	var tickerReturnsSum float64
	var marketReturnsSum float64

	for i := 1; i < len(alignedTickerData); i++ {
		prev := alignedTickerData[i-1]
		cur := alignedTickerData[i]
		ret := (cur - prev) / prev
		tickerReturnsSum += ret
		tickerReturns = append(tickerReturns, ret)

		prev = alignedMarketData[i-1]
		cur = alignedMarketData[i]
		ret = (cur - prev) / prev
		marketReturnsSum += ret
		marketReturns = append(marketReturns, ret)
	}
	tickerReturnsMean = tickerReturnsSum / float64(len(alignedTickerData))
	marketReturnsMean = marketReturnsSum / float64(len(alignedMarketData))

	beta, err := CalculateBeta(tickerReturns, marketReturns, tickerReturnsMean, marketReturnsMean)
	if err != nil {
		return 0, err
	}
	return beta, nil
}
