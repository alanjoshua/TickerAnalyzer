package main

import (
	"TickerAnalyzer/data"
	"TickerAnalyzer/quant"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

var Formatter = message.NewPrinter(language.English)
var EQUITYRISKPREMIUM = 0.045

type RiskAnalysisData struct {
	Ticker        string
	CurrentPrice  float64
	AnnVol        float64
	AnnDrift      float64
	DailyDrag     float64
	MedianPrice   float64
	ExpectedPrice float64
	VaR           float64
	Beta          float64
	ChartData     template.JS
}

type RowData struct {
	YearLabel       string
	RevGrowth       float64
	OpMargin        float64
	TaxRate         float64
	SalesToCapRatio float64
	Wacc            float64
}

type DCFTemplateData struct {
	BaseRevenue       float64
	TotalCash         float64
	TotalDebt         float64
	SharesOutstanding float64
	DataSource        string
	Rows              []RowData
}

func main() {

	// Load API key
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env found")
	}

	// Serve the frontend UI (index.html) on the root route "/"
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, "index.html") })

	// Perform risk analysis
	http.HandleFunc("/api/analyze", riskAnalysisHandler)

	// Perform DCF (running risk analysis also automatically runs dcf)
	http.HandleFunc("/api/dcf", dcfHandler)

	// To serve the script.js file
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Boot server
	fmt.Println(" Ticker analysis engine running on http://localhost:8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func riskAnalysisHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	ticker := strings.ToUpper(strings.TrimSpace(r.FormValue("ticker")))

	if ticker == "" {
		fmt.Fprint(w, `<div class="text-red-400 font-mono mt-2">Error: No ticker symbol provided.</div>`)
		return
	}

	tickerStats, err := data.FetchStockMetrics(ticker)
	if err != nil {
		fmt.Fprintf(w, `<div class="text-red-400 font-mono mt-2">%s</div>`, err)
		return
	}

	// Montecarlo simulation params
	params := quant.SimulationParams{
		CurrentPrice: tickerStats.CurrentPrice,
		DailyDrift:   tickerStats.DailyDrift,
		DailyVol:     tickerStats.DailyVol,
		Days:         252,
		Paths:        100000,
	}

	result := quant.RunMonteCarlo(params)
	chartDataJSON, _ := json.Marshal(result.SamplePaths)

	// We multiplty with sqrt(252) since 252 is the number of trading days in a year, and volatility is the sqrt of variance
	annVol := tickerStats.DailyVol * math.Sqrt(252) * 100 // As percentage
	annDrift := tickerStats.DailyDrift * 252 * 100
	dailyDrag := (0.5 * math.Pow(tickerStats.DailyVol, 2))

	// Beta is calculate by comparing the ticker's returns to the market returns
	beta, err := quant.CalculateBetaFromTicker(ticker, "SPY", "1Week")
	if err != nil {
		fmt.Println(err)
	}

	data := RiskAnalysisData{
		Ticker:        ticker,
		CurrentPrice:  tickerStats.CurrentPrice,
		AnnVol:        annVol,
		AnnDrift:      annDrift,
		DailyDrag:     dailyDrag,
		MedianPrice:   result.MedianPrice,
		ExpectedPrice: result.ExpectedPrice,
		VaR:           result.VaR,
		Beta:          beta,
		ChartData:     template.JS(string(chartDataJSON)),
	}

	// Since DCF is triggered automatically via the htmx triggers that we set in the header, we also need to pass in the required data via the header
	// This is needed because even though the DCF handler should be able to pull the data from the html, htmx triggers the next handler function before it finishes updating the HTML,
	// and thus it ends up using the wrong values if we don't also send the values
	triggerPayload := fmt.Sprintf(`{"run-dcf": {"beta": %.4f, "currentPrice": %.2f}}`, beta, tickerStats.CurrentPrice)
	w.Header().Set("HX-Trigger", triggerPayload)

	tmpl, err := template.ParseFiles("templates/risk_analysis.html")
	if err != nil {
		http.Error(w, "Error parsing template", http.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, "Error executing template", http.StatusInternalServerError)
	}
}

