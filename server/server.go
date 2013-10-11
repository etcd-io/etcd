package server

import (
	"github.com/gorilla/mux"
	"net/http"
)

// The Server provides an HTTP interface to the underlying data store.
type Server struct {
	http.Server
	raftServer  *raftServer
	name        string
	url         string
	tlsConf     *TLSConfig
	tlsInfo     *TLSInfo
	corsOrigins map[string]bool
}

// Creates a new Server.
func New(name string, urlStr string, listenHost string, tlsConf *TLSConfig, tlsInfo *TLSInfo, raftServer *raftServer) *Server {
	s := &etcdServer{
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

	// TODO: Move to main.go.
	// Install the routes for each version of the API.
	// v1.Install(s)
	// v2.Install(s)

	return s
}

// Adds a server handler to the router.
func (s *Server) HandleFunc(path string, f func(http.ResponseWriter, *http.Request, *server.Server) error) *mux.Route {
	r := s.Handler.(*mux.Router)

	// Wrap the standard HandleFunc interface to pass in the server reference.
	return r.HandleFunc(path, func(w http.ResponseWriter, req *http.Request) {
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
func (s *Server) ListenAndServe() {
	infof("etcd server [name %s, listen on %s, advertised url %s]", s.name, s.Server.Addr, s.url)

	if s.tlsConf.Scheme == "http" {
		fatal(s.Server.ListenAndServe())
	} else {
		fatal(s.Server.ListenAndServeTLS(s.tlsInfo.CertFile, s.tlsInfo.KeyFile))
	}
}

// Sets a comma-delimited list of origins that are allowed.
func (s *Server) AllowOrigins(origins string) error {
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
	s.origins = m

	return nil
}

// Determines whether the server will allow a given CORS origin.
func (s *Server) OriginAllowed(origin string) {
	return s.origins["*"] || s.origins[origin]
}
