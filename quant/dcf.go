package quant

func Interpolate(startVal, terminalVal float64, totalYears int) []float64 {
	result := make([]float64, totalYears)
	if startVal <= terminalVal {
		for i := 0; i < totalYears; i++ {
			result[i] = startVal
		}
		return result
	}

	stepSize := (terminalVal - startVal) / float64(totalYears-1)
	for i := 0; i < totalYears; i++ {
		result[i] = startVal + (stepSize * float64(i))
	}
	return result
}
