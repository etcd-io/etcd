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
	"encoding/json"
	"fmt"
	"os"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
	"github.com/coreos/etcd/client"
)

// printKey writes the etcd response to STDOUT in the given format.
func printKey(resp *etcd.Response, format string) {
	// printKey is only for keys, error on directories
	if resp.Node.Dir == true {
		fmt.Fprintln(os.Stderr, fmt.Sprintf("Cannot print key [%s: Is a directory]", resp.Node.Key))
		os.Exit(1)
	}
	printKeyOnly(resp, format)
}

// printAll prints the etcd response in the given format in its best efforts.
func printAll(resp *etcd.Response, format string) {
	if resp.Node.Dir == true {
		return
	}
	printKeyOnly(resp, format)
}

// printKeyOnly only supports to print key correctly.
func printKeyOnly(resp *etcd.Response, format string) {
	// Format the result.
	switch format {
	case "simple":
		fmt.Println(resp.Node.Value)
	case "extended":
		// Extended prints in a rfc2822 style format
		fmt.Println("Key:", resp.Node.Key)
		fmt.Println("Created-Index:", resp.Node.CreatedIndex)
		fmt.Println("Modified-Index:", resp.Node.ModifiedIndex)

		if resp.PrevNode != nil {
			fmt.Println("PrevNode.Value:", resp.PrevNode.Value)
		}

		fmt.Println("TTL:", resp.Node.TTL)
		fmt.Println("Etcd-Index:", resp.EtcdIndex)
		fmt.Println("Raft-Index:", resp.RaftIndex)
		fmt.Println("Raft-Term:", resp.RaftTerm)
		fmt.Println("")
		fmt.Println(resp.Node.Value)
	case "json":
		b, err := json.Marshal(resp)
		if err != nil {
			panic(err)
		}
		fmt.Println(string(b))
	default:
		fmt.Fprintln(os.Stderr, "Unsupported output format:", format)
	}
}

// printResponseKey only supports to print key correctly.
func printResponseKey(resp *client.Response, format string) {
	// Format the result.
	switch format {
	case "simple":
		fmt.Println(resp.Node.Value)
	case "extended":
		// Extended prints in a rfc2822 style format
		fmt.Println("Key:", resp.Node.Key)
		fmt.Println("Created-Index:", resp.Node.CreatedIndex)
		fmt.Println("Modified-Index:", resp.Node.ModifiedIndex)

		if resp.PrevNode != nil {
			fmt.Println("PrevNode.Value:", resp.PrevNode.Value)
		}

		fmt.Println("TTL:", resp.Node.TTL)
		fmt.Println("Index:", resp.Index)
		fmt.Println("")
		fmt.Println(resp.Node.Value)
	case "json":
		b, err := json.Marshal(resp)
		if err != nil {
			panic(err)
		}
		fmt.Println(string(b))
	default:
		fmt.Fprintln(os.Stderr, "Unsupported output format:", format)
	}
}
