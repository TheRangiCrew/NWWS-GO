package internal

import (
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type WWP struct {
	Product             *Product `json:"-"`
	TwoOrMoreTor        string   `json:"twoOrMoreTor"`
	StrongTor           string   `json:"strongTor"`
	TenOrMoreSevereWind string   `json:"tenOrMoreSevereWind"`
	OneOrMoreWind       string   `json:"oneOrMoreWind"`
	TenOrMoreSevereHail string   `json:"tenOrMoreSevereHail"`
	OneOrMoreHail       string   `json:"oneOrMoreHail"`
	SixOrMoreCombo      string   `json:"sixOrMoreCombo"`
	MaxHail             float32  `json:"maxHail"`
	MaxWind             int      `json:"maxWind"`
	MaxTops             int      `json:"maxTops"`
	Degrees             int      `json:"degrees"`
	Speed               int      `json:"speed"`
	PDS                 bool     `json:"pds"`
}

type Watch struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Number     int            `json:"number"`
	WWP        *WWP           `json:"wwp"`
	SEL        *string        `json:"sel"`
	SELProduct *Product       `json:"-"`
	WOU        *string        `json:"wou"`
	WOUProduct *Product       `json:"-"`
	VTEC       *[]VTECSegment `json:"-"`
}

var lock = &sync.Mutex{}

var queue *[]Watch

func Queue() *[]Watch {
	lock.Lock()
	defer lock.Unlock()

	if queue == nil {
		queue = &[]Watch{}
	}

	return queue
}

func findWatch(t string, n int) *Watch {
	q := Queue()

	for _, w := range *q {
		if w.Number == n && w.Type == t {
			foundWatch := w    // Create a local variable and assign the value of 'w'
			return &foundWatch // Return the address of 'foundWatch'
		}
	}

	return nil
}

func createWatch(t string, n int, issued time.Time) (*Watch, error) {
	current := findWatch(t, n)

	if current != nil {
		return current, nil
	}

	q := Queue()

	id := t + "A" + padLeft(strconv.Itoa(n), 4) + strconv.Itoa(issued.Year())

	s := ""
	w := ""

	watch := Watch{
		ID:         id,
		Type:       t,
		Number:     n,
		SEL:        &s,
		SELProduct: &Product{},
		WWP:        &WWP{},
		WOU:        &w,
		WOUProduct: &Product{},
		VTEC:       &[]VTECSegment{},
	}

	*q = append(*q, watch)

	result := findWatch(t, n)

	if result == nil {
		return &Watch{}, errors.New("tried to create watch but failed somehow")
	}

	log.Print("\nWatch Created" + watch.ID + "\n\n")

	return result, nil
}

func checkout(watch *Watch, product Product) error {
	if len(*watch.VTEC) == 0 || watch.WWP.TwoOrMoreTor == "" || *watch.SEL == "" || watch.ID == "" || watch.Number == 0 {
		log.Print("\nWatch " + watch.ID + " incomplete. Waiting...\n\n")
		return nil
	}

	// Push the SEL product to the database
	_, err := Surreal().Create("text_products", watch.SELProduct)
	if err != nil {
		return err
	}

	// Push the WWP product to the database
	_, err = Surreal().Create("text_products", watch.WWP.Product)
	if err != nil {
		return err
	}

	// Push the WOU product to the database
	_, err = Surreal().Create("text_products", watch.WOUProduct)
	if err != nil {
		return err
	}

	_, err = Surreal().Create("severe_watches", *watch)
	if err != nil {
		return err
	}

	for _, vtec := range *watch.VTEC {

		// RELATE the WOU product to the segment
		_, err = Surreal().Query(fmt.Sprintf("RELATE text_products:%s->vtec_text_products->vtec_segments:%s", watch.WOUProduct.ID, vtec.ID), map[string]string{})
		if err != nil {
			return err
		}

		for _, s := range vtec.UGC.States {
			var t string
			if s.Type == "Z" {
				t = "zones:"
			} else {
				t = "counties:"
			}
			for _, c := range s.Zones {
				// RELATE the county/zones to the segment
				_, err = Surreal().Query(fmt.Sprintf("RELATE vtec_segments:%s->vtec_county_zones->%s", vtec.ID, t+s.Name+c), map[string]string{})

				if err != nil {
					return err
				}
			}
		}

		_, err := Surreal().Create("vtec_segments", vtec)
		if err != nil {
			return err
		}

		// RELATE the watch product to the segment
		_, err = Surreal().Query(fmt.Sprintf("RELATE severe_watches:%s->watch_vtec->vtec_segments:%s", watch.ID, vtec.ID), map[string]string{})
		if err != nil {
			return err
		}
	}

	return nil
}

