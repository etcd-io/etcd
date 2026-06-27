// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package scan

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"golang.org/x/vuln/internal"
	"golang.org/x/vuln/internal/govulncheck"
	"golang.org/x/vuln/internal/osv"
	"golang.org/x/vuln/internal/vulncheck"
)

type style int

const (
	defaultStyle = style(iota)
	osvCalledStyle
	osvImportedStyle
	detailsStyle
	sectionStyle
	keyStyle
	valueStyle
)

// NewtextHandler returns a handler that writes govulncheck output as text.
func NewTextHandler(w io.Writer) *TextHandler {
	return &TextHandler{w: w}
}

type TextHandler struct {
	w         io.Writer
	sbom      *govulncheck.SBOM
	osvs      []*osv.Entry
	findings  []*findingSummary
	scanLevel govulncheck.ScanLevel
	scanMode  govulncheck.ScanMode

	err error

	showColor   bool
	showTraces  bool
	showVersion bool
	showVerbose bool
}

const (
	detailsMessage = `For details, see https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck.`

	binaryProgressMessage = `Scanning your binary for known vulnerabilities...`

	noVulnsMessage = `No vulnerabilities found.`

	noOtherVulnsMessage = `No other vulnerabilities found.`

	verboseMessage = `'-show verbose' for more details`

	symbolMessage = `'-scan symbol' for more fine grained vulnerability detection`
)

func (h *TextHandler) Flush() error {
	if h.showVerbose {
		h.printSBOM()
	}
	if len(h.findings) == 0 {
		h.print(noVulnsMessage + "\n")
	} else {
		fixupFindings(h.osvs, h.findings)
		counters := h.allVulns(h.findings)
		h.summary(counters)
	}
	if h.err != nil {
		return h.err
	}
	// We found vulnerabilities when the findings' level matches the scan level.
	if (isCalled(h.findings) && h.scanLevel == govulncheck.ScanLevelSymbol) ||
		(isImported(h.findings) && h.scanLevel == govulncheck.ScanLevelPackage) ||
		(isRequired(h.findings) && h.scanLevel == govulncheck.ScanLevelModule) {
		return errVulnerabilitiesFound
	}

	return nil
}

// Config writes version information only if --version was set.
func (h *TextHandler) Config(config *govulncheck.Config) error {
	h.scanLevel = config.ScanLevel
	h.scanMode = config.ScanMode

	if !h.showVersion {
		return nil
	}
	if config.GoVersion != "" {
		h.style(keyStyle, "Go: ")
		h.print(config.GoVersion, "\n")
	}
	if config.ScannerName != "" {
		h.style(keyStyle, "Scanner: ")
		h.print(config.ScannerName)
		if config.ScannerVersion != "" {
			h.print(`@`, config.ScannerVersion)
		}
		h.print("\n")
	}
	if config.DB != "" {
		h.style(keyStyle, "DB: ")
		h.print(config.DB, "\n")
		if config.DBLastModified != nil {
			h.style(keyStyle, "DB updated: ")
			h.print(*config.DBLastModified, "\n")
		}
	}
	h.print("\n")
	return h.err
}

func (h *TextHandler) SBOM(sbom *govulncheck.SBOM) error {
	h.sbom = sbom
	return nil
}

func (h *TextHandler) printSBOM() error {
	if h.sbom == nil {
		h.print("No packages matched the provided pattern.\n")
		return nil
	}

	printed := false

	for i, root := range h.sbom.Roots {
		if i == 0 {
			if len(h.sbom.Roots) > 1 {
				h.print("The package pattern matched the following ", len(h.sbom.Roots), " root packages:\n")
			} else {
				h.print("The package pattern matched the following root package:\n")
			}
		}

		h.print("  ", root, "\n")
		printed = true
	}
	for i, mod := range h.sbom.Modules {
		if i == 0 && mod.Path != "stdlib" {
			h.print("Govulncheck scanned the following ", len(h.sbom.Modules)-1, " modules and the ", h.sbom.GoVersion, " standard library:\n")
		}

		if mod.Path == "stdlib" {
			continue
		}

		h.print("  ", mod.Path)
		if mod.Version != "" {
			h.print("@", mod.Version)
		}
		h.print("\n")
		printed = true
	}
	if printed {
		h.print("\n")
	}
	return nil
}

