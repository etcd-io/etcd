// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package scan

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"path"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"golang.org/x/telemetry/counter"
	"golang.org/x/vuln/internal/client"
	"golang.org/x/vuln/internal/govulncheck"
	"golang.org/x/vuln/internal/openvex"
	"golang.org/x/vuln/internal/sarif"
)

// RunGovulncheck performs main govulncheck functionality and exits the
// program upon success with an appropriate exit status. Otherwise,
// returns an error.
func RunGovulncheck(ctx context.Context, env []string, r io.Reader, stdout io.Writer, stderr io.Writer, args []string) error {
	cfg := &config{env: env}
	if err := parseFlags(cfg, stderr, args); err != nil {
		return err
	}

	client, err := client.NewClient(cfg.db, nil)
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}

	prepareConfig(ctx, cfg, client)
	var handler govulncheck.Handler
	switch cfg.format {
	case formatJSON:
		handler = govulncheck.NewJSONHandler(stdout)
	case formatSarif:
		handler = sarif.NewHandler(stdout)
	case formatOpenVEX:
		handler = openvex.NewHandler(stdout)
	default:
		th := NewTextHandler(stdout)
		cfg.show.Update(th)
		handler = th
	}

	if err := handler.Config(&cfg.Config); err != nil {
		return err
	}

	if cfg.version {
		// If the -version flag is passed, exit before doing anything else. This is different than
		// passing -show which includes "version".
		return nil
	}

	incTelemetryFlagCounters(cfg)

	switch cfg.ScanMode {
	case govulncheck.ScanModeSource:
		dir := filepath.FromSlash(cfg.dir)
		err = runSource(ctx, handler, cfg, client, dir)
	case govulncheck.ScanModeBinary:
		err = runBinary(ctx, handler, cfg, client)
	case govulncheck.ScanModeExtract:
		return runExtract(cfg, stdout)
	case govulncheck.ScanModeQuery:
		err = runQuery(ctx, handler, cfg, client)
	case govulncheck.ScanModeConvert:
		err = govulncheck.HandleJSON(r, handler)
	}
	if err != nil {
		return err
	}
	return Flush(handler)
}

func prepareConfig(ctx context.Context, cfg *config, client *client.Client) {
	cfg.ProtocolVersion = govulncheck.ProtocolVersion
	cfg.DB = cfg.db
	if cfg.ScanMode == govulncheck.ScanModeSource && cfg.GoVersion == "" {
		const goverPrefix = "GOVERSION="
		for _, env := range cfg.env {
			if val := strings.TrimPrefix(env, goverPrefix); val != env {
				cfg.GoVersion = val
			}
		}
		if cfg.GoVersion == "" {
			if out, err := exec.Command("go", "env", "GOVERSION").Output(); err == nil {
				cfg.GoVersion = strings.TrimSpace(string(out))
			}
		}
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		scannerVersion(cfg, bi)
	}
	if mod, err := client.LastModifiedTime(ctx); err == nil {
		cfg.DBLastModified = &mod
	}
}

// scannerVersion reconstructs the current version of
// this binary used from the build info.
func scannerVersion(cfg *config, bi *debug.BuildInfo) {
	if bi.Path != "" {
		cfg.ScannerName = path.Base(bi.Path)
	}
	if bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		cfg.ScannerVersion = bi.Main.Version
		return
	}

	// TODO(https://go.dev/issue/29228): we need to manually construct the
	// version string when it is "(devel)" until #29228 is resolved.
	var revision, at string
	for _, s := range bi.Settings {
		if s.Key == "vcs.revision" {
			revision = s.Value
		}
		if s.Key == "vcs.time" {
			at = s.Value
		}
	}
	buf := strings.Builder{}
	buf.WriteString("v0.0.0")
	if revision != "" {
		buf.WriteString("-")
		buf.WriteString(revision[:12])
	}
	if at != "" {
		// commit time is of the form 2023-01-25T19:57:54Z
		p, err := time.Parse(time.RFC3339, at)
		if err == nil {
			buf.WriteString("-")
			buf.WriteString(p.Format("20060102150405"))
		}
	}
	cfg.ScannerVersion = buf.String()
}

func incTelemetryFlagCounters(cfg *config) {
	counter.Inc(fmt.Sprintf("govulncheck/mode:%s", cfg.ScanMode))
	counter.Inc(fmt.Sprintf("govulncheck/scan:%s", cfg.ScanLevel))
	counter.Inc(fmt.Sprintf("govulncheck/format:%s", cfg.format))

	if len(cfg.show) == 0 {
		counter.Inc("govulncheck/show:none")
	}
	for _, s := range cfg.show {
		counter.Inc(fmt.Sprintf("govulncheck/show:%s", s))
	}
}

func Flush(h govulncheck.Handler) error {
	if th, ok := h.(interface{ Flush() error }); ok {
		return th.Flush()
	}
	return nil
}
