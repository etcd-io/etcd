/*
Copyright 2013 CoreOS Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
