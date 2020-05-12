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
	routepb "github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	"google.golang.org/grpc/xds/internal/client/fakexds"
)

func (v2c *v2Client) cloneRDSCacheForTesting() map[string]string {
	v2c.mu.Lock()
	defer v2c.mu.Unlock()

	cloneCache := make(map[string]string)
	for k, v := range v2c.rdsCache {
		cloneCache[k] = v
	}
	return cloneCache
}

func TestRDSGetClusterFromRouteConfiguration(t *testing.T) {
	tests := []struct {
		name        string
		rc          *xdspb.RouteConfiguration
		wantCluster string
	}{
		{
			name:        "no-virtual-hosts-in-rc",
			rc:          emptyRouteConfig,
			wantCluster: "",
		},
		{
			name:        "no-domains-in-rc",
			rc:          noDomainsInRouteConfig,
			wantCluster: "",
		},
		{
			name: "non-matching-domain-in-rc",
			rc: &xdspb.RouteConfiguration{
				VirtualHosts: []*routepb.VirtualHost{
					{Domains: []string{uninterestingDomain}},
				},
			},
			wantCluster: "",
		},
		{
			name: "no-routes-in-rc",
			rc: &xdspb.RouteConfiguration{
				VirtualHosts: []*routepb.VirtualHost{
					{Domains: []string{goodMatchingDomain}},
				},
			},
			wantCluster: "",
		},
		{
			name: "default-route-match-field-is-nil",
			rc: &xdspb.RouteConfiguration{
				VirtualHosts: []*routepb.VirtualHost{
					{
						Domains: []string{goodMatchingDomain},
						Routes: []*routepb.Route{
							{
								Action: &routepb.Route_Route{
									Route: &routepb.RouteAction{
										ClusterSpecifier: &routepb.RouteAction_Cluster{Cluster: goodClusterName1},
									},
								},
							},
						},
					},
				},
			},
			wantCluster: "",
		},
		{
			name: "default-route-match-field-is-non-nil",
			rc: &xdspb.RouteConfiguration{
				VirtualHosts: []*routepb.VirtualHost{
					{
						Domains: []string{goodMatchingDomain},
						Routes: []*routepb.Route{
							{
								Match:  &routepb.RouteMatch{},
								Action: &routepb.Route_Route{},
							},
						},
					},
				},
			},
			wantCluster: "",
		},
		{
			name: "default-route-routeaction-field-is-nil",
			rc: &xdspb.RouteConfiguration{
				VirtualHosts: []*routepb.VirtualHost{
					{
						Domains: []string{goodMatchingDomain},
						Routes:  []*routepb.Route{{}},
					},
				},
			},
			wantCluster: "",
		},
		{
			name: "default-route-cluster-field-is-empty",
			rc: &xdspb.RouteConfiguration{
				VirtualHosts: []*routepb.VirtualHost{
					{
						Domains: []string{goodMatchingDomain},
						Routes: []*routepb.Route{
							{
								Action: &routepb.Route_Route{
									Route: &routepb.RouteAction{
										ClusterSpecifier: &routepb.RouteAction_ClusterHeader{},
									},
								},
							},
						},
					},
				},
			},
			wantCluster: "",
		},
		{
			name:        "good-route-config",
			rc:          goodRouteConfig1,
			wantCluster: goodClusterName1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if gotCluster := getClusterFromRouteConfiguration(test.rc, goodLDSTarget1); gotCluster != test.wantCluster {
				t.Errorf("getClusterFromRouteConfiguration(%+v, %v) = %v, want %v", test.rc, goodLDSTarget1, gotCluster, test.wantCluster)
			}
		})
	}
}

