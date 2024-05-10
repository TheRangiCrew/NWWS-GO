package db

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/TheRangiCrew/NWWS-GO/parser/parsers"
	"github.com/TheRangiCrew/NWWS-GO/parser/util"
	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/conn/gorilla"
	"github.com/surrealdb/surrealdb.go/pkg/marshal"
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

		db, err := surrealdb.New(url, gorilla.Create())
		if err != nil {
			return err
		}

		if _, err = db.Use(namespace, database); err != nil {
			return err
		}

		authData := &surrealdb.Auth{
			Username:  username,
			Password:  password,
			Namespace: namespace,
		}
		if _, err = db.Signin(authData); err != nil {
			return err
		}

		surreal = db
	}

	return nil
}

func PushTextProduct(product parsers.Product) error {
	product.WFO = "wfo:" + product.WFO

	// Push the text product to the database
	_, err := Surreal().Create("text_products", product)
	return err
}

type VTECProduct struct {
	ID           string                  `json:"id"`
	Created_At   time.Time               `json:"created_at,omitempty"`
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
	Created_At   time.Time               `json:"created_at,omitempty"`
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
	VTEC         parsers.PVTEC           `json:"vtec"`
	HVETC        *parsers.HVTEC          `json:"hvtec,omitempty"` // TODO: Add HVTEC support
	UGC          parsers.UGC             `json:"ugc"`
	LatLon       *parsers.LATLON         `json:"latlon,omitempty"`
	TML          *parsers.TML            `json:"tml,omitempty"`
	HazardTags   parsers.HazardTags      `json:"tags"`
	Emergency    bool                    `json:"emergency"`
	PDS          bool                    `json:"pds"`
	WFO          string                  `json:"wfo"`
}

