package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/pprof"
	"strings"
	"time"

	"github.com/coreos/etcd/third_party/github.com/goraft/raft"
	"github.com/coreos/etcd/third_party/github.com/gorilla/mux"

	etcdErr "github.com/coreos/etcd/error"
	ehttp "github.com/coreos/etcd/http"
	"github.com/coreos/etcd/log"
	"github.com/coreos/etcd/metrics"
	"github.com/coreos/etcd/mod"
	uhttp "github.com/coreos/etcd/pkg/http"
	"github.com/coreos/etcd/server/v1"
	"github.com/coreos/etcd/server/v2"
	"github.com/coreos/etcd/store"
	_ "github.com/coreos/etcd/store/v2"
)

// This is the default implementation of the Server interface.
type Server struct {
	Name       string
	url        string
	handler    http.Handler
	peerServer *PeerServer
	registry   *Registry
	store      store.Store
	metrics    *metrics.Bucket

	trace bool
}

// Creates a new Server.
func New(name, url string, peerServer *PeerServer, registry *Registry, store store.Store, mb *metrics.Bucket) *Server {
	s := &Server{
		Name:       name,
		url:        url,
		store:      store,
		registry:   registry,
		peerServer: peerServer,
		metrics:    mb,
	}

	return s
}

func (s *Server) EnableTracing() {
	s.trace = true
}

// The current state of the server in the cluster.
func (s *Server) State() string {
	return s.peerServer.RaftServer().State()
}

// The node name of the leader in the cluster.
func (s *Server) Leader() string {
	return s.peerServer.RaftServer().Leader()
}

// The current Raft committed index.
func (s *Server) CommitIndex() uint64 {
	return s.peerServer.RaftServer().CommitIndex()
}

// The current Raft term.
func (s *Server) Term() uint64 {
	return s.peerServer.RaftServer().Term()
}

// The server URL.
func (s *Server) URL() string {
	return s.url
}

// PeerHost retrieves the host part of Peer URL for a given node name.
func (s *Server) PeerHost(name string) (string, bool) {
	return s.registry.PeerHost(name)
}

// Retrives the Peer URL for a given node name.
func (s *Server) PeerURL(name string) (string, bool) {
	return s.registry.PeerURL(name)
}

// ClientURL retrieves the Client URL for a given node name.
func (s *Server) ClientURL(name string) (string, bool) {
	return s.registry.ClientURL(name)
}

// Returns a reference to the Store.
func (s *Server) Store() store.Store {
	return s.store
}

func (s *Server) SetRegistry(registry *Registry) {
	s.registry = registry
}

func (s *Server) SetStore(store store.Store) {
	s.store = store
}

func (s *Server) installV1(r *mux.Router) {
	s.handleFuncV1(r, "/v1/keys/{key:.*}", v1.GetKeyHandler).Methods("GET", "HEAD")
	s.handleFuncV1(r, "/v1/keys/{key:.*}", v1.SetKeyHandler).Methods("POST", "PUT")
	s.handleFuncV1(r, "/v1/keys/{key:.*}", v1.DeleteKeyHandler).Methods("DELETE")
	s.handleFuncV1(r, "/v1/watch/{key:.*}", v1.WatchKeyHandler).Methods("GET", "HEAD", "POST")
	s.handleFunc(r, "/v1/leader", s.GetLeaderHandler).Methods("GET", "HEAD")
	s.handleFunc(r, "/v1/machines", s.GetPeersHandler).Methods("GET", "HEAD")
	s.handleFunc(r, "/v1/peers", s.GetPeersHandler).Methods("GET", "HEAD")
	s.handleFunc(r, "/v1/stats/self", s.GetStatsHandler).Methods("GET", "HEAD")
	s.handleFunc(r, "/v1/stats/leader", s.GetLeaderStatsHandler).Methods("GET", "HEAD")
	s.handleFunc(r, "/v1/stats/store", s.GetStoreStatsHandler).Methods("GET", "HEAD")
}