// Progress writes progress updates during govulncheck execution.
func (h *TextHandler) Progress(progress *govulncheck.Progress) error {
	if h.showVerbose {
		h.print(progress.Message, "\n\n")
	}
	return h.err
}

// OSV gathers osv entries to be written.
func (h *TextHandler) OSV(entry *osv.Entry) error {
	h.osvs = append(h.osvs, entry)
	return nil
}

// Finding gathers vulnerability findings to be written.
func (h *TextHandler) Finding(finding *govulncheck.Finding) error {
	if err := validateFindings(finding); err != nil {
		return err
	}
	h.findings = append(h.findings, newFindingSummary(finding))
	return nil
}

func (h *TextHandler) allVulns(findings []*findingSummary) summaryCounters {
	byVuln := groupByVuln(findings)
	var called, imported, required [][]*findingSummary
	mods := map[string]struct{}{}
	stdlibCalled := false
	for _, findings := range byVuln {
		switch {
		case isCalled(findings):
			called = append(called, findings)
			if isStdFindings(findings) {
				stdlibCalled = true
			} else {
				mods[findings[0].Trace[0].Module] = struct{}{}
			}
		case isImported(findings):
			imported = append(imported, findings)
		default:
			required = append(required, findings)
		}
	}

	if h.scanLevel.WantSymbols() {
		h.style(sectionStyle, "=== Symbol Results ===\n\n")
		if len(called) == 0 {
			h.print(noVulnsMessage, "\n\n")
		}
		for index, findings := range called {
			h.vulnerability(index, findings)
		}
	}

	if h.scanLevel == govulncheck.ScanLevelPackage || (h.scanLevel.WantPackages() && h.showVerbose) {
		h.style(sectionStyle, "=== Package Results ===\n\n")
		if len(imported) == 0 {
			h.print(choose(!h.scanLevel.WantSymbols(), noVulnsMessage, noOtherVulnsMessage), "\n\n")
		}
		for index, findings := range imported {
			h.vulnerability(index, findings)
		}
	}

	if h.showVerbose || h.scanLevel == govulncheck.ScanLevelModule {
		h.style(sectionStyle, "=== Module Results ===\n\n")
		if len(required) == 0 {
			h.print(choose(!h.scanLevel.WantPackages(), noVulnsMessage, noOtherVulnsMessage), "\n\n")
		}
		for index, findings := range required {
			h.vulnerability(index, findings)
		}
	}

	return summaryCounters{
		VulnerabilitiesCalled:   len(called),
		VulnerabilitiesImported: len(imported),
		VulnerabilitiesRequired: len(required),
		ModulesCalled:           len(mods),
		StdlibCalled:            stdlibCalled,
	}
}

func (h *TextHandler) vulnerability(index int, findings []*findingSummary) {
	h.style(keyStyle, "Vulnerability")
	h.print(" #", index+1, ": ")
	if isCalled(findings) {
		h.style(osvCalledStyle, findings[0].OSV.ID)
	} else {
		h.style(osvImportedStyle, findings[0].OSV.ID)
	}
	h.print("\n")
	h.style(detailsStyle)
	description := findings[0].OSV.Summary
	if description == "" {
		description = findings[0].OSV.Details
	}
	h.wrap("    ", description, 80)
	h.style(defaultStyle)
	h.print("\n")
	h.style(keyStyle, "  More info:")
	h.print(" ", findings[0].OSV.DatabaseSpecific.URL, "\n")

	byModule := groupByModule(findings)
	first := true
	for _, module := range byModule {
		// Note: there can be several findingSummaries for the same vulnerability
		// emitted during streaming for different scan levels.

		// The module is same for all finding summaries.
		lastFrame := module[0].Trace[0]
		mod := lastFrame.Module
		// For stdlib, try to show package path as module name where
		// the scan level allows it.
		// TODO: should this be done in byModule as well?
		path := lastFrame.Module
		if stdPkg := h.pkg(module); path == internal.GoStdModulePath && stdPkg != "" {
			path = stdPkg
		}
		// All findings on a module are found and fixed at the same version
		foundVersion := moduleVersionString(lastFrame.Module, lastFrame.Version)
		fixedVersion := moduleVersionString(lastFrame.Module, module[0].FixedVersion)
		if !first {
			h.print("\n")
		}
		first = false
		h.print("  ")
		if mod == internal.GoStdModulePath {
			h.print("Standard library")
		} else {
			h.style(keyStyle, "Module: ")
			h.print(mod)
		}
		h.print("\n    ")
		h.style(keyStyle, "Found in: ")
		h.print(path, "@", foundVersion, "\n    ")
		h.style(keyStyle, "Fixed in: ")
		if fixedVersion != "" {
			h.print(path, "@", fixedVersion)
		} else {
			h.print("N/A")
		}
		h.print("\n")
		platforms := platforms(mod, module[0].OSV)
		if len(platforms) > 0 {
			h.style(keyStyle, "    Platforms: ")
			for ip, p := range platforms {
				if ip > 0 {
					h.print(", ")
				}
				h.print(p)
			}
			h.print("\n")
		}
		h.traces(module)
	}
	h.print("\n")
}

