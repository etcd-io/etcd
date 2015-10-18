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

package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"
)

type URLs []url.URL

// NewURLs creates and initializes a new URLs using the given URL strings
// as its initial contents.
// If no URL string is given, it returns error.
// The given URL string should follow the rules:
// 1. "http" or "https" scheme
// 2. network address is in the form "host:port", "[host]:port" or
// "[ipv6-host%zone]:port"
// 3. empty URL path
// The returned URLs are sorted in increasing order of URL strings.
func NewURLs(strs []string) (URLs, error) {
	all := make([]url.URL, len(strs))
	if len(all) == 0 {
		return nil, errors.New("no valid URLs given")
	}
	for i, in := range strs {
		in = strings.TrimSpace(in)
		u, err := url.Parse(in)
		if err != nil {
			return nil, err
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return nil, fmt.Errorf("URL scheme must be http or https: %s", in)
		}
		if _, _, err := net.SplitHostPort(u.Host); err != nil {
			return nil, fmt.Errorf(`URL address does not have the form "host:port": %s`, in)
		}
		if u.Path != "" {
			return nil, fmt.Errorf("URL must not contain a path: %s", in)
		}
		all[i] = *u
	}
	us := URLs(all)
	us.Sort()

	return us, nil
}

func (us URLs) String() string {
	return strings.Join(us.StringSlice(), ",")
}

// MarshalJSON marshals URLs into valid JSON description,
// which is a JSON array of URL strings.
func (us URLs) MarshalJSON() ([]byte, error) {
	return json.Marshal(us.StringSlice())
}

// UnmarshalJSON unmarshals a JSON description of URLs.
func (us *URLs) UnmarshalJSON(b []byte) error {
	var s []string
	err := json.Unmarshal(b, &s)
	if err != nil {
		return err
	}
	nus, err := NewURLs(s)
	if err != nil {
		return err
	}
	*us = nus
	return nil
}

func (us *URLs) Sort() {
	sort.Sort(us)
}
func (us URLs) Len() int           { return len(us) }
func (us URLs) Less(i, j int) bool { return us[i].String() < us[j].String() }
func (us URLs) Swap(i, j int)      { us[i], us[j] = us[j], us[i] }

func (us URLs) StringSlice() []string {
	out := make([]string, len(us))
	for i := range us {
		out[i] = us[i].String()
	}

	return out
}
