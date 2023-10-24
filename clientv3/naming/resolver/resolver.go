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

package resolver

import (
	"google.golang.org/grpc/resolver"

	"go.etcd.io/etcd/clientv3"
)

type builder struct {
	// ...
}

func (b builder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	// To be implemented...
	// Using endpoints.NewWatcher() to subscribe for endpoints changes.
	return nil, nil
}

func (b builder) Scheme() string {
	return "etcd"
}

func NewBuilder(client *clientv3.Client) (resolver.Builder, error) {
	return builder{}, nil
}