// pkg gives the package information for findings summaries
// if one exists. This is only used to print package path
// instead of a module for stdlib vulnerabilities at symbol
// and package scan level.
func (h *TextHandler) pkg(summaries []*findingSummary) string {
	for _, f := range summaries {
		if pkg := f.Trace[0].Package; pkg != "" {
			return pkg
		}
	}
	return ""
}

// traces prints out the most precise trace information
// found in the given summaries.
func (h *TextHandler) traces(traces []*findingSummary) {
	// Sort the traces by the vulnerable symbol. This
	// guarantees determinism since we are currently
	// showing only one trace per symbol.
	sort.SliceStable(traces, func(i, j int) bool {
		return symbol(traces[i].Trace[0], true) < symbol(traces[j].Trace[0], true)
	})

	// compacts are finding summaries with compact traces
	// suitable for non-verbose textual output. Currently,
	// only traces produced by symbol analysis.
	var compacts []*findingSummary
	for _, t := range traces {
		if t.Compact != "" {
			compacts = append(compacts, t)
		}
	}

	// binLimit is a limit on the number of binary traces
	// to show. Traces for binaries are less interesting
	// as users cannot act on them and they can hence
	// spam users.
	const binLimit = 5
	binary := h.scanMode == govulncheck.ScanModeBinary
	for i, entry := range compacts {
		if i == 0 {
			if binary {
				h.style(keyStyle, "    Vulnerable symbols found:\n")
			} else {
				h.style(keyStyle, "    Example traces found:\n")
			}
		}

		// skip showing all symbols in binary mode unless '-show traces' is on.
		if binary && (i+1) > binLimit && !h.showTraces {
			h.print("      Use '-show traces' to see the other ", len(compacts)-binLimit, " found symbols\n")
			break
		}

		h.print("      #", i+1, ": ")

		if !h.showTraces { // show summarized traces
			h.print(entry.Compact, "\n")
			continue
		}

		if binary {
			// There are no call stacks in binary mode
			// so just show the full symbol name.
			h.print(symbol(entry.Trace[0], false), "\n")
		} else {
			h.print("for function ", symbol(entry.Trace[0], false), "\n")
			for i := len(entry.Trace) - 1; i >= 0; i-- {
				t := entry.Trace[i]
				h.print("        ")
				h.print(symbolName(t))
				if t.Position != nil {
					h.print(" @ ", symbolPath(t))
				}
				h.print("\n")
			}
		}
	}
}

// symbolPath returns a user-friendly path to a symbol.
func symbolPath(t *govulncheck.Frame) string {
	// Add module path prefix to symbol paths to be more
	// explicit to which module the symbols belong to.
	return t.Module + "/" + posToString(t.Position)
}

func (h *TextHandler) summary(c summaryCounters) {
	// print short summary of findings identified at the desired level of scan precision
	var vulnCount int
	h.print("Your code ", choose(h.scanLevel.WantSymbols(), "is", "may be"), " affected by ")
	switch h.scanLevel {
	case govulncheck.ScanLevelSymbol:
		vulnCount = c.VulnerabilitiesCalled
	case govulncheck.ScanLevelPackage:
		vulnCount = c.VulnerabilitiesImported
	case govulncheck.ScanLevelModule:
		vulnCount = c.VulnerabilitiesRequired
	}
	h.style(valueStyle, vulnCount)
	h.print(choose(vulnCount == 1, ` vulnerability`, ` vulnerabilities`))
	if h.scanLevel.WantSymbols() {
		h.print(choose(c.ModulesCalled > 0 || c.StdlibCalled, ` from `, ``))
		if c.ModulesCalled > 0 {
			h.style(valueStyle, c.ModulesCalled)
			h.print(choose(c.ModulesCalled == 1, ` module`, ` modules`))
		}
		if c.StdlibCalled {
			if c.ModulesCalled != 0 {
				h.print(` and `)
			}
			h.print(`the Go standard library`)
		}
	}
	h.print(".\n")

	// print summary for vulnerabilities found at other levels of scan precision
	if other := h.summaryOtherVulns(c); other != "" {
		h.wrap("", other, 80)
		h.print("\n")
	}

	// print suggested flags for more/better info depending on scan level and if in verbose mode
	if sugg := h.summarySuggestion(); sugg != "" {
		h.wrap("", sugg, 80)
		h.print("\n")
	}
}

