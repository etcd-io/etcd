package mvcc

import (
	"os"
	"reflect"
	"testing"

	"github.com/coreos/etcd/auth"
	"github.com/coreos/etcd/auth/authpb"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/lease"
	"github.com/coreos/etcd/mvcc/backend"
)

const (
	T_RIGHTS_NONE  = 0
	T_RIGHTS_VIEW  = uint32(authpb.BUILTIN_RIGHTS_VIEW)
	T_RIGHTS_SETUP = 0x4
	T_RIGHTS_ROOT  = 0x8

	T_PI_Root     = 1
	T_PI_Channels = 2
	T_PI_Channel  = 3
	T_PI_Info     = 4
	T_PI_InfoSub  = 5
)

func newTestProtoCache() *auth.PrototypeCache {
	return auth.NewPrototypeCache(15, 5, []int64{1, 2, 3, 4, 5}, []*authpb.Prototype{
		&authpb.Prototype{
			Name:   []byte("Root"),
			Fields: []*authpb.PrototypeField{},
			Flags:  uint32(authpb.NONE),
		},
		&authpb.Prototype{
			Name: []byte("Channels"),
			Fields: []*authpb.PrototypeField{
				&authpb.PrototypeField{Key: "test1", RightsRead: T_RIGHTS_VIEW, RightsWrite: T_RIGHTS_SETUP},
				&authpb.PrototypeField{Key: "test2", RightsRead: T_RIGHTS_SETUP, RightsWrite: T_RIGHTS_SETUP},
				&authpb.PrototypeField{Key: "lic_count", RightsRead: T_RIGHTS_SETUP, RightsWrite: T_RIGHTS_ROOT},
			},
			Flags: uint32(authpb.FORCE_SUBOBJECTS_FIND),
		},
		&authpb.Prototype{
			Name: []byte("Channel"),
			Fields: []*authpb.PrototypeField{
				&authpb.PrototypeField{Key: "key0", RightsRead: T_RIGHTS_VIEW, RightsWrite: T_RIGHTS_VIEW},
				&authpb.PrototypeField{Key: "key1", RightsRead: T_RIGHTS_VIEW, RightsWrite: T_RIGHTS_VIEW},
				&authpb.PrototypeField{Key: "key2", RightsRead: T_RIGHTS_VIEW, RightsWrite: T_RIGHTS_VIEW},
				&authpb.PrototypeField{Key: "key3", RightsRead: T_RIGHTS_VIEW, RightsWrite: T_RIGHTS_VIEW},
				&authpb.PrototypeField{Key: "skey0", RightsRead: T_RIGHTS_VIEW, RightsWrite: T_RIGHTS_SETUP},
				&authpb.PrototypeField{Key: "skey1", RightsRead: T_RIGHTS_VIEW, RightsWrite: T_RIGHTS_SETUP},
				&authpb.PrototypeField{Key: "skey2", RightsRead: T_RIGHTS_VIEW, RightsWrite: T_RIGHTS_SETUP},
			},
			Flags: uint32(authpb.NONE),
		},
		&authpb.Prototype{
			Name: []byte("Info"),
			Fields: []*authpb.PrototypeField{
				&authpb.PrototypeField{Key: "data0", RightsRead: T_RIGHTS_VIEW, RightsWrite: T_RIGHTS_SETUP},
				&authpb.PrototypeField{Key: "data1", RightsRead: T_RIGHTS_VIEW, RightsWrite: T_RIGHTS_SETUP},
				&authpb.PrototypeField{Key: "sdata", RightsRead: T_RIGHTS_SETUP, RightsWrite: T_RIGHTS_ROOT},
			},
			Flags: uint32(authpb.NONE),
		},
		&authpb.Prototype{
			Name: []byte("InfoSub"),
			Fields: []*authpb.PrototypeField{
				&authpb.PrototypeField{Key: "idata0", RightsRead: T_RIGHTS_VIEW, RightsWrite: T_RIGHTS_SETUP},
				&authpb.PrototypeField{Key: "idata1", RightsRead: T_RIGHTS_VIEW, RightsWrite: T_RIGHTS_SETUP},
			},
			Flags: uint32(authpb.FORCE_SUBOBJECTS_FIND),
		},
	})
}

func newTestAclCache(acl []*authpb.AclEntry) *auth.AclCache {
	return auth.NewAclCache(5, acl)
}

func newTestCapturedState(acl []*authpb.AclEntry) *auth.CapturedState {
	return auth.NewCapturedState(newTestProtoCache(), newTestAclCache(acl))
}

func newTestRootCapturedState() *auth.CapturedState {
	return auth.NewCapturedState(newTestProtoCache(), nil)
}

