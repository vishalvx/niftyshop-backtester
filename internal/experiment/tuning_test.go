package experiment

import (
	"fmt"
	"math"
	"testing"

	"github.com/vishalvx/back-tester/internal/sim"
)

// Reference numbers (normal quantiles, ExpectedMaxSharpe, PSR, DSR) were produced with scipy.stats.norm in a separate
// script and pasted in as constants. Hand-checkable cases carry their arithmetic in a comment.

func near(t *testing.T, name string, got, want, tol float64) {
	t.Helper()
	if math.IsNaN(got) || math.Abs(got-want) > tol {
		t.Errorf("%s = %.12g, want %.12g (tol %g)", name, got, want, tol)
	}
}

// testRNG is a tiny splitmix64 generator so noise fixtures never depend on the standard library's random stream.
type testRNG uint64

func (r *testRNG) next() uint64 {
	*r += 0x9E3779B97F4A7C15
	z := uint64(*r)
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

// unit returns a float in the open interval (0, 1).
func (r *testRNG) unit() float64 { return (float64(r.next()>>11) + 0.5) / (1 << 53) }

// norm returns a standard normal draw (Box-Muller).
func (r *testRNG) norm() float64 {
	return math.Sqrt(-2*math.Log(r.unit())) * math.Cos(2*math.Pi*r.unit())
}

func TestNormInv_KnownQuantiles(t *testing.T) {
	tests := []struct {
		p, want float64
	}{
		{0.975, 1.959963984540054},
		{0.5, 0},
		{0.9, 1.2815515655446004},
		{0.99, 2.3263478740408408},
		{0.999, 3.090232306167813},
		{0.3, -0.5244005127080409},
		{0.7, 0.5244005127080407},
		{0.025, -1.9599639845400545},
		{0.01, -2.3263478740408408},
		{0.001, -3.090232306167813},
		{1e-6, -4.753424308822899},
		{1 - 1e-6, 4.753424308817087},
		{0.8413447460685429, 1}, // P(Z <= 1)
		// Either side of the approximation's tail/central switch at 0.02425.
		{0.02424, -1.9731366119445441},
		{0.02425, -1.972961051311885},
		{0.02426, -1.97278555146786},
		{0.97575, 1.972961051311885},
	}
	for _, tc := range tests {
		near(t, "normInv", normInv(tc.p), tc.want, 1e-8)
	}
}

func TestNormInv_EdgesAndSymmetry(t *testing.T) {
	if got := normInv(0); !math.IsInf(got, -1) {
		t.Errorf("normInv(0) = %v, want -Inf", got)
	}
	if got := normInv(-0.5); !math.IsInf(got, -1) {
		t.Errorf("normInv(-0.5) = %v, want -Inf", got)
	}
	if got := normInv(1); !math.IsInf(got, 1) {
		t.Errorf("normInv(1) = %v, want +Inf", got)
	}
	if got := normInv(1.5); !math.IsInf(got, 1) {
		t.Errorf("normInv(1.5) = %v, want +Inf", got)
	}
	for _, p := range []float64{1e-7, 0.001, 0.01, 0.02, 0.03, 0.1, 0.25, 0.4, 0.49} {
		near(t, "antisymmetry", normInv(p), -normInv(1-p), 1e-9)
	}
	// Strictly increasing across the three internal ranges.
	prev := math.Inf(-1)
	for p := 0.001; p < 1; p += 0.001 {
		if x := normInv(p); x <= prev {
			t.Fatalf("normInv not increasing at p=%v: %v after %v", p, x, prev)
		} else {
			prev = x
		}
	}
}

func TestNormCDF_KnownValues(t *testing.T) {
	tests := []struct {
		x, want float64
	}{
		{0, 0.5},
		{1, 0.8413447460685429},
		{-1, 0.15865525393145707},
		{1.959963984540054, 0.975},
		{3, 0.9986501019683699},
		{-3, 0.001349898031630093},
		{5, 0.9999997133484281},
		{-5, 2.8665157187919344e-07},
		{0.5, 0.6914624612740131},
	}
	for _, tc := range tests {
		near(t, "normCDF", normCDF(tc.x), tc.want, 1e-12)
	}
}

func TestNormCDFAndInvRoundTrip(t *testing.T) {
	for _, p := range []float64{1e-6, 0.001, 0.01, 0.02425, 0.025, 0.1, 0.3, 0.5, 0.7, 0.9, 0.975, 0.99, 0.999, 1 - 1e-6} {
		near(t, "cdf(inv(p))", normCDF(normInv(p)), p, 1e-9)
	}
	for x := -4.5; x <= 4.5; x += 0.25 {
		near(t, "inv(cdf(x))", normInv(normCDF(x)), x, 1e-7)
	}
}

func TestExpectedMaxSharpe(t *testing.T) {
	// Unit-variance values from scipy (0.4228*Z(1-1/n) + 0.5772*Z(1-1/(n e))); e.g. n=2: 0.5772 * Z(0.8161) = 0.5198.
	tests := []struct {
		n    int
		want float64
	}{
		{2, 0.5197553442805939},
		{3, 0.852804496150695},
		{5, 1.1925940010147893},
		{10, 1.57459830134575},
		{50, 2.2763030934203483},
		{100, 2.5306028932016846},
		{1000, 3.255121513652723},
		{10000, 3.86066485551044},
	}
	for _, tc := range tests {
		near(t, "ExpectedMaxSharpe unit variance", ExpectedMaxSharpe(tc.n, 1), tc.want, 1e-7)
	}

	t.Run("scales with the square root of the variance", func(t *testing.T) {
		near(t, "n=100 var 0.0025", ExpectedMaxSharpe(100, 0.0025), 0.12653014466008425, 1e-8)
		near(t, "n=10 var 0.0004", ExpectedMaxSharpe(10, 0.0004), 0.031491966026915, 1e-8)
		near(t, "4x variance doubles it", ExpectedMaxSharpe(30, 4), 2*ExpectedMaxSharpe(30, 1), 1e-12)
	})
	t.Run("zero for fewer than two trials or no dispersion", func(t *testing.T) {
		for _, n := range []int{-3, 0, 1} {
			if got := ExpectedMaxSharpe(n, 1); got != 0 {
				t.Errorf("ExpectedMaxSharpe(%d, 1) = %v, want 0", n, got)
			}
		}
		if got := ExpectedMaxSharpe(100, 0); got != 0 {
			t.Errorf("ExpectedMaxSharpe(100, 0) = %v, want 0", got)
		}
	})
	t.Run("grows with the number of trials", func(t *testing.T) {
		prev := 0.0
		for _, n := range []int{2, 3, 4, 5, 8, 10, 20, 39, 40, 100, 500, 1000, 10000, 100000} {
			got := ExpectedMaxSharpe(n, 0.01)
			if got <= prev {
				t.Errorf("ExpectedMaxSharpe(%d) = %v is not above the value for fewer trials (%v)", n, got, prev)
			}
			prev = got
		}
	})
}

func TestProbabilisticSharpe(t *testing.T) {
	tests := []struct {
		name       string
		sr, bench  float64
		t          int
		skew, kurt float64
		want       float64
	}{
		// Normal returns: den = 1 + (3-1)/4 * 0.1^2 = 1.005; z = 0.1*sqrt(251)/sqrt(1.005) = 1.5804.
		{"normal returns, benchmark 0", 0.1, 0, 252, 0, 3, 0.9429868610243624},
		{"normal returns, benchmark 0.05", 0.1, 0.05, 252, 0, 3, 0.7852875046472259},
		{"left skew and fat tails lower it", 0.1, 0.05, 252, -0.5, 5, 0.7791729759124011},
		{"short history", 0.05, 0, 100, 0, 3, 0.6904700226375926},
		{"longer history", 0.05, 0, 400, 0, 3, 0.8408907672227921},
		{"skewed fat-tailed track record", 0.15, 0, 252, -0.3, 4.5, 0.9893640247569284},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			near(t, "PSR", ProbabilisticSharpe(tc.sr, tc.bench, tc.t, tc.skew, tc.kurt), tc.want, 1e-12)
		})
	}

	t.Run("exactly one half when the estimate equals the benchmark", func(t *testing.T) {
		for _, sr := range []float64{-0.2, 0, 0.05, 0.3} {
			for _, tt := range []int{2, 30, 252, 5000} {
				if got := ProbabilisticSharpe(sr, sr, tt, -0.4, 6); got != 0.5 {
					t.Errorf("PSR(sr=bench=%v, T=%d) = %v, want 0.5", sr, tt, got)
				}
			}
		}
	})
	t.Run("below one half when the estimate is under the benchmark", func(t *testing.T) {
		if got := ProbabilisticSharpe(0.05, 0.1, 252, 0, 3); got >= 0.5 {
			t.Errorf("PSR = %v, want below 0.5", got)
		}
	})
	t.Run("rises with the estimate and with the sample length", func(t *testing.T) {
		prev := 0.0
		for _, sr := range []float64{0.01, 0.03, 0.06, 0.1, 0.2} {
			got := ProbabilisticSharpe(sr, 0, 252, 0, 3)
			if got <= prev {
				t.Errorf("PSR(sr=%v) = %v not above %v", sr, got, prev)
			}
			prev = got
		}
		prev = 0.0
		for _, tt := range []int{10, 50, 100, 252, 1000} {
			got := ProbabilisticSharpe(0.05, 0, tt, 0, 3)
			if got <= prev {
				t.Errorf("PSR(T=%d) = %v not above %v", tt, got, prev)
			}
			prev = got
		}
	})
	t.Run("undefined cases return NaN", func(t *testing.T) {
		// den = 1 - 3*1 + (3-1)/4 = -1.5 <= 0.
		if got := ProbabilisticSharpe(1, 0, 252, 3, 3); !math.IsNaN(got) {
			t.Errorf("non-positive variance term: got %v, want NaN", got)
		}
		if got := ProbabilisticSharpe(0.1, 0, 1, 0, 3); !math.IsNaN(got) {
			t.Errorf("T=1: got %v, want NaN", got)
		}
		if got := ProbabilisticSharpe(0.1, 0, 0, 0, 3); !math.IsNaN(got) {
			t.Errorf("T=0: got %v, want NaN", got)
		}
	})
}

