// Copyright 2015 The etcd Authors
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

// copied from https://github.com/rkt/rkt/blob/master/rkt/help.go

package cobrautl

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"text/template"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	commandUsageTemplate *template.Template
	templFuncs           = template.FuncMap{
		"descToLines": func(s string) []string {
			// trim leading/trailing whitespace and split into slice of lines
			return strings.Split(strings.Trim(s, "\n\t "), "\n")
		},
		"cmdName": func(cmd *cobra.Command, startCmd *cobra.Command) string {
			parts := []string{cmd.Name()}
			for cmd.HasParent() && cmd.Parent().Name() != startCmd.Name() {
				cmd = cmd.Parent()
				parts = append([]string{cmd.Name()}, parts...)
			}
			return strings.Join(parts, " ")
		},
		"indent": func(s string) string {
			pad := strings.Repeat(" ", 2)
			return pad + strings.Replace(s, "\n", "\n"+pad, -1)
		},
	}
)

func init() {
	commandUsage := `
{{ $cmd := .Cmd }}\
{{ $cmdname := cmdName .Cmd .Cmd.Root }}\
NAME:
{{if not .Cmd.HasParent}}\
{{printf "%s - %s" .Cmd.Name .Cmd.Short | indent}}
{{else}}\
{{printf "%s - %s" $cmdname .Cmd.Short | indent}}
{{end}}\

USAGE:
{{printf "%s" .Cmd.UseLine | indent}}
{{ if not .Cmd.HasParent }}\

VERSION:
{{printf "%s" .Version | indent}}
{{end}}\
{{if .Cmd.HasSubCommands}}\

API VERSION:
{{.APIVersion | indent}}
{{end}}\
{{if .Cmd.HasExample}}\

Examples:
{{.Cmd.Example}}
{{end}}\
{{if .Cmd.HasSubCommands}}\

COMMANDS:
{{range .SubCommands}}\
{{ $cmdname := cmdName . $cmd }}\
{{ if .Runnable }}\
{{printf "%s\t%s" $cmdname .Short | indent}}
{{end}}\
{{end}}\
{{end}}\
{{ if .Cmd.Long }}\

DESCRIPTION:
{{range $line := descToLines .Cmd.Long}}{{printf "%s" $line | indent}}
{{end}}\
{{end}}\
{{if .Cmd.HasLocalFlags}}\

OPTIONS:
{{.LocalFlags}}\
{{end}}\
{{if .Cmd.HasInheritedFlags}}\

GLOBAL OPTIONS:
{{.GlobalFlags}}\
{{end}}
`[1:]

	commandUsageTemplate = template.Must(template.New("command_usage").Funcs(templFuncs).Parse(strings.ReplaceAll(commandUsage, "\\\n", "")))
}

func etcdFlagUsages(flagSet *pflag.FlagSet) string {
	x := new(strings.Builder)

	flagSet.VisitAll(func(flag *pflag.Flag) {
		if len(flag.Deprecated) > 0 {
			return
		}
		var format string
		if len(flag.Shorthand) > 0 {
			format = "  -%s, --%s"
		} else {
			format = "   %s   --%s"
		}
		if len(flag.NoOptDefVal) > 0 {
			format = format + "["
		}
		if flag.Value.Type() == "string" {
			// put quotes on the value
			format = format + "=%q"
		} else {
			format = format + "=%s"
		}
		if len(flag.NoOptDefVal) > 0 {
			format = format + "]"
		}
		format = format + "\t%s\n"
		shorthand := flag.Shorthand
		fmt.Fprintf(x, format, shorthand, flag.Name, flag.DefValue, flag.Usage)
	})

	return x.String()
}

func getSubCommands(cmd *cobra.Command) []*cobra.Command {
	var subCommands []*cobra.Command
	for _, subCmd := range cmd.Commands() {
		subCommands = append(subCommands, subCmd)
		subCommands = append(subCommands, getSubCommands(subCmd)...)
	}
	return subCommands
}

func UsageFunc(cmd *cobra.Command, version, APIVersion string) error {
	subCommands := getSubCommands(cmd)
	tabOut := getTabOutWithWriter(os.Stdout)
	commandUsageTemplate.Execute(tabOut, struct {
		Cmd         *cobra.Command
		LocalFlags  string
		GlobalFlags string
		SubCommands []*cobra.Command
		Version     string
		APIVersion  string
	}{
		cmd,
		etcdFlagUsages(cmd.LocalFlags()),
		etcdFlagUsages(cmd.InheritedFlags()),
		subCommands,
		version,
		APIVersion,
	})
	tabOut.Flush()
	return nil
}

func getTabOutWithWriter(writer io.Writer) *tabwriter.Writer {
	aTabOut := new(tabwriter.Writer)
	aTabOut.Init(writer, 0, 8, 1, '\t', 0)
	return aTabOut
}
