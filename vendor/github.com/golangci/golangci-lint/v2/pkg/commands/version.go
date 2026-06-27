package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime/debug"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

type BuildInfo struct {
	GoVersion string           `json:"goVersion"`
	Version   string           `json:"version"`
	Commit    string           `json:"commit"`
	Date      string           `json:"date"`
	BuildInfo *debug.BuildInfo `json:"buildInfo,omitempty"`
}

func (b BuildInfo) String() string {
	return fmt.Sprintf("golangci-lint has version %s built with %s from %s on %s",
		b.Version, b.GoVersion, b.Commit, b.Date)
}

type versionOptions struct {
	Debug bool
	JSON  bool
	Short bool
}

type versionCommand struct {
	cmd  *cobra.Command
	opts versionOptions

	info BuildInfo
}

func newVersionCommand(info BuildInfo) *versionCommand {
	c := &versionCommand{info: info}

	versionCmd := &cobra.Command{
		Use:               "version",
		Short:             "Display the golangci-lint version.",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE:              c.execute,
	}

	fs := versionCmd.Flags()
	fs.SortFlags = false // sort them as they are defined here

	fs.BoolVar(&c.opts.Debug, "debug", false, color.GreenString("Add build information"))
	fs.BoolVar(&c.opts.JSON, "json", false, color.GreenString("Display as JSON"))
	fs.BoolVar(&c.opts.Short, "short", false, color.GreenString("Display only the version number"))

	c.cmd = versionCmd

	return c
}

func (c *versionCommand) execute(_ *cobra.Command, _ []string) error {
	var info *debug.BuildInfo
	if c.opts.Debug {
		info, _ = debug.ReadBuildInfo()
	}

	switch {
	case c.opts.JSON:
		c.info.BuildInfo = info

		return json.NewEncoder(os.Stdout).Encode(c.info)
	case c.opts.Short:
		fmt.Println(c.info.Version)

		return nil

	default:
		if info != nil {
			fmt.Println(info.String())
		}

		return printVersion(os.Stdout, c.info)
	}
}

func printVersion(w io.Writer, info BuildInfo) error {
	_, err := fmt.Fprintln(w, info.String())
	return err
}
