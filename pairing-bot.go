package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

type zulipIncHook struct {
	BotEmail string `json:"bot_email"`
	Data     string `json:"data"`
	Token    string `json:"token"`
	Trigger  string `json:"trigger"`
	Message  struct {
		SenderID    int    `json:"sender_id"`
		SenderEmail string `json:"sender_email"`
	} `json:"message"`
}

type botResponse struct {
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

func indexHandler(w http.ResponseWriter, r *http.Request) {
	var userReq zulipIncHook
	err := json.NewDecoder(r.Body).Decode(&userReq)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	log.Println("before test")
	log.Println(userReq)
	log.Println("after test")

	res := botResponse{`Hello human, please witness my generic response -_-`}
	err = json.NewEncoder(w).Encode(res)
	if err != nil {
		log.Println(err)
	}
}
