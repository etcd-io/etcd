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
	"reflect"
	"sort"
	"strings"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/bgentry/speakeasy"
	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/client"
)

func NewUserCommands() cli.Command {
	return cli.Command{
		Name:  "user",
		Usage: "user add, grant and revoke subcommands",
		Subcommands: []cli.Command{
			cli.Command{
				Name:   "add",
				Usage:  "add a new user for the etcd cluster",
				Action: actionUserAdd,
			},
			cli.Command{
				Name:   "get",
				Usage:  "get details for a user",
				Action: actionUserGet,
			},
			cli.Command{
				Name:   "list",
				Usage:  "list all current users",
				Action: actionUserList,
			},
			cli.Command{
				Name:   "remove",
				Usage:  "remove a user for the etcd cluster",
				Action: actionUserRemove,
			},
			cli.Command{
				Name:   "grant",
				Usage:  "grant roles to an etcd user",
				Flags:  []cli.Flag{cli.StringSliceFlag{Name: "roles", Value: new(cli.StringSlice), Usage: "List of roles to grant or revoke"}},
				Action: actionUserGrant,
			},
			cli.Command{
				Name:   "revoke",
				Usage:  "revoke roles for an etcd user",
				Flags:  []cli.Flag{cli.StringSliceFlag{Name: "roles", Value: new(cli.StringSlice), Usage: "List of roles to grant or revoke"}},
				Action: actionUserRevoke,
			},
			cli.Command{
				Name:   "passwd",
				Usage:  "change password for a user",
				Action: actionUserPasswd,
			},
		},
	}
}

func mustNewAuthUserAPI(c *cli.Context) client.AuthUserAPI {
	hc := mustNewClient(c)

	if c.GlobalBool("debug") {
		fmt.Fprintf(os.Stderr, "Cluster-Endpoints: %s\n", strings.Join(hc.Endpoints(), ", "))
	}

	return client.NewAuthUserAPI(hc)
}

func actionUserList(c *cli.Context) {
	if len(c.Args()) != 0 {
		fmt.Fprintln(os.Stderr, "No arguments accepted")
		os.Exit(1)
	}
	u := mustNewAuthUserAPI(c)
	ctx, cancel := context.WithTimeout(context.Background(), client.DefaultRequestTimeout)
	users, err := u.ListUsers(ctx)
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	for _, user := range users {
		fmt.Printf("%s\n", user)
	}
}

func actionUserAdd(c *cli.Context) {
	api, user := mustUserAPIAndName(c)
	ctx, cancel := context.WithTimeout(context.Background(), client.DefaultRequestTimeout)
	currentUser, err := api.GetUser(ctx, user)
	cancel()
	if currentUser != nil {
		fmt.Fprintf(os.Stderr, "User %s already exists\n", user)
		os.Exit(1)
	}
	pass, err := speakeasy.Ask("New password: ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error reading password:", err)
		os.Exit(1)
	}
	ctx, cancel = context.WithTimeout(context.Background(), client.DefaultRequestTimeout)
	err = api.AddUser(ctx, user, pass)
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	fmt.Printf("User %s created\n", user)
}

func actionUserRemove(c *cli.Context) {
	api, user := mustUserAPIAndName(c)
	ctx, cancel := context.WithTimeout(context.Background(), client.DefaultRequestTimeout)
	err := api.RemoveUser(ctx, user)
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	fmt.Printf("User %s removed\n", user)
}

func actionUserPasswd(c *cli.Context) {
	api, user := mustUserAPIAndName(c)
	ctx, cancel := context.WithTimeout(context.Background(), client.DefaultRequestTimeout)
	currentUser, err := api.GetUser(ctx, user)
	cancel()
	if currentUser == nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	pass, err := speakeasy.Ask("New password: ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error reading password:", err)
		os.Exit(1)
	}

	ctx, cancel = context.WithTimeout(context.Background(), client.DefaultRequestTimeout)
	_, err = api.ChangePassword(ctx, user, pass)
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	fmt.Printf("Password updated\n")
}

func actionUserGrant(c *cli.Context) {
	userGrantRevoke(c, true)
}

func actionUserRevoke(c *cli.Context) {
	userGrantRevoke(c, false)
}

func userGrantRevoke(c *cli.Context, grant bool) {
	roles := c.StringSlice("roles")
	if len(roles) == 0 {
		fmt.Fprintln(os.Stderr, "No roles specified; please use `-roles`")
		os.Exit(1)
	}

	api, user := mustUserAPIAndName(c)
	ctx, cancel := context.WithTimeout(context.Background(), client.DefaultRequestTimeout)
	currentUser, err := api.GetUser(ctx, user)
	cancel()
	if currentUser == nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	ctx, cancel = context.WithTimeout(context.Background(), client.DefaultRequestTimeout)
	var newUser *client.User
	if grant {
		newUser, err = api.GrantUser(ctx, user, roles)
	} else {
		newUser, err = api.RevokeUser(ctx, user, roles)
	}
	cancel()
	sort.Strings(newUser.Roles)
	sort.Strings(currentUser.Roles)
	if reflect.DeepEqual(newUser.Roles, currentUser.Roles) {
		if grant {
			fmt.Printf("User unchanged; roles already granted")
		} else {
			fmt.Printf("User unchanged; roles already revoked")
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	fmt.Printf("User %s updated\n", user)
}

func actionUserGet(c *cli.Context) {
	api, username := mustUserAPIAndName(c)
	ctx, cancel := context.WithTimeout(context.Background(), client.DefaultRequestTimeout)
	user, err := api.GetUser(ctx, username)
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	fmt.Printf("User: %s\n", user.User)
	fmt.Printf("Roles: %s\n", strings.Join(user.Roles, " "))

}

func mustUserAPIAndName(c *cli.Context) (client.AuthUserAPI, string) {
	args := c.Args()
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "Please provide a username")
		os.Exit(1)
	}

	api := mustNewAuthUserAPI(c)
	username := args[0]
	return api, username
}
