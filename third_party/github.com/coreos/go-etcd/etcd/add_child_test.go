package etcd

import "testing"

func TestAddChild(t *testing.T) {
	c := NewClient(nil)
	defer func() {
		c.DeleteAll("fooDir")
		c.DeleteAll("nonexistentDir")
	}()

	c.SetDir("fooDir", 5)

	_, err := c.AddChild("fooDir", "v0", 5)
	if err != nil {
		t.Fatal(err)
	}

	_, err = c.AddChild("fooDir", "v1", 5)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.Get("fooDir", true)
	// The child with v0 should proceed the child with v1 because it's added
	// earlier, so it should have a lower key.
	if !(len(resp.Kvs) == 2 && (resp.Kvs[0].Value == "v0" && resp.Kvs[1].Value == "v1")) {
		t.Fatalf("AddChild 1 failed.  There should be two chlidren whose values are v0 and v1, respectively."+
			"  The response was: %#v", resp)
	}

	// Creating a child under a nonexistent directory should succeed.
	// The directory should be created.
	resp, err = c.AddChild("nonexistentDir", "foo", 5)
	if err != nil {
		t.Fatal(err)
	}
}

func TestAddChildDir(t *testing.T) {
	c := NewClient(nil)
	defer func() {
		c.DeleteAll("fooDir")
		c.DeleteAll("nonexistentDir")
	}()

	c.SetDir("fooDir", 5)

	_, err := c.AddChildDir("fooDir", 5)
	if err != nil {
		t.Fatal(err)
	}

	_, err = c.AddChildDir("fooDir", 5)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.Get("fooDir", true)
	// The child with v0 should proceed the child with v1 because it's added
	// earlier, so it should have a lower key.
	if !(len(resp.Kvs) == 2 && (len(resp.Kvs[0].KVPairs) == 0 && len(resp.Kvs[1].KVPairs) == 0)) {
		t.Fatalf("AddChildDir 1 failed.  There should be two chlidren whose values are v0 and v1, respectively."+
			"  The response was: %#v", resp)
	}

	// Creating a child under a nonexistent directory should succeed.
	// The directory should be created.
	resp, err = c.AddChildDir("nonexistentDir", 5)
	if err != nil {
		t.Fatal(err)
	}
}
