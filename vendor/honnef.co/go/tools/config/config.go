package config

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/BurntSushi/toml"
	"golang.org/x/tools/go/analysis"
)

// Dir looks at a list of absolute file names, which should make up a
// single package, and returns the path of the directory that may
// contain a staticcheck.conf file. It returns the empty string if no
// such directory could be determined, for example because all files
// were located in Go's build cache.
func Dir(files []string) string {
	if len(files) == 0 {
		return ""
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		cache = ""
	}
	var path string
	for _, p := range files {
		// FIXME(dh): using strings.HasPrefix isn't technically
		// correct, but it should be good enough for now.
		if cache != "" && strings.HasPrefix(p, cache) {
			// File in the build cache of the standard Go build system
			continue
		}
		path = p
		break
	}

	if path == "" {
		// The package only consists of generated files.
		return ""
	}

	dir := filepath.Dir(path)
	return dir
}

func dirAST(files []*ast.File, fset *token.FileSet) string {
	names := make([]string, len(files))
	for i, f := range files {
		names[i] = fset.PositionFor(f.Pos(), true).Filename
	}
	return Dir(names)
}

var Analyzer = &analysis.Analyzer{
	Name: "config",
	Doc:  "loads configuration for the current package tree",
	Run: func(pass *analysis.Pass) (any, error) {
		dir := dirAST(pass.Files, pass.Fset)
		if dir == "" {
			cfg := DefaultConfig
			return &cfg, nil
		}
		cfg, err := Load(dir)
		if err != nil {
			return nil, fmt.Errorf("error loading staticcheck.conf: %s", err)
		}
		return &cfg, nil
	},
	RunDespiteErrors: true,
	ResultType:       reflect.TypeFor[*Config](),
}

func For(pass *analysis.Pass) *Config {
	return pass.ResultOf[Analyzer].(*Config)
}

func mergeLists(a, b []string) []string {
	out := make([]string, 0, len(a)+len(b))
	for _, el := range b {
		if el == "inherit" {
			out = append(out, a...)
		} else {
			out = append(out, el)
		}
	}

	return out
}

func normalizeList(list []string) []string {
	if len(list) > 1 {
		nlist := make([]string, 0, len(list))
		nlist = append(nlist, list[0])
		for i, el := range list[1:] {
			if el != list[i] {
				nlist = append(nlist, el)
			}
		}
		list = nlist
	}

	for _, el := range list {
		if el == "inherit" {
			// This should never happen, because the default config
			// should not use "inherit"
			panic(`unresolved "inherit"`)
		}
	}

	return list
}

func (cfg Config) Merge(ocfg Config) Config {
	if ocfg.Checks != nil {
		cfg.Checks = mergeLists(cfg.Checks, ocfg.Checks)
	}
	if ocfg.Initialisms != nil {
		cfg.Initialisms = mergeLists(cfg.Initialisms, ocfg.Initialisms)
	}
	if ocfg.DotImportWhitelist != nil {
		cfg.DotImportWhitelist = mergeLists(cfg.DotImportWhitelist, ocfg.DotImportWhitelist)
	}
	if ocfg.HTTPStatusCodeWhitelist != nil {
		cfg.HTTPStatusCodeWhitelist = mergeLists(cfg.HTTPStatusCodeWhitelist, ocfg.HTTPStatusCodeWhitelist)
	}
	return cfg
}

type Config struct {
	// TODO(dh): this implementation makes it impossible for external
	// clients to add their own checkers with configuration. At the
	// moment, we don't really care about that; we don't encourage
	// that people use this package. In the future, we may. The
	// obvious solution would be using map[string]interface{}, but
	// that's obviously subpar.

	Checks                  []string `toml:"checks"`
	Initialisms             []string `toml:"initialisms"`
	DotImportWhitelist      []string `toml:"dot_import_whitelist"`
	HTTPStatusCodeWhitelist []string `toml:"http_status_code_whitelist"`
}

