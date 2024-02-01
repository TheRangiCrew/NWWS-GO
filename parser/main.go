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

var productQueueDirectory string = os.Getenv("PRODUCT_QUEUE_DIR")
var errorDumpDirectory string = os.Getenv("PRODUCT_ERROR_DIR")

type FileProduct struct {
	Name  string
	Index int
	Text  string
}

type Directory struct {
	Name        string
	Time        time.Time
	Products    *[]FileProduct
	LastProduct time.Time
}

func getProducts(d Directory) error {
	path := productQueueDirectory + d.Name + "/"
	productDir, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	products := []FileProduct{}
	for _, d := range productDir {
		if !d.IsDir() {
			index, err := strconv.Atoi(strings.Split(d.Name(), ".")[0])
			if err != nil {
				return err
			}
			file, err := os.ReadFile(path + d.Name())
			if err != nil {
				return err
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

	if len(*d.Products) != 0 {
		d.LastProduct = time.Now().UTC()
	}

	*d.Products = products

	return nil
}

func getDirectories() ([]Directory, error) {
	dirs, err := os.ReadDir(productQueueDirectory)
	if err != nil {
		if err == os.ErrNotExist {
			return nil, errors.New("product queue directory does not exist. Make sure the XMPP server is running")
		}
		return nil, err
	}

	directories := []Directory{}

	for _, dir := range dirs {
		name := dir.Name()

		if len(name) != 8 {
			log.Println("Invalid directory name " + name)
			continue
		}

		year, err := strconv.Atoi(name[:4])
		if err != nil {
			log.Println(err.Error())
			continue
		}
		monthNum, err := strconv.Atoi(name[4:6])
		if err != nil {
			log.Println(err.Error())
			continue
		}
		month := time.Month(monthNum)
		day, err := strconv.Atoi(name[6:8])
		if err != nil {
			log.Println(err.Error())
			continue
		}

		t := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)

		d := Directory{
			Name:        name,
			Time:        t,
			Products:    &[]FileProduct{},
			LastProduct: time.Now().UTC(),
		}

		err = getProducts(d)
		if err != nil {
			return nil, err
		}

		directories = append(directories, d)
	}

	sort.Slice(directories, func(i, j int) bool {
		return directories[i].Time.After(directories[j].Time)
	})

	return directories, nil

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

	dirs, err := getDirectories()
	if err != nil {
		return err
	}

	d := dirs[0]

	log.Printf("Found %d products in directory with time %s", len(*d.Products), d.Time.Format("02/01/2006"))

	for {
		if len(*d.Products) == 0 {
			log.Println(time.Now().UTC().Sub(d.Time))
			if time.Now().UTC().Sub(d.Time) > (24 * time.Hour) {
				log.Println("Day has passed. Moving on...")
				err = os.Remove(productQueueDirectory + d.Name)
				if err != nil {
					return err
				}
				dirs, err := getDirectories()
				if err != nil {
					return err
				}

				d = dirs[0]
				log.Printf("Found %d products in directory with time %s", len(*d.Products), d.Time.Format("02/01/2006"))
				continue
			}
			time.Sleep(1 * time.Second)
		} else {
			if err = Processor((*d.Products)[0].Text); err != nil {
				log.Println(err.Error())
				name := time.Now().UTC().Format("2006_01_02_15_04_05_") + (*d.Products)[0].Name
				log.Println("Moving to error dump as " + name)
				err = moveToErrorDump(name, (*d.Products)[0].Text)
				if err != nil {
					return err
				}
			}
			err = os.Remove(productQueueDirectory + d.Name + "/" + (*d.Products)[0].Name)
			if err != nil {
				return err
			}
		}

		if err = getProducts(d); err != nil {
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

	// err := godotenv.Load(".env")
	// if err != nil {
	// 	log.Fatal("Error loading .env file")
	// }

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
