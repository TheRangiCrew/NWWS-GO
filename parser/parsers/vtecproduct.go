package parsers

import (
	"regexp"
	"strings"
	"time"
)

type VTECProduct struct {
	Product  *Product
	Segments []VTECSegment
}

type VTECSegment struct {
	Original     string          `json:"original"`
	Start        *time.Time      `json:"start"`   // From VTEC
	End          *time.Time      `json:"end"`     // From VTEC
	Issued       time.Time       `json:"issued"`  // From WMO line
	Expires      time.Time       `json:"expires"` // From UGC
	EventNumber  int             `json:"event_number"`
	Action       string          `json:"action"`
	Phenomena    string          `json:"phenomena"`
	Significance string          `json:"significance"`
	Polygon      *PolygonFeature `json:"polygon,omitempty"`
	VTEC         PVTEC           `json:"vtec"`
	HVETC        *HVTEC          `json:"hvtec,omitempty"` // TODO: Add HVTEC support
	UGC          UGC             `json:"ugc"`
	LatLon       *LATLON         `json:"latlon,omitempty"`
	TML          *TML            `json:"tml,omitempty"`
	HazardTags   HazardTags      `json:"tags"`
	Emergency    bool            `json:"emergency"`
	PDS          bool            `json:"pds"`
	WFO          string          `json:"wfo"`
}

func parseVTECProductSegment(segment string, product *Product) ([]VTECSegment, error) {

	var err error = nil

	emergencyRegexp := regexp.MustCompile(`(TORNADO|FLASH\s+FLOOD)\s+EMERGENCY`)
	pdsRegexp := regexp.MustCompile(`THIS\s+IS\s+A\s+PARTICULARLY\s+DANGEROUS\s+SITUATION`)

	ugc, err := ParseUGC(segment, product.Issued)
	if err != nil {
		return nil, err
	}
	if ugc == nil {
		return nil, nil
	}

	vtecs, err := ParsePVTEC(segment, product.Issued.UTC(), *ugc)
	if err != nil {
		return nil, err
	}

	hvtec := ParseHVTEC(segment, product.Issued)

	latlon, err := ParseLatLon(segment)
	if err != nil {
		return nil, err
	}

	var polygon *PolygonFeature
	if latlon != nil {
		polygon = latlon.Polygon
	} else {
		polygon = nil
	}

	tml, err := ParseTML(segment, product.Issued)
	if err != nil {
		return nil, err
	}

	emergency := emergencyRegexp.MatchString(segment)
	pds := pdsRegexp.MatchString(segment)

	hazardTags := ParseHazardTags(segment)

	segments := []VTECSegment{}
	for _, vtec := range vtecs {
		segments = append(segments, VTECSegment{
			Original:     segment,
			Start:        vtec.Start,     // From VTEC
			End:          vtec.End,       // From VTEC
			Issued:       product.Issued, // From WMO line
			Expires:      ugc.Expires,    // From UGC
			EventNumber:  vtec.ETN,
			Action:       vtec.Action,
			Phenomena:    vtec.Phenomena,
			Significance: vtec.Significance,
			Polygon:      polygon,
			VTEC:         vtec,
			HVETC:        hvtec,
			UGC:          *ugc,
			LatLon:       latlon,
			TML:          tml,
			HazardTags:   hazardTags,
			Emergency:    emergency,
			PDS:          pds,
			WFO:          vtec.WFO,
		})
	}

	return segments, nil

}

func ParseVTECProduct(product *Product) (*VTECProduct, error) {
	vtecProduct := VTECProduct{
		Product:  product,
		Segments: []VTECSegment{},
	}

	segments := strings.Split(product.Text, "$$")

	if len(segments) > 1 {
		segments = segments[:len(segments)-1]
	}

	for _, s := range segments {
		segment, err := parseVTECProductSegment(s, product)
		if err != nil {
			return nil, err
		}

		vtecProduct.Segments = append(vtecProduct.Segments, segment...)
	}

	return &vtecProduct, nil
}
