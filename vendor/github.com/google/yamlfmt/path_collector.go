// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package yamlfmt

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/google/yamlfmt/internal/collections"
	"github.com/google/yamlfmt/internal/logger"
	ignore "github.com/sabhiram/go-gitignore"
)

type MatchType string

const (
	MatchTypeStandard   MatchType = "standard"
	MatchTypeDoublestar MatchType = "doublestar"
	MatchTypeGitignore  MatchType = "gitignore"
)

type PathCollector interface {
	CollectPaths() ([]string, error)
}

type FilepathCollector struct {
	Include    []string
	Exclude    []string
	Extensions []string
}

func (c *FilepathCollector) CollectPaths() ([]string, error) {
	logger.Debug(logger.DebugCodePaths, "using file path matching. include patterns: %s", c.Include)
	pathsFound := []string{}
	for _, inclPath := range c.Include {
		info, err := os.Stat(inclPath)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, err
			}
			continue
		}
		if !info.IsDir() {
			pathsFound = append(pathsFound, inclPath)
			continue
		}
		paths, err := c.walkDirectoryForYaml(inclPath)
		if err != nil {
			fmt.Printf("received errors walking %s:\n%v\n", inclPath, err)
		}
		pathsFound = append(pathsFound, paths...)
	}
	logger.Debug(logger.DebugCodePaths, "found paths: %s", pathsFound)

	pathsFoundSet := collections.SliceToSet(pathsFound)
	pathsToFormat := collections.SliceToSet(pathsFound)
	for _, exclPath := range c.Exclude {
		info, err := os.Stat(exclPath)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, err
			}
			continue
		}

		if info.IsDir() {
			logger.Debug(logger.DebugCodePaths, "for exclude dir: %s", exclPath)
			for foundPath := range pathsFoundSet {
				if strings.HasPrefix(foundPath, exclPath) {
					logger.Debug(logger.DebugCodePaths, "excluding %s", foundPath)
					pathsToFormat.Remove(foundPath)
				}
			}
		} else {
			logger.Debug(logger.DebugCodePaths, "for exclude file: %s", exclPath)
			removed := pathsToFormat.Remove(exclPath)
			if removed {
				logger.Debug(logger.DebugCodePaths, "found in paths, excluding")
			}
		}
	}

	pathsToFormatSlice := pathsToFormat.ToSlice()
	logger.Debug(logger.DebugCodePaths, "paths to format: %s", pathsToFormat)
	return pathsToFormatSlice, nil
}

func (c *FilepathCollector) walkDirectoryForYaml(dir string) ([]string, error) {
	var paths []string
	var walkErrs []error
	err := filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			walkErrs = append(
				walkErrs,
				fmt.Errorf("error for path %s: %v", path, err),
			)
			return nil
		}
		if info == nil {
			// Defensive programming, but this case should never hit.
			walkErrs = append(
				walkErrs,
				fmt.Errorf("error for path %s: %v", path, errors.New("error for pathnil file info, please report a GitHub issue if you see this failure")),
			)
			return nil
		}

		if info.IsDir() {
			return nil
		}

		if c.extensionMatches(info.Name()) {
			paths = append(paths, path)
		}

		return nil
	})
	// We join the errors for all paths and return that, so filepath.Walk's error should always be nil.
	// This is a defensive programming maneuver against that scenario that should cause yamlfmt
	// to surface the error and no-op for the provided path.
	if err != nil {
		return []string{}, err
	}
	return paths, errors.Join(walkErrs...)
}

func (c *FilepathCollector) extensionMatches(name string) bool {
	for _, ext := range c.Extensions {
		// Users may specify "yaml", but we only want to match ".yaml", not "buyaml".
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}

		if strings.HasSuffix(name, ext) {
			return true
		}
	}
	return false
}

type DoublestarCollector struct {
	Include []string
	Exclude []string
}

func (c *DoublestarCollector) CollectPaths() ([]string, error) {
	logger.Debug(logger.DebugCodePaths, "using doublestar path matching. include patterns: %s", c.Include)
	includedPaths := []string{}
	for _, pattern := range c.Include {
		logger.Debug(logger.DebugCodePaths, "trying pattern: %s", pattern)
		globMatches, err := doublestar.FilepathGlob(pattern)
		if err != nil {
			return nil, err
		}
		logger.Debug(logger.DebugCodePaths, "pattern %s matches: %s", pattern, globMatches)
		includedPaths = append(includedPaths, globMatches...)
	}

	pathsToFormatSet := collections.Set[string]{}
	for _, path := range includedPaths {
		if len(c.Exclude) == 0 {
			pathsToFormatSet.Add(path)
			continue
		}
		excluded := false
		logger.Debug(logger.DebugCodePaths, "calculating excludes for %s", path)
		for _, pattern := range c.Exclude {
			match, err := doublestar.PathMatch(filepath.Clean(pattern), path)
			if err != nil {
				return nil, err
			}
			if match {
				logger.Debug(logger.DebugCodePaths, "pattern %s matched, excluding", pattern)
				excluded = true
				break
			}
			logger.Debug(logger.DebugCodePaths, "pattern %s did not match path", pattern)
		}
		if !excluded {
			logger.Debug(logger.DebugCodePaths, "path %s included", path)
			pathsToFormatSet.Add(path)
		}
	}

	pathsToFormat := pathsToFormatSet.ToSlice()
	logger.Debug(logger.DebugCodePaths, "paths to format: %s", pathsToFormat)
	return pathsToFormat, nil
}

