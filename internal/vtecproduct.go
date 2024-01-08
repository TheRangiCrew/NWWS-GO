package internal

import (
	"fmt"
	"regexp"
	"time"

	"github.com/surrealdb/surrealdb.go"
)

type VTECSegment struct {
	Created_at string          `json:"created_at,omitempty"`
	ID         string          `json:"id,omitempty"`
	VTECUID    string          `json:"vtec_uid"`
	Original   string          `json:"original"`
	Issued     time.Time       `json:"issued"`  // From WMO line
	Start      time.Time       `json:"start"`   // From VTEC
	End        time.Time       `json:"end"`     // From VTEC
	Expires    time.Time       `json:"expires"` // From UGC
	Polygon    *PolygonFeature `json:"polygon,omitempty"`
	WFO        string          `json:"wfo"`
	VTEC       PVTEC           `json:"vtec"`
	HVETC      *HVTEC          `json:"hvtec,omitempty"` // TODO: Add HVTEC support
	UGC        UGC             `json:"ugc"`
	LatLon     *LATLON         `json:"latlon,omitempty"`
	TML        *TML            `json:"tml,omitempty"`
	HazardTags HazardTags      `json:"tags"`
	Emergency  bool            `json:"emergency"`
	PDS        bool            `json:"pds"`
}

func ParseVTECProduct(segment string, product Product) error {

	emergencyRegexp := regexp.MustCompile(`(TORNADO|FLASH\s+FLOOD)\s+EMERGENCY`)
	pdsRegexp := regexp.MustCompile(`THIS\s+IS\s+A\s+PARTICULARLY\s+DANGEROUS\s+SITUATION`)

	ugc, err := ParseUGC(segment, product.Issued)
	if err != nil {
		return err
	}

	vtecs, err := ParsePVTEC(segment, product.Issued)
	if err != nil {
		return err
	}

	hvtec := ParseHVTEC(segment, product.Issued)

	latlon, err := ParseLatLon(segment)
	if err != nil {
		return err
	}

	var polygon *PolygonFeature
	if latlon != nil {
		polygon = latlon.Polygon
	} else {
		polygon = nil
	}

	tml, err := ParseTML(segment, product.Issued)
	if err != nil {
		return err
	}

	emergency := emergencyRegexp.MatchString(segment)
	pds := pdsRegexp.MatchString(segment)

	hazardTags := ParseHazardTags(segment)

	for _, vtec := range vtecs {

		final := VTECSegment{
			VTECUID:    vtec.ID,
			Original:   segment,
			Issued:     product.WMO.Issued, // From WMO line
			Start:      vtec.Start,         // From VTEC
			End:        vtec.End,           // From VTEC
			Expires:    ugc.Expires,        // From UGC
			Polygon:    polygon,
			WFO:        product.WMO.WFO,
			VTEC:       vtec,
			HVETC:      hvtec,
			UGC:        ugc,
			LatLon:     latlon,
			TML:        tml,
			HazardTags: hazardTags,
			Emergency:  emergency,
			PDS:        pds,
		}

		// Push the text product to the database
		_, err := Surreal().Create("text_products", product)
		if err != nil {
			return err
		}

		result, err := Surreal().Create("vtec_segments", final)
		if err != nil {
			return err
		}

		// NOTE: Surreal returns an array of the result which requires an array to be Unmarshalled. This is referenced later
		record := new([]VTECSegment)
		err = surrealdb.Unmarshal(result, &record)
		if err != nil {
			return err
		}

		// RELATE the text product to the segment
		_, err = Surreal().Query(fmt.Sprintf("RELATE text_products:%s->vtec_text_products->%s", product.ID, (*record)[0].ID), map[string]string{})
		if err != nil {
			return err
		}

		for _, s := range final.UGC.States {
			var t string
			if s.Type == "Z" {
				t = "zones:"
			} else {
				t = "counties:"
			}
			for _, c := range s.Zones {
				// RELATE the county/zones to the segment
				_, err = Surreal().Query(fmt.Sprintf("RELATE %s->vtec_county_zones->%s", (*record)[0].ID, t+s.Name+c), map[string]string{})

				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}
