package lintcmd

// Notes on GitHub-specific restrictions:
//
// Result.Message needs to either have ID or Text set. Markdown
// gets ignored. Text isn't treated verbatim however: Markdown
// formatting gets stripped, except for links.
//
// GitHub does not display RelatedLocations. The only way to make
// use of them is to link to them (via their ID) in the
// Result.Message. And even then, it will only show the referred
// line of code, not the message. We can duplicate the messages in
// the Result.Message, but we can't even indent them, because
// leading whitespace gets stripped.
//
// GitHub does use the Markdown version of rule help, but it
// renders it the way it renders comments on issues – that is, it
// turns line breaks into hard line breaks, even though it
// shouldn't.
//
// GitHub doesn't make use of the tool's URI or version, nor of
// the help URIs of rules.
//
// There does not seem to be a way of using SARIF for "normal" CI,
// without results showing up as code scanning alerts. Also, a
// SARIF file containing only warnings, no errors, will not fail
// CI by default, but this is configurable.
// GitHub does display some parts of SARIF results in PRs, but
// most of the useful parts of SARIF, such as help text of rules,
// is only accessible via the code scanning alerts, which are only
// accessible by users with write permissions.
//
// Result.Suppressions is being ignored.
//
//
// Notes on other tools
//
// VS Code Sarif viewer
//
// The Sarif viewer in VS Code displays the full message in the
// tabular view, removing newlines. That makes our multi-line
// messages (which we use as a workaround for missing related
// information) very ugly.
//
// Much like GitHub, the Sarif viewer does not make related
// information visible unless we explicitly refer to it in the
// message.
//
// Suggested fixes are not exposed in any way.
//
// It only shows the shortDescription or fullDescription of a
// rule, not its help. We can't put the help in fullDescription,
// because the fullDescription isn't meant to be that long. For
// example, GitHub displays it in a single line, under the
// shortDescription.
//
// VS Code can filter based on Result.Suppressions, but it doesn't
// display our suppression message. Also, by default, suppressed
// results get shown, and the column indicating that a result is
// suppressed is hidden, which makes for a confusing experience.
//
// When a rule has only an ID, no name, VS Code displays a
// prominent dash in place of the name. When the name and ID are
// identical, it prints both. However, we can't make them
// identical, as SARIF requires that either the ID and name are
// different, or that the name is omitted.

// FIXME(dh): we're currently reporting column information using UTF-8
// byte offsets, not using Unicode code points or UTF-16, which are
// the only two ways allowed by SARIF.

// TODO(dh) set properties.tags – we can use different tags for the
// staticcheck, simple, stylecheck and unused checks, so users can
// filter their results

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/sarif"
)

type sarifFormatter struct {
	driverName    string
	driverVersion string
	driverWebsite string
}

func sarifLevel(severity lint.Severity) string {
	switch severity {
	case lint.SeverityNone:
		// no configured severity, default to warning
		return "warning"
	case lint.SeverityError:
		return "error"
	case lint.SeverityDeprecated:
		return "warning"
	case lint.SeverityWarning:
		return "warning"
	case lint.SeverityInfo:
		return "note"
	case lint.SeverityHint:
		return "note"
	default:
		// unreachable
		return "none"
	}
}

func encodePath(path string) string {
	return (&url.URL{Path: path}).EscapedPath()
}

func sarifURI(path string) string {
	u := url.URL{
		Scheme: "file",
		Path:   path,
	}
	return u.String()
}

func sarifArtifactLocation(name string) sarif.ArtifactLocation {
	// Ideally we use relative paths so that GitHub can resolve them
	name = shortPath(name)
	if filepath.IsAbs(name) {
		return sarif.ArtifactLocation{
			URI: sarifURI(name),
		}
	} else {
		return sarif.ArtifactLocation{
			URI:       encodePath(name),
			URIBaseID: "%SRCROOT%", // This is specific to GitHub,
		}
	}
}

func sarifFormatText(s string) string {
	// GitHub doesn't ignore line breaks, even though it should, so we remove them.

	var out strings.Builder
	lines := strings.Split(s, "\n")
	for i, line := range lines[:len(lines)-1] {
		out.WriteString(line)
		if line == "" {
			out.WriteString("\n")
		} else {
			nextLine := lines[i+1]
			if nextLine == "" || strings.HasPrefix(line, "> ") || strings.HasPrefix(line, "    ") {
				out.WriteString("\n")
			} else {
				out.WriteString(" ")
			}
		}
	}
	out.WriteString(lines[len(lines)-1])
	return convertCodeBlocks(out.String())
}

func moreCodeFollows(lines []string) bool {
	for _, line := range lines {
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "    ") {
			return true
		} else {
			return false
		}
	}
	return false
}

var alpha = regexp.MustCompile(`^[a-zA-Z ]+$`)

func convertCodeBlocks(text string) string {
	var buf strings.Builder
	lines := strings.Split(text, "\n")

	inCode := false
	empties := 0
	for i, line := range lines {
		if inCode {
			if !moreCodeFollows(lines[i:]) {
				if inCode {
					fmt.Fprintln(&buf, "```")
					inCode = false
				}
			}
		}

		prevEmpties := empties
		if line == "" && !inCode {
			empties++
		} else {
			empties = 0
		}

		if line == "" {
			fmt.Fprintln(&buf)
			continue
		}

		if strings.HasPrefix(line, "    ") {
			line = line[4:]
			if !inCode {
				fmt.Fprintln(&buf, "```go")
				inCode = true
			}
		}

		onlyAlpha := alpha.MatchString(line)
		out := line
		if !inCode && prevEmpties >= 2 && onlyAlpha {
			fmt.Fprintf(&buf, "## %s\n", out)
		} else {
			fmt.Fprint(&buf, out)
			fmt.Fprintln(&buf)
		}
	}
	if inCode {
		fmt.Fprintln(&buf, "```")
	}

	return buf.String()
}

