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

package resolver

import (
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/resolver/manual"
	"google.golang.org/grpc/serviceconfig"

	"go.etcd.io/etcd/client/v3/internal/endpoint"
)

const (
	Schema = "etcd-endpoints"
)

// EtcdManualResolver is a Resolver (and resolver.Builder) that can be updated
// using SetEndpoints.
type EtcdManualResolver struct {
	*manual.Resolver
	endpoints     []string
	serviceConfig *serviceconfig.ParseResult
}

func New(endpoints ...string) *EtcdManualResolver {
	r := manual.NewBuilderWithScheme(Schema)
	return &EtcdManualResolver{Resolver: r, endpoints: endpoints, serviceConfig: nil}
}

// Build returns itself for Resolver, because it's both a builder and a resolver.
func (r *EtcdManualResolver) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	r.serviceConfig = cc.ParseServiceConfig(`{"loadBalancingPolicy": "round_robin"}`)
	if r.serviceConfig.Err != nil {
		return nil, r.serviceConfig.Err
	}
	// Seed the initial state before the underlying manual.Resolver is built so
	// that gRPC receives a single resolver update with both the endpoints and
	// the round_robin ServiceConfig. Otherwise Build would emit an empty first
	// update and updateState a second one, forcing gRPC to switch balancers
	// mid-connection and tear down an in-flight SubConn. See issue #21660.
	r.InitialState(r.buildState())
	return r.Resolver.Build(target, cc, opts)
}

func (r *EtcdManualResolver) SetEndpoints(endpoints []string) {
	r.endpoints = endpoints
	r.updateState()
}

func (r EtcdManualResolver) updateState() {
	if getCC(r) != nil {
		r.UpdateState(r.buildState())
	}
}

func (r EtcdManualResolver) buildState() resolver.State {
	eps := make([]resolver.Endpoint, len(r.endpoints))
	for i, ep := range r.endpoints {
		addr, serverName := endpoint.Interpret(ep)
		eps[i] = resolver.Endpoint{Addresses: []resolver.Address{
			{Addr: addr, ServerName: serverName},
		}}
	}
	return resolver.State{
		Endpoints:     eps,
		ServiceConfig: r.serviceConfig,
	}
}

func getCC(r EtcdManualResolver) (cc resolver.ClientConn) {
	defer func() {
		if rec := recover(); rec != nil {
			cc = nil
		}
	}()

	return r.CC()
}
