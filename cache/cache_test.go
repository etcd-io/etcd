package cache

import "testing"

func TestCacheSetGet(t *testing.T) {
	c := NewCache()
	obj := Object{Key: "foo", Value: []byte("bar")}
	c.Set(obj)

	got, ok := c.Get("foo")
	if !ok || string(got.Value) != "bar" {
		t.Errorf("Expected bar, got %s", got.Value)
	}
}

func TestCacheDelete(t *testing.T) {
	c := NewCache()
	obj := Object{Key: "foo", Value: []byte("bar")}
	c.Set(obj)
	c.Delete("foo")

	_, ok := c.Get("foo")
	if ok {
		t.Errorf("Expected deletion of key 'foo'")
	}
}
