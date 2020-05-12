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

package fakexds

import (
	"io"
	"sync"

	corepb "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	endpointpb "github.com/envoyproxy/go-control-plane/envoy/api/v2/endpoint"
	lrsgrpc "github.com/envoyproxy/go-control-plane/envoy/service/load_stats/v2"
	lrspb "github.com/envoyproxy/go-control-plane/envoy/service/load_stats/v2"
	"github.com/golang/protobuf/proto"
	durationpb "github.com/golang/protobuf/ptypes/duration"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// LRSServer implements the LRS service, and is to be installed on the fakexds
// server. It collects load reports, and returned them later for comparison.
type LRSServer struct {
	// ReportingInterval will be sent in the first response to control reporting
	// interval.
	ReportingInterval *durationpb.Duration
	// ExpectedEDSClusterName is checked against the first LRS request. The RPC
	// is failed if they don't match.
	ExpectedEDSClusterName string

	mu        sync.Mutex
	dropTotal uint64
	drops     map[string]uint64
}

func newLRSServer() *LRSServer {
	return &LRSServer{
		drops: make(map[string]uint64),
		ReportingInterval: &durationpb.Duration{
			Seconds: 60 * 60, // 1 hour, each test can override this to a shorter duration.
		},
	}
}

// StreamLoadStats implements LRS service.
func (lrss *LRSServer) StreamLoadStats(stream lrsgrpc.LoadReportingService_StreamLoadStatsServer) error {
	req, err := stream.Recv()
	if err != nil {
		return err
	}
	wantReq := &lrspb.LoadStatsRequest{
		ClusterStats: []*endpointpb.ClusterStats{{
			ClusterName: lrss.ExpectedEDSClusterName,
		}},
		Node: &corepb.Node{},
	}
	if !proto.Equal(req, wantReq) {
		return status.Errorf(codes.FailedPrecondition, "unexpected req: %+v, want %+v, diff: %s", req, wantReq, cmp.Diff(req, wantReq, cmp.Comparer(proto.Equal)))
	}
	if err := stream.Send(&lrspb.LoadStatsResponse{
		Clusters:              []string{lrss.ExpectedEDSClusterName},
		LoadReportingInterval: lrss.ReportingInterval,
	}); err != nil {
		return err
	}

	for {
		req, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		stats := req.ClusterStats[0]
		lrss.mu.Lock()
		lrss.dropTotal += stats.TotalDroppedRequests
		for _, d := range stats.DroppedRequests {
			lrss.drops[d.Category] += d.DroppedCount
		}
		lrss.mu.Unlock()
	}
}

// GetDrops returns the drops reported to this server.
func (lrss *LRSServer) GetDrops() map[string]uint64 {
	lrss.mu.Lock()
	defer lrss.mu.Unlock()
	return lrss.drops
}
