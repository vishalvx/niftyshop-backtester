package indicators

import (
	"reflect"
	"testing"
)

func TestCalculateSMA(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		window int
		want   []float64
	}{
		{
			name:   "empty values",
			values: []float64{},
			window: 3,
			want:   []float64{},
		},
		{
			name:   "values less than window",
			values: []float64{10.0, 20.0},
			window: 3,
			want:   []float64{0.0, 0.0},
		},
		{
			name:   "window <= 0",
			values: []float64{10.0, 20.0, 30.0},
			window: 0,
			want:   []float64{0.0, 0.0, 0.0},
		},
		{
			name:   "normal sma calculation",
			values: []float64{10.0, 20.0, 30.0, 40.0, 50.0},
			window: 3,
			// index 0: 0.0 (less than window)
			// index 1: 0.0 (less than window)
			// index 2: (10+20+30)/3 = 20.0
			// index 3: (20+30+40)/3 = 30.0
			// index 4: (30+40+50)/3 = 40.0
			want: []float64{0.0, 0.0, 20.0, 30.0, 40.0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateSMA(tt.values, tt.window)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CalculateSMA() = %v, want %v", got, tt.want)
			}
		})
	}
}
