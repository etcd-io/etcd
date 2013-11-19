package server

// Info describes the non-mutable state of the server upon initialization.
// These fields cannot be changed without deleting the server fields and
// reinitializing.
type Info struct {
	Name string `json:"name"`

	RaftURL string `json:"raftURL"`
	EtcdURL string `json:"etcdURL"`

	RaftListenHost string `json:"raftListenHost"`
	EtcdListenHost string `json:"etcdListenHost"`

	RaftTLS TLSInfo `json:"raftTLS"`
	EtcdTLS TLSInfo `json:"etcdTLS"`
}
