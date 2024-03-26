package clientv3

import (
	"context"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/mvccpb"
)

func NewKubernetes(c *Client) Kubernetes {
	return &kubernetes{kv: RetryKVClient(c), watcher: c}
}

type Kubernetes interface {
	Get(ctx context.Context, key string, revision int64) (kv *mvccpb.KeyValue, rev int64, err error)
	List(ctx context.Context, prefix string, revision, limit int64, fromKey string) (kvs []*mvccpb.KeyValue, count, rev int64, err error)
	Count(ctx context.Context, prefix string) (int64, error)
	OptimisticCreate(ctx context.Context, key string, value []byte) (success bool, rev int64, err error)
	OptimisticUpdate(ctx context.Context, key string, value []byte, expectedRevision int64) (kv *mvccpb.KeyValue, success bool, rev int64, err error)
	OptimisticDelete(ctx context.Context, key string, expectedRevision int64) (kv *mvccpb.KeyValue, success bool, rev int64, err error)

	Watch(ctx context.Context, key string, rev int64, prefix bool) WatchChan
	RequestProgress(ctx context.Context) error

	// Deprecated: Using leases breaks the implicit Kubernetes-etcd contract.
	// Lease invalidation deletes multiple keys in one transaction breaking Kubernetes watch assumptions about resourceVersion uniqness.
	OptimisticCreateWithLease(ctx context.Context, key string, value []byte, lease LeaseID) (success bool, rev int64, err error)
	// Deprecated: Using leases breaks the implicit Kubernetes-etcd contract.
	// Lease invalidation deletes multiple keys in one transaction breaking Kubernetes watch assumptions about resourceVersion uniqness.
	OptimisticUpdateWithLease(ctx context.Context, key string, value []byte, expectedRevision int64, lease LeaseID) (kv *mvccpb.KeyValue, success bool, rev int64, err error)
}

type kubernetes struct {
	kv      pb.KVClient
	watcher Watcher
}

func (k kubernetes) Get(ctx context.Context, key string, revision int64) (kv *mvccpb.KeyValue, rev int64, err error) {
	resp, err := k.kv.Range(ctx, &pb.RangeRequest{
		Key:      []byte(key),
		Revision: revision,
		Limit:    1,
	})
	if err != nil {
		return nil, 0, toErr(ctx, err)
	}
	if len(resp.Kvs) == 1 {
		kv = resp.Kvs[0]
	}
	return kv, resp.Header.Revision, nil
}

func (k kubernetes) List(ctx context.Context, prefix string, revision, limit int64, fromKey string) (kvs []*mvccpb.KeyValue, count, rev int64, err error) {
	rangeStart := prefix
	if fromKey != "" {
		rangeStart = fromKey
	}
	rangeEnd := GetPrefixRangeEnd(prefix)
	resp, err := k.kv.Range(ctx, &pb.RangeRequest{
		Key:      []byte(rangeStart),
		RangeEnd: []byte(rangeEnd),
		Limit:    limit,
		Revision: revision,
	})
	if err != nil {
		return nil, 0, 0, toErr(ctx, err)
	}
	return resp.Kvs, resp.Count, resp.Header.Revision, nil
}

func (k kubernetes) Count(ctx context.Context, prefix string) (count int64, err error) {
	resp, err := k.kv.Range(ctx, &pb.RangeRequest{
		Key:       []byte(prefix),
		RangeEnd:  []byte(GetPrefixRangeEnd(prefix)),
		CountOnly: true,
	})
	if err != nil {
		return 0, toErr(ctx, err)
	}
	return resp.Count, nil
}

func (k kubernetes) OptimisticCreateWithLease(ctx context.Context, key string, value []byte, lease LeaseID) (success bool, rev int64, err error) {
	put := &pb.RequestOp{Request: &pb.RequestOp_RequestPut{&pb.PutRequest{Key: []byte(key), Value: value, Lease: int64(lease)}}}

	resp, err := k.optimisticOp(ctx, key, 0, put, nil)
	if err != nil {
		return false, 0, toErr(ctx, err)
	}
	return resp.Succeeded, resp.Header.Revision, nil
}