func (h *TextHandler) summaryOtherVulns(c summaryCounters) string {
	var summary strings.Builder
	if c.VulnerabilitiesRequired+c.VulnerabilitiesImported == 0 {
		summary.WriteString("This scan found no other vulnerabilities in ")
		if h.scanLevel.WantSymbols() {
			summary.WriteString("packages you import or ")
		}
		summary.WriteString("modules you require.")
	} else {
		summary.WriteString(choose(h.scanLevel.WantPackages(), "This scan also found ", ""))
		if h.scanLevel.WantSymbols() {
			summary.WriteString(fmt.Sprint(c.VulnerabilitiesImported))
			summary.WriteString(choose(c.VulnerabilitiesImported == 1, ` vulnerability `, ` vulnerabilities `))
			summary.WriteString("in packages you import and ")
		}
		if h.scanLevel.WantPackages() {
			summary.WriteString(fmt.Sprint(c.VulnerabilitiesRequired))
			summary.WriteString(choose(c.VulnerabilitiesRequired == 1, ` vulnerability `, ` vulnerabilities `))
			summary.WriteString("in modules you require")
			summary.WriteString(choose(h.scanLevel.WantSymbols(), ", but your code doesn't appear to call these vulnerabilities.", "."))
		}
	}
	return summary.String()
}

func (h *TextHandler) summarySuggestion() string {
	var sugg strings.Builder
	switch h.scanLevel {
	case govulncheck.ScanLevelSymbol:
		if !h.showVerbose {
			sugg.WriteString("Use " + verboseMessage + ".")
		}
	case govulncheck.ScanLevelPackage:
		sugg.WriteString("Use " + symbolMessage)
		if !h.showVerbose {
			sugg.WriteString(" and " + verboseMessage)
		}
		sugg.WriteString(".")
	case govulncheck.ScanLevelModule:
		sugg.WriteString("Use " + symbolMessage + ".")
	}
	return sugg.String()
}

func (h *TextHandler) style(style style, values ...any) {
	if h.showColor {
		switch style {
		default:
			h.print(colorReset)
		case osvCalledStyle:
			h.print(colorBold, fgRed)
		case osvImportedStyle:
			h.print(colorBold, fgGreen)
		case detailsStyle:
			h.print(colorFaint)
		case sectionStyle:
			h.print(fgBlue)
		case keyStyle:
			h.print(colorFaint, fgYellow)
		case valueStyle:
			h.print(colorBold, fgCyan)
		}
	}
	h.print(values...)
	if h.showColor && len(values) > 0 {
		h.print(colorReset)
	}
}

func (h *TextHandler) print(values ...any) int {
	total, w := 0, 0
	for _, v := range values {
		if h.err != nil {
			return total
		}
		// do we need to specialize for some types, like time?
		w, h.err = fmt.Fprint(h.w, v)
		total += w
	}
	return total
}

// wrap wraps s to fit in maxWidth by breaking it into lines at whitespace. If a
// single word is longer than maxWidth, it is retained as its own line.
func (h *TextHandler) wrap(indent string, s string, maxWidth int) {
	w := 0
	for _, f := range strings.Fields(s) {
		if w > 0 && w+len(f)+1 > maxWidth {
			// line would be too long with this word
			h.print("\n")
			w = 0
		}
		if w == 0 {
			// first field on line, indent
			w = h.print(indent)
		} else {
			// not first word, space separate
			w += h.print(" ")
		}
		// now write the word
		w += h.print(f)
	}
}

func choose[t any](b bool, yes, no t) t {
	if b {
		return yes
	}
	return no
}

func isStdFindings(findings []*findingSummary) bool {
	for _, f := range findings {
		if vulncheck.IsStdPackage(f.Trace[0].Package) || f.Trace[0].Module == internal.GoStdModulePath {
			return true
		}
	}
	return false
}
