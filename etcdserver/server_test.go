package etcdserver

import (
	"fmt"
	"math/rand"
	"reflect"
	"runtime"
	"sync"
	"testing"
	"time"

	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/store"
	"github.com/coreos/etcd/third_party/code.google.com/p/go.net/context"
)

// TestDoLocalAction tests requests which do not need to go through raft to be applied,
// and are served through local data.
func TestDoLocalAction(t *testing.T) {
	tests := []struct {
		req pb.Request

		wresp   Response
		werr    error
		waction []string
	}{
		{
			pb.Request{Method: "GET", Id: 1, Wait: true},
			Response{Watcher: &stubWatcher{}}, nil, []string{"Watch"},
		},
		{
			pb.Request{Method: "GET", Id: 1},
			Response{Event: &store.Event{}}, nil, []string{"Get"},
		},
		{
			pb.Request{Method: "BADMETHOD", Id: 1},
			Response{}, ErrUnknownMethod, []string{},
		},
	}
	for i, tt := range tests {
		st := &storeRecorder{}
		srv := &EtcdServer{Store: st}
		resp, err := srv.Do(context.TODO(), tt.req)

		if err != tt.werr {
			t.Fatalf("#%d: err = %+v, want %+v", i, err, tt.werr)
		}
		if !reflect.DeepEqual(resp, tt.wresp) {
			t.Errorf("#%d: resp = %+v, want %+v", i, resp, tt.wresp)
		}
		action := st.Action()
		if !reflect.DeepEqual(action, tt.waction) {
			t.Errorf("#%d: action = %+v, want %+v", i, action, tt.waction)
		}
	}
}

func TestApply(t *testing.T) {
	tests := []struct {
		req pb.Request

		wresp   Response
		waction []string
	}{
		{
			pb.Request{Method: "POST", Id: 1},
			Response{Event: &store.Event{}}, []string{"Create"},
		},
		{
			pb.Request{Method: "PUT", Id: 1, PrevExists: boolp(true), PrevIndex: 1},
			Response{Event: &store.Event{}}, []string{"Update"},
		},
		{
			pb.Request{Method: "PUT", Id: 1, PrevExists: boolp(false), PrevIndex: 1},
			Response{Event: &store.Event{}}, []string{"Create"},
		},
		{
			pb.Request{Method: "PUT", Id: 1, PrevExists: boolp(true)},
			Response{Event: &store.Event{}}, []string{"Update"},
		},
		{
			pb.Request{Method: "PUT", Id: 1, PrevExists: boolp(false)},
			Response{Event: &store.Event{}}, []string{"Create"},
		},
		{
			pb.Request{Method: "PUT", Id: 1, PrevIndex: 1},
			Response{Event: &store.Event{}}, []string{"CompareAndSwap"},
		},
		{
			pb.Request{Method: "PUT", Id: 1, PrevValue: "bar"},
			Response{Event: &store.Event{}}, []string{"CompareAndSwap"},
		},
		{
			pb.Request{Method: "PUT", Id: 1},
			Response{Event: &store.Event{}}, []string{"Set"},
		},
		{
			pb.Request{Method: "DELETE", Id: 1, PrevIndex: 1},
			Response{Event: &store.Event{}}, []string{"CompareAndDelete"},
		},
		{
			pb.Request{Method: "DELETE", Id: 1, PrevValue: "bar"},
			Response{Event: &store.Event{}}, []string{"CompareAndDelete"},
		},
		{
			pb.Request{Method: "DELETE", Id: 1},
			Response{Event: &store.Event{}}, []string{"Delete"},
		},
		{
			pb.Request{Method: "QGET", Id: 1},
			Response{Event: &store.Event{}}, []string{"Get"},
		},
		{
			pb.Request{Method: "SYNC", Id: 1},
			Response{}, []string{"DeleteExpiredKeys"},
		},
		{
			pb.Request{Method: "BADMETHOD", Id: 1},
			Response{err: ErrUnknownMethod}, []string{},
		},
	}

	for i, tt := range tests {
		st := &storeRecorder{}
		srv := &EtcdServer{Store: st}
		resp := srv.apply(tt.req)

		if !reflect.DeepEqual(resp, tt.wresp) {
			t.Errorf("#%d: resp = %+v, want %+v", i, resp, tt.wresp)
		}
		action := st.Action()
		if !reflect.DeepEqual(action, tt.waction) {
			t.Errorf("#%d: action = %+v, want %+v", i, action, tt.waction)
		}
	}
}