func (o *sarifFormatter) Format(checks []*lint.Analyzer, diagnostics []diagnostic) {
	// TODO(dh): some diagnostics shouldn't be reported as results. For example, when the user specifies a package on the command line that doesn't exist.

	cwd, _ := os.Getwd()
	run := sarif.Run{
		Tool: sarif.Tool{
			Driver: sarif.ToolComponent{
				Name:           o.driverName,
				Version:        o.driverVersion,
				InformationURI: o.driverWebsite,
			},
		},
		Invocations: []sarif.Invocation{{
			Arguments: os.Args[1:],
			WorkingDirectory: sarif.ArtifactLocation{
				URI: sarifURI(cwd),
			},
			ExecutionSuccessful: true,
		}},
	}
	for _, c := range checks {
		doc := c.Doc.Compile()
		run.Tool.Driver.Rules = append(run.Tool.Driver.Rules,
			sarif.ReportingDescriptor{
				// We don't set Name, as Name and ID mustn't be identical.
				ID: c.Analyzer.Name,
				ShortDescription: sarif.Message{
					Text:     doc.Title,
					Markdown: doc.TitleMarkdown,
				},
				HelpURI: "https://staticcheck.dev/docs/checks#" + c.Analyzer.Name,
				// We use our markdown as the plain text version, too. We
				// use very little markdown, primarily quotations,
				// indented code blocks and backticks. All of these are
				// fine as plain text, too.
				Help: sarif.Message{
					Text:     sarifFormatText(doc.Format(false)),
					Markdown: sarifFormatText(doc.FormatMarkdown(false)),
				},
				DefaultConfiguration: sarif.ReportingConfiguration{
					// TODO(dh): we could figure out which checks were disabled globally
					Enabled: true,
					Level:   sarifLevel(doc.Severity),
				},
			})
	}

	for _, p := range diagnostics {
		r := sarif.Result{
			RuleID: p.Category,
			Kind:   sarif.Fail,
			Message: sarif.Message{
				Text: p.Message,
			},
		}
		r.Locations = []sarif.Location{{
			PhysicalLocation: sarif.PhysicalLocation{
				ArtifactLocation: sarifArtifactLocation(p.Position.Filename),
				Region: sarif.Region{
					StartLine:   p.Position.Line,
					StartColumn: p.Position.Column,
					EndLine:     p.End.Line,
					EndColumn:   p.End.Column,
				},
			},
		}}
		for _, fix := range p.SuggestedFixes {
			sfix := sarif.Fix{
				Description: sarif.Message{
					Text: fix.Message,
				},
			}
			// file name -> replacements
			changes := map[string][]sarif.Replacement{}
			for _, edit := range fix.TextEdits {
				changes[edit.Position.Filename] = append(changes[edit.Position.Filename], sarif.Replacement{
					DeletedRegion: sarif.Region{
						StartLine:   edit.Position.Line,
						StartColumn: edit.Position.Column,
						EndLine:     edit.End.Line,
						EndColumn:   edit.End.Column,
					},
					InsertedContent: sarif.ArtifactContent{
						Text: string(edit.NewText),
					},
				})
			}
			for path, replacements := range changes {
				sfix.ArtifactChanges = append(sfix.ArtifactChanges, sarif.ArtifactChange{
					ArtifactLocation: sarifArtifactLocation(path),
					Replacements:     replacements,
				})
			}
			r.Fixes = append(r.Fixes, sfix)
		}
		for i, related := range p.Related {
			r.Message.Text += fmt.Sprintf("\n\t[%s](%d)", related.Message, i+1)

			r.RelatedLocations = append(r.RelatedLocations,
				sarif.Location{
					ID: i + 1,
					Message: &sarif.Message{
						Text: related.Message,
					},
					PhysicalLocation: sarif.PhysicalLocation{
						ArtifactLocation: sarifArtifactLocation(related.Position.Filename),
						Region: sarif.Region{
							StartLine:   related.Position.Line,
							StartColumn: related.Position.Column,
							EndLine:     related.End.Line,
							EndColumn:   related.End.Column,
						},
					},
				})
		}

		if p.Severity == severityIgnored {
			// Note that GitHub does not support suppressions, which is why Staticcheck still requires the -show-ignored flag to be set for us to emit ignored diagnostics.

			r.Suppressions = []sarif.Suppression{{
				Kind: "inSource",
				// TODO(dh): populate the Justification field
			}}
		} else {
			// We want an empty slice, not nil. SARIF differentiates
			// between the two. An empty slice means that the diagnostic
			// wasn't suppressed, while nil means that we don't have the
			// information available.
			r.Suppressions = []sarif.Suppression{}
		}
		run.Results = append(run.Results, r)
	}

	json.NewEncoder(os.Stdout).Encode(sarif.Log{
		Version: sarif.Version,
		Schema:  sarif.Schema,
		Runs:    []sarif.Run{run},
	})
}
