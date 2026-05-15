package healthcheck

import (
	"google.golang.org/grpc"
	_ "google.golang.org/grpc/health"
)

func GRPCClientHealthCheckOptions() []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithDisableServiceConfig(),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy": "round_robin", "healthCheckConfig": {"serviceName": ""}}`),
	}
}
