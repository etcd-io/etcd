/*
   Copyright 2014 CoreOS, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

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

func mustNewMembersAPI(c *cli.Context) client.MembersAPI {
	eps, err := getEndpoints(c)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	tr, err := getTransport(c)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	hc, err := client.NewHTTPClient(tr, eps)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	if !c.GlobalBool("no-sync") {
		ctx, cancel := context.WithTimeout(context.Background(), client.DefaultRequestTimeout)
		err := hc.Sync(ctx)
		cancel()
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	}

	if c.GlobalBool("debug") {
		fmt.Fprintf(os.Stderr, "Cluster-Endpoints: %s\n", strings.Join(hc.Endpoints(), ", "))
	}

	return client.NewMembersAPI(hc)
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
		fmt.Printf("%s: name=%s peerURLs=%s clientURLs=%s\n", m.ID, m.Name, strings.Join(m.PeerURLs, ","), strings.Join(m.ClientURLs, ","))
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
	for _, m := range members {
		for _, u := range m.PeerURLs {
			n := m.Name
			if m.ID == newID {
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

	mAPI := mustNewMembersAPI(c)
	mID := args[0]
	ctx, cancel := context.WithTimeout(context.Background(), client.DefaultRequestTimeout)
	err := mAPI.Remove(ctx, mID)
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	fmt.Printf("Removed member %s from cluster\n", mID)
}
