package main

import (
	"encoding/json"
	"fmt"
	"github.com/coreos/etcd/web"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

//--------------------------------------
// etcd http Helper
//--------------------------------------

// Convert string duration to time format
func durationToExpireTime(strDuration string) (time.Time, error) {
	if strDuration != "" {
		duration, err := strconv.Atoi(strDuration)

		if err != nil {
			return time.Unix(0, 0), err
		}
		return time.Now().Add(time.Second * (time.Duration)(duration)), nil

	} else {
		return time.Unix(0, 0), nil
	}
}

//--------------------------------------
// Web Helper
//--------------------------------------
var storeMsg chan string

// Help to send msg from store to webHub
func webHelper() {
	storeMsg = make(chan string)
	etcdStore.SetMessager(storeMsg)
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
		warnf("Malformed json request: %v", err)
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

// sanitizeURL will cleanup a host string in the format hostname:port and
// attach a schema.
func sanitizeURL(host string, defaultScheme string) string {
	// Blank URLs are fine input, just return it
	if len(host) == 0 {
		return host
	}

	p, err := url.Parse(host)
	if err != nil {
		fatal(err)
	}

	// Make sure the host is in Host:Port format
	_, _, err = net.SplitHostPort(host)
	if err != nil {
		fatal(err)
	}

	p = &url.URL{Host: host, Scheme: defaultScheme}

	return p.String()
}

//--------------------------------------
// Log
//--------------------------------------

var logger *log.Logger

func init() {
	logger = log.New(os.Stdout, "[etcd] ", log.Lmicroseconds)
}

func infof(msg string, v ...interface{}) {
	logger.Printf("INFO "+msg+"\n", v...)
}

func debugf(msg string, v ...interface{}) {
	if verbose {
		logger.Printf("DEBUG "+msg+"\n", v...)
	}
}

func debug(v ...interface{}) {
	if verbose {
		logger.Println("DEBUG " + fmt.Sprint(v...))
	}
}

func warnf(msg string, v ...interface{}) {
	logger.Printf("WARN  "+msg+"\n", v...)
}

func warn(v ...interface{}) {
	logger.Println("WARN " + fmt.Sprint(v...))
}

func fatalf(msg string, v ...interface{}) {
	logger.Printf("FATAL "+msg+"\n", v...)
	os.Exit(1)
}

func fatal(v ...interface{}) {
	logger.Println("FATAL " + fmt.Sprint(v...))
	os.Exit(1)
}