func TestClusterOf1(t *testing.T) { testServer(t, 1) }
func TestClusterOf3(t *testing.T) { testServer(t, 3) }

func testServer(t *testing.T, ns int64) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ss := make([]*EtcdServer, ns)

	send := func(msgs []raftpb.Message) {
		for _, m := range msgs {
			t.Logf("m = %+v\n", m)
			ss[m.To-1].Node.Step(ctx, m)
		}
	}

	peers := make([]int64, ns)
	for i := int64(0); i < ns; i++ {
		peers[i] = i + 1
	}

	for i := int64(0); i < ns; i++ {
		id := i + 1
		n := raft.StartNode(id, peers, 10, 1)
		tk := time.NewTicker(10 * time.Millisecond)
		defer tk.Stop()
		srv := &EtcdServer{
			Node:    n,
			Store:   store.New(),
			Send:    send,
			Storage: &storageRecorder{},
			Ticker:  tk.C,
		}
		srv.Start()
		// TODO(xiangli): randomize election timeout
		// then remove this sleep.
		time.Sleep(1 * time.Millisecond)
		ss[i] = srv
	}

	for i := 1; i <= 10; i++ {
		r := pb.Request{
			Method: "PUT",
			Id:     int64(i),
			Path:   "/foo",
			Val:    "bar",
		}
		j := rand.Intn(len(ss))
		t.Logf("ss = %d", j)
		resp, err := ss[j].Do(ctx, r)
		if err != nil {
			t.Fatal(err)
		}

		g, w := resp.Event.Node, &store.NodeExtern{
			Key:           "/foo",
			ModifiedIndex: uint64(i),
			CreatedIndex:  uint64(i),
			Value:         stringp("bar"),
		}

		if !reflect.DeepEqual(g, w) {
			t.Error("value:", *g.Value)
			t.Errorf("g = %+v, w %+v", g, w)
		}
	}

	time.Sleep(10 * time.Millisecond)

	var last interface{}
	for i, sv := range ss {
		sv.Stop()
		g, _ := sv.Store.Get("/", true, true)
		if last != nil && !reflect.DeepEqual(last, g) {
			t.Errorf("server %d: Root = %#v, want %#v", i, g, last)
		}
		last = g
	}
}

func TestDoProposal(t *testing.T) {
	tests := []pb.Request{
		pb.Request{Method: "POST", Id: 1},
		pb.Request{Method: "PUT", Id: 1},
		pb.Request{Method: "DELETE", Id: 1},
		pb.Request{Method: "GET", Id: 1, Quorum: true},
	}

	for i, tt := range tests {
		ctx, _ := context.WithCancel(context.Background())
		n := raft.StartNode(0xBAD0, []int64{0xBAD0}, 10, 1)
		st := &storeRecorder{}
		tk := make(chan time.Time)
		// this makes <-tk always successful, which accelerates internal clock
		close(tk)
		srv := &EtcdServer{
			Node:    n,
			Store:   st,
			Send:    func(_ []raftpb.Message) {},
			Storage: &storageRecorder{},
			Ticker:  tk,
		}
		srv.Start()
		resp, err := srv.Do(ctx, tt)
		srv.Stop()

		action := st.Action()
		if len(action) != 1 {
			t.Errorf("#%d: len(action) = %d, want 1", i, len(action))
		}
		if err != nil {
			t.Fatalf("#%d: err = %v, want nil", i, err)
		}
		wresp := Response{Event: &store.Event{}}
		if !reflect.DeepEqual(resp, wresp) {
			t.Errorf("#%d: resp = %v, want %v", i, resp, wresp)
		}
	}
}

func TestDoProposalCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	// node cannot make any progress because there are two nodes
	n := raft.StartNode(0xBAD0, []int64{0xBAD0, 0xBAD1}, 10, 1)
	st := &storeRecorder{}
	wait := &waitRecorder{}
	srv := &EtcdServer{
		// TODO: use fake node for better testability
		Node:  n,
		Store: st,
		w:     wait,
	}

	done := make(chan struct{})
	var err error
	go func() {
		_, err = srv.Do(ctx, pb.Request{Method: "PUT", Id: 1})
		close(done)
	}()
	cancel()
	<-done

	action := st.Action()
	if len(action) != 0 {
		t.Errorf("len(action) = %v, want 0", len(action))
	}
	if err != context.Canceled {
		t.Fatalf("err = %v, want %v", err, context.Canceled)
	}
	w := []string{"Register1", "Trigger1"}
	if !reflect.DeepEqual(wait.action, w) {
		t.Errorf("wait.action = %+v, want %+v", wait.action, w)
	}
}

func TestDoProposalStopped(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// node cannot make any progress because there are two nodes
	n := raft.StartNode(0xBAD0, []int64{0xBAD0, 0xBAD1}, 10, 1)
	st := &storeRecorder{}
	tk := make(chan time.Time)
	// this makes <-tk always successful, which accelarates internal clock
	close(tk)
	srv := &EtcdServer{
		// TODO: use fake node for better testability
		Node:    n,
		Store:   st,
		Send:    func(_ []raftpb.Message) {},
		Storage: &storageRecorder{},
		Ticker:  tk,
	}
	srv.Start()

	done := make(chan struct{})
	var err error
	go func() {
		_, err = srv.Do(ctx, pb.Request{Method: "PUT", Id: 1})
		close(done)
	}()
	srv.Stop()
	<-done

	action := st.Action()
	if len(action) != 0 {
		t.Errorf("len(action) = %v, want 0", len(action))
	}
	if err != ErrStopped {
		t.Errorf("err = %v, want %v", err, ErrStopped)
	}
}

// TestSync tests sync 1. is nonblocking 2. sends out SYNC request.
func TestSync(t *testing.T) {
	n := raft.StartNode(0xBAD0, []int64{0xBAD0}, 10, 1)
	n.Campaign(context.TODO())
	select {
	case <-n.Ready():
	case <-time.After(time.Millisecond):
		t.Fatalf("expect to receive ready within 1ms, but fail")
	}

	srv := &EtcdServer{
		// TODO: use fake node for better testability
		Node: n,
	}
	start := time.Now()
	srv.sync(defaultSyncTimeout)

	// check that sync is non-blocking
	if d := time.Since(start); d > time.Millisecond {
		t.Errorf("CallSyncTime = %v, want < %v", d, time.Millisecond)
	}

	// give time for goroutine in sync to run
	// TODO: use fake clock
	var ready raft.Ready
	select {
	case ready = <-n.Ready():
	case <-time.After(time.Millisecond):
		t.Fatalf("expect to receive ready within 1ms, but fail")
	}

	if len(ready.CommittedEntries) != 1 {
		t.Fatalf("len(CommittedEntries) = %d, want 1", len(ready.CommittedEntries))
	}
	e := ready.CommittedEntries[0]
	var req pb.Request
	if err := req.Unmarshal(e.Data); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if req.Method != "SYNC" {
		t.Errorf("method = %s, want SYNC", req.Method)
	}
}

