// Copyright 2018 The etcd Authors
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

// resolves to etcd entpoints for grpc targets of the form 'etcd://<cluster-name>/<endpoint>'.
package endpoint

import (
	"fmt"
	"sync"

	"google.golang.org/grpc/resolver"
)

const (
	scheme = "etcd"
)

var (
	bldr *builder
)

func init() {
	bldr = &builder{
		clusterResolvers: make(map[string]*Resolver),
	}
	resolver.Register(bldr)
}

type builder struct {
	clusterResolvers map[string]*Resolver
	sync.RWMutex
}

// Build creates or reuses an etcd resolver for the etcd cluster name identified by the authority part of the target.
func (b *builder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOption) (resolver.Resolver, error) {
	if len(target.Authority) < 1 {
		return nil, fmt.Errorf("'etcd' target scheme requires non-empty authority identifying etcd cluster being routed to")
	}
	r := b.getResolver(target.Authority)
	r.cc = cc
	if r.bootstrapAddrs != nil {
		r.NewAddress(r.bootstrapAddrs)
	}
	return r, nil
}

func (b *builder) getResolver(clusterName string) *Resolver {
	b.RLock()
	r, ok := b.clusterResolvers[clusterName]
	b.RUnlock()
	if !ok {
		r = &Resolver{
			clusterName: clusterName,
		}
		b.Lock()
		b.clusterResolvers[clusterName] = r
		b.Unlock()
	}
	return r
}

func (b *builder) addResolver(r *Resolver) {
	bldr.Lock()
	bldr.clusterResolvers[r.clusterName] = r
	bldr.Unlock()
}

func (b *builder) removeResolver(r *Resolver) {
	bldr.Lock()
	delete(bldr.clusterResolvers, r.clusterName)
	bldr.Unlock()
}

func (r *builder) Scheme() string {
	return scheme
}

// EndpointResolver gets the resolver for  given etcd cluster name.
func EndpointResolver(clusterName string) *Resolver {
	return bldr.getResolver(clusterName)
}

// Resolver provides a resolver for a single etcd cluster, identified by name.
type Resolver struct {
	clusterName    string
	cc             resolver.ClientConn
	bootstrapAddrs []resolver.Address
}

// InitialAddrs sets the initial endpoint addresses for the resolver.
func (r *Resolver) InitialAddrs(addrs []resolver.Address) {
	r.bootstrapAddrs = addrs
}

func (r *Resolver) InitialEndpoints(eps []string) {
	r.InitialAddrs(epsToAddrs(eps...))
}

// TODO: use balancer.epsToAddrs
func epsToAddrs(eps ...string) (addrs []resolver.Address) {
	addrs = make([]resolver.Address, 0, len(eps))
	for _, ep := range eps {
		addrs = append(addrs, resolver.Address{Addr: ep})
	}
	return addrs
}

// NewAddress updates the addresses of the resolver.
func (r *Resolver) NewAddress(addrs []resolver.Address) error {
	if r.cc == nil {
		return fmt.Errorf("resolver not yet built, use InitialAddrs to provide initialization endpoints")
	}
	r.cc.NewAddress(addrs)
	return nil
}

func (*Resolver) ResolveNow(o resolver.ResolveNowOption) {}

func (r *Resolver) Close() {
	bldr.removeResolver(r)
}
