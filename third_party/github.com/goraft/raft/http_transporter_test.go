package raft

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"
)

// Ensure that we can start several servers and have them communicate.
func TestHTTPTransporter(t *testing.T) {
	transporter := NewHTTPTransporter("/raft", testElectionTimeout)
	transporter.DisableKeepAlives = true

	servers := []Server{}
	f0 := func(server Server, httpServer *http.Server) {
		// Stop the leader and wait for an election.
		server.Stop()
		time.Sleep(testElectionTimeout * 2)

		if servers[1].State() != Leader && servers[2].State() != Leader {
			t.Fatal("Expected re-election:", servers[1].State(), servers[2].State())
		}
		server.Start()
	}
	f1 := func(server Server, httpServer *http.Server) {
	}
	f2 := func(server Server, httpServer *http.Server) {
	}
	runTestHttpServers(t, &servers, transporter, f0, f1, f2)
}

// Starts multiple independent Raft servers wrapped with HTTP servers.
func runTestHttpServers(t *testing.T, servers *[]Server, transporter *HTTPTransporter, callbacks ...func(Server, *http.Server)) {
	var wg sync.WaitGroup
	httpServers := []*http.Server{}
	listeners := []net.Listener{}
	for i := range callbacks {
		wg.Add(1)
		port := 9000 + i

		// Create raft server.
		server := newTestServer(fmt.Sprintf("localhost:%d", port), transporter)
		server.SetHeartbeatInterval(testHeartbeatInterval)
		server.SetElectionTimeout(testElectionTimeout)
		server.Start()

		defer server.Stop()
		*servers = append(*servers, server)

		// Create listener for HTTP server and start it.
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			panic(err)
		}
		defer listener.Close()
		listeners = append(listeners, listener)

		// Create wrapping HTTP server.
		mux := http.NewServeMux()
		transporter.Install(server, mux)
		httpServer := &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: mux}
		httpServers = append(httpServers, httpServer)
		go func() { httpServer.Serve(listener) }()
	}

	// Setup configuration.
	for _, server := range *servers {
		if _, err := (*servers)[0].Do(&DefaultJoinCommand{Name: server.Name(), ConnectionString: fmt.Sprintf("http://%s", server.Name())}); err != nil {
			t.Fatalf("Server %s unable to join: %v", server.Name(), err)
		}
	}

	// Wait for configuration to propagate.
	time.Sleep(testHeartbeatInterval * 2)

	// Execute all the callbacks at the same time.
	for _i, _f := range callbacks {
		i, f := _i, _f
		go func() {
			defer wg.Done()
			f((*servers)[i], httpServers[i])
		}()
	}

	// Wait until everything is done.
	wg.Wait()
}

func BenchmarkSpeed(b *testing.B) {

	transporter := NewHTTPTransporter("/raft", testElectionTimeout)
	transporter.DisableKeepAlives = true

	servers := []Server{}

	for i := 0; i < 3; i++ {
		port := 9000 + i

		// Create raft server.
		server := newTestServer(fmt.Sprintf("localhost:%d", port), transporter)
		server.SetHeartbeatInterval(testHeartbeatInterval)
		server.SetElectionTimeout(testElectionTimeout)
		server.Start()

		defer server.Stop()
		servers = append(servers, server)

		// Create listener for HTTP server and start it.
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			panic(err)
		}
		defer listener.Close()

		// Create wrapping HTTP server.
		mux := http.NewServeMux()
		transporter.Install(server, mux)
		httpServer := &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: mux}

		go func() { httpServer.Serve(listener) }()
	}

	// Setup configuration.
	for _, server := range servers {
		(servers)[0].Do(&DefaultJoinCommand{Name: server.Name(), ConnectionString: fmt.Sprintf("http://%s", server.Name())})
	}

	c := make(chan bool)

	// Wait for configuration to propagate.
	time.Sleep(testHeartbeatInterval * 2)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		for i := 0; i < 1000; i++ {
			go send(c, servers[0])
		}

		for i := 0; i < 1000; i++ {
			<-c
		}
	}
}

func send(c chan bool, s Server) {
	for i := 0; i < 20; i++ {
		s.Do(&NOPCommand{})
	}
	c <- true
}
