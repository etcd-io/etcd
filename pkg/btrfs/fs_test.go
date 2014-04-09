package btrfs

import (
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestSetNOCOW(t *testing.T) {
	name, err := ioutil.TempDir(".", "etcdtest")
	if err != nil {
		t.Fatal("Failed creating temp dir")
	}
	defer os.Remove(name)

	if IsBtrfs(name) {
		SetNOCOWDir(name)
		cmd := exec.Command("lsattr", name)
		out, err := cmd.Output()
		if err != nil {
			t.Fatal("Failed executing lsattr")
		}
		if !strings.Contains(string(out), "---------------C") {
			t.Fatal("Failed setting NOCOW:\n", string(out))
		}
	}
}
