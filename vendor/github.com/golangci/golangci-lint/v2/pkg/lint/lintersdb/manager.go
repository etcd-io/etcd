package lintersdb

import (
	"cmp"
	"fmt"
	"maps"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
	"github.com/golangci/golangci-lint/v2/pkg/goformatters"
	"github.com/golangci/golangci-lint/v2/pkg/lint/linter"
	"github.com/golangci/golangci-lint/v2/pkg/logutils"
)

type Builder interface {
	Build(cfg *config.Config) ([]*linter.Config, error)
}

// Manager is a type of database for all linters (internals or plugins).
// It provides methods to access to the linter sets.
type Manager struct {
	log    logutils.Log
	debugf logutils.DebugFunc

	cfg *config.Config

	linters []*linter.Config

	nameToLCs map[string][]*linter.Config
}

// NewManager creates a new Manager.
// This constructor will call the builders to build and store the linters.
func NewManager(log logutils.Log, cfg *config.Config, builders ...Builder) (*Manager, error) {
	m := &Manager{
		log:       log,
		debugf:    logutils.Debug(logutils.DebugKeyEnabledLinters),
		nameToLCs: make(map[string][]*linter.Config),
	}

	m.cfg = cfg
	if cfg == nil {
		m.cfg = config.NewDefault()
	}

	for _, builder := range builders {
		linters, err := builder.Build(m.cfg)
		if err != nil {
			return nil, fmt.Errorf("build linters: %w", err)
		}

		m.linters = append(m.linters, linters...)
	}

	for _, lc := range m.linters {
		for _, name := range lc.AllNames() {
			m.nameToLCs[name] = append(m.nameToLCs[name], lc)
		}
	}

	err := NewValidator(m).Validate(m.cfg)
	if err != nil {
		return nil, err
	}

	return m, nil
}

func (m *Manager) GetLinterConfigs(name string) []*linter.Config {
	return m.nameToLCs[name]
}

func (m *Manager) GetAllSupportedLinterConfigs() []*linter.Config {
	return m.linters
}

func (m *Manager) GetEnabledLintersMap() (map[string]*linter.Config, error) {
	enabledLinters, err := m.build()
	if err != nil {
		return nil, err
	}

	if os.Getenv(logutils.EnvTestRun) == "1" {
		m.verbosePrintLintersStatus(enabledLinters)
	}

	return enabledLinters, nil
}

// GetOptimizedLinters returns enabled linters after optimization (merging) of multiple linters into a fewer number of linters.
// E.g. some go/analysis linters can be optimized into one metalinter for data reuse and speed up.
func (m *Manager) GetOptimizedLinters() ([]*linter.Config, error) {
	resultLintersSet, err := m.build()
	if err != nil {
		return nil, err
	}

	m.verbosePrintLintersStatus(resultLintersSet)

	m.combineGoAnalysisLinters(resultLintersSet)

	// Make order of execution of linters (go/analysis metalinter and unused) stable.
	resultLinters := slices.SortedFunc(maps.Values(resultLintersSet), func(a *linter.Config, b *linter.Config) int {
		if b.Name() == linter.LastLinter {
			return -1
		}

		if a.Name() == linter.LastLinter {
			return 1
		}

		if a.DoesChangeTypes != b.DoesChangeTypes {
			// move type-changing linters to the end to optimize speed
			if b.DoesChangeTypes {
				return -1
			}
			return 1
		}

		return strings.Compare(a.Name(), b.Name())
	})

	return resultLinters, nil
}

