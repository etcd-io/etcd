package v2

import (
	"github.com/coreos/etcd/store"
	"github.com/coreos/go-raft"
	"net/http"
)

// The Server interface provides all the methods required for the v2 API.
type Server interface {
	State() string
	Leader() string
	CommitIndex() uint64
	Term() uint64
	PeerURL(string) (string, bool)
	Store() store.Store
	Dispatch(raft.Command, http.ResponseWriter, *http.Request) error
}