func TestDeflatedSharpe(t *testing.T) {
	// sr 0.15 per period, cross-trial SR variance 0.0025, T=252, skew -0.3, kurtosis 4.5. Values from scipy.
	// n=1 charges nothing (benchmark 0); each extra trial raises the bar SR0 and lowers the probability.
	const sr, srVar, T, skew, kurt = 0.15, 0.0025, 252, -0.3, 4.5
	want := []struct {
		n int
		v float64
	}{
		{1, 0.9893640247569284},
		{2, 0.9715515781146017},
		{10, 0.8630864425585169},
		{100, 0.6407109136954063},
		{1000, 0.4223603813783471},
	}
	prev := 2.0
	for _, w := range want {
		got := DeflatedSharpe(sr, srVar, w.n, T, skew, kurt)
		near(t, fmt.Sprintf("DSR n=%d", w.n), got, w.v, 1e-8)
		if got >= prev {
			t.Errorf("DSR(n=%d) = %v did not fall below the value for fewer trials (%v)", w.n, got, prev)
		}
		prev = got
	}

	t.Run("no dispersion across trials means no deflation", func(t *testing.T) {
		near(t, "srVar 0", DeflatedSharpe(sr, 0, 500, T, skew, kurt), ProbabilisticSharpe(sr, 0, T, skew, kurt), 1e-12)
	})
	t.Run("a skill-less strategy at the expected maximum scores one half", func(t *testing.T) {
		sr0 := ExpectedMaxSharpe(50, srVar)
		near(t, "DSR at SR0", DeflatedSharpe(sr0, srVar, 50, T, 0, 3), 0.5, 1e-12)
	})
}

