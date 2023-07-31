package kubernetes

import (
	"context"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type Interface interface {
	Get(ctx context.Context, key string, opts GetOptions) (GetResponse, error)
	List(ctx context.Context, prefix string, opts ListOptions) (ListResponse, error)
	Count(ctx context.Context, prefix string) (int64, error)
	OptimisticPut(ctx context.Context, key string, value []byte, opts PutOptions) (PutResponse, error)
	OptimisticDelete(ctx context.Context, key string, opts DeleteOptions) (DeleteResponse, error)
}

type GetOptions struct {
	Revision int64
}

type ListOptions struct {
	Revision int64
	Limit    int64
	Continue string
}

type PutOptions struct {
	ExpectedRevision int64
	GetOnFailure     bool
	// LeaseID
	// Deprecated: Should be replaced with TTL when Interface starts using one lease per object.
	LeaseID clientv3.LeaseID
}

type DeleteOptions struct {
	ExpectedRevision int64
	GetOnFailure     bool
}

type GetResponse struct {
	KV       *mvccpb.KeyValue
	Revision int64
}

type ListResponse struct {
	KVs      []*mvccpb.KeyValue
	Count    int64
	Revision int64
}

type PutResponse struct {
	KV        *mvccpb.KeyValue
	Succeeded bool
	Revision  int64
}

type DeleteResponse struct {
	KV        *mvccpb.KeyValue
	Succeeded bool
	Revision  int64
}
