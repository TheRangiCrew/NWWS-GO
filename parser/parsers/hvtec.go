package parsers

import (
	"regexp"
	"time"
)

type HVTEC struct {
	Original string
}

func ParseHVTEC(text string, issued time.Time) *HVTEC {
	vtecRegexp := regexp.MustCompile(`([A-Z0-9]){5}\.([0-3UN])\.([A-Z]{2})(\.[0-9TZ]+){3}\.(OO|NO|NR|UU)`)
	original := vtecRegexp.FindString(text)

	if original == "" {
		return nil
	}

	return &HVTEC{
		Original: original,
	}
}
