package proxy

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/coreos/etcd/log"
	"github.com/coreos/go-etcd/etcd"
)

type Proxy struct {
	client *etcd.Client
}

func New(machines []string, scheme string) *Proxy {
	log.Infof("New proxy to machines %v", machines)

	urls := make([]string, len(machines))

	for i, machine := range machines {
		u := &url.URL{Host: machine, Scheme: scheme}
		urls[i] = u.String()
	}

	etcd.OpenDebug()
	p := &Proxy{
		client: etcd.NewClient(urls),
	}
	return p
}

func (p *Proxy) Get(key string, sort, recursive bool, w http.ResponseWriter) error {
	var resp *etcd.Response
	var err error

	if recursive {
		// TODO: add return raw http response api
		// We need to get headers
		resp, err = p.client.GetAll(key, sort)
	} else {
		resp, err = p.client.Get(key, sort)
	}

	if err != nil {
		return err
	}

	b, _ := json.Marshal(resp)
	w.Write(b)

	return nil
}

func (p *Proxy) Watch(key string, sorted, recursive bool, waitIndex uint64,
	w http.ResponseWriter) {

}
