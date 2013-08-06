// Copyright 2013, Cong Ding. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// author: Cong Ding <dinggnu@gmail.com>
//
package config

import (
	"bufio"
	"os"
	"strings"
)

func Read(filename string) (map[string]string, error) {
	var res = map[string]string{}
	in, err := os.Open(filename)
	if err != nil {
		return res, err
	}
	scanner := bufio.NewScanner(in)
	line := ""
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "//") {
			continue
		}
		if strings.HasPrefix(scanner.Text(), "#") {
			continue
		}
		line += scanner.Text()
		if strings.HasSuffix(line, "\\") {
			line = line[:len(line)-1]
			continue
		}
		sp := strings.SplitN(line, "=", 2)
		if len(sp) != 2 {
			continue
		}
		res[strings.TrimSpace(sp[0])] = strings.TrimSpace(sp[1])
		line = ""
	}
	in.Close()
	return res, nil
}
