package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/surrealdb/surrealdb.go"
)

type VTECProduct struct {
	ID           string          `json:"id"`
	UpdatedAt    time.Time       `json:"updated_at,omitempty"`
	Start        time.Time       `json:"start"`
	End          time.Time       `json:"end"`
	Issued       time.Time       `json:"issued"`
	Expires      time.Time       `json:"expires"`
	EndInitial   time.Time       `json:"end_initial"`
	EventNumber  int             `json:"event_number"`
	Action       string          `json:"action"`
	Phenomena    string          `json:"phenomena"`
	Significance string          `json:"significance"`
	Polygon      *PolygonFeature `json:"polygon"`
	Title        string          `json:"title,omitempty"`
	WFO          string          `json:"wfo"`
	Children     int             `json:"children,omitempty"`
}

type VTECSegment struct {
	ID           string          `json:"id,omitempty"`
	Original     string          `json:"original"`
	Start        time.Time       `json:"start"`   // From VTEC
	End          time.Time       `json:"end"`     // From VTEC
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

	for _, vtec := range vtecs {

		// Get the most recent record if one exists (just to be safe)
		query := fmt.Sprintf(`SELECT *, count(->vtec_product_segments) AS children FROM vtec_product:%s`, vtec.ID)

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
		newParent := false
		if len((*record)[0].Result) != 0 {
			parent = &(*record)[0].Result[0]
		} else {
			parent = &VTECProduct{
				ID:           "vtec_product:" + vtec.ID,
				Start:        vtec.Start,
				End:          vtec.End,
				Issued:       product.Issued,
				Expires:      ugc.Expires,
				EndInitial:   vtec.End,
				EventNumber:  vtec.ETN,
				Action:       "vtec_actions:" + vtec.Action,
				Phenomena:    "phenomena:" + vtec.Phenomena,
				Significance: "vtec_significance:" + vtec.Significance,
				Polygon:      polygon,
				WFO:          vtec.WFO,
				Children:     0,
			}
			newParent = true
		}

		id := vtec.ID + strconv.Itoa(parent.Children)

		final := VTECSegment{
			ID:           id,
			Original:     segment.Text,
			Issued:       product.Issued, // From WMO line
			Start:        vtec.Start,     // From VTEC
			End:          vtec.End,       // From VTEC
			Expires:      ugc.Expires,    // From UGC
			EventNumber:  vtec.ETN,
			Action:       "vtec_actions:" + vtec.Action,
			Phenomena:    "phenomena:" + vtec.Phenomena,
			Significance: "vtec_significance:" + vtec.Significance,
			Polygon:      polygon,
			VTEC:         vtec,
			HVETC:        hvtec,
			UGC:          ugc,
			LatLon:       latlon,
			TML:          tml,
			HazardTags:   hazardTags,
			Emergency:    emergency,
			PDS:          pds,
			WFO:          product.WMO.WFO,
		}

		// Verify products a little bit
		if parent.WFO != final.WFO {
			return errors.New("vtec WFO mismatch")
		}
		if parent.EventNumber != final.EventNumber {
			return errors.New("vtec ETN mismatch")
		}
		if parent.Phenomena != final.Phenomena {
			return errors.New("vtec phenomena mismatch")
		}
		if parent.Significance != final.Significance {
			return errors.New("vtec significance mismatch")
		}

		/*
			Push the VTEC segments first to make sure that will actually work
		*/
		// RELATE the text product to the segment
		_, err = Surreal().Query(fmt.Sprintf("RELATE text_products:%s->vtec_text_products->vtec_segment:%s", product.ID, id), map[string]string{})
		if err != nil {
			return err
		}

		_, err = Surreal().Create("vtec_segment", final)
		if err != nil {
			return err
		}

		// RELATE the vtec product to the segment
		_, err = Surreal().Query(fmt.Sprintf("RELATE %s->vtec_product_segments->vtec_segment:%s", parent.ID, id), map[string]string{})
		if err != nil {
			return err
		}

		parent.UpdatedAt = time.Now()
		if parent.End.Compare(final.End) < 0 {
			parent.End = final.End
		}
		if parent.Expires.Compare(final.Expires) < 0 {
			parent.Expires = final.Expires
		}

		// Update UGC
		for _, s := range final.UGC.States {

			var t string
			if s.Type == "Z" {
				t = "zones:"
			} else {
				t = "counties:"
			}
			for _, c := range s.Zones {
				id := t + s.Name + c
				// Check to see if the UGC record already exists
				query := fmt.Sprintf(`SELECT (SELECT * FROM $parent->vtec_county_zones WHERE out == %s) AS cz FROM %s`, id, parent.ID)
				ugcResult, err := Surreal().Query(query, map[string]string{})
				if err != nil {
					return err
				}

				// NOTE: Surreal returns an array of the result which requires an array to be Unmarshalled. This is referenced later
				cz := new([]surrealdb.RawQuery[[]struct {
					CZ []struct {
						ID      string    `json:"id"`
						In      string    `json:"in"`
						Out     string    `json:"out"`
						Start   time.Time `json:"start"`
						End     time.Time `json:"end"`
						Issued  time.Time `json:"issued"`
						Expires time.Time `json:"expires"`
						Action  string    `json:"action"`
					} `json:"cz"`
				}])
				err = surrealdb.Unmarshal(ugcResult, &cz)
				if err != nil {
					return err
				}

				if len((*cz)[0].Result) != 0 {
					if len((*cz)[0].Result[0].CZ) != 0 {
						update := false
						current := (*cz)[0].Result[0].CZ[0]
						if current.End.Compare(final.End) < 0 {
							current.End = final.End
							update = true
						}
						if current.Expires.Compare(parent.Expires) < 0 {
							update = true
							current.Expires = parent.Expires
						}
						if current.Action != final.Action {
							update = true
							current.Action = final.Action
						}
						if update {
							end, err := (current.End.MarshalText())
							if err != nil {
								return err
							}
							expires, err := (current.Expires.MarshalText())
							if err != nil {
								return err
							}

							_, err = Surreal().Query("UPDATE $id SET end = $end, expires = $expires, action = $action", map[string]interface{}{
								"id":      current.ID,
								"end":     string(end),
								"expires": string(expires),
								"action":  current.Action,
							})
							if err != nil {
								return err
							}
						}
					}
				} else {
					start, err := (final.Start.MarshalText())
					if err != nil {
						return err
					}
					end, err := (final.End.MarshalText())
					if err != nil {
						return err
					}
					issued, err := (parent.Issued.MarshalText())
					if err != nil {
						return err
					}
					expires, err := (parent.Expires.MarshalText())
					if err != nil {
						return err
					}

					// RELATE the county/zones to the product
					_, err = Surreal().Query("RELATE $product->vtec_county_zones->$zone SET start = $start, end = $end, issued = $issued, expires = $expires, action = $action", map[string]string{
						"product": parent.ID,
						"zone":    id,
						"start":   string(start),
						"end":     string(end),
						"issued":  string(issued),
						"expires": string(expires),
						"action":  final.Action,
					})
					if err != nil {
						return err
					}

				}
			}
		}

		// And finally... push the product
		if newParent {
			_, err = Surreal().Create("vtec_product", parent)
			if err != nil {
				return err
			}
		} else {
			updatedAt, err := (parent.UpdatedAt.MarshalText())
			if err != nil {
				return err
			}
			end, err := (parent.End.MarshalText())
			if err != nil {
				return err
			}
			expires, err := (parent.Expires.MarshalText())
			if err != nil {
				return err
			}

			polygon, err := json.Marshal(*final.Polygon)
			if err != nil {
				return err
			}
			fmt.Println("UPDATE $id SET updated_at = $updated, end = $end, expires = $expires, action = $action, polygon = " + string(polygon))

			r, err := Surreal().Query("UPDATE $id SET updated_at = $updated, end = $end, expires = $expires, action = $action, polygon = "+string(polygon), map[string]interface{}{
				"id":      parent.ID,
				"updated": string(updatedAt),
				"end":     string(end),
				"expires": string(expires),
				"action":  final.Action,
				"polygon": string(polygon),
			})
			if err != nil {
				return err
			}
			fmt.Println(r)
		}

	}

	return nil
}
