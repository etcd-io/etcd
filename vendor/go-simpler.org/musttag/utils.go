package musttag

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

func getMainModule() (string, error) {
	args := [...]string{"go", "mod", "edit", "-json"}

	out, err := exec.Command(args[0], args[1:]...).Output()
	if err != nil {
		return "", fmt.Errorf("running %q: %w", strings.Join(args[:], " "), err)
	}

	var info struct {
		Module struct {
			Path string `json:"Path"`
		} `json:"Module"`
	}
	if err := json.Unmarshal(out, &info); err != nil {
		return "", fmt.Errorf("decoding module info: %w\n%s", err, out)
	}

	return info.Module.Path, nil
}

// based on golang.org/x/tools/imports.VendorlessPath
func cutVendor(path string) string {
	var prefix string

	switch {
	case strings.HasPrefix(path, "(*"):
		prefix, path = "(*", path[len("(*"):]
	case strings.HasPrefix(path, "("):
		prefix, path = "(", path[len("("):]
	}

	if i := strings.LastIndex(path, "/vendor/"); i >= 0 {
		return prefix + path[i+len("/vendor/"):]
	}
	if strings.HasPrefix(path, "vendor/") {
		return prefix + path[len("vendor/"):]
	}

	return prefix + path
}
