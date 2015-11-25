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

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/spf13/cobra"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/grpc"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
)

// NewPutCommand returns the cobra command for "put".
func NewPutCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "put",
		Short: "Put puts the given key into the store.",
		Run:   putCommandFunc,
	}
}

// putCommandFunc executes the "put" command.
func putCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) != 2 {
		ExitWithError(ExitBadArgs, fmt.Errorf("put command needs 2 arguments."))
	}

	key := []byte(args[0])
	value := []byte(args[1])

	endpoint, err := cmd.Flags().GetString("endpoint")
	if err != nil {
		ExitWithError(ExitError, err)
	}
	conn, err := grpc.Dial(endpoint)
	if err != nil {
		ExitWithError(ExitBadConnection, err)
	}
	kv := pb.NewKVClient(conn)
	req := &pb.PutRequest{Key: key, Value: value}

	kv.Put(context.Background(), req)
	fmt.Printf("%s %s\n", key, value)
}
