package v1

import (
	"github.com/coreos/etcd/store"
	"github.com/coreos/go-raft"
	"net/http"
)

// The Server interface provides all the methods required for the v1 API.
type Server interface {
	CommitIndex() uint64
	Term() uint64
	Store() store.Store
	Dispatch(raft.Command, http.ResponseWriter, *http.Request) error
}
