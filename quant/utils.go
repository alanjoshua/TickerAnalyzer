package quant

// wacc = E/V (costOfEquity) + D/V(costOfDebt*(1-T))
// Since interest payments are tax deductible, we take that into account
func CalculateWacc(marketValOfEquity, marketValOfDebt, riskFreeRate, beta, equityRiskPremium, interestExpense, tax float64) float64 {
	V := marketValOfEquity + marketValOfDebt
	costOfEquity := riskFreeRate + (beta * equityRiskPremium)
	costOfDebt := interestExpense / marketValOfDebt

	return ((marketValOfEquity / V) * costOfEquity) + ((marketValOfDebt / V) * (1 - tax) * costOfDebt)
}
