package spancheck

import (
	"flag"
	"fmt"
	"log"
	"regexp"
	"strings"
)

// Check is a type of check that can be enabled or disabled.
type Check int

const (
	// EndCheck if enabled, checks that span.End() is called after span creation and before the function returns.
	EndCheck Check = iota

	// SetStatusCheck if enabled, checks that `span.SetStatus(codes.Error, msg)` is called when returning an error.
	SetStatusCheck

	// RecordErrorCheck if enabled, checks that span.RecordError(err) is called when returning an error.
	RecordErrorCheck
)

var (
	startSpanSignatureCols     = 2
	defaultStartSpanSignatures = []string{
		// https://github.com/open-telemetry/opentelemetry-go/blob/98b32a6c3a87fbee5d34c063b9096f416b250897/trace/trace.go#L523
		`\(go.opentelemetry.io/otel/trace.Tracer\).Start:opentelemetry`,
		// https://pkg.go.dev/go.opencensus.io/trace#StartSpan
		`go.opencensus.io/trace.StartSpan:opencensus`,
		// https://github.com/census-instrumentation/opencensus-go/blob/v0.24.0/trace/trace_api.go#L66
		`go.opencensus.io/trace.StartSpanWithRemoteParent:opencensus`,
	}
)

func (c Check) String() string {
	switch c {
	case EndCheck:
		return "end"
	case SetStatusCheck:
		return "set-status"
	case RecordErrorCheck:
		return "record-error"
	default:
		return ""
	}
}

// Checks is a list of all checks by name.
var Checks = map[string]Check{
	EndCheck.String():         EndCheck,
	SetStatusCheck.String():   SetStatusCheck,
	RecordErrorCheck.String(): RecordErrorCheck,
}

type spanStartMatcher struct {
	signature *regexp.Regexp
	spanType  spanType
}

// Config is a configuration for the spancheck analyzer.
type Config struct {
	fs flag.FlagSet

	// EnabledChecks is a list of checks to enable by name.
	EnabledChecks []string

	// IgnoreChecksSignaturesSlice is a slice of strings that are turned into
	// the IgnoreSetStatusCheckSignatures regex.
	IgnoreChecksSignaturesSlice []string

	StartSpanMatchersSlice []string

	endCheckEnabled    bool
	setStatusEnabled   bool
	recordErrorEnabled bool

	// ignoreChecksSignatures is a regex that, if matched, disables the
	// SetStatus and RecordError checks on error.
	ignoreChecksSignatures *regexp.Regexp

	startSpanMatchers            []spanStartMatcher
	startSpanMatchersCustomRegex *regexp.Regexp
}

// NewDefaultConfig returns a new Config with default values.
func NewDefaultConfig() *Config {
	return &Config{
		EnabledChecks:          []string{EndCheck.String()},
		StartSpanMatchersSlice: defaultStartSpanSignatures,
	}
}

// finalize parses checks and signatures from the public string slices of Config.
func (c *Config) finalize() {
	c.parseSignatures()

	checks := parseChecks(c.EnabledChecks)
	c.endCheckEnabled = contains(checks, EndCheck)
	c.setStatusEnabled = contains(checks, SetStatusCheck)
	c.recordErrorEnabled = contains(checks, RecordErrorCheck)
}

// parseSignatures sets the Ignore*CheckSignatures regex from the string slices.
func (c *Config) parseSignatures() {
	c.parseIgnoreSignatures()
	c.parseStartSpanSignatures()
}

func (c *Config) parseIgnoreSignatures() {
	if c.ignoreChecksSignatures == nil && len(c.IgnoreChecksSignaturesSlice) > 0 {
		if len(c.IgnoreChecksSignaturesSlice) == 1 && c.IgnoreChecksSignaturesSlice[0] == "" {
			return
		}

		c.ignoreChecksSignatures = createRegex(c.IgnoreChecksSignaturesSlice)
	}
}

func (c *Config) parseStartSpanSignatures() {
	if c.startSpanMatchers != nil {
		return
	}

	customMatchers := []string{}
	for i, sig := range c.StartSpanMatchersSlice {
		parts := strings.Split(sig, ":")

		// Make sure we have both a signature and a telemetry type
		if len(parts) != startSpanSignatureCols {
			log.Default().Printf("[WARN] invalid start span signature \"%s\". expected regex:telemetry-type\n", sig)

			continue
		}

		sig, sigType := parts[0], parts[1]
		if len(sig) < 1 {
			log.Default().Print("[WARN] invalid start span signature, empty pattern")

			continue
		}

		spanType, ok := SpanTypes[sigType]
		if !ok {
			validSpanTypes := make([]string, 0, len(SpanTypes))
			for k := range SpanTypes {
				validSpanTypes = append(validSpanTypes, k)
			}

			log.Default().
				Printf("[WARN] invalid start span type \"%s\". expected one of %s\n", sigType, strings.Join(validSpanTypes, ", "))

			continue
		}

		regex, err := regexp.Compile(sig)
		if err != nil {
			log.Default().Printf("[WARN] failed to compile regex from signature %s: %v\n", sig, err)

			continue
		}

		c.startSpanMatchers = append(c.startSpanMatchers, spanStartMatcher{
			signature: regex,
			spanType:  spanType,
		})

		if i >= len(defaultStartSpanSignatures) {
			customMatchers = append(customMatchers, sig)
		}
	}

	c.startSpanMatchersCustomRegex = createRegex(customMatchers)
}

func parseChecks(checksSlice []string) []Check {
	if len(checksSlice) == 0 {
		return nil
	}

	checks := []Check{}
	for _, check := range checksSlice {
		checkName := strings.TrimSpace(check)
		if checkName == "" {
			continue
		}

		check, ok := Checks[checkName]
		if !ok {
			continue
		}

		checks = append(checks, check)
	}

	return checks
}

func createRegex(sigs []string) *regexp.Regexp {
	if len(sigs) == 0 {
		return nil
	}

	regex := fmt.Sprintf("(%s)", strings.Join(sigs, "|"))
	regexCompiled, err := regexp.Compile(regex)
	if err != nil {
		log.Default().Print("[WARN] failed to compile regex from signature flag", "regex", regex, "err", err)
		return nil
	}

	return regexCompiled
}

func contains(s []Check, e Check) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}

	return false
}
