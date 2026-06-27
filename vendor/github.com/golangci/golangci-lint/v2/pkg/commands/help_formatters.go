package commands

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goformatters"
	"github.com/golangci/golangci-lint/v2/pkg/lint/linter"
	"github.com/golangci/golangci-lint/v2/pkg/lint/lintersdb"
	"github.com/golangci/golangci-lint/v2/pkg/logutils"
)

type formatterHelp struct {
	Name        string `json:"name"`
	Desc        string `json:"description"`
	Deprecated  bool   `json:"deprecated"`
	Since       string `json:"since"`
	OriginalURL string `json:"originalURL,omitempty"`
}

func newFormatterHelp(lc *linter.Config) formatterHelp {
	return formatterHelp{
		Name:        lc.Name(),
		Desc:        formatDescription(lc.Linter.Desc()),
		Deprecated:  lc.IsDeprecated(),
		Since:       lc.Since,
		OriginalURL: lc.OriginalURL,
	}
}

func (c *helpCommand) formattersPreRunE(_ *cobra.Command, _ []string) error {
	// The command doesn't depend on the real configuration.
	dbManager, err := lintersdb.NewManager(c.log.Child(logutils.DebugKeyLintersDB), config.NewDefault(), lintersdb.NewLinterBuilder())
	if err != nil {
		return err
	}

	c.dbManager = dbManager

	return nil
}

func (c *helpCommand) formattersExecute(_ *cobra.Command, _ []string) error {
	if c.opts.JSON {
		return c.formattersPrintJSON()
	}

	c.formattersPrint()

	return nil
}

func (c *helpCommand) formattersPrintJSON() error {
	var formatters []formatterHelp

	for _, lc := range c.dbManager.GetAllSupportedLinterConfigs() {
		if lc.Internal {
			continue
		}

		if !goformatters.IsFormatter(lc.Name()) {
			continue
		}

		formatters = append(formatters, newFormatterHelp(lc))
	}

	return json.NewEncoder(c.cmd.OutOrStdout()).Encode(formatters)
}

func (c *helpCommand) formattersPrint() {
	var lcs []*linter.Config
	for _, lc := range c.dbManager.GetAllSupportedLinterConfigs() {
		if lc.Internal {
			continue
		}

		if !goformatters.IsFormatter(lc.Name()) {
			continue
		}

		lcs = append(lcs, lc)
	}

	color.Green("Disabled by default formatters:\n")
	printFormatters(lcs)
}

func printFormatters(lcs []*linter.Config) {
	slices.SortFunc(lcs, func(a, b *linter.Config) int {
		if a.IsDeprecated() && b.IsDeprecated() {
			return strings.Compare(a.Name(), b.Name())
		}

		if a.IsDeprecated() {
			return 1
		}

		if b.IsDeprecated() {
			return -1
		}

		return strings.Compare(a.Name(), b.Name())
	})

	for _, lc := range lcs {
		desc := formatDescription(lc.Linter.Desc())

		deprecatedMark := ""
		if lc.IsDeprecated() {
			deprecatedMark = " [" + color.RedString("deprecated") + "]"
		}

		_, _ = fmt.Fprintf(logutils.StdOut, "%s%s: %s\n",
			color.YellowString(lc.Name()), deprecatedMark, desc)
	}
}
