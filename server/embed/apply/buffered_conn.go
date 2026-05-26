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

package apply

import (
	"bufio"
	"net"
)

type BufferedConn struct {
	net.Conn
	r *bufio.Reader
}

func NewBufferedConn(conn net.Conn) *BufferedConn {
	return &BufferedConn{Conn: conn, r: bufio.NewReaderSize(conn, 1)}
}

func (bc *BufferedConn) Reader() *bufio.Reader {
	return bc.r
}

func (bc *BufferedConn) Read(p []byte) (int, error) {
	return bc.r.Read(p)
}
