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
	"fmt"
	"strconv"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/spf13/cobra"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
)

var (
	leaseStr string
)

// NewPutCommand returns the cobra command for "put".
func NewPutCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "put [options] <key> <value>",
		Short: "Put puts the given key into the store.",
		Long: `
Put puts the given key into the store.

When <value> begins with '-', <value> is interpreted as a flag.
Insert '--' for workaround:

$ put <key> -- <value>
$ put -- <key> <value>
`,
		Run: putCommandFunc,
	}
	cmd.Flags().StringVar(&leaseStr, "lease", "0", "lease ID attached to the put key")
	return cmd
}

// putCommandFunc executes the "put" command.
func putCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) != 2 {
		ExitWithError(ExitBadArgs, fmt.Errorf("put command needs 2 arguments."))
	}

	id, err := strconv.ParseInt(leaseStr, 16, 64)
	if err != nil {
		ExitWithError(ExitBadArgs, fmt.Errorf("bad lease ID arg (%v), expecting ID in Hex", err))
	}

	key := []byte(args[0])
	value := []byte(args[1])

	req := &pb.PutRequest{Key: key, Value: value, Lease: id}
	mustClient(cmd).KV.Put(context.Background(), req)
	fmt.Printf("%s %s\n", key, value)
}
