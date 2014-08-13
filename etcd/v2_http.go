/*
Copyright 2014 CoreOS Inc.

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
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	etcdErr "github.com/coreos/etcd/error"
)

func (p *participant) serveValue(w http.ResponseWriter, r *http.Request) error {
	switch r.Method {
	case "GET":
		return p.GetHandler(w, r)
	case "HEAD":
		w = &HEADResponseWriter{w}
		return p.GetHandler(w, r)
	case "PUT":
		return p.PutHandler(w, r)
	case "POST":
		return p.PostHandler(w, r)
	case "DELETE":
		return p.DeleteHandler(w, r)
	}
	return allow(w, "GET", "PUT", "POST", "DELETE", "HEAD")
}

func (p *participant) serveMachines(w http.ResponseWriter, r *http.Request) error {
	if r.Method != "GET" {
		return allow(w, "GET")
	}
	v, err := p.Store.Get(v2machineKVPrefix, false, false)
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

func (p *participant) serveLeader(w http.ResponseWriter, r *http.Request) error {
	if r.Method != "GET" {
		return allow(w, "GET")
	}
	if p, ok := p.peerHub.peers[p.node.Leader()]; ok {
		w.Write([]byte(p.url))
		return nil
	}
	return fmt.Errorf("no leader")
}

func (p *participant) serveLeaderStats(w http.ResponseWriter, req *http.Request) error {
	if !p.node.IsLeader() {
		return p.redirect(w, req, p.node.Leader())
	}
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(p.peerHub.followersStats)
}

func (p *participant) serveStoreStats(w http.ResponseWriter, req *http.Request) error {
	w.Header().Set("Content-Type", "application/json")
	w.Write(p.Store.JsonStats())
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

	log.Printf("HTTP.serve: req=%s err=\"%v\"\n", r.URL, err)
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

func (p *participant) redirect(w http.ResponseWriter, r *http.Request, id int64) error {
	e, err := p.Store.Get(fmt.Sprintf("%v/%d", v2machineKVPrefix, p.node.Leader()), false, false)
	if err != nil {
		return fmt.Errorf("redirect cannot find node %d", id)
	}

	m, err := url.ParseQuery(*e.Node.Value)
	if err != nil {
		return fmt.Errorf("failed to parse node entry: %s", *e.Node.Value)
	}

	redirectAddr, err := buildRedirectURL(m["etcd"][0], r.URL)
	if err != nil {
		return err
	}

	http.Redirect(w, r, redirectAddr, http.StatusTemporaryRedirect)
	return nil
}

func buildRedirectURL(redirectAddr string, originalURL *url.URL) (string, error) {
	redirectURL, err := url.Parse(redirectAddr)
	if err != nil {
		return "", fmt.Errorf("cannot parse url: %v", err)
	}

	redirectURL.Path = originalURL.Path
	redirectURL.RawQuery = originalURL.RawQuery
	redirectURL.Fragment = originalURL.Fragment
	return redirectURL.String(), nil
}
