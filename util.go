/*
Copyright 2013 CoreOS Inc.

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

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"runtime/pprof"
	"strconv"
	"time"

	"github.com/coreos/etcd/web"
	"github.com/coreos/go-log/log"
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

// startWebInterface starts web interface if webURL is not empty
func startWebInterface() {
	if argInfo.WebURL != "" {
		// start web
		go webHelper()
		go web.Start(r.Server, argInfo.WebURL)
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

// sanitizeListenHost cleans up the ListenHost parameter and appends a port
// if necessary based on the advertised port.
func sanitizeListenHost(listen string, advertised string) string {
	aurl, err := url.Parse(advertised)
	if err != nil {
		fatal(err)
	}

	ahost, aport, err := net.SplitHostPort(aurl.Host)
	if err != nil {
		fatal(err)
	}

	// If the listen host isn't set use the advertised host
	if listen == "" {
		listen = ahost
	}

	return net.JoinHostPort(listen, aport)
}

func redirect(node string, etcd bool, w http.ResponseWriter, req *http.Request) {
	var url string
	path := req.URL.Path

	if etcd {
		etcdAddr, _ := nameToEtcdURL(node)
		url = etcdAddr + path
	} else {
		raftAddr, _ := nameToRaftURL(node)
		url = raftAddr + path
	}

	debugf("Redirect to %s", url)

	http.Redirect(w, req, url, http.StatusTemporaryRedirect)
}

func check(err error) {
	if err != nil {
		fatal(err)
	}
}

//--------------------------------------
// Log
//--------------------------------------

var logger *log.Logger = log.New("etcd", false,
	log.CombinedSink(os.Stdout, "[%s] %s %-9s | %s\n", []string{"prefix", "time", "priority", "message"}))

func infof(format string, v ...interface{}) {
	logger.Infof(format, v...)
}

func debugf(format string, v ...interface{}) {
	if verbose {
		logger.Debugf(format, v...)
	}
}

func debug(v ...interface{}) {
	if verbose {
		logger.Debug(v...)
	}
}

func warnf(format string, v ...interface{}) {
	logger.Warningf(format, v...)
}

func warn(v ...interface{}) {
	logger.Warning(v...)
}

func fatalf(format string, v ...interface{}) {
	logger.Fatalf(format, v...)
}

func fatal(v ...interface{}) {
	logger.Fatalln(v...)
}

//--------------------------------------
// CPU profile
//--------------------------------------
func runCPUProfile() {

	f, err := os.Create(cpuprofile)
	if err != nil {
		fatal(err)
	}
	pprof.StartCPUProfile(f)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for sig := range c {
			infof("captured %v, stopping profiler and exiting..", sig)
			pprof.StopCPUProfile()
			os.Exit(1)
		}
	}()
}

//--------------------------------------
// Testing
//--------------------------------------
func directSet() {
	c := make(chan bool, 1000)
	for i := 0; i < 1000; i++ {
		go send(c)
	}

	for i := 0; i < 1000; i++ {
		<-c
	}
}

func send(c chan bool) {
	for i := 0; i < 10; i++ {
		command := &SetCommand{}
		command.Key = "foo"
		command.Value = "bar"
		command.ExpireTime = time.Unix(0, 0)
		r.Do(command)
	}
	c <- true
}
