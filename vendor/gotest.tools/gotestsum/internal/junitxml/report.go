/*Package junitxml creates a JUnit XML report from a testjson.Execution.
 */
package junitxml

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	"gotest.tools/gotestsum/internal/log"
	"gotest.tools/gotestsum/testjson"
)

// JUnitTestSuites is a collection of JUnit test suites.
type JUnitTestSuites struct {
	XMLName  xml.Name `xml:"testsuites"`
	Name     string   `xml:"name,attr,omitempty"`
	Tests    int      `xml:"tests,attr"`
	Failures int      `xml:"failures,attr"`
	Errors   int      `xml:"errors,attr"`
	Time     string   `xml:"time,attr"`
	Suites   []JUnitTestSuite
}

// JUnitTestSuite is a single JUnit test suite which may contain many
// testcases.
type JUnitTestSuite struct {
	XMLName    xml.Name        `xml:"testsuite"`
	Tests      int             `xml:"tests,attr"`
	Failures   int             `xml:"failures,attr"`
	Skipped    int             `xml:"skipped,attr,omitempty"`
	Time       string          `xml:"time,attr"`
	Name       string          `xml:"name,attr"`
	Properties []JUnitProperty `xml:"properties>property,omitempty"`
	TestCases  []JUnitTestCase
	Timestamp  string `xml:"timestamp,attr"`
}

// JUnitTestCase is a single test case with its result.
type JUnitTestCase struct {
	XMLName     xml.Name          `xml:"testcase"`
	Classname   string            `xml:"classname,attr"`
	Name        string            `xml:"name,attr"`
	Time        string            `xml:"time,attr"`
	Properties  *JUnitProperties  `xml:"properties,omitempty"`
	SkipMessage *JUnitSkipMessage `xml:"skipped,omitempty"`
	Failure     *JUnitFailure     `xml:"failure,omitempty"`
}

// JUnitSkipMessage contains the reason why a testcase was skipped.
type JUnitSkipMessage struct {
	Message string `xml:"message,attr"`
}

// JUnitProperties is a wrapper for the <properties> tag as
// encoding/xml would otherwise always create an empty one.
type JUnitProperties struct {
	Properties []JUnitProperty `xml:"property,omitempty"`
}

// JUnitProperty represents a key/value pair used to define properties.
type JUnitProperty struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

// JUnitFailure contains data related to a failed test.
type JUnitFailure struct {
	Message  string `xml:"message,attr"`
	Type     string `xml:"type,attr"`
	Contents string `xml:",chardata"`
}

// Config used to write a junit XML document.
type Config struct {
	ProjectName             string
	FormatTestSuiteName     FormatFunc
	FormatTestCaseClassname FormatFunc
	HideEmptyPackages       bool
	HideSkippedTests        bool
	// This is used for tests to have a consistent timestamp
	customTimestamp string
	customElapsed   string
}

// FormatFunc converts a string from one format into another.
type FormatFunc func(string) string

// Write creates an XML document and writes it to out.
func Write(out io.Writer, exec *testjson.Execution, cfg Config) error {
	if err := write(out, generate(exec, cfg)); err != nil {
		return fmt.Errorf("failed to write JUnit XML: %v", err)
	}
	return nil
}

func generate(exec *testjson.Execution, cfg Config) JUnitTestSuites {
	cfg = configWithDefaults(cfg)
	version := goVersion()

	total := exec.Total()
	if cfg.HideSkippedTests && len(exec.Skipped()) > 0 {
		total -= len(exec.Skipped())
	}

	suites := JUnitTestSuites{
		Name:     cfg.ProjectName,
		Tests:    total,
		Failures: len(exec.Failed()),
		Errors:   len(exec.Errors()),
		Time:     formatDurationAsSeconds(exec.Elapsed()),
	}

	if cfg.customElapsed != "" {
		suites.Time = cfg.customElapsed
	}

	for _, pkgname := range exec.Packages() {
		pkg := exec.Package(pkgname)
		if cfg.HideEmptyPackages && pkg.IsEmpty() {
			continue
		}

		if cfg.HideSkippedTests && len(pkg.Skipped) > 0 {
			pkg.Total -= len(pkg.Skipped)
			pkg.Skipped = nil
		}

		junitpkg := JUnitTestSuite{
			Name:       cfg.FormatTestSuiteName(pkgname),
			Tests:      pkg.Total,
			Time:       formatDurationAsSeconds(pkg.Elapsed()),
			Properties: packageProperties(version),
			TestCases:  packageTestCases(pkg, cfg.FormatTestCaseClassname),
			Failures:   len(pkg.Failed),
			Skipped:    len(pkg.Skipped),
			Timestamp:  cfg.customTimestamp,
		}
		if cfg.customTimestamp == "" {
			junitpkg.Timestamp = pkg.Start.Format(time.RFC3339)
		}
		suites.Suites = append(suites.Suites, junitpkg)
	}
	return suites
}

