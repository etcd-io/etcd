/*
   Copyright 2014 CoreOS, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package error

import (
	"encoding/json"
	"fmt"
	"net/http"
)

var errors = map[int]string{
	// command related errors
	EcodeKeyNotFound:      "Key not found",
	EcodeTestFailed:       "Compare failed", //test and set
	EcodeNotFile:          "Not a file",
	EcodeNoMorePeer:       "Reached the max number of peers in the cluster",
	EcodeNotDir:           "Not a directory",
	EcodeNodeExist:        "Key already exists", // create
	EcodeRootROnly:        "Root is read only",
	EcodeKeyIsPreserved:   "The prefix of given key is a keyword in etcd",
	EcodeDirNotEmpty:      "Directory not empty",
	EcodeExistingPeerAddr: "Peer address has existed",

	// Post form related errors
	EcodeValueRequired:        "Value is Required in POST form",
	EcodePrevValueRequired:    "PrevValue is Required in POST form",
	EcodeTTLNaN:               "The given TTL in POST form is not a number",
	EcodeIndexNaN:             "The given index in POST form is not a number",
	EcodeValueOrTTLRequired:   "Value or TTL is required in POST form",
	EcodeTimeoutNaN:           "The given timeout in POST form is not a number",
	EcodeNameRequired:         "Name is required in POST form",
	EcodeIndexOrValueRequired: "Index or value is required",
	EcodeIndexValueMutex:      "Index and value cannot both be specified",
	EcodeInvalidField:         "Invalid field",
	EcodeInvalidForm:          "Invalid POST form",

	// raft related errors
	EcodeRaftInternal: "Raft Internal Error",
	EcodeLeaderElect:  "During Leader Election",

	// etcd related errors
	EcodeWatcherCleared:     "watcher is cleared due to etcd recovery",
	EcodeEventIndexCleared:  "The event in requested index is outdated and cleared",
	EcodeStandbyInternal:    "Standby Internal Error",
	EcodeInvalidActiveSize:  "Invalid active size",
	EcodeInvalidRemoveDelay: "Standby remove delay",

	// client related errors
	EcodeClientInternal: "Client Internal Error",
}

var errorStatus = map[int]int{
	EcodeKeyNotFound:  http.StatusNotFound,
	EcodeNotFile:      http.StatusForbidden,
	EcodeDirNotEmpty:  http.StatusForbidden,
	EcodeTestFailed:   http.StatusPreconditionFailed,
	EcodeNodeExist:    http.StatusPreconditionFailed,
	EcodeRaftInternal: http.StatusInternalServerError,
	EcodeLeaderElect:  http.StatusInternalServerError,
}

const (
	EcodeKeyNotFound      = 100
	EcodeTestFailed       = 101
	EcodeNotFile          = 102
	EcodeNoMorePeer       = 103
	EcodeNotDir           = 104
	EcodeNodeExist        = 105
	EcodeKeyIsPreserved   = 106
	EcodeRootROnly        = 107
	EcodeDirNotEmpty      = 108
	EcodeExistingPeerAddr = 109

	EcodeValueRequired        = 200
	EcodePrevValueRequired    = 201
	EcodeTTLNaN               = 202
	EcodeIndexNaN             = 203
	EcodeValueOrTTLRequired   = 204
	EcodeTimeoutNaN           = 205
	EcodeNameRequired         = 206
	EcodeIndexOrValueRequired = 207
	EcodeIndexValueMutex      = 208
	EcodeInvalidField         = 209
	EcodeInvalidForm          = 210

	EcodeRaftInternal = 300
	EcodeLeaderElect  = 301

	EcodeWatcherCleared     = 400
	EcodeEventIndexCleared  = 401
	EcodeStandbyInternal    = 402
	EcodeInvalidActiveSize  = 403
	EcodeInvalidRemoveDelay = 404

	EcodeClientInternal = 500
)

type Error struct {
	ErrorCode int    `json:"errorCode"`
	Message   string `json:"message"`
	Cause     string `json:"cause,omitempty"`
	Index     uint64 `json:"index"`
}

func NewRequestError(errorCode int, cause string) *Error {
	return NewError(errorCode, cause, 0)
}

func NewError(errorCode int, cause string, index uint64) *Error {
	return &Error{
		ErrorCode: errorCode,
		Message:   errors[errorCode],
		Cause:     cause,
		Index:     index,
	}
}

func Message(code int) string {
	return errors[code]
}

// Only for error interface
func (e Error) Error() string {
	return e.Message + " (" + e.Cause + ")"
}

func (e Error) toJsonString() string {
	b, _ := json.Marshal(e)
	return string(b)
}

func (e Error) statusCode() int {
	status, ok := errorStatus[e.ErrorCode]
	if !ok {
		status = http.StatusBadRequest
	}
	return status
}

func (e Error) WriteTo(w http.ResponseWriter) {
	w.Header().Add("X-Etcd-Index", fmt.Sprint(e.Index))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(e.statusCode())
	fmt.Fprintln(w, e.toJsonString())
}
