package main

import (
	"encoding/json"
)

type jsonError struct {
	StatusCode string `json:"statusCode"`
	Message    string `json:"message"`
}

func newJsonError(statusCode string, message string) []byte {
	b, _ := json.Marshal(jsonError{
		StatusCode: "400",
		Message:    "Set: Value Required",
	})
	return b
}
