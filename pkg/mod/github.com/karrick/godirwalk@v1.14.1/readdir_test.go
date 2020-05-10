package godirwalk

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadDirents(t *testing.T) {
	actual, err := ReadDirents(filepath.Join(testRoot, "d0"), nil)

	ensureError(t, err)

	expected := Dirents{
		&Dirent{
			name:     maxName,
			modeType: os.FileMode(0),
		},
		&Dirent{
			name:     "d1",
			modeType: os.ModeDir,
		},
		&Dirent{
			name:     "f1",
			modeType: os.FileMode(0),
		},
		&Dirent{
			name:     "skips",
			modeType: os.ModeDir,
		},
		&Dirent{
			name:     "symlinks",
			modeType: os.ModeDir,
		},
	}

	ensureDirentsMatch(t, actual, expected)
}

func TestReadDirentsSymlinks(t *testing.T) {
	osDirname := filepath.Join(testRoot, "d0/symlinks")

	actual, err := ReadDirents(osDirname, nil)

	ensureError(t, err)

	// Because some platforms set multiple mode type bits, when we create the
	// expected slice, we need to ensure the mode types are set appropriately.
	var expected Dirents
	for _, pathname := range []string{"nothing", "toAbs", "toD1", "toF1", "d4"} {
		info, err := os.Lstat(filepath.Join(osDirname, pathname))
		if err != nil {
			t.Fatal(err)
		}
		expected = append(expected, &Dirent{name: pathname, modeType: info.Mode() & os.ModeType})
	}

	ensureDirentsMatch(t, actual, expected)
}

func TestReadDirnames(t *testing.T) {
	actual, err := ReadDirnames(filepath.Join(testRoot, "d0"), nil)
	ensureError(t, err)
	expected := []string{maxName, "d1", "f1", "skips", "symlinks"}
	ensureStringSlicesMatch(t, actual, expected)
}

func BenchmarkReadDirnamesStandardLibrary(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark using user's Go source directory")
	}

	f := func(osDirname string) ([]string, error) {
		dh, err := os.Open(osDirname)
		if err != nil {
			return nil, err
		}
		return dh.Readdirnames(-1)
	}

	var count int

	for i := 0; i < b.N; i++ {
		actual, err := f(goPrefix)
		if err != nil {
			b.Fatal(err)
		}
		count = len(actual)
	}
	_ = count
}

func BenchmarkReadDirnamesThisLibrary(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark using user's Go source directory")
	}

	var count int

	for i := 0; i < b.N; i++ {
		actual, err := ReadDirnames(goPrefix, nil)
		if err != nil {
			b.Fatal(err)
		}
		count = len(actual)
	}
	_ = count
}
