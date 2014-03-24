package lock

import (
	"fmt"
	"testing"
	"time"

	"github.com/coreos/etcd/server"
	"github.com/coreos/etcd/tests"
	"github.com/coreos/etcd/third_party/github.com/stretchr/testify/assert"
)

// Ensure that a lock can be acquired and released.
func TestModLockAcquireAndRelease(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		// Acquire lock.
		body, status, err := testAcquireLock(s, "foo", "", 10)
		assert.NoError(t, err)
		assert.Equal(t, status, 200)
		assert.Equal(t, body, "2")

		// Check that we have the lock.
		body, status, err = testGetLockIndex(s, "foo")
		assert.NoError(t, err)
		assert.Equal(t, status, 200)
		assert.Equal(t, body, "2")

		// Release lock.
		body, status, err = testReleaseLock(s, "foo", "2", "")
		assert.NoError(t, err)
		assert.Equal(t, status, 200)
		assert.Equal(t, body, "")

		// Check that we have the lock.
		body, status, err = testGetLockIndex(s, "foo")
		assert.NoError(t, err)
		assert.Equal(t, status, 200)
		assert.Equal(t, body, "")
	})
}

// Ensure that a lock can be acquired and another process is blocked until released.
func TestModLockBlockUntilAcquire(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		c := make(chan bool)

		// Acquire lock #1.
		go func() {
			body, status, err := testAcquireLock(s, "foo", "", 10)
			assert.NoError(t, err)
			assert.Equal(t, status, 200)
			assert.Equal(t, body, "2")
			c <- true
		}()
		<-c

		// Acquire lock #2.
		waiting := true
		go func() {
			c <- true
			body, status, err := testAcquireLock(s, "foo", "", 10)
			assert.NoError(t, err)
			assert.Equal(t, status, 200)
			assert.Equal(t, body, "4")
			waiting = false
		}()
		<-c

		time.Sleep(1 * time.Second)

		// Check that we have the lock #1.
		body, status, err := testGetLockIndex(s, "foo")
		assert.NoError(t, err)
		assert.Equal(t, status, 200)
		assert.Equal(t, body, "2")

		// Check that we are still waiting for lock #2.
		assert.Equal(t, waiting, true)

		// Release lock #1.
		_, status, err = testReleaseLock(s, "foo", "2", "")
		assert.NoError(t, err)
		assert.Equal(t, status, 200)

		// Check that we have lock #2.
		body, status, err = testGetLockIndex(s, "foo")
		assert.NoError(t, err)
		assert.Equal(t, status, 200)
		assert.Equal(t, body, "4")

		// Release lock #2.
		_, status, err = testReleaseLock(s, "foo", "4", "")
		assert.NoError(t, err)
		assert.Equal(t, status, 200)

		// Check that we have no lock.
		body, status, err = testGetLockIndex(s, "foo")
		assert.NoError(t, err)
		assert.Equal(t, status, 200)
		assert.Equal(t, body, "")
	})
}

// Ensure that a lock will be released after the TTL.
func TestModLockExpireAndRelease(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		c := make(chan bool)

		// Acquire lock #1.
		go func() {
			body, status, err := testAcquireLock(s, "foo", "", 2)
			assert.NoError(t, err)
			assert.Equal(t, status, 200)
			assert.Equal(t, body, "2")
			c <- true
		}()
		<-c

		// Acquire lock #2.
		go func() {
			c <- true
			body, status, err := testAcquireLock(s, "foo", "", 10)
			assert.NoError(t, err)
			assert.Equal(t, status, 200)
			assert.Equal(t, body, "4")
		}()
		<-c

		time.Sleep(1 * time.Second)

		// Check that we have the lock #1.
		body, status, err := testGetLockIndex(s, "foo")
		assert.NoError(t, err)
		assert.Equal(t, status, 200)
		assert.Equal(t, body, "2")

		// Wait for lock #1 TTL.
		time.Sleep(2 * time.Second)

		// Check that we have lock #2.
		body, status, err = testGetLockIndex(s, "foo")
		assert.NoError(t, err)
		assert.Equal(t, status, 200)
		assert.Equal(t, body, "4")
	})
}

