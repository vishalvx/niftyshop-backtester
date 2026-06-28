package indicators

// CalculateSMA computes the Simple Moving Average for a slice of float64 values with a given window size.
// It returns a slice of the same length, where the first window-1 elements are 0.0.
func CalculateSMA(values []float64, window int) []float64 {
	sma := make([]float64, len(values))
	if len(values) < window || window <= 0 {
		return sma
	}

	sum := 0.0
	for i := 0; i < window; i++ {
		sum += values[i]
	}
	sma[window-1] = sum / float64(window)

	for i := window; i < len(values); i++ {
		sum += values[i] - values[i-window]
		sma[i] = sum / float64(window)
	}

	return sma
}
