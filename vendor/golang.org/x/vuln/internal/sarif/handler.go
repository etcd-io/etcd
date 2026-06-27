// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sarif

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"

	"golang.org/x/vuln/internal"
	"golang.org/x/vuln/internal/govulncheck"
	"golang.org/x/vuln/internal/osv"
	"golang.org/x/vuln/internal/traces"
)

// handler for sarif output.
type handler struct {
	w    io.Writer
	cfg  *govulncheck.Config
	osvs map[string]*osv.Entry
	// findings contains same-level findings for an
	// OSV at the most precise level of granularity
	// available. This means, for instance, that if
	// an osv is indeed called, then all findings for
	// the osv will have call stack info.
	findings map[string][]*govulncheck.Finding
}

func NewHandler(w io.Writer) *handler {
	return &handler{
		w:        w,
		osvs:     make(map[string]*osv.Entry),
		findings: make(map[string][]*govulncheck.Finding),
	}
}

func (h *handler) Config(c *govulncheck.Config) error {
	h.cfg = c
	return nil
}

func (h *handler) Progress(p *govulncheck.Progress) error {
	return nil // not needed by sarif
}

func (h *handler) SBOM(s *govulncheck.SBOM) error {
	return nil // not needed by sarif
}

func (h *handler) OSV(e *osv.Entry) error {
	h.osvs[e.ID] = e
	return nil
}

// moreSpecific favors a call finding over a non-call
// finding and a package finding over a module finding.
func moreSpecific(f1, f2 *govulncheck.Finding) int {
	if len(f1.Trace) > 1 && len(f2.Trace) > 1 {
		// Both are call stack findings.
		return 0
	}
	if len(f1.Trace) > 1 {
		return -1
	}
	if len(f2.Trace) > 1 {
		return 1
	}

	fr1, fr2 := f1.Trace[0], f2.Trace[0]
	if fr1.Function != "" && fr2.Function == "" {
		return -1
	}
	if fr1.Function == "" && fr2.Function != "" {
		return 1
	}
	if fr1.Package != "" && fr2.Package == "" {
		return -1
	}
	if fr1.Package == "" && fr2.Package != "" {
		return -1
	}
	return 0 // findings always have module info
}

func (h *handler) Finding(f *govulncheck.Finding) error {
	fs := h.findings[f.OSV]
	if len(fs) == 0 {
		fs = []*govulncheck.Finding{f}
	} else {
		if ms := moreSpecific(f, fs[0]); ms == -1 {
			// The new finding is more specific, so we need
			// to erase existing findings and add the new one.
			fs = []*govulncheck.Finding{f}
		} else if ms == 0 {
			// The new finding is equal to an existing one and
			// because of the invariant on h.findings, it is
			// also equal to all existing ones.
			fs = append(fs, f)
		}
		// Otherwise, the new finding is at a less precise level.
	}
	h.findings[f.OSV] = fs
	return nil
}

// Flush is used to print out to w the sarif json output.
// This is needed as sarif is not streamed.
func (h *handler) Flush() error {
	sLog := toSarif(h)
	s, err := json.MarshalIndent(sLog, "", "  ")
	if err != nil {
		return err
	}
	h.w.Write(s)
	return nil
}

func toSarif(h *handler) Log {
	cfg := h.cfg
	r := Run{
		Tool: Tool{
			Driver: Driver{
				Name:           cfg.ScannerName,
				Version:        cfg.ScannerVersion,
				InformationURI: "https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck",
				Properties:     *cfg,
				Rules:          rules(h),
			},
		},
		Results: results(h),
	}

	return Log{
		Version: "2.1.0",
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Runs:    []Run{r},
	}
}

func rules(h *handler) []Rule {
	rs := make([]Rule, 0, len(h.findings)) // must not be nil
	for id := range h.findings {
		osv := h.osvs[id]
		// s is either summary if it exists, or details
		// otherwise. Govulncheck text does the same.
		s := osv.Summary
		if s == "" {
			s = osv.Details
		}
		rs = append(rs, Rule{
			ID:               osv.ID,
			ShortDescription: Description{Text: fmt.Sprintf("[%s] %s", osv.ID, s)},
			FullDescription:  Description{Text: s},
			HelpURI:          fmt.Sprintf("https://pkg.go.dev/vuln/%s", osv.ID),
			Help:             Description{Text: osv.Details},
			Properties:       RuleTags{Tags: tags(osv)},
		})
	}
	sort.SliceStable(rs, func(i, j int) bool { return rs[i].ID < rs[j].ID })
	return rs
}

