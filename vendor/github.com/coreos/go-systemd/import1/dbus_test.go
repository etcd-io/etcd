// Copyright 2019 CoreOS, Inc.
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

package import1

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	importPrefix = "importd-test-"
)

func TestImportTar(t *testing.T) {
	conn, err := New()
	if err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(findFixture("image.tar.xz", t))
	if err != nil {
		t.Fatal(err)
	}

	_, err = conn.ImportTar(f, importPrefix+"ImportTar", true, true)
	if err != nil {
		t.Fatal(err)
	}
}

func TestImportRaw(t *testing.T) {
	conn, err := New()
	if err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(findFixture("image.raw.xz", t))
	if err != nil {
		t.Fatal(err)
	}

	_, err = conn.ImportRaw(f, importPrefix+"ImportRaw", true, true)
	if err != nil {
		t.Fatal(err)
	}
}

func TestExportTar(t *testing.T) {
	conn, err := New()
	if err != nil {
		t.Fatal(err)
	}

	err = os.MkdirAll("tmp/", os.ModeDir|0755)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Create("tmp/image-export.tar.xz")
	if err != nil {
		t.Fatal(err)
	}

	_, err = conn.ExportTar(importPrefix+"ImportTar", f, "xz")
	if err != nil {
		t.Fatal(err)
	}
}

func TestExportRaw(t *testing.T) {
	conn, err := New()
	if err != nil {
		t.Fatal(err)
	}

	err = os.MkdirAll("tmp/", os.ModeDir|0755)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Create("tmp/image-export.raw.xz")
	if err != nil {
		t.Fatal(err)
	}

	_, err = conn.ExportRaw(importPrefix+"ImportRaw", f, "xz")
	if err != nil {
		t.Fatal(err)
	}
}

func TestPullTar(t *testing.T) {
	conn, err := New()
	if err != nil {
		t.Fatal(err)
	}

	_, err = conn.PullTar("http://127.0.0.1:8080/image.tar.xz", importPrefix+"PullTar", "no", true)
	if err != nil {
		t.Fatal(err)
	}
}
func TestPullRaw(t *testing.T) {
	conn, err := New()
	if err != nil {
		t.Fatal(err)
	}

	_, err = conn.PullRaw("http://127.0.0.1:8080/image.raw.xz", importPrefix+"PullRaw", "no", true)
	if err != nil {
		t.Fatal(err)
	}
}

func TestListAndCancelTransfers(t *testing.T) {
	conn, err := New()
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		_, _ = conn.PullTar("http://127.0.0.1:8080/image.tar.xz", importPrefix+"ListAndCancelTransfers", "no", true)
		_, _ = conn.PullTar("http://127.0.0.1:8080/image.raw.xz", importPrefix+"ListAndCancelTransfers", "no", true)
	}()

	transfers, err := conn.ListTransfers()
	if err != nil {
		t.Error(err)
	}
	if len(transfers) < 1 {
		t.Error("transfers length is not correct")
	}

	for _, v := range transfers {
		err = conn.CancelTransfer(v.Id)
		if err != nil {
			// Let's just ignore the transfer id not found error.
			if strings.Contains(err.Error(), "No transfer by id") {
				continue
			}
			t.Error(err)
		}
	}
}

func findFixture(target string, t *testing.T) string {
	abs, err := filepath.Abs("../fixtures/" + target)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

func init() {
	go func() {
		err := http.ListenAndServe(":8080", http.FileServer(http.Dir("../fixtures")))
		if err != nil {
			log.Fatal(err)
		}
	}()
}
