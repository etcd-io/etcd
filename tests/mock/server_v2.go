package mock

import (
	"net/http"

	"github.com/coreos/etcd/store"
	"github.com/coreos/go-raft"
	"github.com/stretchr/testify/mock"
)

// A mock Server for the v2 handlers.
type ServerV2 struct {
	mock.Mock
	store store.Store
}

func NewServerV2(store store.Store) *ServerV2 {
	return &ServerV2{
		store: store,
	}
}

func (s *ServerV2) State() string {
	args := s.Called()
	return args.String(0)
}

func (s *ServerV2) Leader() string {
	args := s.Called()
	return args.String(0)
}

func (s *ServerV2) CommitIndex() uint64 {
	args := s.Called()
	return args.Get(0).(uint64)
}

func (s *ServerV2) Term() uint64 {
	args := s.Called()
	return args.Get(0).(uint64)
}

func (s *ServerV2) PeerURL(name string) (string, bool) {
	args := s.Called(name)
	return args.String(0), args.Bool(1)
}

func (s *ServerV2) Store() store.Store {
	return s.store
}

func (s *ServerV2) Dispatch(c raft.Command, w http.ResponseWriter, req *http.Request) error {
	args := s.Called(c, w, req)
	return args.Error(0)
}
