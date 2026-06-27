// Copyright 2016 Google Inc. All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to writing, software distributed
// under the License is distributed on a "AS IS" BASIS, WITHOUT WARRANTIES OR
// CONDITIONS OF ANY KIND, either express or implied.
//
// See the License for the specific language governing permissions and
// limitations under the License.

package embedmd

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"
)

// Fetcher provides an abstraction on a file system.
// The Fetch function is called anytime some content needs to be fetched.
// For now this includes files and URLs.
// The first parameter is the base directory that could be used to resolve
// relative paths. This base directory will be ignored for absolute paths,
// such as URLs.
type Fetcher interface {
	Fetch(dir, path string) ([]byte, error)
}

type fetcher struct{}

func (fetcher) Fetch(dir, path string) ([]byte, error) {
	if !strings.HasPrefix(path, "http://") && !strings.HasPrefix(path, "https://") {
		path = filepath.Join(dir, filepath.FromSlash(path))
		return ioutil.ReadFile(path)
	}

	res, err := http.Get(path)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %s", res.Status)
	}
	return ioutil.ReadAll(res.Body)
}
