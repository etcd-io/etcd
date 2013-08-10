package main

import (
	"net/url"
	"path"
)

// we map node name to url
type nodeInfo struct {
	raftURL string
	etcdURL string
}

var namesMap = make(map[string]*nodeInfo)

// nameToEtcdURL maps node name to its etcd http address
func nameToEtcdURL(name string) (string, bool) {

	if info, ok := namesMap[name]; ok {
		// first try to read from the map
		return info.etcdURL, true
	} else {
		// if fails, try to recover from etcd storage
		key := path.Join("/_etcd/machines", name)

		resps, err := etcdStore.RawGet(key)

		if err != nil {
			return "", false
		}

		m, err := url.ParseQuery(resps[0].Value)

		if err != nil {
			panic("Failed to parse machines entry")
		}

		etcdURL := m["etcd"][0]

		return etcdURL, true

	}
}

// nameToRaftURL maps node name to its raft http address
func nameToRaftURL(name string) (string, bool) {
	if info, ok := namesMap[name]; ok {
		// first try to read from the map
		return info.raftURL, true

	} else {
		// if fails, try to recover from etcd storage
		key := path.Join("/_etcd/machines", name)

		resps, err := etcdStore.RawGet(key)

		if err != nil {
			return "", false
		}

		m, err := url.ParseQuery(resps[0].Value)

		if err != nil {
			panic("Failed to parse machines entry")
		}

		raftURL := m["raft"][0]

		return raftURL, true

	}
}

// addNameToURL add a name that maps to raftURL and etcdURL
func addNameToURL(name string, raftURL string, etcdURL string) {
	namesMap[name] = &nodeInfo{
		raftURL: raftURL,
		etcdURL: etcdURL,
	}
}
