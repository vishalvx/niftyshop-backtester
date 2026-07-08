package indicators

import (
	"math"
	"testing"
)

func TestCalculatePivotLevels(t *testing.T) {
	high := 100.0
	low := 80.0
	closeVal := 90.0

	levels := CalculatePivotLevels(high, low, closeVal)

	// Helper for comparing float with epsilon tolerance
	assertAlmostEqual := func(t *testing.T, name string, got, want float64) {
		t.Helper()
		if math.Abs(got-want) > 1e-6 {
			t.Errorf("Expected %s to be %f, got %f", name, want, got)
		}
	}

	// Classic
	assertAlmostEqual(t, "ClassicS1", levels.ClassicS1, 80.0)
	assertAlmostEqual(t, "ClassicS2", levels.ClassicS2, 70.0)
	assertAlmostEqual(t, "ClassicS3", levels.ClassicS3, 60.0)

	// Fibonacci
	assertAlmostEqual(t, "FibS1", levels.FibS1, 82.36)
	assertAlmostEqual(t, "FibS2", levels.FibS2, 77.64)
	assertAlmostEqual(t, "FibS3", levels.FibS3, 70.0)

	// Camarilla
	assertAlmostEqual(t, "CamS1", levels.CamS1, 88.166667)
	assertAlmostEqual(t, "CamS2", levels.CamS2, 86.333333)
	assertAlmostEqual(t, "CamS3", levels.CamS3, 84.5)
	assertAlmostEqual(t, "CamS4", levels.CamS4, 79.0)
}
