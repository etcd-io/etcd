package gomodguard

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"golang.org/x/mod/modfile"
)

const (
	goModFilename       = "go.mod"
	errReadingGoModFile = "unable to read module file %s: %w"
	errParsingGoModFile = "unable to parse module file %s: %w"
)

var (
	blockReasonInBlockedList            = "import of package `%s` is blocked because the module is in the blocked modules list."
	blockReasonHasLocalReplaceDirective = "import of package `%s` is blocked because the module has a local replace directive."

	// startsWithVersion is used to test when a string begins with the version identifier of a module,
	// after having stripped the prefix base module name. IE "github.com/foo/bar/v2/baz" => "v2/baz"
	// probably indicates that the module is actually github.com/foo/bar/v2, not github.com/foo/bar.
	startsWithVersion = regexp.MustCompile(`^v[0-9]+`)
)

// ruleIndex provides deterministic, specificity-based rule matching.
// Rules are evaluated in three tiers:
//  1. Exact match — O(1) map lookup.
//  2. Prefix match — longest matching prefix wins.
//  3. Regex match — evaluated in alphabetical key order; first match wins.
type ruleIndex struct {
	exactLookup map[string]string  // trimmed module name -> original map key
	prefixKeys  []string           // sorted by length desc, then alphabetically
	regexKeys   []string           // sorted alphabetically
	matchers    map[string]Matcher // key -> compiled matcher
}

// newRuleIndex categorises rule keys into exact, prefix, and regex tiers
// and pre-sorts the prefix and regex tiers for deterministic evaluation.
func newRuleIndex(keys []string, matchTypes map[string]MatchType, matchers map[string]Matcher) *ruleIndex {
	idx := &ruleIndex{
		exactLookup: make(map[string]string, len(keys)),
		matchers:    matchers,
	}

	for _, k := range keys {
		switch matchTypes[k] {
		case ExactMatch:
			idx.exactLookup[strings.TrimSpace(k)] = k
		case PrefixMatch:
			idx.prefixKeys = append(idx.prefixKeys, k)
		case RegexMatch:
			idx.regexKeys = append(idx.regexKeys, k)
		default:
			idx.exactLookup[strings.TrimSpace(k)] = k
		}
	}

	// Longest prefix first for most-specific match.
	slices.SortFunc(idx.prefixKeys, func(a, b string) int {
		return cmp.Compare(len(b), len(a))
	})

	// Alphabetical order for deterministic regex evaluation.
	slices.Sort(idx.regexKeys)

	return idx
}

// bestMatch returns the key of the best-matching rule for moduleName,
// following the tiered precedence: exact > longest prefix > first regex.
func (idx *ruleIndex) bestMatch(moduleName string) (string, bool) {
	trimmed := strings.TrimSpace(moduleName)

	// Tier 1: exact match (O(1))
	if key, ok := idx.exactLookup[trimmed]; ok {
		return key, true
	}

	// Tier 2: longest prefix match
	for _, key := range idx.prefixKeys {
		if idx.matchers[key].Match(moduleName) {
			return key, true
		}
	}

	// Tier 3: first regex match (alphabetical order)
	for _, key := range idx.regexKeys {
		if idx.matchers[key].Match(moduleName) {
			return key, true
		}
	}

	return "", false
}

// Configuration of gomodguard allow and block lists.
type Configuration struct {
	Allowed                Allowed `yaml:"allowed"`
	Blocked                Blocked `yaml:"blocked"`
	LocalReplaceDirectives bool    `yaml:"local_replace_directives"`
}

// InitMatchers initializes matchers for the configuration rules.
func (c *Configuration) InitMatchers() error {
	for i := range c.Allowed {
		m, err := compileMatcher(c.Allowed[i].MatchType, c.Allowed[i].Module)
		if err != nil {
			return fmt.Errorf("failed compiling allowed matcher for '%s': %w", c.Allowed[i].Module, err)
		}

		c.Allowed[i].Matcher = m
	}

	for i := range c.Blocked {
		m, err := compileMatcher(c.Blocked[i].MatchType, c.Blocked[i].Module)
		if err != nil {
			return fmt.Errorf("failed compiling blocked matcher for '%s': %w", c.Blocked[i].Module, err)
		}

		c.Blocked[i].Matcher = m
	}

	return nil
}

