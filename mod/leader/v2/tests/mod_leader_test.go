package leader

import (
	"fmt"
	"testing"
	"time"

	"github.com/coreos/etcd/server"
	"github.com/coreos/etcd/tests"
	"github.com/coreos/etcd/third_party/github.com/stretchr/testify/assert"
)

// Ensure that a leader can be set and read.
func TestModLeaderSet(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		// Set leader.
		body, status, err := testSetLeader(s, "foo", "xxx", 10)
		assert.NoError(t, err)
		assert.Equal(t, status, 200)
		assert.Equal(t, body, "2")

		// Check that the leader is set.
		body, status, err = testGetLeader(s, "foo")
		assert.NoError(t, err)
		assert.Equal(t, status, 200)
		assert.Equal(t, body, "xxx")

		// Delete leader.
		body, status, err = testDeleteLeader(s, "foo", "xxx")
		assert.NoError(t, err)
		assert.Equal(t, status, 200)
		assert.Equal(t, body, "")

		// Check that the leader is removed.
		body, status, err = testGetLeader(s, "foo")
		assert.NoError(t, err)
		assert.Equal(t, status, 200)
		assert.Equal(t, body, "")
	})
}

// Ensure that a leader can be renewed.
func TestModLeaderRenew(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		// Set leader.
		body, status, err := testSetLeader(s, "foo", "xxx", 2)
		assert.NoError(t, err)
		assert.Equal(t, status, 200)
		assert.Equal(t, body, "2")

		time.Sleep(1 * time.Second)

		// Renew leader.
		body, status, err = testSetLeader(s, "foo", "xxx", 3)
		assert.NoError(t, err)
		assert.Equal(t, status, 200)
		assert.Equal(t, body, "2")

		time.Sleep(2 * time.Second)

		// Check that the leader is set.
		body, status, err = testGetLeader(s, "foo")
		assert.NoError(t, err)
		assert.Equal(t, status, 200)
		assert.Equal(t, body, "xxx")
	})
}

func testSetLeader(s *server.Server, key string, name string, ttl int) (string, int, error) {
	resp, err := tests.PutForm(fmt.Sprintf("%s/mod/v2/leader/%s?name=%s&ttl=%d", s.URL(), key, name, ttl), nil)
	ret := tests.ReadBody(resp)
	return string(ret), resp.StatusCode, err
}

func testGetLeader(s *server.Server, key string) (string, int, error) {
	resp, err := tests.Get(fmt.Sprintf("%s/mod/v2/leader/%s", s.URL(), key))
	ret := tests.ReadBody(resp)
	return string(ret), resp.StatusCode, err
}

func testDeleteLeader(s *server.Server, key string, name string) (string, int, error) {
	resp, err := tests.DeleteForm(fmt.Sprintf("%s/mod/v2/leader/%s?name=%s", s.URL(), key, name), nil)
	ret := tests.ReadBody(resp)
	return string(ret), resp.StatusCode, err
}
