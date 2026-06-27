package printers

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/logutils"
	"github.com/golangci/golangci-lint/v2/pkg/report"
	"github.com/golangci/golangci-lint/v2/pkg/result"
)

const (
	outputStdOut = "stdout"
	outputStdErr = "stderr"
)

const defaultFileMode = 0o644

type issuePrinter interface {
	Print(issues []*result.Issue) error
}

// Printer prints issues.
type Printer struct {
	cfg        *config.Formats
	reportData *report.Data
	basePath   string

	log logutils.Log

	stdOut io.Writer
	stdErr io.Writer
}

// NewPrinter creates a new Printer.
func NewPrinter(log logutils.Log, cfg *config.Formats, reportData *report.Data, basePath string) (*Printer, error) {
	if log == nil {
		return nil, errors.New("missing log argument in constructor")
	}
	if cfg == nil {
		return nil, errors.New("missing config argument in constructor")
	}
	if reportData == nil {
		return nil, errors.New("missing reportData argument in constructor")
	}

	return &Printer{
		cfg:        cfg,
		reportData: reportData,
		basePath:   basePath,
		log:        log,
		stdOut:     logutils.StdOut,
		stdErr:     logutils.StdErr,
	}, nil
}

// Print prints issues based on the formats defined.
//
//nolint:gocyclo,funlen // the complexity is related to the number of formats.
func (c *Printer) Print(issues []*result.Issue) error {
	if c.cfg.IsEmpty() {
		c.cfg.Text.Path = outputStdOut
	}

	var printers []issuePrinter

	if c.cfg.Text.Path != "" {
		w, closer, err := c.createWriter(&c.cfg.Text.SimpleFormat)
		if err != nil {
			return fmt.Errorf("can't create output for %s: %w", c.cfg.Text.Path, err)
		}

		defer closer()

		printers = append(printers, NewText(c.log, w, &c.cfg.Text))
	}

	if c.cfg.JSON.Path != "" {
		w, closer, err := c.createWriter(&c.cfg.JSON)
		if err != nil {
			return fmt.Errorf("can't create output for %s: %w", c.cfg.JSON.Path, err)
		}

		defer closer()

		printers = append(printers, NewJSON(w, c.reportData))
	}

	if c.cfg.Tab.Path != "" {
		w, closer, err := c.createWriter(&c.cfg.Tab.SimpleFormat)
		if err != nil {
			return fmt.Errorf("can't create output for %s: %w", c.cfg.Tab.Path, err)
		}

		defer closer()

		printers = append(printers, NewTab(c.log, w, &c.cfg.Tab))
	}

	if c.cfg.HTML.Path != "" {
		w, closer, err := c.createWriter(&c.cfg.HTML)
		if err != nil {
			return fmt.Errorf("can't create output for %s: %w", c.cfg.HTML.Path, err)
		}

		defer closer()

		printers = append(printers, NewHTML(w))
	}

	if c.cfg.Checkstyle.Path != "" {
		w, closer, err := c.createWriter(&c.cfg.Checkstyle)
		if err != nil {
			return fmt.Errorf("can't create output for %s: %w", c.cfg.Checkstyle.Path, err)
		}

		defer closer()

		printers = append(printers, NewCheckstyle(c.log, w))
	}

	if c.cfg.CodeClimate.Path != "" {
		w, closer, err := c.createWriter(&c.cfg.CodeClimate)
		if err != nil {
			return fmt.Errorf("can't create output for %s: %w", c.cfg.CodeClimate.Path, err)
		}

		defer closer()

		printers = append(printers, NewCodeClimate(c.log, w))
	}

	if c.cfg.JUnitXML.Path != "" {
		w, closer, err := c.createWriter(&c.cfg.JUnitXML.SimpleFormat)
		if err != nil {
			return fmt.Errorf("can't create output for %s: %w", c.cfg.JUnitXML.Path, err)
		}

		defer closer()

		printers = append(printers, NewJUnitXML(w, c.cfg.JUnitXML.Extended))
	}

	if c.cfg.TeamCity.Path != "" {
		w, closer, err := c.createWriter(&c.cfg.TeamCity)
		if err != nil {
			return fmt.Errorf("can't create output for %s: %w", c.cfg.TeamCity.Path, err)
		}

		defer closer()

		printers = append(printers, NewTeamCity(c.log, w))
	}

	if c.cfg.Sarif.Path != "" {
		w, closer, err := c.createWriter(&c.cfg.Sarif)
		if err != nil {
			return fmt.Errorf("can't create output for %s: %w", c.cfg.Sarif.Path, err)
		}

		defer closer()

		printers = append(printers, NewSarif(c.log, w))
	}

	for _, printer := range printers {
		err := printer.Print(issues)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Printer) createWriter(cfg *config.SimpleFormat) (io.Writer, func(), error) {
	if cfg.Path == "" || cfg.Path == outputStdOut {
		return c.stdOut, func() {}, nil
	}

	if cfg.Path == outputStdErr {
		return c.stdErr, func() {}, nil
	}

	if !filepath.IsAbs(cfg.Path) {
		cfg.Path = filepath.Join(c.basePath, cfg.Path)
	}

	err := os.MkdirAll(filepath.Dir(cfg.Path), os.ModePerm)
	if err != nil {
		return nil, func() {}, err
	}

	f, err := os.OpenFile(cfg.Path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, defaultFileMode)
	if err != nil {
		return nil, func() {}, err
	}

	return f, func() { _ = f.Close() }, nil
}

type severitySanitizer struct {
	allowedSeverities []string
	defaultSeverity   string

	unsupportedSeverities map[string]struct{}
}

func (s *severitySanitizer) Sanitize(severity string) string {
	if slices.Contains(s.allowedSeverities, severity) {
		return severity
	}

	if s.unsupportedSeverities == nil {
		s.unsupportedSeverities = make(map[string]struct{})
	}

	s.unsupportedSeverities[severity] = struct{}{}

	return s.defaultSeverity
}

func (s *severitySanitizer) Err() error {
	if len(s.unsupportedSeverities) == 0 {
		return nil
	}

	var names []string
	for k := range s.unsupportedSeverities {
		names = append(names, "'"+k+"'")
	}

	return fmt.Errorf("severities (%v) are not inside supported values (%v), fallback to '%s'",
		strings.Join(names, ", "), strings.Join(s.allowedSeverities, ", "), s.defaultSeverity)
}