func findGitIgnorePath(gitignorePath string) (string, error) {
	// if path is absolute, check if exists and return
	if filepath.IsAbs(gitignorePath) {
		_, err := os.Stat(gitignorePath)
		return gitignorePath, err
	}

	// if path is relative, search for it until the git root
	dir, err := os.Getwd()
	if err != nil {
		return gitignorePath, fmt.Errorf("cannot get current working directory: %w", err)
	}
	for {
		// check if gitignore is there
		gitIgnore := filepath.Join(dir, gitignorePath)
		if _, err := os.Stat(gitIgnore); err == nil {
			return gitIgnore, nil
		}

		// check if we are at the git root directory
		gitRoot := filepath.Join(dir, ".git")
		if _, err := os.Stat(gitRoot); err == nil {
			return gitignorePath, errors.New("gitignore not found")
		}

		// check if we are at the root of the filesystem
		parent := filepath.Dir(dir)
		if parent == dir {
			return gitignorePath, errors.New("no git repository found")
		}

		// level up
		dir = parent
	}
}

func ExcludeWithGitignore(gitignorePath string, paths []string) ([]string, error) {
	gitignorePath, err := findGitIgnorePath(gitignorePath)
	if err != nil {
		return nil, err
	}
	logger.Debug(logger.DebugCodePaths, "excluding paths with gitignore: %s", gitignorePath)
	ignorer, err := ignore.CompileIgnoreFile(gitignorePath)
	if err != nil {
		return nil, err
	}
	pathsToFormat := []string{}
	for _, path := range paths {
		if ok, pattern := ignorer.MatchesPathHow(path); !ok {
			pathsToFormat = append(pathsToFormat, path)
		} else {
			logger.Debug(logger.DebugCodePaths, "pattern %s matches %s, excluding", pattern.Line, path)
		}
	}
	logger.Debug(logger.DebugCodePaths, "paths to format: %s", pathsToFormat)
	return pathsToFormat, nil
}

const DefaultPatternFile = "yamlfmt.patterns"

// PatternFileCollector determines which files to format and which to ignore based on a pattern file in gitignore(5) syntax.
type PatternFileCollector struct {
	fs      fs.FS
	matcher *ignore.GitIgnore
}

// NewPatternFileCollector initializes a new PatternFile using the provided file(s).
// If multiple files are provided, their content is concatenated in order.
// All patterns are relative to the current working directory.
func NewPatternFileCollector(files ...string) (*PatternFileCollector, error) {
	r, err := cat(files...)
	if err != nil {
		return nil, err
	}

	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("os.Getwd: %w", err)
	}

	return NewPatternFileCollectorFS(r, os.DirFS(wd)), nil
}

// cat concatenates the contents of all files in its argument list.
func cat(files ...string) (io.Reader, error) {
	var b bytes.Buffer

	for _, f := range files {
		fh, err := os.Open(f)
		if err != nil {
			return nil, err
		}
		defer fh.Close()

		if _, err := io.Copy(&b, fh); err != nil {
			return nil, fmt.Errorf("copying %q: %w", f, err)
		}
		fh.Close()

		// Append a newline to avoid issues with files lacking a newline at end-of-file.
		fmt.Fprintln(&b)
	}

	return &b, nil
}

// NewPatternFileCollectorFS reads a pattern file from r and uses fs for file lookups.
// It is used by NewPatternFile and primarily public because it is useful for testing.
func NewPatternFileCollectorFS(r io.Reader, fs fs.FS) *PatternFileCollector {
	var lines []string

	s := bufio.NewScanner(r)
	for s.Scan() {
		lines = append(lines, s.Text())
	}

	return &PatternFileCollector{
		fs:      fs,
		matcher: ignore.CompileIgnoreLines(lines...),
	}
}

// CollectPaths implements the PathCollector interface.
func (c *PatternFileCollector) CollectPaths() ([]string, error) {
	var files []string

	err := fs.WalkDir(c.fs, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		ok, pattern := c.matcher.MatchesPathHow(path)
		switch {
		case ok && pattern.Negate && d.IsDir():
			return fs.SkipDir
		case ok && pattern.Negate:
			return nil
		case ok && d.Type().IsRegular():
			files = append(files, path)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("WalkDir: %w", err)
	}

	return files, nil
}
