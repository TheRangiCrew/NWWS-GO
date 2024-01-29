package parsers

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

type WMO struct {
	Original string    `json:"original"`
	Datatype string    `json:"datatype"`
	WFO      string    `json:"wfo"`
	Issued   time.Time `json:"issued"`
	BBB      string    `json:"bbb,omitempty"`
}

func ParseWMO(text string, issuedT time.Time) (WMO, error) {
	wmoRegexp := regexp.MustCompile(`([0-9A-Z]{6})\s([A-Z]{4})\s([0-9]{6})( [A-Z]{3})?`)
	original := wmoRegexp.FindString(text)
	if original == "" {
		return WMO{}, errors.New("could not find WMO line")
	}
	segments := strings.Split(original, " ")

	layout := "021504"

	issuedDecoded, err := time.Parse(layout, segments[2])

	year := issuedT.Year()
	month := issuedT.Month()
	if issuedDecoded.Day() < issuedT.Day() {
		month += 1
		if month == 13 {
			month = 1
			year += 1
		}
	}

	issued := time.Date(year, month, issuedDecoded.Day(), issuedDecoded.Hour(), issuedDecoded.Minute(), 0, 0, time.Now().UTC().Location())
	if err != nil {
		return WMO{}, errors.New("could not find WMO issued datetime")
	}

	bbb := ""
	if len(segments) > 3 {
		bbb = segments[3]
	}

	return WMO{
		Original: original,
		Datatype: segments[0],
		WFO:      segments[1],
		Issued:   issued,
		BBB:      bbb,
	}, nil
}
