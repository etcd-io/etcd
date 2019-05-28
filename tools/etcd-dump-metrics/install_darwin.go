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

// +build darwin

package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"go.etcd.io/etcd/pkg/fileutil"
)

const downloadURL = `https://storage.googleapis.com/etcd/%s/etcd-%s-darwin-amd64.zip`

func install(ver, dir string) (string, error) {
	ep := fmt.Sprintf(downloadURL, ver, ver)

	resp, err := http.Get(ep)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	d, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	zipPath := filepath.Join(dir, "etcd.zip")
	if err = ioutil.WriteFile(zipPath, d, fileutil.PrivateFileMode); err != nil {
		return "", err
	}

	if err = exec.Command("bash", "-c", fmt.Sprintf("unzip %s -d %s", zipPath, dir)).Run(); err != nil {
		return "", err
	}

	bp1 := filepath.Join(dir, fmt.Sprintf("etcd-%s-darwin-amd64", ver), "etcd")
	bp2 := filepath.Join(dir, "etcd")
	return bp2, os.Rename(bp1, bp2)
}