func fillTestStore(vw WriteView) {
	vw.Put([]byte("/"), []byte("Root"), lease.NoLease, PrototypeInfo{T_PI_Root, 0})
	vw.Put([]byte("/channels/"), []byte("src1,Channels"), lease.NoLease, PrototypeInfo{T_PI_Channels, 0})
	vw.Put([]byte("/channels/abc123/"), []byte("src1,Channel"), lease.NoLease, PrototypeInfo{T_PI_Channel, 1})
	vw.Put([]byte("/channels/abc456/"), []byte("src1,Channel"), lease.NoLease, PrototypeInfo{T_PI_Channel, 1})
	vw.Put([]byte("/channels/def789/"), []byte("src1,Channel"), lease.NoLease, PrototypeInfo{T_PI_Channel, 1})
	vw.Put([]byte("/channels/abc123/isub/"), []byte("src1,InfoSub"), lease.NoLease, PrototypeInfo{T_PI_InfoSub, 0})
	vw.Put([]byte("/channels/isub/"), []byte("src1,InfoSub"), lease.NoLease, PrototypeInfo{T_PI_InfoSub, 1})
	vw.Put([]byte("/channels/isub/info/"), []byte("src1,Info"), lease.NoLease, PrototypeInfo{T_PI_Info, 2})
	vw.Put([]byte("/channels/abc456/info/"), []byte("src1,Info"), lease.NoLease, PrototypeInfo{T_PI_Info, 0})
	vw.Put([]byte("/channels/test2"), []byte("src1,bla2"), lease.NoLease, PrototypeInfo{T_PI_Channels, 0})
	vw.Put([]byte("/channels/abc123/key1"), []byte("src1,key1val"), lease.NoLease, PrototypeInfo{T_PI_Channel, 1})
	vw.Put([]byte("/channels/abc123/skey0"), []byte("src1,skey0val"), lease.NoLease, PrototypeInfo{T_PI_Channel, 1})
	vw.Put([]byte("/channels/isub/info/data1"), []byte("src1,data1val"), lease.NoLease, PrototypeInfo{T_PI_Info, 2})
	vw.Put([]byte("/channels/abc456/info/sdata"), []byte("src1,sdataval1"), lease.NoLease, PrototypeInfo{T_PI_Info, 0})
	vw.Put([]byte("/channels/abc456/key2"), []byte("src1,key2val"), lease.NoLease, PrototypeInfo{T_PI_Channel, 1})
}

func TestAclUtilPathFuncs(t *testing.T) {
	if !auth.PathIsDir([]byte("/a/b/c/")) {
		t.Errorf("pathIsDir(/a/b/c/) failed")
	}
	if auth.PathIsDir([]byte("/a/b/c")) {
		t.Errorf("pathIsDir(/a/b/c) failed")
	}
	if auth.PathIsDir([]byte("")) {
		t.Errorf("pathIsDir() failed")
	}
	if !auth.PathIsDir([]byte("/")) {
		t.Errorf("pathIsDir(/) failed")
	}
	if string(auth.PathGetProtoName([]byte("src1,Channel"))) != "Channel" {
		t.Errorf("pathGetProtoName(src1,Channel) failed")
	}
	if string(auth.PathGetProtoName([]byte("Channel"))) != "Channel" {
		t.Errorf("pathGetProtoName(Channel) failed")
	}
	if string(auth.PathGetProtoName([]byte("/Channel"))) != "/Channel" {
		t.Errorf("pathGetProtoName(/Channel) failed")
	}
	if auth.PathGetProtoName([]byte("src1,")) != nil {
		t.Errorf("pathGetProtoName(src1,) failed")
	}
	if auth.PathGetProtoName([]byte("src1,/")) != nil {
		t.Errorf("pathGetProtoName(src1,/) failed")
	}
	if auth.PathGetProtoName([]byte("src1,/a/b/c")) != nil {
		t.Errorf("pathGetProtoName(src1,/a/b/c) failed")
	}
	if auth.PathGetProtoName([]byte("")) != nil {
		t.Errorf("pathGetProtoName() failed")
	}
	if string(auth.PathGetProtoName([]byte(",A"))) != "A" {
		t.Errorf("pathGetProtoName(,A) failed")
	}
	if auth.PathGetProtoName([]byte(",")) != nil {
		t.Errorf("pathGetProtoName(,) failed")
	}
	if string(auth.PathGetPrefix([]byte("/a/b/test1"), 0)) != "/a/b/test1" {
		t.Errorf("pathGetPrefix(/a/b/test1, 0) failed")
	}
	if string(auth.PathGetPrefix([]byte("/a/b/test1"), 1)) != "/a/b/" {
		t.Errorf("pathGetPrefix(/a/b/test1, 1) failed")
	}
	if string(auth.PathGetPrefix([]byte("/a/b/test1"), 2)) != "/a/" {
		t.Errorf("pathGetPrefix(/a/b/test1, 2) failed")
	}
	if string(auth.PathGetPrefix([]byte("/a/b/test1"), 3)) != "/" {
		t.Errorf("pathGetPrefix(/a/b/test1, 3) failed")
	}
	if auth.PathGetPrefix([]byte("/a/b/test1"), 4) != nil {
		t.Errorf("pathGetPrefix(/a/b/test1, 4) failed")
	}
	if string(auth.PathGetPrefix([]byte("/a/b/"), 1)) != "/a/" {
		t.Errorf("pathGetPrefix(/a/b/, 1) failed")
	}
	if string(auth.PathGetPrefix([]byte("/a/b/c"), 1)) != "/a/b/" {
		t.Errorf("pathGetPrefix(/a/b/c, 1) failed")
	}
	if auth.PathGetPrefix([]byte(""), 1) != nil {
		t.Errorf("pathGetPrefix(, 1) failed")
	}
}