// TestRDSHandleResponse starts a fake xDS server, makes a ClientConn to it,
// and creates a v2Client using it. Then, it registers an LDS and RDS watcher
// and tests different RDS responses.
func TestRDSHandleResponse(t *testing.T) {
	fakeServer, sCleanup := fakexds.StartServer(t)
	client, cCleanup := fakeServer.GetClientConn(t)
	defer func() {
		cCleanup()
		sCleanup()
	}()
	v2c := newV2Client(client, goodNodeProto, func(int) time.Duration { return 0 })
	defer v2c.close()

	// Register an LDS watcher, and wait till the request is sent out, the
	// response is received and the callback is invoked.
	cbCh := make(chan error, 1)
	v2c.watchLDS(goodLDSTarget1, func(u ldsUpdate, err error) {
		t.Logf("v2c.watchLDS callback, ldsUpdate: %+v, err: %v", u, err)
		cbCh <- err
	})
	<-fakeServer.RequestChan
	fakeServer.ResponseChan <- &fakexds.Response{Resp: goodLDSResponse1}
	if err := <-cbCh; err != nil {
		t.Fatalf("v2c.watchLDS returned error in callback: %v", err)
	}

	tests := []struct {
		name          string
		rdsResponse   *xdspb.DiscoveryResponse
		wantErr       bool
		wantUpdate    *rdsUpdate
		wantUpdateErr bool
	}{
		// Badly marshaled RDS response.
		{
			name:          "badly-marshaled-response",
			rdsResponse:   badlyMarshaledRDSResponse,
			wantErr:       true,
			wantUpdate:    nil,
			wantUpdateErr: false,
		},
		// Response does not contain RouteConfiguration proto.
		{
			name:          "no-route-config-in-response",
			rdsResponse:   badResourceTypeInRDSResponse,
			wantErr:       true,
			wantUpdate:    nil,
			wantUpdateErr: false,
		},
		// No VirtualHosts in the response. Just one test case here for a bad
		// RouteConfiguration, since the others are covered in
		// TestGetClusterFromRouteConfiguration.
		{
			name:          "no-virtual-hosts-in-response",
			rdsResponse:   noVirtualHostsInRDSResponse,
			wantErr:       true,
			wantUpdate:    nil,
			wantUpdateErr: false,
		},
		// Response contains one good RouteConfiguration, uninteresting though.
		{
			name:          "one-uninteresting-route-config",
			rdsResponse:   goodRDSResponse2,
			wantErr:       false,
			wantUpdate:    nil,
			wantUpdateErr: false,
		},
		// Response contains one good interesting RouteConfiguration.
		{
			name:          "one-good-route-config",
			rdsResponse:   goodRDSResponse1,
			wantErr:       false,
			wantUpdate:    &rdsUpdate{clusterName: goodClusterName1},
			wantUpdateErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotUpdateCh := make(chan rdsUpdate, 1)
			gotUpdateErrCh := make(chan error, 1)

			// Register a watcher, to trigger the v2Client to send an RDS request.
			cancelWatch := v2c.watchRDS(goodRouteName1, func(u rdsUpdate, err error) {
				t.Logf("in v2c.watchRDS callback, rdsUpdate: %+v, err: %v", u, err)
				gotUpdateCh <- u
				gotUpdateErrCh <- err
			})

			// Wait till the request makes it to the fakeServer. This ensures that
			// the watch request has been processed by the v2Client.
			<-fakeServer.RequestChan

			// Directly push the response through a call to handleRDSResponse,
			// thereby bypassing the fakeServer.
			if err := v2c.handleRDSResponse(test.rdsResponse); (err != nil) != test.wantErr {
				t.Fatalf("v2c.handleRDSResponse() returned err: %v, wantErr: %v", err, test.wantErr)
			}

			// If the test needs the callback to be invoked, verify the update and
			// error pushed to the callback.
			if test.wantUpdate != nil {
				timer := time.NewTimer(defaultTestTimeout)
				select {
				case <-timer.C:
					t.Fatal("Timeout when expecting RDS update")
				case gotUpdate := <-gotUpdateCh:
					timer.Stop()
					if !reflect.DeepEqual(gotUpdate, *test.wantUpdate) {
						t.Fatalf("got RDS update : %+v, want %+v", gotUpdate, *test.wantUpdate)
					}
				}
				// Since the callback that we registered pushes to both channels at
				// the same time, this channel read should return immediately.
				gotUpdateErr := <-gotUpdateErrCh
				if (gotUpdateErr != nil) != test.wantUpdateErr {
					t.Fatalf("got RDS update error {%v}, wantErr: %v", gotUpdateErr, test.wantUpdateErr)
				}
			}
			cancelWatch()
		})
	}
}

// TestRDSHandleResponseWithoutLDSWatch tests the case where the v2Client
// receives an RDS response without a registered LDS watcher.
func TestRDSHandleResponseWithoutLDSWatch(t *testing.T) {
	fakeServer, sCleanup := fakexds.StartServer(t)
	client, cCleanup := fakeServer.GetClientConn(t)
	defer func() {
		cCleanup()
		sCleanup()
	}()
	v2c := newV2Client(client, goodNodeProto, func(int) time.Duration { return 0 })
	defer v2c.close()

	if v2c.handleRDSResponse(goodRDSResponse1) == nil {
		t.Fatal("v2c.handleRDSResponse() succeeded, should have failed")
	}
}

