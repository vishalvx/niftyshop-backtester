package data

import "time"

type Bar struct {
	Open   float64   `json:"Open"`
	Close  float64   `json:"Close"`
	High   float64   `json:"High"`
	Low    float64   `json:"Low"`
	Volume int64     `json:"Volume"`
	Date   time.Time `json:"Date"`
}
