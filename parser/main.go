package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/TheRangiCrew/NWWS-GO/parser/db"
	"github.com/surrealdb/surrealdb.go/pkg/marshal"
)

const (
	PurgeTime time.Duration = time.Duration(30 * time.Minute)
)

type pendingProduct struct {
	ID        string    `json:"id"`
	Received  time.Time `json:"received_at"`
	Text      string    `json:"text"`
	Processed time.Time `json:"processed_at,omitempty"`
	Error     string    `json:"error,omitempty"`
}

func runLatestParser() error {

	pending, err := marshal.SmartUnmarshal[pendingProduct](db.Surreal().Query("SELECT * FROM pending_text_products WHERE processed_at == NONE && error == NONE", map[string]string{}))
	if err != nil {
		return err
	}
	fmt.Printf("Found %d products pending\n", len(pending))

	for _, product := range pending {
		errChan := make(chan error)

		go func(text string) {
			errChan <- Processor(text)
		}(product.Text)

		err = <-errChan
		if err != nil {
			product.Processed = time.Now()
			product.Error = err.Error()
			products, er := marshal.SmartUnmarshal[pendingProduct](marshal.SmartMarshal(db.Surreal().Update, product))
			if er != nil {
				log.Println(er)
			} else {
				product = products[0]
				log.Printf("Error on %s: %s\n", product.ID, err)
			}
		} else {
			db.Surreal().Delete(product.ID)
		}
	}

	liveQuery, err := db.Surreal().Live("pending_text_products", false)
	if err != nil {
		return err
	}

	notifications, er := db.Surreal().LiveNotifications(liveQuery)
	if er != nil {
		panic(er)
	}

	fmt.Println("Listening for products")

	go func() {
		for notification := range notifications {
			// Handle each incoming notification
			var product pendingProduct
			err := marshal.Unmarshal(notification.Result, &product)
			if err != nil {
				panic(err)
			}
			errChan := make(chan error)

			go func(text string) {
				errChan <- Processor(text)
			}(product.Text)

			err = <-errChan
			if err != nil {
				product.Processed = time.Now()
				product.Error = err.Error()
				products, er := marshal.SmartUnmarshal[pendingProduct](marshal.SmartMarshal(db.Surreal().Update, product))
				if er != nil {
					log.Println(er)
				} else {
					product = products[0]
					log.Printf("Error on %s: %s\n", product.ID, err)
				}
			} else {
				db.Surreal().Delete(product.ID)
			}
		}
	}()

	select {}

	// products, err := getProducts()
	// if err != nil {
	// 	return err
	// }

	// log.Printf("Found %d products in directory", len(products))

	// for {
	// 	if len(products) == 0 {
	// 		time.Sleep(1 * time.Second)
	// 		products, err = getProducts()
	// 	} else {
	// 		log.Printf("Found %d products in directory", len(products))
	// 		if err = Processor((products)[0].Text); err != nil {
	// 			log.Println(err.Error())
	// 			name := time.Now().UTC().Format("2006_01_02_15_04_05_") + (products)[0].Name
	// 			log.Println("Moving to error dump as " + name)
	// 			err = moveToErrorDump(name, (products)[0].Text)
	// 			if err != nil {
	// 				return err
	// 			}
	// 		}
	// 		err = os.Remove(productQueueDirectory + (products)[0].Name)
	// 		if err != nil {
	// 			return err
	// 		}
	// 		products = products[1:]
	// 	}

	// 	if err != nil {
	// 		return err
	// 	}
	// }

	return nil
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

	if mode == Live {
		for db.SurrealInit() != nil {
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
	if mode == IEMArchive {
		if db.SurrealInit() != nil {
			log.Fatalf("Failed to connect to DB: %s", err.Error())
		}
		if err := RunIEMArchive(args[1:]); err != nil {
			log.Fatal(err)
		}
	}

}
