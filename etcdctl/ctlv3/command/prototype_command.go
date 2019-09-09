package command

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/coreos/etcd/auth/authpb"
	"github.com/spf13/cobra"
)

func NewPrototypeCommand() *cobra.Command {
	ac := &cobra.Command{
		Use:   "prototype <subcommand>",
		Short: "Prototype related commands",
	}

	ac.AddCommand(newPrototypeUpdateCommand())
	ac.AddCommand(newPrototypeDeleteCommand())
	ac.AddCommand(newPrototypeListCommand())

	return ac
}

func newPrototypeUpdateCommand() *cobra.Command {
	cmd := cobra.Command{
		Use:   "update <prototype name> <flags> [field:rightsRead:rightsWrite,...]",
		Short: "Updates a prototype",
		Run:   prototypeUpdateCommandFunc,
	}

	return &cmd
}

func newPrototypeDeleteCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <prototype name>",
		Short: "Deletes a prototype",
		Run:   prototypeDeleteCommandFunc,
	}
}

func newPrototypeListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Lists all prototypes",
		Run:   prototypeListCommandFunc,
	}
}

func prototypeUpdateCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		ExitWithError(ExitBadArgs, fmt.Errorf("prototype update command requires prototype name as its argument."))
	}
	if len(args) < 2 {
		ExitWithError(ExitBadArgs, fmt.Errorf("prototype update command requires flags as its argument."))
	}
	flags, err := strconv.ParseInt(args[1], 10, 32)
	if err != nil {
		ExitWithError(ExitBadArgs, fmt.Errorf("bad flags (%v)", err))
	}

	prototype := &authpb.Prototype{Name: []byte(args[0]), Flags: uint32(flags), Fields: []*authpb.PrototypeField{}}

	if len(args) >= 3 {
		fields := strings.SplitN(args[2], ":", -1)
		for _, field := range fields {
			parts := strings.SplitN(field, ",", 3)
			if len(parts) < 3 {
				ExitWithError(ExitBadArgs, fmt.Errorf("bad field (%v)", field))
			}
			rightsRead, err := strconv.ParseInt(parts[1], 10, 32)
			if err != nil {
				ExitWithError(ExitBadArgs, fmt.Errorf("bad rightsRead (%v)", err))
			}
			rightsWrite, err := strconv.ParseInt(parts[2], 10, 32)
			if err != nil {
				ExitWithError(ExitBadArgs, fmt.Errorf("bad rightsWrite (%v)", err))
			}
			prototype.Fields = append(prototype.Fields,
				&authpb.PrototypeField{Key: parts[0],
					RightsRead:  uint32(rightsRead),
					RightsWrite: uint32(rightsWrite)})
		}
	}

	resp, err := mustClientFromCmd(cmd).Auth.PrototypeUpdate(context.TODO(), prototype)
	if err != nil {
		ExitWithError(ExitError, err)
	}
	display.PrototypeUpdate(prototype, *resp)
}

func prototypeDeleteCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		ExitWithError(ExitBadArgs, fmt.Errorf("prototype delete command requires prototype name as its argument."))
	}

	resp, err := mustClientFromCmd(cmd).Auth.PrototypeDelete(context.TODO(), args[0])
	if err != nil {
		ExitWithError(ExitError, err)
	}
	display.PrototypeDelete(args[0], *resp)
}

func prototypeListCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) != 0 {
		ExitWithError(ExitBadArgs, fmt.Errorf("prototype list command requires no arguments."))
	}

	resp, err := mustClientFromCmd(cmd).Auth.PrototypeList(context.TODO())
	if err != nil {
		ExitWithError(ExitError, err)
	}

	display.PrototypeList(*resp)
}