// TestRDSHandleResponseWithoutRDSWatch tests the case where the v2Client
// receives an RDS response without a registered RDS watcher.
func TestRDSHandleResponseWithoutRDSWatch(t *testing.T) {
	fakeServer, sCleanup := fakexds.StartServer(t)
	client, cCleanup := fakeServer.GetClientConn(t)
	defer func() {
		cCleanup()
		sCleanup()
	}()
	v2c := newV2Client(client, goodNodeProto, func(int) time.Duration { return 0 })
	defer v2c.close()

	// Register an LDS watcher, and wait till the request is sent out, the
	// response is received and the callback is invoked.
	cbCh := make(chan error, 1)
	v2c.watchLDS(goodLDSTarget1, func(u ldsUpdate, err error) {
		t.Logf("v2c.watchLDS callback, ldsUpdate: %+v, err: %v", u, err)
		cbCh <- err
	})
	<-fakeServer.RequestChan
	fakeServer.ResponseChan <- &fakexds.Response{Resp: goodLDSResponse1}
	if err := <-cbCh; err != nil {
		t.Fatalf("v2c.watchLDS returned error in callback: %v", err)
	}

	if v2c.handleRDSResponse(goodRDSResponse1) == nil {
		t.Fatal("v2c.handleRDSResponse() succeeded, should have failed")
	}
}

// rdsTestOp contains all data related to one particular test operation. Not
// all fields make sense for all tests.
type rdsTestOp struct {
	// target is the resource name to watch for.
	target string
	// responseToSend is the xDS response sent to the client
	responseToSend *fakexds.Response
	// wantOpErr specfies whether the main operation should return an error.
	wantOpErr bool
	// wantRDSCache is the expected rdsCache at the end of an operation.
	wantRDSCache map[string]string
	// wantWatchCallback specifies if the watch callback should be invoked.
	wantWatchCallback bool
}

// testRDSCaching is a helper function which starts a fake xDS server, makes a
// ClientConn to it, creates a v2Client using it, registers an LDS watcher and
// pushes a good LDS response. It then reads a bunch of test operations to be
// performed from rdsTestOps and returns error, if any, on the provided error
// channel. This is executed in a separate goroutine.
func testRDSCaching(t *testing.T, rdsTestOps []rdsTestOp, errCh chan error) {
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

	// Register an LDS watcher, and wait till the request is sent out, the
	// response is received and the callback is invoked.
	cbCh := make(chan error, 1)
	v2c.watchLDS(goodLDSTarget1, func(u ldsUpdate, err error) {
		t.Logf("v2c.watchLDS callback, ldsUpdate: %+v, err: %v", u, err)
		cbCh <- err
	})
	<-fakeServer.RequestChan
	fakeServer.ResponseChan <- &fakexds.Response{Resp: goodLDSResponse1}
	if err := <-cbCh; err != nil {
		errCh <- fmt.Errorf("v2c.watchLDS returned error in callback: %v", err)
		return
	}

	callbackCh := make(chan struct{}, 1)
	for _, rdsTestOp := range rdsTestOps {
		// Register a watcher if required, and use a channel to signal the
		// successful invocation of the callback.
		if rdsTestOp.target != "" {
			v2c.watchRDS(rdsTestOp.target, func(u rdsUpdate, err error) {
				t.Logf("Received callback with rdsUpdate {%+v} and error {%v}", u, err)
				callbackCh <- struct{}{}
			})
			t.Logf("Registered a watcher for RDS target: %v...", rdsTestOp.target)

			// Wait till the request makes it to the fakeServer. This ensures that
			// the watch request has been processed by the v2Client.
			<-fakeServer.RequestChan
			t.Log("FakeServer received request...")
		}

		// Directly push the response through a call to handleRDSResponse,
		// thereby bypassing the fakeServer.
		if rdsTestOp.responseToSend != nil {
			if err := v2c.handleRDSResponse(rdsTestOp.responseToSend.Resp); (err != nil) != rdsTestOp.wantOpErr {
				errCh <- fmt.Errorf("v2c.handleRDSResponse() returned err: %v", err)
				return
			}
		}

		// If the test needs the callback to be invoked, just verify that
		// it was invoked. Since we verify the contents of the cache, it's
		// ok not to verify the contents of the callback.
		if rdsTestOp.wantWatchCallback {
			<-callbackCh
		}

		if !reflect.DeepEqual(v2c.cloneRDSCacheForTesting(), rdsTestOp.wantRDSCache) {
			errCh <- fmt.Errorf("gotRDSCache: %v, wantRDSCache: %v", v2c.rdsCache, rdsTestOp.wantRDSCache)
			return
		}
	}
	t.Log("Completed all test ops successfully...")
	errCh <- nil
}

