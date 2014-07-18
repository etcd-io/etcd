package etcd

import (
	"log"
	"net/http"
	"strconv"

	etcdErr "github.com/coreos/etcd/error"
)

func (p *participant) DeleteHandler(w http.ResponseWriter, req *http.Request) error {
	if !p.node.IsLeader() {
		return p.redirect(w, req, p.node.Leader())
	}

	key := req.URL.Path[len("/v2/keys"):]

	recursive := (req.FormValue("recursive") == "true")
	dir := (req.FormValue("dir") == "true")

	req.ParseForm()
	_, valueOk := req.Form["prevValue"]
	_, indexOk := req.Form["prevIndex"]

	if !valueOk && !indexOk {
		return p.serveDelete(w, req, key, dir, recursive)
	}

	var err error
	prevIndex := uint64(0)
	prevValue := req.Form.Get("prevValue")

	if indexOk {
		prevIndexStr := req.Form.Get("prevIndex")
		prevIndex, err = strconv.ParseUint(prevIndexStr, 10, 64)

		// bad previous index
		if err != nil {
			return etcdErr.NewError(etcdErr.EcodeIndexNaN, "CompareAndDelete", p.Store.Index())
		}
	}

	if valueOk {
		if prevValue == "" {
			return etcdErr.NewError(etcdErr.EcodePrevValueRequired, "CompareAndDelete", p.Store.Index())
		}
	}
	return p.serveCAD(w, req, key, prevValue, prevIndex)
}

func (p *participant) serveDelete(w http.ResponseWriter, req *http.Request, key string, dir, recursive bool) error {
	ret, err := p.Delete(key, dir, recursive)
	if err == nil {
		p.handleRet(w, ret)
		return nil
	}
	log.Println("delete:", err)
	return err
}

func (p *participant) serveCAD(w http.ResponseWriter, req *http.Request, key string, prevValue string, prevIndex uint64) error {
	ret, err := p.CAD(key, prevValue, prevIndex)
	if err == nil {
		p.handleRet(w, ret)
		return nil
	}
	log.Println("cad:", err)
	return err
}
