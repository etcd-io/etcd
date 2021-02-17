// Copyright 2021 The etcd Authors
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

package endpoint

import (
	"net/url"
	"regexp"
)

var (
	STRIP_PORT_REGEXP = regexp.MustCompile("(.*):([0-9]+)")
)

func stripPort(ep string) string {
	return STRIP_PORT_REGEXP.ReplaceAllString(ep, "$1")
}

func translateEndpoint(ep string) (addr string, serverName string, requireCreds bool) {
	url, err := url.Parse(ep)
	if err != nil {
		return ep, stripPort(ep), false
	}
	switch url.Scheme {
	case "http", "https":
		return url.Host, url.Hostname(), url.Scheme == "https"
	case "unix", "unixs":
		requireCreds = url.Scheme == "unixs"
		if url.Opaque != "" {
			return "unix:" + url.Opaque, stripPort(url.Opaque), requireCreds
		} else if url.Path != "" {
			return "unix://" + url.Host + url.Path, url.Host + url.Path, requireCreds
		} else {
			return "unix:" + url.Host, url.Hostname(), requireCreds
		}
	case "":
		return url.Host + url.Path, url.Host + url.Path, false
	default:
		return ep, stripPort(ep), false
	}
}

// RequiresCredentials returns whether given endpoint requires
// credentials/certificates for connection.
func RequiresCredentials(ep string) bool {
	_, _, requireCreds := translateEndpoint(ep)
	return requireCreds
}

// Interpret endpoint parses an endpoint of the form
// (http|https)://<host>*|(unix|unixs)://<path>)
// and returns low-level address (supported by 'net') to connect to,
// and a server name used for x509 certificate matching.
func Interpret(ep string) (address string, serverName string) {
	addr, serverName, _ := translateEndpoint(ep)
	return addr, serverName
}
