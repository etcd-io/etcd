package fileSystem

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	etcdErr "github.com/coreos/etcd/error"
)

type FileSystem struct {
	Root         *Node
	EventHistory *EventHistory
	WatcherHub   *watcherHub
	Index        uint64
	Term         uint64
}

func New() *FileSystem {
	return &FileSystem{
		Root:       newDir("/", 0, 0, nil, ""),
		WatcherHub: newWatchHub(1000),
	}

}

func (fs *FileSystem) Get(path string, recusive bool, index uint64, term uint64) (*Event, error) {
	// TODO: add recursive get
	n, err := fs.InternalGet(path, index, term)

	if err != nil {
		return nil, err
	}

	e := newEvent(Get, path, index, term)

	if n.IsDir() { // node is dir
		e.Pairs = make([]KeyValuePair, len(n.Children))

		i := 0

		for _, subN := range n.Children {

			if subN.IsHidden() { // get will not list hidden node
				continue
			}

			if subN.IsDir() {
				e.Pairs[i] = KeyValuePair{
					Key: subN.Path,
					Dir: true,
				}
			} else {
				e.Pairs[i] = KeyValuePair{
					Key:   subN.Path,
					Value: subN.Value,
				}
			}

			i++
		}

	} else { // node is file
		e.Value = n.Value
	}

	return e, nil
}

func (fs *FileSystem) Set(path string, value string, expireTime time.Time, index uint64, term uint64) (*Event, error) {
	path = filepath.Clean("/" + path)

	// update file system known index and term
	fs.Index, fs.Term = index, term

	dir, name := filepath.Split(path)

	// walk through the path and get the last directory node
	d, err := fs.walk(dir, fs.checkDir)

	if err != nil {
		return nil, err
	}

	f := newFile(name, value, fs.Index, fs.Term, d, "", expireTime)
	e := newEvent(Set, path, fs.Index, fs.Term)
	e.Value = f.Value

	// remove previous file if exist
	oldFile, err := d.GetFile(name)

	if err == nil {
		if oldFile != nil {
			oldFile.Remove(false)
			e.PrevValue = oldFile.Value
		}
	} else {
		return nil, err
	}

	err = d.Add(f)

	if err != nil {
		return nil, err
	}

	// Node with TTL
	if expireTime != Permanent {
		go f.Expire()
		e.Expiration = &f.ExpireTime
		e.TTL = int64(expireTime.Sub(time.Now()) / time.Second)
	}

	return e, nil
}

func (fs *FileSystem) TestAndSet(path string, recurisive bool, index uint64, term uint64) {

}

func (fs *FileSystem) TestIndexAndSet() {

}

func (fs *FileSystem) Delete(path string, recurisive bool, index uint64, term uint64) (*Event, error) {
	n, err := fs.InternalGet(path, index, term)

	if err != nil {
		return nil, err
	}

	err = n.Remove(recurisive)

	if err != nil {
		return nil, err
	}

	e := newEvent(Delete, path, index, term)

	if n.IsDir() {
		e.Dir = true
	} else {
		e.PrevValue = n.Value
	}

	return e, nil
}

// walk function walks all the path and apply the walkFunc on each directory
func (fs *FileSystem) walk(path string, walkFunc func(prev *Node, component string) (*Node, error)) (*Node, error) {
	components := strings.Split(path, "/")

	curr := fs.Root

	var err error
	for i := 1; i < len(components); i++ {
		if len(components[i]) == 0 { // ignore empty string
			return curr, nil
		}

		curr, err = walkFunc(curr, components[i])
		if err != nil {
			return nil, err
		}

	}

	return curr, nil
}

// InternalGet function get the node of the given path.
func (fs *FileSystem) InternalGet(path string, index uint64, term uint64) (*Node, error) {
	fmt.Println("GET: ", path)
	path = filepath.Clean("/" + path)

	// update file system known index and term
	fs.Index, fs.Term = index, term

	walkFunc := func(parent *Node, dirName string) (*Node, error) {
		child, ok := parent.Children[dirName]
		if ok {
			return child, nil
		}

		return nil, etcdErr.NewError(100, "get")
	}

	f, err := fs.walk(path, walkFunc)

	if err != nil {
		return nil, err
	}

	return f, nil
}

// checkDir function will check whether the component is a directory under parent node.
// If it is a directory, this function will return the pointer to that node.
// If it does not exist, this function will create a new directory and return the pointer to that node.
// If it is a file, this function will return error.
func (fs *FileSystem) checkDir(parent *Node, dirName string) (*Node, error) {

	subDir, ok := parent.Children[dirName]

	if ok {
		return subDir, nil
	}

	n := newDir(filepath.Join(parent.Path, dirName), fs.Index, fs.Term, parent, parent.ACL)

	parent.Children[dirName] = n

	return n, nil
}
