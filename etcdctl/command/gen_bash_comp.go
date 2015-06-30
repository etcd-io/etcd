/*
Copyright 2015 Red Hat Inc All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

    Unless required by applicable law or agreed to in writing, software
    distributed under the License is distributed on an "AS IS" BASIS,
    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    See the License for the specific language governing permissions and
    limitations under the License.
*/

package command

import (
	"fmt"
	"os"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/spf13/cobra"
)

const (
	BashCompletionFunction = `__etcdctl_get_path()
{
    compopt +o nospace
    local etcdctl_out
    if etcdctl_out=$(etcdctl ls "${dir}" 2>/dev/null); then
        COMPREPLY=( $( compgen -W "${etcdctl_out[*]}" -- "${cur}" ) )
        echo "dir=${dir} comp=${COMPREPLY[@]}" >> /tmp/debug
        if [[ ${#COMPREPLY[@]} -gt 0 ]]; then
            compopt -o nospace
            return 0
        fi
    fi
    return 1
}

__etcdctl_complete_path()
{
    if [[ ${#nouns[@]} -gt 0 ]]; then
        return 1
    fi
    local dir
    dir="${cur}"
    echo -n "one: " >> /tmp/debug
    __etcdctl_get_path && return
    echo -n "two: " >> /tmp/debug
    dir=$(dirname ${cur} 2>/dev/null)
    __etcdctl_get_path && return
}

__custom_func() {
    case ${last_command} in
        etcdctl_ls | etcdctl_get | etcdctl_set | etcdctl_rm | etcdctl_rmdir )
            __etcdctl_complete_path
            return
            ;;
        *)
            ;;
    esac
}
`
)

// NewGetCommand returns the CLI command for "get".
func NewGenerateBashCompletionsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "genbash",
		Short: "generate bash completions for etcdctl",
		Run:   gen,
	}
	cmd.Flags().String("outfile", "etcdctl.sh", "file to write bash completions")
	return cmd
}

func gen(cmd *cobra.Command, args []string) {
	if len(args) != 0 {
		fmt.Fprintf(os.Stderr, "etcdctl genbash does not take arguments\n")
		os.Exit(1)
	}
	outFile, _ := cmd.Flags().GetString("outfile")

	cmd.Root().GenBashCompletionFile(outFile)
}