// TestSyncFail tests the case that sync 1. is non-blocking 2. fails to
// propose SYNC request because there is no leader
func TestSyncFail(t *testing.T) {
	// The node is run without Tick and Campaign, so it has no leader forever.
	n := raft.StartNode(0xBAD0, []int64{0xBAD0}, 10, 1)
	select {
	case <-n.Ready():
	case <-time.After(time.Millisecond):
		t.Fatalf("expect to receive ready within 1ms, but fail")
	}

	srv := &EtcdServer{
		// TODO: use fake node for better testability
		Node: n,
	}
	routineN := runtime.NumGoroutine()
	start := time.Now()
	srv.sync(time.Millisecond)

	// check that sync is non-blocking
	if d := time.Since(start); d > time.Millisecond {
		t.Errorf("CallSyncTime = %v, want < %v", d, time.Millisecond)
	}

	// give time for goroutine in sync to cancel
	// TODO: use fake clock
	time.Sleep(2 * time.Millisecond)
	if g := runtime.NumGoroutine(); g != routineN {
		t.Errorf("NumGoroutine = %d, want %d", g, routineN)
	}
	select {
	case g := <-n.Ready():
		t.Errorf("ready = %+v, want no", g)
	default:
	}
}

func TestSyncTriggerDeleteExpriedKeys(t *testing.T) {
	n := raft.StartNode(0xBAD0, []int64{0xBAD0}, 10, 1)
	n.Campaign(context.TODO())
	st := &storeRecorder{}
	syncInterval := 5 * time.Millisecond
	syncTicker := time.NewTicker(syncInterval)
	defer syncTicker.Stop()
	srv := &EtcdServer{
		// TODO: use fake node for better testability
		Node:       n,
		Store:      st,
		Send:       func(_ []raftpb.Message) {},
		Storage:    &storageRecorder{},
		SyncTicker: syncTicker.C,
	}
	srv.Start()
	// give time for sync request to be proposed and performed
	// TODO: use fake clock
	time.Sleep(syncInterval + time.Millisecond)
	srv.Stop()

	action := st.Action()
	if len(action) != 1 {
		t.Fatalf("len(action) = %d, want 1", len(action))
	}
	if action[0] != "DeleteExpiredKeys" {
		t.Errorf("action = %s, want DeleteExpiredKeys", action[0])
	}
}

// snapshot should snapshot the store and cut the persistent
// TODO: node.Compact is called... we need to make the node an interface
func TestSnapshot(t *testing.T) {
	n := raft.StartNode(0xBAD0, []int64{0xBAD0}, 10, 1)
	defer n.Stop()
	st := &storeRecorder{}
	p := &storageRecorder{}
	s := &EtcdServer{
		Store:   st,
		Storage: p,
		Node:    n,
	}

	s.snapshot()
	action := st.Action()
	if len(action) != 1 {
		t.Fatalf("len(action) = %d, want 1", len(action))
	}
	if action[0] != "Save" {
		t.Errorf("action = %s, want Save", action[0])
	}

	action = p.Action()
	if len(action) != 1 {
		t.Fatalf("len(action) = %d, want 1", len(action))
	}
	if action[0] != "Cut" {
		t.Errorf("action = %s, want Cut", action[0])
	}
}

// Applied > SnapCount should trigger a SaveSnap event
// TODO: receive a snapshot from raft leader should also be able
// to trigger snapSave and also trigger a store.Recover.
// We need fake node!
func TestTriggerSnap(t *testing.T) {
	ctx := context.Background()
	n := raft.StartNode(0xBAD0, []int64{0xBAD0}, 10, 1)
	n.Campaign(ctx)
	st := &storeRecorder{}
	p := &storageRecorder{}
	s := &EtcdServer{
		Store:     st,
		Send:      func(_ []raftpb.Message) {},
		Storage:   p,
		Node:      n,
		SnapCount: 10,
	}

	s.Start()
	for i := 0; int64(i) < s.SnapCount; i++ {
		s.Do(ctx, pb.Request{Method: "PUT", Id: 1})
	}
	time.Sleep(time.Millisecond)
	s.Stop()

	action := p.Action()
	// each operation is recorded as a Save
	// Nop + SnapCount * Puts + Cut + SaveSnap = Save + SnapCount * Save + Cut + SaveSnap
	if len(action) != 3+int(s.SnapCount) {
		t.Fatalf("len(action) = %d, want %d", len(action), 3+int(s.SnapCount))
	}
	if action[12] != "SaveSnap" {
		t.Errorf("action = %s, want SaveSnap", action[12])
	}
}

