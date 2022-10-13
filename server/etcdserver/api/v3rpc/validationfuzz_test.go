package v3rpc

import (
	"context"
	"testing"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	txn "go.etcd.io/etcd/server/v3/etcdserver/txn"
	"go.etcd.io/etcd/server/v3/lease"
	betesting "go.etcd.io/etcd/server/v3/storage/backend/testing"
	"go.etcd.io/etcd/server/v3/storage/mvcc"
	"go.uber.org/zap/zaptest"
)

func FuzzRangeRequest(f *testing.F) {
	testcases := []pb.RangeRequest{
		{
			Key:        []byte{2},
			RangeEnd:   []byte{2},
			Limit:      3,
			Revision:   3,
			SortOrder:  2,
			SortTarget: 2,
		},
	}

	for _, tc := range testcases {
		soValue := pb.RangeRequest_SortOrder_value[tc.SortOrder.String()]
		soTarget := pb.RangeRequest_SortTarget_value[tc.SortTarget.String()]
		f.Add(tc.Key, tc.RangeEnd, tc.Limit, tc.Revision, soValue, soTarget) // Use f.Add to provide a seed corpus
	}
	f.Fuzz(func(t *testing.T,
		key []byte,
		rangeEnd []byte,
		limit int64,
		revision int64,
		sortOrder int32,
		sortTarget int32,
	) {
		b, _ := betesting.NewDefaultTmpBackend(t)
		defer betesting.Close(t, b)
		s := mvcc.NewStore(zaptest.NewLogger(t), b, &lease.FakeLessor{}, mvcc.StoreConfig{})
		defer s.Close()

		// setup cancelled context
		ctx, cancel := context.WithCancel(context.TODO())
		cancel()
		// put some data to prevent early termination in rangeKeys
		// we are expecting failure on cancelled context check
		s.Put(key, []byte("bar"), lease.NoLease)

		request := &pb.TxnRequest{
			Success: []*pb.RequestOp{
				{
					Request: &pb.RequestOp_RequestRange{
						RequestRange: &pb.RangeRequest{
							Key:        key,
							RangeEnd:   rangeEnd,
							Limit:      limit,
							SortOrder:  pb.RangeRequest_SortOrder(sortOrder),
							SortTarget: pb.RangeRequest_SortTarget(sortTarget),
						},
					},
				},
			},
		}
		errCheck := checkRangeRequest(&pb.RangeRequest{
			Key:        key,
			RangeEnd:   rangeEnd,
			Limit:      limit,
			SortOrder:  pb.RangeRequest_SortOrder(sortOrder),
			SortTarget: pb.RangeRequest_SortTarget(sortTarget),
		})

		if errCheck != nil {
			t.Skip("Validation not passing. Skipping the apply.")
		}

		_, _, err := txn.Txn(ctx, zaptest.NewLogger(t), request, false, s, &lease.FakeLessor{})
		if err != nil {
			t.Logf("Check: %s | Apply: %s", errCheck, err)
			t.Skip("Application erroring.")
		}
	})
}

func FuzzPutRequest(f *testing.F) {
	testcases := []pb.PutRequest{
		{
			Key:         []byte{2},
			Value:       []byte{2},
			Lease:       2,
			PrevKv:      false,
			IgnoreValue: false,
			IgnoreLease: false,
		},
	}

	for _, tc := range testcases {
		f.Add(tc.Key, tc.Value, tc.Lease, tc.PrevKv, tc.IgnoreValue, tc.IgnoreLease) // Use f.Add to provide a seed corpus
	}

	f.Fuzz(func(t *testing.T,
		key []byte,
		value []byte,
		leaseValue int64,
		prevKv bool,
		ignoreValue bool,
		IgnoreLease bool,
	) {
		b, _ := betesting.NewDefaultTmpBackend(t)
		defer betesting.Close(t, b)
		s := mvcc.NewStore(zaptest.NewLogger(t), b, &lease.FakeLessor{}, mvcc.StoreConfig{})
		defer s.Close()

		// setup cancelled context
		ctx, cancel := context.WithCancel(context.TODO())
		cancel()
		// put some data to prevent early termination in rangeKeys
		// we are expecting failure on cancelled context check
		s.Put(key, []byte("bar"), lease.NoLease)

		request := &pb.TxnRequest{
			Success: []*pb.RequestOp{
				{
					Request: &pb.RequestOp_RequestPut{
						RequestPut: &pb.PutRequest{
							Key:         key,
							Value:       value,
							Lease:       leaseValue,
							PrevKv:      prevKv,
							IgnoreValue: ignoreValue,
							IgnoreLease: IgnoreLease,
						},
					},
				},
			},
		}
		errCheck := checkPutRequest(&pb.PutRequest{
			Key:         key,
			Value:       value,
			Lease:       leaseValue,
			PrevKv:      prevKv,
			IgnoreValue: ignoreValue,
			IgnoreLease: IgnoreLease,
		})

		if errCheck != nil {
			t.Skip("Validation not passing. Skipping the apply.")
		}

		_, _, err := txn.Txn(ctx, zaptest.NewLogger(t), request, false, s, &lease.FakeLessor{})
		if err != nil {
			t.Logf("Check: %s | Apply: %s", errCheck, err)
			t.Skip("Application erroring.")
		}
	})
}

func FuzzDeleteRangeRequest(f *testing.F) {
	testcases := []pb.DeleteRangeRequest{
		{
			Key:      []byte{2},
			RangeEnd: []byte{2},
			PrevKv:   false,
		},
	}

	for _, tc := range testcases {
		f.Add(tc.Key, tc.RangeEnd, tc.PrevKv) // Use f.Add to provide a seed corpus
	}

	f.Fuzz(func(t *testing.T,
		key []byte,
		rangeEnd []byte,
		prevKv bool,
	) {
		b, _ := betesting.NewDefaultTmpBackend(t)
		defer betesting.Close(t, b)
		s := mvcc.NewStore(zaptest.NewLogger(t), b, &lease.FakeLessor{}, mvcc.StoreConfig{})
		defer s.Close()

		// setup cancelled context
		ctx, cancel := context.WithCancel(context.TODO())
		cancel()
		// put some data to prevent early termination in rangeKeys
		// we are expecting failure on cancelled context check
		s.Put(key, []byte("bar"), lease.NoLease)

		request := &pb.TxnRequest{
			Success: []*pb.RequestOp{
				{
					Request: &pb.RequestOp_RequestDeleteRange{
						RequestDeleteRange: &pb.DeleteRangeRequest{
							Key:      key,
							RangeEnd: rangeEnd,
							PrevKv:   prevKv,
						},
					},
				},
			},
		}
		errCheck := checkDeleteRequest(&pb.DeleteRangeRequest{
			Key:      key,
			RangeEnd: rangeEnd,
			PrevKv:   prevKv,
		})

		if errCheck != nil {
			t.Skip("Validation not passing. Skipping the apply.")
		}

		_, _, err := txn.Txn(ctx, zaptest.NewLogger(t), request, false, s, &lease.FakeLessor{})
		if err != nil {
			t.Logf("Check: %s | Apply: %s", errCheck, err)
			t.Skip("Application erroring.")
		}
	})
}
