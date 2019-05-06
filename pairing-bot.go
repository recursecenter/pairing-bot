package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

type data struct {
	All string
}

func main() {
	http.HandleFunc("/webhooks", indexHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("Defaulting to port %s", port)
	}

	log.Printf("Listening on port %s", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	/*
		if r.URL.Path != "/webhooks" {
			http.NotFound(w, r)
			return
		}
	*/
	//fmt.Fprint(w, `Hello :)`)

	//log.Println(r.Body)
	decoder := json.NewDecoder(r.Body)
	var d data
	err := decoder.Decode(&d)
	if err != nil {
		panic(err)
	}
	//log.Println("before test")
	fmt.Println(d.All)
	//log.Println("after test")
}
