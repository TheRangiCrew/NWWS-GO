package internal

import (
	"errors"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Segment struct {
	ID   int
	Text string
}

type Product struct {
	ID     string    `json:"id"`
	Text   string    `json:"text"`
	WMO    WMO       `json:"wmo"`
	AWIPS  AWIPS     `json:"awips"`
	BIL    string    `json:"bil,omitempty"`
	Issued time.Time `json:"issued"`
}

func Processor(text string, errCh chan error) {

	var err error

	awips, err := ParseAWIPS(text)

	if err != nil {
		errCh <- err
		return
	}

	issuedRegexp := regexp.MustCompile("(?m:^[0-9]{3,4} ((AM|PM) [A-Z]{3,4}|UTC) ([A-Za-z]{3} ){2}[0-9]{1,2} [0-9]{4})")
	issuedString := issuedRegexp.FindString(text)

	var issued time.Time

	if issuedString != "" {
		utcRegexp := regexp.MustCompile("UTC")
		utc := utcRegexp.MatchString(issuedString)

		if utc {
			issued, err = time.Parse("1504 UTC Mon Jan 2 2006", issuedString)
		} else {
			/*
				Since the time package cannot handle the time format that is provided in the NWS text products,
				we have to modify the string to include a clearer seperator between the hour and the minute values
			*/
			split := strings.Split(issuedString, " ")
			t := split[0]
			hours := t[:len(t)-2]
			minutes := t[len(t)-2:]
			split[0] = hours + ":" + minutes
			new := strings.Join(split, " ")
			issued, err = time.Parse("3:04 PM MST Mon Jan 2 2006", new)
		}

		if err != nil {
			errCh <- errors.New("Could not parse issued date line for AWIPS: " + awips.Original)
			return
		}
	} else {
		log.Println("Cannot find issued date line. Defaulting to now...")
		issued = time.Now()
	}

	wmo, err := ParseWMO(text, issued)

	if err != nil {
		errCh <- err
		return
	}

	bilRegexp := regexp.MustCompile("(?m:^(BULLETIN - |URGENT - |EAS ACTIVATION REQUESTED|IMMEDIATE BROADCAST REQUESTED|FLASH - |REGULAR - |HOLD - |TEST...)(.*))")
	bil := bilRegexp.FindString(text)

	year := padLeft(strconv.Itoa(issued.Year()), 2)
	month := padLeft(strconv.Itoa(int(issued.Month())), 2)
	day := padLeft(strconv.Itoa(issued.Day()), 2)
	hour := padLeft(strconv.Itoa(issued.Hour()), 2)
	minute := padLeft(strconv.Itoa(issued.Minute()), 2)
	seconds := padLeft(strconv.Itoa(time.Now().Second()), 2)

	id := wmo.WFO + awips.Product + year + month + day + hour + minute + seconds

	println(id)

	product := Product{
		ID:     id,
		Text:   text,
		WMO:    wmo,
		AWIPS:  awips,
		BIL:    bil,
		Issued: issued,
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

	segments := strings.Split(text, "$$")

	if len(segments) > 1 {
		segments = segments[:len(segments)-1]
	}

	for i, s := range segments {
		segment := Segment{
			ID:   i,
			Text: s,
		}
		vtecs := FindPVTEC(s)
		switch {
		case vtecs > 0:
			err := ParseVTECProduct(segment, product)
			if err != nil {
				errCh <- err
			}
		default:
			close(errCh)
			return
		}
	}

	close(errCh)

}