// Processor processes Go files.
type Processor struct {
	Config                    *Configuration
	Modfile                   *modfile.File
	blockedModulesFromModFile map[string][]string
}

// NewProcessor will create a Processor to lint blocked packages.
func NewProcessor(config *Configuration) (*Processor, error) {
	goModFileBytes, err := loadGoModFile()
	if err != nil {
		return nil, fmt.Errorf(errReadingGoModFile, goModFilename, err)
	}

	modFile, err := modfile.Parse(goModFilename, goModFileBytes, nil)
	if err != nil {
		return nil, fmt.Errorf(errParsingGoModFile, goModFilename, err)
	}

	if err := config.InitMatchers(); err != nil {
		return nil, err
	}

	p := &Processor{
		Config:  config,
		Modfile: modFile,
	}

	p.SetBlockedModules()

	return p, nil
}

// ProcessFiles takes a string slice with file names (full paths)
// and lints them.
func (p *Processor) ProcessFiles(filenames []string) (issues []Issue) {
	for _, filename := range filenames {
		data, err := os.ReadFile(filepath.Clean(filename))
		if err != nil {
			issues = append(issues, Issue{
				FileName:   filename,
				LineNumber: 0,
				Reason:     fmt.Sprintf("unable to read file, file cannot be linted (%s)", err.Error()),
			})

			continue
		}

		issues = append(issues, p.process(filename, data)...)
	}

	return issues
}

// SetBlockedModules determines and sets which modules are blocked by reading
// the go.mod file of the current module.
//
// It works by iterating over the required modules specified in the require
// directive, checking if the module prefix or full name is in the allowed list.
//
// Rules are evaluated using a layered strategy for deterministic results:
//  1. Exact match — O(1) lookup; wins immediately.
//  2. Prefix match — longest matching prefix wins.
//  3. Regex match — evaluated in alphabetical key order; first match wins.
func (p *Processor) SetBlockedModules() { //nolint:gocognit // Ack this is a long func.
	blockedModules := make(map[string][]string, len(p.Modfile.Require))
	currentModuleName := p.Modfile.Module.Mod.Path
	requiredModules := p.Modfile.Require

	// Build tiered rule indices for blocked and allowed rules.
	blockedIdx, blockedLookup := buildRuleIndex(
		p.Config.Blocked,
		func(r BlockedModule) string    { return r.Module },
		func(r BlockedModule) MatchType { return r.MatchType },
		func(r BlockedModule) Matcher   { return r.Matcher },
	)
	allowedIdx, allowedLookup := buildRuleIndex(
		p.Config.Allowed,
		func(r AllowedModule) string    { return r.Module },
		func(r AllowedModule) MatchType { return r.MatchType },
		func(r AllowedModule) Matcher   { return r.Matcher },
	)

	for i := range requiredModules {
		requiredModuleName := strings.TrimSpace(requiredModules[i].Mod.Path)
		requiredModuleVersion := strings.TrimSpace(requiredModules[i].Mod.Version)

		var matchedBlockRule *BlockedModule

		// Check against blocked rules first (exact > longest prefix > first regex)
		if key, ok := blockedIdx.bestMatch(requiredModuleName); ok {
			rule := blockedLookup[key] // copy
			matchedBlockRule = &rule
		}

		if matchedBlockRule != nil && matchedBlockRule.IsCurrentModuleARecommendation(currentModuleName) {
			// The current module is a recommended alternative for this blocked module, allowing it.
			matchedBlockRule = nil
		}

		if matchedBlockRule != nil {
			isVersBlocked, err := matchedBlockRule.CheckVersion(requiredModuleVersion)
			if err != nil {
				// NOTE: Unreachable via real go.mod files; modfile.Parse rejects invalid versions
				// earlier. Left untested by design as this branch cannot be triggered.
				blockedModules[requiredModuleName] = append(blockedModules[requiredModuleName],
					fmt.Sprintf("%s unable to parse version `%s`: %s",
						blockReasonInBlockedList, requiredModuleVersion, err,
					),
				)

				continue
			}

			if !isVersBlocked {
				// Doesn't match the blocked version constraint, so we let it pass the block check
				matchedBlockRule = nil
			}
		}

		// If it's blocked, record it and move to next
		if matchedBlockRule != nil {
			blockedModules[requiredModuleName] = append(blockedModules[requiredModuleName],
				fmt.Sprintf("%s %s", blockReasonInBlockedList,
					matchedBlockRule.BlockReason(requiredModuleVersion),
				),
			)

			continue
		}

		// If no allowed list is specified, default mapping is to allow all
		if len(p.Config.Allowed) == 0 {
			continue
		}

		isAllowed := false

		var matchedButWrongVersion *AllowedModule

		if key, ok := allowedIdx.bestMatch(requiredModuleName); ok {
			rule := allowedLookup[key] // copy

			ok, err := rule.CheckVersion(requiredModuleVersion)

			switch {
			case err != nil:
				// NOTE: Unreachable via real go.mod files; modfile.Parse rejects invalid versions
				// earlier. Left untested by design as this branch cannot be triggered.
				blockedModules[requiredModuleName] = append(blockedModules[requiredModuleName],
					fmt.Sprintf("import of package `%%s` is blocked because the module version `%s` could not be parsed: %s",
						requiredModuleVersion, err,
					),
				)

				isAllowed = true // skip the generic "not allowed" message below
			case ok:
				isAllowed = true
			default:
				matchedButWrongVersion = &rule
			}
		}

		if !isAllowed {
			blockedModules[requiredModuleName] = append(blockedModules[requiredModuleName],
				fmt.Sprintf("import of package `%%s` is blocked because %s", matchedButWrongVersion.NotAllowedReason(requiredModuleVersion)))
		}
	}

	// Blocks local 'replace' directives to prevent committing dev overrides.
	// Legitimate sibling modules in multi-module repos (sharing the same
	// module name) are exempt.
	if p.Config.LocalReplaceDirectives {
		for _, r := range p.Modfile.Replace {
			if isBlockedLocalReplace(r) {
				blockedModules[r.Old.Path] = append(blockedModules[r.Old.Path],
					blockReasonHasLocalReplaceDirective,
				)
			}
		}
	}

	p.blockedModulesFromModFile = blockedModules
}