// Ensure that a lock can be renewed.
func TestModLockRenew(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		// Acquire lock.
		body, status, err := testAcquireLock(s, "foo", "", 3)
		assert.NoError(t, err)
		assert.Equal(t, status, 200)
		assert.Equal(t, body, "2")

		time.Sleep(2 * time.Second)

		// Check that we have the lock.
		body, status, err = testGetLockIndex(s, "foo")
		assert.NoError(t, err)
		assert.Equal(t, status, 200)
		assert.Equal(t, body, "2")

		// Renew lock.
		body, status, err = testRenewLock(s, "foo", "2", "", 3)
		assert.NoError(t, err)
		assert.Equal(t, status, 200)
		assert.Equal(t, body, "")

		time.Sleep(2 * time.Second)

		// Check that we still have the lock.
		body, status, err = testGetLockIndex(s, "foo")
		assert.NoError(t, err)
		assert.Equal(t, status, 200)
		assert.Equal(t, body, "2")

		time.Sleep(2 * time.Second)

		// Check that lock was released.
		body, status, err = testGetLockIndex(s, "foo")
		assert.NoError(t, err)
		assert.Equal(t, status, 200)
		assert.Equal(t, body, "")
	})
}

// Ensure that a lock can be acquired with a value and released by value.
func TestModLockAcquireAndReleaseByValue(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		// Acquire lock.
		body, status, err := testAcquireLock(s, "foo", "XXX", 10)
		assert.NoError(t, err)
		assert.Equal(t, status, 200)
		assert.Equal(t, body, "2")

		// Check that we have the lock.
		body, status, err = testGetLockValue(s, "foo")
		assert.NoError(t, err)
		assert.Equal(t, status, 200)
		assert.Equal(t, body, "XXX")

		// Release lock.
		body, status, err = testReleaseLock(s, "foo", "", "XXX")
		assert.NoError(t, err)
		assert.Equal(t, status, 200)
		assert.Equal(t, body, "")

		// Check that we released the lock.
		body, status, err = testGetLockValue(s, "foo")
		assert.NoError(t, err)
		assert.Equal(t, status, 200)
		assert.Equal(t, body, "")
	})
}

// Ensure that a lock honours the timeout option
func TestModLockAcquireTimeout(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		c := make(chan bool)

		// Acquire lock #1.
		go func() {
			body, status, err := testAcquireLock(s, "foo", "first", 10)
			assert.NoError(t, err)
			assert.Equal(t, status, 200)
			assert.Equal(t, body, "2")
			c <- true
		}()
		<-c

		// Attempt to acquire lock #2, timing out after 1s.
		waiting := true
		go func() {
			c <- true
			_, status, err := testAcquireLockWithTimeout(s, "foo", "second", 10, 1)
			assert.NoError(t, err)
			assert.Equal(t, status, 500)
			waiting = false
		}()
		<-c

		time.Sleep(5 * time.Second)

		// Check that we have the lock #1.
		body, status, err := testGetLockIndex(s, "foo")
		assert.NoError(t, err)
		assert.Equal(t, status, 200)
		assert.Equal(t, body, "2")

		// Check that we are not still waiting for lock #2.
		assert.Equal(t, waiting, false)

		// Release lock #1.
		_, status, err = testReleaseLock(s, "foo", "2", "")
		assert.NoError(t, err)
		assert.Equal(t, status, 200)

		// Check that we have no lock.
		body, status, err = testGetLockIndex(s, "foo")
		assert.NoError(t, err)
		assert.Equal(t, status, 200)
	})
}

