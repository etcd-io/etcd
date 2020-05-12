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

package client

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	xdspb "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	corepb "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/golang/protobuf/proto"
	anypb "github.com/golang/protobuf/ptypes/any"
	"google.golang.org/grpc/xds/internal/client/fakexds"
)

const (
	clusterName1 = "foo-cluster"
	clusterName2 = "bar-cluster"
	serviceName1 = "foo-service"
	serviceName2 = "bar-service"
)

func (v2c *v2Client) cloneCDSCacheForTesting() map[string]CDSUpdate {
	v2c.mu.Lock()
	defer v2c.mu.Unlock()

	cloneCache := make(map[string]CDSUpdate)
	for k, v := range v2c.cdsCache {
		cloneCache[k] = v
	}
	return cloneCache
}

func TestValidateCluster(t *testing.T) {
	emptyUpdate := CDSUpdate{ServiceName: "", EnableLRS: false}
	tests := []struct {
		name       string
		cluster    *xdspb.Cluster
		wantUpdate CDSUpdate
		wantErr    bool
	}{
		{
			name: "non-eds-cluster-type",
			cluster: &xdspb.Cluster{
				ClusterDiscoveryType: &xdspb.Cluster_Type{Type: xdspb.Cluster_STATIC},
				EdsClusterConfig: &xdspb.Cluster_EdsClusterConfig{
					EdsConfig: &corepb.ConfigSource{
						ConfigSourceSpecifier: &corepb.ConfigSource_Ads{
							Ads: &corepb.AggregatedConfigSource{},
						},
					},
				},
				LbPolicy: xdspb.Cluster_LEAST_REQUEST,
			},
			wantUpdate: emptyUpdate,
			wantErr:    true,
		},
		{
			name: "no-eds-config",
			cluster: &xdspb.Cluster{
				ClusterDiscoveryType: &xdspb.Cluster_Type{Type: xdspb.Cluster_EDS},
				LbPolicy:             xdspb.Cluster_ROUND_ROBIN,
			},
			wantUpdate: emptyUpdate,
			wantErr:    true,
		},
		{
			name: "no-ads-config-source",
			cluster: &xdspb.Cluster{
				ClusterDiscoveryType: &xdspb.Cluster_Type{Type: xdspb.Cluster_EDS},
				EdsClusterConfig:     &xdspb.Cluster_EdsClusterConfig{},
				LbPolicy:             xdspb.Cluster_ROUND_ROBIN,
			},
			wantUpdate: emptyUpdate,
			wantErr:    true,
		},
		{
			name: "non-round-robin-lb-policy",
			cluster: &xdspb.Cluster{
				ClusterDiscoveryType: &xdspb.Cluster_Type{Type: xdspb.Cluster_EDS},
				EdsClusterConfig: &xdspb.Cluster_EdsClusterConfig{
					EdsConfig: &corepb.ConfigSource{
						ConfigSourceSpecifier: &corepb.ConfigSource_Ads{
							Ads: &corepb.AggregatedConfigSource{},
						},
					},
				},
				LbPolicy: xdspb.Cluster_LEAST_REQUEST,
			},
			wantUpdate: emptyUpdate,
			wantErr:    true,
		},
		{
			name: "happy-case-no-service-name-no-lrs",
			cluster: &xdspb.Cluster{
				ClusterDiscoveryType: &xdspb.Cluster_Type{Type: xdspb.Cluster_EDS},
				EdsClusterConfig: &xdspb.Cluster_EdsClusterConfig{
					EdsConfig: &corepb.ConfigSource{
						ConfigSourceSpecifier: &corepb.ConfigSource_Ads{
							Ads: &corepb.AggregatedConfigSource{},
						},
					},
				},
				LbPolicy: xdspb.Cluster_ROUND_ROBIN,
			},
			wantUpdate: emptyUpdate,
		},
		{
			name: "happy-case-no-lrs",
			cluster: &xdspb.Cluster{
				ClusterDiscoveryType: &xdspb.Cluster_Type{Type: xdspb.Cluster_EDS},
				EdsClusterConfig: &xdspb.Cluster_EdsClusterConfig{
					EdsConfig: &corepb.ConfigSource{
						ConfigSourceSpecifier: &corepb.ConfigSource_Ads{
							Ads: &corepb.AggregatedConfigSource{},
						},
					},
					ServiceName: serviceName1,
				},
				LbPolicy: xdspb.Cluster_ROUND_ROBIN,
			},
			wantUpdate: CDSUpdate{ServiceName: serviceName1, EnableLRS: false},
		},
		{
			name:       "happiest-case",
			cluster:    goodCluster1,
			wantUpdate: CDSUpdate{ServiceName: serviceName1, EnableLRS: true},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotUpdate, gotErr := validateCluster(test.cluster)
			if (gotErr != nil) != test.wantErr {
				t.Errorf("validateCluster(%+v) returned error: %v, wantErr: %v", test.cluster, gotErr, test.wantErr)
			}
			if !reflect.DeepEqual(gotUpdate, test.wantUpdate) {
				t.Errorf("validateCluster(%+v) = %v, want: %v", test.cluster, gotUpdate, test.wantUpdate)
			}
		})
	}
}

