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
	"os"
	"strconv"
	"strings"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/spf13/cobra"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
)

// NewTxnCommand returns the cobra command for "txn".
func NewTxnCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "txn",
		Short: "Txn processes all the requests in one transaction.",
		Run:   txnCommandFunc,
	}
}

// txnCommandFunc executes the "txn" command.
func txnCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) != 0 {
		ExitWithError(ExitBadArgs, fmt.Errorf("txn command does not accept argument."))
	}

	reader := bufio.NewReader(os.Stdin)

	next := compareState
	txn := &pb.TxnRequest{}
	for next != nil {
		next = next(txn, reader)
	}

	resp, err := mustClient(cmd).KV.Txn(context.Background(), txn)
	if err != nil {
		ExitWithError(ExitError, err)
	}
	if resp.Succeeded {
		fmt.Println("executed success request list")
	} else {
		fmt.Println("executed failure request list")
	}
}

type stateFunc func(txn *pb.TxnRequest, r *bufio.Reader) stateFunc

func compareState(txn *pb.TxnRequest, r *bufio.Reader) stateFunc {
	fmt.Println("entry comparison[key target expected_result compare_value] (end with empty line):")

	line, err := r.ReadString('\n')
	if err != nil {
		ExitWithError(ExitInvalidInput, err)
	}

	if len(line) == 1 {
		return successState
	}

	// remove trialling \n
	line = line[:len(line)-1]
	c, err := parseCompare(line)
	if err != nil {
		ExitWithError(ExitInvalidInput, err)
	}

	txn.Compare = append(txn.Compare, c)

	return compareState
}

func successState(txn *pb.TxnRequest, r *bufio.Reader) stateFunc {
	fmt.Println("entry success request[method key value(end_range)] (end with empty line):")

	line, err := r.ReadString('\n')
	if err != nil {
		ExitWithError(ExitInvalidInput, err)
	}

	if len(line) == 1 {
		return failureState
	}

	// remove trialling \n
	line = line[:len(line)-1]
	ru, err := parseRequestUnion(line)
	if err != nil {
		ExitWithError(ExitInvalidInput, err)
	}

	txn.Success = append(txn.Success, ru)

	return successState
}

func failureState(txn *pb.TxnRequest, r *bufio.Reader) stateFunc {
	fmt.Println("entry failure request[method key value(end_range)] (end with empty line):")

	line, err := r.ReadString('\n')
	if err != nil {
		ExitWithError(ExitInvalidInput, err)
	}

	if len(line) == 1 {
		return nil
	}

	// remove trialling \n
	line = line[:len(line)-1]
	ru, err := parseRequestUnion(line)
	if err != nil {
		ExitWithError(ExitInvalidInput, err)
	}

	txn.Failure = append(txn.Failure, ru)

	return failureState
}

func parseRequestUnion(line string) (*pb.RequestUnion, error) {
	parts := strings.Split(line, " ")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid txn compare request: %s", line)
	}

	ru := &pb.RequestUnion{}
	key := []byte(parts[1])
	switch parts[0] {
	case "r", "range":
		if len(parts) == 3 {
			ru.Request = &pb.RequestUnion_RequestRange{
				RequestRange: &pb.RangeRequest{
					Key:      key,
					RangeEnd: []byte(parts[2]),
				}}
		} else {
			ru.Request = &pb.RequestUnion_RequestRange{
				RequestRange: &pb.RangeRequest{
					Key: key,
				}}
		}
	case "p", "put":
		ru.Request = &pb.RequestUnion_RequestPut{
			RequestPut: &pb.PutRequest{
				Key:   key,
				Value: []byte(parts[2]),
			}}
	case "d", "deleteRange":
		if len(parts) == 3 {
			ru.Request = &pb.RequestUnion_RequestDeleteRange{
				RequestDeleteRange: &pb.DeleteRangeRequest{
					Key:      key,
					RangeEnd: []byte(parts[2]),
				}}
		} else {
			ru.Request = &pb.RequestUnion_RequestDeleteRange{
				RequestDeleteRange: &pb.DeleteRangeRequest{
					Key: key,
				}}
		}
	default:
		return nil, fmt.Errorf("invalid txn request: %s", line)
	}
	return ru, nil
}

func parseCompare(line string) (*pb.Compare, error) {
	parts := strings.Split(line, " ")
	if len(parts) != 4 {
		return nil, fmt.Errorf("invalid txn compare request: %s", line)
	}

	var err error
	c := &pb.Compare{}
	c.Key = []byte(parts[0])
	switch parts[1] {
	case "ver", "version":
		tv, _ := c.TargetUnion.(*pb.Compare_Version)
		if tv != nil {
			tv.Version, err = strconv.ParseInt(parts[3], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid txn compare request: %s", line)
			}
		}
	case "c", "create":
		tv, _ := c.TargetUnion.(*pb.Compare_CreateRevision)
		if tv != nil {
			tv.CreateRevision, err = strconv.ParseInt(parts[3], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid txn compare request: %s", line)
			}
		}
	case "m", "mod":
		tv, _ := c.TargetUnion.(*pb.Compare_ModRevision)
		if tv != nil {
			tv.ModRevision, err = strconv.ParseInt(parts[3], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid txn compare request: %s", line)
			}
		}
	case "val", "value":
		tv, _ := c.TargetUnion.(*pb.Compare_Value)
		if tv != nil {
			tv.Value = []byte(parts[3])
		}
	default:
		return nil, fmt.Errorf("invalid txn compare request: %s", line)
	}

	switch parts[2] {
	case "g", "greater":
		c.Result = pb.Compare_GREATER
	case "e", "equal":
		c.Result = pb.Compare_EQUAL
	case "l", "less":
		c.Result = pb.Compare_LESS
	default:
		return nil, fmt.Errorf("invalid txn compare request: %s", line)
	}
	return c, nil
}
