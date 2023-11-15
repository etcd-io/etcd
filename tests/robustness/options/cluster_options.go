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

package options

import (
	"math/rand"
	"time"

	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

var internalRand = rand.New(rand.NewSource(time.Now().UnixNano()))

type ClusterOptions []e2e.EPClusterOption

// WithClusterOptionGroups takes an array of EPClusterOption arrays, and randomly picks one EPClusterOption array when constructing the config.
// This function is mainly used to group strongly coupled config options together, so that we can dynamically test different groups of options.
func WithClusterOptionGroups(input ...ClusterOptions) e2e.EPClusterOption {
	return func(c *e2e.EtcdProcessClusterConfig) {
		optsPicked := input[internalRand.Intn(len(input))]
		for _, opt := range optsPicked {
			opt(c)
		}
	}
}

// WithSubsetOptions randomly select a subset of input options, and apply the subset to the cluster config.
func WithSubsetOptions(input ...e2e.EPClusterOption) e2e.EPClusterOption {
	return func(c *e2e.EtcdProcessClusterConfig) {
		// selects random subsetLen (0 to len(input)) elements from the input array.
		subsetLen := internalRand.Intn(len(input) + 1)
		perm := internalRand.Perm(len(input))
		for i := 0; i < subsetLen; i++ {
			opt := input[perm[i]]
			opt(c)
		}
	}
}
