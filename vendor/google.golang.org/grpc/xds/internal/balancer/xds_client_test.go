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

package balancer

import (
	"fmt"
	"testing"
	"time"

	xdspb "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	corepb "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	endpointpb "github.com/envoyproxy/go-control-plane/envoy/api/v2/endpoint"
	"github.com/golang/protobuf/proto"
	anypb "github.com/golang/protobuf/ptypes/any"
	structpb "github.com/golang/protobuf/ptypes/struct"
	wrpb "github.com/golang/protobuf/ptypes/wrappers"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/attributes"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/resolver"
	xdsinternal "google.golang.org/grpc/xds/internal"
	"google.golang.org/grpc/xds/internal/balancer/lrs"
	xdsclient "google.golang.org/grpc/xds/internal/client"
	"google.golang.org/grpc/xds/internal/client/bootstrap"
	"google.golang.org/grpc/xds/internal/client/fakexds"
	"google.golang.org/grpc/xds/internal/testutils"
)

const (
	edsType = "type.googleapis.com/envoy.api.v2.ClusterLoadAssignment"
)

var (
	testServiceName           = "test/foo"
	testEDSClusterName        = "test/service/eds"
	testClusterLoadAssignment = &xdspb.ClusterLoadAssignment{
		ClusterName: testEDSClusterName,
		Endpoints: []*endpointpb.LocalityLbEndpoints{{
			Locality: &corepb.Locality{
				Region:  "asia-east1",
				Zone:    "1",
				SubZone: "sa",
			},
			LbEndpoints: []*endpointpb.LbEndpoint{{
				HostIdentifier: &endpointpb.LbEndpoint_Endpoint{
					Endpoint: &endpointpb.Endpoint{
						Address: &corepb.Address{
							Address: &corepb.Address_SocketAddress{
								SocketAddress: &corepb.SocketAddress{
									Address: "1.1.1.1",
									PortSpecifier: &corepb.SocketAddress_PortValue{
										PortValue: 10001,
									},
									ResolverName: "dns",
								},
							},
						},
						HealthCheckConfig: nil,
					},
				},
				Metadata: &corepb.Metadata{
					FilterMetadata: map[string]*structpb.Struct{
						"xx.lb": {
							Fields: map[string]*structpb.Value{
								"endpoint_name": {
									Kind: &structpb.Value_StringValue{
										StringValue: "some.endpoint.name",
									},
								},
							},
						},
					},
				},
			}},
			LoadBalancingWeight: &wrpb.UInt32Value{
				Value: 1,
			},
			Priority: 0,
		}},
	}
	marshaledClusterLoadAssignment, _ = proto.Marshal(testClusterLoadAssignment)
	testEDSResp                       = &xdspb.DiscoveryResponse{
		Resources: []*anypb.Any{
			{
				TypeUrl: edsType,
				Value:   marshaledClusterLoadAssignment,
			},
		},
		TypeUrl: edsType,
	}
)

func (s) TestEDSClientResponseHandling(t *testing.T) {
	td, cleanup := fakexds.StartServer(t)
	defer cleanup()
	edsRespChan := make(chan *xdsclient.EDSUpdate, 10)
	newEDS := func(i *xdsclient.EDSUpdate) error {
		edsRespChan <- i
		return nil
	}
	client := newXDSClientWrapper(newEDS, func() {}, balancer.BuildOptions{Target: resolver.Target{Endpoint: testServiceName}}, nil)
	defer client.close()

	// Test that EDS requests sent match XDSConfig.
	for _, test := range []struct {
		name             string
		edsServiceName   string
		wantResourceName string
	}{
		{
			name:           "empty-edsServiceName",
			edsServiceName: "",
			// EDSServiceName is an empty string, user's dialing target will be
			// set in resource names.
			wantResourceName: testServiceName,
		},
		{
			name:             "non-empty-edsServiceName",
			edsServiceName:   testEDSClusterName,
			wantResourceName: testEDSClusterName,
		},
	} {
		client.handleUpdate(&XDSConfig{
			BalancerName:               td.Address,
			EDSServiceName:             test.edsServiceName,
			LrsLoadReportingServerName: nil,
		}, nil)
		req := <-td.RequestChan
		if req.Err != nil {
			t.Fatalf("EDS RPC failed with err: %v", req.Err)
		}
		wantReq1 := &xdspb.DiscoveryRequest{
			TypeUrl:       edsType,
			ResourceNames: []string{test.wantResourceName},
			Node:          &corepb.Node{},
		}
		if !proto.Equal(req.Req, wantReq1) {
			t.Fatalf("%v: got EDS request %v, expected: %v, diff: %s", test.name, req.Req, wantReq1, cmp.Diff(req.Req, wantReq1, cmp.Comparer(proto.Equal)))
		}
	}

	// Make sure that the responses from the stream are also handled.
	td.ResponseChan <- &fakexds.Response{Resp: testEDSResp}
	gotResp := <-edsRespChan
	want, err := xdsclient.ParseEDSRespProto(testClusterLoadAssignment)
	if err != nil {
		t.Fatalf("parsing wanted EDS response failed: %v", err)
	}
	if !cmp.Equal(gotResp, want) {
		t.Fatalf("received unexpected EDS response, got %v, want %v", gotResp, want)
	}
}

