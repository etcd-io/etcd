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
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
)

// NewDeleteRangeCommand returns the cobra command for "deleteRange".
func NewDeleteRangeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "delete-range",
		Short: "DeleteRange deletes the given range from the store.",
		Run:   deleteRangeCommandFunc,
	}
}

// deleteRangeCommandFunc executes the "deleteRange" command.
func deleteRangeCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		ExitWithError(ExitBadArgs, fmt.Errorf("delete-range command needs arguments."))
	}

	var rangeEnd []byte
	key := []byte(args[0])
	if len(args) > 1 {
		rangeEnd = []byte(args[1])
	}

	req := &pb.DeleteRangeRequest{Key: key, RangeEnd: rangeEnd}
	mustClient(cmd).KV.DeleteRange(context.Background(), req)

	if rangeEnd != nil {
		fmt.Printf("range [%s, %s) is deleted\n", string(key), string(rangeEnd))
	} else {
		fmt.Printf("key %s is deleted\n", string(key))
	}
}
