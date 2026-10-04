// Package panel loads a clean, reproducible price panel for research runs.
//
// Differences from internal/data (the original engine's loader, since deleted), all deliberate:
//   - one canonical CSV per Yahoo ticker (no date-range file names, so no glob collisions);
//   - dividends are loaded so total-return accounting is possible;
//   - symbol aliases map index-constituent symbols to current Yahoo tickers;
//   - symbols without any price history are reported, not silently dropped;
//   - known corporate-action discontinuities can be quarantined (data before the break is unusable).
package panel

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/vishalvx/back-tester/internal/indicators"
)

// Bar is one daily bar. Close is split/bonus adjusted by Yahoo but NOT dividend adjusted.
type Bar struct {
	Date     time.Time
	Open     float64
	High     float64
	Low      float64
	Close    float64
	AdjClose float64
	Volume   int64
}

// Series is the full history of one constituent symbol.
type Series struct {
	Symbol string
	Ticker string
	Source string // "yahoo" or "cache"
	Bars   []Bar
	Div    map[string]float64 // YYYY-MM-DD -> cash dividend per share (same share basis as Close)
}

// Day is one stock on one trading day, with indicators precomputed from that symbol's own bars.
type Day struct {
	Symbol        string
	Bar           Bar
	SMA           float64
	DiffSMA       float64 // (SMA - Close)/Close*100, positive means below SMA
	IsConstituent bool
	Pivots        indicators.PivotLevels // from the previous bar of the same symbol
	Dividend      float64                // cash dividend per share going ex on this day
	NextOpen      float64                // next bar's open for the same symbol (0 if none)
}

// Panel holds everything a simulation needs.
type Panel struct {
	Universe string
	Start    time.Time
	End      time.Time
	Dates    []time.Time      // trading dates inside [Start, End], ascending
	Days     map[string][]Day // YYYY-MM-DD -> stocks that traded, sorted by DiffSMA desc (ties by symbol)
	Series   map[string]*Series
	Missing  []string // universe symbols with no usable price history
}

// Options controls panel construction.
type Options struct {
	DataDir       string // directory holding <TICKER>.csv, <TICKER>_div.csv
	CacheDir      string // fallback directory for tickers Yahoo no longer serves
	AliasFile     string // JSON symbol -> ticker
	WeightsFile   string // constituent CSV (DATE,SYM1,SYM2...)
	Universe      string
	Start, End    time.Time
	MAWindow      int
	MembershipLag int               // 0 = engine behaviour (snapshot of the same calendar month); 1 = previous month's snapshot
	NoMembership  bool              // treat every loaded symbol as a constituent every day (static universe)
	Quarantine    map[string]string // ticker -> first date (YYYY-MM-DD) from which data is trusted
	Adjust        map[string][]Adj  // ticker -> hand adjustments (nil = DefaultAdjust, empty map = none)
	ExtraSymbols  []string          // extra symbols to load even if not in weights file
	// Source is "yahoo" (default: <SYMBOL>.NS files from research/py/fetch_yahoo.py, with aliases, quarantines and hand
	// adjustments applied here) or "nse" (<SYMBOL>.csv files from research/py/nse_build.py: official bhavcopy bars already
	// stitched across renames and back-adjusted, so none of the Yahoo repairs apply).
	Source string
	// Check, when set, makes Build fail instead of running on data that cannot support an honest result.
	Check *Check
}

// Check lists what Build verifies for every trading day of the window when Options.Check is set.
type Check struct {
	Size       int             // exact member count of the index
	Exceptions []SizeException // dated periods when NSE carried a different count
}

// SizeException records a period [From, To) in which the index had Size securities (the Nifty 50 had 51 while it held the
// Tata Motors DVR share).
type SizeException struct {
	From, To time.Time
	Size     int
}

func (c *Check) sizeOn(t time.Time) int {
	for _, x := range c.Exceptions {
		if !t.Before(x.From) && t.Before(x.To) {
			return x.Size
		}
	}
	return c.Size
}

// DefaultQuarantine lists known unadjusted corporate-action breaks in Yahoo's Close series.
// Bars before the date are discarded for that ticker (evidence in the research report).
var DefaultQuarantine = map[string]string{
	"BAJAJFINSV.NS": "2008-05-26", // unadjusted face-value split: 675 -> 49.6 (-92.6%)
	"NESTLEIND.NS":  "2010-01-08", // unadjusted split plus stale zero-volume prints: 527 -> 124 (-76%)
}

// Adj scales every price strictly before Date by Factor (a hand-made corporate-action adjustment).
type Adj struct {
	Date   string
	Factor float64
}