func (s *Server) installV2(r *mux.Router) {
	r2 := mux.NewRouter()
	r.PathPrefix("/v2").Handler(ehttp.NewLowerQueryParamsHandler(r2))

	s.handleFuncV2(r2, "/v2/keys/{key:.*}", v2.GetHandler).Methods("GET", "HEAD")
	s.handleFuncV2(r2, "/v2/keys/{key:.*}", v2.PostHandler).Methods("POST")
	s.handleFuncV2(r2, "/v2/keys/{key:.*}", v2.PutHandler).Methods("PUT")
	s.handleFuncV2(r2, "/v2/keys/{key:.*}", v2.DeleteHandler).Methods("DELETE")
	s.handleFunc(r2, "/v2/leader", s.GetLeaderHandler).Methods("GET", "HEAD")
	s.handleFunc(r2, "/v2/machines", s.GetPeersHandler).Methods("GET", "HEAD")
	s.handleFunc(r2, "/v2/peers", s.GetPeersHandler).Methods("GET", "HEAD")
	s.handleFunc(r2, "/v2/stats/self", s.GetStatsHandler).Methods("GET", "HEAD")
	s.handleFunc(r2, "/v2/stats/leader", s.GetLeaderStatsHandler).Methods("GET", "HEAD")
	s.handleFunc(r2, "/v2/stats/store", s.GetStoreStatsHandler).Methods("GET", "HEAD")
	s.handleFunc(r2, "/v2/speedTest", s.SpeedTestHandler).Methods("GET", "HEAD")
}

func (s *Server) installMod(r *mux.Router) {
	r.PathPrefix("/mod").Handler(http.StripPrefix("/mod", mod.HttpHandler(s.URL())))
}

func (s *Server) installDebug(r *mux.Router) {
	s.handleFunc(r, "/debug/metrics", s.GetMetricsHandler).Methods("GET", "HEAD")
	r.HandleFunc("/debug/pprof", pprof.Index)
	r.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	r.HandleFunc("/debug/pprof/profile", pprof.Profile)
	r.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	r.HandleFunc("/debug/pprof/{name}", pprof.Index)
}

// Adds a v1 server handler to the router.
func (s *Server) handleFuncV1(r *mux.Router, path string, f func(http.ResponseWriter, *http.Request, v1.Server) error) *mux.Route {
	return s.handleFunc(r, path, func(w http.ResponseWriter, req *http.Request) error {
		return f(w, req, s)
	})
}

// Adds a v2 server handler to the router.
func (s *Server) handleFuncV2(r *mux.Router, path string, f func(http.ResponseWriter, *http.Request, v2.Server) error) *mux.Route {
	return s.handleFunc(r, path, func(w http.ResponseWriter, req *http.Request) error {
		return f(w, req, s)
	})
}

type HEADResponseWriter struct {
	http.ResponseWriter
}

func (w *HEADResponseWriter) Write([]byte) (int, error) {
	return 0, nil
}

