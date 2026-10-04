package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/vishalvx/back-tester/internal/analytics"
	"github.com/vishalvx/back-tester/internal/bench"
	"github.com/vishalvx/back-tester/internal/costs"
	"github.com/vishalvx/back-tester/internal/experiment"
	"github.com/vishalvx/back-tester/internal/panel"
)

type universe struct {
	Name    string
	Weights string // membership file: spells (symbol,from,to,...) or month snapshots (DATE,SYM1,...)
	Source  string // price source: "nse" (official bhavcopy, research/py/nse_build.py) or "yahoo"
	Size    int    // exact member count checked on every trading day (0 = unchecked)
	TRI     string
}

// The nse universes are the point-in-time lists rebuilt from NSE Indices press releases, priced from the official
// bhavcopy. A run on them fails if any trading day has the wrong member count or a member without a price bar.
// The yahoo universes reproduce the October 2026 long-run findings (month snapshots, Yahoo prices, nine Nifty 50 members
// never priced) and are kept to measure how much the data change moves a result.
var universes = map[string]universe{
	"nifty50":        {"nifty50", "internal/data/nifty50_members.csv", "nse", 50, ".research-data/indices/NIFTY_50_TRI.json"},
	"niftymidcap150": {"niftymidcap150", "internal/data/niftymidcap150_members.csv", "nse", 150, ".research-data/indices/NIFTY_MIDCAP_150_TRI.json"},
	"nifty50yahoo":   {"nifty50yahoo", "internal/data/nifty50_weights.csv", "yahoo", 0, ".research-data/indices/NIFTY_50_TRI.json"},
	// Survivorship-bias probe (inbox 006): the Nifty 50 members of Aug 2025 held fixed for every month of 2018-2025.
	"nifty50static": {"nifty50static", "research/rules/nifty50_static_2025-08_weights.csv", "yahoo", 0, ".research-data/indices/NIFTY_50_TRI.json"},
}

// defaultData is where each price source's files live unless -data says otherwise.
var defaultData = map[string]string{"nse": ".research-data/nse/prices", "yahoo": ".research-data/yahoo"}

// sizeExceptions reads the dated periods in which NSE carried a different member count than the index's nominal size,
// from the same hand-checked file the member lists are built with.
func sizeExceptions(universe string) ([]panel.SizeException, error) {
	b, err := os.ReadFile("research/nse/members_overrides.json")
	if err != nil {
		return nil, err
	}
	var ov struct {
		SizeExceptions []struct {
			Index, From, To string
			Size            int
		} `json:"size_exceptions"`
	}
	if err := json.Unmarshal(b, &ov); err != nil {
		return nil, fmt.Errorf("research/nse/members_overrides.json: %v", err)
	}
	var out []panel.SizeException
	for _, x := range ov.SizeExceptions {
		if x.Index == universe {
			out = append(out, panel.SizeException{From: mustDate(x.From), To: mustDate(x.To), Size: x.Size})
		}
	}
	return out, nil
}

type common struct {
	Universe string
	Start    string
	End      string
	Data     string
	Cache    string
	Capital  float64
	Lag      int
	Scenario string
	LogPath  string
	Rf       float64
	Bench    string // optional TRI file stem overriding the universe's own, e.g. NIFTY_50
}

func gitSHA() string {
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func mustDate(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bad date", s)
		os.Exit(2)
	}
	return t
}

// buildEnv constructs the panel and benchmark series for one window.
func buildEnv(c common) (*experiment.Env, *panel.Membership, error) {
	u, ok := universes[c.Universe]
	if !ok {
		return nil, nil, fmt.Errorf("unknown universe %q", c.Universe)
	}
	start, end := mustDate(c.Start), mustDate(c.End)
	data := c.Data
	if data == "" {
		data = defaultData[u.Source]
	}
	var check *panel.Check
	if u.Size > 0 {
		ex, err := sizeExceptions(u.Name)
		if err != nil {
			return nil, nil, err
		}
		check = &panel.Check{Size: u.Size, Exceptions: ex}
	}
	p, mem, err := panel.Build(panel.Options{DataDir: data, CacheDir: c.Cache, AliasFile: "research/symbol_aliases.json",
		WeightsFile: u.Weights, Universe: u.Name, Start: start, End: end, MAWindow: 20, MembershipLag: c.Lag,
		Source: u.Source, Check: check})
	if err != nil {
		return nil, nil, err
	}
	triPath := u.TRI
	if c.Bench != "" {
		triPath = ".research-data/indices/" + c.Bench + "_TRI.json"
	}
	tri, err := bench.LoadTRI(triPath)
	if err != nil {
		return nil, nil, err
	}
	sc := experiment.Base
	if c.Scenario == "harsh" {
		sc = experiment.Harsh
	}
	env := &experiment.Env{Panel: p, TRI: tri, TRIName: triName(triPath), Rf: c.Rf, Capital: c.Capital, Window: fmt.Sprintf("%s %s..%s lag%d", u.Name, c.Start, c.End, c.Lag),
		LogPath: c.LogPath, GitSHA: gitSHA(), Scenario: sc}
	return env, mem, nil
}

// triName turns an index file path such as .research-data/indices/NIFTY_50_TRI.json into "NIFTY 50 TRI".
func triName(path string) string {
	return strings.ReplaceAll(strings.TrimSuffix(filepath.Base(path), ".json"), "_", " ")
}

func pct(x float64) string { return fmt.Sprintf("%6.2f%%", x*100) }

func benchRows(env *experiment.Env, first, last time.Time) {
	fmt.Println("\nBenchmark: buy and hold the total-return index (growth-option index fund, taxed only on redemption)")
	for _, m := range []costs.TaxMode{costs.Dated, costs.Today} {
		r, err := bench.BuyHold(env.TRI, first, last, env.Capital, m)
		if err != nil {
			fmt.Println("  bench error:", err)
			continue
		}
		name := "tax-dated"
		if m == costs.Today {
			name = "tax-today"
		}
		fmt.Printf("  %-10s gross CAGR %s  after-tax CAGR %s  tax Rs %.0f\n", name, pct(r.GrossCAGR), pct(r.NetCAGR), r.Tax)
	}
}

func printHeader() {
	fmt.Printf("%-34s %-9s %8s %7s %8s %6s %6s %6s %6s %7s %7s %6s\n", "variant", "stage", "CAGR", "vol", "maxDD", "Sharpe", "Sortin", "Calmar", "lots", "winRate", "winAll", "avgHd")
}

func printRow(o *experiment.Outcome) {
	m := o.Metrics
	fmt.Printf("%-34s %-9s %8s %7s %8s %6.2f %6.2f %6.2f %6d %7s %7s %6.0f\n", o.Spec.ID, o.Stage, pct(o.CAGR), pct(m.Vol), pct(m.MaxDD.Depth), m.Sharpe, m.Sortino, m.Calmar,
		m.ClosedLots, pct(m.WinRateClosed), pct(m.WinRateAll), m.AvgHoldDays)
}

var _ = analytics.Point{}