// TestCDSHandleResponse starts a fake xDS server, makes a ClientConn to it,
// and creates a v2Client using it. Then, it registers a CDS watcher and tests
// different CDS responses.
func TestCDSHandleResponse(t *testing.T) {
	fakeServer, sCleanup := fakexds.StartServer(t)
	client, cCleanup := fakeServer.GetClientConn(t)
	defer func() {
		cCleanup()
		sCleanup()
	}()
	v2c := newV2Client(client, goodNodeProto, func(int) time.Duration { return 0 })
	defer v2c.close()

	tests := []struct {
		name          string
		cdsResponse   *xdspb.DiscoveryResponse
		wantErr       bool
		wantUpdate    *CDSUpdate
		wantUpdateErr bool
	}{
		// Badly marshaled CDS response.
		{
			name:          "badly-marshaled-response",
			cdsResponse:   badlyMarshaledCDSResponse,
			wantErr:       true,
			wantUpdate:    nil,
			wantUpdateErr: false,
		},
		// Response does not contain Cluster proto.
		{
			name:          "no-cluster-proto-in-response",
			cdsResponse:   badResourceTypeInLDSResponse,
			wantErr:       true,
			wantUpdate:    nil,
			wantUpdateErr: false,
		},
		// Response contains no clusters.
		{
			name:          "no-cluster",
			cdsResponse:   &xdspb.DiscoveryResponse{},
			wantErr:       false,
			wantUpdate:    &CDSUpdate{},
			wantUpdateErr: true,
		},
		// Response contains one good cluster we are not interested in.
		{
			name:          "one-uninteresting-cluster",
			cdsResponse:   goodCDSResponse2,
			wantErr:       false,
			wantUpdate:    &CDSUpdate{},
			wantUpdateErr: true,
		},
		// Response contains one cluster and it is good.
		{
			name:          "one-good-cluster",
			cdsResponse:   goodCDSResponse1,
			wantErr:       false,
			wantUpdate:    &CDSUpdate{ServiceName: serviceName1, EnableLRS: true},
			wantUpdateErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotUpdateCh := make(chan CDSUpdate, 1)
			gotUpdateErrCh := make(chan error, 1)

			// Register a watcher, to trigger the v2Client to send an CDS request.
			cancelWatch := v2c.watchCDS(clusterName1, func(u CDSUpdate, err error) {
				t.Logf("in v2c.watchCDS callback, CDSUpdate: %+v, err: %v", u, err)
				gotUpdateCh <- u
				gotUpdateErrCh <- err
			})

			// Wait till the request makes it to the fakeServer. This ensures that
			// the watch request has been processed by the v2Client.
			<-fakeServer.RequestChan

			// Directly push the response through a call to handleLDSResponse,
			// thereby bypassing the fakeServer.
			if err := v2c.handleCDSResponse(test.cdsResponse); (err != nil) != test.wantErr {
				t.Fatalf("v2c.handleCDSResponse() returned err: %v, wantErr: %v", err, test.wantErr)
			}

			// If the test needs the callback to be invoked, verify the update and
			// error pushed to the callback.
			if test.wantUpdate != nil {
				timer := time.NewTimer(defaultTestTimeout)
				select {
				case <-timer.C:
					t.Fatal("Timeout when expecting CDS update")
				case gotUpdate := <-gotUpdateCh:
					timer.Stop()
					if !reflect.DeepEqual(gotUpdate, *test.wantUpdate) {
						t.Fatalf("got CDS update : %+v, want %+v", gotUpdate, test.wantUpdate)
					}
				}
				// Since the callback that we registered pushes to both channels at
				// the same time, this channel read should return immediately.
				gotUpdateErr := <-gotUpdateErrCh
				if (gotUpdateErr != nil) != test.wantUpdateErr {
					t.Fatalf("got CDS update error {%v}, wantErr: %v", gotUpdateErr, test.wantUpdateErr)
				}
			}
			cancelWatch()
		})
	}
}