// DefaultAdjust fixes demergers that Yahoo's Close series does not adjust for. Factor = post/pre close on the
// event day, i.e. it assumes no market move that day (approximation; documented in the report).
var DefaultAdjust = map[string][]Adj{
	"TMPV.NS": {{Date: "2025-10-14", Factor: 395.45 / 660.75}}, // Tata Motors demerger into TMPV + TMCV
}

func parseDate(s string) (time.Time, error) { return time.Parse("2006-01-02", s) }

func readBars(path string) ([]Bar, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	recs, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	var bars []Bar
	for i, rec := range recs {
		if i == 0 || len(rec) < 7 {
			continue
		}
		d, err := parseDate(rec[0])
		if err != nil {
			continue
		}
		cl, err := strconv.ParseFloat(rec[4], 64)
		if err != nil || cl <= 0 {
			continue
		}
		o, _ := strconv.ParseFloat(rec[1], 64)
		h, _ := strconv.ParseFloat(rec[2], 64)
		l, _ := strconv.ParseFloat(rec[3], 64)
		ac, _ := strconv.ParseFloat(rec[5], 64)
		v, _ := strconv.ParseInt(rec[6], 10, 64)
		if h <= 0 || l <= 0 {
			h, l = cl, cl
		}
		bars = append(bars, Bar{Date: d, Open: o, High: h, Low: l, Close: cl, AdjClose: ac, Volume: v})
	}
	sort.Slice(bars, func(i, j int) bool { return bars[i].Date.Before(bars[j].Date) })
	return bars, nil
}

func readDiv(path string) map[string]float64 {
	out := map[string]float64{}
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()
	recs, _ := csv.NewReader(f).ReadAll()
	for i, rec := range recs {
		if i == 0 || len(rec) < 2 {
			continue
		}
		v, err := strconv.ParseFloat(rec[1], 64)
		if err == nil && v > 0 {
			out[rec[0]] += v
		}
	}
	return out
}

// Membership answers "was this symbol an index member on this day?" from either of two file formats:
//   - month snapshots, "DATE,SYM1,SYM2,..." with weights (the retired engine's weights files): a snapshot applies to its
//     whole calendar month, as internal/data.LoadHistoricalConstituents did;
//   - membership spells, "symbol,from,to,added_by,removed_by" (internal/data/*_members.csv, rebuilt from NSE Indices press
//     releases by research/py/nse_build.py): a symbol is a member from `from` up to but not including `to` (empty = today).
type Membership struct {
	Months  map[string]map[string]bool // snapshot files only
	Keys    []string                   // snapshot files only
	Symbols []string
	Spells  map[string][]Spell // spell files only
}

// Spell is one continuous membership of a symbol; To is exclusive and zero while it is still a member.
type Spell struct {
	From, To time.Time
}

// Exact reports whether the file gives exact effective dates (spells) rather than month snapshots.
func (m *Membership) Exact() bool { return m.Spells != nil }

// LoadMembership reads either format (see Membership). Snapshot rows sharing a calendar month are unioned and any
// positive value counts as membership, the same way the production loader did.
func LoadMembership(path string) (*Membership, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	recs, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, err
	}
	if len(recs) < 2 {
		return nil, fmt.Errorf("weights file %s has too few rows", path)
	}
	hdr := recs[0]
	if hdr[0] == "symbol" {
		return loadSpells(path, recs)
	}
	m := &Membership{Months: map[string]map[string]bool{}}
	uniq := map[string]bool{}
	for _, row := range recs[1:] {
		if len(row) < len(hdr) {
			continue
		}
		d, err := parseDate(row[0])
		if err != nil {
			continue
		}
		k := d.Format("2006-01")
		if m.Months[k] == nil {
			m.Months[k] = map[string]bool{}
		}
		for j := 1; j < len(row); j++ {
			v, err := strconv.ParseFloat(row[j], 64)
			if err == nil && v > 0 {
				m.Months[k][hdr[j]] = true
				uniq[hdr[j]] = true
			}
		}
	}
	for k := range m.Months {
		m.Keys = append(m.Keys, k)
	}
	sort.Strings(m.Keys)
	for s := range uniq {
		m.Symbols = append(m.Symbols, s)
	}
	sort.Strings(m.Symbols)
	return m, nil
}