func TestAclUtilCheckPutRoot(t *testing.T) {
	tests := []struct {
		requests []*pb.PutRequest
		results  []CheckPutResult
	}{
		{
			requests: []*pb.PutRequest{
				&pb.PutRequest{Key: []byte("/abba/111/data1"), Value: []byte("src2,hhhhh")},
				&pb.PutRequest{Key: []byte("/channels/ch1/isub/"), Value: []byte("src2,InfoSub")},
				&pb.PutRequest{Key: []byte("/channels/ch1/key2"), Value: []byte("src2,kkk")},
				&pb.PutRequest{Key: []byte("/channels/isub/info/data1"), Value: []byte("src2,pppppppp")},
				&pb.PutRequest{Key: []byte("/channels/ch1/skey2"), Value: []byte("src2,kkkooo")},
				&pb.PutRequest{Key: []byte("/channels/ch1/"), Value: []byte("src2,Channel")},
				&pb.PutRequest{Key: []byte("/channels/ch1/isub/isub/"), Value: []byte("src2,InfoSub")},
				&pb.PutRequest{Key: []byte("/channels/ch2/"), Value: []byte("src2,Channel")},
				&pb.PutRequest{Key: []byte("/channels/i2/i3/"), Value: []byte("src2,InfoSub")},
				&pb.PutRequest{Key: []byte("/channels/i2/"), Value: []byte("src2,InfoSub")},
				&pb.PutRequest{Key: []byte("/abba/"), Value: []byte("src2,Channel")},
				&pb.PutRequest{Key: []byte("/abba/111/"), Value: []byte("src2,Info")},
			},
			results: []CheckPutResult{
				CheckPutResult{CanWrite: true, CanRead: true, ProtoInfo: PrototypeInfo{T_PI_Info, 0}},
				CheckPutResult{CanWrite: true, CanRead: true, ProtoInfo: PrototypeInfo{T_PI_InfoSub, 0}},
				CheckPutResult{CanWrite: true, CanRead: true, ProtoInfo: PrototypeInfo{T_PI_Channel, 1}},
				CheckPutResult{CanWrite: true, CanRead: true, ProtoInfo: PrototypeInfo{T_PI_Info, 2}},
				CheckPutResult{CanWrite: true, CanRead: true, ProtoInfo: PrototypeInfo{T_PI_Channel, 1}},
				CheckPutResult{CanWrite: true, CanRead: true, ProtoInfo: PrototypeInfo{T_PI_Channel, 1}},
				CheckPutResult{CanWrite: true, CanRead: true, ProtoInfo: PrototypeInfo{T_PI_InfoSub, 1}},
				CheckPutResult{CanWrite: true, CanRead: true, ProtoInfo: PrototypeInfo{T_PI_Channel, 1}},
				CheckPutResult{CanWrite: true, CanRead: true, ProtoInfo: PrototypeInfo{T_PI_InfoSub, 2}},
				CheckPutResult{CanWrite: true, CanRead: true, ProtoInfo: PrototypeInfo{T_PI_InfoSub, 1}},
				CheckPutResult{CanWrite: true, CanRead: true, ProtoInfo: PrototypeInfo{T_PI_Channel, 0}},
				CheckPutResult{CanWrite: true, CanRead: true, ProtoInfo: PrototypeInfo{T_PI_Info, 0}},
			},
		},
	}

	b, tmpPath := backend.NewDefaultTmpBackend()
	s := NewStore(b, &lease.FakeLessor{}, nil)
	defer s.Close()
	defer os.Remove(tmpPath)

	fillTestStore(s)

	cs := newTestRootCapturedState()

	txn := s.Read()

	for i, tst := range tests {
		res := CheckPut(txn, cs, tst.requests)
		if len(res) != len(tst.results) {
			t.Errorf("#%d: failed, size mismatch", i)
		}
		for j, r := range res {
			if !reflect.DeepEqual(r, tst.results[j]) {
				t.Errorf("#%d: failed at %d: %v != %v", i, j, r, tst.results[j])
			}
		}
	}

	txn.End()
}

