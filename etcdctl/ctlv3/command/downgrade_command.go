package command

import (
	"errors"

	"github.com/spf13/cobra"
	"go.etcd.io/etcd/clientv3"
)

func NewDowngradeCommand() *cobra.Command {
	dc := &cobra.Command{
		Use:   "downgrade <subcommand>",
		Short: "Downgrade related commands",
	}

	dc.AddCommand(NewDowngradeValidateCommand())
	dc.AddCommand(NewDowngradeStartCommand())
	dc.AddCommand(NewDowngradeCancelCommand())

	return dc
}

// NewDowngradeValidateCommand returns the cobra command for "downgrade validate"
func NewDowngradeValidateCommand() *cobra.Command {
	cc := &cobra.Command{
		Use:   "validate <targetVersion>",
		Short: "Validate the downgrade capability against target version",
		Run:   downgradeValidateCommandFunc,
	}
	return cc
}

// NewDowngradeStartCommand returns the cobra command for "downgrade start"
func NewDowngradeStartCommand() *cobra.Command {
	cc := &cobra.Command{
		Use:   "enable <targetVersion>",
		Short: "Enable the cluster to downgrade to target version",
		Run:   downgradeEnableCommandFunc,
	}
	return cc
}

// NewDowngradeCancelCommand returns the cobra command for "downgrade cancel"
func NewDowngradeCancelCommand() *cobra.Command {
	cc := &cobra.Command{
		Use:   "cancel",
		Short: "Cancel the current downgrade job",
		Run:   downgradeCancelCommandFunc,
	}
	return cc
}

// downgradeValidateCommandFunc executes the "downgrade validate" command
func downgradeValidateCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		ExitWithError(ExitBadArgs, errors.New("target version not provided"))
	}

	if len(args) > 1 {
		ExitWithError(ExitBadArgs, errors.New("too many arguments"))
	}

	version := args[0]
	ctx, cancel := commandCtx(cmd)
	cli := mustClientFromCmd(cmd)
	var (
		resp *clientv3.DowngradeResponse
		err  error
	)

	resp, err = cli.DowngradeValidate(ctx, version)

	cancel()
	if err != nil {
		ExitWithError(ExitError, err)
	}
	display.DowngradeValidate(*resp)
}

// downgradeEnableCommandFunc executes the "downgrade start" command
func downgradeEnableCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		ExitWithError(ExitBadArgs, errors.New("target version not provided"))
	}

	if len(args) > 1 {
		ExitWithError(ExitBadArgs, errors.New("too many arguments"))
	}

	version := args[0]
	ctx, cancel := commandCtx(cmd)
	cli := mustClientFromCmd(cmd)
	var (
		resp *clientv3.DowngradeResponse
		err  error
	)

	resp, err = cli.DowngradeEnable(ctx, version)

	cancel()
	if err != nil {
		ExitWithError(ExitError, err)
	}
	display.DowngradeEnable(*resp)
}

// downgradeCancelCommandFunc executes the "downgrade cancel" command
func downgradeCancelCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) > 0 {
		ExitWithError(ExitBadArgs, errors.New("too many arguments"))
	}

	ctx, cancel := commandCtx(cmd)
	cli := mustClientFromCmd(cmd)
	var (
		resp *clientv3.DowngradeResponse
		err  error
	)

	resp, err = cli.DowngradeCancel(ctx)

	cancel()
	if err != nil {
		ExitWithError(ExitError, err)
	}
	display.DowngradeCancel(*resp)
}
