package cli

import (
	"fmt"
	"os"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

var boolFlagTests = []struct {
	name     string
	expected string
}{
	{"help", "--help\t"},
	{"h", "-h\t"},
}

func TestBoolFlagHelpOutput(t *testing.T) {
	for _, test := range boolFlagTests {
		flag := BoolFlag{Name: test.name}
		output := flag.String()

		if output != test.expected {
			t.Errorf("%q does not match %q", output, test.expected)
		}
	}
}

var stringFlagTests = []struct {
	name     string
	usage    string
	value    string
	expected string
}{
	{"foo", "", "", "--foo value\t"},
	{"f", "", "", "-f value\t"},
	{"f", "The total `foo` desired", "all", "-f foo\tThe total foo desired (default: \"all\")"},
	{"test", "", "Something", "--test value\t(default: \"Something\")"},
	{"config,c", "Load configuration from `FILE`", "", "--config FILE, -c FILE\tLoad configuration from FILE"},
	{"config,c", "Load configuration from `CONFIG`", "config.json", "--config CONFIG, -c CONFIG\tLoad configuration from CONFIG (default: \"config.json\")"},
}

func TestStringFlagHelpOutput(t *testing.T) {
	for _, test := range stringFlagTests {
		flag := StringFlag{Name: test.name, Usage: test.usage, Value: test.value}
		output := flag.String()

		if output != test.expected {
			t.Errorf("%q does not match %q", output, test.expected)
		}
	}
}

func TestStringFlagWithEnvVarHelpOutput(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_FOO", "derp")
	for _, test := range stringFlagTests {
		flag := StringFlag{Name: test.name, Value: test.value, EnvVar: "APP_FOO"}
		output := flag.String()

		expectedSuffix := " [$APP_FOO]"
		if runtime.GOOS == "windows" {
			expectedSuffix = " [%APP_FOO%]"
		}
		if !strings.HasSuffix(output, expectedSuffix) {
			t.Errorf("%s does not end with"+expectedSuffix, output)
		}
	}
}

var stringSliceFlagTests = []struct {
	name     string
	value    *StringSlice
	expected string
}{
	{"foo", func() *StringSlice {
		s := &StringSlice{}
		s.Set("")
		return s
	}(), "--foo value\t"},
	{"f", func() *StringSlice {
		s := &StringSlice{}
		s.Set("")
		return s
	}(), "-f value\t"},
	{"f", func() *StringSlice {
		s := &StringSlice{}
		s.Set("Lipstick")
		return s
	}(), "-f value\t(default: \"Lipstick\")"},
	{"test", func() *StringSlice {
		s := &StringSlice{}
		s.Set("Something")
		return s
	}(), "--test value\t(default: \"Something\")"},
}

func TestStringSliceFlagHelpOutput(t *testing.T) {
	for _, test := range stringSliceFlagTests {
		flag := StringSliceFlag{Name: test.name, Value: test.value}
		output := flag.String()

		if output != test.expected {
			t.Errorf("%q does not match %q", output, test.expected)
		}
	}
}

func TestStringSliceFlagWithEnvVarHelpOutput(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_QWWX", "11,4")
	for _, test := range stringSliceFlagTests {
		flag := StringSliceFlag{Name: test.name, Value: test.value, EnvVar: "APP_QWWX"}
		output := flag.String()

		expectedSuffix := " [$APP_QWWX]"
		if runtime.GOOS == "windows" {
			expectedSuffix = " [%APP_QWWX%]"
		}
		if !strings.HasSuffix(output, expectedSuffix) {
			t.Errorf("%q does not end with"+expectedSuffix, output)
		}
	}
}

var intFlagTests = []struct {
	name     string
	expected string
}{
	{"hats", "--hats value\t(default: 9)"},
	{"H", "-H value\t(default: 9)"},
}

func TestIntFlagHelpOutput(t *testing.T) {
	for _, test := range intFlagTests {
		flag := IntFlag{Name: test.name, Value: 9}
		output := flag.String()

		if output != test.expected {
			t.Errorf("%s does not match %s", output, test.expected)
		}
	}
}

func TestIntFlagWithEnvVarHelpOutput(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_BAR", "2")
	for _, test := range intFlagTests {
		flag := IntFlag{Name: test.name, EnvVar: "APP_BAR"}
		output := flag.String()

		expectedSuffix := " [$APP_BAR]"
		if runtime.GOOS == "windows" {
			expectedSuffix = " [%APP_BAR%]"
		}
		if !strings.HasSuffix(output, expectedSuffix) {
			t.Errorf("%s does not end with"+expectedSuffix, output)
		}
	}
}

var int64FlagTests = []struct {
	name     string
	expected string
}{
	{"hats", "--hats value\t(default: 8589934592)"},
	{"H", "-H value\t(default: 8589934592)"},
}

func TestInt64FlagHelpOutput(t *testing.T) {
	for _, test := range int64FlagTests {
		flag := Int64Flag{Name: test.name, Value: 8589934592}
		output := flag.String()

		if output != test.expected {
			t.Errorf("%s does not match %s", output, test.expected)
		}
	}
}

func TestInt64FlagWithEnvVarHelpOutput(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_BAR", "2")
	for _, test := range int64FlagTests {
		flag := IntFlag{Name: test.name, EnvVar: "APP_BAR"}
		output := flag.String()

		expectedSuffix := " [$APP_BAR]"
		if runtime.GOOS == "windows" {
			expectedSuffix = " [%APP_BAR%]"
		}
		if !strings.HasSuffix(output, expectedSuffix) {
			t.Errorf("%s does not end with"+expectedSuffix, output)
		}
	}
}

var uintFlagTests = []struct {
	name     string
	expected string
}{
	{"nerfs", "--nerfs value\t(default: 41)"},
	{"N", "-N value\t(default: 41)"},
}

func TestUintFlagHelpOutput(t *testing.T) {
	for _, test := range uintFlagTests {
		flag := UintFlag{Name: test.name, Value: 41}
		output := flag.String()

		if output != test.expected {
			t.Errorf("%s does not match %s", output, test.expected)
		}
	}
}

func TestUintFlagWithEnvVarHelpOutput(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_BAR", "2")
	for _, test := range uintFlagTests {
		flag := UintFlag{Name: test.name, EnvVar: "APP_BAR"}
		output := flag.String()

		expectedSuffix := " [$APP_BAR]"
		if runtime.GOOS == "windows" {
			expectedSuffix = " [%APP_BAR%]"
		}
		if !strings.HasSuffix(output, expectedSuffix) {
			t.Errorf("%s does not end with"+expectedSuffix, output)
		}
	}
}

var uint64FlagTests = []struct {
	name     string
	expected string
}{
	{"gerfs", "--gerfs value\t(default: 8589934582)"},
	{"G", "-G value\t(default: 8589934582)"},
}

func TestUint64FlagHelpOutput(t *testing.T) {
	for _, test := range uint64FlagTests {
		flag := Uint64Flag{Name: test.name, Value: 8589934582}
		output := flag.String()

		if output != test.expected {
			t.Errorf("%s does not match %s", output, test.expected)
		}
	}
}

func TestUint64FlagWithEnvVarHelpOutput(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_BAR", "2")
	for _, test := range uint64FlagTests {
		flag := UintFlag{Name: test.name, EnvVar: "APP_BAR"}
		output := flag.String()

		expectedSuffix := " [$APP_BAR]"
		if runtime.GOOS == "windows" {
			expectedSuffix = " [%APP_BAR%]"
		}
		if !strings.HasSuffix(output, expectedSuffix) {
			t.Errorf("%s does not end with"+expectedSuffix, output)
		}
	}
}

var durationFlagTests = []struct {
	name     string
	expected string
}{
	{"hooting", "--hooting value\t(default: 1s)"},
	{"H", "-H value\t(default: 1s)"},
}

func TestDurationFlagHelpOutput(t *testing.T) {
	for _, test := range durationFlagTests {
		flag := DurationFlag{Name: test.name, Value: 1 * time.Second}
		output := flag.String()

		if output != test.expected {
			t.Errorf("%q does not match %q", output, test.expected)
		}
	}
}

func TestDurationFlagWithEnvVarHelpOutput(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_BAR", "2h3m6s")
	for _, test := range durationFlagTests {
		flag := DurationFlag{Name: test.name, EnvVar: "APP_BAR"}
		output := flag.String()

		expectedSuffix := " [$APP_BAR]"
		if runtime.GOOS == "windows" {
			expectedSuffix = " [%APP_BAR%]"
		}
		if !strings.HasSuffix(output, expectedSuffix) {
			t.Errorf("%s does not end with"+expectedSuffix, output)
		}
	}
}

var intSliceFlagTests = []struct {
	name     string
	value    *IntSlice
	expected string
}{
	{"heads", &IntSlice{}, "--heads value\t"},
	{"H", &IntSlice{}, "-H value\t"},
	{"H, heads", func() *IntSlice {
		i := &IntSlice{}
		i.Set("9")
		i.Set("3")
		return i
	}(), "-H value, --heads value\t(default: 9, 3)"},
}

func TestIntSliceFlagHelpOutput(t *testing.T) {
	for _, test := range intSliceFlagTests {
		flag := IntSliceFlag{Name: test.name, Value: test.value}
		output := flag.String()

		if output != test.expected {
			t.Errorf("%q does not match %q", output, test.expected)
		}
	}
}

func TestIntSliceFlagWithEnvVarHelpOutput(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_SMURF", "42,3")
	for _, test := range intSliceFlagTests {
		flag := IntSliceFlag{Name: test.name, Value: test.value, EnvVar: "APP_SMURF"}
		output := flag.String()

		expectedSuffix := " [$APP_SMURF]"
		if runtime.GOOS == "windows" {
			expectedSuffix = " [%APP_SMURF%]"
		}
		if !strings.HasSuffix(output, expectedSuffix) {
			t.Errorf("%q does not end with"+expectedSuffix, output)
		}
	}
}

var int64SliceFlagTests = []struct {
	name     string
	value    *Int64Slice
	expected string
}{
	{"heads", &Int64Slice{}, "--heads value\t"},
	{"H", &Int64Slice{}, "-H value\t"},
	{"H, heads", func() *Int64Slice {
		i := &Int64Slice{}
		i.Set("2")
		i.Set("17179869184")
		return i
	}(), "-H value, --heads value\t(default: 2, 17179869184)"},
}

func TestInt64SliceFlagHelpOutput(t *testing.T) {
	for _, test := range int64SliceFlagTests {
		flag := Int64SliceFlag{Name: test.name, Value: test.value}
		output := flag.String()

		if output != test.expected {
			t.Errorf("%q does not match %q", output, test.expected)
		}
	}
}

func TestInt64SliceFlagWithEnvVarHelpOutput(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_SMURF", "42,17179869184")
	for _, test := range int64SliceFlagTests {
		flag := Int64SliceFlag{Name: test.name, Value: test.value, EnvVar: "APP_SMURF"}
		output := flag.String()

		expectedSuffix := " [$APP_SMURF]"
		if runtime.GOOS == "windows" {
			expectedSuffix = " [%APP_SMURF%]"
		}
		if !strings.HasSuffix(output, expectedSuffix) {
			t.Errorf("%q does not end with"+expectedSuffix, output)
		}
	}
}

var float64FlagTests = []struct {
	name     string
	expected string
}{
	{"hooting", "--hooting value\t(default: 0.1)"},
	{"H", "-H value\t(default: 0.1)"},
}

func TestFloat64FlagHelpOutput(t *testing.T) {
	for _, test := range float64FlagTests {
		flag := Float64Flag{Name: test.name, Value: float64(0.1)}
		output := flag.String()

		if output != test.expected {
			t.Errorf("%q does not match %q", output, test.expected)
		}
	}
}

func TestFloat64FlagWithEnvVarHelpOutput(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_BAZ", "99.4")
	for _, test := range float64FlagTests {
		flag := Float64Flag{Name: test.name, EnvVar: "APP_BAZ"}
		output := flag.String()

		expectedSuffix := " [$APP_BAZ]"
		if runtime.GOOS == "windows" {
			expectedSuffix = " [%APP_BAZ%]"
		}
		if !strings.HasSuffix(output, expectedSuffix) {
			t.Errorf("%s does not end with"+expectedSuffix, output)
		}
	}
}

var genericFlagTests = []struct {
	name     string
	value    Generic
	expected string
}{
	{"toads", &Parser{"abc", "def"}, "--toads value\ttest flag (default: abc,def)"},
	{"t", &Parser{"abc", "def"}, "-t value\ttest flag (default: abc,def)"},
}

func TestGenericFlagHelpOutput(t *testing.T) {
	for _, test := range genericFlagTests {
		flag := GenericFlag{Name: test.name, Value: test.value, Usage: "test flag"}
		output := flag.String()

		if output != test.expected {
			t.Errorf("%q does not match %q", output, test.expected)
		}
	}
}

func TestGenericFlagWithEnvVarHelpOutput(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_ZAP", "3")
	for _, test := range genericFlagTests {
		flag := GenericFlag{Name: test.name, EnvVar: "APP_ZAP"}
		output := flag.String()

		expectedSuffix := " [$APP_ZAP]"
		if runtime.GOOS == "windows" {
			expectedSuffix = " [%APP_ZAP%]"
		}
		if !strings.HasSuffix(output, expectedSuffix) {
			t.Errorf("%s does not end with"+expectedSuffix, output)
		}
	}
}

func TestParseMultiString(t *testing.T) {
	(&App{
		Flags: []Flag{
			StringFlag{Name: "serve, s"},
		},
		Action: func(ctx *Context) error {
			if ctx.String("serve") != "10" {
				t.Errorf("main name not set")
			}
			if ctx.String("s") != "10" {
				t.Errorf("short name not set")
			}
			return nil
		},
	}).Run([]string{"run", "-s", "10"})
}

func TestParseDestinationString(t *testing.T) {
	var dest string
	a := App{
		Flags: []Flag{
			StringFlag{
				Name:        "dest",
				Destination: &dest,
			},
		},
		Action: func(ctx *Context) error {
			if dest != "10" {
				t.Errorf("expected destination String 10")
			}
			return nil
		},
	}
	a.Run([]string{"run", "--dest", "10"})
}

func TestParseMultiStringFromEnv(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_COUNT", "20")
	(&App{
		Flags: []Flag{
			StringFlag{Name: "count, c", EnvVar: "APP_COUNT"},
		},
		Action: func(ctx *Context) error {
			if ctx.String("count") != "20" {
				t.Errorf("main name not set")
			}
			if ctx.String("c") != "20" {
				t.Errorf("short name not set")
			}
			return nil
		},
	}).Run([]string{"run"})
}

func TestParseMultiStringFromEnvCascade(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_COUNT", "20")
	(&App{
		Flags: []Flag{
			StringFlag{Name: "count, c", EnvVar: "COMPAT_COUNT,APP_COUNT"},
		},
		Action: func(ctx *Context) error {
			if ctx.String("count") != "20" {
				t.Errorf("main name not set")
			}
			if ctx.String("c") != "20" {
				t.Errorf("short name not set")
			}
			return nil
		},
	}).Run([]string{"run"})
}

func TestParseMultiStringSlice(t *testing.T) {
	(&App{
		Flags: []Flag{
			StringSliceFlag{Name: "serve, s", Value: &StringSlice{}},
		},
		Action: func(ctx *Context) error {
			if !reflect.DeepEqual(ctx.StringSlice("serve"), []string{"10", "20"}) {
				t.Errorf("main name not set")
			}
			if !reflect.DeepEqual(ctx.StringSlice("s"), []string{"10", "20"}) {
				t.Errorf("short name not set")
			}
			return nil
		},
	}).Run([]string{"run", "-s", "10", "-s", "20"})
}

func TestParseMultiStringSliceFromEnv(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_INTERVALS", "20,30,40")

	(&App{
		Flags: []Flag{
			StringSliceFlag{Name: "intervals, i", Value: &StringSlice{}, EnvVar: "APP_INTERVALS"},
		},
		Action: func(ctx *Context) error {
			if !reflect.DeepEqual(ctx.StringSlice("intervals"), []string{"20", "30", "40"}) {
				t.Errorf("main name not set from env")
			}
			if !reflect.DeepEqual(ctx.StringSlice("i"), []string{"20", "30", "40"}) {
				t.Errorf("short name not set from env")
			}
			return nil
		},
	}).Run([]string{"run"})
}

func TestParseMultiStringSliceFromEnvCascade(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_INTERVALS", "20,30,40")

	(&App{
		Flags: []Flag{
			StringSliceFlag{Name: "intervals, i", Value: &StringSlice{}, EnvVar: "COMPAT_INTERVALS,APP_INTERVALS"},
		},
		Action: func(ctx *Context) error {
			if !reflect.DeepEqual(ctx.StringSlice("intervals"), []string{"20", "30", "40"}) {
				t.Errorf("main name not set from env")
			}
			if !reflect.DeepEqual(ctx.StringSlice("i"), []string{"20", "30", "40"}) {
				t.Errorf("short name not set from env")
			}
			return nil
		},
	}).Run([]string{"run"})
}

func TestParseMultiInt(t *testing.T) {
	a := App{
		Flags: []Flag{
			IntFlag{Name: "serve, s"},
		},
		Action: func(ctx *Context) error {
			if ctx.Int("serve") != 10 {
				t.Errorf("main name not set")
			}
			if ctx.Int("s") != 10 {
				t.Errorf("short name not set")
			}
			return nil
		},
	}
	a.Run([]string{"run", "-s", "10"})
}

func TestParseDestinationInt(t *testing.T) {
	var dest int
	a := App{
		Flags: []Flag{
			IntFlag{
				Name:        "dest",
				Destination: &dest,
			},
		},
		Action: func(ctx *Context) error {
			if dest != 10 {
				t.Errorf("expected destination Int 10")
			}
			return nil
		},
	}
	a.Run([]string{"run", "--dest", "10"})
}

func TestParseMultiIntFromEnv(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_TIMEOUT_SECONDS", "10")
	a := App{
		Flags: []Flag{
			IntFlag{Name: "timeout, t", EnvVar: "APP_TIMEOUT_SECONDS"},
		},
		Action: func(ctx *Context) error {
			if ctx.Int("timeout") != 10 {
				t.Errorf("main name not set")
			}
			if ctx.Int("t") != 10 {
				t.Errorf("short name not set")
			}
			return nil
		},
	}
	a.Run([]string{"run"})
}

func TestParseMultiIntFromEnvCascade(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_TIMEOUT_SECONDS", "10")
	a := App{
		Flags: []Flag{
			IntFlag{Name: "timeout, t", EnvVar: "COMPAT_TIMEOUT_SECONDS,APP_TIMEOUT_SECONDS"},
		},
		Action: func(ctx *Context) error {
			if ctx.Int("timeout") != 10 {
				t.Errorf("main name not set")
			}
			if ctx.Int("t") != 10 {
				t.Errorf("short name not set")
			}
			return nil
		},
	}
	a.Run([]string{"run"})
}

func TestParseMultiIntSlice(t *testing.T) {
	(&App{
		Flags: []Flag{
			IntSliceFlag{Name: "serve, s", Value: &IntSlice{}},
		},
		Action: func(ctx *Context) error {
			if !reflect.DeepEqual(ctx.IntSlice("serve"), []int{10, 20}) {
				t.Errorf("main name not set")
			}
			if !reflect.DeepEqual(ctx.IntSlice("s"), []int{10, 20}) {
				t.Errorf("short name not set")
			}
			return nil
		},
	}).Run([]string{"run", "-s", "10", "-s", "20"})
}

func TestParseMultiIntSliceFromEnv(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_INTERVALS", "20,30,40")

	(&App{
		Flags: []Flag{
			IntSliceFlag{Name: "intervals, i", Value: &IntSlice{}, EnvVar: "APP_INTERVALS"},
		},
		Action: func(ctx *Context) error {
			if !reflect.DeepEqual(ctx.IntSlice("intervals"), []int{20, 30, 40}) {
				t.Errorf("main name not set from env")
			}
			if !reflect.DeepEqual(ctx.IntSlice("i"), []int{20, 30, 40}) {
				t.Errorf("short name not set from env")
			}
			return nil
		},
	}).Run([]string{"run"})
}

func TestParseMultiIntSliceFromEnvCascade(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_INTERVALS", "20,30,40")

	(&App{
		Flags: []Flag{
			IntSliceFlag{Name: "intervals, i", Value: &IntSlice{}, EnvVar: "COMPAT_INTERVALS,APP_INTERVALS"},
		},
		Action: func(ctx *Context) error {
			if !reflect.DeepEqual(ctx.IntSlice("intervals"), []int{20, 30, 40}) {
				t.Errorf("main name not set from env")
			}
			if !reflect.DeepEqual(ctx.IntSlice("i"), []int{20, 30, 40}) {
				t.Errorf("short name not set from env")
			}
			return nil
		},
	}).Run([]string{"run"})
}

func TestParseMultiInt64Slice(t *testing.T) {
	(&App{
		Flags: []Flag{
			Int64SliceFlag{Name: "serve, s", Value: &Int64Slice{}},
		},
		Action: func(ctx *Context) error {
			if !reflect.DeepEqual(ctx.Int64Slice("serve"), []int64{10, 17179869184}) {
				t.Errorf("main name not set")
			}
			if !reflect.DeepEqual(ctx.Int64Slice("s"), []int64{10, 17179869184}) {
				t.Errorf("short name not set")
			}
			return nil
		},
	}).Run([]string{"run", "-s", "10", "-s", "17179869184"})
}

func TestParseMultiInt64SliceFromEnv(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_INTERVALS", "20,30,17179869184")

	(&App{
		Flags: []Flag{
			Int64SliceFlag{Name: "intervals, i", Value: &Int64Slice{}, EnvVar: "APP_INTERVALS"},
		},
		Action: func(ctx *Context) error {
			if !reflect.DeepEqual(ctx.Int64Slice("intervals"), []int64{20, 30, 17179869184}) {
				t.Errorf("main name not set from env")
			}
			if !reflect.DeepEqual(ctx.Int64Slice("i"), []int64{20, 30, 17179869184}) {
				t.Errorf("short name not set from env")
			}
			return nil
		},
	}).Run([]string{"run"})
}

func TestParseMultiInt64SliceFromEnvCascade(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_INTERVALS", "20,30,17179869184")

	(&App{
		Flags: []Flag{
			Int64SliceFlag{Name: "intervals, i", Value: &Int64Slice{}, EnvVar: "COMPAT_INTERVALS,APP_INTERVALS"},
		},
		Action: func(ctx *Context) error {
			if !reflect.DeepEqual(ctx.Int64Slice("intervals"), []int64{20, 30, 17179869184}) {
				t.Errorf("main name not set from env")
			}
			if !reflect.DeepEqual(ctx.Int64Slice("i"), []int64{20, 30, 17179869184}) {
				t.Errorf("short name not set from env")
			}
			return nil
		},
	}).Run([]string{"run"})
}

func TestParseMultiFloat64(t *testing.T) {
	a := App{
		Flags: []Flag{
			Float64Flag{Name: "serve, s"},
		},
		Action: func(ctx *Context) error {
			if ctx.Float64("serve") != 10.2 {
				t.Errorf("main name not set")
			}
			if ctx.Float64("s") != 10.2 {
				t.Errorf("short name not set")
			}
			return nil
		},
	}
	a.Run([]string{"run", "-s", "10.2"})
}

func TestParseDestinationFloat64(t *testing.T) {
	var dest float64
	a := App{
		Flags: []Flag{
			Float64Flag{
				Name:        "dest",
				Destination: &dest,
			},
		},
		Action: func(ctx *Context) error {
			if dest != 10.2 {
				t.Errorf("expected destination Float64 10.2")
			}
			return nil
		},
	}
	a.Run([]string{"run", "--dest", "10.2"})
}

func TestParseMultiFloat64FromEnv(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_TIMEOUT_SECONDS", "15.5")
	a := App{
		Flags: []Flag{
			Float64Flag{Name: "timeout, t", EnvVar: "APP_TIMEOUT_SECONDS"},
		},
		Action: func(ctx *Context) error {
			if ctx.Float64("timeout") != 15.5 {
				t.Errorf("main name not set")
			}
			if ctx.Float64("t") != 15.5 {
				t.Errorf("short name not set")
			}
			return nil
		},
	}
	a.Run([]string{"run"})
}

func TestParseMultiFloat64FromEnvCascade(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_TIMEOUT_SECONDS", "15.5")
	a := App{
		Flags: []Flag{
			Float64Flag{Name: "timeout, t", EnvVar: "COMPAT_TIMEOUT_SECONDS,APP_TIMEOUT_SECONDS"},
		},
		Action: func(ctx *Context) error {
			if ctx.Float64("timeout") != 15.5 {
				t.Errorf("main name not set")
			}
			if ctx.Float64("t") != 15.5 {
				t.Errorf("short name not set")
			}
			return nil
		},
	}
	a.Run([]string{"run"})
}

func TestParseMultiBool(t *testing.T) {
	a := App{
		Flags: []Flag{
			BoolFlag{Name: "serve, s"},
		},
		Action: func(ctx *Context) error {
			if ctx.Bool("serve") != true {
				t.Errorf("main name not set")
			}
			if ctx.Bool("s") != true {
				t.Errorf("short name not set")
			}
			return nil
		},
	}
	a.Run([]string{"run", "--serve"})
}

func TestParseDestinationBool(t *testing.T) {
	var dest bool
	a := App{
		Flags: []Flag{
			BoolFlag{
				Name:        "dest",
				Destination: &dest,
			},
		},
		Action: func(ctx *Context) error {
			if dest != true {
				t.Errorf("expected destination Bool true")
			}
			return nil
		},
	}
	a.Run([]string{"run", "--dest"})
}

func TestParseMultiBoolFromEnv(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_DEBUG", "1")
	a := App{
		Flags: []Flag{
			BoolFlag{Name: "debug, d", EnvVar: "APP_DEBUG"},
		},
		Action: func(ctx *Context) error {
			if ctx.Bool("debug") != true {
				t.Errorf("main name not set from env")
			}
			if ctx.Bool("d") != true {
				t.Errorf("short name not set from env")
			}
			return nil
		},
	}
	a.Run([]string{"run"})
}

func TestParseMultiBoolFromEnvCascade(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_DEBUG", "1")
	a := App{
		Flags: []Flag{
			BoolFlag{Name: "debug, d", EnvVar: "COMPAT_DEBUG,APP_DEBUG"},
		},
		Action: func(ctx *Context) error {
			if ctx.Bool("debug") != true {
				t.Errorf("main name not set from env")
			}
			if ctx.Bool("d") != true {
				t.Errorf("short name not set from env")
			}
			return nil
		},
	}
	a.Run([]string{"run"})
}

func TestParseMultiBoolT(t *testing.T) {
	a := App{
		Flags: []Flag{
			BoolTFlag{Name: "serve, s"},
		},
		Action: func(ctx *Context) error {
			if ctx.BoolT("serve") != true {
				t.Errorf("main name not set")
			}
			if ctx.BoolT("s") != true {
				t.Errorf("short name not set")
			}
			return nil
		},
	}
	a.Run([]string{"run", "--serve"})
}

func TestParseDestinationBoolT(t *testing.T) {
	var dest bool
	a := App{
		Flags: []Flag{
			BoolTFlag{
				Name:        "dest",
				Destination: &dest,
			},
		},
		Action: func(ctx *Context) error {
			if dest != true {
				t.Errorf("expected destination BoolT true")
			}
			return nil
		},
	}
	a.Run([]string{"run", "--dest"})
}

func TestParseMultiBoolTFromEnv(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_DEBUG", "0")
	a := App{
		Flags: []Flag{
			BoolTFlag{Name: "debug, d", EnvVar: "APP_DEBUG"},
		},
		Action: func(ctx *Context) error {
			if ctx.BoolT("debug") != false {
				t.Errorf("main name not set from env")
			}
			if ctx.BoolT("d") != false {
				t.Errorf("short name not set from env")
			}
			return nil
		},
	}
	a.Run([]string{"run"})
}

func TestParseMultiBoolTFromEnvCascade(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_DEBUG", "0")
	a := App{
		Flags: []Flag{
			BoolTFlag{Name: "debug, d", EnvVar: "COMPAT_DEBUG,APP_DEBUG"},
		},
		Action: func(ctx *Context) error {
			if ctx.BoolT("debug") != false {
				t.Errorf("main name not set from env")
			}
			if ctx.BoolT("d") != false {
				t.Errorf("short name not set from env")
			}
			return nil
		},
	}
	a.Run([]string{"run"})
}

type Parser [2]string

func (p *Parser) Set(value string) error {
	parts := strings.Split(value, ",")
	if len(parts) != 2 {
		return fmt.Errorf("invalid format")
	}

	(*p)[0] = parts[0]
	(*p)[1] = parts[1]

	return nil
}

func (p *Parser) String() string {
	return fmt.Sprintf("%s,%s", p[0], p[1])
}

func TestParseGeneric(t *testing.T) {
	a := App{
		Flags: []Flag{
			GenericFlag{Name: "serve, s", Value: &Parser{}},
		},
		Action: func(ctx *Context) error {
			if !reflect.DeepEqual(ctx.Generic("serve"), &Parser{"10", "20"}) {
				t.Errorf("main name not set")
			}
			if !reflect.DeepEqual(ctx.Generic("s"), &Parser{"10", "20"}) {
				t.Errorf("short name not set")
			}
			return nil
		},
	}
	a.Run([]string{"run", "-s", "10,20"})
}

func TestParseGenericFromEnv(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_SERVE", "20,30")
	a := App{
		Flags: []Flag{
			GenericFlag{Name: "serve, s", Value: &Parser{}, EnvVar: "APP_SERVE"},
		},
		Action: func(ctx *Context) error {
			if !reflect.DeepEqual(ctx.Generic("serve"), &Parser{"20", "30"}) {
				t.Errorf("main name not set from env")
			}
			if !reflect.DeepEqual(ctx.Generic("s"), &Parser{"20", "30"}) {
				t.Errorf("short name not set from env")
			}
			return nil
		},
	}
	a.Run([]string{"run"})
}

func TestParseGenericFromEnvCascade(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_FOO", "99,2000")
	a := App{
		Flags: []Flag{
			GenericFlag{Name: "foos", Value: &Parser{}, EnvVar: "COMPAT_FOO,APP_FOO"},
		},
		Action: func(ctx *Context) error {
			if !reflect.DeepEqual(ctx.Generic("foos"), &Parser{"99", "2000"}) {
				t.Errorf("value not set from env")
			}
			return nil
		},
	}
	a.Run([]string{"run"})
}
