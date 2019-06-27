package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
)

const key = ""
const baseURL = "https://recurse.com/api/v1/"
const layout = "2006-01-02"

type Batch struct {
	ID        int       `json:"id"`
	StartDate time.Time `json:"start_date"`
}

type Profile struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Start work on talking to the RC API
func main() {

	respBody := makeAPICall("batches")

	var batches []Batch
	if err := json.Unmarshal(respBody, &batches); err != nil {
		log.Fatal(err)
	}

	//Made the below to test
	//sampleDate := "2019-05-20"
	//currDate, _ := time.Parse(layout, sampleDate)

	var currBatches []int
	currDate := time.Now()
	for _, batch := range batches {
		if batch.StartDate == currDate {
			currBatches = append(currBatches, batch.ID)
		} else {
			break
		}
	}

	var profiles []Profile
	for _, batchID := range currBatches {
		var partialProfiles []Profile
		respBody := makeAPICall("profiles", "batch_id="+fmt.Sprint(batchID))
		if err := json.Unmarshal(respBody, &partialProfiles); err != nil {
			log.Fatal(err)
		}
		profiles = append(profiles, partialProfiles...)
	}

}

func makeAPICall(args ...string) []byte {

	//TODO: not best way to concatenate strings; use strings.Builder instead
	url := baseURL + args[0] + "?access_token=" + key + "&" + strings.Join(args[1:], "&")
	resp, err := http.Get(url)

	if err != nil {
		log.Fatal(err)
	}

	respBody, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		log.Fatal(err)
	}

	return respBody
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
