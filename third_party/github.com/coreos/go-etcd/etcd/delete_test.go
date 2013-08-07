package etcd

import (
	"testing"
)

func TestDelete(t *testing.T) {

	c := NewClient()

	c.Set("foo", "bar", 100)
	result, err := c.Delete("foo")
	if err != nil {
		t.Fatal(err)
	}

	if result.PrevValue != "bar" || result.Value != "" {
		t.Fatalf("Delete failed with %s %s", result.PrevValue,
			result.Value)
	}

}
