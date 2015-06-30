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
	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/spf13/cobra"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/client"
)

func NewUserCommands() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "user add, grant and revoke subcommands",
	}

	add := &cobra.Command{
		Use:   "add",
		Short: "add a new user for the etcd cluster",
		Run:   actionUserAdd,
	}

	get := &cobra.Command{
		Use:   "get",
		Short: "get details for a user",
		Run:   actionUserGet,
	}

	list := &cobra.Command{
		Use:   "list",
		Short: "list all current users",
		Run:   actionUserList,
	}

	remove := &cobra.Command{
		Use:   "remove",
		Short: "remove a user for the etcd cluster",
		Run:   actionUserRemove,
	}

	grant := &cobra.Command{
		Use:   "grant",
		Short: "grant roles to an etcd user",
		Run:   actionUserGrant,
	}
	_ = grant.Flags().StringSlice("roles", []string{}, "List of roles to grant")

	revoke := &cobra.Command{
		Use:   "revoke",
		Short: "revoke roles for an etcd user",
		Run:   actionUserRevoke,
	}
	_ = revoke.Flags().StringSlice("roles", []string{}, "List of roles to revoke")

	passwd := &cobra.Command{
		Use:   "passwd",
		Short: "change password for a user",
		Run:   actionUserPasswd,
	}

	cmd.AddCommand(add, get, list, remove, grant, revoke, passwd)
	return cmd
}

func mustNewAuthUserAPI(cmd *cobra.Command) client.AuthUserAPI {
	hc := mustNewClient(cmd)

	if d, _ := cmd.Flags().GetBool("debug"); d {
		fmt.Fprintf(os.Stderr, "Cluster-Endpoints: %s\n", strings.Join(hc.Endpoints(), ", "))
	}

	return client.NewAuthUserAPI(hc)
}

func actionUserList(cmd *cobra.Command, args []string) {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "No arguments accepted")
		os.Exit(1)
	}
	u := mustNewAuthUserAPI(cmd)
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

func actionUserAdd(cmd *cobra.Command, args []string) {
	api, user := mustUserAPIAndName(cmd, args)
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

func actionUserRemove(cmd *cobra.Command, args []string) {
	api, user := mustUserAPIAndName(cmd, args)
	ctx, cancel := context.WithTimeout(context.Background(), client.DefaultRequestTimeout)
	err := api.RemoveUser(ctx, user)
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	fmt.Printf("User %s removed\n", user)
}

func actionUserPasswd(cmd *cobra.Command, args []string) {
	api, user := mustUserAPIAndName(cmd, args)
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

func actionUserGrant(cmd *cobra.Command, args []string) {
	userGrantRevoke(cmd, args, true)
}

func actionUserRevoke(cmd *cobra.Command, args []string) {
	userGrantRevoke(cmd, args, false)
}

func userGrantRevoke(cmd *cobra.Command, args []string, grant bool) {
	roles, _ := cmd.Flags().GetStringSlice("roles")
	if len(roles) == 0 {
		fmt.Fprintln(os.Stderr, "No roles specified; please use `-roles`")
		os.Exit(1)
	}

	api, user := mustUserAPIAndName(cmd, args)
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

func actionUserGet(cmd *cobra.Command, args []string) {
	api, username := mustUserAPIAndName(cmd, args)
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

func mustUserAPIAndName(cmd *cobra.Command, args []string) (client.AuthUserAPI, string) {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "Please provide a username")
		os.Exit(1)
	}

	api := mustNewAuthUserAPI(cmd)
	username := args[0]
	return api, username
}