// TestCDSHandleResponseWithoutWatch tests the case where the v2Client receives
// a CDS response without a registered watcher.
func TestCDSHandleResponseWithoutWatch(t *testing.T) {
	fakeServer, sCleanup := fakexds.StartServer(t)
	client, cCleanup := fakeServer.GetClientConn(t)
	defer func() {
		cCleanup()
		sCleanup()
	}()
	v2c := newV2Client(client, goodNodeProto, func(int) time.Duration { return 0 })
	defer v2c.close()

	if v2c.handleCDSResponse(goodCDSResponse1) == nil {
		t.Fatal("v2c.handleCDSResponse() succeeded, should have failed")
	}
}

// cdsTestOp contains all data related to one particular test operation. Not
// all fields make sense for all tests.
type cdsTestOp struct {
	// target is the resource name to watch for.
	target string
	// responseToSend is the xDS response sent to the client
	responseToSend *fakexds.Response
	// wantOpErr specfies whether the main operation should return an error.
	wantOpErr bool
	// wantCDSCache is the expected rdsCache at the end of an operation.
	wantCDSCache map[string]CDSUpdate
	// wantWatchCallback specifies if the watch callback should be invoked.
	wantWatchCallback bool
}

// testCDSCaching is a helper function which starts a fake xDS server, makes a
// ClientConn to it, creates a v2Client using it.  It then reads a bunch of
// test operations to be performed from cdsTestOps and returns error, if any,
// on the provided error channel. This is executed in a separate goroutine.
func testCDSCaching(t *testing.T, cdsTestOps []cdsTestOp, errCh chan error) {
	t.Helper()

	fakeServer, sCleanup := fakexds.StartServer(t)
	client, cCleanup := fakeServer.GetClientConn(t)
	defer func() {
		cCleanup()
		sCleanup()
	}()
	v2c := newV2Client(client, goodNodeProto, func(int) time.Duration { return 0 })
	defer v2c.close()
	t.Log("Started xds v2Client...")

	callbackCh := make(chan struct{}, 1)
	for _, cdsTestOp := range cdsTestOps {
		// Register a watcher if required, and use a channel to signal the
		// successful invocation of the callback.
		if cdsTestOp.target != "" {
			v2c.watchCDS(cdsTestOp.target, func(u CDSUpdate, err error) {
				t.Logf("Received callback with CDSUpdate {%+v} and error {%v}", u, err)
				callbackCh <- struct{}{}
			})
			t.Logf("Registered a watcher for CDS target: %v...", cdsTestOp.target)

			// Wait till the request makes it to the fakeServer. This ensures that
			// the watch request has been processed by the v2Client.
			<-fakeServer.RequestChan
			t.Log("FakeServer received request...")
		}

		// Directly push the response through a call to handleCDSResponse,
		// thereby bypassing the fakeServer.
		if cdsTestOp.responseToSend != nil {
			if err := v2c.handleCDSResponse(cdsTestOp.responseToSend.Resp); (err != nil) != cdsTestOp.wantOpErr {
				errCh <- fmt.Errorf("v2c.handleCDSResponse() returned err: %v", err)
				return
			}
		}

		// If the test needs the callback to be invoked, just verify that
		// it was invoked. Since we verify the contents of the cache, it's
		// ok not to verify the contents of the callback.
		if cdsTestOp.wantWatchCallback {
			<-callbackCh
		}

		if !reflect.DeepEqual(v2c.cloneCDSCacheForTesting(), cdsTestOp.wantCDSCache) {
			errCh <- fmt.Errorf("gotCDSCache: %v, wantCDSCache: %v", v2c.rdsCache, cdsTestOp.wantCDSCache)
			return
		}
	}
	t.Log("Completed all test ops successfully...")
	errCh <- nil
}

