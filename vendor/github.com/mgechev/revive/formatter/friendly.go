package formatter

import (
	"bytes"
	"cmp"
	"fmt"
	"io"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/fatih/color"

	"github.com/mgechev/revive/lint"
)

// Friendly is an implementation of the [lint.Formatter] interface
// which formats the errors to a friendly, human-readable format.
type Friendly struct {
	Metadata lint.FormatterMetadata
}

// Name returns the name of the formatter.
func (*Friendly) Name() string {
	return "friendly"
}

// Format formats the failures gotten from the lint.
func (f *Friendly) Format(failures <-chan lint.Failure, config lint.Config) (string, error) {
	var buf strings.Builder
	errorMap := map[string]int{}
	warningMap := map[string]int{}
	totalErrors := 0
	totalWarnings := 0
	warningEmoji := color.YellowString("⚠")
	errorEmoji := color.RedString("✘")
	for failure := range failures {
		sev := severity(config, failure)
		firstCol := warningEmoji
		if sev == lint.SeverityError {
			firstCol = errorEmoji
		}
		if err := f.printFriendlyFailure(&buf, firstCol, failure); err != nil {
			return "", err
		}
		switch sev {
		case lint.SeverityWarning:
			warningMap[failure.RuleName]++
			totalWarnings++
		case lint.SeverityError:
			errorMap[failure.RuleName]++
			totalErrors++
		}
	}

	emoji := warningEmoji
	if totalErrors > 0 {
		emoji = errorEmoji
	}
	if err := f.printSummary(&buf, emoji, totalErrors, totalWarnings); err != nil {
		return "", err
	}
	if err := f.printStatistics(&buf, color.RedString("Errors:"), errorMap); err != nil {
		return "", err
	}
	if err := f.printStatistics(&buf, color.YellowString("Warnings:"), warningMap); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (f *Friendly) printFriendlyFailure(sb *strings.Builder, firstColumn string, failure lint.Failure) error {
	f.printHeaderRow(sb, firstColumn, failure)
	if err := f.printFilePosition(sb, failure); err != nil {
		return err
	}
	_, err := sb.WriteString("\n\n")
	return err
}

func (*Friendly) printHeaderRow(sb *strings.Builder, firstColumn string, failure lint.Failure) {
	sb.WriteString(table([][]string{{firstColumn, ruleDescriptionURL(failure.RuleName), color.GreenString(failure.Failure)}}))
}

func (*Friendly) printFilePosition(sb *strings.Builder, failure lint.Failure) error {
	_, err := fmt.Fprintf(sb, "  %s:%d:%d", failure.Filename(), failure.Position.Start.Line, failure.Position.Start.Column)
	return err
}

type statEntry struct {
	name     string
	failures int
}

func (*Friendly) printSummary(w io.Writer, firstColumn string, errors, warnings int) error {
	problemsLabel := "problems"
	if errors+warnings == 1 {
		problemsLabel = "problem"
	}
	warningsLabel := "warnings"
	if warnings == 1 {
		warningsLabel = "warning"
	}
	errorsLabel := "errors"
	if errors == 1 {
		errorsLabel = "error"
	}
	str := fmt.Sprintf("%d %s (%d %s, %d %s)", errors+warnings, problemsLabel, errors, errorsLabel, warnings, warningsLabel)
	if errors > 0 {
		_, err := fmt.Fprintf(w, "%s %s\n\n", firstColumn, color.RedString(str))
		return err
	}
	if warnings > 0 {
		_, err := fmt.Fprintf(w, "%s %s\n\n", firstColumn, color.YellowString(str))
		return err
	}
	return nil
}

func (*Friendly) printStatistics(w io.Writer, header string, stats map[string]int) error {
	if len(stats) == 0 {
		return nil
	}
	data := make([]statEntry, 0, len(stats))
	for name, total := range stats {
		data = append(data, statEntry{name, total})
	}
	slices.SortFunc(data, func(a, b statEntry) int {
		return -cmp.Compare(a.failures, b.failures)
	})
	formatted := [][]string{}
	for _, entry := range data {
		formatted = append(formatted, []string{color.GreenString(fmt.Sprintf("%d", entry.failures)), entry.name})
	}
	if _, err := fmt.Fprintln(w, header); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, table(formatted)); err != nil {
		return err
	}
	return nil
}

func table(rows [][]string) string {
	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	for _, row := range rows {
		_, _ = tw.Write([]byte{'\t'})
		for i, col := range row {
			_, _ = tw.Write([]byte(col))
			if i < len(row)-1 {
				_, _ = tw.Write([]byte{'\t'})
			}
		}
		_, _ = tw.Write([]byte{'\n'})
	}
	_ = tw.Flush()
	return buf.String()
}
