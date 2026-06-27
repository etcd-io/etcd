package goenv

import (
	"errors"
	"os/exec"
	"strings"
)

func Read() (map[string]string, error) {
	// pass in a fixed set of var names to avoid needing to unescape output
	// pass in literals here instead of a variable list to avoid security linter warnings about command injection
	out, err := exec.Command("go", "env", "GOROOT", "GOPATH", "GOARCH", "GOOS", "CGO_ENABLED").CombinedOutput()
	if err != nil {
		return nil, err
	}
	return parseGoEnv([]string{"GOROOT", "GOPATH", "GOARCH", "GOOS", "CGO_ENABLED"}, out)
}

func parseGoEnv(varNames []string, data []byte) (map[string]string, error) {
	vars := make(map[string]string)

	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	for i, varName := range varNames {
		if i < len(lines) && len(lines[i]) > 0 {
			vars[varName] = lines[i]
		}
	}

	if len(vars) == 0 {
		return nil, errors.New("empty env set")
	}

	return vars, nil
}
