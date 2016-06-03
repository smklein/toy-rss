package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

func handler(w http.ResponseWriter, r *http.Request) {
	fileName := "test_files/" + r.URL.Path[1:]
	log.Println("Accessing file: ", fileName)
	body, err := ioutil.ReadFile(fileName)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Fprintf(w, "%s", body)
}

func main() {
	log.Println("Test server started")

	http.HandleFunc("/", handler)
	http.ListenAndServe(":8080", nil)
}
