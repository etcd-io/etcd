// Copyright 2016 Nippon Telegraph and Telephone Corporation.
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

	return ac
}

func newRoleAddCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "add <role name>",
		Short: "add a new role",
		Run:   roleAddCommandFunc,
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
