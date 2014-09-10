package etcdhttp

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"reflect"
	"sync"
	"testing"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/store"
	"github.com/coreos/etcd/third_party/code.google.com/p/go.net/context"
)

func boolp(b bool) *bool { return &b }

func mustNewURL(t *testing.T, s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		t.Fatalf("error creating URL from %q: %v", s, err)
	}
	return u
}

func mustNewRequest(t *testing.T, p string) *http.Request {
	return &http.Request{
		URL: mustNewURL(t, path.Join(keysPrefix, p)),
	}
}

func TestBadParseRequest(t *testing.T) {
	tests := []struct {
		in *http.Request
	}{
		{
			// parseForm failure
			&http.Request{
				Body:   nil,
				Method: "PUT",
			},
		},
		{
			// bad key prefix
			&http.Request{
				URL: mustNewURL(t, "/badprefix/"),
			},
		},
		// bad values for prevIndex, waitIndex, ttl
		{
			mustNewRequest(t, "?prevIndex=foo"),
		},
		{
			mustNewRequest(t, "?prevIndex=1.5"),
		},
		{
			mustNewRequest(t, "?prevIndex=-1"),
		},
		{
			mustNewRequest(t, "?waitIndex=garbage"),
		},
		{
			mustNewRequest(t, "?waitIndex=??"),
		},
		{
			mustNewRequest(t, "?ttl=-1"),
		},
		// bad values for recursive, sorted, wait
		{
			mustNewRequest(t, "?recursive=hahaha"),
		},
		{
			mustNewRequest(t, "?recursive=1234"),
		},
		{
			mustNewRequest(t, "?recursive=?"),
		},
		{
			mustNewRequest(t, "?sorted=hahaha"),
		},
		{
			mustNewRequest(t, "?sorted=!!"),
		},
		{
			mustNewRequest(t, "?wait=notreally"),
		},
		{
			mustNewRequest(t, "?wait=what!"),
		},
	}
	for i, tt := range tests {
		got, err := parseRequest(tt.in, 1234)
		if err == nil {
			t.Errorf("case %d: unexpected nil error!", i)
		}
		if !reflect.DeepEqual(got, etcdserverpb.Request{}) {
			t.Errorf("case %d: unexpected non-empty Request: %#v", i, got)
		}
	}
}

func TestGoodParseRequest(t *testing.T) {
	tests := []struct {
		in *http.Request
		w  etcdserverpb.Request
	}{
		{
			// good prefix, all other values default
			mustNewRequest(t, "foo"),
			etcdserverpb.Request{
				Id:   1234,
				Path: "/foo",
			},
		},
		{
			// value specified
			mustNewRequest(t, "foo?value=some_value"),
			etcdserverpb.Request{
				Id:   1234,
				Val:  "some_value",
				Path: "/foo",
			},
		},
		{
			// prevIndex specified
			mustNewRequest(t, "foo?prevIndex=98765"),
			etcdserverpb.Request{
				Id:        1234,
				PrevIndex: 98765,
				Path:      "/foo",
			},
		},
		{
			// recursive specified
			mustNewRequest(t, "foo?recursive=true"),
			etcdserverpb.Request{
				Id:        1234,
				Recursive: true,
				Path:      "/foo",
			},
		},
		{
			// sorted specified
			mustNewRequest(t, "foo?sorted=true"),
			etcdserverpb.Request{
				Id:     1234,
				Sorted: true,
				Path:   "/foo",
			},
		},
		{
			// wait specified
			mustNewRequest(t, "foo?wait=true"),
			etcdserverpb.Request{
				Id:   1234,
				Wait: true,
				Path: "/foo",
			},
		},
		{
			// prevExists should be non-null if specified
			mustNewRequest(t, "foo?prevExists=true"),
			etcdserverpb.Request{
				Id:         1234,
				PrevExists: boolp(true),
				Path:       "/foo",
			},
		},
		{
			// prevExists should be non-null if specified
			mustNewRequest(t, "foo?prevExists=false"),
			etcdserverpb.Request{
				Id:         1234,
				PrevExists: boolp(false),
				Path:       "/foo",
			},
		},
	}

	for i, tt := range tests {
		got, err := parseRequest(tt.in, 1234)
		if err != nil {
			t.Errorf("#%d: err = %v, want %v", i, err, nil)
		}
		if !reflect.DeepEqual(got, tt.w) {
			t.Errorf("#%d: bad request: got %#v, want %#v", i, got, tt.w)
		}
	}
}

