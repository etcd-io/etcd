/*
   Copyright 2014 CoreOS, Inc.

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

package netutil

import (
	"log"
	"net"
	"net/url"
)

var (
	// indirection for testing
	resolveTCPAddr = net.ResolveTCPAddr
)

// ResolveTCPAddrs is a convenience wrapper for net.ResolveTCPAddr.
// ResolveTCPAddrs resolves all DNS hostnames in-place for the given set of
// url.URLs.
func ResolveTCPAddrs(urls ...[]url.URL) error {
	for _, us := range urls {
		for i, u := range us {
			host, _, err := net.SplitHostPort(u.Host)
			if err != nil {
				log.Printf("netutil: Could not parse url %s during tcp resolving.", u.Host)
				return err
			}
			if host == "localhost" {
				continue
			}
			if net.ParseIP(host) != nil {
				continue
			}
			tcpAddr, err := resolveTCPAddr("tcp", u.Host)
			if err != nil {
				log.Printf("netutil: Could not resolve host: %s", u.Host)
				return err
			}
			log.Printf("netutil: Resolving %s to %s", u.Host, tcpAddr.String())
			us[i].Host = tcpAddr.String()
		}
	}
	return nil
}
