package parsers

import (
	"regexp"
)

type AWIPS struct {
	Original string `json:"original"`
	Product  string `json:"product"`
	WFO      string `json:"wfo"`
}

func ParseAWIPS(text string) *AWIPS {
	awipsRegex := regexp.MustCompile("(?m:^[A-Z0-9]{4,6}[ ]*\n)")
	original := awipsRegex.FindString(text)
	if original == "" {
		return nil
	}
	original = original[:len(original)-1]

	product := original[0:3]
	wfo := original[3:]

	return &AWIPS{
		Original: original,
		Product:  product,
		WFO:      wfo,
	}
}
