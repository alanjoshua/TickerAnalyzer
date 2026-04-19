package data

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

var FMP_BaseURL string = "https://financialmodelingprep.com/stable/"

type FMPIncomeStatement struct {
	Revenue                  float64 `json:"revenue"`
	OperatingIncome          float64 `json:"operatingIncome"`
	IncomeBeforeTax          float64 `json:"incomeBeforeTax"`
	IncomeTaxExpense         float64 `json:"incomeTaxExpense"`
	WeightedAverageShsOutDil float64 `json:"weightedAverageShsOutDil"`
	InterestExpense          float64 `json:"interestExpense"`
}

type FMPBalanceSheet struct {
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

func (i FMPIncomeStatement) Standardize() IncomeStatement {
	return IncomeStatement{
		Revenue:           i.Revenue,
		OperatingIncome:   i.OperatingIncome,
		IncomeBeforeTax:   i.IncomeBeforeTax,
		IncomeTaxExpense:  i.IncomeTaxExpense,
		SharesOutstanding: i.WeightedAverageShsOutDil,
		InterestExpense:   i.InterestExpense,
	}
}

func (b FMPBalanceSheet) Standardize() BalanceSheet {
	return BalanceSheet{
		CashAndCashEquivalents:  b.CashAndCashEquivalents,
		ShortTermInvestments:    b.ShortTermInvestments,
		TotalDebt:               b.TotalDebt,
		TotalStockholdersEquity: b.TotalStockholdersEquity,
	}
}

// Returns the 10 year US Treasury yield
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

// Returns Fundamental Data calculated from the ticker's Income Statement and Balance sheet with FMP as the data source
func GetCompanyFundamentals_FMP(ticker string) (TickerFundamentals, error) {
	var funds TickerFundamentals

	apikey := strings.TrimSpace(os.Getenv("FMP_APIKEY"))
	if apikey == "" {
		return funds, fmt.Errorf("missing FMP credentials")
	}

	client := &http.Client{Timeout: 10 * time.Second}

	incUrl := fmt.Sprintf("%sincome-statement?symbol=%s&limit=5&apikey=%s", FMP_BaseURL, ticker, apikey)
	incResp, err := client.Get(incUrl)
	if err != nil {
		return funds, err
	}
	defer incResp.Body.Close()

	var incData []FMPIncomeStatement
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

	var balData []FMPBalanceSheet
	if err := json.NewDecoder(balResp.Body).Decode(&balData); err != nil {
		return funds, err
	}
	if len(balData) == 0 {
		return funds, fmt.Errorf("no balance sheet data found")
	}

	incomeStatements := make([]IncomeStatement, len(incData))
	balanceStatements := make([]BalanceSheet, len(balData))

	// income and balance sheet arrays don't necessarily have to be the same length, though they should be in most cases
	for i, sheet := range incData {
		incomeStatements[i] = sheet.Standardize()
	}
	for i, sheet := range balData {
		balanceStatements[i] = sheet.Standardize()
	}

	funds = CalculateTickerFundamentals(incomeStatements, balanceStatements)
	return funds, nil
}
