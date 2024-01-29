package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/TheRangiCrew/NWWS-GO/internal"
	"github.com/joho/godotenv"
)

func runTestFiles() {

	for i := 1; i <= 1; i++ {
		tornado, err := os.ReadFile("texts/blizzard/1/" + strconv.Itoa(i) + ".txt")
		// tornado, err := os.ReadFile("texts/tornado/1/5.txt")

		if err != nil {
			panic(err)
		}

		fmt.Println("Parsing " + strconv.Itoa(i) + "...")

		errCh := make(chan error)
		go internal.Processor(string(tornado), errCh)
		err = <-errCh
		if err != nil {
			fmt.Println(err)
			_, err := internal.Surreal().Query(fmt.Sprintf("INSERT INTO error_logs (error) VALUES ('%s')", err), map[string]interface{}{})
			if err != nil {
				fmt.Println(err)
			}
		}
	}
}

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	if internal.Surreal() == nil {
		log.Fatal("Failed to connect to DB")
	}

	// Lazy but comment out what shouldn't be running
	runTestFiles()
	// XMPP()
}
