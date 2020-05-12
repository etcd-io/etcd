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
	"google.golang.org/grpc/xds/internal/client/fakexds"
)

func TestLDSGetRouteConfig(t *testing.T) {
	tests := []struct {
		name      string
		lis       *xdspb.Listener
		wantRoute string
		wantErr   bool
	}{
		{
			name:      "no-apiListener-field",
			lis:       &xdspb.Listener{},
			wantRoute: "",
			wantErr:   true,
		},
		{
			name:      "badly-marshaled-apiListener",
			lis:       badAPIListener1,
			wantRoute: "",
			wantErr:   true,
		},
		{
			name:      "wrong-type-in-apiListener",
			lis:       badResourceListener,
			wantRoute: "",
			wantErr:   true,
		},
		{
			name:      "empty-httpConnMgr-in-apiListener",
			lis:       listenerWithEmptyHTTPConnMgr,
			wantRoute: "",
			wantErr:   true,
		},
		{
			name:      "scopedRoutes-routeConfig-in-apiListener",
			lis:       listenerWithScopedRoutesRouteConfig,
			wantRoute: "",
			wantErr:   true,
		},
		{
			name:      "goodListener1",
			lis:       goodListener1,
			wantRoute: goodRouteName1,
			wantErr:   false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotRoute, err := getRouteConfigNameFromListener(test.lis)
			if gotRoute != test.wantRoute {
				t.Errorf("getRouteConfigNameFromListener(%+v) = %v, want %v", test.lis, gotRoute, test.wantRoute)
			}
			if (err != nil) != test.wantErr {
				t.Errorf("getRouteConfigNameFromListener(%+v) = %v, want %v", test.lis, err, test.wantErr)
			}
		})
	}
}

// TestLDSHandleResponse starts a fake xDS server, makes a ClientConn to it,
// and creates a v2Client using it. Then, it registers a watchLDS and tests
// different LDS responses.
func TestLDSHandleResponse(t *testing.T) {
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
		ldsResponse   *xdspb.DiscoveryResponse
		wantErr       bool
		wantUpdate    *ldsUpdate
		wantUpdateErr bool
	}{
		// Badly marshaled LDS response.
		{
			name:          "badly-marshaled-response",
			ldsResponse:   badlyMarshaledLDSResponse,
			wantErr:       true,
			wantUpdate:    nil,
			wantUpdateErr: false,
		},
		// Response does not contain Listener proto.
		{
			name:          "no-listener-proto-in-response",
			ldsResponse:   badResourceTypeInLDSResponse,
			wantErr:       true,
			wantUpdate:    nil,
			wantUpdateErr: false,
		},
		// No APIListener in the response. Just one test case here for a bad
		// ApiListener, since the others are covered in
		// TestGetRouteConfigNameFromListener.
		{
			name:          "no-apiListener-in-response",
			ldsResponse:   noAPIListenerLDSResponse,
			wantErr:       true,
			wantUpdate:    nil,
			wantUpdateErr: false,
		},
		// Response contains one listener and it is good.
		{
			name:          "one-good-listener",
			ldsResponse:   goodLDSResponse1,
			wantErr:       false,
			wantUpdate:    &ldsUpdate{routeName: goodRouteName1},
			wantUpdateErr: false,
		},
		// Response contains multiple good listeners, including the one we are
		// interested in.
		{
			name:          "multiple-good-listener",
			ldsResponse:   ldsResponseWithMultipleResources,
			wantErr:       false,
			wantUpdate:    &ldsUpdate{routeName: goodRouteName1},
			wantUpdateErr: false,
		},
		// Response contains two good listeners (one interesting and one
		// uninteresting), and one badly marshaled listener.
		{
			name:          "good-bad-ugly-listeners",
			ldsResponse:   goodBadUglyLDSResponse,
			wantErr:       false,
			wantUpdate:    &ldsUpdate{routeName: goodRouteName1},
			wantUpdateErr: false,
		},
		// Response contains one listener, but we are not interested in it.
		{
			name:          "one-uninteresting-listener",
			ldsResponse:   goodLDSResponse2,
			wantErr:       false,
			wantUpdate:    &ldsUpdate{routeName: ""},
			wantUpdateErr: true,
		},
		// Response constains no resources. This is the case where the server
		// does not know about the target we are interested in.
		{
			name:          "empty-response",
			ldsResponse:   emptyLDSResponse,
			wantErr:       false,
			wantUpdate:    &ldsUpdate{routeName: ""},
			wantUpdateErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotUpdateCh := make(chan ldsUpdate, 1)
			gotUpdateErrCh := make(chan error, 1)

			// Register a watcher, to trigger the v2Client to send an LDS request.
			cancelWatch := v2c.watchLDS(goodLDSTarget1, func(u ldsUpdate, err error) {
				t.Logf("in v2c.watchLDS callback, ldsUpdate: %+v, err: %v", u, err)
				gotUpdateCh <- u
				gotUpdateErrCh <- err
			})

			// Wait till the request makes it to the fakeServer. This ensures that
			// the watch request has been processed by the v2Client.
			<-fakeServer.RequestChan

			// Directly push the response through a call to handleLDSResponse,
			// thereby bypassing the fakeServer.
			if err := v2c.handleLDSResponse(test.ldsResponse); (err != nil) != test.wantErr {
				t.Fatalf("v2c.handleLDSResponse() returned err: %v, wantErr: %v", err, test.wantErr)
			}

			// If the test needs the callback to be invoked, verify the update and
			// error pushed to the callback.
			if test.wantUpdate != nil {
				timer := time.NewTimer(defaultTestTimeout)
				select {
				case <-timer.C:
					t.Fatal("Timeout when expecting LDS update")
				case gotUpdate := <-gotUpdateCh:
					timer.Stop()
					if !reflect.DeepEqual(gotUpdate, *test.wantUpdate) {
						t.Fatalf("got LDS update : %+v, want %+v", gotUpdate, *test.wantUpdate)
					}
				}
				// Since the callback that we registered pushes to both channels at
				// the same time, this channel read should return immediately.
				gotUpdateErr := <-gotUpdateErrCh
				if (gotUpdateErr != nil) != test.wantUpdateErr {
					t.Fatalf("got LDS update error {%v}, wantErr: %v", gotUpdateErr, test.wantUpdateErr)
				}
			}
			cancelWatch()
		})
	}
}

