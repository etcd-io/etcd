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
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
)

var (
	rangeLimit      int
	rangeSortOrder  string
	rangeSortTarget string
)

// NewRangeCommand returns the cobra command for "range".
func NewRangeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "range",
		Short: "Range gets the keys in the range from the store.",
		Run:   rangeCommandFunc,
	}
	cmd.Flags().StringVar(&rangeSortOrder, "order", "", "order of results; ASCEND or DESCEND")
	cmd.Flags().StringVar(&rangeSortTarget, "sort-by", "", "sort target; CREATE, KEY, MODIFY, VALUE, or VERSION")
	cmd.Flags().IntVar(&rangeLimit, "limit", 0, "maximum number of results")
	return cmd
}

// rangeCommandFunc executes the "range" command.
func rangeCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		ExitWithError(ExitBadArgs, fmt.Errorf("range command needs arguments."))
	}

	var rangeEnd []byte
	key := []byte(args[0])
	if len(args) > 1 {
		rangeEnd = []byte(args[1])
	}

	sortByOrder := pb.RangeRequest_NONE
	sortOrder := strings.ToUpper(rangeSortOrder)
	switch {
	case sortOrder == "ASCEND":
		sortByOrder = pb.RangeRequest_ASCEND
	case sortOrder == "DESCEND":
		sortByOrder = pb.RangeRequest_DESCEND
	case sortOrder == "":
		sortByOrder = pb.RangeRequest_NONE
	default:
		ExitWithError(ExitBadFeature, fmt.Errorf("bad sort order %v", rangeSortOrder))
	}

	sortByTarget := pb.RangeRequest_KEY
	sortTarget := strings.ToUpper(rangeSortTarget)
	switch {
	case sortTarget == "CREATE":
		sortByTarget = pb.RangeRequest_CREATE
	case sortTarget == "KEY":
		sortByTarget = pb.RangeRequest_KEY
	case sortTarget == "MODIFY":
		sortByTarget = pb.RangeRequest_MOD
	case sortTarget == "VALUE":
		sortByTarget = pb.RangeRequest_VALUE
	case sortTarget == "VERSION":
		sortByTarget = pb.RangeRequest_VERSION
	case sortTarget == "":
		sortByTarget = pb.RangeRequest_KEY
	default:
		ExitWithError(ExitBadFeature, fmt.Errorf("bad sort target %v", rangeSortTarget))
	}

	req := &pb.RangeRequest{
		Key:        key,
		RangeEnd:   rangeEnd,
		SortOrder:  sortByOrder,
		SortTarget: sortByTarget,
		Limit:      int64(rangeLimit),
	}
	resp, err := mustClient(cmd).KV.Range(context.Background(), req)
	if err != nil {
		ExitWithError(ExitError, err)
	}
	for _, kv := range resp.Kvs {
		fmt.Printf("%s %s\n", string(kv.Key), string(kv.Value))
	}
}
