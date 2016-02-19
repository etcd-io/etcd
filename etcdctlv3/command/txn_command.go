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
	"github.com/coreos/etcd/clientv3"
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

	txn := clientv3.NewKV(mustClientFromCmd(cmd)).Txn(context.Background())
	fmt.Println("entry comparison[key target expected_result compare_value] (end with empty line):")
	txn.If(readCompares(reader)...)
	fmt.Println("entry success request[method key value(end_range)] (end with empty line):")
	txn.Then(readOps(reader)...)
	fmt.Println("entry failure request[method key value(end_range)] (end with empty line):")
	txn.Else(readOps(reader)...)

	resp, err := txn.Commit()
	if err != nil {
		ExitWithError(ExitError, err)
	}
	if resp.Succeeded {
		fmt.Println("executed success request list")
	} else {
		fmt.Println("executed failure request list")
	}
}

func readCompares(r *bufio.Reader) (cmps []clientv3.Cmp) {
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			ExitWithError(ExitInvalidInput, err)
		}
		if len(line) == 1 {
			break
		}

		// remove trialling \n
		line = line[:len(line)-1]
		cmp, err := parseCompare(line)
		if err != nil {
			ExitWithError(ExitInvalidInput, err)
		}
		cmps = append(cmps, *cmp)
	}

	return cmps
}

func readOps(r *bufio.Reader) (ops []clientv3.Op) {
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			ExitWithError(ExitInvalidInput, err)
		}
		if len(line) == 1 {
			break
		}

		// remove trialling \n
		line = line[:len(line)-1]
		op, err := parseRequestUnion(line)
		if err != nil {
			ExitWithError(ExitInvalidInput, err)
		}
		ops = append(ops, *op)
	}

	return ops
}

func parseRequestUnion(line string) (*clientv3.Op, error) {
	parts := strings.Split(line, " ")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid txn compare request: %s", line)
	}

	op := &clientv3.Op{}
	key := parts[1]
	switch parts[0] {
	case "r", "range":
		if len(parts) == 3 {
			*op = clientv3.OpGet(key, clientv3.WithRange(parts[2]))
		} else {
			*op = clientv3.OpGet(key)
		}
	case "p", "put":
		*op = clientv3.OpPut(key, parts[2])
	case "d", "deleteRange":
		if len(parts) == 3 {
			*op = clientv3.OpDelete(key, clientv3.WithRange(parts[2]))
		} else {
			*op = clientv3.OpDelete(key)
		}
	default:
		return nil, fmt.Errorf("invalid txn request: %s", line)
	}
	return op, nil
}

func parseCompare(line string) (*clientv3.Cmp, error) {
	parts := strings.Split(line, " ")
	if len(parts) != 4 {
		return nil, fmt.Errorf("invalid txn compare request: %s", line)
	}

	cmpType := ""
	switch parts[2] {
	case "g", "greater":
		cmpType = ">"
	case "e", "equal":
		cmpType = "="
	case "l", "less":
		cmpType = "<"
	default:
		return nil, fmt.Errorf("invalid txn compare request: %s", line)
	}

	var (
		v   int64
		err error
		cmp clientv3.Cmp
	)

	key := parts[0]
	switch parts[1] {
	case "ver", "version":
		if v, err = strconv.ParseInt(parts[3], 10, 64); err != nil {
			cmp = clientv3.Compare(clientv3.Version(key), cmpType, v)
		}
	case "c", "create":
		if v, err = strconv.ParseInt(parts[3], 10, 64); err != nil {
			cmp = clientv3.Compare(clientv3.CreatedRevision(key), cmpType, v)
		}
	case "m", "mod":
		if v, err = strconv.ParseInt(parts[3], 10, 64); err != nil {
			cmp = clientv3.Compare(clientv3.ModifiedRevision(key), cmpType, v)
		}
	case "val", "value":
		cmp = clientv3.Compare(clientv3.Value(key), cmpType, parts[3])
	}

	if err != nil {
		return nil, fmt.Errorf("invalid txn compare request: %s", line)
	}

	return &cmp, nil
}
