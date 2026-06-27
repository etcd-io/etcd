package gomoddirectives

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ldez/grignotin/goenv"
	"golang.org/x/mod/modfile"
)

// GetModuleFile gets module file.
func GetModuleFile() (*modfile.File, error) {
	goMod, err := goenv.GetOne(context.Background(), goenv.GOMOD)
	if err != nil {
		return nil, err
	}

	mod, err := parseGoMod(goMod)
	if err != nil {
		return nil, fmt.Errorf("failed to parse go.mod (%s): %w", goMod, err)
	}

	return mod, nil
}

func parseGoMod(goMod string) (*modfile.File, error) {
	raw, err := os.ReadFile(filepath.Clean(goMod))
	if err != nil {
		return nil, fmt.Errorf("reading go.mod file: %w", err)
	}

	return modfile.Parse("go.mod", raw, nil)
}