func TestSpearman(t *testing.T) {
	tests := []struct {
		name string
		x, y []float64
		want float64
	}{
		{"increasing", []float64{1, 2, 3, 4, 5}, []float64{10, 20, 30, 40, 50}, 1},
		{"monotone but non-linear", []float64{1, 2, 3, 4, 5}, []float64{1, 4, 9, 16, 25}, 1},
		{"unsorted but same order", []float64{3, 1, 2}, []float64{30, 10, 20}, 1},
		{"decreasing", []float64{1, 2, 3, 4, 5}, []float64{5, 4, 3, 2, 1}, -1},
		{"monotone decreasing non-linear", []float64{1, 2, 3, 4, 5}, []float64{-1, -8, -27, -64, -125}, -1},
		// d = rank differences (-1, 1, -1, 1, 0): rho = 1 - 6*sum(d^2)/(n(n^2-1)) = 1 - 6*4/120 = 0.8.
		{"hand-computed 0.8", []float64{1, 2, 3, 4, 5}, []float64{2, 1, 4, 3, 5}, 0.8},
		// x ranks 1, 2.5, 2.5, 4 and y ranks 1, 2, 3, 4: cov sum 4.5, var sums 4.5 and 5 -> 4.5/sqrt(22.5).
		{"ties in one series", []float64{1, 2, 2, 3}, []float64{1, 2, 3, 4}, 0.9486832980505138},
		{"identical tied series", []float64{1, 1, 2, 2}, []float64{5, 5, 9, 9}, 1},
		{"constant series has no rank correlation", []float64{4, 4, 4, 4}, []float64{1, 2, 3, 4}, 0},
		{"both constant", []float64{4, 4, 4}, []float64{7, 7, 7}, 0},
		{"single point", []float64{1}, []float64{2}, 0},
		{"empty", nil, nil, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			near(t, "Spearman", Spearman(tc.x, tc.y), tc.want, 1e-12)
			// Symmetric in its arguments.
			near(t, "Spearman swapped", Spearman(tc.y, tc.x), tc.want, 1e-12)
		})
	}
}

