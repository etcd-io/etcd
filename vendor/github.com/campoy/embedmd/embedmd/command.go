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
	"errors"
	"path/filepath"
	"strings"
)

type command struct {
	path, lang string
	start, end *string
}

func parseCommand(s string) (*command, error) {
	s = strings.TrimSpace(s)
	if len(s) < 2 || s[0] != '(' || s[len(s)-1] != ')' {
		return nil, errors.New("argument list should be in parenthesis")
	}

	args, err := fields(s[1 : len(s)-1])
	if err != nil {
		return nil, err
	}
	if len(args) == 0 {
		return nil, errors.New("missing file name")
	}

	cmd := &command{path: args[0]}
	args = args[1:]
	if len(args) > 0 && args[0][0] != '/' {
		cmd.lang, args = args[0], args[1:]
	} else {
		ext := filepath.Ext(cmd.path[1:])
		if len(ext) == 0 {
			return nil, errors.New("language is required when file has no extension")
		}
		cmd.lang = ext[1:]
	}

	switch {
	case len(args) == 1:
		cmd.start = &args[0]
	case len(args) == 2:
		cmd.start, cmd.end = &args[0], &args[1]
	case len(args) > 2:
		return nil, errors.New("too many arguments")
	}

	return cmd, nil
}

// fields returns a list of the groups of text separated by blanks,
// keeping all text surrounded by / as a group.
func fields(s string) ([]string, error) {
	var args []string

	for s = strings.TrimSpace(s); len(s) > 0; s = strings.TrimSpace(s) {
		if s[0] == '/' {
			sep := nextSlash(s[1:])
			if sep < 0 {
				return nil, errors.New("unbalanced /")
			}
			args, s = append(args, s[:sep+2]), s[sep+2:]
		} else {
			sep := strings.IndexByte(s[1:], ' ')
			if sep < 0 {
				return append(args, s), nil
			}
			args, s = append(args, s[:sep+1]), s[sep+1:]
		}
	}

	return args, nil
}

// nextSlash will find the index of the next unescaped slash in a string.
func nextSlash(s string) int {
	for sep := 0; ; sep++ {
		i := strings.IndexByte(s[sep:], '/')
		if i < 0 {
			return -1
		}
		sep += i
		if sep == 0 || s[sep-1] != '\\' {
			return sep
		}
	}
}
