package lock

import (
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/coreos/etcd/server"
	"github.com/coreos/etcd/tests"
	"github.com/stretchr/testify/assert"
)

// Ensure that a lock can be acquired and released.
func TestModLockAcquire(t *testing.T) {
	v := url.Values{}
	tests.RunServer(func(s *server.Server) {
		// Acquire lock.
		resp, err := tests.PutForm(fmt.Sprintf("http://%s%s", s.URL(), "/mod/lock"), v)
		assert.NoError(t, err)
		ret := tests.ReadBody(resp)
		assert.Equal(t, string(ret), "XXX")

		fmt.Println("URL:", fmt.Sprintf("http://%s%s", s.URL(), "/mod/lock"))
		time.Sleep(60 * time.Second)
		// TODO: Check that it has been acquired.
		// TODO: Release lock.
		// TODO: Check that it has been released.
	})
}

// Ensure that a lock can be acquired and another process is blocked until released.
func TestModLockAcquireBlocked(t *testing.T) {
	// TODO: Acquire lock with process #1.
	// TODO: Acquire lock with process #2.
	// TODO: Check that process #2 has not obtained lock.
	// TODO: Release lock from process #1.
	// TODO: Check that process #2 obtains the lock.
	// TODO: Release lock from process #2.
	// TODO: Check that no lock exists.
}

// Ensure that an unowned lock can be released by force.
func TestModLockForceRelease(t *testing.T) {
	// TODO: Acquire lock.
	// TODO: Check that it has been acquired.
	// TODO: Force release lock.
	// TODO: Check that it has been released.
	// TODO: Check that acquiring goroutine is notified that their lock has been released.
}