func TestRanks(t *testing.T) {
	tests := []struct {
		in, want []float64
	}{
		{[]float64{3, 1, 2}, []float64{3, 1, 2}},
		{[]float64{10, 30, 20, 20}, []float64{1, 4, 2.5, 2.5}},
		{[]float64{5, 5, 5}, []float64{2, 2, 2}},
		{[]float64{1, 2, 2, 2, 9}, []float64{1, 3, 3, 3, 5}},
		{nil, []float64{}},
	}
	for _, tc := range tests {
		got := ranks(tc.in)
		if len(got) != len(tc.want) {
			t.Fatalf("ranks(%v) = %v, want %v", tc.in, got, tc.want)
		}
		for i := range got {
			near(t, "rank", got[i], tc.want[i], 1e-12)
		}
	}
}

func TestSkewKurt(t *testing.T) {
	t.Run("symmetric sample has zero skew", func(t *testing.T) {
		skew, _ := SkewKurt([]float64{-2, -1, 0, 1, 2})
		near(t, "skew", skew, 0, 1e-12)
	})
	t.Run("two-point symmetric sample has kurtosis near 1 and zero skew", func(t *testing.T) {
		// Values +-1 in equal numbers: the population kurtosis is exactly 1. The code divides by the n-1 standard
		// deviation, which shifts this by a factor (1-1/n)^2, so allow 0.5 percent at n = 1000.
		x := make([]float64, 1000)
		for i := range x {
			x[i] = 1 - 2*float64(i%2)
		}
		skew, kurt := SkewKurt(x)
		near(t, "skew", skew, 0, 1e-12)
		near(t, "kurt", kurt, 1, 0.005)
	})
	t.Run("sign follows the long tail and mirroring flips it", func(t *testing.T) {
		x := []float64{0, 0, 0, 0, 1, 1, 2, 10}
		neg := make([]float64, len(x))
		for i, v := range x {
			neg[i] = -v
		}
		s, k := SkewKurt(x)
		sn, kn := SkewKurt(neg)
		if s <= 0.5 {
			t.Errorf("right-tailed sample skew = %v, want clearly positive", s)
		}
		near(t, "mirrored skew", sn, -s, 1e-12)
		near(t, "mirrored kurt", kn, k, 1e-12)
		if k <= 3 {
			t.Errorf("single extreme value should give kurtosis above 3, got %v", k)
		}
	})
	t.Run("constant sample returns the normal-distribution defaults", func(t *testing.T) {
		skew, kurt := SkewKurt([]float64{0.01, 0.01, 0.01})
		if skew != 0 || kurt != 3 {
			t.Errorf("SkewKurt(constant) = %v, %v, want 0, 3", skew, kurt)
		}
		skew, kurt = SkewKurt(nil)
		if skew != 0 || kurt != 3 {
			t.Errorf("SkewKurt(nil) = %v, %v, want 0, 3", skew, kurt)
		}
	})
}

