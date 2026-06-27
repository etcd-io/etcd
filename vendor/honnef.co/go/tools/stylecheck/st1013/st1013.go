package st1013

import (
	"fmt"
	"go/ast"
	"go/constant"
	"strconv"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/edit"
	"honnef.co/go/tools/analysis/facts/generated"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/config"
	"honnef.co/go/tools/pattern"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "ST1013",
		Run:      run,
		Requires: append([]*analysis.Analyzer{generated.Analyzer, config.Analyzer}, code.RequiredAnalyzers...),
	},
	Doc: &lint.RawDocumentation{
		Title: `Should use constants for HTTP error codes, not magic numbers`,
		Text: `HTTP has a tremendous number of status codes. While some of those are
well known (200, 400, 404, 500), most of them are not. The \'net/http\'
package provides constants for all status codes that are part of the
various specifications. It is recommended to use these constants
instead of hard-coding magic numbers, to vastly improve the
readability of your code.`,
		Since:   "2019.1",
		Options: []string{"http_status_code_whitelist"},
		MergeIf: lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var query = pattern.MustParse(`
	(CallExpr
		(Symbol
			name@(Or
			"net/http.Error"
			"net/http.Redirect"
			"net/http.StatusText"
			"net/http.RedirectHandler"))
		args)`)

func run(pass *analysis.Pass) (any, error) {
	whitelist := map[string]bool{}
	for _, code := range config.For(pass).HTTPStatusCodeWhitelist {
		whitelist[code] = true
	}
	for _, m := range code.Matches(pass, query) {
		var arg int
		switch m.State["name"].(string) {
		case "net/http.Error":
			arg = 2
		case "net/http.Redirect":
			arg = 3
		case "net/http.StatusText":
			arg = 0
		case "net/http.RedirectHandler":
			arg = 1
		default:
			continue
		}
		args := m.State["args"].([]ast.Expr)
		if arg >= len(args) {
			continue
		}
		tv, ok := code.IntegerLiteral(pass, args[arg])
		if !ok {
			continue
		}
		n, ok := constant.Int64Val(tv.Value)
		if !ok {
			continue
		}
		if whitelist[strconv.FormatInt(n, 10)] {
			continue
		}

		s, ok := httpStatusCodes[n]
		if !ok {
			continue
		}
		lit := args[arg]
		report.Report(pass, lit, fmt.Sprintf("should use constant http.%s instead of numeric literal %d", s, n),
			report.FilterGenerated(),
			report.Fixes(edit.Fix(fmt.Sprintf("Use http.%s instead of %d", s, n), edit.ReplaceWithString(lit, "http."+s))))
	}
	return nil, nil
}

var httpStatusCodes = map[int64]string{
	100: "StatusContinue",
	101: "StatusSwitchingProtocols",
	102: "StatusProcessing",
	200: "StatusOK",
	201: "StatusCreated",
	202: "StatusAccepted",
	203: "StatusNonAuthoritativeInfo",
	204: "StatusNoContent",
	205: "StatusResetContent",
	206: "StatusPartialContent",
	207: "StatusMultiStatus",
	208: "StatusAlreadyReported",
	226: "StatusIMUsed",
	300: "StatusMultipleChoices",
	301: "StatusMovedPermanently",
	302: "StatusFound",
	303: "StatusSeeOther",
	304: "StatusNotModified",
	305: "StatusUseProxy",
	307: "StatusTemporaryRedirect",
	308: "StatusPermanentRedirect",
	400: "StatusBadRequest",
	401: "StatusUnauthorized",
	402: "StatusPaymentRequired",
	403: "StatusForbidden",
	404: "StatusNotFound",
	405: "StatusMethodNotAllowed",
	406: "StatusNotAcceptable",
	407: "StatusProxyAuthRequired",
	408: "StatusRequestTimeout",
	409: "StatusConflict",
	410: "StatusGone",
	411: "StatusLengthRequired",
	412: "StatusPreconditionFailed",
	413: "StatusRequestEntityTooLarge",
	414: "StatusRequestURITooLong",
	415: "StatusUnsupportedMediaType",
	416: "StatusRequestedRangeNotSatisfiable",
	417: "StatusExpectationFailed",
	418: "StatusTeapot",
	422: "StatusUnprocessableEntity",
	423: "StatusLocked",
	424: "StatusFailedDependency",
	426: "StatusUpgradeRequired",
	428: "StatusPreconditionRequired",
	429: "StatusTooManyRequests",
	431: "StatusRequestHeaderFieldsTooLarge",
	451: "StatusUnavailableForLegalReasons",
	500: "StatusInternalServerError",
	501: "StatusNotImplemented",
	502: "StatusBadGateway",
	503: "StatusServiceUnavailable",
	504: "StatusGatewayTimeout",
	505: "StatusHTTPVersionNotSupported",
	506: "StatusVariantAlsoNegotiates",
	507: "StatusInsufficientStorage",
	508: "StatusLoopDetected",
	510: "StatusNotExtended",
	511: "StatusNetworkAuthenticationRequired",
}
