package etcd

import (
	"log"
	"net/http"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/store"
)

func (s *Server) PostHandler(w http.ResponseWriter, req *http.Request) error {
	if !s.node.IsLeader() {
		return s.redirect(w, req, s.node.Leader())
	}

	key := req.URL.Path[len("/v2/keys"):]

	value := req.FormValue("value")
	dir := (req.FormValue("dir") == "true")
	expireTime, err := store.TTL(req.FormValue("ttl"))
	if err != nil {
		return etcdErr.NewError(etcdErr.EcodeTTLNaN, "Create", s.Store.Index())
	}

	ret, err := s.Create(key, dir, value, expireTime, true)
	if err == nil {
		s.handleRet(w, ret)
		return nil
	}
	log.Println("unique:", err)
	return err
}
