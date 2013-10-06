package error

import (
	"encoding/json"
	"fmt"
	"net/http"
)

var errors map[int]string

const (
	EcodeKeyNotFound    = 100
	EcodeTestFailed     = 101
	EcodeNotFile        = 102
	EcodeNoMoreMachine  = 103
	EcodeNotDir         = 104
	EcodeNodeExist      = 105
	EcodeKeyIsPreserved = 106

	EcodeValueRequired      = 200
	EcodePrevValueRequired  = 201
	EcodeTTLNaN             = 202
	EcodeIndexNaN           = 203
	EcodeValueOrTTLRequired = 204

	EcodeRaftInternal = 300
	EcodeLeaderElect  = 301

	EcodeWatcherCleared    = 400
	EcodeEventIndexCleared = 401
)

func init() {
	errors = make(map[int]string)

	// command related errors
	errors[EcodeKeyNotFound] = "Key Not Found"
	errors[EcodeTestFailed] = "Test Failed" //test and set
	errors[EcodeNotFile] = "Not A File"
	errors[EcodeNoMoreMachine] = "Reached the max number of machines in the cluster"
	errors[EcodeNotDir] = "Not A Directory"
	errors[EcodeNodeExist] = "Already exists" // create
	errors[EcodeKeyIsPreserved] = "The prefix of given key is a keyword in etcd"

	// Post form related errors
	errors[EcodeValueRequired] = "Value is Required in POST form"
	errors[EcodePrevValueRequired] = "PrevValue is Required in POST form"
	errors[EcodeTTLNaN] = "The given TTL in POST form is not a number"
	errors[EcodeIndexNaN] = "The given index in POST form is not a number"
	errors[EcodeValueOrTTLRequired] = "Value or TTL is required in POST form"

	// raft related errors
	errors[EcodeRaftInternal] = "Raft Internal Error"
	errors[EcodeLeaderElect] = "During Leader Election"

	// etcd related errors
	errors[EcodeWatcherCleared] = "watcher is cleared due to etcd recovery"
	errors[EcodeEventIndexCleared] = "The event in requested index is outdated and cleared"

}

type Error struct {
	ErrorCode int    `json:"errorCode"`
	Message   string `json:"message"`
	Cause     string `json:"cause,omitempty"`
	Index     uint64 `json:"index"`
	Term      uint64 `json:"term"`
}

func NewError(errorCode int, cause string, index uint64, term uint64) *Error {
	return &Error{
		ErrorCode: errorCode,
		Message:   errors[errorCode],
		Cause:     cause,
		Index:     index,
		Term:      term,
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
	w.Header().Add("X-Etcd-Index", fmt.Sprint(e.Index))
	w.Header().Add("X-Etcd-Term", fmt.Sprint(e.Term))
	// 3xx is reft internal error
	if e.ErrorCode/100 == 3 {
		http.Error(w, e.toJsonString(), http.StatusInternalServerError)
	} else {
		http.Error(w, e.toJsonString(), http.StatusBadRequest)
	}
}
