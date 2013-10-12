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

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/store"
	"github.com/coreos/go-raft"
)

//--------------------------------------
// etcd http Helper
//--------------------------------------

// Convert string duration to time format
func durationToExpireTime(strDuration string) (time.Time, error) {
	if strDuration != "" {
		duration, err := strconv.Atoi(strDuration)

		if err != nil {
			return store.Permanent, err
		}
		return time.Now().Add(time.Second * (time.Duration)(duration)), nil

	} else {
		return store.Permanent, nil
	}
}

//--------------------------------------
// HTTP Utilities
//--------------------------------------


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

func getNodePath(urlPath string) string {
	pathPrefixLen := len("/" + version + "/keys")
	return urlPath[pathPrefixLen:]
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
		command := &UpdateCommand{}
		command.Key = "foo"
		command.Value = "bar"
		command.ExpireTime = time.Unix(0, 0)
		//r.Do(command)
	}
	c <- true
}
