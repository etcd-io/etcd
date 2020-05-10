package godirwalk

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

// maxName is the tested maximum length of a filename this library will
// handle.  Previous attempts to set it to one less than the size of
// syscall.Dirent.Name array resulted in runtime errors trying to create
// a test scaffolding file whose size exceeded 255 bytes.  This filename
// is 255 characters long.
const maxName = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

// testRoot is the temporary directory root for scaffold directory.
var testRoot string

func TestMain(m *testing.M) {
	flag.Parse()

	var code int // program exit code

	// All tests use the same directory test scaffolding.  Create the directory
	// hierarchy, run the tests, then remove the root directory of the test
	// scaffolding.

	defer func() {
		if err := teardown(); err != nil {
			fmt.Fprintf(os.Stderr, "godirwalk teardown: %s\n", err)
			code = 1
		}
		os.Exit(code)
	}()

	// When cannot complete setup, dump the directory so we see what we have,
	// then bail.
	if err := setup(); err != nil {
		fmt.Fprintf(os.Stderr, "godirwalk setup: %s\n", err)
		dumpDirectory()
		code = 1
		return
	}

	code = m.Run()

	// When any test was a failure, then use standard library to walk test
	// scaffolding directory and print its contents.
	if code != 0 {
		dumpDirectory()
	}
}

func setup() error {
	var err error

	testRoot, err = ioutil.TempDir(os.TempDir(), "godirwalk-")
	if err != nil {
		return err
	}

	entries := []Creater{
		file{"d0/" + maxName},
		file{"d0/f0"},               // will be deleted after symlink for it created
		file{"d0/f1"},               //
		file{"d0/d1/f2"},            //
		file{"d0/skips/d2/f3"},      // node precedes skip
		file{"d0/skips/d2/skip"},    // skip is non-directory
		file{"d0/skips/d2/z1"},      // node follows skip non-directory: should never be visited
		file{"d0/skips/d3/f4"},      // node precedes skip
		file{"d0/skips/d3/skip/f5"}, // skip is directory: this node should never be visited
		file{"d0/skips/d3/z2"},      // node follows skip directory: should be visited

		link{"d0/symlinks/nothing", "../f0"},    // referent will be deleted
		link{"d0/symlinks/toF1", "../f1"},       //
		link{"d0/symlinks/toD1", "../d1"},       //
		link{"d0/symlinks/d4/toSD1", "../toD1"}, // chained symbolic links
		link{"d0/symlinks/d4/toSF1", "../toF1"}, // chained symbolic links
	}

	for _, entry := range entries {
		if err := entry.Create(); err != nil {
			return fmt.Errorf("cannot create scaffolding entry: %s", err)
		}
	}

	oldname, err := filepath.Abs(filepath.Join(testRoot, "d0/f1"))
	if err != nil {
		return fmt.Errorf("cannot create scaffolding entry: %s", err)
	}
	if err := (link{"d0/symlinks/toAbs", oldname}).Create(); err != nil {
		return fmt.Errorf("cannot create scaffolding entry: %s", err)
	}

	if err := os.Remove(filepath.Join(testRoot, "d0/f0")); err != nil {
		return fmt.Errorf("cannot remove file from test scaffolding: %s", err)
	}

	return nil
}

func teardown() error {
	if testRoot == "" {
		return nil // if we do not even have a test root directory then exit
	}
	if err := os.RemoveAll(testRoot); err != nil {
		return err
	}
	return nil
}

func dumpDirectory() {
	trim := len(testRoot) // trim rootDir from prefix of strings
	err := filepath.Walk(testRoot, func(osPathname string, info os.FileInfo, err error) error {
		if err != nil {
			// we have no info, so get it
			info, err2 := os.Lstat(osPathname)
			if err2 != nil {
				fmt.Fprintf(os.Stderr, "?--------- %s: %s\n", osPathname[trim:], err2)
			} else {
				fmt.Fprintf(os.Stderr, "%s %s: %s\n", info.Mode(), osPathname[trim:], err)
			}
			return nil
		}

		var suffix string

		if info.Mode()&os.ModeSymlink != 0 {
			referent, err := os.Readlink(osPathname)
			if err != nil {
				suffix = fmt.Sprintf(": cannot read symlink: %s", err)
				err = nil
			} else {
				suffix = fmt.Sprintf(" -> %s", referent)
			}
		}
		fmt.Fprintf(os.Stderr, "%s %s%s\n", info.Mode(), osPathname[trim:], suffix)
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot walk test directory: %s\n", err)
	}
}

////////////////////////////////////////
// helpers to create file system entries for test scaffolding

type Creater interface {
	Create() error
}

type file struct {
	name string
}

func (f file) Create() error {
	newname := filepath.Join(testRoot, filepath.FromSlash(f.name))
	if err := os.MkdirAll(filepath.Dir(newname), os.ModePerm); err != nil {
		return fmt.Errorf("cannot create directory for test scaffolding: %s", err)
	}
	if err := ioutil.WriteFile(newname, []byte(newname+"\n"), os.ModePerm); err != nil {
		return fmt.Errorf("cannot create file for test scaffolding: %s", err)
	}
	return nil
}

type link struct {
	name, referent string
}

func (s link) Create() error {
	newname := filepath.Join(testRoot, filepath.FromSlash(s.name))
	if err := os.MkdirAll(filepath.Dir(newname), os.ModePerm); err != nil {
		return fmt.Errorf("cannot create directory for test scaffolding: %s", err)
	}
	oldname := filepath.FromSlash(s.referent)
	if err := os.Symlink(oldname, newname); err != nil {
		return fmt.Errorf("cannot create symbolic link for test scaffolding: %s", err)
	}
	return nil
}
