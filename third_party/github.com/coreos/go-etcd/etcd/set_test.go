package etcd

import (
	"testing"
	"time"
)

func TestSet(t *testing.T) {
	c := NewClient()

	result, err := c.Set("foo", "bar", 100)

	if err != nil || result.Key != "/foo" || result.Value != "bar" || result.TTL != 99 {
		if err != nil {
			t.Fatal(err)
		}

		t.Fatalf("Set 1 failed with %s %s %v", result.Key, result.Value, result.TTL)
	}

	time.Sleep(time.Second)

	result, err = c.Set("foo", "bar", 100)

	if err != nil || result.Key != "/foo" || result.Value != "bar" || result.PrevValue != "bar" || result.TTL != 99 {
		if err != nil {
			t.Fatal(err)
		}
		t.Fatalf("Set 2 failed with %s %s %v", result.Key, result.Value, result.TTL)
	}

	result, err = c.SetTo("toFoo", "bar", 100, "0.0.0.0:4001")

	if err != nil || result.Key != "/toFoo" || result.Value != "bar" || result.TTL != 99 {
		if err != nil {
			t.Fatal(err)
		}

		t.Fatalf("SetTo failed with %s %s %v", result.Key, result.Value, result.TTL)
	}

}
