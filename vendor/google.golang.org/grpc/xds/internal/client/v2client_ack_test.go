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
 */

package client

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	xdspb "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/golang/protobuf/proto"
	anypb "github.com/golang/protobuf/ptypes/any"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc/xds/internal/client/fakexds"
)

func emptyChanRecvWithTimeout(ch <-chan struct{}, d time.Duration) error {
	timer := time.NewTimer(d)
	select {
	case <-timer.C:
		return fmt.Errorf("timeout")
	case <-ch:
		timer.Stop()
		return nil
	}
}

func requestChanRecvWithTimeout(ch <-chan *fakexds.Request, d time.Duration) (*fakexds.Request, error) {
	timer := time.NewTimer(d)
	select {
	case <-timer.C:
		return nil, fmt.Errorf("timeout waiting for request")
	case r := <-ch:
		timer.Stop()
		return r, nil
	}
}

// compareXDSRequest reads requests from channel, compare it with want.
func compareXDSRequest(ch <-chan *fakexds.Request, d time.Duration, want *xdspb.DiscoveryRequest, version, nonce string) error {
	r, err := requestChanRecvWithTimeout(ch, d)
	if err != nil {
		return err
	}
	if r.Err != nil {
		return fmt.Errorf("unexpected error from request: %v", r.Err)
	}
	wantClone := proto.Clone(want).(*xdspb.DiscoveryRequest)
	wantClone.VersionInfo = version
	wantClone.ResponseNonce = nonce
	if !cmp.Equal(r.Req, wantClone, cmp.Comparer(proto.Equal)) {
		return fmt.Errorf("received request different from want, diff: %s", cmp.Diff(r.Req, wantClone))
	}
	return nil
}

func sendXDSRespWithVersion(ch chan<- *fakexds.Response, respWithoutVersion *xdspb.DiscoveryResponse, version int) (nonce string) {
	respToSend := proto.Clone(respWithoutVersion).(*xdspb.DiscoveryResponse)
	respToSend.VersionInfo = strconv.Itoa(version)
	nonce = strconv.Itoa(int(time.Now().UnixNano()))
	respToSend.Nonce = nonce
	ch <- &fakexds.Response{Resp: respToSend}
	return
}

// TestV2ClientAck verifies that valid responses are acked, and invalid ones are
// nacked.
//
// This test also verifies the version for different types are independent.
func TestV2ClientAck(t *testing.T) {
	var (
		versionLDS = 1000
		versionRDS = 2000
		versionCDS = 3000
		versionEDS = 4000
	)

	fakeServer, sCleanup := fakexds.StartServer(t)
	client, cCleanup := fakeServer.GetClientConn(t)
	defer func() {
		cCleanup()
		sCleanup()
	}()
	v2c := newV2Client(client, goodNodeProto, func(int) time.Duration { return 0 })
	defer v2c.close()
	t.Log("Started xds v2Client...")

	// Start the watch, send a good response, and check for ack.
	cbLDS := startXDS(t, "LDS", v2c, fakeServer, goodLDSRequest)
	sendGoodResp(t, "LDS", fakeServer, versionLDS, goodLDSResponse1, goodLDSRequest, cbLDS)
	versionLDS++
	cbRDS := startXDS(t, "RDS", v2c, fakeServer, goodRDSRequest)
	sendGoodResp(t, "RDS", fakeServer, versionRDS, goodRDSResponse1, goodRDSRequest, cbRDS)
	versionRDS++
	cbCDS := startXDS(t, "CDS", v2c, fakeServer, goodCDSRequest)
	sendGoodResp(t, "CDS", fakeServer, versionCDS, goodCDSResponse1, goodCDSRequest, cbCDS)
	versionCDS++
	cbEDS := startXDS(t, "EDS", v2c, fakeServer, goodEDSRequest)
	sendGoodResp(t, "EDS", fakeServer, versionEDS, goodEDSResponse1, goodEDSRequest, cbEDS)
	versionEDS++

	// Send a bad response, and check for nack.
	sendBadResp(t, "LDS", fakeServer, versionLDS, goodLDSRequest)
	versionLDS++
	sendBadResp(t, "RDS", fakeServer, versionRDS, goodRDSRequest)
	versionRDS++
	sendBadResp(t, "CDS", fakeServer, versionCDS, goodCDSRequest)
	versionCDS++
	sendBadResp(t, "EDS", fakeServer, versionEDS, goodEDSRequest)
	versionEDS++

	// send another good response, and check for ack, with the new version.
	sendGoodResp(t, "LDS", fakeServer, versionLDS, goodLDSResponse1, goodLDSRequest, cbLDS)
	versionLDS++
	sendGoodResp(t, "RDS", fakeServer, versionRDS, goodRDSResponse1, goodRDSRequest, cbRDS)
	versionRDS++
	sendGoodResp(t, "CDS", fakeServer, versionCDS, goodCDSResponse1, goodCDSRequest, cbCDS)
	versionCDS++
	sendGoodResp(t, "EDS", fakeServer, versionEDS, goodEDSResponse1, goodEDSRequest, cbEDS)
	versionEDS++
}

