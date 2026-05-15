// Copyright 2026 The etcd Authors
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

package etcdutl

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestNewVersionCommand_Metadata(t *testing.T) {
	cmd := NewVersionCommand()

	if cmd.Use != "version" {
		t.Fatalf("unexpected Use: got %q, want %q", cmd.Use, "version")
	}
	if cmd.Short != "Prints the version of etcdutl" {
		t.Fatalf("unexpected Short: got %q, want %q", cmd.Short, "Prints the version of etcdutl")
	}
	if cmd.Run == nil {
		t.Fatal("unexpected nil Run")
	}
}

func TestVersionCommandFunc_PrintsVersionAndAPIVersion(t *testing.T) {
	cmd := NewVersionCommand()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	defer r.Close()

	origStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()

	versionCommandFunc(cmd, nil)

	if err = w.Close(); err != nil {
		t.Fatalf("failed to close pipe writer: %v", err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read captured stdout: %v", err)
	}

	output := string(out)
	if !strings.Contains(output, "etcdutl version:") {
		t.Fatalf("output missing etcdutl version line: %q", output)
	}
	if !strings.Contains(output, "API version:") {
		t.Fatalf("output missing API version line: %q", output)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 {
		t.Fatalf("unexpected line count: got %d, want 2, output=%q", len(lines), output)
	}
}
