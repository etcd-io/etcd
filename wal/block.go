/*
Copyright 2014 CoreOS Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package wal

import (
	"fmt"
	"io"
)

type block struct {
	t int64
	d []byte
}

func writeBlock(w io.Writer, t int64, d []byte) error {
	if err := writeInt64(w, t); err != nil {
		return err
	}
	if err := writeInt64(w, int64(len(d))); err != nil {
		return err
	}
	_, err := w.Write(d)
	return err
}

func readBlock(r io.Reader, b *block) error {
	t, err := readInt64(r)
	if err != nil {
		return err
	}
	l, err := readInt64(r)
	if err != nil {
		return unexpectedEOF(err)
	}
	d := make([]byte, l)
	n, err := r.Read(d)
	if err != nil {
		return unexpectedEOF(err)
	}
	if n != int(l) {
		return fmt.Errorf("len(data) = %d, want %d", n, l)
	}
	b.t = t
	b.d = d
	return nil
}