func ParseWOU(product Product) error {
	segments := strings.Split(product.Text, "$$")

	out := []VTECSegment{}

	for i := 0; i < len(segments)-1; i++ {
		segment := segments[i]

		ugc, err := ParseUGC(segment, product.Issued)
		if err != nil {
			return err
		}

		vtecs, err := ParsePVTEC(segment, product.Issued)
		if err != nil {
			return err
		}

		if len(vtecs) > 1 {
			log.Println("Watch VTEC count is greater than one...?")
		}

		vtec := vtecs[0]

		month := padLeft(strconv.Itoa(int(product.Issued.Month())), 2)
		day := padLeft(strconv.Itoa(product.Issued.Day()), 2)
		hour := padLeft(strconv.Itoa(product.Issued.Hour()), 2)
		minute := padLeft(strconv.Itoa(product.Issued.Minute()), 2)
		second := padLeft(strconv.Itoa(time.Now().Second()), 2)

		id := vtec.ID + month + day + hour + minute + second + padLeft(strconv.Itoa(i), 2)

		final := VTECSegment{
			ID:        id,
			VTECUID:   vtec.ID,
			Original:  segment,
			Issued:    product.WMO.Issued, // From WMO line
			Start:     vtec.Start,         // From VTEC
			End:       vtec.End,           // From VTEC
			Expires:   ugc.Expires,        // From UGC
			WFO:       product.WMO.WFO,
			VTEC:      vtec,
			UGC:       ugc,
			Emergency: false,
			PDS:       false,
		}

		out = append(out, final)
	}

	watch := findWatch(out[0].VTEC.Phenomena, out[0].VTEC.ETN)

	var err error
	if watch == nil {
		watch, err = createWatch(out[0].VTEC.Phenomena, out[0].VTEC.ETN, product.Issued)

		if err != nil {
			return err
		}
	}

	*watch.WOU = product.Text
	*watch.WOUProduct = product
	*watch.VTEC = out
	checkout(watch, product)

	return nil
}