func configWithDefaults(cfg Config) Config {
	noop := func(v string) string {
		return v
	}
	if cfg.FormatTestSuiteName == nil {
		cfg.FormatTestSuiteName = noop
	}
	if cfg.FormatTestCaseClassname == nil {
		cfg.FormatTestCaseClassname = noop
	}
	return cfg
}

func formatDurationAsSeconds(d time.Duration) string {
	return fmt.Sprintf("%f", d.Seconds())
}

func packageProperties(goVersion string) []JUnitProperty {
	return []JUnitProperty{
		{Name: "go.version", Value: goVersion},
	}
}

// goVersion returns the version as reported by the go binary in PATH. This
// version will not be the same as runtime.Version, which is always the version
// of go used to build the gotestsum binary.
//
// To skip the os/exec call set the GOVERSION environment variable to the
// desired value.
func goVersion() string {
	if version, ok := os.LookupEnv("GOVERSION"); ok {
		return version
	}
	log.Debugf("exec: go version")
	cmd := exec.Command("go", "version")
	out, err := cmd.Output()
	if err != nil {
		log.Warnf("Failed to lookup go version for junit xml: %v", err)
		return "unknown"
	}
	return strings.TrimPrefix(strings.TrimSpace(string(out)), "go version ")
}

func packageTestCases(pkg *testjson.Package, formatClassname FormatFunc) []JUnitTestCase {
	cases := []JUnitTestCase{}

	if pkg.TestMainFailed() {
		var buf bytes.Buffer
		pkg.WriteOutputTo(&buf, 0) //nolint:errcheck
		jtc := newJUnitTestCase(testjson.TestCase{Test: "TestMain"}, formatClassname)
		jtc.Failure = &JUnitFailure{
			Message:  "Failed",
			Contents: buf.String(),
		}
		cases = append(cases, jtc)
	}

	for _, tc := range pkg.Failed {
		jtc := newJUnitTestCase(tc, formatClassname)
		jtc.Failure = &JUnitFailure{
			Message:  "Failed",
			Contents: strings.Join(pkg.OutputLines(tc), ""),
		}
		cases = append(cases, jtc)
	}

	for _, tc := range pkg.Skipped {
		jtc := newJUnitTestCase(tc, formatClassname)
		jtc.SkipMessage = &JUnitSkipMessage{
			Message: strings.Join(pkg.OutputLines(tc), ""),
		}
		cases = append(cases, jtc)
	}

	for _, tc := range pkg.Passed {
		jtc := newJUnitTestCase(tc, formatClassname)
		cases = append(cases, jtc)
	}
	return cases
}

func newJUnitTestCase(tc testjson.TestCase, formatClassname FormatFunc) JUnitTestCase {
	return JUnitTestCase{
		Classname:  formatClassname(tc.Package),
		Name:       tc.Test.Name(),
		Time:       formatDurationAsSeconds(tc.Elapsed),
		Properties: encodeAttributes(tc.Attributes),
	}
}

// encodeAttributes encodes the given attributes into a JUnitProperties wrapper.
// Properties are sorted in lexicographic order.
func encodeAttributes(attributes map[string]string) *JUnitProperties {
	if len(attributes) == 0 {
		return nil
	}

	properties := make([]JUnitProperty, 0, len(attributes))
	for k, v := range attributes {
		p := JUnitProperty{Name: k, Value: v}
		properties = append(properties, p)
	}

	slices.SortFunc(properties, func(a, b JUnitProperty) int {
		return strings.Compare(a.Name, b.Name)
	})
	return &JUnitProperties{Properties: properties}
}

func write(out io.Writer, suites JUnitTestSuites) error {
	doc, err := xml.MarshalIndent(suites, "", "\t")
	if err != nil {
		return err
	}
	_, err = out.Write([]byte(xml.Header))
	if err != nil {
		return err
	}
	_, err = out.Write(doc)
	return err
}