func TestSharpePeriodAndMeanStd(t *testing.T) {
	// [0.01, 0.03]: mean 0.02, sample sd sqrt((0.01^2 + 0.01^2)/1) = 0.01414213562373095.
	r := []float64{0.01, 0.03}
	near(t, "SharpePeriod rf 0", SharpePeriod(r, 0), 1.4142135623730951, 1e-12)
	near(t, "SharpePeriod rf 0.01", SharpePeriod(r, 0.01), 0.7071067811865476, 1e-12)
	near(t, "SharpePeriod negative", SharpePeriod([]float64{-0.01, -0.03}, 0), -1.4142135623730951, 1e-12)
	if got := SharpePeriod([]float64{0.02, 0.02, 0.02}, 0); got != 0 {
		t.Errorf("SharpePeriod(constant) = %v, want 0", got)
	}
	if got := SharpePeriod(nil, 0); got != 0 {
		t.Errorf("SharpePeriod(nil) = %v, want 0", got)
	}

	m, s := meanStd([]float64{2, 4, 4, 4, 5, 5, 7, 9}) // sum of squared deviations 32 -> sqrt(32/7)
	near(t, "mean", m, 5, 1e-12)
	near(t, "sd", s, 2.138089935299395, 1e-12)
	if m, s := meanStd([]float64{7}); m != 7 || s != 0 {
		t.Errorf("meanStd(single) = %v, %v, want 7, 0", m, s)
	}
	if m, s := meanStd(nil); m != 0 || s != 0 {
		t.Errorf("meanStd(nil) = %v, %v, want 0, 0", m, s)
	}
}

func TestDailyReturns(t *testing.T) {
	// Points 110, 121, 133.1 from 100: +10%, +10%. The last return is measured to the outcome's Final value
	// (equity after tax), so with Final = 120 the last step is 120/121 - 1.
	o := &Outcome{
		Result: &sim.Result{
			StartCap: 100,
			Points:   []sim.Point{{Equity: 110}, {Equity: 121}, {Equity: 133.1}},
		},
		Final: 120,
	}
	got := DailyReturns(o)
	want := []float64{0.1, 0.1, 120.0/121 - 1}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		near(t, "return", got[i], want[i], 1e-12)
	}
	if got := DailyReturns(&Outcome{Result: &sim.Result{StartCap: 100}}); len(got) != 0 {
		t.Errorf("no points: got %v, want empty", got)
	}
}

func TestPBO_InvalidInputs(t *testing.T) {
	flat := func(n, T int) [][]float64 {
		out := make([][]float64, n)
		for i := range out {
			out[i] = make([]float64, T)
		}
		return out
	}
	tests := []struct {
		name string
		rets [][]float64
		s    int
	}{
		{"fewer than three variants", flat(2, 100), 4},
		{"fewer than four blocks", flat(5, 100), 2},
		{"odd block count", flat(5, 100), 5},
		{"blocks shorter than two observations", flat(5, 7), 4},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pbo, splits, rank := PBO(tc.rets, tc.s)
			if !math.IsNaN(pbo) || splits != 0 || !math.IsNaN(rank) {
				t.Errorf("PBO = %v, %d, %v; want NaN, 0, NaN", pbo, splits, rank)
			}
		})
	}
}

func TestPBO_SplitCountIsNChooseHalf(t *testing.T) {
	rng := testRNG(7)
	rets := make([][]float64, 4)
	for v := range rets {
		rets[v] = make([]float64, 120)
		for i := range rets[v] {
			rets[v][i] = 0.01 * rng.norm()
		}
	}
	for s, want := range map[int]int{4: 6, 6: 20, 8: 70, 10: 252} {
		_, got, _ := PBO(rets, s)
		if got != want {
			t.Errorf("PBO(s=%d) splits = %d, want C(%d,%d) = %d", s, got, s, s/2, want)
		}
	}
}

// noiseVariants returns n variants of T iid normal daily returns (sd 1%), so no variant has any edge.
func noiseVariants(seed uint64, n, T int) [][]float64 {
	rng := testRNG(seed)
	rets := make([][]float64, n)
	for v := range rets {
		rets[v] = make([]float64, T)
		for i := range rets[v] {
			rets[v][i] = 0.01 * rng.norm()
		}
	}
	return rets
}

