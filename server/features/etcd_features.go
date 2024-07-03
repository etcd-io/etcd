// Copyright 2024 The etcd Authors
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

package features

import (
	"fmt"

	"go.uber.org/zap"

	"go.etcd.io/etcd/pkg/v3/featuregate"
)

const (
	// Every feature gate should add method here following this template:
	//
	// // owner: @username
	// // kep: https://kep.k8s.io/NNN (or issue: https://github.com/etcd-io/etcd/issues/NNN)
	// // alpha: v3.X
	// MyFeature featuregate.Feature = "MyFeature"
	//
	// Feature gates should be listed in alphabetical, case-sensitive
	// (upper before any lower case character) order. This reduces the risk
	// of code conflicts because changes are more likely to be scattered
	// across the file.

	// DistributedTracing enables experimental distributed  tracing using OpenTelemetry Tracing.
	// alpha: v3.5
	// issue: https://github.com/etcd-io/etcd/issues/12460
	DistributedTracing featuregate.Feature = "DistributedTracing"
	// StopGRPCServiceOnDefrag enables etcd gRPC service to stop serving client requests on defragmentation.
	// owner: @chaochn47
	// alpha: v3.6
	StopGRPCServiceOnDefrag featuregate.Feature = "StopGRPCServiceOnDefrag"
)

var (
	DefaultEtcdServerFeatureGates = map[featuregate.Feature]featuregate.FeatureSpec{
		DistributedTracing:      {Default: false, PreRelease: featuregate.Alpha},
		StopGRPCServiceOnDefrag: {Default: false, PreRelease: featuregate.Alpha},
	}
	ExperimentalFlagToFeatureMap = map[string]featuregate.Feature{
		"experimental-enable-distributed-tracing":  DistributedTracing,
		"experimental-stop-grpc-service-on-defrag": StopGRPCServiceOnDefrag,
	}
)

func NewDefaultServerFeatureGate(name string, lg *zap.Logger) featuregate.FeatureGate {
	fg := featuregate.New(fmt.Sprintf("%sServerFeatureGate", name), lg)
	if err := fg.Add(DefaultEtcdServerFeatureGates); err != nil {
		panic(err)
	}
	return fg
}
