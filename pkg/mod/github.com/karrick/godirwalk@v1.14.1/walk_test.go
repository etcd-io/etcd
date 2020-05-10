package godirwalk

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func filepathWalk(tb testing.TB, osDirname string) []string {
	tb.Helper()
	var entries []string
	err := filepath.Walk(osDirname, func(osPathname string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Name() == "skip" {
			return filepath.SkipDir
		}
		entries = append(entries, filepath.FromSlash(osPathname))
		return nil
	})
	ensureError(tb, err)
	return entries
}

func godirwalkWalk(tb testing.TB, osDirname string) []string {
	tb.Helper()
	var entries []string
	err := Walk(osDirname, &Options{
		Callback: func(osPathname string, dirent *Dirent) error {
			if dirent.Name() == "skip" {
				return filepath.SkipDir
			}
			entries = append(entries, filepath.FromSlash(osPathname))
			return nil
		},
	})
	ensureError(tb, err)
	return entries
}

func godirwalkWalkUnsorted(tb testing.TB, osDirname string) []string {
	tb.Helper()
	var entries []string
	err := Walk(osDirname, &Options{
		Callback: func(osPathname string, dirent *Dirent) error {
			if dirent.Name() == "skip" {
				return filepath.SkipDir
			}
			entries = append(entries, filepath.FromSlash(osPathname))
			return nil
		},
		Unsorted: true,
	})
	ensureError(tb, err)
	return entries
}

// Ensure the results from calling this library's Walk function exactly match
// those returned by filepath.Walk
func ensureSameAsStandardLibrary(tb testing.TB, osDirname string) {
	tb.Helper()
	osDirname = filepath.Join(testRoot, osDirname)
	actual := godirwalkWalk(tb, osDirname)
	sort.Strings(actual)
	expected := filepathWalk(tb, osDirname)
	ensureStringSlicesMatch(tb, actual, expected)
}

// Test the entire test root hierarchy with all of its artifacts.  This library
// advertises itself as visiting the same file system entries as the standard
// library, and responding to discovered errors the same way, including
// responding to filepath.SkipDir exactly like the standard library does.  This
// test ensures that behavior is correct by enumerating the contents of the test
// root directory.
func TestWalkCompatibleWithFilepathWalk(t *testing.T) {
	t.Run("test root", func(t *testing.T) {
		ensureSameAsStandardLibrary(t, "d0")
	})
	t.Run("ignore skips", func(t *testing.T) {
		// When filepath.SkipDir is returned, the remainder of the children in a
		// directory are not visited. This causes results to be different when
		// visiting in lexicographical order or natural order. For this test, we
		// want to ensure godirwalk can optimize traversals when unsorted using
		// the Scanner, but recognize that we cannot test against standard
		// library when we skip any nodes within it.
		osDirname := filepath.Join(testRoot, "d0/d1")
		actual := godirwalkWalkUnsorted(t, osDirname)
		sort.Strings(actual)
		expected := filepathWalk(t, osDirname)
		ensureStringSlicesMatch(t, actual, expected)
	})
}

// Test cases for encountering the filepath.SkipDir error at different
// relative positions from the invocation argument.
func TestWalkSkipDir(t *testing.T) {
	t.Run("skip file at root", func(t *testing.T) {
		ensureSameAsStandardLibrary(t, "d0/skips/d2")
	})

	t.Run("skip dir at root", func(t *testing.T) {
		ensureSameAsStandardLibrary(t, "d0/skips/d3")
	})

	t.Run("skip nodes under root", func(t *testing.T) {
		ensureSameAsStandardLibrary(t, "d0/skips")
	})

	t.Run("SkipDirOnSymlink", func(t *testing.T) {
		var actual []string
		err := Walk(filepath.Join(testRoot, "d0/skips"), &Options{
			Callback: func(osPathname string, dirent *Dirent) error {
				if dirent.Name() == "skip" {
					return filepath.SkipDir
				}
				actual = append(actual, filepath.FromSlash(osPathname))
				return nil
			},
			FollowSymbolicLinks: true,
		})

		ensureError(t, err)

		expected := []string{
			filepath.Join(testRoot, "d0/skips"),
			filepath.Join(testRoot, "d0/skips/d2"),
			filepath.Join(testRoot, "d0/skips/d2/f3"),
			filepath.Join(testRoot, "d0/skips/d3"),
			filepath.Join(testRoot, "d0/skips/d3/f4"),
			filepath.Join(testRoot, "d0/skips/d3/z2"),
		}

		ensureStringSlicesMatch(t, actual, expected)
	})
}

