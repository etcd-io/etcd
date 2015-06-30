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
	"strings"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/spf13/cobra"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/client"
)

func NewRoleCommands() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "role",
		Short: "role add, grant and revoke subcommands",
	}

	add := &cobra.Command{
		Use:   "add",
		Short: "add a new role for the etcd cluster",
		Run:   actionRoleAdd,
	}

	get := &cobra.Command{
		Use:   "get",
		Short: "get details for a role",
		Run:   actionRoleGet,
	}

	list := &cobra.Command{
		Use:   "list",
		Short: "list all roles",
		Run:   actionRoleList,
	}

	remove := &cobra.Command{
		Use:   "remove",
		Short: "remove a role from the etcd cluster",
		Run:   actionRoleRemove,
	}

	grant := &cobra.Command{
		Use:   "grant",
		Short: "grant path matches to an etcd role",
		Run:   actionRoleGrant,
	}
	_ = grant.Flags().String("path", "", "Path granted for the role to access")
	_ = grant.Flags().Bool("read", false, "Grant read-only access")
	_ = grant.Flags().Bool("write", false, "Grant write-only access")
	_ = grant.Flags().Bool("readwrite", false, "Grant read-write access")

	revoke := &cobra.Command{
		Use:   "revoke",
		Short: "revoke path matches for an etcd role",
		Run:   actionRoleRevoke,
	}
	_ = revoke.Flags().String("path", "", "Path granted for the role to access")
	_ = revoke.Flags().Bool("read", false, "Grant read-only access")
	_ = revoke.Flags().Bool("write", false, "Grant write-only access")
	_ = revoke.Flags().Bool("readwrite", false, "Grant read-write access")

	cmd.AddCommand(add, get, list, remove, grant, revoke)

	return cmd
}

func mustNewAuthRoleAPI(cmd *cobra.Command) client.AuthRoleAPI {
	hc := mustNewClient(cmd)

	if d, _ := cmd.Flags().GetBool("debug"); d {
		fmt.Fprintf(os.Stderr, "Cluster-Endpoints: %s\n", strings.Join(hc.Endpoints(), ", "))
	}

	return client.NewAuthRoleAPI(hc)
}

func actionRoleList(cmd *cobra.Command, args []string) {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "No arguments accepted")
		os.Exit(1)
	}
	r := mustNewAuthRoleAPI(cmd)
	ctx, cancel := context.WithTimeout(context.Background(), client.DefaultRequestTimeout)
	roles, err := r.ListRoles(ctx)
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	for _, role := range roles {
		fmt.Printf("%s\n", role)
	}
}

func actionRoleAdd(cmd *cobra.Command, args []string) {
	api, role := mustRoleAPIAndName(cmd, args)
	ctx, cancel := context.WithTimeout(context.Background(), client.DefaultRequestTimeout)
	currentRole, err := api.GetRole(ctx, role)
	cancel()
	if currentRole != nil {
		fmt.Fprintf(os.Stderr, "Role %s already exists\n", role)
		os.Exit(1)
	}
	ctx, cancel = context.WithTimeout(context.Background(), client.DefaultRequestTimeout)
	err = api.AddRole(ctx, role)
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	fmt.Printf("Role %s created\n", role)
}

func actionRoleRemove(cmd *cobra.Command, args []string) {
	api, role := mustRoleAPIAndName(cmd, args)
	ctx, cancel := context.WithTimeout(context.Background(), client.DefaultRequestTimeout)
	err := api.RemoveRole(ctx, role)
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	fmt.Printf("Role %s removed\n", role)
}

func actionRoleGrant(cmd *cobra.Command, args []string) {
	roleGrantRevoke(cmd, args, true)
}

func actionRoleRevoke(cmd *cobra.Command, args []string) {
	roleGrantRevoke(cmd, args, false)
}

func roleGrantRevoke(cmd *cobra.Command, args []string, grant bool) {
	path, _ := cmd.Flags().GetString("path")
	if path == "" {
		fmt.Fprintln(os.Stderr, "No path specified; please use `-path`")
		os.Exit(1)
	}

	read, _ := cmd.Flags().GetBool("read")
	write, _ := cmd.Flags().GetBool("write")
	rw, _ := cmd.Flags().GetBool("readwrite")
	permcount := 0
	for _, v := range []bool{read, write, rw} {
		if v {
			permcount++
		}
	}
	if permcount != 1 {
		fmt.Fprintln(os.Stderr, "Please specify exactly one of -read, -write or -readwrite")
		os.Exit(1)
	}
	var permType client.PermissionType
	switch {
	case read:
		permType = client.ReadPermission
	case write:
		permType = client.WritePermission
	case rw:
		permType = client.ReadWritePermission
	}

	api, role := mustRoleAPIAndName(cmd, args)
	ctx, cancel := context.WithTimeout(context.Background(), client.DefaultRequestTimeout)
	currentRole, err := api.GetRole(ctx, role)
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	ctx, cancel = context.WithTimeout(context.Background(), client.DefaultRequestTimeout)
	var newRole *client.Role
	if grant {
		newRole, err = api.GrantRoleKV(ctx, role, []string{path}, permType)
	} else {
		newRole, err = api.RevokeRoleKV(ctx, role, []string{path}, permType)
	}
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	if reflect.DeepEqual(newRole, currentRole) {
		if grant {
			fmt.Printf("Role unchanged; already granted")
		} else {
			fmt.Printf("Role unchanged; already revoked")
		}
	}

	fmt.Printf("Role %s updated\n", role)
}

func actionRoleGet(cmd *cobra.Command, args []string) {
	api, rolename := mustRoleAPIAndName(cmd, args)

	ctx, cancel := context.WithTimeout(context.Background(), client.DefaultRequestTimeout)
	role, err := api.GetRole(ctx, rolename)
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	fmt.Printf("Role: %s\n", role.Role)
	fmt.Printf("KV Read:\n")
	for _, v := range role.Permissions.KV.Read {
		fmt.Printf("\t%s\n", v)
	}
	fmt.Printf("KV Write:\n")
	for _, v := range role.Permissions.KV.Write {
		fmt.Printf("\t%s\n", v)
	}
}

func mustRoleAPIAndName(cmd *cobra.Command, args []string) (client.AuthRoleAPI, string) {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "Please provide a role name")
		os.Exit(1)
	}

	name := args[0]
	api := mustNewAuthRoleAPI(cmd)
	return api, name
}
