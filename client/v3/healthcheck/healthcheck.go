// Copyright 2026 The etcd Authors
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

package healthcheck

import (
	"google.golang.org/grpc"
	// Register the gRPC client's health check implementation via its init().
	// It sets internal.HealthCheckFunc = clientHealthCheck, which is required
	// for the client-side healthCheckConfig (see GRPCClientHealthCheckOptions)
	// to perform health checks against the server.
	_ "google.golang.org/grpc/health"
)

func GRPCClientHealthCheckOptions() []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithDisableServiceConfig(),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy": "round_robin", "healthCheckConfig": {"serviceName": ""}}`),
	}
}
