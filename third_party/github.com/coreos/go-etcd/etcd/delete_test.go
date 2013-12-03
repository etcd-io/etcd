package etcd

import (
	"testing"
)

func TestDelete(t *testing.T) {
	c := NewClient(nil)
	defer func() {
		c.Delete("foo", true)
	}()

	c.Set("foo", "bar", 5)
	resp, err := c.Delete("foo", false)
	if err != nil {
		t.Fatal(err)
	}

	if !(resp.Node.PrevValue == "bar" && resp.Node.Value == "") {
		t.Fatalf("Delete failed with %s %s", resp.Node.PrevValue,
			resp.Node.Value)
	}

	resp, err = c.Delete("foo", false)
	if err == nil {
		t.Fatalf("Delete should have failed because the key foo did not exist.  "+
			"The response was: %v", resp)
	}
}

func TestDeleteAll(t *testing.T) {
	c := NewClient(nil)
	defer func() {
		c.Delete("foo", true)
		c.Delete("fooDir", true)
	}()

	c.Set("foo", "bar", 5)
	resp, err := c.Delete("foo", true)
	if err != nil {
		t.Fatal(err)
	}

	if !(resp.Node.PrevValue == "bar" && resp.Node.Value == "") {
		t.Fatalf("DeleteAll 1 failed: %#v", resp)
	}

	c.SetDir("fooDir", 5)
	c.Set("fooDir/foo", "bar", 5)
	resp, err = c.Delete("fooDir", true)
	if err != nil {
		t.Fatal(err)
	}

	if !(resp.Node.PrevValue == "" && resp.Node.Value == "") {
		t.Fatalf("DeleteAll 2 failed: %#v", resp)
	}

	resp, err = c.Delete("foo", true)
	if err == nil {
		t.Fatalf("DeleteAll should have failed because the key foo did not exist.  "+
			"The response was: %v", resp)
	}
}
