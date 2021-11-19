// Copyright 2018 The etcd Authors
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

//go:build linux
// +build linux

package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"go.etcd.io/etcd/client/pkg/v3/fileutil"
)

const downloadURL = `https://storage.googleapis.com/etcd/%s/etcd-%s-linux-amd64.tar.gz`

func install(ver, dir string) (string, error) {
	ep := fmt.Sprintf(downloadURL, ver, ver)

	resp, err := http.Get(ep)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	d, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	tarPath := filepath.Join(dir, "etcd.tar.gz")
	if err = os.WriteFile(tarPath, d, fileutil.PrivateFileMode); err != nil {
		return "", err
	}

	// parametrizes to prevent attackers from adding arbitrary OS commands
	if err = exec.Command("tar", "xzvf", tarPath, "-C", dir, "--strip-components=1").Run(); err != nil {
		return "", err
	}
	return filepath.Join(dir, "etcd"), nil
}
