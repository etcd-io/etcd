package formatter

import (
	"bytes"
	"encoding/xml"
	plain "text/template"

	"github.com/mgechev/revive/lint"
)

// Checkstyle is an implementation of the [lint.Formatter] interface
// which formats the errors to Checkstyle-like format.
type Checkstyle struct {
	Metadata lint.FormatterMetadata
}

// Name returns the name of the formatter.
func (*Checkstyle) Name() string {
	return "checkstyle"
}

type issue struct {
	Line       int
	Col        int
	What       string
	Confidence float64
	Severity   lint.Severity
	RuleName   string
}

// Format formats the failures gotten from the lint.
func (*Checkstyle) Format(failures <-chan lint.Failure, config lint.Config) (string, error) {
	issues := map[string][]issue{}
	for failure := range failures {
		buf := new(bytes.Buffer)
		xml.Escape(buf, []byte(failure.Failure))
		what := buf.String()
		iss := issue{
			Line:       failure.Position.Start.Line,
			Col:        failure.Position.Start.Column,
			What:       what,
			Confidence: failure.Confidence,
			Severity:   severity(config, failure),
			RuleName:   failure.RuleName,
		}
		fn := failure.Filename()
		if issues[fn] == nil {
			issues[fn] = []issue{}
		}
		issues[fn] = append(issues[fn], iss)
	}

	t, err := plain.New("revive").Parse(checkstyleTemplate)
	if err != nil {
		return "", err
	}

	buf := new(bytes.Buffer)

	err = t.Execute(buf, issues)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

const checkstyleTemplate = `<?xml version='1.0' encoding='UTF-8'?>
<checkstyle version="5.0">
{{- range $k, $v := . }}
    <file name="{{ $k }}">
      {{- range $i, $issue := $v }}
      <error line="{{ $issue.Line }}" column="{{ $issue.Col }}" message="{{ $issue.What }} (confidence {{ $issue.Confidence}})" severity="{{ $issue.Severity }}" source="revive/{{ $issue.RuleName }}"/>
      {{- end }}
    </file>
{{- end }}
</checkstyle>`
