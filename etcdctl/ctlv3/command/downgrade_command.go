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
	"errors"

	"github.com/spf13/cobra"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/pkg/v3/cobrautl"
)

// NewDowngradeCommand returns the cobra command for "downgrade".
func NewDowngradeCommand() *cobra.Command {
	dc := &cobra.Command{
		Use:   "downgrade <TARGET_VERSION>",
		Short: "Downgrade related commands",
	}

	dc.AddCommand(NewDowngradeValidateCommand())
	dc.AddCommand(NewDowngradeEnableCommand())
	dc.AddCommand(NewDowngradeCancelCommand())

	return dc
}

// NewDowngradeValidateCommand returns the cobra command for "downgrade validate".
func NewDowngradeValidateCommand() *cobra.Command {
	cc := &cobra.Command{
		Use:   "validate <TARGET_VERSION>",
		Short: "Validate downgrade capability before starting downgrade",

		Run: downgradeValidateCommandFunc,
	}
	return cc
}

// NewDowngradeEnableCommand returns the cobra command for "downgrade enable".
func NewDowngradeEnableCommand() *cobra.Command {
	cc := &cobra.Command{
		Use:   "enable <TARGET_VERSION>",
		Short: "Start a downgrade action to cluster",

		Run: downgradeEnableCommandFunc,
	}
	return cc
}

// NewDowngradeCancelCommand returns the cobra command for "downgrade cancel".
func NewDowngradeCancelCommand() *cobra.Command {
	cc := &cobra.Command{
		Use:   "cancel",
		Short: "Cancel the ongoing downgrade action to cluster",

		Run: downgradeCancelCommandFunc,
	}
	return cc
}

// downgradeValidateCommandFunc executes the "downgrade validate" command.
func downgradeValidateCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		cobrautl.ExitWithError(cobrautl.ExitBadArgs, errors.New("TARGET_VERSION not provided"))
	}
	if len(args) > 1 {
		cobrautl.ExitWithError(cobrautl.ExitBadArgs, errors.New("too many arguments"))
	}
	targetVersion := args[0]

	if len(targetVersion) == 0 {
		cobrautl.ExitWithError(cobrautl.ExitBadArgs, errors.New("target version not provided"))
	}

	ctx, cancel := commandCtx(cmd)
	cli := mustClientFromCmd(cmd)

	resp, err := cli.Downgrade(ctx, clientv3.DowngradeValidate, targetVersion)
	cancel()
	if err != nil {
		cobrautl.ExitWithError(cobrautl.ExitError, err)
	}

	display.DowngradeValidate(*resp)
}

// downgradeEnableCommandFunc executes the "downgrade enable" command.
func downgradeEnableCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		cobrautl.ExitWithError(cobrautl.ExitBadArgs, errors.New("TARGET_VERSION not provided"))
	}
	if len(args) > 1 {
		cobrautl.ExitWithError(cobrautl.ExitBadArgs, errors.New("too many arguments"))
	}
	targetVersion := args[0]

	if len(targetVersion) == 0 {
		cobrautl.ExitWithError(cobrautl.ExitBadArgs, errors.New("target version not provided"))
	}

	ctx, cancel := commandCtx(cmd)
	cli := mustClientFromCmd(cmd)

	resp, err := cli.Downgrade(ctx, clientv3.DowngradeEnable, targetVersion)
	cancel()
	if err != nil {
		cobrautl.ExitWithError(cobrautl.ExitError, err)
	}

	display.DowngradeEnable(*resp)
}

// downgradeCancelCommandFunc executes the "downgrade cancel" command.
func downgradeCancelCommandFunc(cmd *cobra.Command, args []string) {
	ctx, cancel := commandCtx(cmd)
	cli := mustClientFromCmd(cmd)

	resp, err := cli.Downgrade(ctx, clientv3.DowngradeCancel, "")
	cancel()
	if err != nil {
		cobrautl.ExitWithError(cobrautl.ExitError, err)
	}

	display.DowngradeCancel(*resp)
}