// Adds a server handler to the router.
func (s *Server) handleFunc(r *mux.Router, path string, f func(http.ResponseWriter, *http.Request) error) *mux.Route {

	// Wrap the standard HandleFunc interface to pass in the server reference.
	return r.HandleFunc(path, func(w http.ResponseWriter, req *http.Request) {
		if req.Method == "HEAD" {
			w = &HEADResponseWriter{w}
		}

		// Log request.
		log.Debugf("[recv] %s %s %s [%s]", req.Method, s.URL(), req.URL.Path, req.RemoteAddr)

		// Execute handler function and return error if necessary.
		if err := f(w, req); err != nil {
			if etcdErr, ok := err.(*etcdErr.Error); ok {
				log.Debug("Return error: ", (*etcdErr).Error())
				w.Header().Set("Content-Type", "application/json")
				etcdErr.Write(w)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}
	})
}

func (s *Server) HTTPHandler() http.Handler {
	router := mux.NewRouter()

	// Install the routes.
	s.handleFunc(router, "/version", s.GetVersionHandler).Methods("GET")
	s.installV1(router)
	s.installV2(router)
	// Mod is deprecated temporariy due to its unstable state.
	// It would be added back later.
	// s.installMod(router)

	if s.trace {
		s.installDebug(router)
	}

	return router
}

// Dispatch command to the current leader
func (s *Server) Dispatch(c raft.Command, w http.ResponseWriter, req *http.Request) error {
	ps := s.peerServer
	if ps.raftServer.State() == raft.Leader {
		result, err := ps.raftServer.Do(c)
		if err != nil {
			return err
		}

		if result == nil {
			return etcdErr.NewError(300, "Empty result from raft", s.Store().Index())
		}

		// response for raft related commands[join/remove]
		if b, ok := result.([]byte); ok {
			w.WriteHeader(http.StatusOK)
			w.Write(b)
			return nil
		}

		var b []byte
		if strings.HasPrefix(req.URL.Path, "/v1") {
			b, _ = json.Marshal(result.(*store.Event).Response(0))
			w.WriteHeader(http.StatusOK)
		} else {
			e, _ := result.(*store.Event)
			b, _ = json.Marshal(e)

			w.Header().Set("Content-Type", "application/json")
			// etcd index should be the same as the event index
			// which is also the last modified index of the node
			w.Header().Add("X-Etcd-Index", fmt.Sprint(e.Index()))
			w.Header().Add("X-Raft-Index", fmt.Sprint(s.CommitIndex()))
			w.Header().Add("X-Raft-Term", fmt.Sprint(s.Term()))

			if e.IsCreated() {
				w.WriteHeader(http.StatusCreated)
			} else {
				w.WriteHeader(http.StatusOK)
			}
		}

		w.Write(b)

		return nil

	}

	leader := ps.raftServer.Leader()
	if leader == "" {
		return etcdErr.NewError(300, "", s.Store().Index())
	}

	var url string
	switch c.(type) {
	case *JoinCommand, *RemoveCommand,
		*SetClusterConfigCommand:
		url, _ = ps.registry.PeerURL(leader)
	default:
		url, _ = ps.registry.ClientURL(leader)
	}

	uhttp.Redirect(url, w, req)

	return nil
}

// Handler to return the current version of etcd.
func (s *Server) GetVersionHandler(w http.ResponseWriter, req *http.Request) error {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "etcd %s", ReleaseVersion)
	return nil
}

// Handler to return the current leader's raft address
func (s *Server) GetLeaderHandler(w http.ResponseWriter, req *http.Request) error {
	leader := s.peerServer.RaftServer().Leader()
	if leader == "" {
		return etcdErr.NewError(etcdErr.EcodeLeaderElect, "", s.Store().Index())
	}
	w.WriteHeader(http.StatusOK)
	url, _ := s.registry.PeerURL(leader)
	w.Write([]byte(url))
	return nil
}

// Handler to return all the known peers in the current cluster.
func (s *Server) GetPeersHandler(w http.ResponseWriter, req *http.Request) error {
	peers := s.registry.ClientURLs(s.peerServer.RaftServer().Leader(), s.Name)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(strings.Join(peers, ", ")))
	return nil
}

// Retrieves stats on the Raft server.
func (s *Server) GetStatsHandler(w http.ResponseWriter, req *http.Request) error {
	w.Header().Set("Content-Type", "application/json")
	w.Write(s.peerServer.Stats())
	return nil
}

// Retrieves stats on the leader.
func (s *Server) GetLeaderStatsHandler(w http.ResponseWriter, req *http.Request) error {
	if s.peerServer.RaftServer().State() == raft.Leader {
		w.Header().Set("Content-Type", "application/json")
		w.Write(s.peerServer.PeerStats())
		return nil
	}

	leader := s.peerServer.RaftServer().Leader()
	if leader == "" {
		return etcdErr.NewError(300, "", s.Store().Index())
	}
	hostname, _ := s.registry.ClientURL(leader)
	uhttp.Redirect(hostname, w, req)
	return nil
}

// Retrieves stats on the leader.
func (s *Server) GetStoreStatsHandler(w http.ResponseWriter, req *http.Request) error {
	w.Header().Set("Content-Type", "application/json")
	w.Write(s.store.JsonStats())
	return nil
}

// Executes a speed test to evaluate the performance of update replication.
func (s *Server) SpeedTestHandler(w http.ResponseWriter, req *http.Request) error {
	count := 1000
	c := make(chan bool, count)
	for i := 0; i < count; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				c := s.Store().CommandFactory().CreateSetCommand("foo", false, "bar", time.Unix(0, 0))
				s.peerServer.RaftServer().Do(c)
			}
			c <- true
		}()
	}

	for i := 0; i < count; i++ {
		<-c
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("speed test success"))
	return nil
}

// Retrieves metrics from bucket
func (s *Server) GetMetricsHandler(w http.ResponseWriter, req *http.Request) error {
	(*s.metrics).Dump(w)
	return nil
}
