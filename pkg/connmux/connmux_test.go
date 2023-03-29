// Copyright 2023 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package connmux

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net"
	"strings"
	"sync"
	"time"

	"net/http"
	"net/url"
	"testing"

	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	testpb "google.golang.org/grpc/test/grpc_testing"
)

func newTestConnMux(t *testing.T) *ConnMux {
	certFile := "../../tests/fixtures/server.crt"
	keyFile := "../../tests/fixtures/server.key.insecure"

	tcpListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("error creating listener: %v", err)
	}
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		t.Errorf("failed to load TLS keys: %v", err)
	}
	connMux := New(Config{
		Logger:   zap.NewExample(),
		Listener: tcpListener,
		Insecure: true,
		Secure:   true,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
			NextProtos:   []string{"h2", "http/1.1"},
		},
	})

	t.Logf("Listening on address %s", connMux.root.Addr().String())
	return connMux
}

func TestConnMux(t *testing.T) {
	connMux := newTestConnMux(t)
	// HTTP server
	httpL := connMux.HTTPListener()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "HTTP response Proto: %s %v\n", r.Proto, r.Header)
	})
	h2s := &http2.Server{}
	httpServer := &http.Server{
		Handler: h2c.NewHandler(mux, h2s),
	}
	http2.ConfigureServer(httpServer, h2s)
	go httpServer.Serve(httpL)
	defer httpServer.Close()

	// GRPC Server
	grpcServer := grpc.NewServer()
	testpb.RegisterTestServiceServer(grpcServer, &dummyStubServer{body: []byte("grpc response")})

	grpcL := connMux.GRPCListener()
	go grpcServer.Serve(grpcL)

	go connMux.Serve()
	defer connMux.Close()

	destHost := connMux.root.Addr().String()
	destHTTP := url.URL{Scheme: "http", Host: destHost}
	destHTTPS := url.URL{Scheme: "https", Host: destHost}

	// HTTP plain
	t.Run("Test HTTP1 without TLS works", func(t *testing.T) {
		//t.Skip()
		client := &http.Client{}
		verifyHTTPRequest(t, client, destHTTP.String())
	})

	// HTTPS
	t.Run("Test HTTP1 with TLS works", func(t *testing.T) {
		client := &http.Client{}
		client.Transport = &http.Transport{TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		}}
		verifyHTTPRequest(t, client, destHTTPS.String())
	})

	// HTTP2 plain
	t.Run("Test HTTP2 without TLS works", func(t *testing.T) {
		client := &http.Client{}
		client.Transport = &http2.Transport{
			AllowHTTP: true,
			DialTLS: func(netw, addr string, cfg *tls.Config) (net.Conn, error) {
				return net.Dial(netw, addr)
			}}
		verifyHTTPRequest(t, client, destHTTP.String())
	})

	// HTTP2 TLS
	t.Run("Test HTTP2 with TLS works", func(t *testing.T) {
		client := &http.Client{}
		client.Transport = &http2.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
				NextProtos:         []string{http2.NextProtoTLS},
			},
		}
		verifyHTTPRequest(t, client, destHTTPS.String())
	})

	// Set up a connection to the server.

	t.Run("Test GRPC with TLS works", func(t *testing.T) {
		conn, err := grpc.Dial(destHost, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			t.Errorf("did not connect: %v", err)
		}
		defer conn.Close()
		verifyGRPCRequest(t, conn)
	})
}

// TestConnMuxConcurrency tests that HTTP and GRPC connections work repet
func TestConnMuxConcurrency(t *testing.T) {
	connMux := newTestConnMux(t)
	// HTTP server
	httpL := connMux.HTTPListener()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "HTTP response Proto: %s %v\n", r.Proto, r.Header)
	})
	h2s := &http2.Server{}
	httpServer := &http.Server{
		Handler: h2c.NewHandler(mux, h2s),
	}
	http2.ConfigureServer(httpServer, h2s)
	go httpServer.Serve(httpL)

	// GRPC Server
	grpcServer := grpc.NewServer()
	testpb.RegisterTestServiceServer(grpcServer, &dummyStubServer{body: []byte("grpc response")})

	grpcL := connMux.GRPCListener()
	go grpcServer.Serve(grpcL)

	go connMux.Serve()
	defer connMux.Close()

	destHost := connMux.root.Addr().String()
	destHTTP := url.URL{Scheme: "http", Host: destHost}
	destHTTPS := url.URL{Scheme: "https", Host: destHost}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(5)
		go func() {
			// HTTP plain
			defer wg.Done()
			t.Logf("Test HTTP1 without TLS works")
			client := &http.Client{}
			verifyHTTPRequest(t, client, destHTTP.String())
		}()

		go func() {
			// HTTPS
			defer wg.Done()
			t.Logf("Test HTTP1 with TLS works")
			client := &http.Client{}
			client.Transport = &http.Transport{TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			}}
			verifyHTTPRequest(t, client, destHTTPS.String())
		}()

		go func() {
			// HTTP2 plain
			defer wg.Done()
			t.Logf("Test HTTP2 without TLS works")
			client := &http.Client{}
			client.Transport = &http2.Transport{
				AllowHTTP: true,
				DialTLS: func(netw, addr string, cfg *tls.Config) (net.Conn, error) {
					return net.Dial(netw, addr)
				}}
			verifyHTTPRequest(t, client, destHTTP.String())
		}()

		go func() {
			// HTTP2 TLS
			defer wg.Done()
			t.Logf("Test HTTP2 with TLS works")
			client := &http.Client{}
			client.Transport = &http2.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
					NextProtos:         []string{http2.NextProtoTLS},
				},
			}
			verifyHTTPRequest(t, client, destHTTPS.String())
		}()

		go func() {
			// Set up a connection to the server.
			defer wg.Done()
			t.Logf("Test GRPC works")
			conn, err := grpc.Dial(destHost, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				t.Errorf("did not connect: %v", err)
			}
			defer conn.Close()
			verifyGRPCRequest(t, conn)
		}()
	}
	wg.Wait()

}

func verifyHTTPRequest(t *testing.T, client *http.Client, destination string) {
	request, err := http.NewRequest("Get", destination, nil)
	if err != nil {
		t.Error(err)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Error(err)
	}
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		t.Error(err)
	}
	t.Logf("received body: %s", string(body))
	if !strings.Contains(string(body), "HTTP response Proto") {
		t.Errorf("expected \"HTTP response Proto\" , got %s", body)
	}

}

func verifyGRPCRequest(t *testing.T, conn grpc.ClientConnInterface) {
	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	c := testpb.NewTestServiceClient(conn)

	c.HalfDuplexCall(ctx)
	resp, err := c.UnaryCall(context.TODO(), &testpb.SimpleRequest{}, grpc.WaitForReady(true))
	if err != nil {
		t.Error("failed to invoke rpc to foo (e1)", err)
	}
	if resp.GetPayload() == nil || !bytes.Equal(resp.GetPayload().GetBody(), []byte("grpc response")) {
		t.Errorf("unexpected response from foo (e1): %s", resp.GetPayload().GetBody())
	}

}

type dummyStubServer struct {
	testpb.UnimplementedTestServiceServer
	body []byte
}

func (d dummyStubServer) UnaryCall(context.Context, *testpb.SimpleRequest) (*testpb.SimpleResponse, error) {
	return &testpb.SimpleResponse{
		Payload: &testpb.Payload{
			Type: testpb.PayloadType_COMPRESSABLE,
			Body: d.body,
		},
	}, nil
}
