// Copyright 2021 The etcd Authors
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
	"go.etcd.io/etcd/pkg/v3/cobrautl"
	"strconv"
	"strings"
)

// NewNamespaceQuotaCommand returns the cobra command for "namespacequota".
func NewNamespaceQuotaCommand() *cobra.Command {
	nqc := &cobra.Command{
		Use:   "namespacequota <subcommand>",
		Short: "Namespace Quota related commands",
	}

	nqc.AddCommand(SetNamespaceQuotaCommand())
	nqc.AddCommand(DeleteNamespaceQuotaCommand())
	nqc.AddCommand(GetNamespaceQuotaCommand())
	nqc.AddCommand(ListNamespaceQuotaCommand())
	return nqc
}

// SetNamespaceQuotaCommand returns the cobra command for "lease grant".
func SetNamespaceQuotaCommand() *cobra.Command {
	nqs := &cobra.Command{
		Use:   "set <key> <b:bytequota,k:keyquota>",
		Short: "Sets Namespace Quota",

		Run: namespaceQuotaSetCommandFunc,
	}

	return nqs
}

// namespaceQuotaSetCommandFunc executes the "set <b:bytequota,k:keyquota>" command.
func namespaceQuotaSetCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) != 2 {
		cobrautl.ExitWithError(cobrautl.ExitBadArgs, fmt.Errorf("namespace quota command needs a key and atleast a byte quota or key quota value"))
	}

	key := strings.TrimSpace(args[0])
	values := strings.Split(strings.TrimSpace(args[1]), ",")

	if len(key) == 0 || len(args[1]) == 0 {
		cobrautl.ExitWithError(cobrautl.ExitBadArgs, fmt.Errorf("namespace quota command needs a key AND, a byte quota OR key quota value"))
	}

	var byteQuota, keyQuota uint64
	var err error
	for _, value := range values {
		currQuota := strings.Split(value, "=")
		if currQuota[0] == "b" {
			byteQuota, err = strconv.ParseUint(currQuota[1], 10, 64)
			if err != nil {
				cobrautl.ExitWithError(cobrautl.ExitBadArgs, fmt.Errorf("bad byteq quota count (%v)", err))
			}
		} else if currQuota[0] == "k" {
			keyQuota, err = strconv.ParseUint(currQuota[1], 10, 64)
			if err != nil {
				cobrautl.ExitWithError(cobrautl.ExitBadArgs, fmt.Errorf("bad key quota count (%v)", err))
			}
		} else {
			cobrautl.ExitWithError(cobrautl.ExitBadArgs, fmt.Errorf("unrecognizable quota type (%v)", currQuota[0]))
		}
	}

	ctx, cancel := commandCtx(cmd)
	resp, err := mustClientFromCmd(cmd).SetNamespaceQuota(ctx, []byte(key), byteQuota, keyQuota)
	cancel()
	if err != nil {
		cobrautl.ExitWithError(cobrautl.ExitError, fmt.Errorf("failed to set namespace quota (%v)", err))
	}
	display.NamespaceQuotaSet(*resp)
}

// DeleteNamespaceQuotaCommand returns the cobra command for "namespacequota delete".
func DeleteNamespaceQuotaCommand() *cobra.Command {
	nqs := &cobra.Command{
		Use:   "delete <key>",
		Short: "Delete Namespace Quota",

		Run: namespaceQuotaDeleteCommandFunc,
	}

	return nqs
}

// namespaceQuotaDeleteCommandFunc executes the "delete <key>" command.
func namespaceQuotaDeleteCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		cobrautl.ExitWithError(cobrautl.ExitBadArgs, fmt.Errorf("namespace quota delete command needs one non empty key"))
	}

	key := strings.TrimSpace(args[0])

	if len(key) == 0 {
		cobrautl.ExitWithError(cobrautl.ExitBadArgs, fmt.Errorf("namespace quota delete command needs one non empty key"))
	}

	ctx, cancel := commandCtx(cmd)
	resp, err := mustClientFromCmd(cmd).DeleteNamespaceQuota(ctx, []byte(key))
	cancel()
	if err != nil {
		cobrautl.ExitWithError(cobrautl.ExitError, fmt.Errorf("failed to delete namespace quota (%v)", err))
	}
	display.NamespaceQuotaDelete(*resp)
}

// GetNamespaceQuotaCommand returns the cobra command for "namespacequota get".
func GetNamespaceQuotaCommand() *cobra.Command {
	nqs := &cobra.Command{
		Use:   "get <key>",
		Short: "Get Namespace Quota",

		Run: namespaceQuotaGetCommandFunc,
	}

	return nqs
}

// namespaceQuotaGetCommandFunc executes the "get <key>" command.
func namespaceQuotaGetCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		cobrautl.ExitWithError(cobrautl.ExitBadArgs, fmt.Errorf("namespace quota get command needs one non empty key"))
	}

	key := strings.TrimSpace(args[0])

	if len(key) == 0 {
		cobrautl.ExitWithError(cobrautl.ExitBadArgs, fmt.Errorf("namespace quota get command needs one non empty key"))
	}

	ctx, cancel := commandCtx(cmd)
	resp, err := mustClientFromCmd(cmd).GetNamespaceQuota(ctx, []byte(key))
	cancel()
	if err != nil {
		cobrautl.ExitWithError(cobrautl.ExitError, fmt.Errorf("failed to get namespace quota (%v)", err))
	}
	display.NamespaceQuotaGet(*resp)
}

// ListNamespaceQuotaCommand returns the cobra command for "namespacequota list".
func ListNamespaceQuotaCommand() *cobra.Command {
	nqs := &cobra.Command{
		Use:   "list",
		Short: "List Namespace Quota",

		Run: namespaceQuotaListCommandFunc,
	}

	return nqs
}

// namespaceQuotaListCommandFunc executes the "list" command.
func namespaceQuotaListCommandFunc(cmd *cobra.Command, args []string) {
	ctx, cancel := commandCtx(cmd)
	resp, err := mustClientFromCmd(cmd).ListNamespaceQuota(ctx)
	cancel()
	if err != nil {
		cobrautl.ExitWithError(cobrautl.ExitError, fmt.Errorf("failed to list namespace quota (%v)", err))
	}
	display.NamespaceQuotaList(*resp)
}