func TestWalkFollowSymbolicLinks(t *testing.T) {
	var actual []string
	var errorCallbackVisited bool

	err := Walk(filepath.Join(testRoot, "d0/symlinks"), &Options{
		Callback: func(osPathname string, _ *Dirent) error {
			actual = append(actual, filepath.FromSlash(osPathname))
			return nil
		},
		ErrorCallback: func(osPathname string, err error) ErrorAction {
			if filepath.Base(osPathname) == "nothing" {
				errorCallbackVisited = true
				return SkipNode
			}
			return Halt
		},
		FollowSymbolicLinks: true,
	})

	ensureError(t, err)

	if got, want := errorCallbackVisited, true; got != want {
		t.Errorf("GOT: %v; WANT: %v", got, want)
	}

	expected := []string{
		filepath.Join(testRoot, "d0/symlinks"),
		filepath.Join(testRoot, "d0/symlinks/d4"),
		filepath.Join(testRoot, "d0/symlinks/d4/toSD1"),    // chained symbolic link
		filepath.Join(testRoot, "d0/symlinks/d4/toSD1/f2"), // chained symbolic link
		filepath.Join(testRoot, "d0/symlinks/d4/toSF1"),    // chained symbolic link
		filepath.Join(testRoot, "d0/symlinks/nothing"),
		filepath.Join(testRoot, "d0/symlinks/toAbs"),
		filepath.Join(testRoot, "d0/symlinks/toD1"),
		filepath.Join(testRoot, "d0/symlinks/toD1/f2"),
		filepath.Join(testRoot, "d0/symlinks/toF1"),
	}

	ensureStringSlicesMatch(t, actual, expected)
}

// While filepath.Walk will deliver the no access error to the regular callback,
// godirwalk should deliver it first to the ErrorCallback handler, then take
// action based on the return value of that callback function.
func TestErrorCallback(t *testing.T) {
	t.Run("halt", func(t *testing.T) {
		var callbackVisited, errorCallbackVisited bool

		err := Walk(filepath.Join(testRoot, "d0/symlinks"), &Options{
			Callback: func(osPathname string, dirent *Dirent) error {
				switch dirent.Name() {
				case "nothing":
					callbackVisited = true
				}
				return nil
			},
			ErrorCallback: func(osPathname string, err error) ErrorAction {
				switch filepath.Base(osPathname) {
				case "nothing":
					errorCallbackVisited = true
					return Halt // Direct Walk to propagate error to caller
				}
				t.Fatalf("unexpected error callback for %s: %s", osPathname, err)
				return SkipNode
			},
			FollowSymbolicLinks: true,
		})

		ensureError(t, err, "nothing") // Ensure caller receives propagated access error
		if got, want := callbackVisited, true; got != want {
			t.Errorf("GOT: %v; WANT: %v", got, want)
		}
		if got, want := errorCallbackVisited, true; got != want {
			t.Errorf("GOT: %v; WANT: %v", got, want)
		}
	})

	t.Run("skipnode", func(t *testing.T) {
		var callbackVisited, errorCallbackVisited bool

		err := Walk(filepath.Join(testRoot, "d0/symlinks"), &Options{
			Callback: func(osPathname string, dirent *Dirent) error {
				switch dirent.Name() {
				case "nothing":
					callbackVisited = true
				}
				return nil
			},
			ErrorCallback: func(osPathname string, err error) ErrorAction {
				switch filepath.Base(osPathname) {
				case "nothing":
					errorCallbackVisited = true
					return SkipNode // Direct Walk to ignore this error
				}
				t.Fatalf("unexpected error callback for %s: %s", osPathname, err)
				return Halt
			},
			FollowSymbolicLinks: true,
		})

		ensureError(t, err) // Ensure caller receives no access error
		if got, want := callbackVisited, true; got != want {
			t.Errorf("GOT: %v; WANT: %v", got, want)
		}
		if got, want := errorCallbackVisited, true; got != want {
			t.Errorf("GOT: %v; WANT: %v", got, want)
		}
	})
}

// Invokes PostChildrenCallback for all directories and nothing else.
func TestPostChildrenCallback(t *testing.T) {
	var actual []string

	err := Walk(filepath.Join(testRoot, "d0"), &Options{
		Callback: func(_ string, _ *Dirent) error { return nil },
		PostChildrenCallback: func(osPathname string, _ *Dirent) error {
			actual = append(actual, osPathname)
			return nil
		},
	})

	ensureError(t, err)

	expected := []string{
		filepath.Join(testRoot, "d0"),
		filepath.Join(testRoot, "d0/d1"),
		filepath.Join(testRoot, "d0/skips"),
		filepath.Join(testRoot, "d0/skips/d2"),
		filepath.Join(testRoot, "d0/skips/d3"),
		filepath.Join(testRoot, "d0/skips/d3/skip"),
		filepath.Join(testRoot, "d0/symlinks"),
		filepath.Join(testRoot, "d0/symlinks/d4"),
	}

	ensureStringSlicesMatch(t, actual, expected)
}

const flameIterations = 10

var goPrefix = filepath.Join(os.Getenv("GOPATH"), "src")

func BenchmarkFilepathWalk(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark using user's Go source directory")
	}
	for i := 0; i < b.N; i++ {
		_ = filepathWalk(b, goPrefix)
	}
}

func BenchmarkGodirwalk(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark using user's Go source directory")
	}
	for i := 0; i < b.N; i++ {
		_ = godirwalkWalk(b, goPrefix)
	}
}

func BenchmarkGodirwalkUnsorted(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark using user's Go source directory")
	}
	for i := 0; i < b.N; i++ {
		_ = godirwalkWalkUnsorted(b, goPrefix)
	}
}

func BenchmarkFlameGraphFilepathWalk(b *testing.B) {
	for i := 0; i < flameIterations; i++ {
		_ = filepathWalk(b, goPrefix)
	}
}

func BenchmarkFlameGraphGodirwalk(b *testing.B) {
	for i := 0; i < flameIterations; i++ {
		_ = godirwalkWalk(b, goPrefix)
	}
}
