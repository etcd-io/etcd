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
		StatusCode: statusCode,
		Message:    message,
	})
	return b
}
