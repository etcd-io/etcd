package rule

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mgechev/revive/lint"
)

// PackageDirectoryMismatchRule detects when package name doesn't match directory name.
type PackageDirectoryMismatchRule struct {
	ignoredDirs *regexp.Regexp
}

const defaultIgnoredDirs = "testdata"

// Configure the rule to exclude certain directories.
func (r *PackageDirectoryMismatchRule) Configure(arguments lint.Arguments) error {
	if len(arguments) < 1 {
		var err error
		r.ignoredDirs, err = r.buildIgnoreRegex([]string{defaultIgnoredDirs})
		return err
	}

	args, ok := arguments[0].(map[string]any)
	if !ok {
		return fmt.Errorf("invalid argument type: expected map[string]any, got %T", arguments[0])
	}

	for k, v := range args {
		if !isRuleOption(k, "ignoreDirectories") {
			return fmt.Errorf("unknown argument %s for %s rule", k, r.Name())
		}

		ignoredAny, ok := v.([]any)
		if !ok {
			return fmt.Errorf("invalid value %v for argument %s of rule %s, expected []string got %T", v, k, r.Name(), v)
		}

		ignoredDirs := make([]string, len(ignoredAny))
		for i, item := range ignoredAny {
			str, ok := item.(string)
			if !ok {
				return fmt.Errorf("invalid value in %s argument of rule %s: expected string, got %T", k, r.Name(), item)
			}
			ignoredDirs[i] = str
		}

		var err error
		r.ignoredDirs, err = r.buildIgnoreRegex(ignoredDirs)
		return err
	}

	return nil
}

func (*PackageDirectoryMismatchRule) buildIgnoreRegex(ignoredDirs []string) (*regexp.Regexp, error) {
	if len(ignoredDirs) == 0 {
		return nil, nil
	}

	patterns := make([]string, len(ignoredDirs))
	for i, dir := range ignoredDirs {
		patterns[i] = regexp.QuoteMeta(dir)
	}
	pattern := strings.Join(patterns, "|")

	regex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to compile regex for ignored directories: %w", err)
	}

	return regex, nil
}

// skipDirs contains directory names that should be unconditionally ignored when checking.
// These entries handle edge cases where [filepath.Base] might return these values.
var skipDirs = map[string]struct{}{
	".": {}, // Current directory
	"/": {}, // Root directory
	"":  {}, // Empty path
}

// semanticallyEqual checks if package and directory names are semantically equal to each other.
func (*PackageDirectoryMismatchRule) semanticallyEqual(packageName, dirName string) bool {
	normDir := normalizePath(dirName)
	normPkg := normalizePath(packageName)
	return normDir == normPkg || normDir == "go"+normPkg
}

// Apply applies the rule to the given file.
func (r *PackageDirectoryMismatchRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	if file.Pkg.IsMain() {
		return nil
	}

	absPath, err := filepath.Abs(file.Name)
	if err != nil {
		return nil
	}

	dirPath := filepath.Dir(absPath)
	dirName := filepath.Base(dirPath)

	if r.ignoredDirs != nil && r.ignoredDirs.MatchString(dirPath) {
		return nil
	}

	// Check if we got an invalid directory.
	if _, skipDir := skipDirs[dirName]; skipDir {
		return nil
	}

	// Files directly in 'internal/' (like 'internal/abcd.go') should not be checked.
	// But files in subdirectories of 'internal/' (like 'internal/foo/abcd.go') should be checked.
	if dirName == "internal" {
		return nil
	}

	packageName := file.AST.Name.Name

	if r.semanticallyEqual(packageName, dirName) {
		return nil
	}

	if isRootDir(dirPath) {
		return nil
	}

	if file.IsTest() {
		// treat main_test differently because it's a common package name for tests
		if packageName == "main_test" {
			return nil
		}
		// External test package (directory + '_test' suffix)
		if r.semanticallyEqual(packageName, dirName+"_test") {
			return nil
		}
	}

	// define a default failure message
	failure := fmt.Sprintf("package name %q does not match directory name %q", packageName, dirName)

	// For version directories (v1, v2, etc.), we need to check also the parent directory
	if isVersionPath(dirName) {
		parentDirName := filepath.Base(filepath.Dir(dirPath))
		if r.semanticallyEqual(packageName, parentDirName) {
			return nil
		}

		if file.IsTest() {
			// External test package (directory + '_test' suffix)
			if r.semanticallyEqual(packageName, parentDirName+"_test") {
				return nil
			}
		}

		failure = fmt.Sprintf("package name %q does not match directory name %q or parent directory name %q", packageName, dirName, parentDirName)
	}

	return []lint.Failure{
		{
			Failure:    failure,
			Confidence: 1,
			Node:       file.AST.Name,
			Category:   lint.FailureCategoryNaming,
		},
	}
}

// isRootDir checks if the given directory contains go.mod or .git, indicating it's a root directory.
func isRootDir(dirPath string) bool {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return false
	}

	for _, e := range entries {
		switch e.Name() {
		case "go.mod", ".git":
			return true
		}
	}

	return false
}

// Name returns the rule name.
func (*PackageDirectoryMismatchRule) Name() string {
	return "package-directory-mismatch"
}