func PushVTECProduct(p *parsers.VTECProduct) error {
	product := p.Product
	for _, segment := range p.Segments {

		// Create ID
		year := strconv.Itoa(product.Issued.Year())

		vtecID := segment.WFO + segment.Phenomena + segment.Significance + util.PadZero(strconv.Itoa(segment.EventNumber), 4) + year

		// Get the most recent record if one exists
		query := fmt.Sprintf(`SELECT *, count(->vtec_product_segments) AS children FROM vtec_product:%s`, vtecID)

		record, err := marshal.SmartUnmarshal[VTECProduct](Surreal().Query(query, map[string]string{}))
		if err != nil {
			return err
		}

		var parent *VTECProduct
		if len(record) != 0 {
			parent = &record[0]
		}

		if segment.VTEC.Start == nil {
			if parent == nil {
				segment.VTEC.Start = &segment.Issued
			} else {
				segment.VTEC.Start = &parent.Start
			}
		}

		if segment.VTEC.End == nil {
			segment.VTEC.End = &segment.Expires
		}

		newParent := false
		if parent == nil {

			parent = &VTECProduct{
				ID:           "vtec_product:" + vtecID,
				Created_At:   time.Now(),
				Start:        *segment.VTEC.Start,
				End:          *segment.VTEC.End,
				Issued:       product.Issued,
				Expires:      segment.UGC.Expires,
				EndInitial:   *segment.VTEC.End,
				EventNumber:  segment.VTEC.ETN,
				Action:       "vtec_actions:" + segment.VTEC.Action,
				Phenomena:    "phenomena:" + segment.VTEC.Phenomena,
				Significance: "vtec_significance:" + segment.VTEC.Significance,
				Polygon:      segment.Polygon,
				WFO:          "wfo:" + segment.VTEC.WFO,
				Children:     0,
			}
			newParent = true
		}

		id := vtecID + strconv.Itoa(parent.Children)

		final := VTECSegment{
			ID:           id,
			Created_At:   time.Now(),
			Original:     segment.Original,
			Start:        *segment.VTEC.Start, // From VTEC
			End:          *segment.VTEC.End,   // From VTEC
			Issued:       product.Issued,      // From WMO line
			Expires:      segment.UGC.Expires, // From UGC
			EventNumber:  segment.VTEC.ETN,
			Action:       "vtec_actions:" + segment.VTEC.Action,
			Phenomena:    "phenomena:" + segment.VTEC.Phenomena,
			Significance: "vtec_significance:" + segment.VTEC.Significance,
			Polygon:      segment.Polygon,
			VTEC:         segment.VTEC,
			HVETC:        segment.HVETC,
			UGC:          segment.UGC,
			LatLon:       segment.LatLon,
			TML:          segment.TML,
			HazardTags:   segment.HazardTags,
			Emergency:    segment.Emergency,
			PDS:          segment.PDS,
			WFO:          "wfo:" + segment.VTEC.WFO,
		}

		// Verify products a little bit
		if parent.WFO != final.WFO {
			return fmt.Errorf("vtec WFO mismatch. Found %s needed %s on VTEC %s", final.WFO, parent.WFO, segment.VTEC.Original)
		}
		if parent.EventNumber != final.EventNumber {
			return fmt.Errorf("vtec ETN mismatch. Found %d needed %d on VTEC %s", final.EventNumber, parent.EventNumber, segment.VTEC.Original)
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
		_, err = Surreal().Create("vtec_segment", final)
		if err != nil {
			return errors.New("error while creating vtec_segment: " + err.Error())
		}

		// RELATE the text product to the segment
		_, err = Surreal().Query(fmt.Sprintf("RELATE text_products:%s->vtec_text_products->vtec_segment:%s", product.ID, id), map[string]string{})
		if err != nil {
			return err
		}

		// RELATE the vtec product to the segment
		_, err = Surreal().Query(fmt.Sprintf("RELATE %s->vtec_product_segments->vtec_segment:%s", parent.ID, id), map[string]string{})
		if err != nil {
			return err
		}

		parent.UpdatedAt = time.Now()
		if parent.Start.Compare(final.Start) > 0 {
			parent.Start = final.Start
		}
		if parent.Issued.Compare(final.Issued) > 0 {
			parent.Issued = final.Issued
		}
		if parent.End.Compare(final.End) < 0 {
			parent.End = final.End
		}
		if parent.EndInitial.Compare(final.End) > 0 {
			parent.EndInitial = final.End
		}
		if parent.Expires.Compare(final.Expires) < 0 {
			parent.Expires = final.Expires
		}

		// Update UGC
		for _, s := range final.UGC.States {

			for _, c := range s.Zones {
				id := "ugc:" + s.Name + s.Type + c
				// Check to see if the UGC record already exists
				cz, err := marshal.SmartUnmarshal[struct {
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
				}](Surreal().Query(fmt.Sprintf(`SELECT (SELECT * FROM $parent->vtec_ugc WHERE out == %s) AS cz FROM %s`, id, parent.ID), map[string]string{}))
				if err != nil {
					return err
				}

				if len(cz) != 0 && len(cz[0].CZ) != 0 {

					update := false
					current := cz[0].CZ[0]
					if current.Start.Compare(final.Start) > 0 {
						current.Start = final.Start
						update = true
					}
					if current.End.Compare(final.End) < 0 {
						current.End = final.End
						update = true
					}
					if current.Expires.Compare(parent.Expires) < 0 {
						current.Expires = parent.Expires
						update = true
					}
					if current.Action != final.Action {
						current.Action = final.Action
						update = true
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
					_, err = Surreal().Query("RELATE $product->vtec_ugc->$ugc SET start = $start, end = $end, issued = $issued, expires = $expires, action = $action", map[string]string{
						"product": parent.ID,
						"ugc":     id,
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
			start, err := (parent.Start.MarshalText())
			if err != nil {
				return err
			}
			issued, err := (parent.Issued.MarshalText())
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

			polygon := "NONE"
			if final.Polygon != nil {
				polygonJSON, err := json.Marshal(*final.Polygon)
				if err != nil {
					return err
				}
				polygon = string(polygonJSON)
			}

			_, err = Surreal().Query("UPDATE $id SET updated_at = $updated, start = $start, issued = $issued, end = $end, expires = $expires, action = $action, polygon = "+string(polygon), map[string]interface{}{
				"id":      parent.ID,
				"updated": string(updatedAt),
				"start":   string(start),
				"issued":  string(issued),
				"end":     string(end),
				"expires": string(expires),
				"action":  final.Action,
				"polygon": string(polygon),
			})
			if err != nil {
				return err
			}
		}
	}

	PushTextProduct(*product)

	return nil
}

func PushWatch(watch *parsers.Watch) error {
	_, err := Surreal().Create("severe_watches", *watch)

	return err
}

func PushMCD(mcd *parsers.MCD, p *parsers.Product) error {
	concerningRegexp := regexp.MustCompile(`(Concerning\.\.\.)([A-Za-z0-9 \.\n]+)\n\n`)
	concerningLine := strings.TrimSpace(concerningRegexp.FindString(mcd.Original))
	concerningLine = strings.Replace(concerningLine, "Concerning...", "", 1)
	if concerningLine != "" {
		phenomenaRegexp := regexp.MustCompile("(Severe Thunderstorm Watch|Tornado Watch) ([0-9]+)")
		phenomenaString := phenomenaRegexp.FindString(concerningLine)
		if phenomenaString != "" {
			phenomena := "TO"
			if phenomenaString == "" {
				phenomena = "SV"
			}

			watchNumberRegexp := regexp.MustCompile(`[0-9]+`)
			watchNumber := watchNumberRegexp.FindString(concerningLine)
			if watchNumber == "" {
				return errors.New("Found concerning watch in MCD but couldn't parse number MCD " + strconv.Itoa(mcd.Number))
			}
			watchID := "severe_watches:" + phenomena + "A" + util.PadZero(watchNumber, 4) + strconv.Itoa(mcd.Issued.Year())

			// RELATE the watch product to the segment
			_, err := Surreal().Query(fmt.Sprintf("RELATE mcd:%s->mcd_watch->%s", mcd.ID, watchID), map[string]string{})
			if err != nil {
				return err
			}
		}
	}

	mcd.Concerning = concerningLine

	_, err := Surreal().Create("mcd", mcd)
	if err != nil {
		return err
	}

	// RELATE the text product to the mcd
	_, err = Surreal().Query(fmt.Sprintf("RELATE text_products:%s->mcd_text_products->mcd:%s", p.ID, mcd.ID), map[string]string{})

	return err
}
