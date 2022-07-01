package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
)

type RecurseAPI struct {
	rcAPIURL string
}

type RecurserProfile struct {
	Stints []Stint
}

type Stint struct {
	In_progress bool
}

func (ra *RecurseAPI) userIsCurrentlyAtRC(accessToken string, email string) bool {
	currentlyAtRC := false

	resp, err := http.Get(ra.rcAPIURL + "/profiles/" + email + "?access_token=" + accessToken)

	if err != nil {
		log.Printf("Got the following error while checking if the user was active at RC: %s\n", err)
		return currentlyAtRC
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)

	//Parse the json response from the API
	profile := RecurserProfile{}
	json.Unmarshal([]byte(body), &profile)

	log.Println("Recurser Profile: ", profile)

	currentlyAtRC = profile.Stints[0].In_progress

	log.Println("in_progress:", currentlyAtRC)

	return currentlyAtRC
}

func (ra *RecurseAPI) getCurrentlyActiveEmails(accessToken string) []string {
	//TODO, batch the API call since the limit is 50 results per

	resp, err := http.Get(ra.rcAPIURL + "/profiles?scope=current&limit=50&role=recurser&access_token=" + accessToken)
	if err != nil {
		log.Printf("Got the following error while getting the RC batches from the RC API: %s\n", err)
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)

	//Parse the json response from the API
	var recursers []map[string]interface{}
	json.Unmarshal([]byte(body), &recursers)

	var emailsOfPeopleAtRC []string

	for i := range recursers {
		email := recursers[i]["email"].(string)
		emailsOfPeopleAtRC = append(emailsOfPeopleAtRC, email)
	}

	return emailsOfPeopleAtRC
}

func (ra *RecurseAPI) isSecondWeekOfBatch(accessToken string) bool {
	resp, err := http.Get(ra.rcAPIURL + "/batches?access_token=" + accessToken)
	if err != nil {
		log.Printf("Got the following error while getting the RC batches from the RC API: %s\n", err)
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)

	//Parse the json response from the API
	var batches []map[string]interface{}
	json.Unmarshal([]byte(body), &batches)

	var batchStart string

	//Loop through the batches until we find the first non-mini batch. Mini batches are only 1 week long, so it doesn't make sense to send a message
	//1 week after a mini batch has started :joy:
	for i := range batches {
		if !strings.HasPrefix(batches[i]["name"].(string), "Mini") {
			batchStart = batches[i]["start_date"].(string)

			break
		}
	}

	//Convert strings of the form "YYYY-MM-DD" into time objects that Go can perform mathematical operations with
	const shortForm = "2006-01-02"

	todayDate := time.Now()
	batchStartDate, _ := time.Parse(shortForm, batchStart)

	if err != nil {
		log.Printf("Unable to parse the batch start date of: %s. Exiting out of the welcome cron job.", batchStart)
		return false
	}

	hoursSinceStartOfBatch := todayDate.Sub(batchStartDate).Hours()

	log.Printf("Hours since start of the batch: %f", hoursSinceStartOfBatch)

	//Has 1 week (168 hours) passed since the start of the batch?
	return hoursSinceStartOfBatch > 168
