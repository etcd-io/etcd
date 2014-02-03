package etcd

import (
	"path"
	"testing"
)

func testKey(t *testing.T, in, exp string) {
	if keyToPath(in) != exp {
		t.Errorf("Expected %s got %s", exp, keyToPath(in))
	}
}

// TestKeyToPath ensures the key cleaning funciton keyToPath works in a number
// of cases.
func TestKeyToPath(t *testing.T) {
	testKey(t, "", "/")
	testKey(t, "/", "/")
	testKey(t, "///", "/")
	testKey(t, "hello/world/", "hello/world")
	testKey(t, "///hello////world/../", "/hello")
}

func testPath(t *testing.T, c *Client, in, exp string) {
	out := c.getHttpPath(false, in)

	if out != exp {
		t.Errorf("Expected %s got %s", exp, out)
	}
}

// TestHttpPath ensures that the URLs generated make sense for the given keys
func TestHttpPath(t *testing.T) {
	c := NewClient(nil)

	testPath(t, c,
		path.Join(c.keyPrefix, "hello") + "?prevInit=true",
		"http://127.0.0.1:4001/v2/keys/hello?prevInit=true")

	testPath(t, c,
		path.Join(c.keyPrefix, "///hello///world") + "?prevInit=true",
		"http://127.0.0.1:4001/v2/keys/hello/world?prevInit=true")

	c = NewClient([]string{"https://discovery.etcd.io"})
	c.SetKeyPrefix("")

	testPath(t, c,
		path.Join(c.keyPrefix, "hello") + "?prevInit=true",
		"https://discovery.etcd.io/hello?prevInit=true")
}
