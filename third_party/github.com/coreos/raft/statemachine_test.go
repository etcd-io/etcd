package raft

import (
	"github.com/stretchr/testify/mock"
)

type mockStateMachine struct {
	mock.Mock
}

func (m *mockStateMachine) Save() ([]byte, error) {
	args := m.Called()
	return args.Get(0).([]byte), args.Error(1)
}

func (m *mockStateMachine) Recovery(b []byte) error {
	args := m.Called(b)
	return args.Error(0)
}
