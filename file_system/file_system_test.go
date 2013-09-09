package fileSystem

import (
	"math/rand"
	"strconv"
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

	e, err := fs.Get("/fooDir", false, false, 3, 1)

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
		t.Fatalf("cannot create %s=bar [%s]", "/foo/bar", err.Error())
	}

	_, err = fs.Update("/foo/bar", "barbar", Permanent, 2, 1)

	if err != nil {
		t.Fatalf("cannot update %s=barbar [%s]", "/foo/bar", err.Error())
	}

	e, err := fs.Get("/foo/bar", false, false, 2, 1)

	if err != nil {
		t.Fatalf("cannot get %s [%s]", "/foo/bar", err.Error())
	}

	if e.Value != "barbar" {
		t.Fatalf("expect value of %s is barbar [%s]", "/foo/bar", e.Value)
	}

	// create a directory, update its ttl, to see if it will be deleted

	_, err = fs.Create("/foo/foo", "", Permanent, 3, 1)

	if err != nil {
		t.Fatalf("cannot create dir [%s] [%s]", "/foo/foo", err.Error())
	}

	_, err = fs.Create("/foo/foo/foo1", "bar1", Permanent, 4, 1)

	if err != nil {
		t.Fatal("cannot create [%s]", err.Error())
	}

	_, err = fs.Create("/foo/foo/foo2", "", Permanent, 5, 1)
	if err != nil {
		t.Fatal("cannot create [%s]", err.Error())
	}

	_, err = fs.Create("/foo/foo/foo2/boo", "boo1", Permanent, 6, 1)
	if err != nil {
		t.Fatal("cannot create [%s]", err.Error())
	}

	expire := time.Now().Add(time.Second * 2)
	_, err = fs.Update("/foo/foo", "", expire, 7, 1)
	if err != nil {
		t.Fatalf("cannot update dir [%s] [%s]", "/foo/foo", err.Error())
	}

	// sleep 50ms, it should still reach the node
	time.Sleep(time.Microsecond * 50)
	e, err = fs.Get("/foo/foo", true, false, 7, 1)

	if err != nil || e.Key != "/foo/foo" {
		t.Fatalf("cannot get dir before expiration [%s]", err.Error())
	}

	if e.KVPairs[0].Key != "/foo/foo/foo1" || e.KVPairs[0].Value != "bar1" {
		t.Fatalf("cannot get sub node before expiration [%s]", err.Error())
	}

	if e.KVPairs[1].Key != "/foo/foo/foo2" || e.KVPairs[1].Dir != true {
		t.Fatalf("cannot get sub dir before expiration [%s]", err.Error())
	}

	/*if e.KVPairs[2].Key != "/foo/foo/foo2/boo" || e.KVPairs[2].Value != "boo1" {
		t.Fatalf("cannot get sub node of sub dir before expiration [%s]", err.Error())
	}*/

	// wait for expiration
	time.Sleep(time.Second * 3)
	e, err = fs.Get("/foo/foo", true, false, 7, 1)

	if err == nil {
		t.Fatal("still can get dir after expiration [%s]")
	}

	_, err = fs.Get("/foo/foo/foo1", true, false, 7, 1)
	if err == nil {
		t.Fatal("still can get sub node after expiration [%s]")
	}

	_, err = fs.Get("/foo/foo/foo2", true, false, 7, 1)
	if err == nil {
		t.Fatal("still can get sub dir after expiration [%s]")
	}

	_, err = fs.Get("/foo/foo/foo2/boo", true, false, 7, 1)
	if err == nil {
		t.Fatalf("still can get sub node of sub dir after expiration [%s]", err.Error())
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

	e, err := fs.Get("/foo", true, false, 2, 1)

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

	e, _ = fs.Get("/foo", false, false, 2, 1)

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

	_, err = fs.Get("/foo", false, false, 1, 1)

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

	_, err = fs.Get("/foo", false, false, 1, 1)

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

	// test if we can reach the node before expiration
	expire = time.Now().Add(time.Second)
	fs.Create("/foo", "bar", expire, 1, 1)

	time.Sleep(time.Millisecond * 50)
	_, err = fs.InternalGet("/foo", 1, 1)

	if err != nil {
		t.Fatalf("cannot get the node before expiration", err.Error())
	}

	expire = time.Now().Add(time.Second)

	fs.Create("/foo", "bar", expire, 1, 1)
	_, err = fs.Delete("/foo", false, 1, 1)

	if err != nil {
		t.Fatalf("cannot delete the node before expiration", err.Error())
	}

}

func TestTestAndSet(t *testing.T) { // TODO prevValue == nil ?
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

	//e, err = fs.TestAndSet("/foo", )
}

