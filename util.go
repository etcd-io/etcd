package main

import (
	"encoding/json"
	"fmt"
	"github.com/coreos/etcd/web"
	"io"
	"log"
	"net/http"
	"os"
)

//--------------------------------------
// Web Helper
//--------------------------------------
var storeMsg chan string

// Help to send msg from store to webHub
func webHelper() {
	storeMsg = make(chan string)
	for {
		// transfer the new msg to webHub
		web.Hub().Send(<-storeMsg)
	}
}

//--------------------------------------
// HTTP Utilities
//--------------------------------------

func decodeJsonRequest(req *http.Request, data interface{}) error {
	decoder := json.NewDecoder(req.Body)
	if err := decoder.Decode(&data); err != nil && err != io.EOF {
		warn("Malformed json request: %v", err)
		return fmt.Errorf("Malformed json request: %v", err)
	}
	return nil
}

func encodeJsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if data != nil {
		encoder := json.NewEncoder(w)
		encoder.Encode(data)
	}
}

//--------------------------------------
// Log
//--------------------------------------

var logger *log.Logger

func init() {
	logger = log.New(os.Stdout, "[etcd] ", log.Lmicroseconds)
}

func debug(msg string, v ...interface{}) {
	if verbose {
		logger.Printf("DEBUG "+msg+"\n", v...)
	}
}

func warn(msg string, v ...interface{}) {
	logger.Printf("WARN  "+msg+"\n", v...)
}

func fatal(msg string, v ...interface{}) {
	logger.Printf("FATAL "+msg+"\n", v...)
	os.Exit(1)
}
