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
	"strings"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/grpc"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
)

// NewWatchCommand returns the CLI command for "watch".
func NewWatchCommand() cli.Command {
	return cli.Command{
		Name: "watch",
		Action: func(c *cli.Context) {
			watchCommandFunc(c)
		},
	}
}

// watchCommandFunc executes the "watch" command.
func watchCommandFunc(c *cli.Context) {
	conn, err := grpc.Dial(c.GlobalString("endpoint"))
	if err != nil {
		panic(err)
	}

	wAPI := pb.NewWatchClient(conn)
	wStream, err := wAPI.Watch(context.TODO())
	if err != nil {
		panic(err)
	}

	go recvLoop(wStream)

	reader := bufio.NewReader(os.Stdin)

	for {
		l, err := reader.ReadString('\n')
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading watch request line: %v", err)
			os.Exit(1)
		}
		l = strings.TrimSuffix(l, "\n")

		// TODO: support start and end revision
		segs := strings.Split(l, " ")
		if len(segs) != 2 {
			fmt.Fprintf(os.Stderr, "Invalid watch request format: use watch key or watchprefix prefix\n")
			continue
		}

		var r *pb.WatchRequest
		switch segs[0] {
		case "watch":
			r = &pb.WatchRequest{Key: []byte(segs[1])}
		case "watchprefix":
			r = &pb.WatchRequest{Prefix: []byte(segs[1])}
		default:
			fmt.Fprintf(os.Stderr, "Invalid watch request format: use watch key or watchprefix prefix\n")
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
			os.Exit(0)
		}
		if err != nil {
			panic(err)
		}
		fmt.Printf("%s: %s %s\n", resp.Event.Type, string(resp.Event.Kv.Key), string(resp.Event.Kv.Value))
	}
}
