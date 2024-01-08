package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/TheRangiCrew/NWWS-GO/internal"
)

func runTestFiles() {

	for i := 1; i < 4; i++ {
		tornado, err := os.ReadFile("texts/watches/1/" + strconv.Itoa(i) + ".txt")
		// tornado, err := os.ReadFile("texts/tornado/1/5.txt")

		if err != nil {
			panic(err)
		}

		fmt.Println("Parsing " + strconv.Itoa(i) + "...")

		errCh := make(chan error)
		go internal.Processor(string(tornado), errCh)
		err = <-errCh
		if err != nil {
			log.Println(err)
		}

		time.Sleep(1 * time.Second)
	}
}

func main() {
	// err := godotenv.Load()
	// if err != nil {
	// 	log.Fatal("Error loading .env file")
	// }

	if internal.Surreal() == nil {
		log.Fatal("Failed to connect to DB")
	}

	// Lazy but comment out what shouldn't be running
	// runTestFiles()
	XMPP()
}