// TestCDSCaching tests some end-to-end CDS flows using a fake xDS server, and
// verifies the CDS data cached at the v2Client.
func TestCDSCaching(t *testing.T) {
	errCh := make(chan error, 1)
	ops := []cdsTestOp{
		// Add an CDS watch for a cluster name (clusterName1), which returns one
		// matching resource in the response.
		{
			target:         clusterName1,
			responseToSend: &fakexds.Response{Resp: goodCDSResponse1},
			wantCDSCache: map[string]CDSUpdate{
				clusterName1: {serviceName1, true},
			},
			wantWatchCallback: true,
		},
		// Push an CDS response which contains a new resource (apart from the
		// one received in the previous response). This should be cached.
		{
			responseToSend: &fakexds.Response{Resp: cdsResponseWithMultipleResources},
			wantCDSCache: map[string]CDSUpdate{
				clusterName1: {serviceName1, true},
				clusterName2: {serviceName2, false},
			},
			wantWatchCallback: true,
		},
		// Switch the watch target to clusterName2, which was already cached.  No
		// response is received from the server (as expected), but we want the
		// callback to be invoked with the new serviceName.
		{
			target: clusterName2,
			wantCDSCache: map[string]CDSUpdate{
				clusterName1: {serviceName1, true},
				clusterName2: {serviceName2, false},
			},
			wantWatchCallback: true,
		},
		// Push an empty CDS response. This should clear the cache.
		{
			responseToSend:    &fakexds.Response{Resp: &xdspb.DiscoveryResponse{TypeUrl: cdsURL}},
			wantOpErr:         false,
			wantCDSCache:      map[string]CDSUpdate{},
			wantWatchCallback: true,
		},
	}
	go testCDSCaching(t, ops, errCh)

	timer := time.NewTimer(defaultTestTimeout)
	select {
	case <-timer.C:
		t.Fatal("Timeout when expecting CDS update")
	case err := <-errCh:
		timer.Stop()
		if err != nil {
			t.Fatal(err)
		}
	}
}

