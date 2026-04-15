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

	params := quant.SimulationParams{
		CurrentPrice: tickerStats.CurrentPrice,
		DailyDrift:   tickerStats.DailyDrift, // Annual return divided by trading days
		DailyVol:     tickerStats.DailyVol,   // Annual vol scaled to daily
		Days:         252,                    // Simulate 1 year into the future
		Paths:        100000,                 // 100,000 monte carlo paths
	}

	result := quant.RunMonteCarlo(params)
	chartDataJSON, _ := json.Marshal(result.SamplePaths)

	annVol := tickerStats.DailyVol * math.Sqrt(252) * 100
	annDrift := tickerStats.DailyDrift * 252 * 100
	dailyDrag := (0.5 * math.Pow(tickerStats.DailyVol, 2)) * 100 // Volatility Drag percentage

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

	// Fetch Live Data
	funds, err := data.GetCompanyFundamentals(ticker)
	riskFreeRate, err := data.Get10YearRiskFreeRate()
	if err != nil {
		riskFreeRate = 0.04
	}
	terminalWacc := riskFreeRate + EQUITYRISKPREMIUM

	if err != nil {
		fmt.Fprintf(w, `<tr id="dcf-rows"><td colspan="6" class="text-red-400 p-4">Error fetching %s: %v</td></tr>`, ticker, err)
		return
	}

	fmt.Println("Fundamentals for ", ticker, ": ", funds)
	fmt.Println("Risk free rate: ", riskFreeRate)

	marketCap := currentPrice * funds.SharesOutstanding
	curWacc := quant.CalculateWacc(marketCap, funds.TotalDebt, riskFreeRate, beta, EQUITYRISKPREMIUM, funds.InterestExpense, funds.TaxRate)
	curRevGrowth := funds.HistRevCAGR
	// Couldn't calculate rev growth due to insufficient historic data
	if curRevGrowth == -1 {
		curRevGrowth = riskFreeRate
	}

	// Generate the 10-Year data
	revGrowthRates := quant.Interpolate(curRevGrowth, riskFreeRate, 11)
	WACCs := quant.Interpolate(curWacc, terminalWacc, len(revGrowthRates)) // TODO: Calculate current wacc
	opMargins := quant.Interpolate(funds.AvgOperatingMargin, funds.AvgOperatingMargin, len(revGrowthRates))
	taxRates := quant.Interpolate(funds.AvgTaxRate, funds.AvgTaxRate, len(revGrowthRates))
	salesToCapitalRatio := quant.Interpolate(funds.SalesToCapital, funds.SalesToCapital, len(revGrowthRates))

	// terminal Reinvestment Rate = Terminal Growth​ / Terminal ROIC
	// where ROIC can be assumed to equal to terminal wacc
	salesToCapitalRatio[len(revGrowthRates)-1] = revGrowthRates[len(revGrowthRates)-1] / WACCs[len(revGrowthRates)-1]

	data := DCFTemplateData{
		BaseRevenue:       funds.BaseRevenue,
		TotalCash:         funds.TotalCash,
		TotalDebt:         funds.TotalDebt,
		SharesOutstanding: funds.SharesOutstanding,
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
