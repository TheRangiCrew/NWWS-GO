package internal

import (
	"errors"
	"regexp"
)

type AWIPS struct {
	Original string `json:"original"`
	Product  string `json:"product"`
	WFO      string `json:"wfo"`
}

func ParseAWIPS(text string) (AWIPS, error) {
	awipsRegex := regexp.MustCompile("(?m:^[A-Z0-9 ]{6}\n)")
	original := awipsRegex.FindString(text)
	if original == "" {
		return AWIPS{}, errors.New("could not find AWIPS ID")
	}
	original = original[:len(original)-1]

	product := original[0:3]
	wfo := original[3:6]

	return AWIPS{
		Original: original,
		Product:  product,
		WFO:      wfo,
	}, nil
}
