package main

import (
	"net"
	"net/url"
	"os"
	"os/signal"
	"runtime/pprof"

	"github.com/coreos/etcd/log"
)

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
		log.Fatal(err)
	}

	// Make sure the host is in Host:Port format
	_, _, err = net.SplitHostPort(host)
	if err != nil {
		log.Fatal(err)
	}

	p = &url.URL{Host: host, Scheme: defaultScheme}

	return p.String()
}

// sanitizeListenHost cleans up the ListenHost parameter and appends a port
// if necessary based on the advertised port.
func sanitizeListenHost(listen string, advertised string) string {
	aurl, err := url.Parse(advertised)
	if err != nil {
		log.Fatal(err)
	}

	ahost, aport, err := net.SplitHostPort(aurl.Host)
	if err != nil {
		log.Fatal(err)
	}

	// If the listen host isn't set use the advertised host
	if listen == "" {
		listen = ahost
	}

	return net.JoinHostPort(listen, aport)
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

//--------------------------------------
// CPU profile
//--------------------------------------
func runCPUProfile() {

	f, err := os.Create(cpuprofile)
	if err != nil {
		log.Fatal(err)
	}
	pprof.StartCPUProfile(f)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for sig := range c {
			log.Infof("captured %v, stopping profiler and exiting..", sig)
			pprof.StopCPUProfile()
			os.Exit(1)
		}
	}()
}
