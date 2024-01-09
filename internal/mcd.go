package internal

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type MCD struct {
	ID               string          `json:"id"`
	Original         string          `json:"original"`
	Number           int             `json:"number"`
	Issued           time.Time       `json:"issued"`
	Expires          time.Time       `json:"expires"`
	Polygon          *PolygonFeature `json:"polygon"`
	WatchProbability int             `json:"watch_probability"`
}

func ParseMCD(product Product) error {

	idRegexp := regexp.MustCompile(`(Mesoscale Discussion [0-9]{1,4})`)
	idString := idRegexp.FindString(product.Text)
	if idString == "" {
		return errors.New("failed to find a MCD ID string")
	}

	numberSplit := strings.Split(idString, " ")
	if len(numberSplit) != 3 {
		return errors.New("failed to find MCD ID string (wrong length)")
	}

	numberString := numberSplit[2]
	number, err := strconv.Atoi(numberString)
	if err != nil {
		return err
	}

	dateLineRegexp := regexp.MustCompile(`([0-9]{6}Z - [0-9]{6}Z)`)
	dateLine := dateLineRegexp.FindString(product.Text)
	if dateLine == "" {
		return errors.New("failed to find date line in MCD")
	}

	dateLineSplit := strings.Split(dateLine, " - ")
	if len(dateLineSplit) != 2 {
		return errors.New("date line does not have two dates in MCD")
	}

	layout := "021504Z"

	startT, err := time.Parse(layout, dateLineSplit[0])
	if err != nil {
		return err
	}

	endT, err := time.Parse(layout, dateLineSplit[1])
	if err != nil {
		return err
	}

	year := product.Issued.Year()
	month := product.Issued.Month()
	if endT.Day() < startT.Day() {
		month += 1
		if month == 13 {
			month = 1
			year += 1
		}
	}

	start := time.Date(product.Issued.Year(), product.Issued.Month(), startT.Day(), startT.Hour(), startT.Minute(), 0, 0, time.Now().UTC().Location())
	end := time.Date(year, month, endT.Day(), endT.Hour(), endT.Minute(), 0, 0, time.Now().UTC().Location())

	latlon, err := ParseLatLon(product.Text)
	if err != nil {
		return err
	}

	polygon := latlon.Polygon

	watch := 0

	watchLineRegexp := regexp.MustCompile("Probability of Watch Issuance...[0-9]+ percent")
	watchLine := watchLineRegexp.FindString(product.Text)
	if watchLine != "" {
		percentRegexp := regexp.MustCompile("[0-9]+")
		percentString := percentRegexp.FindString(watchLine)
		if percentString != "" {
			percent, err := strconv.Atoi(percentString)
			if err != nil {
				watch = 0
			} else {
				watch = percent
			}
		}
	}

	id := "MCD" + padLeft(strconv.Itoa(number), 4) + strconv.Itoa(start.Year())

	mcd := MCD{
		ID:               id,
		Original:         product.Text,
		Number:           number,
		Issued:           start,
		Expires:          end,
		Polygon:          polygon,
		WatchProbability: watch,
	}

	concerningRegexp := regexp.MustCompile(`(Concerning\.\.\.)(Tornado|Severe Thunderstorm) (Watch) [0-9]+`)
	concerningLine := concerningRegexp.FindString(product.Text)
	if concerningLine != "" {
		phenomenaRegexp := regexp.MustCompile("Tornado Watch")
		phenomenaString := phenomenaRegexp.FindString(concerningLine)
		phenomena := "TO"
		if phenomenaString == "" {
			phenomena = "SV"
		}

		watchNumberRegexp := regexp.MustCompile(`[0-9]+`)
		watchNumber := watchNumberRegexp.FindString(concerningLine)
		if watchNumber == "" {
			return errors.New("Found concerning watch in MCD but couldn't parse number MCD " + numberString)
		}
		watchID := "severe_watches:" + phenomena + "A" + padLeft(watchNumber, 4) + strconv.Itoa(start.Year())

		// RELATE the watch product to the segment
		_, err = Surreal().Query(fmt.Sprintf("RELATE mcd:%s->mcd_watch->%s", mcd.ID, watchID), map[string]string{})
		if err != nil {
			return err
		}
	}

	_, err = Surreal().Create("mcd", mcd)
	if err != nil {
		return err
	}

	// Push the WOU product to the database
	_, err = Surreal().Create("text_products", product)
	if err != nil {
		return err
	}

	// RELATE the text product to the mcd
	_, err = Surreal().Query(fmt.Sprintf("RELATE text_products:%s->mcd_text_products->mcd:%s", product.ID, mcd.ID), map[string]string{})
	if err != nil {
		return err
	}

	return nil
}
