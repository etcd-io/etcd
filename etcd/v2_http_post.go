package etcd

import (
	"log"
	"net/http"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/store"
)

func (p *participant) PostHandler(w http.ResponseWriter, req *http.Request) error {
	if !p.node.IsLeader() {
		return p.redirect(w, req, p.node.Leader())
	}

	key := req.URL.Path[len("/v2/keys"):]

	value := req.FormValue("value")
	dir := (req.FormValue("dir") == "true")
	expireTime, err := store.TTL(req.FormValue("ttl"))
	if err != nil {
		return etcdErr.NewError(etcdErr.EcodeTTLNaN, "Create", p.Store.Index())
	}

	ret, err := p.Create(key, dir, value, expireTime, true)
	if err == nil {
		p.handleRet(w, ret)
		return nil
	}
	log.Println("unique:", err)
	return err
}