func loadSpells(path string, recs [][]string) (*Membership, error) {
	m := &Membership{Spells: map[string][]Spell{}}
	for i, row := range recs[1:] {
		if len(row) < 3 {
			return nil, fmt.Errorf("%s line %d: want symbol,from,to", path, i+2)
		}
		from, err := parseDate(row[1])
		if err != nil {
			return nil, fmt.Errorf("%s line %d: %v", path, i+2, err)
		}
		var to time.Time
		if row[2] != "" {
			if to, err = parseDate(row[2]); err != nil {
				return nil, fmt.Errorf("%s line %d: %v", path, i+2, err)
			}
			if !to.After(from) {
				return nil, fmt.Errorf("%s line %d: %s leaves on %s, not after it joins on %s", path, i+2, row[0], row[2], row[1])
			}
		}
		if len(m.Spells[row[0]]) == 0 {
			m.Symbols = append(m.Symbols, row[0])
		}
		m.Spells[row[0]] = append(m.Spells[row[0]], Spell{From: from, To: to})
	}
	sort.Strings(m.Symbols)
	return m, nil
}

// IsMember answers for one day. For a snapshot file it mirrors cmd/backtester/main.go: the exact month, else the closest
// earlier month, else false; lag shifts the month back (1 avoids using a month-end snapshot taken after the date).
// For a spell file the answer is exact and lag must be 0: NSE announces every change weeks before it takes effect.
func (m *Membership) IsMember(symbol string, t time.Time, lag int) bool {
	if m.Spells != nil {
		for _, s := range m.Spells[symbol] {
			if !t.Before(s.From) && (s.To.IsZero() || t.Before(s.To)) {
				return true
			}
		}
		return false
	}
	key := time.Date(t.Year(), t.Month()-time.Month(lag), 1, 0, 0, 0, 0, time.UTC).Format("2006-01") // first of the month, so a 31st cannot overflow into the same month
	if set, ok := m.Months[key]; ok {
		return set[symbol]
	}
	i := sort.SearchStrings(m.Keys, key) // first index with Keys[i] >= key
	if i == 0 {
		return false
	}
	return m.Months[m.Keys[i-1]][symbol]
}

// Members lists the symbols that were members on day t, sorted.
func (m *Membership) Members(t time.Time, lag int) []string {
	var out []string
	for _, s := range m.Symbols {
		if m.IsMember(s, t, lag) {
			out = append(out, s)
		}
	}
	return out
}

