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

package command

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/spf13/cobra"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/clientv3"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
)

var (
	watchRev         int64
	watchPrefix      bool
	watchHex         bool
	watchInteractive bool
)

// NewWatchCommand returns the cobra command for "watch".
func NewWatchCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch [key or prefix]",
		Short: "Watch watches events stream on keys or prefixes.",
		Run:   watchCommandFunc,
	}

	cmd.Flags().BoolVar(&watchHex, "hex", false, "print out key and value as hex encode string for text format")
	cmd.Flags().BoolVarP(&watchInteractive, "interactive", "i", false, "interactive mode")
	cmd.Flags().BoolVar(&watchPrefix, "prefix", false, "watch on a prefix if prefix is set")
	cmd.Flags().Int64Var(&watchRev, "rev", 0, "revision to start watching")

	return cmd
}

// watchCommandFunc executes the "watch" command.
func watchCommandFunc(cmd *cobra.Command, args []string) {
	if watchInteractive {
		watchInteractiveFunc(cmd, args)
		return
	}

	if len(args) != 1 {
		ExitWithError(ExitBadArgs, fmt.Errorf("watch in non-interactive mode requires an argument as key or prefix"))
	}

	c := mustClientFromCmd(cmd)
	w := clientv3.NewWatcher(c)

	var wc clientv3.WatchChan
	if !watchPrefix {
		wc = w.Watch(context.TODO(), args[0], watchRev)
	} else {
		wc = w.Watch(context.TODO(), args[0], watchRev)
	}
	for resp := range wc {
		for _, e := range resp.Events {
			fmt.Println(e.Type)
			printKV(watchHex, e.Kv)
		}
	}
	err := w.Close()
	if err == nil {
		ExitWithError(ExitInterrupted, fmt.Errorf("watch is canceled by the server"))
	}
	ExitWithError(ExitBadConnection, err)
}

func watchInteractiveFunc(cmd *cobra.Command, args []string) {
	wStream, err := mustClientFromCmd(cmd).Watch.Watch(context.TODO())
	if err != nil {
		ExitWithError(ExitBadConnection, err)
	}

	go recvLoop(wStream)

	reader := bufio.NewReader(os.Stdin)

	for {
		l, err := reader.ReadString('\n')
		if err != nil {
			ExitWithError(ExitInvalidInput, fmt.Errorf("Error reading watch request line: %v", err))
		}
		l = strings.TrimSuffix(l, "\n")

		// TODO: support start and end revision
		segs := strings.Split(l, " ")
		if len(segs) != 2 {
			fmt.Fprintf(os.Stderr, "Invalid watch request format: use \"watch [key]\", \"watchprefix [prefix]\" or \"cancel [watcher ID]\"\n")
			continue
		}

		var r *pb.WatchRequest
		switch segs[0] {
		case "watch":
			r = &pb.WatchRequest{
				RequestUnion: &pb.WatchRequest_CreateRequest{
					CreateRequest: &pb.WatchCreateRequest{
						Key: []byte(segs[1])}}}
		case "watchprefix":
			r = &pb.WatchRequest{
				RequestUnion: &pb.WatchRequest_CreateRequest{
					CreateRequest: &pb.WatchCreateRequest{
						Prefix: []byte(segs[1])}}}
		case "cancel":
			id, perr := strconv.ParseInt(segs[1], 10, 64)
			if perr != nil {
				fmt.Fprintf(os.Stderr, "Invalid cancel ID (%v)\n", perr)
				continue
			}
			r = &pb.WatchRequest{
				RequestUnion: &pb.WatchRequest_CancelRequest{
					CancelRequest: &pb.WatchCancelRequest{
						WatchId: id}}}
		default:
			fmt.Fprintf(os.Stderr, "Invalid watch request type: use watch, watchprefix or cancel\n")
			continue
		}

		err = wStream.Send(r)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error sending request to server: %v\n", err)
		}
	}
}

func recvLoop(wStream pb.Watch_WatchClient) {
	for {
		resp, err := wStream.Recv()
		if err == io.EOF {
			os.Exit(ExitSuccess)
		}
		if err != nil {
			ExitWithError(ExitError, err)
		}

		switch {
		// TODO: handle canceled/compacted and other control response types
		case resp.Created:
			fmt.Printf("watcher created: id %08x\n", resp.WatchId)
		case resp.Canceled:
			fmt.Printf("watcher canceled: id %08x\n", resp.WatchId)
		default:
			for _, ev := range resp.Events {
				fmt.Printf("%s: %s %s\n", ev.Type, string(ev.Kv.Key), string(ev.Kv.Value))
			}
		}
	}
}
