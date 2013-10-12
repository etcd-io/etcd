package server

import (
	"net/http"
	"net/url"

	"github.com/coreos/go-raft"
	"github.com/gorilla/mux"
)

// The Server provides an HTTP interface to the underlying store.
type Server interface {
	CommitIndex() uint64
	Term() uint64
	Dispatch(raft.Command, http.ResponseWriter, *http.Request)
}

// This is the default implementation of the Server interface.
type server struct {
	http.Server
	raftServer  *raft.Server
	name        string
	url         string
	tlsConf     *TLSConfig
	tlsInfo     *TLSInfo
	corsOrigins map[string]bool
}

// Creates a new Server.
func New(name string, urlStr string, listenHost string, tlsConf *TLSConfig, tlsInfo *TLSInfo, raftServer *raft.Server) *Server {
	s := &server{
		Server: http.Server{
			Handler:   mux.NewRouter(),
			TLSConfig: &tlsConf.Server,
			Addr:      listenHost,
		},
		name:       name,
		url:        urlStr,
		tlsConf:    tlsConf,
		tlsInfo:    tlsInfo,
		raftServer: raftServer,
	}

	// Install the routes for each version of the API.
	s.installV1()

	return s
}

// The current Raft committed index.
func (s *server) CommitIndex() uint64 {
	return c.raftServer.CommitIndex()
}

// The current Raft term.
func (s *server) Term() uint64 {
	return c.raftServer.Term()
}

func (s *server) installV1() {
	s.handleFunc("/v1/keys/{key:.*}", v1.GetKeyHandler).Methods("GET")
	s.handleFunc("/v1/keys/{key:.*}", v1.SetKeyHandler).Methods("POST", "PUT")
	s.handleFunc("/v1/keys/{key:.*}", v1.DeleteKeyHandler).Methods("DELETE")

	s.handleFunc("/v1/watch/{key:.*}", v1.WatchKeyHandler).Methods("GET", "POST")
}

// Adds a server handler to the router.
func (s *server) handleFunc(path string, f func(http.ResponseWriter, *http.Request, Server) error) *mux.Route {
	r := s.Handler.(*mux.Router)

	// Wrap the standard HandleFunc interface to pass in the server reference.
	return r.HandleFunc(path, func(w http.ResponseWriter, req *http.Request) {
		// Log request.
		debugf("[recv] %s %s [%s]", req.Method, s.url, req.URL.Path, req.RemoteAddr)

		// Write CORS header.
		if s.OriginAllowed("*") {
			w.Header().Add("Access-Control-Allow-Origin", "*")
		} else if s.OriginAllowed(r.Header.Get("Origin")) {
			w.Header().Add("Access-Control-Allow-Origin", r.Header.Get("Origin"))
		}

		// Execute handler function and return error if necessary.
		if err := f(w, req, s); err != nil {
			if etcdErr, ok := err.(*etcdErr.Error); ok {
				debug("Return error: ", (*etcdErr).Error())
				etcdErr.Write(w)
			} else {
				http.Error(w, e.Error(), http.StatusInternalServerError)
			}
		}
	})
}

// Start to listen and response etcd client command
func (s *server) ListenAndServe() {
	infof("etcd server [name %s, listen on %s, advertised url %s]", s.name, s.Server.Addr, s.url)

	if s.tlsConf.Scheme == "http" {
		fatal(s.Server.ListenAndServe())
	} else {
		fatal(s.Server.ListenAndServeTLS(s.tlsInfo.CertFile, s.tlsInfo.KeyFile))
	}
}

// Sets a comma-delimited list of origins that are allowed.
func (s *server) AllowOrigins(origins string) error {
	// Construct a lookup of all origins.
	m := make(map[string]bool)
	for _, v := range strings.Split(cors, ",") {
		if v != "*" {
			if _, err := url.Parse(v); err != nil {
				return fmt.Errorf("Invalid CORS origin: %s", err)
			}
		}
		m[v] = true
	}
	s.corsOrigins = m

	return nil
}

// Determines whether the server will allow a given CORS origin.
func (s *server) OriginAllowed(origin string) {
	return s.corsOrigins["*"] || s.corsOrigins[origin]
}
