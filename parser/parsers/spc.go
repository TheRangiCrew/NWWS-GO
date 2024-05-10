package parsers

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

type PTSCategories struct {
	Category string               `json:"category"`
	Features *MultiPolygonFeature `json:"features"`
}

type PTSSegments struct {
	Type       string           `json:"type"`
	Categories *[]PTSCategories `json:"categories"`
}

type PTS struct {
	ID       string         `json:"id"`
	Original string         `json:"original"`
	Day      int            `json:"dat"`
	Issued   time.Time      `json:"issued"`
	Expires  time.Time      `json:"expires"`
	Segments *[]PTSSegments `json:"segments"`
}

func ParsePTSProduct(product *Product) (*PTS, error) {

	switch product.AWIPS.WFO {
	case "DY1":
		parseDY1(product.Text)
		return nil, nil
		// case "DY2":

		// case "DY3":

		// case "D48":
	}

	return nil, nil

}

func parseSegment(segment string, name string) (*[]PTSSegments, error) {
	categoryRegexp := regexp.MustCompile(`(0\.[0-9]{2}|[A-Z]{3,4})( {3,7}[0-9 ]+\n)+`)
	categorySegments := categoryRegexp.FindAllString(segment, -1)

	categories := []*PTSCategories{}
	for _, s := range categorySegments {
		s = strings.TrimSpace(s)

		spaceRegexp := regexp.MustCompile(`[ \n]{2,}`)
		pointTextArr := strings.Split(spaceRegexp.ReplaceAllString(s, " "), " ")

		title := pointTextArr[0]
		category := &PTSCategories{
			Category: title,
			Features: &MultiPolygonFeature{
				Type:        "MultiPolygon",
				Coordinates: [][][][2]float64{{{}}},
			},
		}

		found := false
		for _, i := range categories {
			if i.Category == title && !found {
				found = true
			}
		}

		if !found {
			categories = append(categories, category)
		}

		fmt.Println(found)

		items := pointTextArr[1:]

		points := [][2]float64{}
		fmt.Println(category.Features.Coordinates[0][0])
		for _, i := range items {
			point, err := ParsePoint([]string{i})
			if err != nil {
				return nil, err
			}
			asd := *point
			points = append(points, asd)
		}

		// category.Features.Coordinates[0][0] = append(category.Features.Coordinates[0][0], points)
		fmt.Println(category.Features.Coordinates[0][0])

	}

	return nil, nil
}

func parseDY1(text string) {

	segmentEndRegexp := regexp.MustCompile("&&")

	tornadoRegexp := regexp.MustCompile(`\.\.\. TORNADO \.\.\.`)
	startIndex := tornadoRegexp.FindStringIndex(text)[1]

	endIndex := segmentEndRegexp.FindStringIndex(text)[0]
	segment := strings.TrimSpace(text[startIndex:endIndex])

	parseSegment(segment, "TORNADO")

}