// buildRuleIndex constructs a ruleIndex and a key→rule lookup from any slice of rules.
// The three accessor functions extract the module name, match type, and compiled matcher
// from each rule, keeping this function independent of the concrete rule type.
func buildRuleIndex[R any](
	rules []R,
	moduleFn func(R) string,
	matchTypeFn func(R) MatchType,
	matcherFn func(R) Matcher,
) (*ruleIndex, map[string]R) {
	keys := make([]string, 0, len(rules))
	matchTypes := make(map[string]MatchType, len(rules))
	matchers := make(map[string]Matcher, len(rules))
	lookup := make(map[string]R, len(rules))

	for _, r := range rules {
		mod := moduleFn(r)
		keys = append(keys, mod)
		matchTypes[mod] = matchTypeFn(r)
		matchers[mod] = matcherFn(r)
		lookup[mod] = r
	}

	return newRuleIndex(keys, matchTypes, matchers), lookup
}

// process file imports and add lint error if blocked package is imported.
func (p *Processor) process(filename string, data []byte) (issues []Issue) {
	fileSet := token.NewFileSet()

	file, err := parser.ParseFile(fileSet, filename, data, parser.ParseComments)
	if err != nil {
		issues = append(issues, Issue{
			FileName:   filename,
			LineNumber: 0,
			Reason:     fmt.Sprintf("invalid syntax, file cannot be linted (%s)", err.Error()),
		})

		return
	}

	imports := file.Imports
	for n := range imports {
		importedPkg := strings.TrimSpace(strings.Trim(imports[n].Path.Value, "\""))

		blockReasons := p.isBlockedPackageFromModFile(importedPkg)
		if blockReasons == nil {
			continue
		}

		for _, blockReason := range blockReasons {
			issues = append(issues, p.addError(fileSet, imports[n].Pos(), blockReason))
		}
	}

	return issues
}