func loadAliases(path string) map[string]string {
	out := map[string]string{}
	if path == "" {
		return out
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	var raw map[string]any
	if json.Unmarshal(b, &raw) != nil {
		return out
	}
	for k, v := range raw {
		if s, ok := v.(string); ok && !strings.HasPrefix(k, "_") {
			out[k] = s
		}
	}
	return out
}

// Build constructs the panel.
func Build(o Options) (*Panel, *Membership, error) {
	if o.MAWindow <= 0 {
		o.MAWindow = 20
	}
	var mem *Membership
	var symbols []string
	if o.WeightsFile != "" {
		var err error
		mem, err = LoadMembership(o.WeightsFile)
		if err != nil {
			return nil, nil, err
		}
		symbols = append(symbols, mem.Symbols...)
	}
	symbols = append(symbols, o.ExtraSymbols...)
	nse := o.Source == "nse"
	if o.Source != "" && o.Source != "yahoo" && !nse {
		return nil, nil, fmt.Errorf("unknown price source %q (want yahoo or nse)", o.Source)
	}
	aliases := loadAliases(o.AliasFile)
	quarantine := o.Quarantine
	if quarantine == nil {
		quarantine = DefaultQuarantine
	}
	if nse {
		aliases, quarantine = map[string]string{}, map[string]string{}
		o.CacheDir = ""
	}

	p := &Panel{Universe: o.Universe, Start: o.Start, End: o.End, Days: map[string][]Day{}, Series: map[string]*Series{}}
	seen := map[string]bool{}
	for _, sym := range symbols {
		if seen[sym] {
			continue
		}
		seen[sym] = true
		ticker, src := sym+".NS", "yahoo"
		if nse {
			ticker, src = sym, "nse"
		}
		if a, ok := aliases[sym]; ok {
			ticker = a
		}
		path := filepath.Join(o.DataDir, strings.ReplaceAll(ticker, "&", "_and_")+".csv")
		bars, err := readBars(path)
		if err != nil || len(bars) == 0 {
			if o.CacheDir != "" {
				path = filepath.Join(o.CacheDir, strings.ReplaceAll(ticker, "&", "_and_")+".csv")
				bars, err = readBars(path)
				src = "cache"
			}
		}
		if err != nil || len(bars) == 0 {
			p.Missing = append(p.Missing, sym)
			continue
		}
		if q, ok := quarantine[ticker]; ok {
			qd, _ := parseDate(q)
			var kept []Bar
			for _, b := range bars {
				if !b.Date.Before(qd) {
					kept = append(kept, b)
				}
			}
			bars = kept
		}
		div := readDiv(strings.TrimSuffix(path, ".csv") + "_div.csv")
		adjs := o.Adjust
		if adjs == nil && !nse {
			adjs = DefaultAdjust
		}
		for _, a := range adjs[ticker] {
			ad, _ := parseDate(a.Date)
			for i := range bars {
				if bars[i].Date.Before(ad) {
					bars[i].Open *= a.Factor
					bars[i].High *= a.Factor
					bars[i].Low *= a.Factor
					bars[i].Close *= a.Factor
					bars[i].AdjClose *= a.Factor
				}
			}
			for k, v := range div {
				if dd, _ := parseDate(k); dd.Before(ad) {
					div[k] = v * a.Factor
				}
			}
		}
		p.Series[sym] = &Series{Symbol: sym, Ticker: ticker, Source: src, Bars: bars, Div: div}
	}

	dateSet := map[string]bool{}
	for sym, s := range p.Series {
		closes := make([]float64, len(s.Bars))
		for i, b := range s.Bars {
			closes[i] = b.Close
		}
		sma := indicators.CalculateSMA(closes, o.MAWindow)
		for i, b := range s.Bars {
			if i < o.MAWindow-1 || i == 0 {
				continue
			}
			if b.Date.Before(o.Start) || b.Date.After(o.End) {
				continue
			}
			isC := true
			if mem != nil && !o.NoMembership {
				isC = mem.IsMember(sym, b.Date, o.MembershipLag)
			}
			ds := b.Date.Format("2006-01-02")
			prev := s.Bars[i-1]
			nextOpen := 0.0
			if i+1 < len(s.Bars) {
				nextOpen = s.Bars[i+1].Open
			}
			day := Day{
				Symbol:        sym,
				Bar:           b,
				SMA:           sma[i],
				DiffSMA:       (sma[i] - b.Close) / b.Close * 100,
				IsConstituent: isC,
				Pivots:        indicators.CalculatePivotLevels(prev.High, prev.Low, prev.Close),
				Dividend:      s.Div[ds],
				NextOpen:      nextOpen,
			}
			p.Days[ds] = append(p.Days[ds], day)
			dateSet[ds] = true
		}
	}
	for ds := range dateSet {
		d, _ := parseDate(ds)
		p.Dates = append(p.Dates, d)
		list := p.Days[ds]
		sort.SliceStable(list, func(i, j int) bool {
			if list[i].DiffSMA != list[j].DiffSMA {
				return list[i].DiffSMA > list[j].DiffSMA
			}
			return list[i].Symbol < list[j].Symbol
		})
	}
	sort.Slice(p.Dates, func(i, j int) bool { return p.Dates[i].Before(p.Dates[j]) })
	sort.Strings(p.Missing)
	if o.Check != nil {
		if err := check(p, mem, o); err != nil {
			return nil, nil, err
		}
	}
	return p, mem, nil
}

// check fails the build when a run would silently trade a wrong universe: a member with no price file, a day whose member
// count is not the index's, or a member with no bar on a day the market traded.
func check(p *Panel, mem *Membership, o Options) error {
	if mem == nil || !mem.Exact() {
		return fmt.Errorf("%s: member checks need a membership-spell file (symbol,from,to), not month snapshots", o.Universe)
	}
	if o.MembershipLag != 0 {
		return fmt.Errorf("%s: -lag %d does not apply to exact membership dates (NSE announces each change weeks before it "+
			"takes effect); use -lag 0", o.Universe, o.MembershipLag)
	}
	if len(p.Missing) > 0 {
		return fmt.Errorf("%s: %d members have no price file in %s: %s (run research/py/nse_build.py prices)",
			o.Universe, len(p.Missing), o.DataDir, strings.Join(p.Missing, ", "))
	}
	if len(p.Dates) == 0 {
		return fmt.Errorf("%s: no trading days with prices between %s and %s", o.Universe, o.Start.Format("2006-01-02"), o.End.Format("2006-01-02"))
	}
	var problems []string
	for _, t := range p.Dates {
		ds := t.Format("2006-01-02")
		members := mem.Members(t, 0)
		if want := o.Check.sizeOn(t); len(members) != want {
			problems = append(problems, fmt.Sprintf("%s has %d members, want %d", ds, len(members), want))
		}
		traded := map[string]bool{}
		for _, d := range p.Days[ds] {
			traded[d.Symbol] = true
		}
		for _, s := range members {
			if !traded[s] {
				problems = append(problems, fmt.Sprintf("%s: member %s has no price bar", ds, s))
			}
		}
	}
	if len(problems) == 0 {
		return nil
	}
	shown := problems
	if len(shown) > 10 {
		shown = shown[:10]
	}
	return fmt.Errorf("%s: %d member-list or price problems, for example:\n  %s", o.Universe, len(problems), strings.Join(shown, "\n  "))
}
