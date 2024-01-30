package parsers

import (
	"errors"
	"log"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/TheRangiCrew/NWWS-GO/parser/util"
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
	ID         string   `json:"id"`
	Type       string   `json:"type"`
	Number     int      `json:"number"`
	WOU        *string  `json:"wou"`
	WWP        *WWP     `json:"wwp"`
	SEL        *string  `json:"sel"`
	SELProduct *Product `json:"-"`
}

func (w *Watch) IsReady() bool {
	if w.WWP.Product == nil || *w.SEL == "" || *w.WOU == "" || w.ID == "" || w.Number == 0 {
		log.Println("Watch " + w.ID + " incomplete. Waiting...")
		return false
	}
	log.Println("Watch " + w.ID + " ready")
	return true
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

	id := t + "A" + util.PadZero(strconv.Itoa(n), 4) + strconv.Itoa(issued.Year())

	s := ""
	w := ""

	watch := Watch{
		ID:         id,
		Type:       t,
		Number:     n,
		SEL:        &s,
		WOU:        &w,
		SELProduct: &Product{},
		WWP:        &WWP{},
	}

	*q = append(*q, watch)

	result := findWatch(t, n)

	if result == nil {
		return &Watch{}, errors.New("tried to create watch but failed somehow")
	}

	log.Println("Watch Created " + watch.ID)

	return result, nil
}

func parseWOU(product *Product) (*Watch, error) {
	watchIDRegexp := regexp.MustCompile("(WS|WT) ([0-9]{1,4})")
	watchID := strings.Split(watchIDRegexp.FindString(product.Text), " ")

	var phenomena string
	if watchID[0] == "WS" {
		phenomena = "SV"
	} else {
		phenomena = "TO"
	}

	if phenomena == "" {
		return nil, errors.New("failed to parse WWP watch phenomena")
	}

	number, err := strconv.Atoi(watchID[1])

	if err != nil {
		return nil, errors.New("failed to parse WWP watch number")
	}

	watch := findWatch(phenomena, number)

	if watch == nil {
		watch, err = createWatch(phenomena, number, product.Issued)

		if err != nil {
			return nil, err
		}
	}

	*watch.WOU = product.Text

	return watch, nil
}

func parseWWP(product *Product) (*Watch, error) {
	watchIDRegexp := regexp.MustCompile("(WS|WT) ([0-9]{4})")
	watchID := strings.Split(watchIDRegexp.FindString(product.Text), " ")

	var phenomena string
	if watchID[0] == "WS" {
		phenomena = "SV"
	} else {
		phenomena = "TO"
	}

	if phenomena == "" {
		return nil, errors.New("failed to parse WWP watch phenomena")
	}

	number, err := strconv.Atoi(watchID[1])

	if err != nil {
		return nil, errors.New("failed to parse WWP watch number")
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
		return nil, errors.New("failed to parse float of WWP Max Hail")
	}
	maxWind, err := strconv.Atoi(strings.TrimSpace(rows[1]))
	if err != nil {
		return nil, errors.New("failed to parse int of WWP Max Wind")
	}
	maxTops, err := strconv.Atoi(strings.TrimSpace(rows[2]))
	if err != nil {
		return nil, errors.New("failed to parse int of WWP Max Tops")
	}
	meanStorm := strings.TrimSpace(rows[3])
	degrees, err := strconv.Atoi(meanStorm[:3])
	if err != nil {
		return nil, errors.New("failed to parse degrees of WWP Mean Storm Vector")
	}
	speed, err := strconv.Atoi(meanStorm[3:])
	if err != nil {
		return nil, errors.New("failed to parse speed of WWP Mean Storm Vector")
	}
	pdsString := strings.TrimSpace(rows[4])

	pds := false
	if pdsString == "YES" {
		pds = true
	}

	wwp := WWP{
		Product:             product,
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
			return nil, err
		}
	}

	*watch.WWP = wwp

	return watch, nil
}

func parseSEL(product *Product) (*Watch, error) {
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
		return nil, errors.New("failed to parse SEL watch number")
	}

	watch := findWatch(phenomena, number)

	if watch == nil {
		watch, err = createWatch(phenomena, number, product.Issued)

		if err != nil {
			return nil, err
		}
	}

	*watch.SELProduct = *product
	*watch.SEL = original

	return watch, nil
}

func ParseWatchProduct(product *Product) (*Watch, error) {
	var watch *Watch
	var err error
	switch product.AWIPS.Product {
	case "WWP":
		watch, err = parseWWP(product)
	case "SEL":
		watch, err = parseSEL(product)
	case "WOU":
		watch, err = parseWOU(product)
	}

	return watch, err
}
