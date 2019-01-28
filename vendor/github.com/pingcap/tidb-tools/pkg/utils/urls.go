// Copyright 2018 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package utils

import (
	"net"
	"net/url"
	"strings"

	"github.com/pingcap/errors"
)

// ParseHostPortAddr returns a scheme://host:port or host:port list
func ParseHostPortAddr(s string) ([]string, error) {
	strs := strings.Split(s, ",")
	addrs := make([]string, 0, len(strs))

	for _, str := range strs {
		str = strings.TrimSpace(str)

		// str may looks like 127.0.0.1:8000
		if _, _, err := net.SplitHostPort(str); err == nil {
			addrs = append(addrs, str)
			continue
		}

		u, err := url.Parse(str)
		if err != nil {
			return nil, errors.Errorf("parse url %s failed %v", str, err)
		}
		if u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "unix" && u.Scheme != "unixs" {
			return nil, errors.Errorf("URL scheme must be http, https, unix, or unixs: %s", str)
		}
		if _, _, err := net.SplitHostPort(u.Host); err != nil {
			return nil, errors.Errorf(`URL address does not have the form "host:port": %s`, str)
		}
		if u.Path != "" {
			return nil, errors.Errorf("URL must not contain a path: %s", str)
		}
		addrs = append(addrs, u.String())
	}

	return addrs, nil
}
