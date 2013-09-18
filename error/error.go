package error

import (
	"encoding/json"
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

	EcodeValueRequired     = 200
	EcodePrevValueRequired = 201
	EcodeTTLNaN            = 202
	EcodeIndexNaN          = 203

	EcodeRaftInternal = 300
	EcodeLeaderElect  = 301

	EcodeWatcherCleared    = 400
	EcodeEventIndexCleared = 401
)

func init() {
	errors = make(map[int]string)

	// command related errors
	errors[100] = "Key Not Found"
	errors[101] = "Test Failed" //test and set
	errors[102] = "Not A File"
	errors[103] = "Reached the max number of machines in the cluster"
	errors[104] = "Not A Directory"
	errors[105] = "Already exists" // create
	errors[106] = "The prefix of given key is a keyword in etcd"

	// Post form related errors
	errors[200] = "Value is Required in POST form"
	errors[201] = "PrevValue is Required in POST form"
	errors[202] = "The given TTL in POST form is not a number"
	errors[203] = "The given index in POST form is not a number"

	// raft related errors
	errors[300] = "Raft Internal Error"
	errors[301] = "During Leader Election"

	// etcd related errors
	errors[400] = "watcher is cleared due to etcd recovery"
	errors[401] = "The event in requested index is outdated and cleared"

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
