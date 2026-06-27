package config

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	hcversion "github.com/hashicorp/go-version"
	"github.com/ldez/grignotin/goenv"
	"github.com/ldez/grignotin/gomod"
	"golang.org/x/mod/modfile"

	"github.com/golangci/golangci-lint/v2/pkg/logutils"
)

// defaultGoVersion the value should be "oldstable" - 1.
// If the current stable version is 1.24 then 1.23 - 1 = 1.22.
const defaultGoVersion = "1.22"

// Config encapsulates the config data specified in the golangci-lint YAML config file.
type Config struct {
	cfgDir   string // Path to the directory containing golangci-lint config file.
	basePath string // Path the root directory related to [Run.RelativePathMode].

	Version string `mapstructure:"version"`

	Run Run `mapstructure:"run"`

	Output Output `mapstructure:"output"`

	Linters Linters `mapstructure:"linters"`

	Issues   Issues   `mapstructure:"issues"`
	Severity Severity `mapstructure:"severity"`

	Formatters Formatters `mapstructure:"formatters"`

	InternalCmdTest bool // Option is used only for testing golangci-lint command, don't use it
	InternalTest    bool // Option is used only for testing golangci-lint code, don't use it
}

// GetConfigDir returns the directory that contains golangci-lint config file.
func (c *Config) GetConfigDir() string {
	return c.cfgDir
}

// SetConfigDir sets the path to directory that contains golangci-lint config file.
func (c *Config) SetConfigDir(dir string) {
	c.cfgDir = dir
}

func (c *Config) GetBasePath() string {
	return c.basePath
}

func (c *Config) IsInternalTest() bool {
	return c.InternalTest
}

func (c *Config) Validate() error {
	validators := []func() error{
		c.Run.Validate,
		c.Output.Validate,
		c.Linters.Validate,
		c.Formatters.Validate,
		c.Severity.Validate,
	}

	for _, v := range validators {
		if err := v(); err != nil {
			return err
		}
	}

	return nil
}

func NewDefault() *Config {
	return &Config{
		Linters: Linters{
			Settings: defaultLintersSettings,
		},
		Formatters: Formatters{
			Settings: defaultFormatterSettings,
		},
	}
}

func IsGoGreaterThanOrEqual(current, limit string) bool {
	v1, err := hcversion.NewVersion(strings.TrimPrefix(current, "go"))
	if err != nil {
		return false
	}

	l, err := hcversion.NewVersion(limit)
	if err != nil {
		return false
	}

	return v1.GreaterThanOrEqual(l)
}

func detectGoVersion(ctx context.Context, log logutils.Log) string {
	return cmp.Or(detectGoVersionFromGoMod(ctx, log), defaultGoVersion)
}

// detectGoVersionFromGoMod tries to get Go version from go.mod.
// It returns `toolchain` version if present,
// else it returns `go` version if present,
// else it returns `GOVERSION` version if present,
// else it returns empty.
func detectGoVersionFromGoMod(ctx context.Context, log logutils.Log) string {
	values, err := goenv.Get(ctx, goenv.GOMOD, goenv.GOVERSION)
	if err != nil {
		values = map[string]string{
			goenv.GOMOD: detectGoModFallback(ctx),
		}
	}

	if values[goenv.GOMOD] == "" {
		return parseGoVersion(values[goenv.GOVERSION])
	}

	file, err := parseGoMod(values[goenv.GOMOD])
	if err != nil {
		return parseGoVersion(values[goenv.GOVERSION])
	}

	if file.Module != nil {
		log.Infof("Module name %q", file.Module.Mod.Path)
	}

	// The toolchain exists only if 'toolchain' version > 'go' version.
	// If 'toolchain' version <= 'go' version, `go mod tidy` will remove 'toolchain' version from go.mod.
	if file.Toolchain != nil && file.Toolchain.Name != "" {
		return parseGoVersion(file.Toolchain.Name)
	}

	if file.Go != nil && file.Go.Version != "" {
		return file.Go.Version
	}

	return parseGoVersion(values[goenv.GOVERSION])
}

func parseGoVersion(v string) string {
	raw := strings.TrimPrefix(v, "go")

	// prerelease version (ex: go1.24rc1)
	idx := strings.IndexFunc(raw, func(r rune) bool {
		return (r < '0' || r > '9') && r != '.'
	})

	if idx != -1 {
		raw = raw[:idx]
	}

	return raw
}

func parseGoMod(goMod string) (*modfile.File, error) {
	raw, err := os.ReadFile(filepath.Clean(goMod))
	if err != nil {
		return nil, fmt.Errorf("reading go.mod file: %w", err)
	}

	return modfile.Parse("go.mod", raw, nil)
}

func detectGoModFallback(ctx context.Context) string {
	info, err := gomod.GetModuleInfo(ctx)
	if err != nil {
		return ""
	}

	wd, err := os.Getwd()
	if err != nil {
		return ""
	}

	slices.SortFunc(info, func(a, b gomod.ModInfo) int {
		return cmp.Compare(len(b.Path), len(a.Path))
	})

	goMod := info[0]
	for _, m := range info {
		if !strings.HasPrefix(wd, m.Dir) {
			continue
		}

		goMod = m

		break
	}

	return goMod.GoMod
}