// eventingWatcher immediately returns a simple event of the given action on its channel
type eventingWatcher struct {
	action string
}

func (w *eventingWatcher) EventChan() chan *store.Event {
	ch := make(chan *store.Event)
	go func() {
		ch <- &store.Event{
			Action: w.action,
			Node:   &store.NodeExtern{},
		}
	}()
	return ch
}

func (w *eventingWatcher) Remove() {}

func TestWriteInternalError(t *testing.T) {
	// nil error should not panic
	rw := httptest.NewRecorder()
	writeInternalError(rw, nil)
	h := rw.Header()
	if len(h) > 0 {
		t.Fatalf("unexpected non-empty headers: %#v", h)
	}
	b := rw.Body.String()
	if len(b) > 0 {
		t.Fatalf("unexpected non-empty body: %q", b)
	}

	tests := []struct {
		err  error
		code int
		idx  string
	}{
		{
			etcdErr.NewError(etcdErr.EcodeKeyNotFound, "/foo/bar", 123),
			http.StatusNotFound,
			"123",
		},
		{
			etcdErr.NewError(etcdErr.EcodeTestFailed, "/foo/bar", 456),
			http.StatusPreconditionFailed,
			"456",
		},
		{
			err:  errors.New("something went wrong"),
			code: http.StatusInternalServerError,
		},
	}

	for i, tt := range tests {
		rw := httptest.NewRecorder()
		writeInternalError(rw, tt.err)
		if code := rw.Code; code != tt.code {
			t.Errorf("#%d: got %d, want %d", i, code, tt.code)
		}
		if idx := rw.Header().Get("X-Etcd-Index"); idx != tt.idx {
			t.Errorf("#%d: got %q, want %q", i, idx, tt.idx)
		}
	}
}

func TestWriteEvent(t *testing.T) {
	// nil event should not panic
	rw := httptest.NewRecorder()
	writeEvent(rw, nil)
	h := rw.Header()
	if len(h) > 0 {
		t.Fatalf("unexpected non-empty headers: %#v", h)
	}
	b := rw.Body.String()
	if len(b) > 0 {
		t.Fatalf("unexpected non-empty body: %q", b)
	}

	tests := []struct {
		ev   *store.Event
		idx  string
		code int
		err  error
	}{
		// standard case, standard 200 response
		{
			&store.Event{
				Action:   store.Get,
				Node:     &store.NodeExtern{},
				PrevNode: &store.NodeExtern{},
			},
			"0",
			http.StatusOK,
			nil,
		},
		// check new nodes return StatusCreated
		{
			&store.Event{
				Action:   store.Create,
				Node:     &store.NodeExtern{},
				PrevNode: &store.NodeExtern{},
			},
			"0",
			http.StatusCreated,
			nil,
		},
	}

	for i, tt := range tests {
		rw := httptest.NewRecorder()
		writeEvent(rw, tt.ev)
		if gct := rw.Header().Get("Content-Type"); gct != "application/json" {
			t.Errorf("case %d: bad Content-Type: got %q, want application/json", i, gct)
		}
		if gei := rw.Header().Get("X-Etcd-Index"); gei != tt.idx {
			t.Errorf("case %d: bad X-Etcd-Index header: got %s, want %s", i, gei, tt.idx)
		}
		if rw.Code != tt.code {
			t.Errorf("case %d: bad response code: got %d, want %v", i, rw.Code, tt.code)
		}

	}
}

