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

type PVTECQueryResult struct {
	Created_At time.Time `json:"created_at"`
	VTEC       PVTEC     `json:"vtec"`
}

type PVTEC struct {
	ID           string       `json:"-"`
	Original     string       `json:"original"`
	Type         string       `json:"type"`
	Action       string       `json:"action"`
	WFO          string       `json:"wfo"`
	Phenomena    string       `json:"phenomena"`
	Significance string       `json:"significance"`
	ETN          int          `json:"etn"`
	Start        time.Time    `json:"start"`
	End          time.Time    `json:"end"`
	Parent       *VTECProduct `json:"-"`
}

const TYPES = "OTEX"

func FindPVTEC(text string) int {
	vtecRegex := regexp.MustCompile("([A-Z]).([A-Z]+).([A-Z]+).([A-Z]+).([A-Z]).([0-9]+).([0-9TZ]+)-([0-9TZ]+)")
	result := vtecRegex.FindAllString(text, -1)
	return len(result)
}

func ParsePVTEC(text string, issued time.Time, ugc UGC) ([]PVTEC, error) {
	vtecRegex := regexp.MustCompile("([A-Z]).([A-Z]+).([A-Z]+).([A-Z]+).([A-Z]).([0-9]+).([0-9TZ]+)-([0-9TZ]+)")
	instances := vtecRegex.FindAllString(text, -1)

	var vtecs []PVTEC

	for _, original := range instances {

		segments := strings.Split(original, ".")

		// Get VTEC type
		productType := []rune(segments[0])

		if !strings.ContainsRune(TYPES, productType[0]) {
			return vtecs, errors.New("invalid VTEC product type")
		}

		// Get VTEC action
		action := segments[1]
		allowedActions := [10]string{"NEW", "CON", "EXA", "EXT", "EXB", "UPG", "CAN", "EXP", "COR", "ROU"}

		result := false
		for _, a := range allowedActions {
			if a == action {
				result = true
			}
		}

		if !result {
			return vtecs, errors.New("invalid VTEC action")
		}

		// Get WFO
		wfo := segments[2]
		wfo = wfo[1:]

		// Get phenomena
		phenomena := segments[3]
		allowedPhenomena := [...]string{
			"AF",
			"AS",
			"BH",
			"BW",
			"BZ",
			"CF",
			"DF",
			"DS",
			"EC",
			"EH",
			"EW",
			"FA",
			"FF",
			"FG",
			"FL",
			"FR",
			"FW",
			"FZ",
			"GL",
			"HF",
			"HT",
			"HU",
			"HW",
			"HY",
			"HZ",
			"IS",
			"LE",
			"LO",
			"LS",
			"LW",
			"MA",
			"MF",
			"MH",
			"MS",
			"RB",
			"RP",
			"SC",
			"SE",
			"SI",
			"SM",
			"SQ",
			"SR",
			"SS",
			"SU",
			"SV",
			"SW",
			"TO",
			"TR",
			"TS",
			"TY",
			"UP",
			"WC",
			"WI",
			"WS",
			"WW",
			"ZF",
			"ZR",
		}

		result = false
		for _, a := range allowedPhenomena {
			if a == phenomena {
				result = true
			}
		}

		if !result {
			return vtecs, errors.New("invalid VTEC phenomena")
		}

		// Get significance
		significance := segments[4]
		allowedSignificance := [7]string{"W", "A", "Y", "S", "F", "O", "N"}

		result = false
		for _, a := range allowedSignificance {
			if a == significance {
				result = true
			}
		}

		if !result {
			return vtecs, errors.New("invalid VTEC significance")
		}

		// Get tracking number
		etnString := segments[5]
		etn, err := strconv.Atoi(etnString)

		if err != nil {
			return vtecs, errors.New("failed to parse VTEC ETN")
		}

		// Create ID
		year := strconv.Itoa(issued.Year())

		id := wfo + phenomena + significance + etnString + year

		// Get the most recent record if one exists
		query := fmt.Sprintf(`SELECT *, count(->vtec_product_segments) AS children FROM vtec_product:%s`, id)

		parentResult, err := Surreal().Query(query, map[string]string{})

		if err != nil {
			return vtecs, err
		}

		// NOTE: Surreal returns an array of the result which requires an array to be Unmarshalled. This is referenced later
		record := new([]surrealdb.RawQuery[[]VTECProduct])
		err = surrealdb.Unmarshal(parentResult, &record)
		if err != nil {
			return vtecs, err
		}

		var parent *VTECProduct
		if len((*record)[0].Result) != 0 {
			parent = &(*record)[0].Result[0]
		}

		// Get time
		datetimeString := segments[6]
		dateSegments := strings.Split(datetimeString, "-")

		layout := "060102T1504Z"

		var start time.Time
		var end time.Time

		zeroRegexp := regexp.MustCompile("000000T0000Z")

		// Sort out start datetime

		if zeroRegexp.MatchString(dateSegments[0]) {
			if err != nil {
				return vtecs, errors.New("Failed to find UTC\n" + err.Error())
			}

			if err != nil {
				return vtecs, errors.New("Failed to parse VTEC query issued time\n" + err.Error())
			}

			if parent == nil {
				start = issued
			} else {
				if parent.Start.Compare(start) > 0 {
					start = issued
				} else {
					start = parent.Start
				}
			}

		} else {
			s, err := time.Parse(layout, dateSegments[0])
			if err != nil {
				return vtecs, errors.New("failed to parse VTEC start date")
			}

			start = s
		}

		if zeroRegexp.MatchString(dateSegments[1]) {
			end = ugc.Expires
		} else {

			e, err := time.Parse(layout, dateSegments[1])

			if err != nil {
				return vtecs, errors.New("failed to parse VTEC end date")
			}

			end = e
		}

		vtecs = append(vtecs, PVTEC{
			ID:           id,
			Original:     original,
			Type:         string(productType),
			Action:       action,
			WFO:          wfo,
			Phenomena:    phenomena,
			Significance: significance,
			ETN:          etn,
			Start:        start,
			End:          end,
			Parent:       parent,
		})
	}

	return vtecs, nil

}