func ParseWWP(product Product) error {
	watchIDRegexp := regexp.MustCompile("(WS|WT) ([0-9]{4})")
	watchID := strings.Split(watchIDRegexp.FindString(product.Text), " ")

	var phenomena string
	if watchID[0] == "WS" {
		phenomena = "SV"
	} else {
		phenomena = "TO"
	}

	if phenomena == "" {
		return errors.New("failed to parse WWP watch phenomena")
	}

	number, err := strconv.Atoi(watchID[1])

	if err != nil {
		return errors.New("failed to parse WWP watch number")
	}

	ampRegexp := regexp.MustCompile(`(&&)`)

	probTableRegexp := regexp.MustCompile(`(PROBABILITY TABLE:\n)`)
	probTableStartIndex := probTableRegexp.FindStringIndex(product.Text)
	probTableStart := product.Text[probTableStartIndex[1]:]

	probTableEndIndex := ampRegexp.FindStringIndex(probTableStart)
	probTableString := strings.ReplaceAll(strings.TrimSpace(probTableStart[:probTableEndIndex[0]]), "  ", "")

	rows := strings.Split(probTableString, "\n")

	for i, r := range rows {
		rows[i] = strings.Split(r, ":")[1]
	}

	twoOrMoreTor := rows[0]
	strongTor := rows[1]
	tenOrMoreSevereWind := rows[2]
	oneOrMoreWind := rows[3]
	tenOrMoreSevereHail := rows[4]
	oneOrMoreHail := rows[5]
	sixOrMoreCombo := rows[6]

	attrTableRegexp := regexp.MustCompile(`(ATTRIBUTE TABLE:\n)`)
	attrTableStartIndex := attrTableRegexp.FindStringIndex(product.Text)
	attrTableStart := product.Text[attrTableStartIndex[1]:]

	attrTableEndIndex := ampRegexp.FindStringIndex(attrTableStart)
	attrTableString := strings.ReplaceAll(strings.TrimSpace(attrTableStart[:attrTableEndIndex[0]]), "  ", "")

	rows = strings.Split(attrTableString, "\n")

	for i, r := range rows {
		rows[i] = strings.Split(r, ":")[1]
	}

	maxHail, err := strconv.ParseFloat(strings.TrimSpace(rows[0]), 32)
	if err != nil {
		return errors.New("failed to parse float of WWP Max Hail")
	}
	maxWind, err := strconv.Atoi(strings.TrimSpace(rows[1]))
	if err != nil {
		return errors.New("failed to parse int of WWP Max Wind")
	}
	maxTops, err := strconv.Atoi(strings.TrimSpace(rows[2]))
	if err != nil {
		return errors.New("failed to parse int of WWP Max Tops")
	}
	meanStorm := strings.TrimSpace(rows[3])
	degrees, err := strconv.Atoi(meanStorm[:3])
	if err != nil {
		return errors.New("failed to parse degrees of WWP Mean Storm Vector")
	}
	speed, err := strconv.Atoi(meanStorm[3:])
	if err != nil {
		return errors.New("failed to parse speed of WWP Mean Storm Vector")
	}
	pdsString := strings.TrimSpace(rows[4])

	pds := false
	if pdsString == "YES" {
		pds = true
	}

	wwp := WWP{
		Product:             &product,
		TwoOrMoreTor:        twoOrMoreTor,
		StrongTor:           strongTor,
		TenOrMoreSevereWind: tenOrMoreSevereWind,
		OneOrMoreWind:       oneOrMoreWind,
		TenOrMoreSevereHail: tenOrMoreSevereHail,
		OneOrMoreHail:       oneOrMoreHail,
		SixOrMoreCombo:      sixOrMoreCombo,
		MaxHail:             float32(maxHail),
		MaxWind:             maxWind,
		MaxTops:             maxTops,
		Degrees:             degrees,
		Speed:               speed,
		PDS:                 pds,
	}

	watch := findWatch(phenomena, number)

	if watch == nil {
		watch, err = createWatch(phenomena, number, product.Issued)

		if err != nil {
			return err
		}
	}

	*watch.WWP = wwp
	err = checkout(watch, product)
	if err != nil {
		return err
	}

	return nil
}

func ParseSEL(product Product) error {
	original := product.Text

	idLineRegexp := regexp.MustCompile(`(Severe Thunderstorm|Tornado)( Watch Number )[0-9]+`)
	idLine := idLineRegexp.FindString(original)

	phenomenaRegexp := regexp.MustCompile(`(Severe Thunderstorm|Tornado)`)
	phenomenaString := phenomenaRegexp.FindString(idLine)

	var phenomena string
	if phenomenaString == "Severe Thunderstorm" {
		phenomena = "SV"
	} else {
		phenomena = "TO"
	}

	numberRegexp := regexp.MustCompile(`[0-9]+`)
	numberString := numberRegexp.FindString(idLine)
	number, err := strconv.Atoi(numberString)
	if err != nil {
		return errors.New("failed to parse SEL watch number")
	}

	watch := findWatch(phenomena, number)

	if watch == nil {
		watch, err = createWatch(phenomena, number, product.Issued)

		if err != nil {
			return err
		}
	}

	*watch.SELProduct = product
	*watch.SEL = original

	err = checkout(watch, product)

	if err != nil {
		return err
	}

	return nil
}
