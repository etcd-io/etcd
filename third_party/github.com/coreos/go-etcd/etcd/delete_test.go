package etcd

import (
	"testing"
)

func TestDelete(t *testing.T) {
	c := NewClient(nil)
	defer func() {
		c.DeleteAll("foo")
	}()

	c.Set("foo", "bar", 5)
	resp, err := c.Delete("foo")
	if err != nil {
		t.Fatal(err)
	}

	if !(resp.PrevValue == "bar" && resp.Value == "") {
		t.Fatalf("Delete failed with %s %s", resp.PrevValue,
			resp.Value)
	}

	resp, err = c.Delete("foo")
	if err == nil {
		t.Fatalf("Delete should have failed because the key foo did not exist.  "+
			"The response was: %v", resp)
	}
}

func TestDeleteAll(t *testing.T) {
	c := NewClient(nil)
	defer func() {
		c.DeleteAll("foo")
		c.DeleteAll("fooDir")
	}()

	c.Set("foo", "bar", 5)
	resp, err := c.DeleteAll("foo")
	if err != nil {
		t.Fatal(err)
	}

	if !(resp.PrevValue == "bar" && resp.Value == "") {
		t.Fatalf("DeleteAll 1 failed: %#v", resp)
	}

	c.SetDir("fooDir", 5)
	c.Set("fooDir/foo", "bar", 5)
	resp, err = c.DeleteAll("fooDir")
	if err != nil {
		t.Fatal(err)
	}

	if !(resp.PrevValue == "" && resp.Value == "") {
		t.Fatalf("DeleteAll 2 failed: %#v", resp)
	}

	resp, err = c.DeleteAll("foo")
	if err == nil {
		t.Fatalf("DeleteAll should have failed because the key foo did not exist.  "+
			"The response was: %v", resp)
	}
}
