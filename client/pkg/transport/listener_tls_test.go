// Copyright 2017 The etcd Authors
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

package transport

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"
	"time"
)

func TestLruCache(t *testing.T) {
	file := "./crl.byte"
	defer func() {
		os.Remove(file)
	}()
	data := []byte("data")
	data2 := []byte("data")
	ioutil.WriteFile(file, data, 0666)
	crl, err := memoizeCRL(crlBytesLru, file, time.Millisecond*500)
	if err != nil {
		return
	}
	if !bytes.Equal(crl, data) {
		t.Error("cache invalid")
	}
	ioutil.WriteFile(file, data2, 0666)
	time.Sleep(time.Millisecond * 500)
	crl, err = memoizeCRL(crlBytesLru, file, time.Millisecond*500)
	if err != nil {
		return
	}
	if !bytes.Equal(crl, data2) {
		t.Error("cache invalid")
	}
}
