package quant

import (
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"
)

// inputs for the model.
type SimulationParams struct {
	CurrentPrice float64
	DailyDrift   float64 // Expected daily return
	DailyVol     float64 // Expected daily volatility
	Days         int     // Time horizon (e.g., 252 days for a trading year)
	Paths        int     // Number of simulations (e.g., 100,000)
}

type SimulationResult struct {
	VaR           float64
	ExpectedPrice float64 // The Mean
	MedianPrice   float64 // The 50th Percentile
	SamplePaths   [][]float64
}

// RunMonteCarlo executes a parallelized simulation and returns the 99% Value at Risk (VaR).
func RunMonteCarlo(params SimulationParams) SimulationResult {

	finalPrices := make([]float64, params.Paths)
	samplesToKeep := 50
	samplePaths := make([][]float64, samplesToKeep)

	// Split paths into 10 work groups for concurrency
	numWorkers := 10
	pathsPerWorker := params.Paths / numWorkers

	var wg sync.WaitGroup
	wg.Add(numWorkers)

	// Pre-calculate the static drift component of the Black-Scholes/GBM formula
	drift := params.DailyDrift - (0.5 * math.Pow(params.DailyVol, 2))

	for w := 0; w < numWorkers; w++ {

		// Launch a lightweight virtual thread
		go func(workerID int) {
			defer wg.Done()

			// We give every worker its own private, locally-seeded Random Number Generator to avoid threads waiting for the global mutex lock
			rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(workerID)))

			startIdx := workerID * pathsPerWorker
			endIdx := startIdx + pathsPerWorker
			for i := startIdx; i < endIdx; i++ {
				price := params.CurrentPrice
				var path []float64
				if workerID == 0 && i < samplesToKeep {
					path = make([]float64, params.Days)
				}

				// Simulate the daily price movements
				for d := 0; d < params.Days; d++ {
					shock := params.DailyVol * rng.NormFloat64()
					price *= math.Exp(drift + shock)
					if len(path) > 0 {
						path[d] = price
					}
				}
				finalPrices[i] = price
				if len(path) > 0 {
					samplePaths[i] = path
				}
			}
		}(w)
	}

	wg.Wait()

	// CALCULATE VALUE AT RISK (VaR)
	sort.Float64s(finalPrices)
	// The Expected Value (The Mean)
	// We loop through all 100,000 prices and average them out
	var sum float64
	for _, p := range finalPrices {
		sum += p
	}
	expectedPrice := sum / float64(params.Paths)

	// Find the price at the 1st percentile (The 99% worst-case scenario)
	percentileIndex := int(float64(params.Paths) * 0.01)
	worstCasePrice := finalPrices[percentileIndex]

	percentile50Index := int(float64(params.Paths) * 0.50)
	medianPrice := finalPrices[percentile50Index]

	// Return the maximum expected loss
	return SimulationResult{
		VaR:           params.CurrentPrice - worstCasePrice,
		SamplePaths:   samplePaths,
		ExpectedPrice: expectedPrice,
		MedianPrice:   medianPrice,
	}
}
