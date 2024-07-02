package kubernetes

import (
	"context"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// New creates KubernetesClient from config.
// Caller is responsible to call Close() to clean up client.
func New(cfg clientv3.Config) (*KubernetesClient, error) {
	c, err := clientv3.New(cfg)
	if err != nil {
		return nil, err
	}
	return fromClient(c, c.Close), nil
}

// FromClient creates KubernetesClient from existing client.
// Caller can still call Close() however, it's not needed as long caller should call Close() on the original client.
func FromClient(c *clientv3.Client) *KubernetesClient {
	return fromClient(c, nil)
}

func fromClient(c *clientv3.Client, close func() error) *KubernetesClient {
	return &KubernetesClient{
		kv:    clientv3.RetryKVClient(c),
		close: close,
	}
}

type KubernetesClient struct {
	kv    pb.KVClient
	close func() error
}

var _ Interface = (*KubernetesClient)(nil)

func (k KubernetesClient) Get(ctx context.Context, key string, opts GetOptions) (resp GetResponse, err error) {
	rangeResp, err := k.kv.Range(ctx, getRequest(key, opts.Revision))
	if err != nil {
		return resp, clientv3.ContextError(ctx, err)
	}
	resp.Revision = rangeResp.Header.Revision
	if len(rangeResp.Kvs) == 1 {
		resp.KV = rangeResp.Kvs[0]
	}
	return resp, nil
}

func (k KubernetesClient) List(ctx context.Context, prefix string, opts ListOptions) (resp ListResponse, err error) {
	rangeStart := prefix + opts.Continue
	rangeEnd := clientv3.GetPrefixRangeEnd(prefix)

	rangeResp, err := k.kv.Range(ctx, &pb.RangeRequest{
		Key:      []byte(rangeStart),
		RangeEnd: []byte(rangeEnd),
		Limit:    opts.Limit,
		Revision: opts.Revision,
	})
	if err != nil {
		return resp, clientv3.ContextError(ctx, err)
	}
	resp.KVs = rangeResp.Kvs
	resp.Count = rangeResp.Count
	resp.Revision = rangeResp.Header.Revision
	return resp, nil
}

func (k KubernetesClient) Count(ctx context.Context, prefix string) (int64, error) {
	resp, err := k.kv.Range(ctx, &pb.RangeRequest{
		Key:       []byte(prefix),
		RangeEnd:  []byte(clientv3.GetPrefixRangeEnd(prefix)),
		CountOnly: true,
	})
	if err != nil {
		return 0, clientv3.ContextError(ctx, err)
	}
	return resp.Count, nil
}

func (k KubernetesClient) OptimisticPut(ctx context.Context, key string, value []byte, opts PutOptions) (resp PutResponse, err error) {
	onSuccess := &pb.RequestOp{Request: &pb.RequestOp_RequestPut{RequestPut: &pb.PutRequest{Key: []byte(key), Value: value, Lease: int64(opts.LeaseID)}}}

	var onFailure *pb.RequestOp
	if opts.GetOnFailure {
		onFailure = &pb.RequestOp{Request: &pb.RequestOp_RequestRange{RequestRange: getRequest(key, 0)}}
	}

	txnResp, err := k.optimisticTxn(ctx, key, opts.ExpectedRevision, onSuccess, onFailure)
	if err != nil {
		return resp, clientv3.ContextError(ctx, err)
	}
	resp.Succeeded = txnResp.Succeeded
	resp.Revision = txnResp.Header.Revision
	if opts.GetOnFailure && !txnResp.Succeeded {
		resp.KV = kvFromTxnResponse(txnResp.Responses[0])
	}
	return resp, nil
}

func (k KubernetesClient) OptimisticDelete(ctx context.Context, key string, opts DeleteOptions) (resp DeleteResponse, err error) {
	onSuccess := &pb.RequestOp{Request: &pb.RequestOp_RequestDeleteRange{RequestDeleteRange: &pb.DeleteRangeRequest{Key: []byte(key)}}}

	var onFailure *pb.RequestOp
	if opts.GetOnFailure {
		onFailure = &pb.RequestOp{Request: &pb.RequestOp_RequestRange{RequestRange: getRequest(key, 0)}}
	}

	txnResp, err := k.optimisticTxn(ctx, key, opts.ExpectedRevision, onSuccess, onFailure)
	if err != nil {
		return resp, clientv3.ContextError(ctx, err)
	}
	resp.Succeeded = txnResp.Succeeded
	resp.Revision = txnResp.Header.Revision
	if opts.GetOnFailure && !txnResp.Succeeded {
		resp.KV = kvFromTxnResponse(txnResp.Responses[0])
	}
	return resp, nil
}

func (k KubernetesClient) optimisticTxn(ctx context.Context, key string, expectRevision int64, onSuccess, onFailure *pb.RequestOp) (*pb.TxnResponse, error) {
	txn := &pb.TxnRequest{
		Compare: []*pb.Compare{
			{
				Result:      pb.Compare_EQUAL,
				Target:      pb.Compare_MOD,
				Key:         []byte(key),
				TargetUnion: &pb.Compare_ModRevision{ModRevision: expectRevision},
			},
		},
	}
	if onSuccess != nil {
		txn.Success = []*pb.RequestOp{onSuccess}
	}
	if onFailure != nil {
		txn.Failure = []*pb.RequestOp{onFailure}
	}
	return k.kv.Txn(ctx, txn)
}

func getRequest(key string, revision int64) *pb.RangeRequest {
	return &pb.RangeRequest{
		Key:      []byte(key),
		Revision: revision,
		Limit:    1,
	}
}

func kvFromTxnResponse(resp *pb.ResponseOp) *mvccpb.KeyValue {
	getResponse := resp.GetResponseRange()
	if len(getResponse.Kvs) == 1 {
		return getResponse.Kvs[0]
	}
	return nil
}

func (k KubernetesClient) Close() error {
	if k.close == nil {
		return nil
	}
	return k.close()
}