// Ensure that a lock succeeds when timeout=0 (nowait)
func TestModLockAcquireNoWait(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		c := make(chan bool)

		// Acquire lock with no waiting.
		go func() {
			body, status, err := testAcquireLockWithTimeout(s, "foo", "first", 10, 0)
			assert.NoError(t, err)
			assert.Equal(t, status, 200)
			assert.Equal(t, body, "2")
			c <- true
		}()
		<-c

		time.Sleep(1 * time.Second)

		// Check that we have the lock #1.
		body, status, err := testGetLockIndex(s, "foo")
		assert.NoError(t, err)
		assert.Equal(t, status, 200)
		assert.Equal(t, body, "2")

		// Release lock #1.
		_, status, err = testReleaseLock(s, "foo", "2", "")
		assert.NoError(t, err)
		assert.Equal(t, status, 200)

		// Check that we have no lock.
		body, status, err = testGetLockIndex(s, "foo")
		assert.NoError(t, err)
		assert.Equal(t, status, 200)
	})
}

// Ensure that a lock honours the timeout=0 (nowait) option when lock is already held
func TestModLockAcquireNoWaitWhileLocked(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		c := make(chan bool)

		// Acquire lock #1.
		go func() {
			body, status, err := testAcquireLock(s, "foo", "first", 10)
			assert.NoError(t, err)
			assert.Equal(t, status, 200)
			assert.Equal(t, body, "2")
			c <- true
		}()
		<-c

		// Attempt to acquire lock #2; fail if no lock immediately acquired
		waiting := true
		go func() {
			c <- true
			_, status, err := testAcquireLockWithTimeout(s, "foo", "second", 10, 0)
			assert.NoError(t, err)
			assert.Equal(t, status, 500)
			waiting = false
		}()
		<-c

		time.Sleep(1 * time.Second)

		// Check that we have the lock #1.
		body, status, err := testGetLockIndex(s, "foo")
		assert.NoError(t, err)
		assert.Equal(t, status, 200)
		assert.Equal(t, body, "2")

		// Check that we are not still waiting for lock #2.
		assert.Equal(t, waiting, false)

		// Release lock #1.
		_, status, err = testReleaseLock(s, "foo", "2", "")
		assert.NoError(t, err)
		assert.Equal(t, status, 200)

		// Check that we have no lock.
		body, status, err = testGetLockIndex(s, "foo")
		assert.NoError(t, err)
		assert.Equal(t, status, 200)
	})
}

func testAcquireLock(s *server.Server, key string, value string, ttl int) (string, int, error) {
	resp, err := tests.PostForm(fmt.Sprintf("%s/mod/v2/lock/%s?value=%s&ttl=%d", s.URL(), key, value, ttl), nil)
	ret := tests.ReadBody(resp)
	return string(ret), resp.StatusCode, err
}

func testAcquireLockWithTimeout(s *server.Server, key string, value string, ttl int, timeout int) (string, int, error) {
	resp, err := tests.PostForm(fmt.Sprintf("%s/mod/v2/lock/%s?value=%s&ttl=%d&timeout=%d", s.URL(), key, value, ttl, timeout), nil)
	ret := tests.ReadBody(resp)
	return string(ret), resp.StatusCode, err
}

func testGetLockIndex(s *server.Server, key string) (string, int, error) {
	resp, err := tests.Get(fmt.Sprintf("%s/mod/v2/lock/%s?field=index", s.URL(), key))
	ret := tests.ReadBody(resp)
	return string(ret), resp.StatusCode, err
}

func testGetLockValue(s *server.Server, key string) (string, int, error) {
	resp, err := tests.Get(fmt.Sprintf("%s/mod/v2/lock/%s", s.URL(), key))
	ret := tests.ReadBody(resp)
	return string(ret), resp.StatusCode, err
}

func testReleaseLock(s *server.Server, key string, index string, value string) (string, int, error) {
	resp, err := tests.DeleteForm(fmt.Sprintf("%s/mod/v2/lock/%s?index=%s&value=%s", s.URL(), key, index, value), nil)
	ret := tests.ReadBody(resp)
	return string(ret), resp.StatusCode, err
}

func testRenewLock(s *server.Server, key string, index string, value string, ttl int) (string, int, error) {
	resp, err := tests.PutForm(fmt.Sprintf("%s/mod/v2/lock/%s?index=%s&value=%s&ttl=%d", s.URL(), key, index, value, ttl), nil)
	ret := tests.ReadBody(resp)
	return string(ret), resp.StatusCode, err
}
