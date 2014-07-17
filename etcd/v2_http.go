package etcd

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	etcdErr "github.com/coreos/etcd/error"
)

func (s *Server) serveValue(w http.ResponseWriter, r *http.Request) error {
	switch r.Method {
	case "GET":
		return s.GetHandler(w, r)
	case "HEAD":
		w = &HEADResponseWriter{w}
		return s.GetHandler(w, r)
	case "PUT":
		return s.PutHandler(w, r)
	case "POST":
		return s.PostHandler(w, r)
	case "DELETE":
		return s.DeleteHandler(w, r)
	}
	return allow(w, "GET", "PUT", "POST", "DELETE", "HEAD")
}

func (s *Server) serveMachines(w http.ResponseWriter, r *http.Request) error {
	if r.Method != "GET" {
		return allow(w, "GET")
	}
	v, err := s.Store.Get(v2machineKVPrefix, false, false)
	if err != nil {
		panic(err)
	}
	ns := make([]string, len(v.Node.Nodes))
	for i, n := range v.Node.Nodes {
		m, err := url.ParseQuery(*n.Value)
		if err != nil {
			continue
		}
		ns[i] = m["etcd"][0]
	}
	w.Write([]byte(strings.Join(ns, ",")))
	return nil
}

func (s *Server) serveLeader(w http.ResponseWriter, r *http.Request) error {
	if r.Method != "GET" {
		return allow(w, "GET")
	}
	if p, ok := s.peerHub.peers[s.node.Leader()]; ok {
		w.Write([]byte(p.url))
		return nil
	}
	return fmt.Errorf("no leader")
}

func (s *Server) serveStoreStats(w http.ResponseWriter, req *http.Request) error {
	w.Header().Set("Content-Type", "application/json")
	w.Write(s.Store.JsonStats())
	return nil
}

type handlerErr func(w http.ResponseWriter, r *http.Request) error

func (eh handlerErr) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	err := eh(w, r)
	if err == nil {
		return
	}

	if r.Method == "HEAD" {
		w = &HEADResponseWriter{w}
	}

	if etcdErr, ok := err.(*etcdErr.Error); ok {
		w.Header().Set("Content-Type", "application/json")
		etcdErr.Write(w)
		return
	}

	log.Println("http error", err)
	http.Error(w, "Internal Server Error", http.StatusInternalServerError)
}

func allow(w http.ResponseWriter, m ...string) error {
	w.Header().Set("Allow", strings.Join(m, ","))
	return nil
}

type HEADResponseWriter struct {
	http.ResponseWriter
}

func (w *HEADResponseWriter) Write([]byte) (int, error) {
	return 0, nil
}

func (s *Server) redirect(w http.ResponseWriter, r *http.Request, id int64) error {
	e, err := s.Store.Get(fmt.Sprintf("%v/%d", v2machineKVPrefix, s.node.Leader()), false, false)
	if err != nil {
		log.Println("redirect cannot find node", id)
		return fmt.Errorf("redirect cannot find node %d", id)
	}

	m, err := url.ParseQuery(*e.Node.Value)
	if err != nil {
		return fmt.Errorf("failed to parse node entry: %s", *e.Node.Value)
	}

	redirectAddr, err := s.buildRedirectURL(m["etcd"][0], r.URL)
	if err != nil {
		log.Println("redirect cannot build new url:", err)
		return err
	}

	http.Redirect(w, r, redirectAddr, http.StatusTemporaryRedirect)
	return nil
}

func (s *Server) buildRedirectURL(redirectAddr string, originalURL *url.URL) (string, error) {
	redirectURL, err := url.Parse(redirectAddr)
	if err != nil {
		return "", fmt.Errorf("redirect cannot parse url: %v", err)
	}

	redirectURL.Path = originalURL.Path
	redirectURL.RawQuery = originalURL.RawQuery
	redirectURL.Fragment = originalURL.Fragment
	return redirectURL.String(), nil
}
