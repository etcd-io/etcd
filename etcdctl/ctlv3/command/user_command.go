// Copyright 2016 The etcd Authors
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
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/bgentry/speakeasy"
	"github.com/coreos/etcd/auth/authpb"
	"github.com/spf13/cobra"
)

var (
	userShowDetail bool
)

// NewUserCommand returns the cobra command for "user".
func NewUserCommand() *cobra.Command {
	ac := &cobra.Command{
		Use:   "user <subcommand>",
		Short: "User related commands",
	}

	ac.AddCommand(newUserAddCommand())
	ac.AddCommand(newUserDeleteCommand())
	ac.AddCommand(newUserGetCommand())
	ac.AddCommand(newUserListCommand())
	ac.AddCommand(newUserChangePasswordCommand())
	ac.AddCommand(newUserGrantRoleCommand())
	ac.AddCommand(newUserRevokeRoleCommand())
	ac.AddCommand(newUserListAclCommand())
	ac.AddCommand(newUserUpdateAclCommand())
	ac.AddCommand(newUserRevisionsCommand())

	return ac
}

var (
	passwordInteractive bool
)

func newUserAddCommand() *cobra.Command {
	cmd := cobra.Command{
		Use:   "add <user name or user:password> [options]",
		Short: "Adds a new user",
		Run:   userAddCommandFunc,
	}

	cmd.Flags().BoolVar(&passwordInteractive, "interactive", true, "Read password from stdin instead of interactive terminal")

	return &cmd
}

func newUserDeleteCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <user name>",
		Short: "Deletes a user",
		Run:   userDeleteCommandFunc,
	}
}

func newUserGetCommand() *cobra.Command {
	cmd := cobra.Command{
		Use:   "get <user name> [options]",
		Short: "Gets detailed information of a user",
		Run:   userGetCommandFunc,
	}

	cmd.Flags().BoolVar(&userShowDetail, "detail", false, "Show permissions of roles granted to the user")

	return &cmd
}

func newUserListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Lists all users",
		Run:   userListCommandFunc,
	}
}

func newUserChangePasswordCommand() *cobra.Command {
	cmd := cobra.Command{
		Use:   "passwd <user name> [options]",
		Short: "Changes password of user",
		Run:   userChangePasswordCommandFunc,
	}

	cmd.Flags().BoolVar(&passwordInteractive, "interactive", true, "If true, read password from stdin instead of interactive terminal")

	return &cmd
}

func newUserGrantRoleCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "grant-role <user name> <role name>",
		Short: "Grants a role to a user",
		Run:   userGrantRoleCommandFunc,
	}
}

func newUserRevokeRoleCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "revoke-role <user name> <role name>",
		Short: "Revokes a role from a user",
		Run:   userRevokeRoleCommandFunc,
	}
}

func newUserListAclCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "listacl <user name>",
		Short: "List user's acl",
		Run:   userListAclCommandFunc,
	}
}

func newUserUpdateAclCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "updateacl <user name> [path:rightsSet:rightsUnset,...]",
		Short: "Update user's acl",
		Run:   userUpdateAclCommandFunc,
	}
}

func newUserRevisionsCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "revisions <user name>",
		Short: "Get user revisions",
		Run:   userRevisionsCommandFunc,
	}
}

// userAddCommandFunc executes the "user add" command.
func userAddCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		ExitWithError(ExitBadArgs, fmt.Errorf("user add command requires user name as its argument."))
	}

	var password string
	var user string

	splitted := strings.SplitN(args[0], ":", 2)
	if len(splitted) < 2 {
		user = args[0]
		if !passwordInteractive {
			fmt.Scanf("%s", &password)
		} else {
			password = readPasswordInteractive(args[0])
		}
	} else {
		user = splitted[0]
		password = splitted[1]
		if len(user) == 0 {
			ExitWithError(ExitBadArgs, fmt.Errorf("empty user name is not allowed."))
		}
	}

	resp, err := mustClientFromCmd(cmd).Auth.UserAdd(context.TODO(), user, password)
	if err != nil {
		ExitWithError(ExitError, err)
	}

	display.UserAdd(user, *resp)
}

// userDeleteCommandFunc executes the "user delete" command.
func userDeleteCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		ExitWithError(ExitBadArgs, fmt.Errorf("user delete command requires user name as its argument."))
	}

	resp, err := mustClientFromCmd(cmd).Auth.UserDelete(context.TODO(), args[0])
	if err != nil {
		ExitWithError(ExitError, err)
	}
	display.UserDelete(args[0], *resp)
}

// userGetCommandFunc executes the "user get" command.
func userGetCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		ExitWithError(ExitBadArgs, fmt.Errorf("user get command requires user name as its argument."))
	}

	name := args[0]
	client := mustClientFromCmd(cmd)
	resp, err := client.Auth.UserGet(context.TODO(), name)
	if err != nil {
		ExitWithError(ExitError, err)
	}

	if userShowDetail {
		fmt.Printf("User: %s\n", name)
		for _, role := range resp.Roles {
			fmt.Printf("\n")
			roleResp, err := client.Auth.RoleGet(context.TODO(), role)
			if err != nil {
				ExitWithError(ExitError, err)
			}
			display.RoleGet(role, *roleResp)
		}
	} else {
		display.UserGet(name, *resp)
	}
}

