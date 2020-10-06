// Copyright 2020 The etcd Authors
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
	"log"
	"net/http"
	"sync"
	"time"

	"go.etcd.io/etcd/v3/pkg/transport"
)

type LazyCluster interface {
	// EndpointsV2 - call to this method might initialize the cluster.
	EndpointsV2() []string

	// EndpointsV2 - call to this method might initialize the cluster.
	EndpointsV3() []string

	// Transport - call to this method might initialize the cluster.
	Transport() *http.Transport

	Terminate()
}

type lazyCluster struct {
	cfg       ClusterConfig
	cluster   *ClusterV3
	transport *http.Transport
	once      sync.Once
}

// NewLazyCluster returns a new test cluster handler that gets created on the
// first call to GetEndpoints() or GetTransport()
func NewLazyCluster() LazyCluster {
	return &lazyCluster{cfg: ClusterConfig{Size: 1}}
}

func (lc *lazyCluster) mustLazyInit() {
	lc.once.Do(func() {
		var err error
		lc.transport, err = transport.NewTransport(transport.TLSInfo{}, time.Second)
		if err != nil {
			log.Fatal(err)
		}
		lc.cluster = NewClusterV3(nil, &lc.cfg)
	})
}

func (lc *lazyCluster) Terminate() {
	if lc != nil {
		lc.cluster.Terminate(nil)
	}
}

func (lc *lazyCluster) EndpointsV2() []string {
	lc.mustLazyInit()
	return []string{lc.cluster.Members[0].URL()}
}

func (lc *lazyCluster) EndpointsV3() []string {
	lc.mustLazyInit()
	return []string{lc.cluster.Client(0).Endpoints()[0]}
}

func (lc *lazyCluster) Transport() *http.Transport {
	lc.mustLazyInit()
	return lc.transport
}
