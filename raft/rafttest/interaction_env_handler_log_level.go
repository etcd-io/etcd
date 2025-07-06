// Copyright 2019 The etcd Authors
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

package rafttest

import (
	"fmt"
	"strings"
	"testing"

	"github.com/cockroachdb/datadriven"
)

func (env *InteractionEnv) handleLogLevel(t *testing.T, d datadriven.TestData) error {
	return env.LogLevel(d.CmdArgs[0].Key)
}

func (env *InteractionEnv) LogLevel(name string) error {
	for i, s := range lvlNames {
		if strings.EqualFold(s, name) {
			env.Output.Lvl = i
			return nil
		}
	}
	return fmt.Errorf("log levels must be either of %v", lvlNames)
}
