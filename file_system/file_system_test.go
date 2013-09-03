package fileSystem

import (
	"testing"
	"time"
)

func TestSetAndGet(t *testing.T) {
	fs := New()
	setAndGet(fs, "/foobar", t)
	setAndGet(fs, "/foo/bar", t)
	setAndGet(fs, "/foo/foo/bar", t)
}

func TestRemove(t *testing.T) {
	fs := New()

	fs.Set("/foo", "bar", Permanent, 1, 1)
	err := fs.Delete("/foo", false, 1, 1)

	if err != nil {
		t.Fatalf("cannot delete %s [%s]", "/foo", err.Error())
	}

	_, err = fs.InternalGet("/foo", 1, 1)

	if err == nil || err.Error() != "Key Not Found" {
		t.Fatalf("can get the node after deletion")
	}

	fs.Set("/foo/bar", "bar", Permanent, 1, 1)
	fs.Set("/foo/car", "car", Permanent, 1, 1)
	fs.Set("/foo/dar/dar", "dar", Permanent, 1, 1)

	err = fs.Delete("/foo", false, 1, 1)

	if err == nil {
		t.Fatalf("should not be able to delete a directory without recursive")
	}

	err = fs.Delete("/foo", true, 1, 1)

	if err != nil {
		t.Fatalf("cannot delete %s [%s]", "/foo", err.Error())
	}

	_, err = fs.InternalGet("/foo", 1, 1)

	if err == nil || err.Error() != "Key Not Found" {
		t.Fatalf("can get the node after deletion ")
	}

}

func TestExpire(t *testing.T) {
	fs := New()

	expire := time.Now().Add(time.Second)

	fs.Set("/foo", "bar", expire, 1, 1)

	_, err := fs.InternalGet("/foo", 1, 1)

	if err != nil {
		t.Fatalf("can not get the node")
	}

	time.Sleep(time.Second * 2)

	_, err = fs.InternalGet("/foo", 1, 1)

	if err == nil {
		t.Fatalf("can get the node after expiration time")
	}

	fs.Set("/foo", "bar", expire, 1, 1)

	time.Sleep(time.Millisecond * 50)
	_, err = fs.InternalGet("/foo", 1, 1)

	if err == nil {
		t.Fatalf("can get the node after expiration time")
	}

	expire = time.Now().Add(time.Second)

	fs.Set("/foo", "bar", expire, 1, 1)
	fs.Delete("/foo", false, 1, 1)

}

func setAndGet(fs *FileSystem, path string, t *testing.T) {
	err := fs.Set(path, "bar", Permanent, 1, 1)

	if err != nil {
		t.Fatalf("cannot set %s=bar [%s]", path, err.Error())
	}

	n, err := fs.InternalGet(path, 1, 1)

	if err != nil {
		t.Fatalf("cannot get %s [%s]", path, err.Error())
	}

	value, err := n.Read()

	if err != nil {
		t.Fatalf("cannot read %s [%s]", path, err.Error())
	}

	if value != "bar" {
		t.Fatalf("expect value of %s is bar [%s]", path, value)
	}
}
