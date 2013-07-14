package main

import (
	"encoding/json"
)

var errors map[int]string

func init() {
	errors = make(map[int]string)

	// command related errors
	errors[100] = "Key Not Found"
	errors[101] = "The given PrevValue is not equal to the value of the key"
	errors[102] = "Not A File"
	// Post form related errors
	errors[200] = "Value is Required in POST form"
	errors[201] = "PrevValue is Required in POST form"
	errors[202] = "The given TTL in POST form is not a number"
	errors[203] = "The given index in POST form is not a number"
	// raft related errors
	errors[300] = "Raft Internal Error"
	errors[301] = "During Leader Election"
}

type jsonError struct {
	ErrorCode int    `json:"errorCode"`
	Message   string `json:"message"`
	Cause     string `json:"cause,omitempty"`
}

func newJsonError(errorCode int, cause string) []byte {
	b, _ := json.Marshal(jsonError{
		ErrorCode: errorCode,
		Message:   errors[errorCode],
		Cause:     cause,
	})
	return b
}
