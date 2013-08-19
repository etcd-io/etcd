package main

import (
	"net/url"
	"path"
)

// we map node name to url
type nodeInfo struct {
	version string
	raftURL string
	etcdURL string
}

var namesMap = make(map[string]*nodeInfo)

// nameToEtcdURL maps node name to its etcd http address
func nameToEtcdURL(name string) (string, bool) {

	if info, ok := namesMap[name]; ok {
		// first try to read from the map
		return info.etcdURL, true
	}

	// if fails, try to recover from etcd storage
	return readURL(name, "etcd")

}

// nameToRaftURL maps node name to its raft http address
func nameToRaftURL(name string) (string, bool) {
	if info, ok := namesMap[name]; ok {
		// first try to read from the map
		return info.raftURL, true

	}

	// if fails, try to recover from etcd storage
	return readURL(name, "raft")
}

// addNameToURL add a name that maps to raftURL and etcdURL
func addNameToURL(name string, version string, raftURL string, etcdURL string) {
	namesMap[name] = &nodeInfo{
		version: version,
		raftURL: raftURL,
		etcdURL: etcdURL,
	}
}

func readURL(nodeName string, urlName string) (string, bool) {
	// if fails, try to recover from etcd storage
	key := path.Join("/_etcd/machines", nodeName)

	resps, err := etcdStore.RawGet(key)

	if err != nil {
		return "", false
	}

	m, err := url.ParseQuery(resps[0].Value)

	if err != nil {
		panic("Failed to parse machines entry")
	}

	url := m[urlName][0]

	return url, true
}
