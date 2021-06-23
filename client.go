package main

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// This is a struct that gets only what
// we need from the incoming JSON payload
type incomingJSON struct {
	Data    string `json:"data"`
	Token   string `json:"token"`
	Trigger string `json:"trigger"`
	Message struct {
		SenderID         int         `json:"sender_id"`
		DisplayRecipient interface{} `json:"display_recipient"`
		SenderEmail      string      `json:"sender_email"`
		SenderFullName   string      `json:"sender_full_name"`
	} `json:"message"`
}

type UserDataFromJSON struct {
	userID    string
	userEmail string
	userName  string
}

// Zulip has to get JSON back from the bot,
// this does that. An empty message field stops
// zulip from throwing an error at the user that
// messaged the bot, but doesn't send a response
type botResponse struct {
	Message string `json:"content"`
}

type botNoResponse struct {
	Message bool `json:"response_not_required"`
}

type userRequest interface {
	validateJSON(r *http.Request) error
	validateAuthCreds(tokenFromDB string) bool
	validateInteractionType() *botResponse
	ignoreInteractionType() *botNoResponse
	sanitizeUserInput() (string, []string, error)
	extractUserData() *UserDataFromJSON // does this need an error return value? anything that hasn't been validated previously?
}

type userNotification interface {
	sendUserMessage(ctx context.Context, botPassword, user, message string) error
}

// implements userRequest
type zulipUserRequest struct {
	json incomingJSON
}

// implements userNotification
type zulipUserNotification struct {
	botUsername string
	zulipAPIURL string
}

func (zun *zulipUserNotification) sendUserMessage(ctx context.Context, botPassword, user, message string) error {

	zulipClient := &http.Client{}
	messageRequest := url.Values{}
	messageRequest.Add("type", "private")
	messageRequest.Add("to", user)
	messageRequest.Add("content", message)

	req, err := http.NewRequestWithContext(ctx, "POST", zun.zulipAPIURL, strings.NewReader(messageRequest.Encode()))
	if err != nil {
		return err
	}
	req.SetBasicAuth(zun.botUsername, botPassword)
	req.Header.Set("content-type", "application/x-www-form-urlencoded")

	resp, err := zulipClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBodyText, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	log.Println(string(respBodyText))

	return nil
}

func (zur *zulipUserRequest) validateJSON(r *http.Request) error {
	var userReq incomingJSON
	// Look at the incoming webhook and slurp up the JSON
	// Error if the JSON from Zulip itself is bad
	err := json.NewDecoder(r.Body).Decode(&userReq)

	if err == nil {
		zur.json = userReq
	}
	return err
}

func (zur *zulipUserRequest) validateAuthCreds(tokenFromDB string) bool {
	if zur.json.Token != tokenFromDB {
		log.Println("Unauthorized interaction attempt")
		return false
	}
	return true
}

// if the zulip msg is posted in a stream, don't treat it as a command
func (zur *zulipUserRequest) validateInteractionType() *botResponse {
	if zur.json.Trigger != "private_message" {
		return &botResponse{"Hi! I'm Pairing Bot (she/her)!\n\nSend me a PM that says `subscribe` to get started :smiley:\n\n:pear::robot:\n:octopus::octopus:"}
	}
	return nil
}

// if there aren't two 'recipients' (one sender and one receiver),
// then don't respond. this stops pairing bot from responding in the group
// chat she starts when she matches people
func (zur *zulipUserRequest) ignoreInteractionType() *botNoResponse {
	if len(zur.json.Message.DisplayRecipient.([]interface{})) != 2 {
		return &botNoResponse{true}
	}
	return nil
}

func (zur *zulipUserRequest) sanitizeUserInput() (string, []string, error) {
	return parseCmd(zur.json.Data)
}

func (zur *zulipUserRequest) extractUserData() *UserDataFromJSON {
	return &UserDataFromJSON{
		userID:    strconv.Itoa(zur.json.Message.SenderID),
		userEmail: zur.json.Message.SenderEmail,
		userName:  zur.json.Message.SenderFullName,
	}
}

// Mock types

// implements userRequest
type mockUserRequest struct {
}

// implements userNotification
type mockUserNotification struct {
}

func (mun *mockUserNotification) sendUserMessage(ctx context.Context, botPassword, user, message string) error {
	return nil
}

func (mur *mockUserRequest) validateJSON(r *http.Request) error {
	return nil
}

func (mur *mockUserRequest) validateAuthCreds(tokenFromDB string) bool {
	return false
}

func (mur *mockUserRequest) validateInteractionType() *botResponse {
	return nil
}

func (mur *mockUserRequest) ignoreInteractionType() *botNoResponse {
	return nil
}

func (mur *mockUserRequest) sanitizeUserInput() (string, []string, error) {
	return "", nil, nil
}

func (mur *mockUserRequest) extractUserData() *UserDataFromJSON {
	return &UserDataFromJSON{}
}