// TestRDSCaching tests some end-to-end RDS flows using a fake xDS server, and
// verifies the RDS data cached at the v2Client.
func TestRDSCaching(t *testing.T) {
	errCh := make(chan error, 1)
	ops := []rdsTestOp{
		// Add an RDS watch for a resource name (goodRouteName1), which returns one
		// matching resource in the response.
		{
			target:            goodRouteName1,
			responseToSend:    &fakexds.Response{Resp: goodRDSResponse1},
			wantRDSCache:      map[string]string{goodRouteName1: goodClusterName1},
			wantWatchCallback: true,
		},
		// Push an RDS response with a new resource. This resource is considered
		// good because its domain field matches our LDS watch target, but the
		// routeConfigName does not match our RDS watch (so the watch callback will
		// not be invoked). But this should still be cached.
		{
			responseToSend: &fakexds.Response{Resp: goodRDSResponse2},
			wantRDSCache: map[string]string{
				goodRouteName1: goodClusterName1,
				goodRouteName2: goodClusterName2,
			},
		},
		// Push an uninteresting RDS response. This should cause handleRDSResponse
		// to return an error. But the watch callback should not be invoked, and
		// the cache should not be updated.
		{
			responseToSend: &fakexds.Response{Resp: uninterestingRDSResponse},
			wantOpErr:      true,
			wantRDSCache: map[string]string{
				goodRouteName1: goodClusterName1,
				goodRouteName2: goodClusterName2,
			},
		},
		// Switch the watch target to goodRouteName2, which was already cached.  No
		// response is received from the server (as expected), but we want the
		// callback to be invoked with the new clusterName.
		{
			target: goodRouteName2,
			wantRDSCache: map[string]string{
				goodRouteName1: goodClusterName1,
				goodRouteName2: goodClusterName2,
			},
			wantWatchCallback: true,
		},
	}
	go testRDSCaching(t, ops, errCh)

	timer := time.NewTimer(defaultTestTimeout)
	select {
	case <-timer.C:
		t.Fatal("Timeout when expecting RDS update")
	case err := <-errCh:
		timer.Stop()
		if err != nil {
			t.Fatal(err)
		}
	}
}

// TestRDSWatchExpiryTimer tests the case where the client does not receive an
// RDS response for the request that it sends out. We want the watch callback
// to be invoked with an error once the watchExpiryTimer fires.
func TestRDSWatchExpiryTimer(t *testing.T) {
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

	// Register an LDS watcher, and wait till the request is sent out, the
	// response is received and the callback is invoked.
	ldsCallbackCh := make(chan struct{})
	v2c.watchLDS(goodLDSTarget1, func(u ldsUpdate, err error) {
		t.Logf("v2c.watchLDS callback, ldsUpdate: %+v, err: %v", u, err)
		close(ldsCallbackCh)
	})
	<-fakeServer.RequestChan
	fakeServer.ResponseChan <- &fakexds.Response{Resp: goodLDSResponse1}
	<-ldsCallbackCh

	// Wait till the request makes it to the fakeServer. This ensures that
	// the watch request has been processed by the v2Client.
	rdsCallbackCh := make(chan error, 1)
	v2c.watchRDS(goodRouteName1, func(u rdsUpdate, err error) {
		t.Logf("Received callback with rdsUpdate {%+v} and error {%v}", u, err)
		if u.clusterName != "" {
			rdsCallbackCh <- fmt.Errorf("received clusterName %v in rdsCallback, wanted empty string", u.clusterName)
		}
		if err == nil {
			rdsCallbackCh <- errors.New("received nil error in rdsCallback")
		}
		rdsCallbackCh <- nil
	})
	<-fakeServer.RequestChan

	timer := time.NewTimer(2 * time.Second)
	select {
	case <-timer.C:
		t.Fatalf("Timeout expired when expecting RDS update")
	case err := <-rdsCallbackCh:
		timer.Stop()
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestHostFromTarget(t *testing.T) {
	tests := []struct {
		name    string
		target  string
		want    string
		wantErr bool
	}{
		{
			name:    "correct",
			target:  "foo.bar.com:1234",
			want:    "foo.bar.com",
			wantErr: false,
		},
		{
			name:    "error",
			target:  "invalid:1234:3421",
			want:    "",
			wantErr: true,
		},
		{
			name:    "correct but missing port",
			target:  "foo.bar.com",
			want:    "foo.bar.com",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := hostFromTarget(tt.target)
			if (err != nil) != tt.wantErr {
				t.Errorf("hostFromTarget() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("hostFromTarget() got = %v, want %v", got, tt.want)
			}
		})
	}
}
