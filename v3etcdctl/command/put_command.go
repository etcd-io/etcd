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

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/grpc"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
)

// NewPutCommand returns the CLI command for "put".
func NewPutCommand() cli.Command {
	return cli.Command{
		Name: "put",
		Action: func(c *cli.Context) {
			putCommandFunc(c)
		},
	}
}

// putCommandFunc executes the "put" command.
func putCommandFunc(c *cli.Context) {
	if len(c.Args()) != 2 {
		panic("bad arg")
	}

	key := []byte(c.Args()[0])
	value := []byte(c.Args()[1])
	conn, err := grpc.Dial("127.0.0.1:12379")
	if err != nil {
		panic(err)
	}
	etcd := pb.NewEtcdClient(conn)
	req := &pb.PutRequest{Key: key, Value: value}

	etcd.Put(context.Background(), req)
	fmt.Printf("%s %s\n", key, value)
}
