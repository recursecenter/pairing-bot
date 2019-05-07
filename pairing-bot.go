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
	Data     string `json:"data"`
	Token    string `json:"token"`
	Trigger  string `json:"trigger"`
	Message  struct {
		SenderID    int `json:"sender_id"`
		SenderEmail int `json:"sender_email"`
	} `json:"message"`
}

type response struct {
	Message string `json:"content"`
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

func indexHandler(w http.ResponseWriter, req *http.Request) {
	var d data
	err := json.NewDecoder(req.Body).Decode(&d)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	log.Println("before test")
	log.Println(d.BotEmail, d.Data, d.Token, d.Trigger, d.Message.SenderID, d.Message.SenderEmail)
	log.Println("after test")

	r := response{`Hello human, please witness my generic response -_-`}
	b, err := json.Marshal(r)
	if err != nil {
		panic(err)
	}
	log.Println(b)
}
