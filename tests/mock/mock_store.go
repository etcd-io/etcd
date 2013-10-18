package mock

import (
	"github.com/coreos/etcd/store"
	"github.com/stretchr/testify/mock"
	"time"
)

// A mock Store object used for testing.
type Store struct {
	mock.Mock
}

func NewStore() *Store {
	return &Store{}
}

func (s *Store) Get(nodePath string, recursive, sorted bool, index uint64, term uint64) (*store.Event, error) {
	args := s.Called(nodePath, recursive, sorted, index, term)
	return args.Get(0).(*store.Event), args.Error(1)
}

func (s *Store) Set(nodePath string, value string, expireTime time.Time, index uint64, term uint64) (*store.Event, error) {
	args := s.Called(nodePath, value, expireTime, index, term)
	return args.Get(0).(*store.Event), args.Error(1)
}

func (s *Store) Update(nodePath string, newValue string, expireTime time.Time, index uint64, term uint64) (*store.Event, error) {
	args := s.Called(nodePath, newValue, expireTime, index, term)
	return args.Get(0).(*store.Event), args.Error(1)
}

func (s *Store) Create(nodePath string, value string, incrementalSuffix bool, expireTime time.Time, index uint64, term uint64) (*store.Event, error) {
	args := s.Called(nodePath, value, incrementalSuffix, expireTime, index, term)
	return args.Get(0).(*store.Event), args.Error(1)
}

func (s *Store) CompareAndSwap(nodePath string, prevValue string, prevIndex uint64, value string, expireTime time.Time, index uint64, term uint64) (*store.Event, error) {
	args := s.Called(nodePath, prevValue, prevIndex, value, expireTime, index, term)
	return args.Get(0).(*store.Event), args.Error(1)
}

func (s *Store) Delete(nodePath string, recursive bool, index uint64, term uint64) (*store.Event, error) {
	args := s.Called(nodePath, recursive, index, term)
	return args.Get(0).(*store.Event), args.Error(1)
}

func (s *Store) Watch(prefix string, recursive bool, sinceIndex uint64, index uint64, term uint64) (<-chan *store.Event, error) {
	args := s.Called(prefix, recursive, sinceIndex, index, term)
	return args.Get(0).(<-chan *store.Event), args.Error(1)
}

func (s *Store) Save() ([]byte, error) {
	args := s.Called()
	return args.Get(0).([]byte), args.Error(1)
}

func (s *Store) Recovery(b []byte) error {
	args := s.Called(b)
	return args.Error(1)
}

func (s *Store) JsonStats() []byte {
	args := s.Called()
	return args.Get(0).([]byte)
}
