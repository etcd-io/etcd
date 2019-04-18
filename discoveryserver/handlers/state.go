package handlers

import (
	"net/url"

	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/clientv3/concurrency"
	"go.etcd.io/etcd/etcdserver/api/v2store"
)

// State is the discovery server configuration
// state shared between handlers.
type State struct {
	etcdHost string
	etcdCURL *url.URL
	discHost string
	session  *concurrency.Session
	client   *clientv3.Client
	v2       v2store.Store
}
