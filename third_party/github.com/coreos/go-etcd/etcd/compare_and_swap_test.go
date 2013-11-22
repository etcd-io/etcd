package etcd

import (
	"testing"
)

func TestCompareAndSwap(t *testing.T) {
	c := NewClient(nil)
	defer func() {
		c.DeleteAll("foo")
	}()

	c.Set("foo", "bar", 5)

	// This should succeed
	resp, err := c.CompareAndSwap("foo", "bar2", 5, "bar", 0)
	if err != nil {
		t.Fatal(err)
	}
	if !(resp.Value == "bar2" && resp.PrevValue == "bar" &&
		resp.Key == "/foo" && resp.TTL == 5) {
		t.Fatalf("CompareAndSwap 1 failed: %#v", resp)
	}

	// This should fail because it gives an incorrect prevValue
	resp, err = c.CompareAndSwap("foo", "bar3", 5, "xxx", 0)
	if err == nil {
		t.Fatalf("CompareAndSwap 2 should have failed.  The response is: %#v", resp)
	}

	resp, err = c.Set("foo", "bar", 5)
	if err != nil {
		t.Fatal(err)
	}

	// This should succeed
	resp, err = c.CompareAndSwap("foo", "bar2", 5, "", resp.ModifiedIndex)
	if err != nil {
		t.Fatal(err)
	}
	if !(resp.Value == "bar2" && resp.PrevValue == "bar" &&
		resp.Key == "/foo" && resp.TTL == 5) {
		t.Fatalf("CompareAndSwap 1 failed: %#v", resp)
	}

	// This should fail because it gives an incorrect prevIndex
	resp, err = c.CompareAndSwap("foo", "bar3", 5, "", 29817514)
	if err == nil {
		t.Fatalf("CompareAndSwap 2 should have failed.  The response is: %#v", resp)
	}
}
