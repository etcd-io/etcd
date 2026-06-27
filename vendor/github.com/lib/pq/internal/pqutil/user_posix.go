//go:build !windows && !js && !android && !hurd && !zos && !wasip1 && !appengine

package pqutil

import (
	"os"
	"os/user"
	"runtime"
)

func User() (string, error) {
	env := "USER"
	if runtime.GOOS == "plan9" {
		env = "user"
	}
	if n := os.Getenv(env); n != "" {
		return n, nil
	}

	u, err := user.Current()
	if err != nil {
		return "", err
	}
	return u.Username, nil
}
