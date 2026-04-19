package data

import "math"

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

type IncomeStatement struct {
	Revenue           float64
	OperatingIncome   float64
	IncomeBeforeTax   float64
	IncomeTaxExpense  float64
	SharesOutstanding float64
	InterestExpense   float64
}

type BalanceSheet struct {
	CashAndCashEquivalents  float64
	ShortTermInvestments    float64
	TotalDebt               float64
	TotalStockholdersEquity float64
}

// Calculates Ticker Fundamentals from income statement and balance sheet data
func CalculateTickerFundamentals(incomeStatements []IncomeStatement, balanceSheets []BalanceSheet) TickerFundamentals {
	var funds TickerFundamentals

	if (len(incomeStatements)) > 1 {
		latestRev := incomeStatements[0].Revenue
		oldestRev := incomeStatements[len(incomeStatements)-1].Revenue
		years := float64(len(incomeStatements) - 1)

		funds.HistRevCAGR = math.Pow(latestRev/oldestRev, 1.0/years) - 1.0
	} else {
		funds.HistRevCAGR = -1
	}

	var totalMargin, totalTaxRate float64
	for _, year := range incomeStatements {
		totalMargin += year.OperatingIncome / year.Revenue
		if year.IncomeBeforeTax > 0 {
			totalTaxRate += year.IncomeTaxExpense / year.IncomeBeforeTax
		}
	}

	funds.InterestExpense = incomeStatements[0].InterestExpense / 1000000.0
	funds.TotalShareHoldersEquity = balanceSheets[0].TotalStockholdersEquity / 1000000.0
	funds.AvgOperatingMargin = totalMargin / float64(len(incomeStatements))
	funds.AvgTaxRate = totalTaxRate / float64(len(incomeStatements))

	funds.BaseRevenue = incomeStatements[0].Revenue / 1000000.0
	funds.SharesOutstanding = incomeStatements[0].SharesOutstanding / 1000000.0

	totalLiquidCash := balanceSheets[0].CashAndCashEquivalents + balanceSheets[0].ShortTermInvestments
	funds.TotalCash = totalLiquidCash / 1000000.0
	funds.TotalDebt = balanceSheets[0].TotalDebt / 1000000.0

	// TODO: Average this over the last 5 years if data is available
	// Invested Capital = Equity + Debt - Cash
	investedCapital := (balanceSheets[0].TotalStockholdersEquity / 1000000.0) + funds.TotalDebt - funds.TotalCash

	var salesToCapital float64

	// Set a default value such as 3 if the invest capital is very low or negative
	if investedCapital <= 0 {
		salesToCapital = 3.0
	} else {
		salesToCapital = funds.BaseRevenue / investedCapital
	}

	// If the ratio is above 5.0, they are "too efficient" for a sustainable 10-year model.
	// If it's below 0.2, they are burning cash too fast.
	if salesToCapital > 5.0 {
		salesToCapital = 5.0
	} else if salesToCapital < 0.2 {
		salesToCapital = 0.2
	}

	funds.SalesToCapital = salesToCapital

	taxRate := incomeStatements[0].IncomeTaxExpense / incomeStatements[0].IncomeBeforeTax

	// clamp
	if taxRate < 0 {
		taxRate = 0
	} else if taxRate > 0.30 { // 30% max cap
		taxRate = 0.30
	}
	funds.TaxRate = taxRate

	return funds
}
