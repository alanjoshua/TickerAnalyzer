package data

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"strings"
	"time"
)

// FIX: Officially using the modern Stable API
var FMP_BaseURL string = "https://financialmodelingprep.com/stable/"

type FMPIncomeStatement []struct {
	Revenue                  float64 `json:"revenue"`
	OperatingIncome          float64 `json:"operatingIncome"`
	IncomeBeforeTax          float64 `json:"incomeBeforeTax"`
	IncomeTaxExpense         float64 `json:"incomeTaxExpense"`
	WeightedAverageShsOutDil float64 `json:"weightedAverageShsOutDil"`
	InterestExpense          float64 `json:"interestExpense"`
}

type FMPBalanceSheet []struct {
	CashAndCashEquivalents  float64 `json:"cashAndCashEquivalents"`
	ShortTermInvestments    float64 `json:"shortTermInvestments"`
	TotalDebt               float64 `json:"totalDebt"`
	TotalStockholdersEquity float64 `json:"totalStockholdersEquity"`
}

type FMPTreasury []struct {
	Date   string  `json:"date"`
	Year1  float64 `json:"year1"`
	Year5  float64 `json:"year5"`
	Year10 float64 `json:"year10"`
}

type CompanyProfile []struct {
	Beta    float64 `json:"beta"`
	IpoDate string  `json:"ipoDate"`
}

type TickerFundamentals struct {
	BaseRevenue             float64
	TotalCash               float64
	TotalDebt               float64
	InterestExpense         float64
	SharesOutstanding       float64
	TotalShareHoldersEquity float64

	HistRevCAGR        float64
	AvgOperatingMargin float64
	AvgTaxRate         float64
	SalesToCapital     float64
	TaxRate            float64
}

func Get10YearRiskFreeRate() (float64, error) {
	apikey := strings.TrimSpace(os.Getenv("FMP_APIKEY"))
	if apikey == "" {
		return -1, fmt.Errorf("missing FMP credentials")
	}

	curDate := time.Now()
	startDate := curDate.AddDate(-1, 0, 0)
	start := fmt.Sprintf("%v-%v-%v", startDate.Year(), startDate.Month(), startDate.Day())
	end := fmt.Sprintf("%v-%v-%v", curDate.Year(), curDate.Month(), curDate.Day())

	client := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("%streasury-rates?from=%s&to=%s&apikey=%s", FMP_BaseURL, start, end, apikey)

	resp, err := client.Get(url)
	if err != nil {
		return -1, err
	}
	defer resp.Body.Close()

	var treasuryData FMPTreasury
	if err := json.NewDecoder(resp.Body).Decode(&treasuryData); err != nil {
		return -1, err
	}

	if len(treasuryData) == 0 {
		return 0, fmt.Errorf("No treasury date found")
	}

	return treasuryData[0].Year10 / 100.0, nil

}

func GetDataYearsLimit(ticker string, client *http.Client, apiKey string) int {
	var companyProfile CompanyProfile

	url := fmt.Sprintf("%sprofile?symbol=%s&apikey=%s", FMP_BaseURL, ticker, apiKey)
	resp, err := client.Get(url)
	if err != nil {
		return -1
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&companyProfile); err != nil {
		return -1
	}
	if len(companyProfile) == 0 {
		return -1
	}

	curDate := time.Now()
	layout := "2006-01-02"
	ipoDate, err := time.Parse(layout, companyProfile[0].IpoDate)
	if err != nil {
		return -1
	}

	return curDate.Year() - ipoDate.Year()
}

