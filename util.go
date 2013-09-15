package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"runtime/pprof"
	"strconv"
	"time"

	"github.com/coreos/etcd/file_system"
	"github.com/coreos/etcd/web"
)

//--------------------------------------
// etcd http Helper
//--------------------------------------

// Convert string duration to time format
func durationToExpireTime(strDuration string) (time.Time, error) {
	if strDuration != "" {
		duration, err := strconv.Atoi(strDuration)

		if err != nil {
			return fileSystem.Permanent, err
		}
		return time.Now().Add(time.Second * (time.Duration)(duration)), nil

	} else {
		return fileSystem.Permanent, nil
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

func dispatch(c Command, w http.ResponseWriter, req *http.Request, toURL func(name string) (string, bool)) error {
	if r.State() == raft.Leader {
		if body, err := r.Do(c); err != nil {
			return err
		} else {
			if body == nil {
				return etcdErr.NewError(300, "Empty result from raft")
			} else {
				body, _ := body.([]byte)
				w.WriteHeader(http.StatusOK)
				w.Write(body)
				return nil
			}
		}

	} else {
		leader := r.Leader()
		// current no leader
		if leader == "" {
			return etcdErr.NewError(300, "")
		}
		url, _ := toURL(leader)

		redirect(url, w, req)

		return nil
	}
	return etcdErr.NewError(300, "")
}

func redirect(hostname string, w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path

	url := hostname + path

	debugf("Redirect to %s", url)

	http.Redirect(w, req, url, http.StatusTemporaryRedirect)
}

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

func check(err error) {
	if err != nil {
		fatal(err)
	}
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
