package v3rpc

import (
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/pkg/pbutil"
	"github.com/opentracing/opentracing-go"
	"golang.org/x/net/context"
	"google.golang.org/grpc/metadata"
)

type tracedKVServer struct {
	pb.KVServer
	tracer opentracing.Tracer
}

func NewTracedKVServer(tracer opentracing.Tracer, kv pb.KVServer) pb.KVServer {
	return &tracedKVServer{
		KVServer: kv,
		tracer:   tracer,
	}
}

func (t *tracedKVServer) Range(ctx context.Context, r *pb.RangeRequest) (*pb.RangeResponse, error) {
	sp, ctx := startSpanFromMetadata(ctx, "kvServer/v3/Range")
	defer sp.Finish()
	return t.KVServer.Range(ctx, r)
}

func (t *tracedKVServer) Put(ctx context.Context, r *pb.PutRequest) (*pb.PutResponse, error) {
	sp, ctx := startSpanFromMetadata(ctx, "kvServer/v3/Put")
	defer sp.Finish()

	return t.KVServer.Put(ctx, r)
}

func (t *tracedKVServer) DeleteRange(ctx context.Context, r *pb.DeleteRangeRequest) (*pb.DeleteRangeResponse, error) {
	sp, ctx := startSpanFromMetadata(ctx, "kvServer/v3/DeleteRange")
	defer sp.Finish()

	return t.KVServer.DeleteRange(ctx, r)
}

func (t *tracedKVServer) Txn(ctx context.Context, r *pb.TxnRequest) (*pb.TxnResponse, error) {
	sp, ctx := startSpanFromMetadata(ctx, "kvServer/v3/DeleteRange")
	defer sp.Finish()

	return t.KVServer.Txn(ctx, r)
}

// Compact compacts the event history in the etcd key-value store. The key-value
// store should be periodically compacted or the event history will continue to grow
// indefinitely.
func (t *tracedKVServer) Compact(ctx context.Context, r *pb.CompactionRequest) (*pb.CompactionResponse, error) {
	sp, ctx := startSpanFromMetadata(ctx, "kvServer/v3/Compact")
	defer sp.Finish()

	return t.KVServer.Compact(ctx, r)
}

func (t *tracedKVServer) startSpanFromMetadata(ctx context.Context, operationName string) (opentracing.Span, context.Context) {
	md, ok := metadata.FromContext(ctx)
	if ok {
		sp, err := t.tracer.Join(operationName, opentracing.TextMap, pbutil.MetadataReaderWriter{&md})
		if err != nil {
			plog.Warningf("Couldn't join trace %v", err)
			sp = t.tracer.StartSpan(operationName)
		}
		return sp, opentracing.ContextWithSpan(ctx, sp)
	}
	sp := t.tracer.StartSpan(operationName)
	return sp, opentracing.ContextWithSpan(ctx, sp)
}

func startSpanFromMetadata(ctx context.Context, operationName string) (opentracing.Span, context.Context) {
	md, ok := metadata.FromContext(ctx)
	if ok {
		sp, err := opentracing.GlobalTracer().Join(operationName, opentracing.TextMap, pbutil.MetadataReaderWriter{&md})
		if err != nil {
			plog.Warningf("Couldn't join trace %v", err)
			sp = opentracing.StartSpan(operationName)
		}
		return sp, opentracing.ContextWithSpan(ctx, sp)
	}
	sp := opentracing.StartSpan(operationName)
	return sp, opentracing.ContextWithSpan(ctx, sp)
}
