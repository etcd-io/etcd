// Copyright 2015 CoreOS, Inc.
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

package fileutil

import (
	"log"
	"os"
	"path"
	"sort"
	"strings"
	"time"
)

func PurgeFile(dirname string, suffix string, max uint, interval time.Duration, stop <-chan struct{}) <-chan error {
	errC := make(chan error, 1)
	go func() {
		for {
			fnames, err := ReadDir(dirname)
			if err != nil {
				errC <- err
				return
			}
			newfnames := make([]string, 0)
			for _, fname := range fnames {
				if strings.HasSuffix(fname, suffix) {
					newfnames = append(newfnames, fname)
				}
			}
			sort.Strings(newfnames)
			for len(newfnames) > int(max) {
				f := path.Join(dirname, newfnames[0])
				l, err := NewLock(f)
				if err != nil {
					errC <- err
					return
				}
				err = l.TryLock()
				if err != nil {
					break
				}
				err = os.Remove(f)
				if err != nil {
					errC <- err
					return
				}
				err = l.Unlock()
				if err != nil {
					log.Printf("filePurge: unlock %s error %v", l.Name(), err)
				}
				err = l.Destroy()
				if err != nil {
					log.Printf("filePurge: destroy lock %s error %v", l.Name(), err)
				}
				log.Printf("filePurge: successfully removed file %s", f)
				newfnames = newfnames[1:]
			}
			select {
			case <-time.After(interval):
			case <-stop:
				return
			}
		}
	}()
	return errC
}