// addError adds an error for the file and line number for the current token.Pos
// with the given reason.
func (p *Processor) addError(fileset *token.FileSet, pos token.Pos, reason string) Issue {
	position := fileset.Position(pos)

	return Issue{
		FileName:   position.Filename,
		LineNumber: position.Line,
		Position:   position,
		Reason:     reason,
	}
}

// isBlockedPackageFromModFile returns the block reason if the package is blocked.
func (p *Processor) isBlockedPackageFromModFile(packageName string) []string {
	for blockedModuleName, blockReasons := range p.blockedModulesFromModFile {
		if isPackageInModule(packageName, blockedModuleName) {
			formattedReasons := make([]string, 0, len(blockReasons))

			for _, blockReason := range blockReasons {
				formattedReasons = append(formattedReasons, fmt.Sprintf(blockReason, packageName))
			}

			return formattedReasons
		}
	}

	return nil
}

// loadGoModFile loads the contents of the go.mod file in the current working directory.
// It first checks the "GOMOD" environment variable to determine the path of the go.mod file.
// If the environment variable is not set or the file does not exist, it falls back to reading the go.mod file in the current directory.
// If the "GOMOD" environment variable is set to "/dev/null", it returns an error indicating that the current working directory must have a go.mod file.
// The function returns the contents of the go.mod file as a byte slice and any error encountered during the process.
func loadGoModFile() ([]byte, error) {
	cmd := exec.Command("go", "env", "-json") //nolint:noctx // Ack at some point might use os/exec.CommandContext.
	stdout, _ := cmd.StdoutPipe()
	_ = cmd.Start()

	if stdout == nil {
		return os.ReadFile(filepath.Clean(goModFilename))
	}

	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(stdout)

	goEnv := make(map[string]string)

	err := json.Unmarshal(buf.Bytes(), &goEnv)
	if err != nil {
		return os.ReadFile(goModFilename)
	}

	if _, ok := goEnv["GOMOD"]; !ok {
		return os.ReadFile(goModFilename)
	}

	if _, err = os.Stat(goEnv["GOMOD"]); os.IsNotExist(err) {
		return os.ReadFile(goModFilename)
	}

	if goEnv["GOMOD"] == "/dev/null" || goEnv["GOMOD"] == "NUL" {
		return nil, errors.New("current working directory must have a go.mod file")
	}

	return os.ReadFile(goEnv["GOMOD"])
}

// isBlockedLocalReplace returns true if the replace directive points to a local
// filesystem path that is not a legitimate sibling module.
func isBlockedLocalReplace(r *modfile.Replace) bool {
	if r.New.Path == "" || r.New.Version != "" {
		return false
	}

	replacePath := r.New.Path
	if !filepath.IsAbs(replacePath) {
		wd, err := os.Getwd()
		if err != nil {
			wd = "."
		}

		replacePath = filepath.Join(wd, replacePath)
	}

	return !isModuleAtPath(replacePath, r.Old.Path)
}

// isModuleAtPath returns true if the directory at path contains a go.mod file
// that declares moduleName as its module, indicating a legitimate sibling module
// in a multi-module repository rather than a local development override.
func isModuleAtPath(path, moduleName string) bool {
	data, err := os.ReadFile(filepath.Clean(filepath.Join(path, goModFilename)))
	if err != nil {
		return false
	}

	mf, err := modfile.Parse(goModFilename, data, nil)
	if err != nil {
		return false
	}

	return mf.Module.Mod.Path == moduleName
}

// isPackageInModule determines if a package is a part of the specified Go module.
func isPackageInModule(pkg, mod string) bool {
	// Split pkg and mod paths into parts
	pkgPart := strings.Split(pkg, "/")
	modPart := strings.Split(mod, "/")

	pkgPartMatches := 0

	// Count number of times pkg path matches the mod path
	for i, m := range modPart {
		if len(pkgPart) > i && pkgPart[i] == m {
			pkgPartMatches++
		}
	}

	// If pkgPartMatches are not the same length as modPart
	// than the package is not in this module
	if pkgPartMatches != len(modPart) {
		return false
	}

	if len(pkgPart) > len(modPart) {
		// If pkgPart path starts with a major version
		// than the package is not in this module as
		// major versions are completely different modules
		if startsWithVersion.MatchString(pkgPart[len(modPart)]) {
			return false
		}
	}

	return true
}
