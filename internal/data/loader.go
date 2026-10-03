package data

import (
	"crypto/tls"
	"encoding/csv"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/piquette/finance-go"
	"github.com/piquette/finance-go/chart"
	"github.com/piquette/finance-go/datetime"
)

type HeaderTransport struct {
	http.RoundTripper
}

func (t *HeaderTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	return t.RoundTripper.RoundTrip(req)
}

// LoadData downloads historical daily price data for a symbol and writes it to a CSV.
// It fetches starting 50 days before startDate to ensure enough history for indicator calculations.
func LoadData(symbol string, startDateStr string, endDateStr string) error {
	filePath := fmt.Sprintf("internal/data/stocks/%s_%s_to_%s.csv", symbol, startDateStr, endDateStr)
	if _, err := os.Stat(filePath); err == nil {
		fmt.Printf("Data for %s already exists, skipping download.\n", symbol)
		return nil
	}

	// Introduce a small delay to avoid rate-limiting from Yahoo Finance API
	time.Sleep(250 * time.Millisecond)

	// Skip TLS verification — Yahoo Finance certificates are sometimes
	// untrusted on macOS depending on the system root CA bundle.
	// This is acceptable for a read-only backtesting data downloader.
	customClient := &http.Client{
		Transport: &HeaderTransport{
			RoundTripper: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
			},
		},
	}
	finance.SetHTTPClient(customClient)

	startTime, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		return fmt.Errorf("invalid start date: %w", err)
	}
	endTime, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		return fmt.Errorf("invalid end date: %w", err)
	}

	// Fetch 50 days prior to start date to support moving averages
	adjStartTime := startTime.AddDate(0, 0, -50)

	params := &chart.Params{
		Symbol:   symbol,
		Interval: datetime.OneDay,
		Start: &datetime.Datetime{
			Year:  adjStartTime.Year(),
			Month: int(adjStartTime.Month()),
			Day:   adjStartTime.Day(),
		},
		End: &datetime.Datetime{
			Year:  endTime.Year(),
			Month: int(endTime.Month()),
			Day:   endTime.Day(),
		},
	}

	iter := chart.Get(params)

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}

	var success bool
	defer func() {
		file.Close()
		if !success {
			os.Remove(filePath)
		}
	}()

	w := csv.NewWriter(file)
	defer w.Flush()

	// Write CSV headers (raw price data only)
	if err := w.Write([]string{"Date", "Open", "High", "Low", "Close", "AdjClose", "Volume"}); err != nil {
		return err
	}

	for iter.Next() {
		bar := iter.Bar()
		adjClose, _ := bar.AdjClose.Float64()
		barDate := time.Unix(int64(bar.Timestamp), 0).Format("2006-01-02")

		row := []string{
			barDate,
			bar.Open.String(),
			bar.High.String(),
			bar.Low.String(),
			bar.Close.String(),
			strconv.FormatFloat(adjClose, 'f', 6, 64),
			strconv.Itoa(bar.Volume),
		}

		if err := w.Write(row); err != nil {
			return err
		}
	}

	if err := iter.Err(); err != nil {
		return fmt.Errorf("failed to fetch chart data for %s: %w", symbol, err)
	}

	success = true
	return nil
}

// GetAllStock reads historical CSV files from dir and returns a map of symbol -> sorted slice of Bars.
//
// The cache file name carries the requested date range (see LoadData), so one symbol can have several files after
// runs with different windows. Earlier versions kept only the lexically last file, which silently truncated history
// (a "2024-06-04_to_..." file sorts after a "2018-01-01_to_..." file). All files of a symbol are now merged by date;
// where two files disagree on a date the lexically later file wins, and a warning names the symbol.
func GetAllStock(dir string) (map[string][]Bar, error) {
	merged := make(map[string]map[time.Time]Bar)
	filesPerSymbol := make(map[string]int)

	files, err := filepath.Glob(filepath.Join(dir, "*.csv"))
	if err != nil {
		return nil, fmt.Errorf("error globbing csv files: %w", err)
	}

	for _, path := range files {
		file, err := os.Open(path)
		if err != nil {
			fmt.Printf("Error opening file %s: %v\n", path, err)
			continue
		}

		symbol := strings.Split(filepath.Base(file.Name()), "_")[0]
		reader := csv.NewReader(file)
		reader.FieldsPerRecord = -1
		reader.TrimLeadingSpace = true

		records, err := reader.ReadAll()
		file.Close() // Close file immediately since we read all
		if err != nil {
			fmt.Printf("Error reading CSV %s: %v\n", path, err)
			continue
		}

		var bars []Bar
		for idx, r := range records {
			if idx == 0 {
				continue // skip header
			}
			if len(r) < 7 {
				continue
			}

			recordDate, err := time.Parse("2006-01-02", r[0])
			if err != nil {
				continue
			}

			open, _ := strconv.ParseFloat(r[1], 64)
			high, _ := strconv.ParseFloat(r[2], 64)
			low, _ := strconv.ParseFloat(r[3], 64)
			close, _ := strconv.ParseFloat(r[4], 64)
			volume, _ := strconv.ParseInt(r[6], 10, 64)

			bars = append(bars, Bar{
				Date:   recordDate,
				Open:   open,
				High:   high,
				Low:    low,
				Close:  close,
				Volume: volume,
			})
		}

		if merged[symbol] == nil {
			merged[symbol] = make(map[time.Time]Bar)
		}
		for _, b := range bars {
			merged[symbol][b.Date] = b
		}
		filesPerSymbol[symbol]++
	}

	symbolWiseBars := make(map[string][]Bar, len(merged))
	for symbol, byDate := range merged {
		bars := make([]Bar, 0, len(byDate))
		for _, b := range byDate {
			bars = append(bars, b)
		}
		sort.Slice(bars, func(i, j int) bool {
			return bars[i].Date.Before(bars[j].Date)
		})
		symbolWiseBars[symbol] = bars
		if filesPerSymbol[symbol] > 1 {
			fmt.Printf("WARNING: %s has %d cache files in %s; merged by date (%d bars)\n", symbol, filesPerSymbol[symbol], dir, len(bars))
		}
	}

	return symbolWiseBars, nil
}

// LoadHistoricalConstituents reads index weights CSV file and returns the month-wise map (YYYY-MM -> symbol -> isConstituent), and the set of unique symbols.
func LoadHistoricalConstituents(path string) (map[string]map[string]bool, []string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, nil, err
	}

	if len(records) < 2 {
		return nil, nil, fmt.Errorf("CSV file has too few rows")
	}

	header := records[0]

	monthMap := make(map[string]map[string]bool)
	uniqueSymbolsMap := make(map[string]bool)

	for i := 1; i < len(records); i++ {
		row := records[i]
		if len(row) < len(header) {
			continue
		}

		dateStr := row[0]
		parsedDate, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}
		monthKey := parsedDate.Format("2006-01")

		if _, exists := monthMap[monthKey]; !exists {
			monthMap[monthKey] = make(map[string]bool)
		}

		for j := 1; j < len(row); j++ {
			weightVal, err := strconv.ParseFloat(row[j], 64)
			if err != nil {
				continue
			}
			symbol := header[j] + ".NS"
			if weightVal > 0.0 {
				monthMap[monthKey][symbol] = true
				uniqueSymbolsMap[symbol] = true
			}
		}
	}

	var uniqueSymbols []string
	for sym := range uniqueSymbolsMap {
		uniqueSymbols = append(uniqueSymbols, sym)
	}

	return monthMap, uniqueSymbols, nil
}
