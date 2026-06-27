package matrix

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dnephin/pflag"
	"gotest.tools/gotestsum/internal/log"
	"gotest.tools/gotestsum/testjson"
)

func Run(name string, args []string) error {
	flags, opts := setupFlags(name)
	switch err := flags.Parse(args); {
	case err == pflag.ErrHelp:
		return nil
	case err != nil:
		usage(os.Stderr, name, flags)
		return err
	}
	opts.stdin = os.Stdin
	opts.stdout = os.Stdout
	return run(*opts)
}

type options struct {
	numPartitions      uint
	timingFilesPattern string
	debug              bool

	// shims for testing
	stdin  io.Reader
	stdout io.Writer
}

func setupFlags(name string) (*pflag.FlagSet, *options) {
	opts := &options{}
	flags := pflag.NewFlagSet(name, pflag.ContinueOnError)
	flags.SetInterspersed(false)
	flags.Usage = func() {
		usage(os.Stdout, name, flags)
	}
	flags.UintVar(&opts.numPartitions, "partitions", 0,
		"number of parallel partitions to create in the test matrix")
	flags.StringVar(&opts.timingFilesPattern, "timing-files", "",
		"glob pattern to match files that contain test2json events, ex: ./logs/*.log")
	flags.BoolVar(&opts.debug, "debug", false,
		"enable debug logging")
	return flags, opts
}

func usage(out io.Writer, name string, flags *pflag.FlagSet) {
	fmt.Fprintf(out, `Usage:
    %[1]s [flags]

Read a list of packages from stdin and output a GitHub Actions matrix strategy
that splits the packages by previous run times to minimize overall CI runtime.

    echo -n "matrix=" >> $GITHUB_OUTPUT
    go list ./... | %[1]s --timing-files ./*.log --partitions 4 >> $GITHUB_OUTPUT

The output of the command is a JSON object that can be used as the matrix
strategy for a test job.


Flags:
`, name)
	flags.SetOutput(out)
	flags.PrintDefaults()
}

func run(opts options) error {
	log.SetLevel(log.InfoLevel)
	if opts.debug {
		log.SetLevel(log.DebugLevel)
	}
	if opts.numPartitions < 2 {
		return fmt.Errorf("--partitions must be atleast 2")
	}
	if opts.timingFilesPattern == "" {
		return fmt.Errorf("--timing-files is required")
	}

	pkgs, err := readPackages(opts.stdin)
	if err != nil {
		return fmt.Errorf("failed to read packages from stdin: %v", err)
	}

	files, err := readTimingReports(opts)
	if err != nil {
		return fmt.Errorf("failed to read or delete timing files: %v", err)
	}
	defer closeFiles(files)

	pkgTiming, err := packageTiming(files)
	if err != nil {
		return err
	}

	buckets := bucketPackages(packagePercentile(pkgTiming), pkgs, opts.numPartitions)
	return writeMatrix(opts.stdout, buckets)
}

func readPackages(stdin io.Reader) ([]string, error) {
	var packages []string
	scan := bufio.NewScanner(stdin)
	for scan.Scan() {
		packages = append(packages, scan.Text())
	}
	return packages, scan.Err()
}

func readTimingReports(opts options) ([]*os.File, error) {
	fileNames, err := filepath.Glob(opts.timingFilesPattern)
	if err != nil {
		return nil, err
	}

	files := make([]*os.File, 0, len(fileNames))
	for _, fileName := range fileNames {
		fh, err := os.Open(fileName)
		if err != nil {
			return nil, err
		}
		files = append(files, fh)
	}

	log.Infof("Found %v timing files in %v", len(files), opts.timingFilesPattern)
	return files, nil
}

func parseEvent(reader io.Reader) (testjson.TestEvent, error) {
	event := testjson.TestEvent{}
	err := json.NewDecoder(reader).Decode(&event)
	return event, err
}

func packageTiming(files []*os.File) (map[string][]time.Duration, error) {
	timing := make(map[string][]time.Duration)
	for _, fh := range files {
		exec, err := testjson.ScanTestOutput(testjson.ScanConfig{Stdout: fh})
		if err != nil {
			return nil, fmt.Errorf("failed to read events from %v: %v", fh.Name(), err)
		}

		for _, pkg := range exec.Packages() {
			timing[pkg] = append(timing[pkg], exec.Package(pkg).Elapsed())
		}
	}
	return timing, nil
}

func packagePercentile(timing map[string][]time.Duration) map[string]time.Duration {
	result := make(map[string]time.Duration)
	for pkg, times := range timing {
		lenTimes := len(times)
		if lenTimes == 0 {
			result[pkg] = 0
			continue
		}

		sort.Slice(times, func(i, j int) bool {
			return times[i] < times[j]
		})

		r := int(math.Ceil(0.85 * float64(lenTimes)))
		if r == 0 {
			result[pkg] = times[0]
			continue
		}
		result[pkg] = times[r-1]
	}
	return result
}

func closeFiles(files []*os.File) {
	for _, fh := range files {
		_ = fh.Close()
	}
}

func bucketPackages(timing map[string]time.Duration, packages []string, n uint) []bucket {
	sort.SliceStable(packages, func(i, j int) bool {
		return timing[packages[i]] >= timing[packages[j]]
	})

	buckets := make([]bucket, n)
	for _, pkg := range packages {
		i := minBucket(buckets)
		buckets[i].Total += timing[pkg]
		buckets[i].Packages = append(buckets[i].Packages, pkg)
		log.Debugf("adding %v (%v) to bucket %v with total %v",
			pkg, timing[pkg], i, buckets[i].Total)
	}
	return buckets
}

func minBucket(buckets []bucket) int {
	var n int
	var minDuration time.Duration = -1
	for i, b := range buckets {
		switch {
		case minDuration < 0 || b.Total < minDuration:
			minDuration = b.Total
			n = i
		case b.Total == minDuration && len(buckets[i].Packages) < len(buckets[n].Packages):
			n = i
		}
	}
	return n
}

type bucket struct {
	Total    time.Duration
	Packages []string
}

type matrix struct {
	Include []Partition `json:"include"`
}

type Partition struct {
	ID               int    `json:"id"`
	EstimatedRuntime string `json:"estimatedRuntime"`
	Packages         string `json:"packages"`
	Description      string `json:"description"`
}

func writeMatrix(out io.Writer, buckets []bucket) error {
	m := matrix{Include: make([]Partition, len(buckets))}
	for i, bucket := range buckets {
		p := Partition{
			ID:               i,
			EstimatedRuntime: bucket.Total.String(),
			Packages:         strings.Join(bucket.Packages, " "),
		}
		if len(bucket.Packages) > 0 {
			var extra string
			if len(bucket.Packages) > 1 {
				extra = fmt.Sprintf(" and %d others", len(bucket.Packages)-1)
			}
			p.Description = fmt.Sprintf("%d - %v%v",
				p.ID, testjson.RelativePackagePath(bucket.Packages[0]), extra)
		}

		m.Include[i] = p
	}

	log.Debugf("%v\n", debugMatrix(m))

	err := json.NewEncoder(out).Encode(m)
	if err != nil {
		return fmt.Errorf("failed to json encode output: %v", err)
	}
	return nil
}

type debugMatrix matrix

func (d debugMatrix) String() string {
	raw, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return fmt.Sprintf("failed to marshal: %v", err.Error())
	}
	return string(raw)
}
