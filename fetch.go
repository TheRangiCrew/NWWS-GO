package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/TheRangiCrew/NWWS-GO/internal"
)

type Metadata struct {
	Data []struct {
		ProductID string `json:"product_id"`
	} `json:"data"`
}

func roundUp(date string) {
	awipsIDS := []string{
		"NPW", "WSW", "CFW", "FFA", "FFS", "FFW", "FLS", "FLW", "MWS", "MWW", "SMW", "SVR", "SVS", "WCN", "RFW",
		"DSW", "TOR", "SIG", "SQW", "SEL", "SWO", "WOU", "WWP",
	}

	products := []string{}

	for _, p := range awipsIDS {
		url := fmt.Sprintf("https://mesonet.agron.iastate.edu/api/1/nws/afos/list.json?pil=%s&date=%s", p, date)
		res, err := http.Get(url)
		if err != nil {
			panic(err)
		}

		var m Metadata

		err = json.NewDecoder(res.Body).Decode(&m)
		if err != nil {
			panic(err)
		}

		for _, p := range m.Data {
			products = append(products, p.ProductID)
		}

	}

	if len(products) == 0 {
		panic("Got no products")
	}

	fmt.Printf("\n\nFound %d products\n\n", len(products))

	for i, p := range products {

		url := fmt.Sprintf("https://mesonet.agron.iastate.edu/api/1/nwstext/%s", p)
		res, err := http.Get(url)
		if err != nil {
			panic(err)
		}

		defer res.Body.Close()

		b, err := io.ReadAll(res.Body)
		if err != nil {
			panic(err)
		}

		errCh := make(chan error)
		fmt.Printf("%d/%d\n", i+1, len(products))
		go internal.Processor(string(b), errCh)
		err = <-errCh
		if err != nil {
			log.Println(err)
		}

	}

}
