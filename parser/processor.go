package main

import (
	"fmt"
	"strconv"

	"github.com/TheRangiCrew/NWWS-GO/parser/parsers"
	"github.com/TheRangiCrew/NWWS-GO/parser/util"
	"github.com/surrealdb/surrealdb.go"
)

func Processor(text string) error {
	var err error

	product, err := parsers.NewAWIPSProduct(text)
	if err != nil {
		return err
	}

	txt, err := util.Surreal().Query(fmt.Sprintf("SELECT group FROM text_products WHERE group == '%s'", product.Group), map[string]interface{}{})
	if err != nil {
		return err
	}
	record := new([]surrealdb.RawQuery[[]struct {
		Group string `json:"group"`
	}])
	err = surrealdb.Unmarshal(txt, &record)
	if err != nil {
		return err
	}

	sequence := len((*record)[0].Result)

	id := product.Group + util.PadZero(strconv.Itoa(sequence), 2)

	println(id)

	// Push the text product to the database
	_, err = util.Surreal().Create("text_products", product)
	if err != nil {
		return err
	}

	// Send products that need special treatment on their way
	// Severe Watches
	// if product.AWIPS.Product == "WOU" {
	// 	err = ParseWOU(product)
	// 	return err
	// }
	// if product.AWIPS.Product == "WWP" {
	// 	err = ParseWWP(product)
	// 	return err
	// }
	// if product.AWIPS.Product == "SEL" {
	// 	err = ParseSEL(product)
	// 	return err
	// }
	// if product.AWIPS.Product == "SWO" {
	// 	if product.AWIPS.WFO == "MCD" {
	// 		err = ParseMCD(product)
	// 		return err
	// 	}
	// }

	if product.HasVTEC() {
		vtecProduct, err := product.VTECProduct()
		if err != nil {
			return err
		}
		vtecProduct.Product.
	}

	return nil
}