// startXDS calls watch to send the first request. It then sends a good response
// and checks for ack.
func startXDS(t *testing.T, xdsname string, v2c *v2Client, fakeServer *fakexds.Server, goodReq *xdspb.DiscoveryRequest) <-chan struct{} {
	callbackCh := make(chan struct{}, 1)
	switch xdsname {
	case "LDS":
		v2c.watchLDS(goodLDSTarget1, func(u ldsUpdate, err error) {
			t.Logf("Received %s callback with ldsUpdate {%+v} and error {%v}", xdsname, u, err)
			callbackCh <- struct{}{}
		})
	case "RDS":
		v2c.watchRDS(goodRouteName1, func(u rdsUpdate, err error) {
			t.Logf("Received %s callback with ldsUpdate {%+v} and error {%v}", xdsname, u, err)
			callbackCh <- struct{}{}
		})
	case "CDS":
		v2c.watchCDS(goodClusterName1, func(u CDSUpdate, err error) {
			t.Logf("Received %s callback with ldsUpdate {%+v} and error {%v}", xdsname, u, err)
			callbackCh <- struct{}{}
		})
	case "EDS":
		v2c.watchEDS(goodEDSName, func(u *EDSUpdate, err error) {
			t.Logf("Received %s callback with ldsUpdate {%+v} and error {%v}", xdsname, u, err)
			callbackCh <- struct{}{}
		})
	}

	if err := compareXDSRequest(fakeServer.RequestChan, defaultTestTimeout, goodReq, "", ""); err != nil {
		t.Fatalf("Failed to receive %s request: %v", xdsname, err)
	}
	t.Logf("FakeServer received %s request...", xdsname)
	return callbackCh
}

// sendGoodResp sends the good response, with the given version, and a random
// nonce.
//
// It also waits and checks that the ack request contains the given version, and
// the generated nonce.
func sendGoodResp(t *testing.T, xdsname string, fakeServer *fakexds.Server, version int, goodResp *xdspb.DiscoveryResponse, wantReq *xdspb.DiscoveryRequest, callbackCh <-chan struct{}) {
	nonce := sendXDSRespWithVersion(fakeServer.ResponseChan, goodResp, version)
	t.Logf("Good %s response pushed to fakeServer...", xdsname)

	if err := compareXDSRequest(fakeServer.RequestChan, defaultTestTimeout, wantReq, strconv.Itoa(version), nonce); err != nil {
		t.Errorf("Failed to receive %s request: %v", xdsname, err)
	}
	t.Logf("Good %s response acked", xdsname)
	if err := emptyChanRecvWithTimeout(callbackCh, defaultTestTimeout); err != nil {
		t.Errorf("Timeout when expecting %s update", xdsname)
	}
	t.Logf("Good %s response callback executed", xdsname)
}