func (c Config) String() string {
	buf := &bytes.Buffer{}

	fmt.Fprintf(buf, "Checks: %#v\n", c.Checks)
	fmt.Fprintf(buf, "Initialisms: %#v\n", c.Initialisms)
	fmt.Fprintf(buf, "DotImportWhitelist: %#v\n", c.DotImportWhitelist)
	fmt.Fprintf(buf, "HTTPStatusCodeWhitelist: %#v", c.HTTPStatusCodeWhitelist)

	return buf.String()
}

// DefaultConfig is the default configuration.
// Its initial value describes the majority of the default configuration,
// but the Checks field can be updated at runtime based on the analyzers being used, to disable non-default checks.
// For cmd/staticcheck, this is handled by (*lintcmd.Command).Run.
//
// Note that DefaultConfig shouldn't be modified while analyzers are executing.
var DefaultConfig = Config{
	Checks: []string{"all"},
	Initialisms: []string{
		"ACL", "API", "ASCII", "CPU", "CSS", "DNS",
		"EOF", "GUID", "HTML", "HTTP", "HTTPS", "ID",
		"IP", "JSON", "QPS", "RAM", "RPC", "SLA",
		"SMTP", "SQL", "SSH", "TCP", "TLS", "TTL",
		"UDP", "UI", "GID", "UID", "UUID", "URI",
		"URL", "UTF8", "VM", "XML", "XMPP", "XSRF",
		"XSS", "SIP", "RTP", "AMQP", "DB", "TS",
	},
	DotImportWhitelist: []string{
		"simd/archsimd",
		"github.com/mmcloughlin/avo/build",
		"github.com/mmcloughlin/avo/operand",
		"github.com/mmcloughlin/avo/reg",
	},
	HTTPStatusCodeWhitelist: []string{"200", "400", "404", "500"},
}

const ConfigName = "staticcheck.conf"

type ParseError struct {
	Filename string
	toml.ParseError
}

func parseConfigs(dir string) ([]Config, error) {
	var out []Config

	// TODO(dh): consider stopping at the GOPATH/module boundary
	for dir != "" {
		path := filepath.Join(dir, ConfigName)
		fi, err := os.Stat(path)
		if os.IsNotExist(err) || (err == nil && !fi.Mode().IsRegular()) {
			// walk up
			ndir := filepath.Dir(dir)
			if ndir == dir {
				break
			}
			dir = ndir
			continue
		}
		if err != nil {
			return nil, err
		}

		// There is a small TOCTOU window here, but we're fine with reporting an
		// error if the source tree is modified concurrently in weird ways while
		// running Staticcheck.
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}

		var cfg Config
		_, err = toml.NewDecoder(f).Decode(&cfg)
		f.Close()
		if err != nil {
			if err, ok := err.(toml.ParseError); ok {
				return nil, ParseError{
					Filename:   filepath.Join(dir, ConfigName),
					ParseError: err,
				}
			}
			return nil, err
		}
		out = append(out, cfg)
		ndir := filepath.Dir(dir)
		if ndir == dir {
			break
		}
		dir = ndir
	}
	out = append(out, DefaultConfig)
	if len(out) < 2 {
		return out, nil
	}
	for i := 0; i < len(out)/2; i++ {
		out[i], out[len(out)-1-i] = out[len(out)-1-i], out[i]
	}
	return out, nil
}

func mergeConfigs(confs []Config) Config {
	if len(confs) == 0 {
		// This shouldn't happen because we always have at least a
		// default config.
		panic("trying to merge zero configs")
	}
	if len(confs) == 1 {
		return confs[0]
	}
	conf := confs[0]
	for _, oconf := range confs[1:] {
		conf = conf.Merge(oconf)
	}
	return conf
}

func Load(dir string) (Config, error) {
	confs, err := parseConfigs(dir)
	if err != nil {
		return Config{}, err
	}
	conf := mergeConfigs(confs)

	conf.Checks = normalizeList(conf.Checks)
	conf.Initialisms = normalizeList(conf.Initialisms)
	conf.DotImportWhitelist = normalizeList(conf.DotImportWhitelist)
	conf.HTTPStatusCodeWhitelist = normalizeList(conf.HTTPStatusCodeWhitelist)

	return conf, nil
}
