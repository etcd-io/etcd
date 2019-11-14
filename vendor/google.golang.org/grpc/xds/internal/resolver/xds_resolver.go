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

// Package resolver implements the xds resolver.
//
// At this point, the resolver is named xds-experimental, and doesn't do very
// much at all, except for returning a hard-coded service config which selects
// the xds_experimental balancer.
package resolver

import (
	"fmt"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/internal"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/serviceconfig"
)

const (
	// The JSON form of the hard-coded service config which picks the
	// xds_experimental balancer with round_robin as the child policy.
	jsonSC = `{
    "loadBalancingConfig":[
      {
        "xds_experimental":{
          "childPolicy":[
            {
              "round_robin": {}
            }
          ]
        }
      }
    ]
  }`
	// xDS balancer name is xds_experimental while resolver scheme is
	// xds-experimental since "_" is not a valid character in the URL.
	xdsScheme = "xds-experimental"
)

var (
	parseOnce sync.Once
	parsedSC  serviceconfig.Config
)

// NewBuilder creates a new implementation of the resolver.Builder interface
// for the xDS resolver.
func NewBuilder() resolver.Builder {
	return &xdsBuilder{}
}

type xdsBuilder struct{}

// Build helps implement the resolver.Builder interface.
func (b *xdsBuilder) Build(t resolver.Target, cc resolver.ClientConn, o resolver.BuildOption) (resolver.Resolver, error) {
	parseOnce.Do(func() {
		// The xds balancer must have been registered at this point for the service
		// config to be parsed properly.
		psc, err := internal.ParseServiceConfig(jsonSC)
		if err != nil {
			panic(fmt.Sprintf("service config %s parsing failed: %v", jsonSC, err))
		}

		var ok bool
		if parsedSC, ok = psc.(*grpc.ServiceConfig); !ok {
			panic(fmt.Sprintf("service config type is [%T], want [grpc.ServiceConfig]", psc))
		}
	})

	// We return a resolver which bacically does nothing. The hard-coded service
	// config returned here picks the xds balancer.
	cc.UpdateState(resolver.State{ServiceConfig: parsedSC})
	return &xdsResolver{}, nil
}

// Name helps implement the resolver.Builder interface.
func (*xdsBuilder) Scheme() string {
	return xdsScheme
}

type xdsResolver struct{}

// ResolveNow is a no-op at this point.
func (*xdsResolver) ResolveNow(o resolver.ResolveNowOption) {}

// Close is a no-op at this point.
func (*xdsResolver) Close() {}
