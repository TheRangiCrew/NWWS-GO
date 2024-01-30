package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	day = time.Hour * 24
)

type IEMArchiveSettings struct {
	WFO     string
	Product string
	Start   time.Time
	End     time.Time
}

type ListItem struct {
	ProductID string `json:"product_id"`
}

type List struct {
	Data []ListItem `json:"data"`
}

func RunIEMArchive(args []string) error {

	productArgRegexp := regexp.MustCompile("(--(.)+)")
	productRegexp := regexp.MustCompile("([A-Za-z0-9,]{3,})")
	dateRegexp := regexp.MustCompile("[0-9]{4}-[0-9]{2}-[0-9]{2}")
	dateLayout := "2006-01-02"

	settings := IEMArchiveSettings{}

	argsLen := len(args)
	for index := 0; index < argsLen; index++ {
		arg := args[index]
		switch arg {
		case "--product":
			index++
			settings.Product = args[index]
			if productArgRegexp.MatchString(settings.Product) || !productRegexp.MatchString(settings.Product) {
				return fmt.Errorf("argument %s is not a valid product", settings.Product)
			}
		case "--start":
			index++
			str := args[index]
			if !dateRegexp.MatchString(str) {
				return fmt.Errorf("argument %s is not a valid date string", str)
			}
			t, err := time.Parse(dateLayout, str)
			if err != nil {
				return err
			}
			settings.Start = t.UTC()
		case "--end":
			index++
			str := args[index]
			if !dateRegexp.MatchString(str) {
				return fmt.Errorf("argument %s is not a valid date string", str)
			}
			t, err := time.Parse(dateLayout, str)
			if err != nil {
				return err
			}
			settings.End = t.UTC()
		}
	}

	if settings.Product == "" && settings.WFO == "" {
		return fmt.Errorf("arg --product or --wfo is required but neither were provided")
	}
	if settings.Start.IsZero() {
		return fmt.Errorf("arg --start is required but was't provided")
	}
	if settings.End.IsZero() {
		settings.End = settings.Start.Add(-24 * time.Hour)
		log.Printf("no end date provided. Defaulting to day before (%s)\n", settings.End.Format(dateLayout))
	}

	days := settings.Start.Sub(settings.End).Hours() / 24
	baseURL := "https://mesonet.agron.iastate.edu/api/1/nws/afos/list.json?"
	list := List{}

	if settings.Product != "" {
		pils := strings.Split(settings.Product, ",")
		checked := 0
		total := int(days) * len(pils)
		for _, pil := range pils {
			params := []string{}
			params = append(params, "pil="+pil)
			if settings.WFO != "" {
				params = append(params, "cccc="+settings.WFO)
			}

			url := baseURL

			for i, p := range params {
				if i > 0 {
					url += "&"
				}
				url += p
			}

			for i := 0; i < int(days); i++ {
				// Fetch the list from the IEM
				url = url + fmt.Sprintf("&date=%s", settings.Start.Add(time.Duration(-i)*day).Format(dateLayout))

				res, err := http.Get(url)
				if err != nil {
					return err
				}
				// Decode JSON
				defer res.Body.Close()
				j := List{}
				if err := json.NewDecoder(res.Body).Decode(&j); err != nil {
					return err
				}
				list.Data = append(list.Data, j.Data...)
				checked++
				fmt.Printf("\rChecking days | %*d/%d ", len(strconv.Itoa(total)), checked, total)
				width := int((float32(checked)/float32(total))*100) / 5
				for i := 0; i < width; i++ {
					fmt.Print("=")
				}
				fmt.Print(">")
			}
		}
	} else {
		params := []string{}
		if settings.WFO != "" {
			params = append(params, "cccc="+settings.WFO)
		}

		for i, p := range params {
			if i > 0 {
				baseURL += "&"
			}
			baseURL += p
		}

		for i := 0; i < int(days); i++ {
			// Fetch the list from the IEM
			url := baseURL + fmt.Sprintf("&date=%s", settings.Start.Add(time.Duration(-i)*day).Format(dateLayout))
			res, err := http.Get(url)
			if err != nil {
				return err
			}
			// Decode JSON
			defer res.Body.Close()
			j := List{}
			if err := json.NewDecoder(res.Body).Decode(&j); err != nil {
				return err
			}
			list.Data = append(list.Data, j.Data...)
			// Parse the products
		}
	}

	fmt.Printf("\n\n%d products found\n\n", len(list.Data))

	start := time.Now()
	timeSum := time.Duration(0)
	parsed := 0
	avg := time.Duration(0)
	for _, p := range list.Data {
		parseStart := time.Now()
		// Get the product
		url := "https://mesonet.agron.iastate.edu/api/1/nwstext/" + p.ProductID
		res, err := http.Get(url)
		if err != nil {
			return err
		}
		// Decode the product
		defer res.Body.Close()
		s, err := io.ReadAll(res.Body)
		if err != nil {
			return err
		}
		if err = Processor(string(s)); err != nil {
			return err
		}
		parseEnd := time.Now()
		parsed++
		timeSum += parseEnd.Sub(parseStart)
		avg = time.Duration(float64(timeSum.Nanoseconds()) / float64(parsed))
		end := start.Add(time.Duration(int(avg.Nanoseconds()) * len(list.Data)))
		dur := time.Until(end)
		fmt.Printf("\r%02d mins %02d secs | % *d/%d Complete", int(dur.Minutes())%60, int(dur.Seconds())%60,
			len(strconv.Itoa(len(list.Data))), parsed, len(list.Data))
	}

	fmt.Println()

	return nil
}
