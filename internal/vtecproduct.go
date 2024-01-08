package internal

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
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

func ParseVTECProduct(segment Segment, product Product) error {

	emergencyRegexp := regexp.MustCompile(`(TORNADO|FLASH\s+FLOOD)\s+EMERGENCY`)
	pdsRegexp := regexp.MustCompile(`THIS\s+IS\s+A\s+PARTICULARLY\s+DANGEROUS\s+SITUATION`)

	ugc, err := ParseUGC(segment.Text, product.Issued)
	if err != nil {
		return err
	}

	vtecs, err := ParsePVTEC(segment.Text, product.Issued)
	if err != nil {
		return err
	}

	hvtec := ParseHVTEC(segment.Text, product.Issued)

	latlon, err := ParseLatLon(segment.Text)
	if err != nil {
		return err
	}

	var polygon *PolygonFeature
	if latlon != nil {
		polygon = latlon.Polygon
	} else {
		polygon = nil
	}

	tml, err := ParseTML(segment.Text, product.Issued)
	if err != nil {
		return err
	}

	emergency := emergencyRegexp.MatchString(segment.Text)
	pds := pdsRegexp.MatchString(segment.Text)

	hazardTags := ParseHazardTags(segment.Text)

	month := padLeft(strconv.Itoa(int(product.Issued.Month())), 2)
	day := padLeft(strconv.Itoa(product.Issued.Day()), 2)
	hour := padLeft(strconv.Itoa(product.Issued.Hour()), 2)
	minute := padLeft(strconv.Itoa(product.Issued.Minute()), 2)
	second := padLeft(strconv.Itoa(time.Now().Second()), 2)

	for _, vtec := range vtecs {

		id := vtec.ID + month + day + hour + minute + second + padLeft(strconv.Itoa(segment.ID), 2)

		final := VTECSegment{
			ID:         id,
			VTECUID:    vtec.ID,
			Original:   segment.Text,
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

		// RELATE the text product to the segment
		_, err = Surreal().Query(fmt.Sprintf("RELATE text_products:%s->vtec_text_products->vtec_segments:%s", product.ID, id), map[string]string{})
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
				_, err = Surreal().Query(fmt.Sprintf("RELATE vtec_segments:%s->vtec_county_zones->%s", id, t+s.Name+c), map[string]string{})

				if err != nil {
					return err
				}
			}
		}

		_, err := Surreal().Create("vtec_segments", final)
		if err != nil {
			return err
		}

	}

	return nil
}
