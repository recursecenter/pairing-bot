package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// This is a struct that gets only what
// we need from the incoming JSON payload
type incomingJSON struct {
	Data    string `json:"data"`
	Token   string `json:"token"`
	Trigger string `json:"trigger"`
	Message struct {
		SenderID         int64       `json:"sender_id"`
		DisplayRecipient interface{} `json:"display_recipient"`
		SenderEmail      string      `json:"sender_email"`
		SenderFullName   string      `json:"sender_full_name"`
	} `json:"message"`
}

type UserDataFromJSON struct {
	userID    int64
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
	getCommandString() string
	sanitizeUserInput() (string, []string, error)
	extractUserData() *UserDataFromJSON // does this need an error return value? anything that hasn't been validated previously?
}

type userNotification interface {
	sendUserMessage(ctx context.Context, botPassword string, userIDs []int64, message string) error
}

type streamMessage interface {
	postToTopic(ctx context.Context, botPassword, message string, stream string, topic string) error
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

// implements streamMessage
type zulipStreamMessage struct {
	botUsername string
	zulipAPIURL string
}

func (zsm *zulipStreamMessage) postToTopic(ctx context.Context, botPassword, message string, stream string, topic string) error {
	appEnv := os.Getenv("APP_ENV")

	if appEnv != "production" {
		log.Println("In the Prod environment Pairing Bot would have posted the following message: ", message)
		return nil
	}

	zulipClient := &http.Client{}
	messageRequest := url.Values{}

	messageRequest.Add("type", "stream")
	messageRequest.Add("to", stream)
	messageRequest.Add("topic", topic)
	messageRequest.Add("content", message)

	req, err := http.NewRequestWithContext(ctx, "POST", zsm.zulipAPIURL, strings.NewReader(messageRequest.Encode()))
	if err != nil {
		return err
	}
	req.SetBasicAuth(zsm.botUsername, botPassword)
	req.Header.Set("content-type", "application/x-www-form-urlencoded")

	resp, err := zulipClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	log.Printf("zulip response: %d %s\n", resp.StatusCode, string(respBodyText))
	if resp.StatusCode >= 400 {
		return fmt.Errorf("error response from Zulip: status: %s", resp.Status)
	}

	return nil
}

func (zun *zulipUserNotification) sendUserMessage(ctx context.Context, botPassword string, userIDs []int64, message string) error {

	zulipClient := &http.Client{}
	messageRequest := url.Values{}
	messageRequest.Add("type", "private")
	users := []string{}
	for _, id := range userIDs {
		users = append(users, fmt.Sprint(id))
	}
	messageRequest.Add("to", strings.Join(users, ","))
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
	respBodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	log.Printf("zulip response: %d %s\n", resp.StatusCode, string(respBodyText))
	if resp.StatusCode >= 400 {
		return fmt.Errorf("error response from Zulip: status: %s", resp.Status)
	}

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
	if zur.json.Trigger != "direct_message" {
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

func (zur *zulipUserRequest) getCommandString() string {
	return zur.json.Data
}

func (zur *zulipUserRequest) extractUserData() *UserDataFromJSON {
	return &UserDataFromJSON{
		userID:    int64(zur.json.Message.SenderID),
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

func (mun *mockUserNotification) sendUserMessage(ctx context.Context, botPassword string, userIDs []int64, message string) error {
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