func TestAclUtilCheckPutUser(t *testing.T) {
	tests := []struct {
		requests []*pb.PutRequest
		results  []CheckPutResult
	}{
		{
			requests: []*pb.PutRequest{
				&pb.PutRequest{Key: []byte("/abba/111/data1"), Value: []byte("src2,hhhhh")},
				&pb.PutRequest{Key: []byte("/channels/-----"), Value: []byte("fdfdfd")},
				&pb.PutRequest{Key: []byte("/channels/test1"), Value: []byte("aaa")},
				&pb.PutRequest{Key: []byte("/channels/test2"), Value: []byte("aaa")},
				&pb.PutRequest{Key: []byte("/channels/isub"), Value: []byte("bbb")},
			},
			results: []CheckPutResult{
				CheckPutResult{CanWrite: false, CanRead: false, ProtoInfo: PrototypeInfo{}},
				CheckPutResult{CanWrite: false, CanRead: false, ProtoInfo: PrototypeInfo{T_PI_Channels, 0}},
				CheckPutResult{CanWrite: false, CanRead: true, ProtoInfo: PrototypeInfo{T_PI_Channels, 0}},
				CheckPutResult{CanWrite: false, CanRead: false, ProtoInfo: PrototypeInfo{T_PI_Channels, 0}},
				CheckPutResult{CanWrite: false, CanRead: false, ProtoInfo: PrototypeInfo{T_PI_Channels, 0}},
			},
		},
		{
			requests: []*pb.PutRequest{
				&pb.PutRequest{Key: []byte("/channels/abc456/"), Value: []byte("kkk")},
				&pb.PutRequest{Key: []byte("/channels/abc123/"), Value: []byte("kkk")},
				&pb.PutRequest{Key: []byte("/channels/abc123/skey0"), Value: []byte("kkk")},
				&pb.PutRequest{Key: []byte("/channels/abc456/skey0"), Value: []byte("kkk")},
				&pb.PutRequest{Key: []byte("/channels/def789/skey0"), Value: []byte("kkk")},
			},
			results: []CheckPutResult{
				CheckPutResult{CanWrite: false, CanRead: true, ProtoInfo: PrototypeInfo{T_PI_Channel, 1}},
				CheckPutResult{CanWrite: false, CanRead: true, ProtoInfo: PrototypeInfo{T_PI_Channel, 1}},
				CheckPutResult{CanWrite: true, CanRead: true, ProtoInfo: PrototypeInfo{T_PI_Channel, 1}},
				CheckPutResult{CanWrite: false, CanRead: false, ProtoInfo: PrototypeInfo{T_PI_Channel, 1}},
				CheckPutResult{CanWrite: false, CanRead: false, ProtoInfo: PrototypeInfo{T_PI_Channel, 1}},
			},
		},
	}

	b, tmpPath := backend.NewDefaultTmpBackend()
	s := NewStore(b, &lease.FakeLessor{}, nil)
	defer s.Close()
	defer os.Remove(tmpPath)

	fillTestStore(s)

	cs := newTestCapturedState([]*authpb.AclEntry{
		{Path: "", RightsSet: 0, RightsUnset: 0},
		{Path: "/channels", RightsSet: T_RIGHTS_VIEW, RightsUnset: 0},
		{Path: "/channels/abc456", RightsSet: 0, RightsUnset: T_RIGHTS_VIEW},
		{Path: "/channels/def789", RightsSet: 0, RightsUnset: T_RIGHTS_VIEW},
		{Path: "/channels/abc123", RightsSet: T_RIGHTS_SETUP, RightsUnset: 0},
	})

	txn := s.Read()

	for i, tst := range tests {
		res := CheckPut(txn, cs, tst.requests)
		if len(res) != len(tst.results) {
			t.Errorf("#%d: failed, size mismatch", i)
		}
		for j, r := range res {
			if !reflect.DeepEqual(r, tst.results[j]) {
				t.Errorf("#%d: failed at %d: %v != %v", i, j, r, tst.results[j])
			}
		}
	}

	txn.End()
}
