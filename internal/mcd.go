package internal

import "time"

type MCD struct {
	ID               string          `json:"id"`
	Original         string          `json:"original"`
	Number           int             `json:"number"`
	Polygon          *PolygonFeature `json:"polygon"`
	Issued           time.Time       `json:"issued"`
	Expires          time.Time       `json:"expires"`
	WatchProbability int             `json:"watch_probability"`
}

func ParseMCD(product Product) error {

	return nil
}