// tags returns an slice of zero or
// more aliases of o.
func tags(o *osv.Entry) []string {
	if len(o.Aliases) > 0 {
		return o.Aliases
	}
	return []string{} // must not be nil
}

func results(h *handler) []Result {
	results := make([]Result, 0, len(h.findings)) // must not be nil
	for osv, fs := range h.findings {
		var locs []Location
		if h.cfg.ScanMode != govulncheck.ScanModeBinary {
			// Attach result to the go.mod file for source analysis.
			// But there is no such place for binaries.
			locs = []Location{{PhysicalLocation: PhysicalLocation{
				ArtifactLocation: ArtifactLocation{
					URI:       "go.mod",
					URIBaseID: SrcRootID,
				},
				Region: Region{StartLine: 1}, // for now, point to the first line
			},
				Message: Description{Text: fmt.Sprintf("Findings for vulnerability %s", osv)}, // not having a message here results in an invalid sarif
			}}
		}

		res := Result{
			RuleID:    osv,
			Level:     level(fs[0], h.cfg),
			Message:   Description{Text: resultMessage(fs, h.cfg)},
			Stacks:    stacks(h, fs),
			CodeFlows: codeFlows(h, fs),
			Locations: locs,
		}
		results = append(results, res)
	}
	sort.SliceStable(results, func(i, j int) bool { return results[i].RuleID < results[j].RuleID }) // for deterministic output
	return results
}

func resultMessage(findings []*govulncheck.Finding, cfg *govulncheck.Config) string {
	// We can infer the findings' level by just looking at the
	// top trace frame of any finding.
	frame := findings[0].Trace[0]
	uniqueElems := make(map[string]bool)
	if frame.Function == "" && frame.Package == "" { // module level findings
		for _, f := range findings {
			uniqueElems[f.Trace[0].Module] = true
		}
	} else { // symbol and package level findings
		for _, f := range findings {
			uniqueElems[f.Trace[0].Package] = true
		}
	}
	var elems []string
	for e := range uniqueElems {
		elems = append(elems, e)
	}
	sort.Strings(elems)

	l := len(elems)
	elemList := list(elems)
	main, addition := "", ""
	const runCallAnalysis = "Run the call-level analysis to understand whether your code actually calls the vulnerabilities."
	switch {
	case frame.Function != "":
		main = fmt.Sprintf("calls vulnerable functions in %d package%s (%s).", l, choose("", "s", l == 1), elemList)
	case frame.Package != "":
		main = fmt.Sprintf("imports %d vulnerable package%s (%s)", l, choose("", "s", l == 1), elemList)
		addition = choose(", but doesn’t appear to call any of the vulnerable symbols.", ". "+runCallAnalysis, cfg.ScanLevel.WantSymbols())
	default:
		main = fmt.Sprintf("depends on %d vulnerable module%s (%s)", l, choose("", "s", l == 1), elemList)
		informational := ", but doesn't appear to " + choose("call", "import", cfg.ScanLevel.WantSymbols()) + " any of the vulnerable symbols."
		addition = choose(informational, ". "+runCallAnalysis, cfg.ScanLevel.WantPackages())
	}

	return fmt.Sprintf("Your code %s%s", main, addition)
}

const (
	errorLevel         = "error"
	warningLevel       = "warning"
	informationalLevel = "note"
)

func level(f *govulncheck.Finding, cfg *govulncheck.Config) string {
	fr := f.Trace[0]
	switch {
	case cfg.ScanLevel.WantSymbols():
		if fr.Function != "" {
			return errorLevel
		}
		if fr.Package != "" {
			return warningLevel
		}
		return informationalLevel
	case cfg.ScanLevel.WantPackages():
		if fr.Package != "" {
			return errorLevel
		}
		return warningLevel
	default:
		return errorLevel
	}
}

func stacks(h *handler, fs []*govulncheck.Finding) []Stack {
	if fs[0].Trace[0].Function == "" { // not call level findings
		return nil
	}

	var stacks []Stack
	for _, f := range fs {
		stacks = append(stacks, stack(h, f))
	}
	// Sort stacks for deterministic output. We sort by message
	// which is effectively sorting by full symbol name. The
	// performance should not be an issue here.
	sort.SliceStable(stacks, func(i, j int) bool { return stacks[i].Message.Text < stacks[j].Message.Text })
	return stacks
}

