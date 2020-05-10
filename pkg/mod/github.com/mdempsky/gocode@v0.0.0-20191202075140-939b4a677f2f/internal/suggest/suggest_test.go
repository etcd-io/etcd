package suggest_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"go/importer"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mdempsky/gocode/internal/suggest"
)

var testDirFlag = flag.String("testdir", "", "specify a directory to run the test on")

func TestRegress(t *testing.T) {
	testDirs, err := filepath.Glob("testdata/test.*")
	if err != nil {
		t.Fatal(err)
	}
	if *testDirFlag != "" {
		t.Run(*testDirFlag, func(t *testing.T) {
			testRegress(t, "testdata/test."+*testDirFlag)
		})
	} else {
		for _, testDir := range testDirs {
			// Skip test.0011 for Go <= 1.11 because a method was added to reflect.Value.
			// TODO(rstambler): Change this when Go 1.12 comes out.
			if !strings.HasPrefix(runtime.Version(), "devel") && strings.HasSuffix(testDir, "test.0011") {
				continue
			}
			testDir := testDir // capture
			name := strings.TrimPrefix(testDir, "testdata/")
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				testRegress(t, testDir)
			})
		}
	}
}

func testRegress(t *testing.T, testDir string) {
	testDir, err := filepath.Abs(testDir)
	if err != nil {
		t.Errorf("Abs failed: %v", err)
		return
	}

	filename := filepath.Join(testDir, "test.go.in")
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		t.Errorf("ReadFile failed: %v", err)
		return
	}

	cursor := bytes.IndexByte(data, '@')
	if cursor < 0 {
		t.Errorf("Missing @")
		return
	}
	data = append(data[:cursor], data[cursor+1:]...)

	cfg := suggest.Config{
		Importer: importer.Default(),
		Logf:     func(string, ...interface{}) {},
	}
	cfg.Logf = func(string, ...interface{}) {}
	if testing.Verbose() {
		cfg.Logf = t.Logf
	}
	if cfgJSON, err := os.Open(filepath.Join(testDir, "config.json")); err == nil {
		if err := json.NewDecoder(cfgJSON).Decode(&cfg); err != nil {
			t.Errorf("Decode failed: %v", err)
			return
		}
	} else if !os.IsNotExist(err) {
		t.Errorf("Open failed: %v", err)
		return
	}
	candidates, prefixLen := cfg.Suggest(filename, data, cursor)

	var out bytes.Buffer
	suggest.NiceFormat(&out, candidates, prefixLen)
	want, _ := ioutil.ReadFile(filepath.Join(testDir, "out.expected"))
	if got := out.Bytes(); !bytes.Equal(got, want) {
		t.Errorf("%s:\nGot:\n%s\nWant:\n%s\n", testDir, got, want)
		return
	}
}

func contains(haystack []string, needle string) bool {
	for _, x := range haystack {
		if needle == x {
			return true
		}
	}
	return false
}
