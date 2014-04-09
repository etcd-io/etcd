package fs

import (
	"os/exec"
	"strings"
	"testing"
)

func TestSetNOCOW(t *testing.T) {
	if IsBtrfs("/") {
		SetNOCOW("/")
		cmd := exec.Command("lsattr", "/")
		out, err := cmd.Output()
		if err != nil {
			t.Fatal("Failed executing lsattr")
		}
		if strings.Contains(string(out), "---------------C") {
			t.Fatal("Failed setting NOCOW:\n", out)
		}
	}
}
