package filesearch

import (
	"os"
	"path/filepath"
	"strings"
)

// Find returns files based on search string arguments and filters.
func Find(cwd string, skipTests bool, args []string) []string {
	var (
		foundFiles    = []string{}
		filteredFiles = []string{}
	)

	for _, f := range args {
		if strings.HasSuffix(f, "/...") {
			dir, _ := filepath.Split(f)

			foundFiles = append(foundFiles, expandGoWildcard(dir)...)

			continue
		}

		if _, err := os.Stat(f); err == nil {
			foundFiles = append(foundFiles, f)
		}
	}

	// Use relative path to print shorter names, sort out test foundFiles if chosen.
	for _, f := range foundFiles {
		if skipTests {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
		}

		if relativePath, err := filepath.Rel(cwd, f); err == nil {
			filteredFiles = append(filteredFiles, relativePath)

			continue
		}

		filteredFiles = append(filteredFiles, f)
	}

	return filteredFiles
}

// expandGoWildcard path provided.
func expandGoWildcard(root string) []string {
	foundFiles := []string{}

	_ = filepath.Walk(root, func(path string, info os.FileInfo, _ error) error {
		// Only append go foundFiles.
		if !strings.HasSuffix(info.Name(), ".go") {
			return nil
		}

		foundFiles = append(foundFiles, path)

		return nil
	})

	return foundFiles
}