// TODO: test wait trigger correctness in multi-server case

func TestGetBool(t *testing.T) {
	tests := []struct {
		b    *bool
		wb   bool
		wset bool
	}{
		{nil, false, false},
		{boolp(true), true, true},
		{boolp(false), false, true},
	}
	for i, tt := range tests {
		b, set := getBool(tt.b)
		if b != tt.wb {
			t.Errorf("#%d: value = %v, want %v", i, b, tt.wb)
		}
		if set != tt.wset {
			t.Errorf("#%d: set = %v, want %v", i, set, tt.wset)
		}
	}
}

type recorder struct {
	sync.Mutex
	action []string
}

func (r *recorder) record(action string) {
	r.Lock()
	r.action = append(r.action, action)
	r.Unlock()
}
func (r *recorder) Action() []string {
	r.Lock()
	cpy := make([]string, len(r.action))
	copy(cpy, r.action)
	r.Unlock()
	return cpy
}

type storeRecorder struct {
	recorder
}

func (s *storeRecorder) Version() int  { return 0 }
func (s *storeRecorder) Index() uint64 { return 0 }
func (s *storeRecorder) Get(_ string, _, _ bool) (*store.Event, error) {
	s.record("Get")
	return &store.Event{}, nil
}
func (s *storeRecorder) Set(_ string, _ bool, _ string, _ time.Time) (*store.Event, error) {
	s.record("Set")
	return &store.Event{}, nil
}
func (s *storeRecorder) Update(_, _ string, _ time.Time) (*store.Event, error) {
	s.record("Update")
	return &store.Event{}, nil
}
func (s *storeRecorder) Create(_ string, _ bool, _ string, _ bool, _ time.Time) (*store.Event, error) {
	s.record("Create")
	return &store.Event{}, nil
}
func (s *storeRecorder) CompareAndSwap(_, _ string, _ uint64, _ string, _ time.Time) (*store.Event, error) {
	s.record("CompareAndSwap")
	return &store.Event{}, nil
}
func (s *storeRecorder) Delete(_ string, _, _ bool) (*store.Event, error) {
	s.record("Delete")
	return &store.Event{}, nil
}
func (s *storeRecorder) CompareAndDelete(_, _ string, _ uint64) (*store.Event, error) {
	s.record("CompareAndDelete")
	return &store.Event{}, nil
}
func (s *storeRecorder) Watch(_ string, _, _ bool, _ uint64) (store.Watcher, error) {
	s.record("Watch")
	return &stubWatcher{}, nil
}
func (s *storeRecorder) Save() ([]byte, error) {
	s.record("Save")
	return nil, nil
}
func (s *storeRecorder) Recovery(b []byte) error   { return nil }
func (s *storeRecorder) TotalTransactions() uint64 { return 0 }
func (s *storeRecorder) JsonStats() []byte         { return nil }
func (s *storeRecorder) DeleteExpiredKeys(cutoff time.Time) {
	s.record("DeleteExpiredKeys")
}

type stubWatcher struct{}

func (w *stubWatcher) EventChan() chan *store.Event { return nil }
func (w *stubWatcher) Remove()                      {}

type waitRecorder struct {
	action []string
}

func (w *waitRecorder) Register(id int64) <-chan interface{} {
	w.action = append(w.action, fmt.Sprint("Register", id))
	return nil
}
func (w *waitRecorder) Trigger(id int64, x interface{}) {
	w.action = append(w.action, fmt.Sprint("Trigger", id))
}

func boolp(b bool) *bool { return &b }

func stringp(s string) *string { return &s }

type storageRecorder struct {
	recorder
}

func (p *storageRecorder) Save(st raftpb.HardState, ents []raftpb.Entry) {
	p.record("Save")
}
func (p *storageRecorder) Cut() error {
	p.record("Cut")
	return nil
}
func (p *storageRecorder) SaveSnap(st raftpb.Snapshot) {
	if raft.IsEmptySnap(st) {
		return
	}
	p.record("SaveSnap")
}
