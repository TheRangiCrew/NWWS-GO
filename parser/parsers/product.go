package parsers

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/TheRangiCrew/NWWS-GO/parser/util"
)

type Segment struct {
	ID   int
	Text string
}

type Product struct {
	ID      string    `json:"id"`
	Group   string    `json:"group"`
	Text    string    `json:"text"`
	WMO     WMO       `json:"wmo"`
	AWIPS   AWIPS     `json:"-"`
	BIL     string    `json:"bil,omitempty"`
	Issued  time.Time `json:"issued"`
	WFO     string    `json:"wfo"`
	Product string    `json:"product"`
}

func (p *Product) HasVTEC() bool {
	return FindPVTEC(p.Text) > 0
}

func (p *Product) VTECProduct() (*VTECProduct, error) {
	return ParseVTECProduct(p)
}

func NewAWIPSProduct(text string) (*Product, error) {

	var err error = nil

	awips := ParseAWIPS(text)
	if awips == nil {
		return nil, err
	}

	issuedRegexp := regexp.MustCompile("[0-9]{3,4} ((AM|PM) [A-Za-z]{3,4}|UTC) ([A-Za-z]{3} ){2}[0-9]{1,2} [0-9]{4}")
	issuedString := issuedRegexp.FindString(text)

	timezones := map[string]*time.Location{
		"GMT":  time.FixedZone("GMT", 0*60*60),
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
		utcRegexp := regexp.MustCompile("UTC")
		utc := utcRegexp.MatchString(issuedString)
		if utc {
			issued, err = time.ParseInLocation("1504 UTC Mon Jan 2 2006", issuedString, timezones["UTC"])
		} else {
			/*
				Since the time package cannot handle the time format that is provided in the NWS text products,
				we have to modify the string to include a clearer seperator between the hour and the minute values
			*/
			tzString := strings.Split(issuedString, " ")[2]
			tz := timezones[tzString]
			if tz == nil {
				return nil, errors.New("Missing timezone " + tzString + " AWIPS: " + awips.Original)
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
			return nil, errors.New("Could not parse issued date line for AWIPS: " + awips.Original)
		}
	} else {
		issued = time.Now().UTC()
	}

	wmo, err := ParseWMO(text, issued)
	if err != nil {
		return nil, err
	}

	bilRegexp := regexp.MustCompile("(?m:^(BULLETIN - |URGENT - |EAS ACTIVATION REQUESTED|IMMEDIATE BROADCAST REQUESTED|FLASH - |REGULAR - |HOLD - |TEST...)(.*))")
	bil := bilRegexp.FindString(text)

	year := util.PadZero(strconv.Itoa(issued.Year()), 2)
	month := util.PadZero(strconv.Itoa(int(issued.Month())), 2)
	day := util.PadZero(strconv.Itoa(issued.Day()), 2)
	hour := util.PadZero(strconv.Itoa(issued.Hour()), 2)
	minute := util.PadZero(strconv.Itoa(issued.Minute()), 2)

	group := awips.WFO + awips.Product + year + month + day + hour + minute

	product := Product{
		Group:   group,
		Text:    text,
		WMO:     wmo,
		AWIPS:   *awips,
		BIL:     bil,
		Issued:  issued,
		WFO:     awips.WFO,
		Product: awips.Product,
	}

	return &product, nil

}
