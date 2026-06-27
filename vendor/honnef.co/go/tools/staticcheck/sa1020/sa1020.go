package sa1020

import (
	"go/constant"
	"net"
	"strconv"
	"strings"

	"honnef.co/go/tools/analysis/callcheck"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/internal/passes/buildir"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA1020",
		Requires: []*analysis.Analyzer{buildir.Analyzer},
		Run:      callcheck.Analyzer(checkListenAddressRules),
	},
	Doc: &lint.RawDocumentation{
		Title:    `Using an invalid host:port pair with a \'net.Listen\'-related function`,
		Since:    "2017.1",
		Severity: lint.SeverityError,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var checkListenAddressRules = map[string]callcheck.Check{
	"net/http.ListenAndServe":    checkValidHostPort(0),
	"net/http.ListenAndServeTLS": checkValidHostPort(0),
}

func checkValidHostPort(arg int) callcheck.Check {
	return func(call *callcheck.Call) {
		if !ValidHostPort(call.Args[arg].Value) {
			const MsgInvalidHostPort = "invalid port or service name in host:port pair"
			call.Args[arg].Invalid(MsgInvalidHostPort)
		}
	}
}

func ValidHostPort(v callcheck.Value) bool {
	if k := callcheck.ExtractConstExpectKind(v, constant.String); k != nil {
		s := constant.StringVal(k.Value)
		if s == "" {
			return true
		}
		_, port, err := net.SplitHostPort(s)
		if err != nil {
			return false
		}
		// TODO(dh): check hostname
		if !validatePort(port) {
			return false
		}
	}
	return true
}

func validateServiceName(s string) bool {
	if len(s) < 1 || len(s) > 15 {
		return false
	}
	if s[0] == '-' || s[len(s)-1] == '-' {
		return false
	}
	if strings.Contains(s, "--") {
		return false
	}
	hasLetter := false
	for _, r := range s {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			hasLetter = true
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		return false
	}
	return hasLetter
}

func validatePort(s string) bool {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return validateServiceName(s)
	}
	return n >= 0 && n <= 65535
}
