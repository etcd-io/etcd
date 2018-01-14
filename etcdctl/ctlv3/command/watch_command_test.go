// Copyright 2017 The etcd Authors
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

package command

import (
	"reflect"
	"testing"
)

func Test_parseWatchArgs(t *testing.T) {
	tt := []struct {
		osArgs           []string // raw arguments to "watch" command
		commandArgs      []string // arguments after "spf13/cobra" preprocessing
		envKey, envRange string
		interactive      bool

		watchArgs []string
		execArgs  []string
		err       error
	}{
		{
			osArgs:      []string{"./bin/etcdctl", "watch", "foo", "bar"},
			commandArgs: []string{"foo", "bar"},
			interactive: false,
			watchArgs:   []string{"foo", "bar"},
			execArgs:    nil,
			err:         nil,
		},
		{
			osArgs:      []string{"./bin/etcdctl", "watch", "foo", "bar", "--"},
			commandArgs: []string{"foo", "bar"},
			interactive: false,
			watchArgs:   nil,
			execArgs:    nil,
			err:         errBadArgsNumSeparator,
		},
		{
			osArgs:      []string{"./bin/etcdctl", "watch"},
			commandArgs: nil,
			envKey:      "foo",
			envRange:    "bar",
			interactive: false,
			watchArgs:   []string{"foo", "bar"},
			execArgs:    nil,
			err:         nil,
		},
		{
			osArgs:      []string{"./bin/etcdctl", "watch", "foo"},
			commandArgs: []string{"foo"},
			envKey:      "foo",
			envRange:    "",
			interactive: false,
			watchArgs:   nil,
			execArgs:    nil,
			err:         errBadArgsNumConflictEnv,
		},
		{
			osArgs:      []string{"./bin/etcdctl", "watch", "foo", "bar"},
			commandArgs: []string{"foo", "bar"},
			envKey:      "foo",
			envRange:    "",
			interactive: false,
			watchArgs:   nil,
			execArgs:    nil,
			err:         errBadArgsNumConflictEnv,
		},
		{
			osArgs:      []string{"./bin/etcdctl", "watch", "foo", "bar"},
			commandArgs: []string{"foo", "bar"},
			envKey:      "foo",
			envRange:    "bar",
			interactive: false,
			watchArgs:   nil,
			execArgs:    nil,
			err:         errBadArgsNumConflictEnv,
		},
		{
			osArgs:      []string{"./bin/etcdctl", "watch", "foo"},
			commandArgs: []string{"foo"},
			interactive: false,
			watchArgs:   []string{"foo"},
			execArgs:    nil,
			err:         nil,
		},
		{
			osArgs:      []string{"./bin/etcdctl", "watch"},
			commandArgs: nil,
			envKey:      "foo",
			interactive: false,
			watchArgs:   []string{"foo"},
			execArgs:    nil,
			err:         nil,
		},
		{
			osArgs:      []string{"./bin/etcdctl", "watch", "--rev", "1", "foo"},
			commandArgs: []string{"foo"},
			interactive: false,
			watchArgs:   []string{"foo"},
			execArgs:    nil,
			err:         nil,
		},
		{
			osArgs:      []string{"./bin/etcdctl", "watch", "--rev", "1", "foo"},
			commandArgs: []string{"foo"},
			envKey:      "foo",
			interactive: false,
			watchArgs:   nil,
			execArgs:    nil,
			err:         errBadArgsNumConflictEnv,
		},
		{
			osArgs:      []string{"./bin/etcdctl", "watch", "--rev", "1"},
			commandArgs: nil,
			envKey:      "foo",
			interactive: false,
			watchArgs:   []string{"foo"},
			execArgs:    nil,
			err:         nil,
		},
		{
			osArgs:      []string{"./bin/etcdctl", "watch", "foo", "--rev", "1"},
			commandArgs: []string{"foo"},
			interactive: false,
			watchArgs:   []string{"foo"},
			execArgs:    nil,
			err:         nil,
		},
		{
			osArgs:      []string{"./bin/etcdctl", "watch", "foo", "--", "echo", "Hello", "World"},
			commandArgs: []string{"foo", "echo", "Hello", "World"},
			interactive: false,
			watchArgs:   []string{"foo"},
			execArgs:    []string{"echo", "Hello", "World"},
			err:         nil,
		},
		{
			osArgs:      []string{"./bin/etcdctl", "watch", "foo", "--rev", "1", "--", "echo", "Hello", "World"},
			commandArgs: []string{"foo", "echo", "Hello", "World"},
			interactive: false,
			watchArgs:   []string{"foo"},
			execArgs:    []string{"echo", "Hello", "World"},
			err:         nil,
		},
		{
			osArgs:      []string{"./bin/etcdctl", "watch", "foo", "bar", "--", "echo", "Hello", "World"},
			commandArgs: []string{"foo", "bar", "echo", "Hello", "World"},
			interactive: false,
			watchArgs:   []string{"foo", "bar"},
			execArgs:    []string{"echo", "Hello", "World"},
			err:         nil,
		},
		{
			osArgs:      []string{"./bin/etcdctl", "watch", "--rev", "1", "foo", "bar", "--", "echo", "Hello", "World"},
			commandArgs: []string{"foo", "bar", "echo", "Hello", "World"},
			interactive: false,
			watchArgs:   []string{"foo", "bar"},
			execArgs:    []string{"echo", "Hello", "World"},
			err:         nil,
		},
		{
			osArgs:      []string{"./bin/etcdctl", "watch", "foo", "--rev", "1", "bar", "--", "echo", "Hello", "World"},
			commandArgs: []string{"foo", "bar", "echo", "Hello", "World"},
			interactive: false,
			watchArgs:   []string{"foo", "bar"},
			execArgs:    []string{"echo", "Hello", "World"},
			err:         nil,
		},
		{
			osArgs:      []string{"./bin/etcdctl", "watch", "foo", "bar", "--rev", "1", "--", "echo", "Hello", "World"},
			commandArgs: []string{"foo", "bar", "echo", "Hello", "World"},
			interactive: false,
			watchArgs:   []string{"foo", "bar"},
			execArgs:    []string{"echo", "Hello", "World"},
			err:         nil,
		},
		{
			osArgs:      []string{"./bin/etcdctl", "watch", "--rev", "1", "--", "echo", "Hello", "World"},
			commandArgs: []string{"echo", "Hello", "World"},
			envKey:      "foo",
			envRange:    "",
			interactive: false,
			watchArgs:   []string{"foo"},
			execArgs:    []string{"echo", "Hello", "World"},
			err:         nil,
		},
		{
			osArgs:      []string{"./bin/etcdctl", "watch", "--rev", "1", "--", "echo", "Hello", "World"},
			commandArgs: []string{"echo", "Hello", "World"},
			envKey:      "foo",
			envRange:    "bar",
			interactive: false,
			watchArgs:   []string{"foo", "bar"},
			execArgs:    []string{"echo", "Hello", "World"},
			err:         nil,
		},
		{
			osArgs:      []string{"./bin/etcdctl", "watch", "foo", "bar", "--rev", "1", "--", "echo", "Hello", "World"},
			commandArgs: []string{"foo", "bar", "echo", "Hello", "World"},
			envKey:      "foo",
			interactive: false,
			watchArgs:   nil,
			execArgs:    nil,
			err:         errBadArgsNumConflictEnv,
		},
		{
			osArgs:      []string{"./bin/etcdctl", "watch", "-i"},
			commandArgs: []string{"foo", "bar", "--", "echo", "Hello", "World"},
			interactive: true,
			watchArgs:   nil,
			execArgs:    nil,
			err:         errBadArgsInteractiveWatch,
		},
		{
			osArgs:      []string{"./bin/etcdctl", "watch", "-i"},
			commandArgs: []string{"watch", "foo"},
			interactive: true,
			watchArgs:   []string{"foo"},
			execArgs:    nil,
			err:         nil,
		},
		{
			osArgs:      []string{"./bin/etcdctl", "watch", "-i"},
			commandArgs: []string{"watch", "foo", "bar"},
			interactive: true,
			watchArgs:   []string{"foo", "bar"},
			execArgs:    nil,
			err:         nil,
		},
		{
			osArgs:      []string{"./bin/etcdctl", "watch", "-i"},
			commandArgs: []string{"watch"},
			envKey:      "foo",
			envRange:    "bar",
			interactive: true,
			watchArgs:   []string{"foo", "bar"},
			execArgs:    nil,
			err:         nil,
		},
		{
			osArgs:      []string{"./bin/etcdctl", "watch", "-i"},
			commandArgs: []string{"watch"},
			envKey:      "hello world!",
			envRange:    "bar",
			interactive: true,
			watchArgs:   []string{"hello world!", "bar"},
			execArgs:    nil,
			err:         nil,
		},
		{
			osArgs:      []string{"./bin/etcdctl", "watch", "-i"},
			commandArgs: []string{"watch", "foo", "--rev", "1"},
			interactive: true,
			watchArgs:   []string{"foo"},
			execArgs:    nil,
			err:         nil,
		},
		{
			osArgs:      []string{"./bin/etcdctl", "watch", "-i"},
			commandArgs: []string{"watch", "foo", "--rev", "1", "--", "echo", "Hello", "World"},
			interactive: true,
			watchArgs:   []string{"foo"},
			execArgs:    []string{"echo", "Hello", "World"},
			err:         nil,
		},
		{
			osArgs:      []string{"./bin/etcdctl", "watch", "-i"},
			commandArgs: []string{"watch", "--rev", "1", "foo", "--", "echo", "Hello", "World"},
			interactive: true,
			watchArgs:   []string{"foo"},
			execArgs:    []string{"echo", "Hello", "World"},
			err:         nil,
		},
		{
			osArgs:      []string{"./bin/etcdctl", "watch", "-i"},
			commandArgs: []string{"watch", "--", "echo", "Hello", "World"},
			envKey:      "foo",
			interactive: true,
			watchArgs:   []string{"foo"},
			execArgs:    []string{"echo", "Hello", "World"},
			err:         nil,
		},
		{
			osArgs:      []string{"./bin/etcdctl", "watch", "-i"},
			commandArgs: []string{"watch", "--", "echo", "Hello", "World"},
			envKey:      "foo",
			envRange:    "bar",
			interactive: true,
			watchArgs:   []string{"foo", "bar"},
			execArgs:    []string{"echo", "Hello", "World"},
			err:         nil,
		},
		{
			osArgs:      []string{"./bin/etcdctl", "watch", "-i"},
			commandArgs: []string{"watch", "foo", "bar", "--", "echo", "Hello", "World"},
			interactive: true,
			watchArgs:   []string{"foo", "bar"},
			execArgs:    []string{"echo", "Hello", "World"},
			err:         nil,
		},
		{
			osArgs:      []string{"./bin/etcdctl", "watch", "-i"},
			commandArgs: []string{"watch", "--rev", "1", "foo", "bar", "--", "echo", "Hello", "World"},
			interactive: true,
			watchArgs:   []string{"foo", "bar"},
			execArgs:    []string{"echo", "Hello", "World"},
			err:         nil,
		},
		{
			osArgs:      []string{"./bin/etcdctl", "watch", "-i"},
			commandArgs: []string{"watch", "--rev", "1", "--", "echo", "Hello", "World"},
			envKey:      "foo",
			envRange:    "bar",
			interactive: true,
			watchArgs:   []string{"foo", "bar"},
			execArgs:    []string{"echo", "Hello", "World"},
			err:         nil,
		},
		{
			osArgs:      []string{"./bin/etcdctl", "watch", "-i"},
			commandArgs: []string{"watch", "foo", "--rev", "1", "bar", "--", "echo", "Hello", "World"},
			interactive: true,
			watchArgs:   []string{"foo", "bar"},
			execArgs:    []string{"echo", "Hello", "World"},
			err:         nil,
		},
		{
			osArgs:      []string{"./bin/etcdctl", "watch", "-i"},
			commandArgs: []string{"watch", "foo", "bar", "--rev", "1", "--", "echo", "Hello", "World"},
			interactive: true,
			watchArgs:   []string{"foo", "bar"},
			execArgs:    []string{"echo", "Hello", "World"},
			err:         nil,
		},
	}
	for i, ts := range tt {
		watchArgs, execArgs, err := parseWatchArgs(ts.osArgs, ts.commandArgs, ts.envKey, ts.envRange, ts.interactive)
		if err != ts.err {
			t.Fatalf("#%d: error expected %v, got %v", i, ts.err, err)
		}
		if !reflect.DeepEqual(watchArgs, ts.watchArgs) {
			t.Fatalf("#%d: watchArgs expected %q, got %v", i, ts.watchArgs, watchArgs)
		}
		if !reflect.DeepEqual(execArgs, ts.execArgs) {
			t.Fatalf("#%d: execArgs expected %q, got %v", i, ts.execArgs, execArgs)
		}
	}
}
