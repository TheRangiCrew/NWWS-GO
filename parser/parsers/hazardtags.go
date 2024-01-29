package parsers

import (
	"regexp"
	"strings"
)

type HazardTags struct {
	Tornado            string `json:"tornado,omitempty"`
	TornadoDamage      string `json:"tornadoDamage,omitempty"`
	ThunderstormDamage string `json:"thunderstormDamage,omitempty"`
	HailThreat         string `json:"hailThreat,omitempty"`
	Hail               string `json:"hail,omitempty"`
	Wind               string `json:"wind,omitempty"`
	WindThreat         string `json:"windThreat,omitempty"`
	Waterspout         string `json:"waterspout,omitempty"`
}

func findAndVerify(regex regexp.Regexp, values []string, text string) string {
	found := regex.FindString(text)

	if found == "" {
		return found
	}

	value := strings.Split(found, "...")[1]

	result := false
	for _, a := range values {
		if a == value {
			result = true
		}
	}

	if !result {
		println("Unusual " + regex.String() + " tag found...")
	}

	return value
}

func ParseHazardTags(text string) HazardTags {
	tornadoRegexp := regexp.MustCompile(`TORNADO\.\.\.([A-Z ]+)`)
	tornadoPossibles := [...]string{"POSSIBLE", "RADAR INDICATED", "OBSERVED"}
	tornado := findAndVerify(*tornadoRegexp, tornadoPossibles[:], text)

	tornadoDamageRegexp := regexp.MustCompile(`TORNADO DAMAGE THREAT\.\.\.([A-Z ]+)`)
	tornadoDamagePossibles := [2]string{"CONSIDERABLE", "CATASTROPHIC"}
	tornadoDamage := findAndVerify(*tornadoDamageRegexp, tornadoDamagePossibles[:], text)

	thunderstormDamageRegexp := regexp.MustCompile(`THUNDERSTORM DAMAGE THREAT\.\.\.([A-Z ]+)`)
	thunderstormDamagePossibles := [2]string{"CONSIDERABLE", "DESTRUCTIVE"}
	thunderstormDamage := findAndVerify(*thunderstormDamageRegexp, thunderstormDamagePossibles[:], text)

	hailThreatRegexp := regexp.MustCompile(`HAIL THREAT\.\.\.([A-Z ]+)`)
	hailThreatPossibles := [...]string{"RADAR INDICATED", "OBSERVED"}
	hailThreat := findAndVerify(*hailThreatRegexp, hailThreatPossibles[:], text)

	hailRegexp := regexp.MustCompile(`.*(HAIL|MAX HAIL SIZE)\.\.\.[><\.0-9]+\s?IN`)
	hailString := hailRegexp.FindString(text)
	if hailString != "" {
		hailString = strings.Split(hailString, "...")[1]
	}
	hail := hailString

	windThreatRegexp := regexp.MustCompile(`WIND THREAT\.\.\.([A-Z ]+)`)
	windThreatPossibles := [...]string{"RADAR INDICATED", "OBSERVED"}
	windThreat := findAndVerify(*windThreatRegexp, windThreatPossibles[:], text)

	windRegexp := regexp.MustCompile(`.*(WIND|MAX WIND GUST)\.\.\.[><\.0-9]+\s?(MPH|KTS)`)
	windString := windRegexp.FindString(text)
	if windString != "" {
		windString = strings.Split(windString, "...")[1]
	}
	wind := windString

	waterspoutRegexp := regexp.MustCompile(`.*(WATERSPOUT)\.\.\.(.)+\n`)
	waterspoutString := waterspoutRegexp.FindString(text)
	if waterspoutString != "" {
		waterspoutString = strings.Split(waterspoutString, "...")[1]
	}
	waterspout := waterspoutString

	return HazardTags{
		Tornado:            tornado,
		TornadoDamage:      tornadoDamage,
		ThunderstormDamage: thunderstormDamage,
		HailThreat:         hailThreat,
		Hail:               hail,
		WindThreat:         windThreat,
		Wind:               wind,
		Waterspout:         waterspout,
	}
}
