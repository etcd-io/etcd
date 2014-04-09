package fs

import (
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestSetNOCOW(t *testing.T) {
	f, err := ioutil.TempFile(".", "etcdtest")
	if err != nil {
		t.Fatal("Failed creating temp file")
	}
	f.Close()
	defer os.Remove(f.Name())

	if IsBtrfs(f.Name()) {
		SetNOCOW(f.Name())
		cmd := exec.Command("lsattr", f.Name())
		out, err := cmd.Output()
		if err != nil {
			t.Fatal("Failed executing lsattr")
		}
		if !strings.Contains(string(out), "---------------C") {
			t.Fatal("Failed setting NOCOW:\n", string(out))
		}
	}
}
