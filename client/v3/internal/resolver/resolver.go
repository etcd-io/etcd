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
	"fmt"

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
	endpoints     []string // "127.0.0.1:2379"
	serviceConfig *serviceconfig.ParseResult
}

func New(endpoints ...string) *EtcdManualResolver {
	r := manual.NewBuilderWithScheme(Schema)
	return &EtcdManualResolver{Resolver: r, endpoints: endpoints, serviceConfig: nil}
}

// Build returns itself for Resolver, because it's both a builder and a resolver.
func (r *EtcdManualResolver) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	// 解析获取到服务配置
	r.serviceConfig = cc.ParseServiceConfig(`{"loadBalancingPolicy": "round_robin"}`)
	if r.serviceConfig.Err != nil {
		return nil, r.serviceConfig.Err
	}
	// 构造解析器，这块实际返回的还是 EtcdManualResolver
	//
	res, err := r.Resolver.Build(target, cc, opts)
	if err != nil {
		return nil, err
	}
	// Populates endpoints stored in r into ClientConn (cc).
	// 更新grpc的状态和配置
	r.updateState()
	return res, nil
}

func (r *EtcdManualResolver) SetEndpoints(endpoints []string) {
	r.endpoints = endpoints
	r.updateState()
}

func (r EtcdManualResolver) updateState() {
	if r.CC != nil {
		addresses := make([]resolver.Address, len(r.endpoints))
		for i, ep := range r.endpoints {
			addr, serverName := endpoint.Interpret(ep)
			fmt.Printf("addr=%s, serverName=%s \n", addr, serverName)
			addresses[i] = resolver.Address{Addr: addr, ServerName: serverName}
		}

		fmt.Printf("r.serviceConfig=%+v \n", r.serviceConfig)

		// 对于一个Resolver，需要将解析出的地址，传入resolver.State中
		// 状态里包含地址和服务配置
		state := resolver.State{
			Addresses:     addresses,
			ServiceConfig: r.serviceConfig,
		}
		// 去更新grpc的状态
		r.UpdateState(state)
	}
}
