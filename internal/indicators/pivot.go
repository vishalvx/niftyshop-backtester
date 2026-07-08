package indicators

type PivotLevels struct {
	ClassicS1 float64
	ClassicS2 float64
	ClassicS3 float64

	FibS1 float64
	FibS2 float64
	FibS3 float64

	CamS1 float64
	CamS2 float64
	CamS3 float64
	CamS4 float64
}

func CalculatePivotLevels(high, low, closeVal float64) PivotLevels {
	p := (high + low + closeVal) / 3.0
	rangeVal := high - low

	return PivotLevels{
		ClassicS1: 2.0*p - high,
		ClassicS2: p - rangeVal,
		ClassicS3: low - 2.0*(high-p),

		FibS1: p - 0.382*rangeVal,
		FibS2: p - 0.618*rangeVal,
		FibS3: p - 1.000*rangeVal,

		CamS1: closeVal - rangeVal*(1.1/12.0),
		CamS2: closeVal - rangeVal*(1.1/6.0),
		CamS3: closeVal - rangeVal*(1.1/4.0),
		CamS4: closeVal - rangeVal*(1.1/2.0),
	}
}
