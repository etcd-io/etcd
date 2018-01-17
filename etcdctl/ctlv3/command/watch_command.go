// Copyright 2015 The etcd Authors
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
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/coreos/etcd/clientv3"

	"github.com/spf13/cobra"
)

var (
	errBadArgsNum              = errors.New("bad number of arguments")
	errBadArgsNumConflictEnv   = errors.New("bad number of arguments (found conflicting environment key)")
	errBadArgsNumSeparator     = errors.New("bad number of arguments (found separator --, but no commands)")
	errBadArgsInteractiveWatch = errors.New("args[0] must be 'watch' for interactive calls")
)

var (
	watchRev         int64
	watchPrefix      bool
	watchInteractive bool
	watchPrevKey     bool
)

// NewWatchCommand returns the cobra command for "watch".
func NewWatchCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch [options] [key or prefix] [range_end] [--] [exec-command arg1 arg2 ...]",
		Short: "Watches events stream on keys or prefixes",
		Run:   watchCommandFunc,
	}

	cmd.Flags().BoolVarP(&watchInteractive, "interactive", "i", false, "Interactive mode")
	cmd.Flags().BoolVar(&watchPrefix, "prefix", false, "Watch on a prefix if prefix is set")
	cmd.Flags().Int64Var(&watchRev, "rev", 0, "Revision to start watching")
	cmd.Flags().BoolVar(&watchPrevKey, "prev-kv", false, "get the previous key-value pair before the event happens")

	return cmd
}

// watchCommandFunc executes the "watch" command.
func watchCommandFunc(cmd *cobra.Command, args []string) {
	envKey, envRange := os.Getenv("ETCDCTL_WATCH_KEY"), os.Getenv("ETCDCTL_WATCH_RANGE_END")
	if envKey == "" && envRange != "" {
		ExitWithError(ExitBadArgs, fmt.Errorf("ETCDCTL_WATCH_KEY is empty but got ETCDCTL_WATCH_RANGE_END=%q", envRange))
	}

	if watchInteractive {
		watchInteractiveFunc(cmd, os.Args, envKey, envRange)
		return
	}

	watchArgs, execArgs, err := parseWatchArgs(os.Args, args, envKey, envRange, false)
	if err != nil {
		ExitWithError(ExitBadArgs, err)
	}

	c := mustClientFromCmd(cmd)
	wc, err := getWatchChan(c, watchArgs)
	if err != nil {
		ExitWithError(ExitBadArgs, err)
	}

	printWatchCh(c, wc, execArgs)
	if err = c.Close(); err != nil {
		ExitWithError(ExitBadConnection, err)
	}
	ExitWithError(ExitInterrupted, fmt.Errorf("watch is canceled by the server"))
}

func watchInteractiveFunc(cmd *cobra.Command, osArgs []string, envKey, envRange string) {
	c := mustClientFromCmd(cmd)

	reader := bufio.NewReader(os.Stdin)

	for {
		l, err := reader.ReadString('\n')
		if err != nil {
			ExitWithError(ExitInvalidInput, fmt.Errorf("Error reading watch request line: %v", err))
		}
		l = strings.TrimSuffix(l, "\n")

		args := argify(l)
		if len(args) < 2 && envKey == "" {
			fmt.Fprintf(os.Stderr, "Invalid command %s (command type or key is not provided)\n", l)
			continue
		}

		if args[0] != "watch" {
			fmt.Fprintf(os.Stderr, "Invalid command %s (only support watch)\n", l)
			continue
		}

		watchArgs, execArgs, perr := parseWatchArgs(osArgs, args, envKey, envRange, true)
		if perr != nil {
			ExitWithError(ExitBadArgs, perr)
		}

		ch, err := getWatchChan(c, watchArgs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid command %s (%v)\n", l, err)
			continue
		}
		go printWatchCh(c, ch, execArgs)
	}
}

func getWatchChan(c *clientv3.Client, args []string) (clientv3.WatchChan, error) {
	if len(args) < 1 {
		return nil, errBadArgsNum
	}

	key := args[0]
	opts := []clientv3.OpOption{clientv3.WithRev(watchRev)}
	if len(args) == 2 {
		if watchPrefix {
			return nil, fmt.Errorf("`range_end` and `--prefix` are mutually exclusive")
		}
		opts = append(opts, clientv3.WithRange(args[1]))
	}
	if watchPrefix {
		opts = append(opts, clientv3.WithPrefix())
	}
	if watchPrevKey {
		opts = append(opts, clientv3.WithPrevKV())
	}
	return c.Watch(clientv3.WithRequireLeader(context.Background()), key, opts...), nil
}