type testXDSClient struct {
	clusterNameChan *testutils.Channel
	edsCb           func(*xdsclient.EDSUpdate, error)
}

func newTestXDSClient() *testXDSClient {
	return &testXDSClient{
		clusterNameChan: testutils.NewChannel(),
	}
}

func (c *testXDSClient) WatchEDS(clusterName string, edsCb func(*xdsclient.EDSUpdate, error)) (cancel func()) {
	c.clusterNameChan.Send(clusterName)
	c.edsCb = edsCb
	return func() {}
}

func (c *testXDSClient) ReportLoad(server string, clusterName string, loadStore lrs.Store) (cancel func()) {
	panic("implement me 2")
}

func (c *testXDSClient) Close() {
	panic("implement me 3")
}

// Test that error from the xds client is handled correctly.
func (s) TestEDSClientResponseErrorHandling(t *testing.T) {
	td, cleanup := fakexds.StartServer(t)
	defer cleanup()
	edsRespChan := make(chan *xdsclient.EDSUpdate, 10)
	newEDS := func(i *xdsclient.EDSUpdate) error {
		edsRespChan <- i
		return nil
	}

	client := newXDSClientWrapper(newEDS, func() {}, balancer.BuildOptions{Target: resolver.Target{Endpoint: testServiceName}}, nil)
	defer client.close()

	// Create a client to be passed in attributes.
	c := newTestXDSClient()
	client.handleUpdate(&XDSConfig{
		BalancerName:               td.Address,
		EDSServiceName:             testEDSClusterName,
		LrsLoadReportingServerName: nil,
	}, attributes.New(xdsinternal.XDSClientID, c),
	)

	if gotClusterName, err := c.clusterNameChan.Receive(); err != nil || gotClusterName != testEDSClusterName {
		t.Fatalf("got EDS watch clusterName %v, expected: %v", gotClusterName, testEDSClusterName)
	}
	c.edsCb(nil, fmt.Errorf("testing err"))

	// The ballback is called with an error, expect no update from edsRespChan.
	//
	// TODO: check for loseContact() when errors indicating "lose contact" are
	// handled correctly.

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-timer.C:
	case resp := <-edsRespChan:
		t.Fatalf("unexpected resp: %v", resp)
	}
}

// Test that if xds_client is in attributes, the xdsclientnew function will not
// be called, and the xds_client from attributes will be used.
//
// And also that when xds_client in attributes is updated, the new one will be
// used, and watch will be restarted.
func (s) TestEDSClientInAttributes(t *testing.T) {
	edsRespChan := make(chan *xdsclient.EDSUpdate, 10)
	newEDS := func(i *xdsclient.EDSUpdate) error {
		edsRespChan <- i
		return nil
	}

	oldxdsclientNew := xdsclientNew
	xdsclientNew = func(opts xdsclient.Options) (clientInterface xdsClientInterface, e error) {
		t.Fatalf("unexpected call to xdsclientNew when xds_client is set in attributes")
		return nil, nil
	}
	defer func() { xdsclientNew = oldxdsclientNew }()

	td, cleanup := fakexds.StartServer(t)
	defer cleanup()
	// Create a client to be passed in attributes.
	c, _ := oldxdsclientNew(xdsclient.Options{
		Config: bootstrap.Config{
			BalancerName: td.Address,
			Creds:        grpc.WithInsecure(),
			NodeProto:    &corepb.Node{},
		},
	})
	// Need to manually close c because xdsclientWrapper won't close it (it's
	// from attributes).
	defer c.Close()

	client := newXDSClientWrapper(newEDS, func() {}, balancer.BuildOptions{Target: resolver.Target{Endpoint: testServiceName}}, nil)
	defer client.close()

	client.handleUpdate(
		&XDSConfig{EDSServiceName: testEDSClusterName, LrsLoadReportingServerName: nil},
		attributes.New(xdsinternal.XDSClientID, c),
	)

	expectedReq := &xdspb.DiscoveryRequest{
		TypeUrl:       edsType,
		ResourceNames: []string{testEDSClusterName},
		Node:          &corepb.Node{},
	}

	// Make sure the requests are sent to the correct td.
	req := <-td.RequestChan
	if req.Err != nil {
		t.Fatalf("EDS RPC failed with err: %v", req.Err)
	}
	if !proto.Equal(req.Req, expectedReq) {
		t.Fatalf("got EDS request %T, expected: %T, diff: %s", req.Req, expectedReq, cmp.Diff(req.Req, expectedReq, cmp.Comparer(proto.Equal)))
	}

	td2, cleanup2 := fakexds.StartServer(t)
	defer cleanup2()
	// Create a client to be passed in attributes.
	c2, _ := oldxdsclientNew(xdsclient.Options{
		Config: bootstrap.Config{
			BalancerName: td2.Address,
			Creds:        grpc.WithInsecure(),
			NodeProto:    &corepb.Node{},
		},
	})
	// Need to manually close c because xdsclientWrapper won't close it (it's
	// from attributes).
	defer c2.Close()

	// Update with a new xds_client in attributes.
	client.handleUpdate(
		&XDSConfig{EDSServiceName: "", LrsLoadReportingServerName: nil},
		attributes.New(xdsinternal.XDSClientID, c2),
	)

	expectedReq2 := &xdspb.DiscoveryRequest{
		TypeUrl: edsType,
		// The edsServiceName in new update is an empty string, user's dial
		// target should be used as eds name to watch.
		ResourceNames: []string{testServiceName},
		Node:          &corepb.Node{},
	}

	// Make sure the requests are sent to the correct td.
	req2 := <-td2.RequestChan
	if req.Err != nil {
		t.Fatalf("EDS RPC failed with err: %v", req.Err)
	}
	if !proto.Equal(req2.Req, expectedReq2) {
		t.Fatalf("got EDS request %T, expected: %T, diff: %s", req2.Req, expectedReq, cmp.Diff(req2.Req, expectedReq2, cmp.Comparer(proto.Equal)))
	}
}

