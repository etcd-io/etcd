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
	"strings"

	"github.com/bgentry/speakeasy"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

// NewUserCommand returns the cobra command for "user".
func NewUserCommand() *cobra.Command {
	ac := &cobra.Command{
		Use:   "user <subcommand>",
		Short: "user related command",
	}

	ac.AddCommand(NewUserAddCommand())

	return ac
}

var (
	passwordInteractive bool
)

func NewUserAddCommand() *cobra.Command {
	cmd := cobra.Command{
		Use:   "add <user name>",
		Short: "add a new user",
		Run:   userAddCommandFunc,
	}

	cmd.Flags().BoolVar(&passwordInteractive, "interactive", true, "read password from stdin instead of interactive terminal")

	return &cmd
}

// userAddCommandFunc executes the "user add" command.
func userAddCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		ExitWithError(ExitBadArgs, fmt.Errorf("user add command requires user name as its argument."))
	}

	var password string

	if !passwordInteractive {
		fmt.Scanf("%s", &password)
	} else {
		prompt1 := fmt.Sprintf("Password of %s: ", args[0])
		password1, err1 := speakeasy.Ask(prompt1)
		if err1 != nil {
			ExitWithError(ExitBadArgs, fmt.Errorf("failed to ask password: %s.", err1))
		}

		if len(password1) == 0 {
			ExitWithError(ExitBadArgs, fmt.Errorf("empty password"))
		}

		prompt2 := fmt.Sprintf("Type password of %s again for confirmation: ", args[0])
		password2, err2 := speakeasy.Ask(prompt2)
		if err2 != nil {
			ExitWithError(ExitBadArgs, fmt.Errorf("failed to ask password: %s.", err2))
		}

		if strings.Compare(password1, password2) != 0 {
			ExitWithError(ExitBadArgs, fmt.Errorf("given passwords are different."))
		}
		password = password1
	}

	_, err := mustClientFromCmd(cmd).Auth.UserAdd(context.TODO(), args[0], password)
	if err != nil {
		ExitWithError(ExitError, err)
	}
}
