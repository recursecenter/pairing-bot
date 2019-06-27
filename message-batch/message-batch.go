package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

const key = ""
const layout = "2006-01-02"

type Batch struct {
	ID        int       `json:"id"`
	StartDate time.Time `json:"start_date"`
}

// Start work on talking to the RC API
func main() {
	resp, err := http.Get("https://recurse.com/api/v1/batches?access_token=" + key)
	if err != nil {
		log.Fatal(err)
	}

	respBody, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		log.Fatal(err)
	}

	var batches []Batch
	if err := json.Unmarshal(respBody, &batches); err != nil {
		log.Fatal(err)
	}

	fmt.Println(batches)

}

func (batch *Batch) UnmarshalJSON(data []byte) error {
	type Alias Batch
	aux := &struct {
		StartDate string `json:"start_date"`
		*Alias
	}{
		Alias: (*Alias)(batch),
	}

	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	val, err := time.Parse(layout, aux.StartDate)
	batch.StartDate = val

	return err
}
