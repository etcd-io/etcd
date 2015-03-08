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

package transport

import (
	"github.com/dchest/spipe"
	"io/ioutil"
	"net"
	"time"
)

type rwTimeoutDialer struct {
	wtimeoutd  time.Duration
	rdtimeoutd time.Duration
	psk        string
	net.Dialer
}

func (d *rwTimeoutDialer) Dial(network, address string) (net.Conn, error) {
	if d.psk != "" {
		psk, err := ioutil.ReadFile(d.psk)
		if err != nil {
			return nil, err
		}
		conn, err := spipe.Dial(psk, network, address)
		tconn := &timeoutConn{
			rdtimeoutd: d.rdtimeoutd,
			wtimeoutd:  d.wtimeoutd,
			Conn:       conn,
		}
		return tconn, err
	}
	conn, err := d.Dialer.Dial(network, address)
	tconn := &timeoutConn{
		rdtimeoutd: d.rdtimeoutd,
		wtimeoutd:  d.wtimeoutd,
		Conn:       conn,
	}
	return tconn, err
}
