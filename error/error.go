package error

import (
	"encoding/json"
	"net/http"
)

var errors map[int]string

const ()

func init() {
	errors = make(map[int]string)

	// command related errors
	errors[100] = "Key Not Found"
	errors[101] = "The given PrevValue is not equal to the value of the key"
	errors[102] = "Not A File"
	errors[103] = "Reached the max number of machines in the cluster"

	// Post form related errors
	errors[200] = "Value is Required in POST form"
	errors[201] = "PrevValue is Required in POST form"
	errors[202] = "The given TTL in POST form is not a number"
	errors[203] = "The given index in POST form is not a number"

	// raft related errors
	errors[300] = "Raft Internal Error"
	errors[301] = "During Leader Election"

	// keyword
	errors[400] = "The prefix of the given key is a keyword in etcd"

	// etcd related errors
	errors[500] = "watcher is cleared due to etcd recovery"

}

type Error struct {
	ErrorCode int    `json:"errorCode"`
	Message   string `json:"message"`
	Cause     string `json:"cause,omitempty"`
}

func NewError(errorCode int, cause string) Error {
	return Error{
		ErrorCode: errorCode,
		Message:   errors[errorCode],
		Cause:     cause,
	}
}

func Message(code int) string {
	return errors[code]
}

// Only for error interface
func (e Error) Error() string {
	return e.Message
}

func (e Error) toJsonString() string {
	b, _ := json.Marshal(e)
	return string(b)
}

func (e Error) Write(w http.ResponseWriter) {
	// 3xx is reft internal error
	if e.ErrorCode/100 == 3 {
		http.Error(w, e.toJsonString(), http.StatusInternalServerError)
	} else {
		http.Error(w, e.toJsonString(), http.StatusBadRequest)
	}
}
