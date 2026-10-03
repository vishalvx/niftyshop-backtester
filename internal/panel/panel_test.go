package panel

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeBars(t *testing.T, dir, ticker string, closes []float64, start time.Time) {
	t.Helper()
	var b strings.Builder
	b.WriteString("Date,Open,High,Low,Close,AdjClose,Volume\n")
	d := start
	for _, c := range closes {
		for d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
			d = d.AddDate(0, 0, 1)
		}
		fmt.Fprintf(&b, "%s,%.2f,%.2f,%.2f,%.2f,%.2f,1000\n", d.Format("2006-01-02"), c, c+1, c-1, c, c)
		d = d.AddDate(0, 0, 1)
	}
	if err := os.WriteFile(filepath.Join(dir, ticker+".csv"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestMembershipLagAndFallback(t *testing.T) {
	dir := t.TempDir()
	w := filepath.Join(dir, "w.csv")
	os.WriteFile(w, []byte("DATE,AAA,BBB\n2020-01-31,1,0\n2020-02-29,1,1\n2020-04-30,0,1\n"), 0o644)
	m, err := LoadMembership(w)
	if err != nil {
		t.Fatal(err)
	}
	d := func(s string) time.Time { x, _ := time.Parse("2006-01-02", s); return x }
	cases := []struct {
		sym  string
		date string
		lag  int
		want bool
	}{
		{"BBB", "2020-01-15", 0, false}, // January snapshot says BBB is out
		{"BBB", "2020-02-10", 0, true},
		{"BBB", "2020-02-10", 1, false}, // one month late: January snapshot
		{"AAA", "2020-03-15", 0, true},  // March has no row: falls back to February
		{"AAA", "2020-04-15", 0, false}, // April snapshot dropped AAA
		{"AAA", "2019-12-15", 0, false}, // before the first snapshot
	}
	for _, c := range cases {
		if got := m.IsMember(c.sym, d(c.date), c.lag); got != c.want {
			t.Errorf("%s on %s lag %d: got %v want %v", c.sym, c.date, c.lag, got, c.want)
		}
	}
}

func TestBuildAliasQuarantineAdjustAndMissing(t *testing.T) {
	dir := t.TempDir()
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	mk := func(n int, base float64) []float64 {
		out := make([]float64, n)
		for i := range out {
			out[i] = base + float64(i)
		}
		return out
	}
	writeBars(t, dir, "NEW.NS", mk(60, 100), start) // alias target
	writeBars(t, dir, "QQQ.NS", mk(60, 50), start)  // quarantined before day 30
	writeBars(t, dir, "ADJ.NS", mk(60, 200), start) // adjusted before day 30
	w := filepath.Join(dir, "w.csv")
	os.WriteFile(w, []byte("DATE,OLD,QQQ,ADJ,GONE\n2020-01-31,1,1,1,1\n2020-02-29,1,1,1,1\n"), 0o644)
	al := filepath.Join(dir, "alias.json")
	os.WriteFile(al, []byte(`{"_c":"x","OLD":"NEW.NS"}`), 0o644)
	// day 30 of the synthetic calendar
	q := start.AddDate(0, 0, 41).Format("2006-01-02")
	p, _, err := Build(panel_opts(dir, w, al, q))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Missing) != 1 || p.Missing[0] != "GONE" {
		t.Fatalf("missing symbols: %v", p.Missing)
	}
	if p.Series["OLD"] == nil || p.Series["OLD"].Ticker != "NEW.NS" {
		t.Fatalf("alias not applied")
	}
	qd, _ := time.Parse("2006-01-02", q)
	if p.Series["QQQ"].Bars[0].Date.Before(qd) {
		t.Fatalf("quarantined bars before %s survived", q)
	}
	// adjusted series: first bar scaled by factor 0.5
	if got := p.Series["ADJ"].Bars[0].Close; got != 100 {
		t.Fatalf("adjusted first close want 100 got %v", got)
	}
	last := p.Series["ADJ"].Bars[len(p.Series["ADJ"].Bars)-1].Close
	if last != 259 {
		t.Fatalf("bars on/after the adjustment date must be untouched: got %v", last)
	}
	// every Day has SMA after the warm-up and sorted by DiffSMA desc
	for _, ds := range p.Dates {
		list := p.Days[ds.Format("2006-01-02")]
		for i := 1; i < len(list); i++ {
			if list[i-1].DiffSMA < list[i].DiffSMA {
				t.Fatalf("day %s not sorted by DiffSMA desc", ds)
			}
		}
	}
}

func panel_opts(dir, weights, alias, quarantineDate string) Options {
	return Options{DataDir: dir, WeightsFile: weights, AliasFile: alias,
		Start: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), End: time.Date(2020, 12, 31, 0, 0, 0, 0, time.UTC), MAWindow: 20,
		Quarantine: map[string]string{"QQQ.NS": quarantineDate},
		Adjust:     map[string][]Adj{"ADJ.NS": {{Date: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, 41).Format("2006-01-02"), Factor: 0.5}}}}
}
