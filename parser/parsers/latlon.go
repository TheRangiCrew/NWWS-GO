package parsers

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
)

type LATLON struct {
	Original string          `json:"original"`
	Points   [][2]float64    `json:"points"`
	Polygon  *PolygonFeature `json:"-"`
}

type PolygonFeature struct {
	Type        string         `json:"type"`
	Coordinates [][][2]float64 `json:"coordinates"`
}

func ParseLatLon(text string) (*LATLON, error) {
	latlonRegexp := regexp.MustCompile(`(?m:^LAT\.\.\.LON\s+(\d+\s+)+)`)
	original := latlonRegexp.FindString(text)

	if original == "" {
		return nil, nil
	}

	segmentRegexp := regexp.MustCompile("[0-9]{4,}")
	segments := segmentRegexp.FindAllString(original, -1)

	points := [][2]float64{}
	if len(segments[0]) > 5 {
		for _, s := range segments {
			latInit, err := strconv.Atoi(s[0:4])
			if err != nil {
				return nil, errors.New("Failed to parse LAT...LON lat")
			}

			lonInit, err := strconv.Atoi(s[4:8])
			if err != nil {
				return nil, errors.New("Failed to parse LAT...LON lon")
			}

			fmt.Println(latInit)
			fmt.Println(lonInit)

			lat := (float64(latInit) / 100)
			lon := (float64(lonInit) / 100) * -1
			fmt.Println(lat)
			fmt.Println(lon)
			if lon > -20.0 {
				lon = lon + -100
			}

			points = append(points, [2]float64{lon, lat})
			fmt.Println(points)
		}
	} else {
		for i := 0; i < len(segments); i += 2 {
			latInit, err := strconv.Atoi(segments[i])
			if err != nil {
				return nil, errors.New("Failed to parse LAT...LON lat")
			}
			lonInit, err := strconv.Atoi(segments[i+1])
			if err != nil {
				return nil, errors.New("Failed to parse LAT...LON lon")
			}

			lat := (float64(latInit) / 100)
			lon := (float64(lonInit) / 100) * -1
			if lon <= -180.0 {
				lon = lon + 360.0
			}
			points = append(points, [2]float64{lon, lat})
		}
		points = append(points, points[0])
	}

	polygon := PolygonFeature{
		Type:        "Polygon",
		Coordinates: [][][2]float64{points},
	}

	return &LATLON{
		Original: original,
		Points:   points,
		Polygon:  &polygon,
	}, nil

}