// userListCommandFunc executes the "user list" command.
func userListCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) != 0 {
		ExitWithError(ExitBadArgs, fmt.Errorf("user list command requires no arguments."))
	}

	resp, err := mustClientFromCmd(cmd).Auth.UserList(context.TODO())
	if err != nil {
		ExitWithError(ExitError, err)
	}

	display.UserList(*resp)
}

// userChangePasswordCommandFunc executes the "user passwd" command.
func userChangePasswordCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		ExitWithError(ExitBadArgs, fmt.Errorf("user passwd command requires user name as its argument."))
	}

	var password string

	if !passwordInteractive {
		fmt.Scanf("%s", &password)
	} else {
		password = readPasswordInteractive(args[0])
	}

	resp, err := mustClientFromCmd(cmd).Auth.UserChangePassword(context.TODO(), args[0], password)
	if err != nil {
		ExitWithError(ExitError, err)
	}

	display.UserChangePassword(*resp)
}

// userGrantRoleCommandFunc executes the "user grant-role" command.
func userGrantRoleCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) != 2 {
		ExitWithError(ExitBadArgs, fmt.Errorf("user grant command requires user name and role name as its argument."))
	}

	resp, err := mustClientFromCmd(cmd).Auth.UserGrantRole(context.TODO(), args[0], args[1])
	if err != nil {
		ExitWithError(ExitError, err)
	}

	display.UserGrantRole(args[0], args[1], *resp)
}

// userRevokeRoleCommandFunc executes the "user revoke-role" command.
func userRevokeRoleCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) != 2 {
		ExitWithError(ExitBadArgs, fmt.Errorf("user revoke-role requires user name and role name as its argument."))
	}

	resp, err := mustClientFromCmd(cmd).Auth.UserRevokeRole(context.TODO(), args[0], args[1])
	if err != nil {
		ExitWithError(ExitError, err)
	}

	display.UserRevokeRole(args[0], args[1], *resp)
}

func readPasswordInteractive(name string) string {
	prompt1 := fmt.Sprintf("Password of %s: ", name)
	password1, err1 := speakeasy.Ask(prompt1)
	if err1 != nil {
		ExitWithError(ExitBadArgs, fmt.Errorf("failed to ask password: %s.", err1))
	}

	if len(password1) == 0 {
		ExitWithError(ExitBadArgs, fmt.Errorf("empty password"))
	}

	prompt2 := fmt.Sprintf("Type password of %s again for confirmation: ", name)
	password2, err2 := speakeasy.Ask(prompt2)
	if err2 != nil {
		ExitWithError(ExitBadArgs, fmt.Errorf("failed to ask password: %s.", err2))
	}

	if strings.Compare(password1, password2) != 0 {
		ExitWithError(ExitBadArgs, fmt.Errorf("given passwords are different."))
	}

	return password1
}

func userListAclCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		ExitWithError(ExitBadArgs, fmt.Errorf("user listacl command requires user name as its argument."))
	}

	name := args[0]
	client := mustClientFromCmd(cmd)
	resp, err := client.Auth.UserListAcl(context.TODO(), name)
	if err != nil {
		ExitWithError(ExitError, err)
	}

	display.UserListAcl(name, *resp)
}

func userUpdateAclCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		ExitWithError(ExitBadArgs, fmt.Errorf("user updateacl command requires user name as its argument."))
	}

	acl := []*authpb.AclEntry{}

	if len(args) >= 2 {
		entries := strings.SplitN(args[1], ":", -1)
		for _, entry := range entries {
			parts := strings.SplitN(entry, ",", 3)
			if len(parts) < 3 {
				ExitWithError(ExitBadArgs, fmt.Errorf("bad acl entry (%v)", entry))
			}
			rightsSet, err := strconv.ParseInt(parts[1], 10, 32)
			if err != nil {
				ExitWithError(ExitBadArgs, fmt.Errorf("bad rightsSet (%v)", err))
			}
			rightsUnset, err := strconv.ParseInt(parts[2], 10, 32)
			if err != nil {
				ExitWithError(ExitBadArgs, fmt.Errorf("bad rightsUnset (%v)", err))
			}
			acl = append(acl,
				&authpb.AclEntry{Path: parts[0],
					RightsSet:   uint32(rightsSet),
					RightsUnset: uint32(rightsUnset)})
		}
	}

	resp, err := mustClientFromCmd(cmd).Auth.UserUpdateAcl(context.TODO(), args[0], acl)
	if err != nil {
		ExitWithError(ExitError, err)
	}
	display.UserUpdateAcl(args[0], acl, *resp)
}

func userRevisionsCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		ExitWithError(ExitBadArgs, fmt.Errorf("user revisions command requires user name as its argument."))
	}

	name := args[0]
	client := mustClientFromCmd(cmd)
	resp, err := client.Auth.UserRevisions(context.TODO(), name)
	if err != nil {
		ExitWithError(ExitError, err)
	}

	display.UserRevisions(*resp)
}
