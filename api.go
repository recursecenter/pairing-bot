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
	Email string
}

func (ra *RecurseAPI) userIsCurrentlyAtRC(accessToken string, email string) bool {
	emailsOfPeopleAtRC := ra.getCurrentlyActiveEmails(accessToken)

	return contains(emailsOfPeopleAtRC, email)
}

//The API endpoint this queries is updated at midnight on the last day (Friday) of a batch.
//Make sure to only query this endpoint after it has been updated
func (ra *RecurseAPI) getCurrentlyActiveEmails(accessToken string) []string {
	var emailsOfPeopleAtRC []string
	//TODO, batch the API call since the limit is 50 results per

	resp, err := http.Get(ra.rcAPIURL + "/profiles?scope=current&limit=50&role=recurser&access_token=" + accessToken)
	if err != nil {
		log.Printf("Got the following error while getting the RC batches from the RC API: %s\n", err)
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		log.Printf("Unable to get the emails of people currently at RC due to the following error: %s", err)
		return emailsOfPeopleAtRC
	}

	//Parse the json response from the API
	recursers := []RecurserProfile{}
	json.Unmarshal([]byte(body), &recursers)

	for i := range recursers {
		email := recursers[i].Email
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
	batchStartDate, _ := time.Parse(shortForm, batchStart)

	if err != nil {
		log.Printf("Unable to parse the batch start date of: %s. Exiting out of the welcome cron job.", batchStart)
		return false
	}

	todayDate := time.Now()
	hoursSinceStartOfBatch := todayDate.Sub(batchStartDate).Hours()

	log.Printf("Hours since start of the batch: %f", hoursSinceStartOfBatch)

	return (hoursSinceStartOfBatch > 168) && (hoursSinceStartOfBatch < 336)
}