func TestPBO_PureNoiseIsNearOneHalf(t *testing.T) {
	t.Run("one fixed dataset", func(t *testing.T) {
		// 20 variants of 240 iid days in 12 blocks (924 splits). A single noise dataset is itself random (PBO has a
		// standard deviation near 0.2 across datasets), so this only checks "not near 0 or 1" for one fixed draw.
		pbo, splits, rank := PBO(noiseVariants(1, 20, 240), 12)
		if splits != 924 {
			t.Fatalf("splits = %d, want C(12,6) = 924", splits)
		}
		if pbo < 0.25 || pbo > 0.75 {
			t.Errorf("PBO on iid noise = %.3f, want within 0.25..0.75", pbo)
		}
		if rank < 0.25 || rank > 0.75 {
			t.Errorf("mean OOS rank on iid noise = %.3f, want within 0.25..0.75", rank)
		}
	})
	t.Run("average over 40 fixed datasets", func(t *testing.T) {
		// Under pure noise the in-sample winner's out-of-sample rank is uniform, so PBO's expected value is 0.5 and
		// the mean relative rank is 0.5. Averaging 40 datasets shrinks the spread to about 0.04.
		const n, T, s, reps = 10, 160, 8, 40
		var sumP, sumR float64
		for seed := uint64(1); seed <= reps; seed++ {
			p, _, r := PBO(noiseVariants(seed, n, T), s)
			sumP += p
			sumR += r
		}
		if m := sumP / reps; m < 0.40 || m > 0.60 {
			t.Errorf("mean PBO over %d noise datasets = %.3f, want within 0.40..0.60", reps, m)
		}
		if m := sumR / reps; m < 0.40 || m > 0.60 {
			t.Errorf("mean OOS rank over %d noise datasets = %.3f, want within 0.40..0.60", reps, m)
		}
	})
}

func TestPBO_OddVariantCountCountsTheMedianAsOverfit(t *testing.T) {
	// With 5 variants the relative OOS rank is r/6, so the median variant sits exactly at 0.5 and counts as
	// "at or below the median". Under pure noise that makes the expected PBO 3/5 = 0.6 rather than 0.5.
	const n, T, s, reps = 5, 160, 4, 40
	var sum float64
	for seed := uint64(1); seed <= reps; seed++ {
		p, _, _ := PBO(noiseVariants(seed, n, T), s)
		sum += p
	}
	if m := sum / reps; m < 0.52 || m > 0.72 {
		t.Errorf("mean PBO over %d noise datasets with 5 variants = %.3f, want about 0.6 (0.52..0.72)", reps, m)
	}
}

func TestPBO_OneGenuinelyBetterVariantHasZeroOverfitting(t *testing.T) {
	// Variant 0 earns 0.2% a day with 0.1% noise (a per-day Sharpe near 2) in every block; nine others are
	// zero-mean noise with 1% volatility. Variant 0 is the in-sample winner in every split and also the best
	// out of sample, so its relative rank is always n/(n+1) = 10/11 and PBO is 0.
	const n, T, s = 10, 160, 8
	rets := noiseVariants(21, n, T)
	rng := testRNG(99)
	for i := range rets[0] {
		rets[0][i] = 0.002 + 0.001*rng.norm()
	}
	pbo, splits, rank := PBO(rets, s)
	if splits != 70 {
		t.Fatalf("splits = %d, want 70", splits)
	}
	if pbo != 0 {
		t.Errorf("PBO = %v, want exactly 0", pbo)
	}
	near(t, "mean OOS rank", rank, 10.0/11, 1e-12)
}

func TestPBO_RotatingWinnersAreFullyOverfit(t *testing.T) {
	// Four variants, four blocks of 20 days. Variant v earns +1% a day only in block v and loses 0.1% a day in the
	// other three. Whichever pair of blocks is in sample, the winner is a variant that is lucky in sample and so
	// unlucky out of sample (its lucky block is the other half's loss), so it always ranks in the bottom half.
	const n, blk = 4, 20
	rets := make([][]float64, n)
	for v := range rets {
		rets[v] = make([]float64, n*blk)
		for b := 0; b < n; b++ {
			for i := 0; i < blk; i++ {
				base := -0.001
				if b == v {
					base = 0.01
				}
				// small deterministic wiggle so no block has zero variance
				rets[v][b*blk+i] = base + 0.0005*float64((i*7+v*3)%5-2)
			}
		}
	}
	pbo, splits, rank := PBO(rets, 4)
	if splits != 6 {
		t.Fatalf("splits = %d, want 6", splits)
	}
	if pbo != 1 {
		t.Errorf("PBO = %v, want exactly 1", pbo)
	}
	if rank >= 0.5 {
		t.Errorf("mean OOS rank = %v, want below 0.5", rank)
	}
}