func TestWatch(t *testing.T) {
	fs := New()
	// watch at a deeper path
	c, _ := fs.WatcherHub.watch("/foo/foo/foo", false, 0)
	fs.Create("/foo/foo/foo", "bar", Permanent, 1, 1)

	e := nonblockingRetrive(c)
	if e.Key != "/foo/foo/foo" {
		t.Fatal("watch for Create node fails")
	}

	c, _ = fs.WatcherHub.watch("/foo/foo/foo", false, 0)
	fs.Update("/foo/foo/foo", "car", Permanent, 2, 1)
	e = nonblockingRetrive(c)
	if e.Key != "/foo/foo/foo" {
		t.Fatal("watch for Update node fails")
	}

	c, _ = fs.WatcherHub.watch("/foo/foo/foo", false, 0)
	fs.TestAndSet("/foo/foo/foo", "car", 0, "bar", Permanent, 3, 1)
	e = nonblockingRetrive(c)
	if e.Key != "/foo/foo/foo" {
		t.Fatal("watch for TestAndSet node fails")
	}

	c, _ = fs.WatcherHub.watch("/foo/foo/foo", false, 0)
	fs.Delete("/foo", true, 4, 1) //recursively delete
	e = nonblockingRetrive(c)
	if e.Key != "/foo" {
		t.Fatal("watch for Delete node fails")
	}

	// watch at a prefix
	c, _ = fs.WatcherHub.watch("/foo", true, 0)
	fs.Create("/foo/foo/boo", "bar", Permanent, 5, 1)
	e = nonblockingRetrive(c)
	if e.Key != "/foo/foo/boo" {
		t.Fatal("watch for Create subdirectory fails")
	}

	c, _ = fs.WatcherHub.watch("/foo", true, 0)
	fs.Update("/foo/foo/boo", "foo", Permanent, 6, 1)
	e = nonblockingRetrive(c)
	if e.Key != "/foo/foo/boo" {
		t.Fatal("watch for Update subdirectory fails")
	}

	c, _ = fs.WatcherHub.watch("/foo", true, 0)
	fs.TestAndSet("/foo/foo/boo", "foo", 0, "bar", Permanent, 7, 1)
	e = nonblockingRetrive(c)
	if e.Key != "/foo/foo/boo" {
		t.Fatal("watch for TestAndSet subdirectory fails")
	}

	c, _ = fs.WatcherHub.watch("/foo", true, 0)
	fs.Delete("/foo/foo/boo", false, 8, 1)
	e = nonblockingRetrive(c)
	if e.Key != "/foo/foo/boo" {
		t.Fatal("watch for Delete subdirectory fails")
	}

}

func createAndGet(fs *FileSystem, path string, t *testing.T) {
	_, err := fs.Create(path, "bar", Permanent, 1, 1)

	if err != nil {
		t.Fatalf("cannot create %s=bar [%s]", path, err.Error())
	}

	e, err := fs.Get(path, false, false, 1, 1)

	if err != nil {
		t.Fatalf("cannot get %s [%s]", path, err.Error())
	}

	if e.Value != "bar" {
		t.Fatalf("expect value of %s is bar [%s]", path, e.Value)
	}

}

func nonblockingRetrive(c <-chan *Event) *Event {
	select {
	case e := <-c:
		return e
	default:
		return nil
	}
}

func TestSort(t *testing.T) {
	fs := New()

	// simulating random creation
	keys := GenKeys(80, 4)

	//t.Log(keys)
	i := uint64(1)
	for _, k := range keys {
		_, err := fs.Create(k, "bar", Permanent, i, 1)
		if err != nil {
			//t.Logf("create node[%s] failed %s", k, err.Error())
		} else {
			i++
		}
	}

	e, err := fs.Get("/foo", true, true, i, 1)
	if err != nil {
		t.Fatalf("get dir nodes failed [%s]", err.Error())
	}

	for i, k := range e.KVPairs[:len(e.KVPairs)-1] {
		//t.Log("root:")
		//t.Log(k)
		if k.Key >= e.KVPairs[i+1].Key {
			t.Fatalf("sort failed, [%s] should be placed after [%s]", k.Key, e.KVPairs[i+1].Key)
		}

		if k.Dir {
			recursiveTestSort(k, t)
		}

	}

	if k := e.KVPairs[len(e.KVPairs)-1]; k.Dir {
		recursiveTestSort(k, t)
	}
}

func recursiveTestSort(k KeyValuePair, t *testing.T) {
	//t.Log("recursive in")
	//t.Log(k)
	for i, v := range k.KVPairs[:len(k.KVPairs)-1] {
		if v.Key >= k.KVPairs[i+1].Key {
			t.Fatalf("sort failed, [%s] should be placed after [%s]", v.Key, k.KVPairs[i+1].Key)
		}

		if v.Dir {
			recursiveTestSort(v, t)
		}

	}

	if v := k.KVPairs[len(k.KVPairs)-1]; v.Dir {
		recursiveTestSort(v, t)
	}
}

// GenKeys randomly generate num of keys with max depth
func GenKeys(num int, depth int) []string {
	keys := make([]string, num)
	for i := 0; i < num; i++ {

		keys[i] = "/foo"
		depth := rand.Intn(depth) + 1

		for j := 0; j < depth; j++ {
			keys[i] += "/" + strconv.Itoa(rand.Int()%20)
		}
	}
	return keys
}
