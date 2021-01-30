package resolver

import (
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/resolver"
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
