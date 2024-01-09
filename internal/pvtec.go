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
	ID           string    `json:"-"`
	Original     string    `json:"original"`
	Type         string    `json:"type"`
	Action       string    `json:"action"`
	WFO          string    `json:"wfo"`
	Phenomena    string    `json:"phenomena"`
	Significance string    `json:"significance"`
	ETN          int       `json:"etn"`
	Start        time.Time `json:"start"`
	End          time.Time `json:"end"`
}

const TYPES = "OTEX"

func FindPVTEC(text string) int {
	vtecRegex := regexp.MustCompile("([A-Z]).([A-Z]+).([A-Z]+).([A-Z]+).([A-Z]).([0-9]+).([0-9TZ]+)-([0-9TZ]+)")
	result := vtecRegex.FindAllString(text, -1)
	return len(result)
}

func ParsePVTEC(text string, issued time.Time) ([]PVTEC, error) {
	vtecRegex := regexp.MustCompile("([A-Z]).([A-Z]+).([A-Z]+).([A-Z]+).([A-Z]).([0-9]+).([0-9TZ]+)-([0-9TZ]+)")
	instances := vtecRegex.FindAllString(text, -1)

	var vtecs []PVTEC

	for _, original := range instances {

		segments := strings.Split(original, ".")

		// Get VTEC type
		productType := []rune(segments[0])

		if !strings.ContainsRune(TYPES, productType[0]) {
			return vtecs, errors.New("Invalid VTEC product type")
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
			return vtecs, errors.New("Invalid VTEC action")
		}

		// Get WFO
		wfo := segments[2]

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
			return vtecs, errors.New("Invalid VTEC phenomena")
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
			return vtecs, errors.New("Invalid VTEC significance")
		}

		// Get tracking number
		etnString := segments[5]
		etn, err := strconv.Atoi(etnString)

		if err != nil {
			return vtecs, errors.New("Failed to parse VTEC ETN")
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
			t, err := issued.MarshalText()

			if err != nil {
				return vtecs, errors.New("Failed to parse VTEC query issued time\n" + err.Error())
			}

			query := fmt.Sprintf(`SELECT created_at, vtec FROM vtec_segments WHERE vtec.etn = %s AND wfo = "%s" AND vtec.phenomena = "%s" AND vtec.start > "%s" - 24h ORDER BY created_at DESC LIMIT 1`, strconv.Itoa(etn), wfo, phenomena, string(t))

			result, err := Surreal().Query(query, map[string]string{})

			if err != nil {
				return vtecs, err
			}

			// NOTE: Surreal returns an array of the result which requires an array to be Unmarshalled. This is referenced later
			record := new([]surrealdb.RawQuery[[]PVTECQueryResult])
			err = surrealdb.Unmarshal(result, &record)
			if err != nil {
				return vtecs, err
			}

			if len(*record) == 0 || len((*record)[0].Result) == 0 {
				return vtecs, errors.New("no previous VTEC records found. Skipping")
			}

			start = (*record)[0].Result[0].VTEC.Start
		} else {

			s, err := time.Parse(layout, dateSegments[0])

			if err != nil {
				return vtecs, errors.New("Failed to parse VTEC start date")
			}

			start = s
		}

		end, err = time.Parse(layout, dateSegments[1])

		if err != nil {
			return vtecs, errors.New("Failed to parse VTEC end date")
		}

		year := strconv.Itoa(issued.Year())

		id := wfo + phenomena + significance + etnString + year

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
		})
	}

	return vtecs, nil

}
