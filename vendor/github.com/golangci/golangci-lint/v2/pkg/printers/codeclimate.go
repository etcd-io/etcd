package printers

import (
	"encoding/json"
	"io"

	"github.com/golangci/golangci-lint/v2/pkg/logutils"
	"github.com/golangci/golangci-lint/v2/pkg/result"
)

const defaultCodeClimateSeverity = "critical"

// CodeClimate prints issues in the Code Climate format.
// https://github.com/codeclimate/platform/blob/HEAD/spec/analyzers/SPEC.md
type CodeClimate struct {
	log       logutils.Log
	w         io.Writer
	sanitizer severitySanitizer
}

func NewCodeClimate(log logutils.Log, w io.Writer) *CodeClimate {
	return &CodeClimate{
		log: log.Child(logutils.DebugKeyCodeClimatePrinter),
		w:   w,
		sanitizer: severitySanitizer{
			// https://github.com/codeclimate/platform/blob/HEAD/spec/analyzers/SPEC.md#data-types
			allowedSeverities: []string{"info", "minor", "major", defaultCodeClimateSeverity, "blocker"},
			defaultSeverity:   defaultCodeClimateSeverity,
		},
	}
}

func (p *CodeClimate) Print(issues []*result.Issue) error {
	ccIssues := make([]codeClimateIssue, 0, len(issues))

	for _, issue := range issues {
		ccIssue := codeClimateIssue{
			Description: issue.Description(),
			CheckName:   issue.FromLinter,
			Severity:    p.sanitizer.Sanitize(issue.Severity),
			Fingerprint: issue.Fingerprint(),
		}

		ccIssue.Location.Path = issue.Pos.Filename
		ccIssue.Location.Lines.Begin = issue.Pos.Line

		ccIssues = append(ccIssues, ccIssue)
	}

	err := p.sanitizer.Err()
	if err != nil {
		p.log.Infof("%v", err)
	}

	return json.NewEncoder(p.w).Encode(ccIssues)
}

// codeClimateIssue is a subset of the Code Climate spec.
// https://github.com/codeclimate/platform/blob/HEAD/spec/analyzers/SPEC.md#data-types
// It is just enough to support GitLab CI Code Quality.
// https://docs.gitlab.com/ee/ci/testing/code_quality.html#code-quality-report-format
type codeClimateIssue struct {
	Description string `json:"description"`
	CheckName   string `json:"check_name"`
	Severity    string `json:"severity,omitempty"`
	Fingerprint string `json:"fingerprint"`
	Location    struct {
		Path  string `json:"path"`
		Lines struct {
			Begin int `json:"begin"`
		} `json:"lines"`
	} `json:"location"`
}
