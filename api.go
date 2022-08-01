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

//The profile API endpoint is updated at midnight on the last day (Friday) of a batch.
func (ra *RecurseAPI) getCurrentlyActiveEmails(accessToken string) []string {
	var emailsOfPeopleAtRC []string
	offset := 0
	limit := 25
	apiHasMoreResults := true

	for apiHasMoreResults {
		emailsStartingFromOffset := ra.getCurrentlyActiveEmailsWithOffset(accessToken, offset, limit)

		emailsOfPeopleAtRC = append(emailsOfPeopleAtRC, emailsStartingFromOffset...)

		log.Println("The API returned this many profiles from the offset", len(emailsStartingFromOffset))

		//The API limits respones to 50 total profiles. Keep querying the API until there are no more Recurser Profiles remaining
		if len(emailsStartingFromOffset) == limit {
			apiHasMoreResults = true
			offset += limit

			log.Println("We had more than limit results")
		} else {
			apiHasMoreResults = false
			log.Println("We did not have more than the limit results")
		}
	}

	log.Println("The API returned this many TOTAL profiles", len(emailsOfPeopleAtRC))

	return emailsOfPeopleAtRC
}

/*
	The RC API limits queries to the profiles endpoint to 50 results. However, there may be more than 50 people currently at RC.
	The RC API takes in an "offset" query param that allows us to grab records beyond that limit of 50 results by performing multiple api calls.
*/
func (ra *RecurseAPI) getCurrentlyActiveEmailsWithOffset(accessToken string, offset int, limit int) []string {
	var emailsOfPeopleAtRC []string

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
