package parsers

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/TheRangiCrew/NWWS-GO/parser/util"
)

type UGC struct {
	Original string    `json:"original"`
	States   []State   `json:"states"`
	Expires  time.Time `json:"expires"`
}

type State struct {
	Name  string   `json:"name"`
	Type  string   `json:"type"`
	Zones []string `json:"zones"`
}

func ParseUGC(text string, issued time.Time) (*UGC, error) {
	ugcStartRegex := regexp.MustCompile("(?m:^[A-Z]{2}(C|Z)[A-Z0-9]{3}(-|>))")
	startIndex := ugcStartRegex.FindStringIndex(text)
	if startIndex == nil {
		return nil, errors.New("Could not find UGC string!")
	}
	start := text[startIndex[0]:]

	ugcEndRegex := regexp.MustCompile("([0-9]{6}-)")
	endIndex := ugcEndRegex.FindStringIndex(start)
	if endIndex == nil {
		return nil, errors.New("Could not find UGC string!")
	}
	// Subtract 1 to remove the - at the end of the UGC
	original := start[:endIndex[1]-1]

	original = strings.ReplaceAll(original, "\n", "")

	segments := strings.Split(original, "-")

	parsedTime, err := time.Parse("021504", segments[len(segments)-1])

	if err != nil {
		return nil, errors.New("Could not parse UGC datetime!")
	}

	year := issued.Year()
	month := issued.Month()
	if parsedTime.Day() < issued.Day() {
		month += 1
		if month == 13 {
			month = 1
			year += 1
		}
	}

	expires := time.Date(year, month, parsedTime.Day(), parsedTime.Hour(), parsedTime.Minute(), 0, 0, time.Now().UTC().Location())

	segments = segments[:len(segments)-1]

	states := []State{}
	currentState := -1
	alphabetRegexp := regexp.MustCompile("[A-Z]")
	bracketRegexp := regexp.MustCompile(">")

	for _, s := range segments {

		if alphabetRegexp.MatchString(s) {
			currentState++
			states = append(states, State{
				Name:  s[0:2],
				Type:  s[2:3],
				Zones: []string{},
			})
			s = s[3:]
		}

		if bracketRegexp.MatchString(s) {
			start, err := strconv.Atoi(s[:3])
			if err != nil {
				return nil, errors.New("Could not parse UGC int")
			}

			end, err := strconv.Atoi(s[4:])
			if err != nil {
				return nil, errors.New("Could not parse UGC int")
			}

			for i := start; i <= end; i++ {
				states[currentState].Zones = append(states[currentState].Zones, util.PadZero(strconv.Itoa(i), 3))
			}
		} else {
			states[currentState].Zones = append(states[currentState].Zones, s)
		}
	}

	return &UGC{
		Original: original,
		States:   states,
		Expires:  expires,
	}, nil
}
