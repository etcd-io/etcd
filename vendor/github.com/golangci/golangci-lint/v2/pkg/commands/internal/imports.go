package internal

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"text/template"
)

const importsTemplate = `
package main

import (
{{range .Imports -}}
	_ "{{.}}"
{{end -}}
)
`

func (b Builder) updatePluginsFile() error {
	importsDest := filepath.Join(b.repo, "cmd", "golangci-lint", "plugins.go")

	info, err := os.Stat(importsDest)
	if err != nil {
		return fmt.Errorf("file %s not found: %w", importsDest, err)
	}

	source, err := generateImports(b.cfg)
	if err != nil {
		return fmt.Errorf("generate imports: %w", err)
	}

	b.log.Infof("generated imports info %s:\n%s\n", importsDest, source)

	err = os.WriteFile(filepath.Clean(importsDest), source, info.Mode())
	if err != nil {
		return fmt.Errorf("write file %s: %w", importsDest, err)
	}

	return nil
}

func generateImports(cfg *Configuration) ([]byte, error) {
	impTmpl, err := template.New("plugins.go").Parse(importsTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	var imps []string
	for _, plugin := range cfg.Plugins {
		imps = append(imps, plugin.Import)
	}

	buf := &bytes.Buffer{}

	err = impTmpl.Execute(buf, map[string]any{"Imports": imps})
	if err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}

	source, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("format source: %w", err)
	}

	return source, nil
}
