package main

import (
	"net/http"
	"io"
	"fmt"
	"encoding/json"
	"github.com/xiangli-cmu/raft-etcd/web"
	"os"
)
//--------------------------------------
// Web Helper
//--------------------------------------

func webHelper() {
	storeMsg = make(chan string)
	for {
		web.Hub().Send(<-storeMsg)
	}
}

//--------------------------------------
// HTTP Utilities
//--------------------------------------

func decodeJsonRequest(req *http.Request, data interface{}) error {
	decoder := json.NewDecoder(req.Body)
	if err := decoder.Decode(&data); err != nil && err != io.EOF {
		logger.Println("Malformed json request: %v", err)
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

func Post(t *transHandler, path string, body io.Reader) (*http.Response, error) {

	if t.client != nil {
		resp, err := t.client.Post("https://"+path, "application/json", body)
		return resp, err
	} else {
		resp, err := http.Post("http://"+path, "application/json", body)
		return resp, err
	}
}

func Get(t *transHandler, path string) (*http.Response, error) {
	if t.client != nil {
		resp, err := t.client.Get("https://" + path)
		return resp, err
	} else {
		resp, err := http.Get("http://" + path)
		return resp, err
	}
}

//--------------------------------------
// Log
//--------------------------------------

func debug(msg string, v ...interface{}) {
	if verbose {
		logger.Printf("DEBUG "+msg+"\n", v...)
	}
}

func info(msg string, v ...interface{}) {
	logger.Printf("INFO  "+msg+"\n", v...)
}

func warn(msg string, v ...interface{}) {
	logger.Printf("Alpaca Server: WARN  "+msg+"\n", v...)
}

func fatal(msg string, v ...interface{}) {
	logger.Printf("FATAL "+msg+"\n", v...)
	os.Exit(1)
}