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
	"fmt"

	"github.com/coreos/etcd/clientv3"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

// NewRoleCommand returns the cobra command for "role".
func NewRoleCommand() *cobra.Command {
	ac := &cobra.Command{
		Use:   "role <subcommand>",
		Short: "role related command",
	}

	ac.AddCommand(newRoleAddCommand())
	ac.AddCommand(newRoleGrantCommand())
	ac.AddCommand(newRoleGetCommand())

	return ac
}

func newRoleAddCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "add <role name>",
		Short: "add a new role",
		Run:   roleAddCommandFunc,
	}
}

func newRoleGrantCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "grant <role name> <permission type> <key>",
		Short: "grant a key to a role",
		Run:   roleGrantCommandFunc,
	}
}

func newRoleGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get <role name>",
		Short: "get detailed information of a role",
		Run:   roleGetCommandFunc,
	}
}

// roleAddCommandFunc executes the "role add" command.
func roleAddCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		ExitWithError(ExitBadArgs, fmt.Errorf("role add command requires role name as its argument."))
	}

	_, err := mustClientFromCmd(cmd).Auth.RoleAdd(context.TODO(), args[0])
	if err != nil {
		ExitWithError(ExitError, err)
	}

	fmt.Printf("Role %s created\n", args[0])
}

// roleGrantCommandFunc executes the "role grant" command.
func roleGrantCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) != 3 {
		ExitWithError(ExitBadArgs, fmt.Errorf("role grant command requires role name, permission type, and key as its argument."))
	}

	perm, err := clientv3.StrToPermissionType(args[1])
	if err != nil {
		ExitWithError(ExitBadArgs, err)
	}

	_, err = mustClientFromCmd(cmd).Auth.RoleGrant(context.TODO(), args[0], args[2], perm)
	if err != nil {
		ExitWithError(ExitError, err)
	}

	fmt.Printf("Role %s updated\n", args[0])
}

// roleGetCommandFunc executes the "role get" command.
func roleGetCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		ExitWithError(ExitBadArgs, fmt.Errorf("role get command requires role name as its argument."))
	}

	resp, err := mustClientFromCmd(cmd).Auth.RoleGet(context.TODO(), args[0])
	if err != nil {
		ExitWithError(ExitError, err)
	}

	fmt.Printf("Role %s\n", args[0])
	fmt.Println("KV Read:")
	for _, perm := range resp.Perm {
		if perm.PermType == clientv3.PermRead || perm.PermType == clientv3.PermReadWrite {
			fmt.Printf("\t%s\n", string(perm.Key))
		}
	}
	fmt.Println("KV Write:")
	for _, perm := range resp.Perm {
		if perm.PermType == clientv3.PermWrite || perm.PermType == clientv3.PermReadWrite {
			fmt.Printf("\t%s\n", string(perm.Key))
		}
	}
}
