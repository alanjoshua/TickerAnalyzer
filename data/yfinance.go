package data

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type IncomeStatementYFinance struct {
	Date              string  `json:"date"`
	Revenue           float64 `json:"Total Revenue"`
	InterestExpense   float64 `json:"Interest Expense"`
	OperatingIncome   float64 `json:"Operating Income"`
	IncomeBeforeTax   float64 `json:"Pretax Income"`
	IncomeTaxExpense  float64 `json:"Tax Provision"`
	SharesOutstanding float64 `json:"Shares Outstanding"`
}
type BalanceSheetYFinance struct {
	Date                    string  `json:"date"`
	CashAndCashEquivalents  float64 `json:"Cash And Cash Equivalents"`
	ShortTermInvestments    float64 `json:"Other Short Term Investments"`
	TotalDebt               float64 `json:"Total Debt"`
	TotalStockholdersEquity float64 `json:"Stockholders Equity"`
	CapitalLeaseObligations float64 `json:"Capital Lease Obligations"`
}
type CashflowYFinance struct {
	Date string `json:"date"`
}

type YFinanceResponse struct {
	Symbol           string                    `json:"symbol"`
	CurrentPrice     float64                   `json:"currentPrice"`
	Beta             float64                   `json:"beta"`
	IncomeStatements []IncomeStatementYFinance `json:"incomeStatement"`
	BalanceSheets    []BalanceSheetYFinance    `json:"balanceSheet"`
	Cashflows        []CashflowYFinance        `json:"cashflow"`
}

func (i IncomeStatementYFinance) Standardize() IncomeStatement {
	return IncomeStatement{
		Revenue:           i.Revenue,
		OperatingIncome:   i.OperatingIncome,
		IncomeBeforeTax:   i.IncomeBeforeTax,
		IncomeTaxExpense:  i.IncomeTaxExpense,
		SharesOutstanding: i.SharesOutstanding,
		InterestExpense:   i.InterestExpense,
	}
}

func (b BalanceSheetYFinance) Standardize() BalanceSheet {
	return BalanceSheet{
		CashAndCashEquivalents:  b.CashAndCashEquivalents,
		ShortTermInvestments:    b.ShortTermInvestments,
		TotalDebt:               b.TotalDebt + b.CapitalLeaseObligations,
		TotalStockholdersEquity: b.TotalStockholdersEquity,
	}
}

// Returns Fundamental Data calculated from the ticker's Income Statement and Balance sheet with Yahoo Finance as the data source, which is being run in a python microservice
func GetCompanyFundamentals_YFinance(ticker string) (TickerFundamentals, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("http://localhost:8000/api/fundamentals/%s", ticker)

	resp, err := client.Get(url)
	if err != nil {
		return TickerFundamentals{}, err
	}

	defer resp.Body.Close()

	var respData YFinanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return TickerFundamentals{}, err
	}

	incomeStatements := make([]IncomeStatement, len(respData.IncomeStatements))
	balanceStatements := make([]BalanceSheet, len(respData.BalanceSheets))

	// income and balance sheet arrays don't necessarily have to be the same length, though they should be in most cases
	for i, sheet := range respData.IncomeStatements {
		incomeStatements[i] = sheet.Standardize()
	}
	for i, sheet := range respData.BalanceSheets {
		balanceStatements[i] = sheet.Standardize()
	}

	funds := CalculateTickerFundamentals(incomeStatements, balanceStatements)
	return funds, nil
}
