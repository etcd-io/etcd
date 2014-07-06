package etcd

import (
	"log"
	"net/http"
	"strconv"

	etcdErr "github.com/coreos/etcd/error"
)

func (s *Server) DeleteHandler(w http.ResponseWriter, req *http.Request) error {
	if !s.node.IsLeader() {
		return s.redirect(w, req, s.node.Leader())
	}

	key := req.URL.Path[len("/v2/keys"):]

	recursive := (req.FormValue("recursive") == "true")
	dir := (req.FormValue("dir") == "true")

	req.ParseForm()
	_, valueOk := req.Form["prevValue"]
	_, indexOk := req.Form["prevIndex"]

	if !valueOk && !indexOk {
		return s.serveDelete(w, req, key, dir, recursive)
	}

	var err error
	prevIndex := uint64(0)
	prevValue := req.Form.Get("prevValue")

	if indexOk {
		prevIndexStr := req.Form.Get("prevIndex")
		prevIndex, err = strconv.ParseUint(prevIndexStr, 10, 64)

		// bad previous index
		if err != nil {
			return etcdErr.NewError(etcdErr.EcodeIndexNaN, "CompareAndDelete", s.Store.Index())
		}
	}

	if valueOk {
		if prevValue == "" {
			return etcdErr.NewError(etcdErr.EcodePrevValueRequired, "CompareAndDelete", s.Store.Index())
		}
	}
	return s.serveCAD(w, req, key, prevValue, prevIndex)
}

func (s *Server) serveDelete(w http.ResponseWriter, req *http.Request, key string, dir, recursive bool) error {
	ret, err := s.Delete(key, dir, recursive)
	if err == nil {
		s.handleRet(w, ret)
		return nil
	}
	log.Println("delete:", err)
	return err
}

func (s *Server) serveCAD(w http.ResponseWriter, req *http.Request, key string, prevValue string, prevIndex uint64) error {
	ret, err := s.CAD(key, prevValue, prevIndex)
	if err == nil {
		s.handleRet(w, ret)
		return nil
	}
	log.Println("cad:", err)
	return err
}
