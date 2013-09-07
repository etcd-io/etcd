package fileSystem

import (
	"testing"
	"time"
)

func TestCreateAndGet(t *testing.T) {
	fs := New()

	// this should create successfully
	createAndGet(fs, "/foobar", t)
	createAndGet(fs, "/foo/bar", t)
	createAndGet(fs, "/foo/foo/bar", t)

	// already exist, create should fail
	_, err := fs.Create("/foobar", "bar", Permanent, 1, 1)

	if err == nil {
		t.Fatal("Create should fail")
	}

	// meet file, create should fail
	_, err = fs.Create("/foo/bar/bar", "bar", Permanent, 2, 1)

	if err == nil {
		t.Fatal("Create should fail")
	}

	// create a directory
	_, err = fs.Create("/fooDir", "", Permanent, 3, 1)

	if err != nil {
		t.Fatal("Cannot create /fooDir")
	}

	e, err := fs.Get("/fooDir", false, 3, 1)

	if err != nil || e.Dir != true {
		t.Fatal("Cannot create /fooDir ")
	}

	// create a file under directory
	_, err = fs.Create("/fooDir/bar", "bar", Permanent, 4, 1)

	if err != nil {
		t.Fatal("Cannot create /fooDir/bar = bar")
	}

}

func TestUpdateFile(t *testing.T) {
	fs := New()

	_, err := fs.Create("/foo/bar", "bar", Permanent, 1, 1)

	if err != nil {
		t.Fatalf("cannot update %s=bar [%s]", "/foo/bar", err.Error())
	}

	_, err = fs.Update("/foo/bar", "barbar", Permanent, 2, 1)

	if err != nil {
		t.Fatalf("cannot update %s=barbar [%s]", "/foo/bar", err.Error())
	}

	e, err := fs.Get("/foo/bar", false, 2, 1)

	if err != nil {
		t.Fatalf("cannot get %s [%s]", "/foo/bar", err.Error())
	}

	if e.Value != "barbar" {
		t.Fatalf("expect value of %s is barbar [%s]", "/foo/bar", e.Value)
	}
}

func TestListDirectory(t *testing.T) {
	fs := New()

	// create dir /foo
	// set key-value /foo/foo=bar
	fs.Create("/foo/foo", "bar", Permanent, 1, 1)

	// create dir /foo/fooDir
	// set key-value /foo/fooDir/foo=bar
	fs.Create("/foo/fooDir/foo", "bar", Permanent, 2, 1)

	e, err := fs.Get("/foo", true, 2, 1)

	if err != nil {
		t.Fatalf("%v", err)
	}

	if len(e.KVPairs) != 2 {
		t.Fatalf("wrong number of kv pairs [%d/2]", len(e.KVPairs))
	}

	if e.KVPairs[0].Key != "/foo/foo" || e.KVPairs[0].Value != "bar" {
		t.Fatalf("wrong kv [/foo/foo/ / %s] -> [bar / %s]", e.KVPairs[0].Key, e.KVPairs[0].Value)
	}

	if e.KVPairs[1].Key != "/foo/fooDir" || e.KVPairs[1].Dir != true {
		t.Fatalf("wrong kv [/foo/fooDir/ / %s] -> [true / %v]", e.KVPairs[1].Key, e.KVPairs[1].Dir)
	}

	if e.KVPairs[1].KVPairs[0].Key != "/foo/fooDir/foo" || e.KVPairs[1].KVPairs[0].Value != "bar" {
		t.Fatalf("wrong kv [/foo/fooDir/foo / %s] -> [bar / %v]", e.KVPairs[1].KVPairs[0].Key, e.KVPairs[1].KVPairs[0].Value)
	}
	// test hidden node

	// create dir /foo/_hidden
	// set key-value /foo/_hidden/foo -> bar
	fs.Create("/foo/_hidden/foo", "bar", Permanent, 3, 1)

	e, _ = fs.Get("/foo", false, 2, 1)

	if len(e.KVPairs) != 2 {
		t.Fatalf("hidden node is not hidden! %s", e.KVPairs[2].Key)
	}
}

