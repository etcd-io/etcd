package etcd

import (
	"testing"
	"time"
)

func TestList(t *testing.T) {
	c := NewClient()

	c.Set("foo_list/foo", "bar", 100)
	c.Set("foo_list/fooo", "barbar", 100)
	c.Set("foo_list/foooo/foo", "barbarbar", 100)
	// wait for commit
	time.Sleep(time.Second)

	_, err := c.Get("foo_list")

	if err != nil {
		t.Fatal(err)
	}

}
