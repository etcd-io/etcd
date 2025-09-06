package common

import (
	"go.etcd.io/etcd/client/pkg/v3/logutil"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

// NewClient creates an etcd client from the given configuration spec.
func NewClient(cfg *clientv3.ConfigSpec) (*clientv3.Client, error) {
	lg, _ := logutil.CreateDefaultZapLogger(zap.InfoLevel)
	cliCfg, err := clientv3.NewClientConfig(cfg, lg)
	if err != nil {
		return nil, err
	}
	return clientv3.New(*cliCfg)
}

// ConfigWithEndpoint returns a shallow copy of cfg with Endpoints set to the
// provided single endpoint.
func ConfigWithEndpoint(cfg *clientv3.ConfigSpec, ep string) *clientv3.ConfigSpec {
	c := *cfg
	c.Endpoints = []string{ep}
	return &c
}