func TestRemove(t *testing.T) {
	fs := New()

	fs.Create("/foo", "bar", Permanent, 1, 1)
	_, err := fs.Delete("/foo", false, 1, 1)

	if err != nil {
		t.Fatalf("cannot delete %s [%s]", "/foo", err.Error())
	}

	_, err = fs.Get("/foo", false, 1, 1)

	if err == nil || err.Error() != "Key Not Found" {
		t.Fatalf("can get the node after deletion")
	}

	fs.Create("/foo/bar", "bar", Permanent, 1, 1)
	fs.Create("/foo/car", "car", Permanent, 1, 1)
	fs.Create("/foo/dar/dar", "dar", Permanent, 1, 1)

	_, err = fs.Delete("/foo", false, 1, 1)

	if err == nil {
		t.Fatalf("should not be able to delete a directory without recursive")
	}

	_, err = fs.Delete("/foo", true, 1, 1)

	if err != nil {
		t.Fatalf("cannot delete %s [%s]", "/foo", err.Error())
	}

	_, err = fs.Get("/foo", false, 1, 1)

	if err == nil || err.Error() != "Key Not Found" {
		t.Fatalf("can get the node after deletion ")
	}

}

func TestExpire(t *testing.T) {
	fs := New()

	expire := time.Now().Add(time.Second)

	fs.Create("/foo", "bar", expire, 1, 1)

	_, err := fs.InternalGet("/foo", 1, 1)

	if err != nil {
		t.Fatalf("can not get the node")
	}

	time.Sleep(time.Second * 2)

	_, err = fs.InternalGet("/foo", 1, 1)

	if err == nil {
		t.Fatalf("can get the node after expiration time")
	}

	fs.Create("/foo", "bar", expire, 1, 1)

	time.Sleep(time.Millisecond * 50)
	_, err = fs.InternalGet("/foo", 1, 1)

	if err == nil {
		t.Fatalf("can get the node after expiration time")
	}

	expire = time.Now().Add(time.Second)

	fs.Create("/foo", "bar", expire, 1, 1)
	fs.Delete("/foo", false, 1, 1)

}

func TestTestAndSet(t *testing.T) {
	fs := New()
	fs.Create("/foo", "bar", Permanent, 1, 1)

	// test on wrong previous value
	_, err := fs.TestAndSet("/foo", "barbar", 0, "car", Permanent, 2, 1)
	if err == nil {
		t.Fatal("test and set should fail barbar != bar")
	}

	// test on value
	e, err := fs.TestAndSet("/foo", "bar", 0, "car", Permanent, 3, 1)

	if err != nil {
		t.Fatal("test and set should succeed bar == bar")
	}

	if e.PrevValue != "bar" || e.Value != "car" {
		t.Fatalf("[%v/%v] [%v/%v]", e.PrevValue, "bar", e.Value, "car")
	}

	// test on index
	e, err = fs.TestAndSet("/foo", "", 3, "bar", Permanent, 4, 1)

	if err != nil {
		t.Fatal("test and set should succeed index 3 == 3")
	}

	if e.PrevValue != "car" || e.Value != "bar" {
		t.Fatalf("[%v/%v] [%v/%v]", e.PrevValue, "car", e.Value, "bar")
	}
}

func createAndGet(fs *FileSystem, path string, t *testing.T) {
	_, err := fs.Create(path, "bar", Permanent, 1, 1)

	if err != nil {
		t.Fatalf("cannot create %s=bar [%s]", path, err.Error())
	}

	e, err := fs.Get(path, false, 1, 1)

	if err != nil {
		t.Fatalf("cannot get %s [%s]", path, err.Error())
	}

	if e.Value != "bar" {
		t.Fatalf("expect value of %s is bar [%s]", path, e.Value)
	}

}
