// Copyright 2017 The etcd Authors
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

package clientv3test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.etcd.io/etcd/client/pkg/v3/transport"
	clientv3 "go.etcd.io/etcd/client/v3"
	integration2 "go.etcd.io/etcd/tests/v3/framework/integration"
)

// MustWaitPinReady waits up to 3-second until connection is up (pin endpoint).
// Fatal on time-out.
func MustWaitPinReady(t *testing.T, cli *clientv3.Client) {
	// TODO: decrease timeout after balancer rewrite!!!
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	_, err := cli.Get(ctx, "foo")
	cancel()
	if err != nil {
		t.Fatal(err)
	}
}

// IsServerCtxTimeout checks reason of the error.
// e.g. due to clock drifts in server-side,
// client context times out first in server-side
// while original client-side context is not timed out yet
func IsServerCtxTimeout(err error) bool {
	if err == nil {
		return false
	}
	ev, ok := status.FromError(err)
	if !ok {
		return false
	}
	code := ev.Code()
	return (code == codes.DeadlineExceeded /*3.5+"*/ || code == codes.Unknown /*<=3.4*/) &&
		strings.Contains(err.Error(), "context deadline exceeded")
}

// IsClientTimeout checks reason of the error.
// In grpc v1.11.3+ dial timeouts can error out with transport.ErrConnClosing. Previously dial timeouts
// would always error out with context.DeadlineExceeded.
func IsClientTimeout(err error) bool {
	if err == nil {
		return false
	}
	if err == context.DeadlineExceeded {
		return true
	}
	ev, ok := status.FromError(err)
	if !ok {
		return false
	}
	code := ev.Code()
	return code == codes.DeadlineExceeded
}

func IsCanceled(err error) bool {
	if err == nil {
		return false
	}
	if err == context.Canceled {
		return true
	}
	ev, ok := status.FromError(err)
	if !ok {
		return false
	}
	code := ev.Code()
	return code == codes.Canceled
}

func IsUnavailable(err error) bool {
	if err == nil {
		return false
	}
	if err == context.Canceled {
		return true
	}
	ev, ok := status.FromError(err)
	if !ok {
		return false
	}
	code := ev.Code()
	return code == codes.Unavailable
}

// populateDataIntoCluster populates the key-value pairs into cluster and the
// key will be named by testing.T.Name()-index.
func populateDataIntoCluster(t *testing.T, cluster *integration2.Cluster, numKeys int, valueSize int) {
	ctx := context.Background()

	for i := 0; i < numKeys; i++ {
		_, err := cluster.RandClient().Put(ctx,
			fmt.Sprintf("%s-%v", t.Name(), i), strings.Repeat("a", valueSize))

		if err != nil {
			t.Errorf("populating data expected no error, but got %v", err)
		}
	}
}

// guaranteeRaftAppliedIndexAdvance ensures raftLog.applied is the same as appliedIndex of etcdserver.EtcdServer
// it should only be used between each raft proposal.
func guaranteeRaftAppliedIndexAdvance(t *testing.T, gRPCURL string, clientURL url.URL) {
	t.Logf("ensuring applied index advance, (gRPCURL %s), (clientURL %v)", gRPCURL, clientURL)
	cc, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{gRPCURL},
		DialTimeout: 5 * time.Second,
	})
	require.NoError(t, err)
	defer cc.Close()
	cresp, err := cc.Status(context.TODO(), gRPCURL)
	require.NoError(t, err)
	appliedIndex := cresp.RaftAppliedIndex

	require.Eventually(t, func() bool {
		raftStatus := getRaftStatus(t, clientURL)
		return raftStatus.Applied == appliedIndex
	}, 5*time.Second, 20*time.Millisecond)
}

func getRaftStatus(t *testing.T, clientURL url.URL) raftStatus {
	tr, err := transport.NewTransport(transport.TLSInfo{}, 5*time.Second)
	require.NoError(t, err)
	clientURL.Path = "/debug/vars"
	response, err := tr.RoundTrip(&http.Request{
		Header: make(http.Header),
		Method: http.MethodGet,
		URL:    &clientURL,
	})

	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &resp))
	var raftStatus raftStatus

	for key, value := range resp {
		if key == "raft.status" {
			rs, err := json.Marshal(value)
			require.NoError(t, err)
			t.Logf("marshalled raft status is %s", string(rs))
			require.NoError(t, json.Unmarshal(rs, &raftStatus))
			break
		}
	}
	return raftStatus
}

// raftStatus should be in sync with raft.Status
type raftStatus struct {
	ID string

	Term   uint64
	Vote   string
	Commit uint64

	Lead      string
	RaftState string

	Applied        uint64
	LeadTransferee string
	Progress       map[string]interface{}
}
