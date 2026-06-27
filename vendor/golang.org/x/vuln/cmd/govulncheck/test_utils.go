// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"
)

// copyTestCase copies the test case at dir into a
// temporary directory. The created files have 0644
// permission and directories 0755. It does not create
// symlinks.
func copyTestCase(dir string, t *testing.T) string {
	newDir, err := filepath.Abs(t.TempDir())
	if err != nil {
		t.Fatalf("failed to copy test case %s: cannot create root %v", dir, err)
	}

	if err := copyDir(dir, newDir); err != nil {
		t.Fatalf("failed to copy test case %s: copy failure %v", dir, err)
	}
	return newDir
}

func copyDir(srcDir, destDir string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		src := filepath.Join(srcDir, entry.Name())
		dest := filepath.Join(destDir, entry.Name())

		fileInfo, err := os.Stat(src)
		if err != nil {
			return err
		}

		switch fileInfo.Mode() & os.ModeType {
		case os.ModeDir:
			if err := os.MkdirAll(dest, 0755); err != nil {
				return err
			}
			if err := copyDir(src, dest); err != nil {
				return err
			}
		default:
			if err := copyFile(src, dest); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyFile(src, dest string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dest, b, 0644)
}

type config struct {
	// SkipGOOS is a list of GOOS to skip
	SkipGOOS []string `json:"skipGOOS,omitempty"`
	// Copy the folder to isolate it
	Copy bool `json:"copy,omitempty"`
	// SkipBuild the test case
	SkipBuild bool `json:"skipBuild,omitempty"`
	// Strip indicates if binaries should be stripped
	Strip bool `json:"strip,omitempty"`
	// EnableSBOM indicates if sbom should be
	// printed in JSON.
	EnableSBOM bool `json:"sbom,omitempty"`

	Fixups []fixup `json:"fixups,omitempty"`
}

func (c *config) skip() bool {
	for _, sg := range c.SkipGOOS {
		if runtime.GOOS == sg {
			return true
		}
	}
	return false
}

type fixup struct {
	Pattern     string `json:"pattern,omitempty"`
	Replace     string `json:"replace,omitempty"`
	compiled    *regexp.Regexp
	replaceFunc func(b []byte) []byte
}

func (f *fixup) init() {
	f.compiled = regexp.MustCompile(f.Pattern)
}

func (f *fixup) apply(data []byte) []byte {
	if f.replaceFunc != nil {
		return f.compiled.ReplaceAllFunc(data, f.replaceFunc)
	}
	return f.compiled.ReplaceAll(data, []byte(f.Replace))
}

// loadConfig loads and initializes the config from path.
func loadConfig(path string) (*config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}
	for i := range cfg.Fixups {
		cfg.Fixups[i].init()
	}
	return &cfg, nil
}