// stack transforms call stack in f to a sarif stack.
func stack(h *handler, f *govulncheck.Finding) Stack {
	trace := f.Trace
	top := trace[len(trace)-1] // belongs to top level module

	frames := make([]Frame, 0, len(trace)) // must not be nil
	for i := len(trace) - 1; i >= 0; i-- { // vulnerable symbol is at the top frame
		frame := trace[i]
		pos := govulncheck.Position{Line: 1, Column: 1}
		if frame.Position != nil {
			pos = *frame.Position
		}

		sf := Frame{
			Module:   frame.Module + "@" + frame.Version,
			Location: Location{Message: Description{Text: symbol(frame)}}, // show the (full) symbol name
		}
		file, base := fileURIInfo(pos.Filename, top.Module, frame.Module, frame.Version)
		if h.cfg.ScanMode != govulncheck.ScanModeBinary {
			sf.Location.PhysicalLocation = PhysicalLocation{
				ArtifactLocation: ArtifactLocation{
					URI:       file,
					URIBaseID: base,
				},
				Region: Region{
					StartLine:   pos.Line,
					StartColumn: pos.Column,
				},
			}
		}
		frames = append(frames, sf)
	}

	return Stack{
		Frames:  frames,
		Message: Description{Text: fmt.Sprintf("A call stack for vulnerable function %s", symbol(trace[0]))},
	}
}

func codeFlows(h *handler, fs []*govulncheck.Finding) []CodeFlow {
	if fs[0].Trace[0].Function == "" { // not call level findings
		return nil
	}

	// group call stacks per symbol. There should
	// be one call stack currently per symbol, but
	// this might change in the future.
	m := make(map[govulncheck.Frame][]*govulncheck.Finding)
	for _, f := range fs {
		// fr.Position is currently the position
		// of the definition of the vuln symbol
		fr := *f.Trace[0]
		m[fr] = append(m[fr], f)
	}

	var codeFlows []CodeFlow
	for fr, fs := range m {
		tfs := threadFlows(h, fs)
		codeFlows = append(codeFlows, CodeFlow{
			ThreadFlows: tfs,
			// TODO: should we instead show the message from govulncheck text output?
			Message: Description{Text: fmt.Sprintf("A summarized code flow for vulnerable function %s", symbol(&fr))},
		})
	}
	// Sort flows for deterministic output. We sort by message
	// which is effectively sorting by full symbol name. The
	// performance should not be an issue here.
	sort.SliceStable(codeFlows, func(i, j int) bool { return codeFlows[i].Message.Text < codeFlows[j].Message.Text })
	return codeFlows
}

func threadFlows(h *handler, fs []*govulncheck.Finding) []ThreadFlow {
	tfs := make([]ThreadFlow, 0, len(fs)) // must not be nil
	for _, f := range fs {
		trace := traces.Compact(f)
		top := trace[len(trace)-1] // belongs to top level module

		var tf []ThreadFlowLocation
		for i := len(trace) - 1; i >= 0; i-- { // vulnerable symbol is at the top frame
			// TODO: should we, similar to govulncheck text output, only
			// mention three elements of the compact trace?
			frame := trace[i]
			pos := govulncheck.Position{Line: 1, Column: 1}
			if frame.Position != nil {
				pos = *frame.Position
			}

			tfl := ThreadFlowLocation{
				Module:   frame.Module + "@" + frame.Version,
				Location: Location{Message: Description{Text: symbol(frame)}}, // show the (full) symbol name
			}
			file, base := fileURIInfo(pos.Filename, top.Module, frame.Module, frame.Version)
			if h.cfg.ScanMode != govulncheck.ScanModeBinary {
				tfl.Location.PhysicalLocation = PhysicalLocation{
					ArtifactLocation: ArtifactLocation{
						URI:       file,
						URIBaseID: base,
					},
					Region: Region{
						StartLine:   pos.Line,
						StartColumn: pos.Column,
					},
				}
			}
			tf = append(tf, tfl)
		}
		tfs = append(tfs, ThreadFlow{Locations: tf})
	}
	return tfs
}

func fileURIInfo(filename, top, module, version string) (string, string) {
	if top == module {
		return filename, SrcRootID
	}
	if module == internal.GoStdModulePath {
		return filename, GoRootID
	}
	return filepath.ToSlash(filepath.Join(module+"@"+version, filename)), GoModCacheID
}
