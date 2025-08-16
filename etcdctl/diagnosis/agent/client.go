package agent

import (
	"context"
	"time"

	"go.uber.org/zap"

	"go.etcd.io/etcd/client/pkg/v3/logutil"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func commandCtx(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}

func createClient(cfgSpec *clientv3.ConfigSpec) (*clientv3.Client, error) {
	lg, _ := logutil.CreateDefaultZapLogger(zap.InfoLevel)
	cfg, err := clientv3.NewClientConfig(cfgSpec, lg)
	if err != nil {
		return nil, err
	}
	return clientv3.New(*cfg)
}