func GetCompanyFundamentals(ticker string) (TickerFundamentals, error) {
	var funds TickerFundamentals

	apikey := strings.TrimSpace(os.Getenv("FMP_APIKEY"))
	if apikey == "" {
		return funds, fmt.Errorf("missing FMP credentials")
	}

	client := &http.Client{Timeout: 10 * time.Second}

	yearLimit := GetDataYearsLimit(ticker, client, apikey)
	if yearLimit <= 0 {
		yearLimit = 1
	} else if yearLimit > 5 {
		yearLimit = 5
	}

	incUrl := fmt.Sprintf("%sincome-statement?symbol=%s&limit=%d&apikey=%s", FMP_BaseURL, ticker, yearLimit, apikey)
	incResp, err := client.Get(incUrl)
	if err != nil {
		return funds, err
	}
	defer incResp.Body.Close()

	var incData FMPIncomeStatement
	if err := json.NewDecoder(incResp.Body).Decode(&incData); err != nil {
		return funds, err
	}
	if len(incData) < 1 {
		return funds, fmt.Errorf("not enough historical data for %s to calculate CAGR", ticker)
	}

	balUrl := fmt.Sprintf("%sbalance-sheet-statement?symbol=%s&limit=1&apikey=%s", FMP_BaseURL, ticker, apikey)
	balResp, err := client.Get(balUrl)
	if err != nil {
		return funds, err
	}
	defer balResp.Body.Close()

	var balData FMPBalanceSheet
	if err := json.NewDecoder(balResp.Body).Decode(&balData); err != nil {
		return funds, err
	}
	if len(balData) == 0 {
		return funds, fmt.Errorf("no balance sheet data found")
	}

	if (len(incData)) > 1 {
		latestRev := incData[0].Revenue
		oldestRev := incData[len(incData)-1].Revenue
		years := float64(len(incData) - 1)

		funds.HistRevCAGR = math.Pow(latestRev/oldestRev, 1.0/years) - 1.0
	} else {
		funds.HistRevCAGR = -1
	}

	var totalMargin, totalTaxRate float64
	for _, year := range incData {
		totalMargin += year.OperatingIncome / year.Revenue
		if year.IncomeBeforeTax > 0 {
			totalTaxRate += year.IncomeTaxExpense / year.IncomeBeforeTax
		}
	}

	funds.InterestExpense = incData[0].InterestExpense / 1000000.0
	funds.TotalShareHoldersEquity = balData[0].TotalStockholdersEquity / 1000000.0
	funds.AvgOperatingMargin = totalMargin / float64(len(incData))
	funds.AvgTaxRate = totalTaxRate / float64(len(incData))

	funds.BaseRevenue = incData[0].Revenue / 1000000.0
	funds.SharesOutstanding = incData[0].WeightedAverageShsOutDil / 1000000.0

	totalLiquidCash := balData[0].CashAndCashEquivalents + balData[0].ShortTermInvestments
	funds.TotalCash = totalLiquidCash / 1000000.0
	funds.TotalDebt = balData[0].TotalDebt / 1000000.0

	// --- 5. Calculate Sales-to-Capital Ratio ---
	// TODO: Average this over the last 5 years if data is available

	// Invested Capital = Equity + Debt - Cash
	investedCapital := (balData[0].TotalStockholdersEquity / 1000000.0) + funds.TotalDebt - funds.TotalCash

	var salesToCapital float64

	// GUARDRAIL: If Invested Capital is negative or absurdly small (like Apple or Starbucks)
	if investedCapital <= 0 {
		salesToCapital = 3.0 // Hardcode a capital-light tech proxy
	} else {
		salesToCapital = funds.BaseRevenue / investedCapital
	}

	// GUARDRAIL 2: Prevent extreme outliers from breaking the DCF
	// If the ratio is above 5.0, they are "too efficient" for a sustainable 10-year model.
	// If it's below 0.2, they are burning cash too fast.
	if salesToCapital > 5.0 {
		salesToCapital = 5.0
	} else if salesToCapital < 0.2 {
		salesToCapital = 0.2
	}

	funds.SalesToCapital = salesToCapital

	// Calculate effective tax rate
	taxRate := incData[0].IncomeTaxExpense / incData[0].IncomeBeforeTax
	// Sanity clamp
	if taxRate < 0 {
		taxRate = 0
	} else if taxRate > 0.30 { // Cap it at a reasonable global max, like 30%
		taxRate = 0.30
	}
	funds.TaxRate = taxRate

	return funds, nil
}
