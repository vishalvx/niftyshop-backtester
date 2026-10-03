// Package panel loads a clean, reproducible price panel for research runs.
//
// Differences from internal/data (the production loader), all deliberate:
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
	// LegacyGlobDir reproduces internal/data.GetAllStock: every *.csv in the directory is read, the symbol is the file name
	// up to the first '_', and for duplicate symbols the lexically LAST file silently wins. Used only to reproduce
	// historical engine results; never for research runs.
	LegacyGlobDir string
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

// Membership maps month key (YYYY-MM) -> symbol set, replicating internal/data.LoadHistoricalConstituents.
type Membership struct {
	Months  map[string]map[string]bool
	Keys    []string
	Symbols []string
}

// LoadMembership reads a weights CSV the same way the production loader does: rows sharing a calendar month
// are unioned, any positive value counts as membership.
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

// IsMember mirrors cmd/backtester/main.go: exact month, else the closest earlier month, else false.
// lag shifts the lookup key back by that many months (use 1 to avoid using a snapshot taken after the date).
func (m *Membership) IsMember(symbol string, t time.Time, lag int) bool {
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
	aliases := loadAliases(o.AliasFile)
	quarantine := o.Quarantine
	if quarantine == nil {
		quarantine = DefaultQuarantine
	}

	p := &Panel{Universe: o.Universe, Start: o.Start, End: o.End, Days: map[string][]Day{}, Series: map[string]*Series{}}
	seen := map[string]bool{}
	for _, sym := range symbols {
		if seen[sym] {
			continue
		}
		seen[sym] = true
		ticker := sym + ".NS"
		if a, ok := aliases[sym]; ok {
			ticker = a
		}
		src := "yahoo"
		path := filepath.Join(o.DataDir, strings.ReplaceAll(ticker, "&", "_and_")+".csv")
		var bars []Bar
		var err error
		if o.LegacyGlobDir != "" {
			files, _ := filepath.Glob(filepath.Join(o.LegacyGlobDir, "*.csv"))
			sort.Strings(files)
			path = ""
			for _, f := range files {
				if strings.Split(filepath.Base(f), "_")[0] == sym+".NS" {
					path = f // last wins, exactly like the engine
				}
			}
			src = "legacy-glob"
			if path == "" {
				err = os.ErrNotExist
			} else {
				bars, err = readBars(path)
			}
		} else {
			bars, err = readBars(path)
		}
		if (err != nil || len(bars) == 0) && o.LegacyGlobDir == "" {
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
		if adjs == nil {
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
	return p, mem, nil
}
