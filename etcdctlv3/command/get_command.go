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
	"strings"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/spf13/cobra"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/clientv3"
)

var (
	getLimit      int64
	getSortOrder  string
	getSortTarget string
)

// NewGetCommand returns the cobra command for "get".
func NewGetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get [options] <key> [range_end]",
		Short: "Get gets the key or a range of keys.",
		Run:   getCommandFunc,
	}

	cmd.Flags().StringVar(&getSortOrder, "order", "", "order of results; ASCEND or DESCEND")
	cmd.Flags().StringVar(&getSortTarget, "sort-by", "", "sort target; CREATE, KEY, MODIFY, VALUE, or VERSION")
	cmd.Flags().Int64Var(&getLimit, "limit", 0, "maximum number of results")
	// TODO: add fromkey.
	// TODO: add prefix.
	// TODO: add consistency.
	return cmd
}

// getCommandFunc executes the "get" command.
func getCommandFunc(cmd *cobra.Command, args []string) {
	key, opts := getGetOp(cmd, args)
	c := mustClientFromCmd(cmd)
	kvapi := clientv3.NewKV(c)
	resp, err := kvapi.Get(context.TODO(), key, opts...)
	if err != nil {
		ExitWithError(ExitError, err)
	}

	display.Get(*resp)
}

func getGetOp(cmd *cobra.Command, args []string) (string, []clientv3.OpOption) {
	if len(args) == 0 {
		ExitWithError(ExitBadArgs, fmt.Errorf("range command needs arguments."))
	}

	opts := []clientv3.OpOption{}
	key := args[0]
	if len(args) > 1 {
		opts = append(opts, clientv3.WithRange(args[1]))
	}
	opts = append(opts, clientv3.WithLimit(getLimit))

	sortByOrder := clientv3.SortNone
	sortOrder := strings.ToUpper(getSortOrder)
	switch {
	case sortOrder == "ASCEND":
		sortByOrder = clientv3.SortAscend
	case sortOrder == "DESCEND":
		sortByOrder = clientv3.SortDescend
	case sortOrder == "":
		// nothing
	default:
		ExitWithError(ExitBadFeature, fmt.Errorf("bad sort order %v", getSortOrder))
	}

	sortByTarget := clientv3.SortByKey
	sortTarget := strings.ToUpper(getSortTarget)
	switch {
	case sortTarget == "CREATE":
		sortByTarget = clientv3.SortByCreatedRev
	case sortTarget == "KEY":
		sortByTarget = clientv3.SortByKey
	case sortTarget == "MODIFY":
		sortByTarget = clientv3.SortByModifiedRev
	case sortTarget == "VALUE":
		sortByTarget = clientv3.SortByValue
	case sortTarget == "VERSION":
		sortByTarget = clientv3.SortByVersion
	case sortTarget == "":
		// nothing
	default:
		ExitWithError(ExitBadFeature, fmt.Errorf("bad sort target %v", getSortTarget))
	}

	opts = append(opts, clientv3.WithSort(sortByTarget, sortByOrder))
	return key, opts
}
