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
	"os"
	"strings"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/client"
)

func NewMemberCommand() cli.Command {
	return cli.Command{
		Name:  "member",
		Usage: "member add, remove and list subcommands",
		Subcommands: []cli.Command{
			cli.Command{
				Name:   "list",
				Usage:  "enumerate existing cluster members",
				Action: actionMemberList,
			},
			cli.Command{
				Name:   "add",
				Usage:  "add a new member to the etcd cluster",
				Action: actionMemberAdd,
			},
			cli.Command{
				Name:   "remove",
				Usage:  "remove an existing member from the etcd cluster",
				Action: actionMemberRemove,
			},
		},
	}
}

func actionMemberList(c *cli.Context) {
	if len(c.Args()) != 0 {
		fmt.Fprintln(os.Stderr, "No arguments accepted")
		os.Exit(1)
	}
	mAPI := mustNewMembersAPI(c)
	ctx, cancel := context.WithTimeout(context.Background(), client.DefaultRequestTimeout)
	members, err := mAPI.List(ctx)
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	for _, m := range members {
		if len(m.Name) == 0 {
			fmt.Printf("%s[unstarted]: peerURLs=%s\n", m.ID, strings.Join(m.PeerURLs, ","))
		} else {
			fmt.Printf("%s: name=%s peerURLs=%s clientURLs=%s\n", m.ID, m.Name, strings.Join(m.PeerURLs, ","), strings.Join(m.ClientURLs, ","))
		}
	}
}

func actionMemberAdd(c *cli.Context) {
	args := c.Args()
	if len(args) != 2 {
		fmt.Fprintln(os.Stderr, "Provide a name and a single member peerURL")
		os.Exit(1)
	}

	mAPI := mustNewMembersAPI(c)

	url := args[1]
	ctx, cancel := context.WithTimeout(context.Background(), client.DefaultRequestTimeout)
	m, err := mAPI.Add(ctx, url)
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	newID := m.ID
	newName := args[0]
	fmt.Printf("Added member named %s with ID %s to cluster\n", newName, newID)

	ctx, cancel = context.WithTimeout(context.Background(), client.DefaultRequestTimeout)
	members, err := mAPI.List(ctx)
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	conf := []string{}
	for _, memb := range members {
		for _, u := range memb.PeerURLs {
			n := memb.Name
			if memb.ID == newID {
				n = newName
			}
			conf = append(conf, fmt.Sprintf("%s=%s", n, u))
		}
	}

	fmt.Print("\n")
	fmt.Printf("ETCD_NAME=%q\n", newName)
	fmt.Printf("ETCD_INITIAL_CLUSTER=%q\n", strings.Join(conf, ","))
	fmt.Printf("ETCD_INITIAL_CLUSTER_STATE=\"existing\"\n")
}

func actionMemberRemove(c *cli.Context) {
	args := c.Args()
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "Provide a single member ID")
		os.Exit(1)
	}
	removalID := args[0]

	mAPI := mustNewMembersAPI(c)
	// Get the list of members.
	listctx, listCancel := context.WithTimeout(context.Background(), client.DefaultRequestTimeout)
	members, err := mAPI.List(listctx)
	listCancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error while verifying ID against known members:", err.Error())
		os.Exit(1)
	}
	// Sanity check the input.
	foundID := false
	for _, m := range members {
		if m.ID == removalID {
			foundID = true
		}
		if m.Name == removalID {
			// Note that, so long as it's not ambiguous, we *could* do the right thing by name here.
			fmt.Fprintf(os.Stderr, "Found a member named %s; if this is correct, please use its ID, eg:\n\tetcdctl member remove %s\n", m.Name, m.ID)
			fmt.Fprintf(os.Stderr, "For more details, read the documentation at https://github.com/coreos/etcd/blob/master/Documentation/runtime-configuration.md#remove-a-member\n\n")
		}
	}
	if !foundID {
		fmt.Fprintf(os.Stderr, "Couldn't find a member in the cluster with an ID of %s.\n", removalID)
		os.Exit(1)
	}

	// Actually attempt to remove the member.
	ctx, removeCancel := context.WithTimeout(context.Background(), client.DefaultRequestTimeout)
	err = mAPI.Remove(ctx, removalID)
	removeCancel()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Recieved an error trying to remove member %s: %s", removalID, err.Error())
		os.Exit(1)
	}

	fmt.Printf("Removed member %s from cluster\n", removalID)
}
