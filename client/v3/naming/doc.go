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

// Package naming provides:
//	- subpackage endpoints: an abstraction layer to store and read endpoints
//		information from etcd.
//	- subpackage resolver: an etcd-backed gRPC resolver for discovering gRPC
//		services based on the endpoints configuration
//
// To use, first import the packages:
//
//	import (
//		"go.etcd.io/etcd/client/v3"
//		"go.etcd.io/etcd/client/v3/naming/endpoints"
//		"go.etcd.io/etcd/client/v3/naming/resolver"
//		"google.golang.org/grpc"
//	)
//
// First, register new endpoint addresses for a service:
//
//	func etcdAdd(c *clientv3.Client, service, addr string) error {
//		em := endpoints.NewManager(c, service)
//		return em.AddEndpoint(c.Ctx(), service+"/"+addr, endpoints.Endpoint{Addr:addr});
//	}
//
// Dial an RPC service using the etcd gRPC resolver and a gRPC Balancer:
//
//	func etcdDial(c *clientv3.Client, service string) (*grpc.ClientConn, error) {
//		etcdResolver, err := resolver.NewBuilder(c);
//		if err { return nil, err }
//		return  grpc.Dial("etcd:///" + service, grpc.WithResolvers(etcdResolver))
//	}
//
// Optionally, force delete an endpoint:
//
//	func etcdDelete(c *clientv3, service, addr string) error {
//		em := endpoints.NewManager(c, service)
//		return em.DeleteEndpoint(c.Ctx(), service+"/"+addr)
//	}
//
// Or register an expiring endpoint with a lease:
//
//	func etcdAdd(c *clientv3.Client, lid clientv3.LeaseID, service, addr string) error {
//		em := endpoints.NewManager(c, service)
//		return em.AddEndpoint(c.Ctx(), service+"/"+addr, endpoints.Endpoint{Addr:addr}, clientv3.WithLease(lid));
//	}
//
package naming
