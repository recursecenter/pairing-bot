package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

const key = "FAKE_KEY"

type batch struct {
	ID        int    `json:"id"`
	StartDate string `json:"start_date"`
}

// Start work on talking to the RC API
func main() {
	resp, err := http.Get("https://recurse.com/api/v1/batches?access_token=" + key)
	if err != nil {
		log.Fatal(err)
	}
	robots, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		log.Fatal(err)
	}
	var batches []batch
	err = json.Unmarshal(robots, &batches)
	//fmt.Println(batches, err)
	layout := "2006-01-02"
	t, err := time.Parse(layout, batches[0].StartDate)
	fmt.Println(t, err)
	batchYear, batchMonth, batchDay := t.Date()
	thisYear, thisMonth, thisDay := time.Now().Date()
	isSame := batchYear == thisYear && batchMonth == thisMonth && batchDay == thisDay
	fmt.Println(isSame)
}