// TestCDSWatchExpiryTimer tests the case where the client does not receive an
// CDS response for the request that it sends out. We want the watch callback
// to be invoked with an error once the watchExpiryTimer fires.
func TestCDSWatchExpiryTimer(t *testing.T) {
	oldWatchExpiryTimeout := defaultWatchExpiryTimeout
	defaultWatchExpiryTimeout = 1 * time.Second
	defer func() {
		defaultWatchExpiryTimeout = oldWatchExpiryTimeout
	}()

	fakeServer, sCleanup := fakexds.StartServer(t)
	client, cCleanup := fakeServer.GetClientConn(t)
	defer func() {
		cCleanup()
		sCleanup()
	}()
	v2c := newV2Client(client, goodNodeProto, func(int) time.Duration { return 0 })
	defer v2c.close()
	t.Log("Started xds v2Client...")

	cdsCallbackCh := make(chan error, 1)
	v2c.watchCDS(clusterName1, func(u CDSUpdate, err error) {
		t.Logf("Received callback with CDSUpdate {%+v} and error {%v}", u, err)
		if u.ServiceName != "" {
			cdsCallbackCh <- fmt.Errorf("received serviceName %v in cdsCallback, wanted empty string", u.ServiceName)
		}
		if err == nil {
			cdsCallbackCh <- errors.New("received nil error in cdsCallback")
		}
		cdsCallbackCh <- nil
	})
	<-fakeServer.RequestChan

	timer := time.NewTimer(2 * time.Second)
	select {
	case <-timer.C:
		t.Fatalf("Timeout expired when expecting CDS update")
	case err := <-cdsCallbackCh:
		timer.Stop()
		if err != nil {
			t.Fatal(err)
		}
	}
}

var (
	badlyMarshaledCDSResponse = &xdspb.DiscoveryResponse{
		Resources: []*anypb.Any{
			{
				TypeUrl: cdsURL,
				Value:   []byte{1, 2, 3, 4},
			},
		},
		TypeUrl: cdsURL,
	}
	goodCluster1 = &xdspb.Cluster{
		Name:                 clusterName1,
		ClusterDiscoveryType: &xdspb.Cluster_Type{Type: xdspb.Cluster_EDS},
		EdsClusterConfig: &xdspb.Cluster_EdsClusterConfig{
			EdsConfig: &corepb.ConfigSource{
				ConfigSourceSpecifier: &corepb.ConfigSource_Ads{
					Ads: &corepb.AggregatedConfigSource{},
				},
			},
			ServiceName: serviceName1,
		},
		LbPolicy: xdspb.Cluster_ROUND_ROBIN,
		LrsServer: &corepb.ConfigSource{
			ConfigSourceSpecifier: &corepb.ConfigSource_Self{
				Self: &corepb.SelfConfigSource{},
			},
		},
	}
	marshaledCluster1, _ = proto.Marshal(goodCluster1)
	goodCluster2         = &xdspb.Cluster{
		Name:                 clusterName2,
		ClusterDiscoveryType: &xdspb.Cluster_Type{Type: xdspb.Cluster_EDS},
		EdsClusterConfig: &xdspb.Cluster_EdsClusterConfig{
			EdsConfig: &corepb.ConfigSource{
				ConfigSourceSpecifier: &corepb.ConfigSource_Ads{
					Ads: &corepb.AggregatedConfigSource{},
				},
			},
			ServiceName: serviceName2,
		},
		LbPolicy: xdspb.Cluster_ROUND_ROBIN,
	}
	marshaledCluster2, _ = proto.Marshal(goodCluster2)
	goodCDSResponse1     = &xdspb.DiscoveryResponse{
		Resources: []*anypb.Any{
			{
				TypeUrl: cdsURL,
				Value:   marshaledCluster1,
			},
		},
		TypeUrl: cdsURL,
	}
	goodCDSResponse2 = &xdspb.DiscoveryResponse{
		Resources: []*anypb.Any{
			{
				TypeUrl: cdsURL,
				Value:   marshaledCluster2,
			},
		},
		TypeUrl: cdsURL,
	}
	cdsResponseWithMultipleResources = &xdspb.DiscoveryResponse{
		Resources: []*anypb.Any{
			{
				TypeUrl: cdsURL,
				Value:   marshaledCluster1,
			},
			{
				TypeUrl: cdsURL,
				Value:   marshaledCluster2,
			},
		},
		TypeUrl: cdsURL,
	}
)