// TestLDSHandleResponseWithoutWatch tests the case where the v2Client receives
// an LDS response without a registered watcher.
func TestLDSHandleResponseWithoutWatch(t *testing.T) {
	fakeServer, sCleanup := fakexds.StartServer(t)
	client, cCleanup := fakeServer.GetClientConn(t)
	defer func() {
		cCleanup()
		sCleanup()
	}()
	v2c := newV2Client(client, goodNodeProto, func(int) time.Duration { return 0 })
	defer v2c.close()

	if v2c.handleLDSResponse(goodLDSResponse1) == nil {
		t.Fatal("v2c.handleLDSResponse() succeeded, should have failed")
	}
}

// TestLDSWatchExpiryTimer tests the case where the client does not receive an
// LDS response for the request that it sends out. We want the watch callback
// to be invoked with an error once the watchExpiryTimer fires.
func TestLDSWatchExpiryTimer(t *testing.T) {
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

	// Wait till the request makes it to the fakeServer. This ensures that
	// the watch request has been processed by the v2Client.
	callbackCh := make(chan error, 1)
	v2c.watchLDS(goodLDSTarget1, func(u ldsUpdate, err error) {
		t.Logf("in v2c.watchLDS callback, ldsUpdate: %+v, err: %v", u, err)
		if u.routeName != "" {
			callbackCh <- fmt.Errorf("received routeName %v in ldsCallback, wanted empty string", u.routeName)
		}
		if err == nil {
			callbackCh <- errors.New("received nil error in ldsCallback")
		}
		callbackCh <- nil
	})
	<-fakeServer.RequestChan

	timer := time.NewTimer(2 * time.Second)
	select {
	case <-timer.C:
		t.Fatalf("Timeout expired when expecting LDS update")
	case err := <-callbackCh:
		timer.Stop()
		if err != nil {
			t.Fatal(err)
		}
	}
}
