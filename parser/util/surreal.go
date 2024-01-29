package util

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/TheRangiCrew/NWWS-GO/parser/parsers"
	"github.com/surrealdb/surrealdb.go"
)

var surrealLock = &sync.Mutex{}

var surreal *surrealdb.DB

func Surreal() *surrealdb.DB {
	return surreal
}

func SurrealInit() error {
	surrealLock.Lock()
	defer surrealLock.Unlock()

	if surreal == nil {

		url := os.Getenv("SURREAL_URL")
		username := os.Getenv("SURREAL_USERNAME")
		password := os.Getenv("SURREAL_PASSWORD")
		database := os.Getenv("SURREAL_DATABASE")
		namespace := os.Getenv("SURREAL_NAMESPACE")

		db, err := surrealdb.New(url)
		if err != nil {
			return err
		}

		if _, err = db.Use(namespace, database); err != nil {
			return err
		}

		if _, err = db.Signin(map[string]interface{}{
			"user": username,
			"pass": password,
			"NS":   namespace,
		}); err != nil {
			return err
		}

		surreal = db
	}

	return nil
}

type VTECProduct struct {
	ID           string                  `json:"id"`
	UpdatedAt    time.Time               `json:"updated_at,omitempty"`
	Start        time.Time               `json:"start"`
	End          time.Time               `json:"end"`
	Issued       time.Time               `json:"issued"`
	Expires      time.Time               `json:"expires"`
	EndInitial   time.Time               `json:"end_initial"`
	EventNumber  int                     `json:"event_number"`
	Action       string                  `json:"action"`
	Phenomena    string                  `json:"phenomena"`
	Significance string                  `json:"significance"`
	Polygon      *parsers.PolygonFeature `json:"polygon,omitempty"`
	Title        string                  `json:"title,omitempty"`
	WFO          string                  `json:"wfo"`
	Children     int                     `json:"children,omitempty"`
}

type VTECSegment struct {
	ID           string                  `json:"id,omitempty"`
	Original     string                  `json:"original"`
	Start        time.Time               `json:"start"`   // From VTEC
	End          time.Time               `json:"end"`     // From VTEC
	Issued       time.Time               `json:"issued"`  // From WMO line
	Expires      time.Time               `json:"expires"` // From UGC
	EventNumber  int                     `json:"event_number"`
	Action       string                  `json:"action"`
	Phenomena    string                  `json:"phenomena"`
	Significance string                  `json:"significance"`
	Polygon      *parsers.PolygonFeature `json:"polygon,omitempty"`
	VTEC         *parsers.PVTEC          `json:"vtec"`
	HVETC        *parsers.HVTEC          `json:"hvtec,omitempty"` // TODO: Add HVTEC support
	UGC          *parsers.UGC            `json:"ugc"`
	LatLon       *parsers.LATLON         `json:"latlon,omitempty"`
	TML          *parsers.TML            `json:"tml,omitempty"`
	HazardTags   *parsers.HazardTags     `json:"tags"`
	Emergency    bool                    `json:"emergency"`
	PDS          bool                    `json:"pds"`
	WFO          string                  `json:"wfo"`
}

func PushVTECProduct(p *parsers.VTECProduct) error {
	product := p.Product
	for _, segment := range p.Segments {

		// Create ID
		year := strconv.Itoa(product.Issued.Year())

		vtecID := segment.WFO + segment.Phenomena + segment.Significance + PadZero(strconv.Itoa(segment.EventNumber), 4) + year

		// Get the most recent record if one exists
		query := fmt.Sprintf(`SELECT *, count(->vtec_product_segments) AS children FROM vtec_product:%s`, vtecID)

		parentResult, err := Surreal().Query(query, map[string]string{})
		if err != nil {
			return err
		}

		// NOTE: Surreal returns an array of the result which requires an array to be Unmarshalled. This is referenced later
		record := new([]surrealdb.RawQuery[[]VTECProduct])
		err = surrealdb.Unmarshal(parentResult, &record)
		if err != nil {
			return err
		}

		var parent *VTECProduct
		if len((*record)[0].Result) != 0 {
			parent = &(*record)[0].Result[0]
		}

	}

	return nil
}
