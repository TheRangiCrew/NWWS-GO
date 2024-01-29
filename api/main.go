package main

import (
	"io"
	"net/http"
)

func first(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "Hello there")
}

func main() {
	http.HandleFunc("/", first)

	http.ListenAndServe(":3333", nil)
}
