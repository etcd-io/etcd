package fileutil

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"testing"
	"time"
)

func TestPurgeFile(t *testing.T) {
	dir, err := ioutil.TempDir("", "purgefile")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	for i := 0; i < 5; i++ {
		_, err := os.Create(path.Join(dir, fmt.Sprintf("%d.test", i)))
		if err != nil {
			t.Fatal(err)
		}
	}

	stop := make(chan struct{})
	errch := PurgeFile(dir, "test", 3, time.Millisecond, stop)
	for i := 5; i < 10; i++ {
		_, err := os.Create(path.Join(dir, fmt.Sprintf("%d.test", i)))
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond)
	}
	fnames, err := ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	wnames := []string{"7.test", "8.test", "9.test"}
	if !reflect.DeepEqual(fnames, wnames) {
		t.Errorf("filenames = %v, want %v", fnames, wnames)
	}
	select {
	case err := <-errch:
		t.Errorf("unexpected purge error %v", err)
	case <-time.After(time.Millisecond):
	}
	close(stop)
}
