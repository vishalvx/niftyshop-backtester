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

func TestSpellMembership(t *testing.T) {
	dir := t.TempDir()
	w := filepath.Join(dir, "m.csv")
	os.WriteFile(w, []byte("symbol,from,to,added_by,removed_by\nAAA,2020-01-01,2020-03-30,start,r1\nAAA,2020-06-29,,r2,\nBBB,2020-03-30,,r1,\n"), 0o644)
	m, err := LoadMembership(w)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Exact() {
		t.Fatal("a symbol,from,to file must load as exact spells")
	}
	d := func(s string) time.Time { x, _ := time.Parse("2006-01-02", s); return x }
	cases := []struct {
		sym, date string
		want      bool
	}{
		{"AAA", "2019-12-31", false}, // before the list starts
		{"AAA", "2020-01-01", true},  // first day of a spell counts
		{"AAA", "2020-03-27", true},
		{"AAA", "2020-03-30", false}, // `to` is exclusive: the replacement takes effect that day
		{"BBB", "2020-03-30", true},
		{"AAA", "2020-06-29", true}, // rejoined, open-ended
		{"AAA", "2026-01-01", true},
	}
	for _, c := range cases {
		if got := m.IsMember(c.sym, d(c.date), 0); got != c.want {
			t.Errorf("%s on %s: got %v want %v", c.sym, c.date, got, c.want)
		}
	}
	if got := m.Members(d("2020-04-01"), 0); len(got) != 1 || got[0] != "BBB" {
		t.Errorf("members on 2020-04-01: %v", got)
	}
	os.WriteFile(w, []byte("symbol,from,to\nAAA,2020-03-30,2020-03-30\n"), 0o644)
	if _, err := LoadMembership(w); err == nil {
		t.Error("a spell that ends the day it starts must be rejected")
	}
}

// nseFixture writes two always-traded stocks plus one that joins mid-way, and a spell file of a two-member index.
func nseFixture(t *testing.T) (dir, members string) {
	t.Helper()
	dir = t.TempDir()
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	flat := func(n int, base float64) []float64 {
		out := make([]float64, n)
		for i := range out {
			out[i] = base
		}
		return out
	}
	writeBars(t, dir, "AAA", flat(80, 100), start)
	writeBars(t, dir, "B&B", flat(80, 50), start) // '&' is stored as _and_ on disk
	writeBars(t, dir, "CCC", flat(80, 20), start)
	os.WriteFile(filepath.Join(dir, "AAA_div.csv"), []byte("Date,Dividend\n2020-03-02,2.5\n"), 0o644)
	members = filepath.Join(dir, "m.csv")
	os.WriteFile(members, []byte("symbol,from,to\nAAA,2020-01-01,\nB&B,2020-01-01,2020-03-02\nCCC,2020-03-02,\n"), 0o644)
	os.Rename(filepath.Join(dir, "B&B.csv"), filepath.Join(dir, "B_and_B.csv"))
	return dir, members
}

func nseOpts(dir, members string) Options {
	return Options{DataDir: dir, WeightsFile: members, Universe: "test", Source: "nse", MAWindow: 20,
		AliasFile: filepath.Join(dir, "none.json"), Check: &Check{Size: 2},
		Start: time.Date(2020, 2, 3, 0, 0, 0, 0, time.UTC), End: time.Date(2020, 4, 10, 0, 0, 0, 0, time.UTC)}
}

func TestBuildNSESourcePassesChecks(t *testing.T) {
	dir, members := nseFixture(t)
	p, _, err := Build(nseOpts(dir, members))
	if err != nil {
		t.Fatal(err)
	}
	if p.Series["B&B"] == nil || p.Series["B&B"].Source != "nse" || p.Series["AAA"].Ticker != "AAA" {
		t.Fatalf("nse series not loaded under their own symbols: %+v", p.Series["B&B"])
	}
	if p.Series["AAA"].Div["2020-03-02"] != 2.5 {
		t.Errorf("dividend file not read: %v", p.Series["AAA"].Div)
	}
	for _, d := range p.Days["2020-03-02"] {
		if d.Symbol == "B&B" && d.IsConstituent {
			t.Error("B&B left on 2020-03-02 but is still a constituent that day")
		}
		if d.Symbol == "CCC" && !d.IsConstituent {
			t.Error("CCC joined on 2020-03-02 but is not a constituent that day")
		}
	}
}

func TestBuildChecksFailTheRun(t *testing.T) {
	cases := []struct {
		name  string
		edit  func(dir, members string, o *Options)
		error string
	}{
		{"wrong member count", func(dir, members string, o *Options) { o.Check.Size = 3 }, "has 2 members, want 3"},
		{"a dated exception is honoured", func(dir, members string, o *Options) {
			o.Check.Size = 3
			o.Check.Exceptions = []SizeException{{From: o.Start, To: o.End.AddDate(0, 0, 1), Size: 2}}
		}, ""},
		{"member without a price file", func(dir, members string, o *Options) { os.Remove(filepath.Join(dir, "CCC.csv")) }, "no price file"},
		{"member without a bar on a trading day", func(dir, members string, o *Options) {
			b, _ := os.ReadFile(filepath.Join(dir, "CCC.csv"))
			lines := strings.Split(string(b), "\n")
			var kept []string
			for _, l := range lines {
				if !strings.HasPrefix(l, "2020-03-10") {
					kept = append(kept, l)
				}
			}
			os.WriteFile(filepath.Join(dir, "CCC.csv"), []byte(strings.Join(kept, "\n")), 0o644)
		}, "2020-03-10: member CCC has no price bar"},
		{"lag on exact dates", func(dir, members string, o *Options) { o.MembershipLag = 1 }, "does not apply to exact membership dates"},
		{"month snapshots", func(dir, members string, o *Options) {
			os.WriteFile(members, []byte("DATE,AAA,CCC\n2020-01-31,1,1\n"), 0o644)
		}, "need a membership-spell file"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir, members := nseFixture(t)
			o := nseOpts(dir, members)
			c.edit(dir, members, &o)
			_, _, err := Build(o)
			if c.error == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.error) {
				t.Fatalf("want error containing %q, got %v", c.error, err)
			}
		})
	}
}
