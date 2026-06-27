package migrate

import (
	"github.com/golangci/golangci-lint/v2/pkg/commands/internal/migrate/ptr"
	"github.com/golangci/golangci-lint/v2/pkg/commands/internal/migrate/versionone"
	"github.com/golangci/golangci-lint/v2/pkg/commands/internal/migrate/versiontwo"
)

func toLinters(old *versionone.Config) versiontwo.Linters {
	enable, disable := ProcessEffectiveLinters(old.Linters)

	return versiontwo.Linters{
		Default:    getDefaultName(old.Linters),
		Enable:     onlyLinterNames(convertStaticcheckLinterNames(enable)),
		Disable:    onlyLinterNames(convertDisabledStaticcheckLinterNames(disable)),
		FastOnly:   nil,
		Settings:   toLinterSettings(old.LintersSettings),
		Exclusions: toExclusions(old),
	}
}

func getDefaultName(old versionone.Linters) *string {
	switch {
	case ptr.Deref(old.DisableAll):
		return ptr.Pointer("none")
	case ptr.Deref(old.EnableAll):
		return ptr.Pointer("all")
	default:
		return nil // standard is the default
	}
}
