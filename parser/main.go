package main

import (
	"errors"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/TheRangiCrew/NWWS-GO/parser/db"
)

const (
	PurgeTime time.Duration = time.Duration(30 * time.Minute)
)

var productQueueDirectory string
var errorDumpDirectory string

type FileProduct struct {
	Name  string
	Index int
	Text  string
}

func getProducts() ([]FileProduct, error) {
	path := productQueueDirectory
	productDir, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	products := []FileProduct{}
	for _, d := range productDir {
		if !d.IsDir() {
			index, err := strconv.Atoi(strings.Split(d.Name(), ".")[0])
			if err != nil {
				return nil, err
			}
			file, err := os.ReadFile(path + d.Name())
			if err != nil {
				return nil, err
			}
			text := string(file)
			products = append(products, FileProduct{
				Name:  d.Name(),
				Index: index,
				Text:  text,
			})
		}
	}

	sort.Slice(products, func(i, j int) bool {
		return products[i].Index < products[j].Index
	})

	return products, nil
}

func moveToErrorDump(name string, text string) error {
	_, err := os.ReadDir(errorDumpDirectory)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			err = os.Mkdir(errorDumpDirectory, os.ModePerm.Perm())
			if err != nil {
				return err
			}
			_, err = os.ReadDir(errorDumpDirectory)
			if err != nil {
				return err
			}
		}
	}

	err = os.WriteFile(errorDumpDirectory+name, []byte(text), os.ModePerm.Perm())

	return err
}

func runLatestParser() error {

	products, err := getProducts()
	if err != nil {
		return err
	}

	log.Printf("Found %d products in directory", len(products))

	for {
		if len(products) == 0 {
			time.Sleep(1 * time.Second)
			products, err = getProducts()
		} else {
			log.Printf("Found %d products in directory", len(products))
			if err = Processor((products)[0].Text); err != nil {
				log.Println(err.Error())
				name := time.Now().UTC().Format("2006_01_02_15_04_05_") + (products)[0].Name
				log.Println("Moving to error dump as " + name)
				err = moveToErrorDump(name, (products)[0].Text)
				if err != nil {
					return err
				}
			}
			err = os.Remove(productQueueDirectory + (products)[0].Name)
			if err != nil {
				return err
			}
			products = products[1:]
		}

		if err != nil {
			return err
		}
	}
}

type Mode int

const (
	Live Mode = iota
	IEMArchive
)

func main() {

	var err error
	mode := Live

	args := os.Args[1:]

	if len(args) > 0 {
		switch args[0] {
		case "--live":
			mode = Live
		case "--iem":
			mode = IEMArchive
		}
	}

	// err = godotenv.Load(".env")
	// if err != nil {
	// 	log.Fatal("Error loading .env file")
	// }

	productQueueDirectory = os.Getenv("PRODUCT_QUEUE_DIR")
	errorDumpDirectory = os.Getenv("PRODUCT_ERROR_DIR")

	if mode == Live {
		for {
			if db.SurrealInit() != nil {
				log.Printf("Failed to connect to DB: %s\nTrying again in 30 seconds\n\n", err.Error())
				time.Sleep(30 * time.Second)
				continue
			}
			log.Printf("Connected to DB. Ready to go\n\n")

			if err := runLatestParser(); err != nil {
				log.Printf("Error during run: %s\n\nRestarting in 30\n\n", err.Error())
				time.Sleep(30 * time.Second)
			}

		}
	}
	if mode == IEMArchive {
		if db.SurrealInit() != nil {
			log.Fatalf("Failed to connect to DB: %s", err.Error())
		}
		if err := RunIEMArchive(args[1:]); err != nil {
			log.Fatal(err)
		}
	}

}