// Test that when edsServiceName from service config is updated, the new one
// will be watched.
func (s) TestEDSServiceNameUpdate(t *testing.T) {
	edsRespChan := make(chan *xdsclient.EDSUpdate, 10)
	newEDS := func(i *xdsclient.EDSUpdate) error {
		edsRespChan <- i
		return nil
	}

	oldxdsclientNew := xdsclientNew
	xdsclientNew = func(opts xdsclient.Options) (clientInterface xdsClientInterface, e error) {
		t.Fatalf("unexpected call to xdsclientNew when xds_client is set in attributes")
		return nil, nil
	}
	defer func() { xdsclientNew = oldxdsclientNew }()

	td, cleanup := fakexds.StartServer(t)
	defer cleanup()
	// Create a client to be passed in attributes.
	c, _ := oldxdsclientNew(xdsclient.Options{
		Config: bootstrap.Config{
			BalancerName: td.Address,
			Creds:        grpc.WithInsecure(),
			NodeProto:    &corepb.Node{},
		},
	})
	// Need to manually close c because xdsclientWrapper won't close it (it's
	// from attributes).
	defer c.Close()

	client := newXDSClientWrapper(newEDS, func() {}, balancer.BuildOptions{Target: resolver.Target{Endpoint: testServiceName}}, nil)
	defer client.close()

	client.handleUpdate(
		&XDSConfig{EDSServiceName: testEDSClusterName, LrsLoadReportingServerName: nil},
		attributes.New(xdsinternal.XDSClientID, c),
	)

	expectedReq := &xdspb.DiscoveryRequest{
		TypeUrl:       edsType,
		ResourceNames: []string{testEDSClusterName},
		Node:          &corepb.Node{},
	}

	// Make sure the requests are sent to the correct td.
	req := <-td.RequestChan
	if req.Err != nil {
		t.Fatalf("EDS RPC failed with err: %v", req.Err)
	}
	if !proto.Equal(req.Req, expectedReq) {
		t.Fatalf("got EDS request %T, expected: %T, diff: %s", req.Req, expectedReq, cmp.Diff(req.Req, expectedReq, cmp.Comparer(proto.Equal)))
	}

	// Update with a new edsServiceName.
	client.handleUpdate(
		&XDSConfig{EDSServiceName: "", LrsLoadReportingServerName: nil},
		attributes.New(xdsinternal.XDSClientID, c),
	)

	expectedReq2 := &xdspb.DiscoveryRequest{
		TypeUrl: edsType,
		// The edsServiceName in new update is an empty string, user's dial
		// target should be used as eds name to watch.
		ResourceNames: []string{testServiceName},
		Node:          &corepb.Node{},
	}

	// Make sure the requests are sent to the correct td.
	req2 := <-td.RequestChan
	if req.Err != nil {
		t.Fatalf("EDS RPC failed with err: %v", req.Err)
	}
	if !proto.Equal(req2.Req, expectedReq2) {
		t.Fatalf("got EDS request %T, expected: %T, diff: %s", req2.Req, expectedReq, cmp.Diff(req2.Req, expectedReq2, cmp.Comparer(proto.Equal)))
	}
}
