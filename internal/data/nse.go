package data

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const (
	nseBaseURL  = "https://www.nseindia.com"
	nseIndexAPI = "https://www.nseindia.com/api/equity-stockIndices?index="
)

type nseResponse struct {
	Data []struct {
		Symbol string `json:"symbol"`
	} `json:"data"`
}

// FetchIndexSymbols fetches the list of symbols for a given index from NSE
func FetchIndexSymbols(indexName string) ([]string, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// 1. Visit homepage to grab cookies
	reqBase, err := http.NewRequest("GET", nseBaseURL, nil)
	if err != nil {
		return nil, err
	}
	// NSE blocks requests without proper user agent
	reqBase.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	reqBase.Header.Set("Accept", "*/*")
	reqBase.Header.Set("Accept-Language", "en-US,en;q=0.5")

	respBase, err := client.Do(reqBase)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch NSE homepage: %w", err)
	}
	defer respBase.Body.Close()

	cookies := respBase.Cookies()

	// 2. Request the actual API using grabbed cookies
	encodedIndex := url.QueryEscape(indexName)
	apiURL := nseIndexAPI + encodedIndex

	reqAPI, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	reqAPI.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	reqAPI.Header.Set("Accept", "*/*")
	reqAPI.Header.Set("Accept-Language", "en-US,en;q=0.5")
	for _, cookie := range cookies {
		reqAPI.AddCookie(cookie)
	}

	respAPI, err := client.Do(reqAPI)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch NSE index data: %w", err)
	}
	defer respAPI.Body.Close()

	if respAPI.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("NSE API returned status: %v", respAPI.Status)
	}

	var result nseResponse
	if err := json.NewDecoder(respAPI.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	var symbols []string
	for _, item := range result.Data {
		// Ignore the index itself if it appears in the data
		if item.Symbol != indexName {
			symbols = append(symbols, item.Symbol)
		}
	}

	return symbols, nil
}