// sendBadResp sends a bad response with the given version. This response will
// be nacked, so we expect a request with the previous version (version-1).
//
// But the nonce in request should be the new nonce.
func sendBadResp(t *testing.T, xdsname string, fakeServer *fakexds.Server, version int, wantReq *xdspb.DiscoveryRequest) {
	var typeURL string
	switch xdsname {
	case "LDS":
		typeURL = ldsURL
	case "RDS":
		typeURL = rdsURL
	case "CDS":
		typeURL = cdsURL
	case "EDS":
		typeURL = edsURL
	}
	nonce := sendXDSRespWithVersion(fakeServer.ResponseChan, &xdspb.DiscoveryResponse{
		Resources: []*anypb.Any{{}},
		TypeUrl:   typeURL,
	}, version)
	t.Logf("Bad %s response pushed to fakeServer...", xdsname)
	if err := compareXDSRequest(fakeServer.RequestChan, defaultTestTimeout, wantReq, strconv.Itoa(version-1), nonce); err != nil {
		t.Errorf("Failed to receive %s request: %v", xdsname, err)
	}
	t.Logf("Bad %s response nacked", xdsname)
}

// Test when the first response is invalid, and is nacked, the nack requests
// should have an empty version string.
func TestV2ClientAckFirstIsNack(t *testing.T) {
	var versionLDS = 1000

	fakeServer, sCleanup := fakexds.StartServer(t)
	client, cCleanup := fakeServer.GetClientConn(t)
	defer func() {
		cCleanup()
		sCleanup()
	}()
	v2c := newV2Client(client, goodNodeProto, func(int) time.Duration { return 0 })
	defer v2c.close()
	t.Log("Started xds v2Client...")

	// Start the watch, send a good response, and check for ack.
	cbLDS := startXDS(t, "LDS", v2c, fakeServer, goodLDSRequest)

	nonce := sendXDSRespWithVersion(fakeServer.ResponseChan, &xdspb.DiscoveryResponse{
		Resources: []*anypb.Any{{}},
		TypeUrl:   ldsURL,
	}, versionLDS)
	t.Logf("Bad response pushed to fakeServer...")

	// The expected version string is an empty string, because this is the first
	// response, and it's nacked (so there's no previous ack version).
	if err := compareXDSRequest(fakeServer.RequestChan, defaultTestTimeout, goodLDSRequest, "", nonce); err != nil {
		t.Errorf("Failed to receive request: %v", err)
	}
	t.Logf("Bad response nacked")
	versionLDS++

	sendGoodResp(t, "LDS", fakeServer, versionLDS, goodLDSResponse1, goodLDSRequest, cbLDS)
	versionLDS++
}

// Test when a nack is sent after a new watch, we nack with the previous acked
// version (instead of resetting to empty string).
func TestV2ClientAckNackAfterNewWatch(t *testing.T) {
	var versionLDS = 1000

	fakeServer, sCleanup := fakexds.StartServer(t)
	client, cCleanup := fakeServer.GetClientConn(t)
	defer func() {
		cCleanup()
		sCleanup()
	}()
	v2c := newV2Client(client, goodNodeProto, func(int) time.Duration { return 0 })
	defer v2c.close()
	t.Log("Started xds v2Client...")

	// Start the watch, send a good response, and check for ack.
	cbLDS := startXDS(t, "LDS", v2c, fakeServer, goodLDSRequest)
	sendGoodResp(t, "LDS", fakeServer, versionLDS, goodLDSResponse1, goodLDSRequest, cbLDS)
	versionLDS++

	// Start a new watch.
	cbLDS = startXDS(t, "LDS", v2c, fakeServer, goodLDSRequest)

	// This is an invalid response after the new watch.
	nonce := sendXDSRespWithVersion(fakeServer.ResponseChan, &xdspb.DiscoveryResponse{
		Resources: []*anypb.Any{{}},
		TypeUrl:   ldsURL,
	}, versionLDS)
	t.Logf("Bad response pushed to fakeServer...")

	// The expected version string is the previous acked version.
	if err := compareXDSRequest(fakeServer.RequestChan, defaultTestTimeout, goodLDSRequest, strconv.Itoa(versionLDS-1), nonce); err != nil {
		t.Errorf("Failed to receive request: %v", err)
	}
	t.Logf("Bad response nacked")
	versionLDS++

	sendGoodResp(t, "LDS", fakeServer, versionLDS, goodLDSResponse1, goodLDSRequest, cbLDS)
	versionLDS++
}