//nolint:gocyclo // the complexity cannot be reduced.
func (m *Manager) build() (map[string]*linter.Config, error) {
	m.debugf("Linters config: %#v", m.cfg.Linters)

	resultLintersSet := map[string]*linter.Config{}

	groupName := cmp.Or(m.cfg.Linters.Default, config.GroupStandard)

	switch groupName {
	case config.GroupNone:
		// no default linters

	case config.GroupAll:
		resultLintersSet = linterConfigsToMap(m.linters)

	case config.GroupFast:
		var selected []*linter.Config
		for _, lc := range m.linters {
			if lc.IsSlowLinter() {
				continue
			}

			selected = append(selected, lc)
		}

		resultLintersSet = linterConfigsToMap(selected)

	case config.GroupStandard:
		var selected []*linter.Config
		for _, lc := range m.linters {
			if !lc.FromGroup(config.GroupStandard) {
				continue
			}

			selected = append(selected, lc)
		}

		resultLintersSet = linterConfigsToMap(selected)

	default:
		return nil, fmt.Errorf("unknown group: %s", groupName)
	}

	for _, name := range slices.Concat(m.cfg.Linters.Enable, m.cfg.Formatters.Enable) {
		for _, lc := range m.GetLinterConfigs(name) {
			// it's important to use lc.Name() nor name because name can be alias
			resultLintersSet[lc.Name()] = lc
		}
	}

	for _, name := range m.cfg.Linters.Disable {
		for _, lc := range m.GetLinterConfigs(name) {
			// it's important to use lc.Name() nor name because name can be alias
			delete(resultLintersSet, lc.Name())
		}
	}

	if m.cfg.Linters.FastOnly {
		for lc := range maps.Values(resultLintersSet) {
			if lc.IsSlowLinter() {
				// it's important to use lc.Name() nor name because name can be alias
				delete(resultLintersSet, lc.Name())
			}
		}
	}

	// typecheck is not a real linter and cannot be disabled.
	if _, ok := resultLintersSet["typecheck"]; !ok && (m.cfg == nil || !m.cfg.InternalCmdTest) {
		for _, lc := range m.GetLinterConfigs("typecheck") {
			// it's important to use lc.Name() nor name because name can be alias
			resultLintersSet[lc.Name()] = lc
		}
	}

	return resultLintersSet, nil
}

func (m *Manager) combineGoAnalysisLinters(linters map[string]*linter.Config) {
	mlConfig := &linter.Config{}

	var goanalysisLinters []*goanalysis.Linter

	for _, lc := range linters {
		lnt, ok := lc.Linter.(*goanalysis.Linter)
		if !ok {
			continue
		}

		if lnt.LoadMode() == goanalysis.LoadModeWholeProgram {
			// It's ineffective by CPU and memory to run whole-program and incremental analyzers at once.
			continue
		}

		mlConfig.LoadMode |= lc.LoadMode

		if lc.IsSlowLinter() {
			mlConfig.ConsiderSlow()
		}

		goanalysisLinters = append(goanalysisLinters, lnt)
	}

	if len(goanalysisLinters) <= 1 {
		m.debugf("Didn't combine go/analysis linters: got only %d linters", len(goanalysisLinters))
		return
	}

	for _, lnt := range goanalysisLinters {
		delete(linters, lnt.Name())
	}

	// Make order of execution of go/analysis analyzers stable.
	sort.Slice(goanalysisLinters, func(i, j int) bool {
		a, b := goanalysisLinters[i], goanalysisLinters[j]

		if b.Name() == linter.LastLinter {
			return true
		}

		if a.Name() == linter.LastLinter {
			return false
		}

		return a.Name() <= b.Name()
	})

	mlConfig.Linter = goanalysis.NewMetaLinter(goanalysisLinters)

	linters[mlConfig.Linter.Name()] = mlConfig

	m.debugf("Combined %d go/analysis linters into one metalinter", len(goanalysisLinters))
}

func (m *Manager) verbosePrintLintersStatus(lcs map[string]*linter.Config) {
	var linterNames []string
	for _, lc := range lcs {
		if lc.Internal {
			continue
		}

		linterNames = append(linterNames, lc.Name())
	}
	sort.Strings(linterNames)
	m.log.Infof("Active %d linters: %s", len(linterNames), linterNames)
}

func linterConfigsToMap(lcs []*linter.Config) map[string]*linter.Config {
	ret := map[string]*linter.Config{}
	for _, lc := range lcs {
		if lc.IsDeprecated() && lc.Deprecation.Level > linter.DeprecationWarning {
			continue
		}

		if goformatters.IsFormatter(lc.Name()) {
			continue
		}

		ret[lc.Name()] = lc
	}

	return ret
}
