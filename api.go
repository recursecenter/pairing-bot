package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type RecurseAPI struct {
	rcAPIURL string
}

type RecurserProfile struct {
	Name    string
	ZulipId int64 `json:"zulip_id"`
}

func (ra *RecurseAPI) userIsCurrentlyAtRC(accessToken string, id int64) (bool, error) {
	// TODO: Unfortunately this is not a query parameter, but it could be.
	ids, err := ra.getCurrentlyActiveZulipIds(accessToken)

	return contains(ids, id), err
}

// The profile API endpoint is updated at midnight on the last day (Friday) of a batch.
func (ra *RecurseAPI) getCurrentlyActiveZulipIds(accessToken string) ([]int64, error) {
	var ids []int64
	offset := 0
	limit := 50
	apiHasMoreResults := true

	for apiHasMoreResults {
		idsStartingFromOffset, err := ra.getCurrentlyActiveZulipIdsWithOffset(accessToken, offset, limit)
		if err != nil {
			return nil, fmt.Errorf("while reading from offset %d: %w", offset, err)
		}

		ids = append(ids, idsStartingFromOffset...)

		log.Printf("The API returned %v profiles from the offset of %v", len(idsStartingFromOffset), offset)

		//The API limits respones to 50 total profiles. Keep querying the API until there are no more Recurser Profiles remaining
		if len(idsStartingFromOffset) == limit {
			apiHasMoreResults = true
			offset += limit

			log.Println("We reached the limit of results from the Profiles API and need to make another query")
		} else {
			apiHasMoreResults = false
		}
	}

	log.Println("The API returned this many TOTAL profiles", len(ids))
	log.Println("Here are the Zulip IDs of people currently at RC", ids)

	return ids, nil
}

/*
The RC API limits queries to the profiles endpoint to 50 results. However, there may be more than 50 people currently at RC.
The RC API takes in an "offset" query param that allows us to grab records beyond that limit of 50 results by performing multiple api calls.
*/
func (ra *RecurseAPI) getCurrentlyActiveZulipIdsWithOffset(accessToken string, offset int, limit int) ([]int64, error) {
	var ids []int64

	endpointString := fmt.Sprintf("/profiles?scope=current&offset=%v&limit=%v&role=recurser&access_token=%v", offset, limit, accessToken)

	resp, err := http.Get(ra.rcAPIURL + endpointString)
	if err != nil {
		return nil, fmt.Errorf("error while getting active RCers from the RC API: %w", err)
	}
	if resp.StatusCode >= 400 {
		err = fmt.Errorf("HTTP error while getting active RCers from the RC API: %s", resp.Status)
		log.Print(err)
	}

	defer resp.Body.Close()

	body, bodyErr := io.ReadAll(resp.Body)
	if bodyErr != nil {
		log.Printf("Unable to get the Zulip IDs of people currently at RC due to the following error: %s", err)
	}
	// Return the first error encountered: the HTTP status error, or the body error.
	// We've logged both of them in either case.
	if err != nil {
		return nil, err
	} else if bodyErr != nil {
		return nil, bodyErr
	}

	//Parse the json response from the API
	recursers := []RecurserProfile{}
	json.Unmarshal([]byte(body), &recursers)

	for i := range recursers {
		zid := recursers[i].ZulipId
		ids = append(ids, zid)
	}

	return ids, nil
}

func (ra *RecurseAPI) isSecondWeekOfBatch(accessToken string) bool {
	resp, err := http.Get(ra.rcAPIURL + "/batches?access_token=" + accessToken)
	if err != nil {
		log.Printf("Got the following error while getting the RC batches from the RC API: %s\n", err)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)

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
