package main

import (
	"fmt"
	"strconv"

	"github.com/TheRangiCrew/NWWS-GO/parser/db"
	"github.com/TheRangiCrew/NWWS-GO/parser/parsers"
	"github.com/TheRangiCrew/NWWS-GO/parser/util"
	"github.com/surrealdb/surrealdb.go/pkg/marshal"
)

func Processor(text string) error {
	var err error

	product, err := parsers.NewAWIPSProduct(text)
	if err != nil {
		return err
	}

	if product == nil {
		return nil
	}

	record, err := marshal.SmartUnmarshal[[]struct {
		Group string `json:"group"`
	}](db.Surreal().Query(fmt.Sprintf("SELECT group FROM text_products WHERE group == '%s'", product.Group), map[string]string{}))

	sequence := len(record)

	id := product.Group + util.PadZero(strconv.Itoa(sequence), 2)

	product.ID = id

	// Send products that need special treatment on their way
	// Severe Watches
	switch product.AWIPS.Product {
	case "WWP":
		fallthrough
	case "SEL":
		fallthrough
	case "WOU":
		watch, err := product.WatchProduct()
		if err != nil {
			return err
		}
		if watch.IsReady() {
			return db.PushWatch(watch)
		}
		return nil
	case "PTS":
		// product.PTSProduct()
	}
	if product.AWIPS.Product == "SWO" {
		if product.AWIPS.WFO == "MCD" {
			mcd, err := product.MCDProduct()
			if err != nil {
				return err
			}
			return db.PushMCD(mcd, product)
		}
	}

	if product.HasVTEC() {
		vtecProduct, err := product.VTECProduct()
		if err != nil {
			return err
		}
		return db.PushVTECProduct(vtecProduct)
	}

	return nil
}
