/*
 *
 * Copyright 2019 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

// Package fakexds provides a very basic fake implementation of the xDS server
// for unit testing purposes.
package fakexds

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	discoverypb "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	adsgrpc "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v2"
	lrsgrpc "github.com/envoyproxy/go-control-plane/envoy/service/load_stats/v2"
)

// TODO: Make this a var or a field in the server if there is a need to use a
// value other than this default.
const defaultChannelBufferSize = 50

// Request wraps an xDS request and error.
type Request struct {
	Req *discoverypb.DiscoveryRequest
	Err error
}

// Response wraps an xDS response and error.
type Response struct {
	Resp *discoverypb.DiscoveryResponse
	Err  error
}

// Server is a very basic implementation of a fake xDS server. It provides a
// request and response channel for the user to control the requests that are
// expected and the responses that needs to be sent out.
type Server struct {
	// RequestChan is a buffered channel on which the fake server writes the
	// received requests onto.
	RequestChan chan *Request
	// ResponseChan is a buffered channel from which the fake server reads the
	// responses that it must send out to the client.
	ResponseChan chan *Response
	// Address is the host:port on which the fake xdsServer is listening on.
	Address string
	// LRS is the LRS server installed.
	LRS *LRSServer
}

// StartServer starts a fakexds.Server. The returned function should be invoked
// by the caller once the test is done.
func StartServer(t *testing.T) (*Server, func()) {
	t.Helper()

	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("net.Listen() failed: %v", err)
	}
	server := grpc.NewServer()

	lrss := newLRSServer()
	lrsgrpc.RegisterLoadReportingServiceServer(server, lrss)

	fs := &Server{
		RequestChan:  make(chan *Request, defaultChannelBufferSize),
		ResponseChan: make(chan *Response, defaultChannelBufferSize),
		Address:      lis.Addr().String(),
		LRS:          lrss,
	}
	adsgrpc.RegisterAggregatedDiscoveryServiceServer(server, fs)

	go server.Serve(lis)
	t.Logf("Starting fake xDS server at %v...", fs.Address)

	return fs, func() { server.Stop() }
}

// GetClientConn returns a grpc.ClientConn talking to the fake server. The
// returned function should be invoked by the caller once the test is done.
func (fs *Server) GetClientConn(t *testing.T) (*grpc.ClientConn, func()) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cc, err := grpc.DialContext(ctx, fs.Address, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		t.Fatalf("grpc.DialContext(%s) failed: %v", fs.Address, err)
	}
	t.Log("Started xDS gRPC client...")

	return cc, func() {
		cc.Close()
	}
}

// StreamAggregatedResources is the fake implementation to handle an ADS
// stream.
func (fs *Server) StreamAggregatedResources(s adsgrpc.AggregatedDiscoveryService_StreamAggregatedResourcesServer) error {
	errCh := make(chan error, 2)
	go func() {
		for {
			req, err := s.Recv()
			if err != nil {
				errCh <- err
				return
			}
			fs.RequestChan <- &Request{req, err}
		}
	}()
	go func() {
		var retErr error
		defer func() {
			errCh <- retErr
		}()

		for {
			select {
			case r := <-fs.ResponseChan:
				if r.Err != nil {
					retErr = r.Err
					return
				}
				if err := s.Send(r.Resp); err != nil {
					retErr = err
					return
				}
			case <-s.Context().Done():
				retErr = s.Context().Err()
				return
			}
		}
	}()

	if err := <-errCh; err != nil {
		return err
	}
	return nil
}

// DeltaAggregatedResources helps implement the ADS service.
func (fs *Server) DeltaAggregatedResources(adsgrpc.AggregatedDiscoveryService_DeltaAggregatedResourcesServer) error {
	return status.Error(codes.Unimplemented, "")
}
