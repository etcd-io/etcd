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

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/grpc"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
)

// NewTxnCommand returns the CLI command for "txn".
func NewTxnCommand() cli.Command {
	return cli.Command{
		Name: "txn",
		Action: func(c *cli.Context) {
			txnCommandFunc(c)
		},
	}
}

// txnCommandFunc executes the "txn" command.
func txnCommandFunc(c *cli.Context) {
	if len(c.Args()) != 0 {
		panic("unexpected args")
	}

	reader := bufio.NewReader(os.Stdin)

	next := compareState
	txn := &pb.TxnRequest{}
	for next != nil {
		next = next(txn, reader)
	}

	conn, err := grpc.Dial(c.GlobalString("endpoint"))
	if err != nil {
		panic(err)
	}
	etcd := pb.NewEtcdClient(conn)

	resp, err := etcd.Txn(context.Background(), txn)
	if err != nil {
		fmt.Println(err)
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
		os.Exit(1)
	}

	if len(line) == 1 {
		return successState
	}

	// remove trialling \n
	line = line[:len(line)-1]
	c, err := parseCompare(line)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	txn.Compare = append(txn.Compare, c)

	return compareState
}

func successState(txn *pb.TxnRequest, r *bufio.Reader) stateFunc {
	fmt.Println("entry success request[method key value(end_range)] (end with empty line):")

	line, err := r.ReadString('\n')
	if err != nil {
		os.Exit(1)
	}

	if len(line) == 1 {
		return failureState
	}

	// remove trialling \n
	line = line[:len(line)-1]
	ru, err := parseRequestUnion(line)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	txn.Success = append(txn.Success, ru)

	return successState
}

func failureState(txn *pb.TxnRequest, r *bufio.Reader) stateFunc {
	fmt.Println("entry failure request[method key value(end_range)] (end with empty line):")

	line, err := r.ReadString('\n')
	if err != nil {
		os.Exit(1)
	}

	if len(line) == 1 {
		return nil
	}

	// remove trialling \n
	line = line[:len(line)-1]
	ru, err := parseRequestUnion(line)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
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
		ru.RequestRange = &pb.RangeRequest{Key: key}
		if len(parts) == 3 {
			ru.RequestRange.RangeEnd = []byte(parts[2])
		}
	case "p", "put":
		ru.RequestPut = &pb.PutRequest{Key: key, Value: []byte(parts[2])}
	case "d", "deleteRange":
		ru.RequestDeleteRange = &pb.DeleteRangeRequest{Key: key}
		if len(parts) == 3 {
			ru.RequestRange.RangeEnd = []byte(parts[2])
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
		c.Target = pb.Compare_VERSION
		c.Version, err = strconv.ParseInt(parts[3], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid txn compare request: %s", line)
		}
	case "c", "create":
		c.Target = pb.Compare_CREATE
		c.CreateRevision, err = strconv.ParseInt(parts[3], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid txn compare request: %s", line)
		}
	case "m", "mod":
		c.Target = pb.Compare_MOD
		c.ModRevision, err = strconv.ParseInt(parts[3], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid txn compare request: %s", line)
		}
	case "val", "value":
		c.Target = pb.Compare_VALUE
		c.Value = []byte(parts[3])
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