func printWatchCh(c *clientv3.Client, ch clientv3.WatchChan, execArgs []string) {
	for resp := range ch {
		if resp.Canceled {
			fmt.Fprintf(os.Stderr, "watch was canceled (%v)\n", resp.Err())
		}
		display.Watch(resp)

		if len(execArgs) > 0 {
			for _, ev := range resp.Events {
				cmd := exec.CommandContext(c.Ctx(), execArgs[0], execArgs[1:]...)
				cmd.Env = os.Environ()
				cmd.Env = append(cmd.Env, fmt.Sprintf("ETCD_WATCH_REVISION=%d", resp.Header.Revision))
				cmd.Env = append(cmd.Env, fmt.Sprintf("ETCD_WATCH_EVENT_TYPE=%q", ev.Type))
				cmd.Env = append(cmd.Env, fmt.Sprintf("ETCD_WATCH_KEY=%q", ev.Kv.Key))
				cmd.Env = append(cmd.Env, fmt.Sprintf("ETCD_WATCH_VALUE=%q", ev.Kv.Value))
				cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
				if err := cmd.Run(); err != nil {
					fmt.Fprintf(os.Stderr, "command %q error (%v)\n", execArgs, err)
					os.Exit(1)
				}
			}
		}
	}
}

// "commandArgs" is the command arguments after "spf13/cobra" parses
// all "watch" command flags, strips out special characters (e.g. "--").
// "orArgs" is the raw arguments passed to "watch" command
// (e.g. ./bin/etcdctl watch foo --rev 1 bar).
// "--" characters are invalid arguments for "spf13/cobra" library,
// so no need to handle such cases.
func parseWatchArgs(osArgs, commandArgs []string, envKey, envRange string, interactive bool) (watchArgs []string, execArgs []string, err error) {
	watchArgs = commandArgs

	// remove preceding commands (e.g. "watch foo bar" in interactive mode)
	idx := 0
	for idx = range watchArgs {
		if watchArgs[idx] == "watch" {
			break
		}
	}
	if idx < len(watchArgs)-1 || envKey != "" {
		if idx < len(watchArgs)-1 {
			watchArgs = watchArgs[idx+1:]
		}

		execIdx, execExist := 0, false
		for execIdx = range osArgs {
			v := osArgs[execIdx]
			if v == "--" && execIdx != len(osArgs)-1 {
				execExist = true
				break
			}
		}

		if idx == len(watchArgs)-1 && envKey != "" {
			if len(watchArgs) > 0 && !interactive {
				// "watch --rev 1 -- echo Hello World" has no conflict
				if !execExist {
					// "watch foo" with ETCDCTL_WATCH_KEY=foo
					// (watchArgs==["foo"])
					return nil, nil, errBadArgsNumConflictEnv
				}
			}
			// otherwise, watch with no argument and environment key is set
			// if interactive, first "watch" command string should be removed
			if interactive {
				watchArgs = []string{}
			}
		}

		// "watch foo -- echo hello" with ETCDCTL_WATCH_KEY=foo
		// (watchArgs==["foo","echo","hello"])
		if envKey != "" && execExist {
			widx, oidx := 0, len(osArgs)-1
			for widx = len(watchArgs) - 1; widx >= 0; widx-- {
				if watchArgs[widx] == osArgs[oidx] {
					oidx--
					continue
				}
				if oidx == execIdx { // watchArgs has extra
					return nil, nil, errBadArgsNumConflictEnv
				}
			}
		}
	} else if interactive { // "watch" not found
		return nil, nil, errBadArgsInteractiveWatch
	}
	if len(watchArgs) < 1 && envKey == "" {
		return nil, nil, errBadArgsNum
	}

	// remove preceding commands (e.g. ./bin/etcdctl watch)
	for idx = range osArgs {
		if osArgs[idx] == "watch" {
			break
		}
	}
	if idx < len(osArgs)-1 {
		osArgs = osArgs[idx+1:]
	} else if envKey == "" {
		return nil, nil, errBadArgsNum
	}

	argsWithSep := osArgs
	if interactive { // interactive mode pass "--" to the command args
		argsWithSep = watchArgs
	}
	foundSep := false
	for idx = range argsWithSep {
		if argsWithSep[idx] == "--" {
			foundSep = true
			break
		}
	}
	if interactive {
		flagset := NewWatchCommand().Flags()
		if err := flagset.Parse(argsWithSep); err != nil {
			return nil, nil, err
		}
		watchArgs = flagset.Args()
	}

	// "watch -- echo hello" with ETCDCTL_WATCH_KEY=foo
	// should be translated to "watch foo -- echo hello"
	// (watchArgs=["echo","hello"] should be ["foo","echo","hello"])
	if envKey != "" {
		tmp := []string{envKey}
		if envRange != "" {
			tmp = append(tmp, envRange)
		}
		watchArgs = append(tmp, watchArgs...)
	}

	if !foundSep {
		return watchArgs, nil, nil
	}

	if idx == len(argsWithSep)-1 {
		// "watch foo bar --" should error
		return nil, nil, errBadArgsNumSeparator
	}
	execArgs = argsWithSep[idx+1:]

	// "watch foo bar --rev 1 -- echo hello" or "watch foo --rev 1 bar -- echo hello",
	// then "watchArgs" is "foo bar echo hello"
	// so need ignore args after "argsWithSep[idx]", which is "--"
	endIdx := 0
	for endIdx = len(watchArgs) - 1; endIdx >= 0; endIdx-- {
		if watchArgs[endIdx] == argsWithSep[idx+1] {
			break
		}
	}
	watchArgs = watchArgs[:endIdx]

	return watchArgs, execArgs, nil
}