type dummyWatcher struct {
	echan chan *store.Event
}

func (w *dummyWatcher) EventChan() chan *store.Event {
	return w.echan
}
func (w *dummyWatcher) Remove() {}

type dummyResponseWriter struct {
	cnchan chan bool
	http.ResponseWriter
}

func (rw *dummyResponseWriter) CloseNotify() <-chan bool {
	return rw.cnchan
}

func TestWaitForEventChan(t *testing.T) {
	ctx := context.Background()
	ec := make(chan *store.Event)
	dw := &dummyWatcher{
		echan: ec,
	}
	w := httptest.NewRecorder()
	var wg sync.WaitGroup
	var ev *store.Event
	var err error
	wg.Add(1)
	go func() {
		ev, err = waitForEvent(ctx, w, dw)
		wg.Done()
	}()
	ec <- &store.Event{
		Action: store.Get,
		Node: &store.NodeExtern{
			Key:           "/foo/bar",
			ModifiedIndex: 12345,
		},
	}
	wg.Wait()
	want := &store.Event{
		Action: store.Get,
		Node: &store.NodeExtern{
			Key:           "/foo/bar",
			ModifiedIndex: 12345,
		},
	}
	if !reflect.DeepEqual(ev, want) {
		t.Fatalf("bad event: got %#v, want %#v", ev, want)
	}
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWaitForEventCloseNotify(t *testing.T) {
	ctx := context.Background()
	dw := &dummyWatcher{}
	cnchan := make(chan bool)
	w := &dummyResponseWriter{
		cnchan: cnchan,
	}
	var wg sync.WaitGroup
	var ev *store.Event
	var err error
	wg.Add(1)
	go func() {
		ev, err = waitForEvent(ctx, w, dw)
		wg.Done()
	}()
	close(cnchan)
	wg.Wait()
	if ev != nil {
		t.Fatalf("non-nil Event returned with CloseNotifier: %v", ev)
	}
	if err == nil {
		t.Fatalf("nil err returned with CloseNotifier!")
	}
}

func TestWaitForEventCancelledContext(t *testing.T) {
	cctx, cancel := context.WithCancel(context.Background())
	dw := &dummyWatcher{}
	w := httptest.NewRecorder()
	var wg sync.WaitGroup
	var ev *store.Event
	var err error
	wg.Add(1)
	go func() {
		ev, err = waitForEvent(cctx, w, dw)
		wg.Done()
	}()
	cancel()
	wg.Wait()
	if ev != nil {
		t.Fatalf("non-nil Event returned with cancelled context: %v", ev)
	}
	if err == nil {
		t.Fatalf("nil err returned with cancelled context!")
	}
}

func TestV2MachinesEndpoint(t *testing.T) {
	tests := []struct {
		method string
		wcode  int
	}{
		{"GET", http.StatusOK},
		{"HEAD", http.StatusOK},
		{"POST", http.StatusMethodNotAllowed},
	}

	h := Handler{Peers: Peers{}}
	s := httptest.NewServer(h)
	defer s.Close()

	for _, tt := range tests {
		req, err := http.NewRequest(tt.method, s.URL+machinesPrefix, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}

		if resp.StatusCode != tt.wcode {
			t.Errorf("StatusCode = %d, expected %d", resp.StatusCode, tt.wcode)
		}
	}
}

func TestServeMachines(t *testing.T) {
	peers := Peers{}
	peers.Set("0xBEEF0=localhost:8080&0xBEEF1=localhost:8081&0xBEEF2=localhost:8082")
	h := Handler{Peers: peers}

	writer := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	h.serveMachines(writer, req)
	w := "http://localhost:8080, http://localhost:8081, http://localhost:8082"
	if g := writer.Body.String(); g != w {
		t.Errorf("body = %s, want %s", g, w)
	}
	if writer.Code != http.StatusOK {
		t.Errorf("header = %d, want %d", writer.Code, http.StatusOK)
	}
}
