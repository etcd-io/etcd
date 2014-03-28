package v2

import (
	"net/http"

	"github.com/coreos/etcd/store"
	"github.com/coreos/etcd/third_party/github.com/goraft/raft"
)

// The Server interface provides all the methods required for the v2 API.
type Server interface {
	State() string
	Leader() string
	CommitIndex() uint64
	Term() uint64
	PeerURL(string) (string, bool)
	ClientURL(string) (string, bool)
	Store() store.Store
	Dispatch(raft.Command, http.ResponseWriter, *http.Request) error
}
