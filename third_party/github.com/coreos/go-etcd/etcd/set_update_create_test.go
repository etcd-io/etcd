package etcd

import (
	"testing"
)

func TestSet(t *testing.T) {
	c := NewClient(nil)
	defer func() {
		c.DeleteAll("foo")
	}()

	resp, err := c.Set("foo", "bar", 5)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Key != "/foo" || resp.Value != "bar" || resp.TTL != 5 {
		t.Fatalf("Set 1 failed: %#v", resp)
	}

	resp, err = c.Set("foo", "bar2", 5)
	if err != nil {
		t.Fatal(err)
	}
	if !(resp.Key == "/foo" && resp.Value == "bar2" &&
		resp.PrevValue == "bar" && resp.TTL == 5) {
		t.Fatalf("Set 2 failed: %#v", resp)
	}
}

func TestUpdate(t *testing.T) {
	c := NewClient(nil)
	defer func() {
		c.DeleteAll("foo")
		c.DeleteAll("nonexistent")
	}()

	resp, err := c.Set("foo", "bar", 5)
	t.Logf("%#v", resp)
	if err != nil {
		t.Fatal(err)
	}

	// This should succeed.
	resp, err = c.Update("foo", "wakawaka", 5)
	if err != nil {
		t.Fatal(err)
	}

	if !(resp.Action == "update" && resp.Key == "/foo" &&
		resp.PrevValue == "bar" && resp.TTL == 5) {
		t.Fatalf("Update 1 failed: %#v", resp)
	}

	// This should fail because the key does not exist.
	resp, err = c.Update("nonexistent", "whatever", 5)
	if err == nil {
		t.Fatalf("The key %v did not exist, so the update should have failed."+
			"The response was: %#v", resp.Key, resp)
	}
}

func TestCreate(t *testing.T) {
	c := NewClient(nil)
	defer func() {
		c.DeleteAll("newKey")
	}()

	newKey := "/newKey"
	newValue := "/newValue"

	// This should succeed
	resp, err := c.Create(newKey, newValue, 5)
	if err != nil {
		t.Fatal(err)
	}

	if !(resp.Action == "create" && resp.Key == newKey &&
		resp.Value == newValue && resp.PrevValue == "" && resp.TTL == 5) {
		t.Fatalf("Create 1 failed: %#v", resp)
	}

	// This should fail, because the key is already there
	resp, err = c.Create(newKey, newValue, 5)
	if err == nil {
		t.Fatalf("The key %v did exist, so the creation should have failed."+
			"The response was: %#v", resp.Key, resp)
	}
}

func TestSetDir(t *testing.T) {
	c := NewClient(nil)
	defer func() {
		c.DeleteAll("foo")
		c.DeleteAll("fooDir")
	}()

	resp, err := c.SetDir("fooDir", 5)
	if err != nil {
		t.Fatal(err)
	}
	if !(resp.Key == "/fooDir" && resp.Value == "" && resp.TTL == 5) {
		t.Fatalf("SetDir 1 failed: %#v", resp)
	}

	// This should fail because /fooDir already points to a directory
	resp, err = c.SetDir("/fooDir", 5)
	if err == nil {
		t.Fatalf("fooDir already points to a directory, so SetDir should have failed."+
			"The response was: %#v", resp)
	}

	_, err = c.Set("foo", "bar", 5)
	if err != nil {
		t.Fatal(err)
	}

	// This should succeed
	resp, err = c.SetDir("foo", 5)
	if err != nil {
		t.Fatal(err)
	}
	if !(resp.Key == "/foo" && resp.Value == "" &&
		resp.PrevValue == "bar" && resp.TTL == 5) {
		t.Fatalf("SetDir 2 failed: %#v", resp)
	}
}

func TestUpdateDir(t *testing.T) {
	c := NewClient(nil)
	defer func() {
		c.DeleteAll("fooDir")
	}()

	resp, err := c.SetDir("fooDir", 5)
	t.Logf("%#v", resp)
	if err != nil {
		t.Fatal(err)
	}

	// This should succeed.
	resp, err = c.UpdateDir("fooDir", 5)
	if err != nil {
		t.Fatal(err)
	}

	if !(resp.Action == "update" && resp.Key == "/fooDir" &&
		resp.Value == "" && resp.PrevValue == "" && resp.TTL == 5) {
		t.Fatalf("UpdateDir 1 failed: %#v", resp)
	}

	// This should fail because the key does not exist.
	resp, err = c.UpdateDir("nonexistentDir", 5)
	if err == nil {
		t.Fatalf("The key %v did not exist, so the update should have failed."+
			"The response was: %#v", resp.Key, resp)
	}
}

func TestCreateDir(t *testing.T) {
	c := NewClient(nil)
	defer func() {
		c.DeleteAll("fooDir")
	}()

	// This should succeed
	resp, err := c.CreateDir("fooDir", 5)
	if err != nil {
		t.Fatal(err)
	}

	if !(resp.Action == "create" && resp.Key == "/fooDir" &&
		resp.Value == "" && resp.PrevValue == "" && resp.TTL == 5) {
		t.Fatalf("CreateDir 1 failed: %#v", resp)
	}

	// This should fail, because the key is already there
	resp, err = c.CreateDir("fooDir", 5)
	if err == nil {
		t.Fatalf("The key %v did exist, so the creation should have failed."+
			"The response was: %#v", resp.Key, resp)
	}
}
