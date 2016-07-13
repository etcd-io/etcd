package cache

import (
	"errors"
	"net/http"
	"sync"

	"github.com/coreos/etcd/client"
	"github.com/coreos/etcd/etcdserver"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/pkg/capnslog"
	"github.com/jonboulle/clockwork"

	"github.com/coreos/etcd/store"
	"golang.org/x/net/context"
)

const defaultV2KeysPrefix = "/v2/keys"

const plog = capnslog.NewPackageLogger("github.com/coreos/etcd/proxy/httpproxy/cache", "v2proxy")

type CacheServer struct {
	client client.KeysAPI
	cache  *threadSafeMap
	usable sync.RWMutex
	//TODO:add a watchhub here, or in cache
}

func (s *CacheServer) Do(ctx context.Context, r pb.Request) (etcdserver.Response, error) {
	if r.Method == "GET" && r.Quorum {
		r.Method = "QGET"
	}

	switch r.Method {
	case "POST":
		return s.client.Create(ctx, r.Path, r.Val)
	case "PUT":
		return s.client.Set(ctx, r.Path, r.Val)
	case "DELETE":
		return s.client.Delete(ctx, r.Path, r.Dir, r.Recursive)
	case "QGET":
		return s.client.Get(ctx, r.Path, &client.GetOptions{Quorum: true, Sort: r.Sorted})
	case "GET":
		if r.Wait {
			//TODO:implement watch
			panic("not implemented")
		}
		s.usable.RLock()
		s.usable.RUnlock()
		event, err := s.cache.Get(ctx, r.Path, r.Recursive, r.Sorted)
		return etcdserver.NewResponse(event, err)
	case "HEAD":
		//TODO:implement haed by forwarding
		panic("not implemented")
	}
	return etcdserver.Response{}, etcdserver.ErrUnknownMethod
}

func NewCacheServer(client client.KeysAPI) *CacheServer {
	s := &CacheServer{
		client: client,
		cache:  NewStore(),
	}

	s.usable.Unlock()

	// update local cache by watch
	go func() {
		ctx := context.TODO()
		for {
			resp, err := s.client.Get(ctx, defaultV2KeysPrefix+"/", &client.GetOptions{Recursive: true})
			if err != nil {
				continue
			}
			//TODO: close all watchers when invalidate the whole cache
			s.cache.InvalidateAll()
			s.AddNode(resp.Node)
			s.usable.Unlock()

			idx := resp.Index
			func() {
				defer s.usable.Lock()
				for {
					watcher := s.client.Watcher(ctx, defaultV2KeysPrefix+"/", &client.WatcherOptions{AfterIndex: idx, Recursive: true})
					resp, err := watcher.Next(ctx)
					if err != nil {
						break
					}
					s.UpdateCache(resp)
				}
			}()
		}
	}()

	return s
}

func (s *CacheServer) AddNode(node client.Node) {
	if node.Dir && len(node.Nodes) != 0 {
		for node := range node.Nodes {
			s.AddNode(node)
		}
	}
	s.cache.Add(node.Key, node.Value)
}

func (s *CacheServer) DeleteNode(node client.Node) {
	if node.Dir && len(node.Nodes) != 0 {
		for node := range node.Nodes {
			s.DeleteNode(node)
		}
	}
	s.cache.Delete(node.Key)
}

func (s *CacheServer) UpdateCache(resp client.Response) {
	switch resp.Action {
	case store.Create:
		s.AddNode(resp.Node)
	case store.Update:
		s.AddNode(resp.Node)
	case store.Delete:
		s.DeleteNode(resp.Node.Key)
	case store.CompareAndDelete:
		s.DeleteNode(resp.Node.Key)
	case store.CompareAndSwap:
		s.AddNode(resp.Node.Key)
	case store.Expire:
		s.DeleteNode(resp.Node)
	default:
		plog.Errorf("unexpected event type: %v", resp.Action)
	}
}

type KeysHandler struct {
	Server *CacheServer
}

func (h *KeysHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r.Method, "HEAD", "GET", "PUT", "POST", "DELETE") {
		return
	}

	// TODO: write "X-Etcd-Cluster-ID", "X-Raft-Term", and "X-Raft-Index" header
	//w.Header().Set("X-Etcd-Cluster-ID", h.cluster.ID().String())

	ctx := context.TODO()
	clock := clockwork.NewRealClock()
	rr, err := parseKeyRequest(r, clock)
	if err != nil {
		writeKeyError(w, err)
		return
	}

	resp, err := h.Server.Do(ctx, rr)
	if err != nil {
		err = trimErrorPrefix(err, etcdserver.StoreKeysPrefix)
		writeKeyError(w, err)
		return
	}
	switch {
	case resp.Event != nil:
		//if err := writeKeyEvent(w, resp.Event, h.timer); err != nil {
		if err := writeKeyEvent(w, resp.Event); err != nil {
			// Should never be reached
			plog.Errorf("error writing event (%v)", err)
		}
	case resp.Watcher != nil:
		//TODO:implement watch
		panic("not implemented")
	default:
		writeKeyError(w, errors.New("received response with no Event/Watcher!"))
	}
}
