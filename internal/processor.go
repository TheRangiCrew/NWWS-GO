package internal

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/surrealdb/surrealdb.go"
)

type Segment struct {
	ID   int
	Text string
}

type Product struct {
	ID      string    `json:"id"`
	GroupID string    `json:"group"`
	Text    string    `json:"text"`
	WMO     WMO       `json:"wmo"`
	AWIPS   AWIPS     `json:"awips"`
	BIL     string    `json:"bil,omitempty"`
	Issued  time.Time `json:"issued"`
}

func Processor(text string, errCh chan error) {

	var err error

	awips := ParseAWIPS(text)
	if err != nil {
		errCh <- err
		close(errCh)
		return
	}
	if awips == nil {
		close(errCh)
		return
	}

	issuedRegexp := regexp.MustCompile("[0-9]{3,4} ((AM|PM) [A-Za-z]{3,4}|UTC) ([A-Za-z]{3} ){2}[0-9]{1,2} [0-9]{4}")
	issuedString := issuedRegexp.FindString(text)

	timezones := map[string]*time.Location{
		"UTC":  time.FixedZone("UTC", 0*60*60),
		"AST":  time.FixedZone("AST", -4*60*60),
		"EST":  time.FixedZone("EST", -5*60*60),
		"EDT":  time.FixedZone("EDT", -5*60*60),
		"CST":  time.FixedZone("CST", -6*60*60),
		"CDT":  time.FixedZone("CDT", -5*60*60),
		"MST":  time.FixedZone("MST", -7*60*60),
		"MDT":  time.FixedZone("MDT", -6*60*60),
		"PST":  time.FixedZone("PST", -8*60*60),
		"PDT":  time.FixedZone("PDT", -7*60*60),
		"AKST": time.FixedZone("AKST", -9*60*60),
		"AKDT": time.FixedZone("AKDT", -8*60*60),
		"HST":  time.FixedZone("HST", -10*60*60),
		"SST":  time.FixedZone("SST", -11*60*60),
		"ChST": time.FixedZone("CHST", 10*60*60),
		"CHST": time.FixedZone("CHST", 10*60*60),
	}

	var issued time.Time

	if issuedString != "" {
		tzString := strings.Split(issuedString, " ")[2]
		if tzString == "UTC" {
			issued, err = time.ParseInLocation("1504 UTC Mon Jan 2 2006", issuedString, timezones[tzString])
		} else {
			/*
				Since the time package cannot handle the time format that is provided in the NWS text products,
				we have to modify the string to include a clearer seperator between the hour and the minute values
			*/
			tz := timezones[tzString]
			if tz == nil {
				errCh <- errors.New("Missing timezone " + tzString + " AWIPS: " + awips.Original)
				close(errCh)
				return
			}
			split := strings.Split(issuedString, " ")
			t := split[0]
			hours := t[:len(t)-2]
			minutes := t[len(t)-2:]
			split[0] = hours + ":" + minutes
			new := strings.Join(split, " ")
			new = strings.Replace(new, tzString+" ", "", -1)
			issued, err = time.ParseInLocation("3:04 PM Mon Jan 2 2006", new, tz)
			issued = issued.UTC()
		}

		if err != nil {
			errCh <- errors.New("Could not parse issued date line for AWIPS: " + awips.Original)
			close(errCh)
			return
		}
	} else {
		issued = time.Now().UTC()
	}

	wmo, err := ParseWMO(text, issued)
	if err != nil {
		errCh <- err
		close(errCh)
		return
	}

	bilRegexp := regexp.MustCompile("(?m:^(BULLETIN - |URGENT - |EAS ACTIVATION REQUESTED|IMMEDIATE BROADCAST REQUESTED|FLASH - |REGULAR - |HOLD - |TEST...)(.*))")
	bil := bilRegexp.FindString(text)

	year := padLeft(strconv.Itoa(issued.Year()), 2)
	month := padLeft(strconv.Itoa(int(issued.Month())), 2)
	day := padLeft(strconv.Itoa(issued.Day()), 2)
	hour := padLeft(strconv.Itoa(issued.Hour()), 2)
	minute := padLeft(strconv.Itoa(issued.Minute()), 2)

	group := wmo.WFO + awips.Product + year + month + day + hour + minute

	txt, err := Surreal().Query(fmt.Sprintf("SELECT group FROM text_products WHERE group == '%s'", group), map[string]interface{}{})
	if err != nil {
		errCh <- err
	}
	record := new([]surrealdb.RawQuery[[]struct {
		Group string `json:"group"`
	}])
	err = surrealdb.Unmarshal(txt, &record)
	if err != nil {
		errCh <- err
	}

	sequence := len((*record)[0].Result)

	id := group + padLeft(strconv.Itoa(sequence), 2)

	println(id)

	product := Product{
		ID:      id,
		GroupID: group,
		Text:    text,
		WMO:     wmo,
		AWIPS:   *awips,
		BIL:     bil,
		Issued:  issued,
	}

	// Send products that need special treatment on their way
	// Severe Watches
	if product.AWIPS.Product == "WOU" {
		err := ParseWOU(product)
		if err != nil {
			errCh <- err
		}
		close(errCh)
		return
	}
	if product.AWIPS.Product == "WWP" {
		err := ParseWWP(product)
		if err != nil {
			errCh <- err
		}
		close(errCh)
		return
	}
	if product.AWIPS.Product == "SEL" {
		err := ParseSEL(product)
		if err != nil {
			errCh <- err
		}
		close(errCh)
		return
	}
	if product.AWIPS.Product == "SWO" {
		if product.AWIPS.WFO == "MCD" {
			err := ParseMCD(product)
			if err != nil {
				errCh <- err
			}
			close(errCh)
			return
		}
	}

	vtecs := FindPVTEC(product.Text)
	switch {
	case vtecs > 0:
		err := ParseVTECProduct(product)
		if err != nil {
			errCh <- err
		}
	default:
	}

	// Push the text product to the database
	_, err = Surreal().Create("text_products", product)
	if err != nil {
		errCh <- err
	}
	close(errCh)

}
