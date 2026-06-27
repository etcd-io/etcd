package gomodguard

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"golang.org/x/mod/modfile"
)

const (
	goModFilename       = "go.mod"
	errReadingGoModFile = "unable to read module file %s: %w"
	errParsingGoModFile = "unable to parse module file %s: %w"
)

var (
	blockReasonNotInAllowedList         = "import of package `%s` is blocked because the module is not in the allowed modules list."
	blockReasonInBlockedList            = "import of package `%s` is blocked because the module is in the blocked modules list."
	blockReasonHasLocalReplaceDirective = "import of package `%s` is blocked because the module has a local replace directive."
	blockReasonInvalidVersionConstraint = "import of package `%s` is blocked because the version constraint is invalid."

	// startsWithVersion is used to test when a string begins with the version identifier of a module,
	// after having stripped the prefix base module name. IE "github.com/foo/bar/v2/baz" => "v2/baz"
	// probably indicates that the module is actually github.com/foo/bar/v2, not github.com/foo/bar.
	startsWithVersion = regexp.MustCompile(`^v[0-9]+`)
)

// Configuration of gomodguard allow and block lists.
type Configuration struct {
	Allowed Allowed `yaml:"allowed"`
	Blocked Blocked `yaml:"blocked"`
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
		data, err := os.ReadFile(filename)
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

// SetBlockedModules determines and sets which modules are blocked by reading
// the go.mod file of the module that is being linted.
//
// It works by iterating over the dependant modules specified in the require
// directive, checking if the module domain or full name is in the allowed list.
func (p *Processor) SetBlockedModules() { //nolint:funlen
	blockedModules := make(map[string][]string, len(p.Modfile.Require))
	currentModuleName := p.Modfile.Module.Mod.Path
	lintedModules := p.Modfile.Require
	replacedModules := p.Modfile.Replace

	for i := range lintedModules {
		lintedModuleName := strings.TrimSpace(lintedModules[i].Mod.Path)
		lintedModuleVersion := strings.TrimSpace(lintedModules[i].Mod.Version)

		var isAllowed bool

		switch {
		case len(p.Config.Allowed.Modules) == 0 && len(p.Config.Allowed.Domains) == 0:
			isAllowed = true
		case p.Config.Allowed.IsAllowedModuleDomain(lintedModuleName):
			isAllowed = true
		case p.Config.Allowed.IsAllowedModule(lintedModuleName):
			isAllowed = true
		default:
			isAllowed = false
		}

		blockModuleReason := p.Config.Blocked.Modules.GetBlockReason(lintedModuleName)
		blockVersionReason := p.Config.Blocked.Versions.GetBlockReason(lintedModuleName)

		if !isAllowed && blockModuleReason == nil && blockVersionReason == nil {
			blockedModules[lintedModuleName] = append(blockedModules[lintedModuleName], blockReasonNotInAllowedList)
			continue
		}

		if blockModuleReason != nil && !blockModuleReason.IsCurrentModuleARecommendation(currentModuleName) {
			blockedModules[lintedModuleName] = append(blockedModules[lintedModuleName],
				fmt.Sprintf("%s %s", blockReasonInBlockedList, blockModuleReason.Message()))
		}

		if blockVersionReason != nil {
			isVersBlocked, err := blockVersionReason.IsLintedModuleVersionBlocked(lintedModuleVersion)

			var msg string

			switch err {
			case nil:
				msg = fmt.Sprintf("%s %s", blockReasonInBlockedList, blockVersionReason.Message(lintedModuleVersion))
			default:
				msg = fmt.Sprintf("%s %s", blockReasonInvalidVersionConstraint, err)
			}

			if isVersBlocked {
				blockedModules[lintedModuleName] = append(blockedModules[lintedModuleName], msg)
			}
		}
	}

	// Replace directives with local paths are blocked.
	// Filesystem paths found in "replace" directives are represented by a path with an empty version.
	// https://github.com/golang/mod/blob/bc388b264a244501debfb9caea700c6dcaff10e2/module/module.go#L122-L124
	if p.Config.Blocked.LocalReplaceDirectives {
		for i := range replacedModules {
			replacedModuleOldName := strings.TrimSpace(replacedModules[i].Old.Path)
			replacedModuleNewName := strings.TrimSpace(replacedModules[i].New.Path)
			replacedModuleNewVersion := strings.TrimSpace(replacedModules[i].New.Version)

			if replacedModuleNewName != "" && replacedModuleNewVersion == "" {
				blockedModules[replacedModuleOldName] = append(blockedModules[replacedModuleOldName],
					blockReasonHasLocalReplaceDirective)
			}
		}
	}

	p.blockedModulesFromModFile = blockedModules
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
	cmd := exec.Command("go", "env", "-json")
	stdout, _ := cmd.StdoutPipe()
	_ = cmd.Start()

	if stdout == nil {
		return os.ReadFile(goModFilename)
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
