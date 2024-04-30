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

//go:build !cluster_proxy

package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	_ "google.golang.org/grpc/health"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

const (
	// in sync with how kubernetes uses etcd
	// https://github.com/kubernetes/kubernetes/blob/release-1.28/staging/src/k8s.io/apiserver/pkg/storage/storagebackend/factory/etcd3.go#L59-L71
	keepaliveTime    = 30 * time.Second
	keepaliveTimeout = 10 * time.Second
	dialTimeout      = 20 * time.Second

	clientRuntime  = 10 * time.Second
	requestTimeout = 100 * time.Millisecond
)

func TestFailoverOnDefrag(t *testing.T) {
	tcs := []struct {
		name string

		experimentalStopGRPCServiceOnDefragEnabled bool
		gRPCDialOptions                            []grpc.DialOption

		// common assertion
		expectedMinTotalRequestsCount int
		// happy case assertion
		expectedMaxFailedRequestsCount int
		// negative case assertion
		expectedMinFailedRequestsCount int
	}{
		{
			name: "defrag failover happy case",
			experimentalStopGRPCServiceOnDefragEnabled: true,
			gRPCDialOptions: []grpc.DialOption{
				grpc.WithDisableServiceConfig(),
				grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy": "round_robin", "healthCheckConfig": {"serviceName": ""}}`),
			},
			expectedMinTotalRequestsCount:  300,
			expectedMaxFailedRequestsCount: 5,
		},
		{
			name: "defrag blocks one-third of requests with stopGRPCServiceOnDefrag set to false",
			experimentalStopGRPCServiceOnDefragEnabled: false,
			gRPCDialOptions: []grpc.DialOption{
				grpc.WithDisableServiceConfig(),
				grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy": "round_robin", "healthCheckConfig": {"serviceName": ""}}`),
			},
			expectedMinTotalRequestsCount:  300,
			expectedMinFailedRequestsCount: 90,
		},
		{
			name: "defrag blocks one-third of requests with stopGRPCServiceOnDefrag set to true and client health check disabled",
			experimentalStopGRPCServiceOnDefragEnabled: true,
			expectedMinTotalRequestsCount:              300,
			expectedMinFailedRequestsCount:             90,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			e2e.BeforeTest(t)
			cfg := e2e.EtcdProcessClusterConfig{
				ClusterSize:                         3,
				GoFailEnabled:                       true,
				ExperimentalStopGRPCServiceOnDefrag: tc.experimentalStopGRPCServiceOnDefragEnabled,
			}
			clus, err := e2e.NewEtcdProcessCluster(t, &cfg)
			require.NoError(t, err)
			t.Cleanup(func() { clus.Stop() })

			endpoints := clus.EndpointsGRPC()

			requestVolume, successfulRequestCount := 0, 0
			g := new(errgroup.Group)
			g.Go(func() (lastErr error) {
				clusterClient, cerr := clientv3.New(clientv3.Config{
					DialTimeout:          dialTimeout,
					DialKeepAliveTime:    keepaliveTime,
					DialKeepAliveTimeout: keepaliveTimeout,
					Endpoints:            endpoints,
					DialOptions:          tc.gRPCDialOptions,
				})
				if cerr != nil {
					return cerr
				}
				defer clusterClient.Close()

				timeout := time.After(clientRuntime)
				for {
					select {
					case <-timeout:
						return lastErr
					default:
					}
					getContext, cancel := context.WithTimeout(context.Background(), requestTimeout)
					_, err := clusterClient.Get(getContext, "health")
					cancel()
					requestVolume++
					if err != nil {
						lastErr = err
						continue
					}
					successfulRequestCount++
				}
			})

			triggerDefrag(t, clus.Procs[0])

			err = g.Wait()
			if err != nil {
				t.Logf("etcd client failed to fail over, error (%v)", err)
			}
			t.Logf("request failure rate is %.2f%%, traffic volume successfulRequestCount %d requests, total %d requests", (1-float64(successfulRequestCount)/float64(requestVolume))*100, successfulRequestCount, requestVolume)

			require.GreaterOrEqual(t, requestVolume, tc.expectedMinTotalRequestsCount)
			failedRequestCount := requestVolume - successfulRequestCount
			if tc.expectedMaxFailedRequestsCount != 0 {
				require.LessOrEqual(t, failedRequestCount, tc.expectedMaxFailedRequestsCount)
			}
			if tc.expectedMinFailedRequestsCount != 0 {
				require.GreaterOrEqual(t, failedRequestCount, tc.expectedMinFailedRequestsCount)
			}
		})
	}
}

func triggerDefrag(t *testing.T, member e2e.EtcdProcess) {
	require.NoError(t, member.Failpoints().SetupHTTP(context.Background(), "defragBeforeCopy", `sleep("10s")`))
	require.NoError(t, member.Etcdctl(e2e.ClientNonTLS, false, false).Defragment(time.Minute))
}
