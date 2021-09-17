// Copyright 2021 The etcd Authors
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

package grpc_testing

import (
	"context"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type GrpcRecorder struct {
	mux      sync.RWMutex
	requests []RequestInfo
}

type RequestInfo struct {
	FullMethod string
	Authority  string
}

func (ri *GrpcRecorder) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		ri.record(toRequestInfo(ctx, info))
		resp, err := handler(ctx, req)
		return resp, err
	}
}

func (ri *GrpcRecorder) RecordedRequests() []RequestInfo {
	ri.mux.RLock()
	defer ri.mux.RUnlock()
	reqs := make([]RequestInfo, len(ri.requests))
	copy(reqs, ri.requests)
	return reqs
}

func toRequestInfo(ctx context.Context, info *grpc.UnaryServerInfo) RequestInfo {
	req := RequestInfo{
		FullMethod: info.FullMethod,
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		as := md.Get(":authority")
		if len(as) != 0 {
			req.Authority = as[0]
		}
	}
	return req
}

func (ri *GrpcRecorder) record(r RequestInfo) {
	ri.mux.Lock()
	defer ri.mux.Unlock()
	ri.requests = append(ri.requests, r)
}
