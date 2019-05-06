package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

type data struct {
	BotEmail string `json:"bot_email"`
	Message  string `json:"data"`
	Token    string `json:"token"`
	Trigger  string `json:"trigger"`
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
	var d data
	err := json.NewDecoder(r.Body).Decode(&d)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	log.Println("before test")
	log.Println(d.BotEmail, d.Message, d.Token, d.Trigger)
	log.Println("after test")
}
