package internal

import (
	"os"
	"sync"

	"github.com/surrealdb/surrealdb.go"
)

var surrealLock = &sync.Mutex{}

var surreal *surrealdb.DB

func Surreal() *surrealdb.DB {
	surrealLock.Lock()
	defer surrealLock.Unlock()

	if surreal == nil {

		url := os.Getenv("SURREAL_URL")
		username := os.Getenv("SURREAL_USERNAME")
		password := os.Getenv("SURREAL_PASSWORD")
		database := os.Getenv("SURREAL_DATABASE")
		namespace := os.Getenv("SURREAL_NAMESPACE")

		db, err := surrealdb.New(url)
		if err != nil {
			panic(err)
		}

		if _, err = db.Use(namespace, database); err != nil {
			panic(err)
		}

		if _, err = db.Signin(map[string]interface{}{
			"user": username,
			"pass": password,
			"NS":   namespace,
		}); err != nil {
			panic(err)
		}

		surreal = db
	}

	return surreal
}
