// Copyright 2022 The etcd Authors
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

package integration

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
	integration2 "go.etcd.io/etcd/tests/v3/framework/integration"
	clientv3test "go.etcd.io/etcd/tests/v3/integration/clientv3"
)

func TestFailover(t *testing.T) {
	cases := []struct {
		name     string
		testFunc func(*testing.T, *tls.Config, *integration2.Cluster) (*clientv3.Client, error)
	}{
		{
			name:     "create client before the first server down",
			testFunc: createClientBeforeServerDown,
		},
		{
			name:     "create client after the first server down",
			testFunc: createClientAfterServerDown,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Starting test [%s]", tc.name)
			integration2.BeforeTest(t)

			// Launch an etcd cluster with 3 members
			t.Logf("Launching an etcd cluster with 3 members [%s]", tc.name)
			clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 3, ClientTLS: &integration2.TestTLSInfo})
			defer clus.Terminate(t)

			cc, err := integration2.TestTLSInfo.ClientConfig()
			require.NoError(t, err)
			// Create an etcd client before or after first server down
			t.Logf("Creating an etcd client [%s]", tc.name)
			cli, err := tc.testFunc(t, cc, clus)
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}
			defer cli.Close()

			// Sanity test
			t.Logf("Running sanity test [%s]", tc.name)
			key, val := "key1", "val1"
			putWithRetries(t, cli, key, val, 10)
			getWithRetries(t, cli, key, val, 10)

			t.Logf("Test done [%s]", tc.name)
		})
	}
}

func createClientBeforeServerDown(t *testing.T, cc *tls.Config, clus *integration2.Cluster) (*clientv3.Client, error) {
	cli, err := createClient(t, cc, clus)
	if err != nil {
		return nil, err
	}
	clus.Members[0].Close()
	return cli, nil
}

func createClientAfterServerDown(t *testing.T, cc *tls.Config, clus *integration2.Cluster) (*clientv3.Client, error) {
	clus.Members[0].Close()
	return createClient(t, cc, clus)
}

func createClient(t *testing.T, cc *tls.Config, clus *integration2.Cluster) (*clientv3.Client, error) {
	cli, err := integration2.NewClient(t, clientv3.Config{
		Endpoints:   clus.Endpoints(),
		DialTimeout: 5 * time.Second,
		DialOptions: []grpc.DialOption{grpc.WithBlock()},
		TLS:         cc,
	})
	if err != nil {
		return nil, err
	}

	return cli, nil
}

func putWithRetries(t *testing.T, cli *clientv3.Client, key, val string, retryCount int) {
	for retryCount > 0 {
		// put data test
		err := func() error {
			t.Log("Sanity test, putting data")
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			if _, putErr := cli.Put(ctx, key, val); putErr != nil {
				t.Logf("Failed to put data (%v)", putErr)
				return putErr
			}
			return nil
		}()
		if err != nil {
			retryCount--
			if shouldRetry(err) {
				continue
			}
			t.Fatal(err)
		}
		break
	}
}

func getWithRetries(t *testing.T, cli *clientv3.Client, key, val string, retryCount int) {
	for retryCount > 0 {
		// get data test
		err := func() error {
			t.Log("Sanity test, getting data")
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			resp, getErr := cli.Get(ctx, key)
			if getErr != nil {
				t.Logf("Failed to get key (%v)", getErr)
				return getErr
			}
			if len(resp.Kvs) != 1 {
				t.Fatalf("Expected 1 key, got %d", len(resp.Kvs))
			}
			if !bytes.Equal([]byte(val), resp.Kvs[0].Value) {
				t.Fatalf("Unexpected value, expected: %s, got: %s", val, resp.Kvs[0].Value)
			}
			return nil
		}()
		if err != nil {
			retryCount--
			if shouldRetry(err) {
				continue
			}
			t.Fatal(err)
		}
		break
	}
}

func shouldRetry(err error) bool {
	if clientv3test.IsClientTimeout(err) || clientv3test.IsServerCtxTimeout(err) ||
		errors.Is(err, rpctypes.ErrTimeout) || errors.Is(err, rpctypes.ErrTimeoutDueToLeaderFail) {
		return true
	}
	return false
}
