package main

import (
	"context"
	"net/http"
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
	validateJSON(ctx context.Context, r *http.Request) error
	validateAuthCreds(ctx context.Context, tokenFromDB string) bool
	validateInteractionType(ctx context.Context) *botResponse
	ignoreInteractionType(ctx context.Context) *botNoResponse
	sanitizeUserInput(ctx context.Context) (string, []string, error)
	extractUserData(ctx context.Context) *UserDataFromJSON // does this need an error return value? anything that hasn't been validated previously?
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
	return nil
}

func (zur *zulipUserRequest) validateJSON(ctx context.Context, r *http.Request) error {
	return nil
}

func (zur *zulipUserRequest) validateAuthCreds(ctx context.Context, tokenFromDB string) bool {
	return false
}

// if the zulip msg is posted in a stream, don't treat it as a command
func (zur *zulipUserRequest) validateInteractionType(ctx context.Context) *botResponse {
	return nil
}

// if there aren't two 'recipients' (one sender and one receiver),
// then don't respond. this stops pairing bot from responding in the group
// chat she starts when she matches people
func (zur *zulipUserRequest) ignoreInteractionType(ctx context.Context) *botNoResponse {
	return nil
}

func (zur *zulipUserRequest) sanitizeUserInput(ctx context.Context) (string, []string, error) {
	return "", nil, nil
}

func (zur *zulipUserRequest) extractUserData(ctx context.Context) *UserDataFromJSON {
	return &UserDataFromJSON{}
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

func (mur *mockUserRequest) validateJSON(ctx context.Context, r *http.Request) error {
	return nil
}

func (mur *mockUserRequest) validateAuthCreds(ctx context.Context, tokenFromDB string) bool {
	return false
}

func (mur *mockUserRequest) validateInteractionType(ctx context.Context) *botResponse {
	return nil
}

func (mur *mockUserRequest) ignoreInteractionType(ctx context.Context) *botNoResponse {
	return nil
}

func (mur *mockUserRequest) sanitizeUserInput(ctx context.Context) (string, []string, error) {
	return "", nil, nil
}

func (mur *mockUserRequest) extractUserData(ctx context.Context) *UserDataFromJSON {
	return &UserDataFromJSON{}
}
