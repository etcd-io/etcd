package raft

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Ensure that a snapshot occurs when there are existing logs.
func TestSnapshot(t *testing.T) {
	runServerWithMockStateMachine(Leader, func(s Server, m *mock.Mock) {
		m.On("Save").Return([]byte("foo"), nil)
		m.On("Recovery", []byte("foo")).Return(nil)

		s.Do(&testCommand1{})
		err := s.TakeSnapshot()
		assert.NoError(t, err)
		assert.Equal(t, s.(*server).lastSnapshot.LastIndex, uint64(2))

		// Repeat to make sure new snapshot gets created.
		s.Do(&testCommand1{})
		err = s.TakeSnapshot()
		assert.NoError(t, err)
		assert.Equal(t, s.(*server).lastSnapshot.LastIndex, uint64(4))

		// Restart server.
		s.Stop()
		s.Start()

		// Recover from snapshot.
		err = s.LoadSnapshot()
		assert.NoError(t, err)
	})
}

// Ensure that a snapshot request can be sent and received.
func TestSnapshotRequest(t *testing.T) {
	runServerWithMockStateMachine(Follower, func(s Server, m *mock.Mock) {
		m.On("Recovery", []byte("bar")).Return(nil)

		// Send snapshot request.
		resp := s.RequestSnapshot(&SnapshotRequest{LastIndex: 5, LastTerm: 1})
		assert.Equal(t, resp.Success, true)
		assert.Equal(t, s.State(), Snapshotting)

		// Send recovery request.
		resp2 := s.SnapshotRecoveryRequest(&SnapshotRecoveryRequest{
			LeaderName: "1",
			LastIndex:  5,
			LastTerm:   2,
			Peers:      make([]*Peer, 0),
			State:      []byte("bar"),
		})
		assert.Equal(t, resp2.Success, true)
	})
}

func runServerWithMockStateMachine(state string, fn func(s Server, m *mock.Mock)) {
	var m mockStateMachine
	s := newTestServer("1", &testTransporter{})
	s.(*server).stateMachine = &m
	if err := s.Start(); err != nil {
		panic("server start error: " + err.Error())
	}
	if state == Leader {
		if _, err := s.Do(&DefaultJoinCommand{Name: s.Name()}); err != nil {
			panic("unable to join server to self: " + err.Error())
		}
	}
	defer s.Stop()
	fn(s, &m.Mock)
}