func dcfHandler(w http.ResponseWriter, r *http.Request) {
	ticker := strings.ToUpper(r.URL.Query().Get("ticker"))
	if ticker == "" {
		fmt.Fprintf(w, `<tr id="dcf-rows"><td colspan="6" class="text-red-400 p-4">Error reading ticker</td></tr>`)
		return
	}

	beta, err := strconv.ParseFloat(r.URL.Query().Get("beta"), 64)
	if err != nil {
		fmt.Fprintf(w, `<tr id="dcf-rows"><td colspan="6" class="text-red-400 p-4">Error converting beta value to float %v</td></tr>`, err)
		return
	}
	currentPrice, err := strconv.ParseFloat(r.URL.Query().Get("currentPrice"), 64)
	if err != nil {
		fmt.Fprintf(w, `<tr id="dcf-rows"><td colspan="6" class="text-red-400 p-4">Error converting currentPrice value to float %v</td></tr>`, err)
		return
	}

	// FMP is the primary data source, but currently I am using the free tier which only provides access to the top X number of companies such as AAPL, NVDA, GOOGL
	dataSource := "FMP"
	funds, err := data.GetCompanyFundamentals_FMP(ticker)

	// Failed to get data from FMP, so fallback to yahoo finance through the python microservice
	if err != nil {
		log.Printf("Couldn't load ticker fundamentals data from FMP for %s\n", ticker)
		funds, err = data.GetCompanyFundamentals_YFinance(ticker)
		dataSource = "Yahoo Finance"
	}

	if err != nil {
		fmt.Fprintf(w, `<tr id="dcf-rows"><td colspan="6" class="text-red-400 p-4">Couldn't get Ticker Fundamentals data from both FMP and Yahoo finance. <br> Check whether the secondary data-source python microservice is running%v</td></tr>`, err)
		return
	}

	// Assume the risk free interest rate is equal to the 10 year treasury rate
	// We are currently using the US rate since I would be using this tool mainly to analyze stocks that are heavily tied to the US stock market
	riskFreeRate, err := data.Get10YearRiskFreeRate()
	if err != nil {
		riskFreeRate = 0.04
	}
	terminalWacc := riskFreeRate + EQUITYRISKPREMIUM
	marketCap := currentPrice * funds.SharesOutstanding
	terminalOperatingMargin := math.Max(funds.AvgOperatingMargin, 0.2) // TODO: Calculate the baseline terminal op margin from industry average data rather than hardcoded
	curWacc := quant.CalculateWacc(marketCap, funds.TotalDebt, riskFreeRate, beta, EQUITYRISKPREMIUM, funds.InterestExpense, funds.TaxRate)
	curRevGrowth := funds.HistRevCAGR

	// Couldn't calculate rev growth due to insufficient historic data, so we assume it is the same as the current 10 year risk free rate
	if curRevGrowth == -1 {
		curRevGrowth = riskFreeRate
	}

	// Generate the 10-Year DCF data
	revGrowthRates := quant.Interpolate(curRevGrowth, riskFreeRate, 11)
	WACCs := quant.Interpolate(curWacc, terminalWacc, len(revGrowthRates))
	opMargins := quant.Interpolate(funds.AvgOperatingMargin, terminalOperatingMargin, len(revGrowthRates))
	taxRates := quant.Interpolate(funds.AvgTaxRate, funds.AvgTaxRate, len(revGrowthRates))
	salesToCapitalRatio := quant.Interpolate(funds.SalesToCapital, funds.SalesToCapital, len(revGrowthRates))

	// terminal Reinvestment Rate = Terminal Growth​ / Terminal ROIC
	salesToCapitalRatio[len(revGrowthRates)-1] = revGrowthRates[len(revGrowthRates)-1] / WACCs[len(revGrowthRates)-1]

	data := DCFTemplateData{
		BaseRevenue:       funds.BaseRevenue,
		TotalCash:         funds.TotalCash,
		TotalDebt:         funds.TotalDebt,
		SharesOutstanding: funds.SharesOutstanding,
		DataSource:        dataSource,
		Rows:              make([]RowData, 0),
	}

	for i := 0; i < len(revGrowthRates); i++ {
		yearLabel := fmt.Sprintf("Year %d", i+1)
		if i == len(revGrowthRates)-1 {
			yearLabel = "Terminal"
		}

		data.Rows = append(data.Rows, RowData{
			YearLabel:       yearLabel,
			RevGrowth:       revGrowthRates[i],
			OpMargin:        opMargins[i],
			TaxRate:         taxRates[i],
			SalesToCapRatio: salesToCapitalRatio[i],
			Wacc:            WACCs[i],
		})
	}

	tmpl, err := template.ParseFiles("templates/dcf_rows.html")
	if err != nil {
		http.Error(w, "Error parsing template", http.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, "Error executing template", http.StatusInternalServerError)
	}

}