func (k kubernetes) OptimisticCreate(ctx context.Context, key string, value []byte) (success bool, rev int64, err error) {
	put := &pb.RequestOp{Request: &pb.RequestOp_RequestPut{&pb.PutRequest{Key: []byte(key), Value: value}}}

	resp, err := k.optimisticOp(ctx, key, 0, put, nil)
	if err != nil {
		return false, 0, toErr(ctx, err)
	}
	return resp.Succeeded, resp.Header.Revision, nil
}

func (k kubernetes) OptimisticDelete(ctx context.Context, key string, expectedRevision int64) (kv *mvccpb.KeyValue, success bool, rev int64, err error) {
	del := &pb.RequestOp{Request: &pb.RequestOp_RequestDeleteRange{&pb.DeleteRangeRequest{Key: []byte(key)}}}
	get := &pb.RequestOp{Request: &pb.RequestOp_RequestRange{&pb.RangeRequest{Key: []byte(key), Limit: 1}}}

	resp, err := k.optimisticOp(ctx, key, expectedRevision, del, get)
	if err != nil {
		return nil, false, 0, toErr(ctx, err)
	}
	if !resp.Succeeded {
		kv = kvFromTxnResponse(resp.Responses[0])
	}
	return kv, resp.Succeeded, resp.Header.Revision, nil
}

func (k kubernetes) OptimisticUpdateWithLease(ctx context.Context, key string, value []byte, expectedRevision int64, lease LeaseID) (kv *mvccpb.KeyValue, success bool, rev int64, err error) {
	put := &pb.RequestOp{Request: &pb.RequestOp_RequestPut{&pb.PutRequest{Key: []byte(key), Value: value, Lease: int64(lease)}}}
	get := &pb.RequestOp{Request: &pb.RequestOp_RequestRange{&pb.RangeRequest{Key: []byte(key), Limit: 1}}}

	resp, err := k.optimisticOp(ctx, key, expectedRevision, put, get)
	if err != nil {
		return nil, false, 0, toErr(ctx, err)
	}
	if !resp.Succeeded {
		kv = kvFromTxnResponse(resp.Responses[0])
	}
	return kv, resp.Succeeded, resp.Header.Revision, nil
}

func (k kubernetes) OptimisticUpdate(ctx context.Context, key string, value []byte, expectedRevision int64) (kv *mvccpb.KeyValue, success bool, rev int64, err error) {
	put := &pb.RequestOp{Request: &pb.RequestOp_RequestPut{&pb.PutRequest{Key: []byte(key), Value: value}}}
	get := &pb.RequestOp{Request: &pb.RequestOp_RequestRange{&pb.RangeRequest{Key: []byte(key), Limit: 1}}}

	resp, err := k.optimisticOp(ctx, key, expectedRevision, put, get)
	if err != nil {
		return nil, false, 0, toErr(ctx, err)
	}
	if !resp.Succeeded {
		kv = kvFromTxnResponse(resp.Responses[0])
	}
	return kv, resp.Succeeded, resp.Header.Revision, nil
}

func (k kubernetes) optimisticOp(ctx context.Context, key string, expectRevision int64, onSuccess, onFailure *pb.RequestOp) (*pb.TxnResponse, error) {
	cmp := pb.Compare(Compare(ModRevision(key), "=", expectRevision))
	txn := &pb.TxnRequest{
		Compare: []*pb.Compare{&cmp},
	}
	if onSuccess != nil {
		txn.Success = []*pb.RequestOp{onSuccess}
	}
	if onFailure != nil {
		txn.Failure = []*pb.RequestOp{onFailure}
	}
	return k.kv.Txn(ctx, txn)
}

func kvFromTxnResponse(resp *pb.ResponseOp) *mvccpb.KeyValue {
	getResponse := resp.GetResponseRange()
	if len(getResponse.Kvs) == 1 {
		return getResponse.Kvs[0]
	}
	return nil
}

func (k kubernetes) Watch(ctx context.Context, key string, rev int64, prefix bool) WatchChan {
	if prefix {
		return k.watcher.Watch(ctx, key, WithPrefix(), WithRev(rev), WithPrevKV(), WithProgressNotify())
	}
	return k.watcher.Watch(ctx, key, WithRev(rev), WithPrevKV(), WithProgressNotify())
}

func (k kubernetes) RequestProgress(ctx context.Context) error {
	return k.watcher.RequestProgress(ctx)
}
