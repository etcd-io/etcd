// Package gomoddirectives a linter that handle directives into `go.mod`.
package gomoddirectives

import (
	"context"
	"fmt"
	"go/token"
	"regexp"
	"slices"
	"strings"

	"github.com/ldez/grignotin/gomod"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
	"golang.org/x/tools/go/analysis"
)

const (
	reasonExclude          = "exclude directive is not allowed"
	reasonGoDebug          = "godebug directive is not allowed"
	reasonGoVersion        = "go directive (%s) doesn't match the pattern '%s'"
	reasonIgnore           = "ignore directive is not allowed"
	reasonReplace          = "replacement are not allowed"
	reasonReplaceDuplicate = "multiple replacement of the same module"
	reasonReplaceIdentical = "the original module and the replacement are identical"
	reasonReplaceLocal     = "local replacement are not allowed"
	reasonRetract          = "a comment is mandatory to explain why the version has been retracted"
	reasonTool             = "tool directive is not allowed"
	reasonToolchain        = "toolchain directive is not allowed"
	reasonToolchainPattern = "toolchain directive (%s) doesn't match the pattern '%s'"
)

// Result the analysis result.
type Result struct {
	Reason string
	Start  token.Position
	End    token.Position
}

// NewResult creates a new Result.
func NewResult(file *modfile.File, line *modfile.Line, reason string) Result {
	return Result{
		Start:  token.Position{Filename: file.Syntax.Name, Line: line.Start.Line, Column: line.Start.LineRune},
		End:    token.Position{Filename: file.Syntax.Name, Line: line.End.Line, Column: line.End.LineRune},
		Reason: reason,
	}
}

func (r Result) String() string {
	return fmt.Sprintf("%s: %s", r.Start, r.Reason)
}

// Options the analyzer options.
type Options struct {
	ReplaceAllowList          []string
	ReplaceAllowLocal         bool
	ExcludeForbidden          bool
	IgnoreForbidden           bool
	RetractAllowNoExplanation bool
	ToolchainForbidden        bool
	ToolchainPattern          *regexp.Regexp
	ToolForbidden             bool
	GoDebugForbidden          bool
	GoVersionPattern          *regexp.Regexp
	CheckModulePath           bool
}

// AnalyzePass analyzes a pass.
func AnalyzePass(pass *analysis.Pass, opts Options) ([]Result, error) {
	info, err := gomod.GetModuleInfo(context.Background())
	if err != nil {
		return nil, fmt.Errorf("get information about modules: %w", err)
	}

	goMod := info[0].GoMod

	if pass.Module != nil && pass.Module.Path != "" {
		for _, m := range info {
			if m.Path == pass.Module.Path {
				goMod = m.GoMod
				break
			}
		}
	}

	f, err := parseGoMod(goMod)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", goMod, err)
	}

	return AnalyzeFile(f, opts), nil
}

// Analyze analyzes a project.
func Analyze(opts Options) ([]Result, error) {
	f, err := GetModuleFile()
	if err != nil {
		return nil, fmt.Errorf("failed to get module file: %w", err)
	}

	return AnalyzeFile(f, opts), nil
}

// AnalyzeFile analyzes a mod file.
func AnalyzeFile(file *modfile.File, opts Options) []Result {
	checks := []func(file *modfile.File, opts Options) []Result{
		checkModulePath,
		checkRetractDirectives,
		checkExcludeDirectives,
		checkToolDirectives,
		checkIgnoreDirectives,
		checkReplaceDirectives,
		checkToolchainDirective,
		checkGoDebugDirectives,
		checkGoVersionDirectives,
	}

	var results []Result
	for _, check := range checks {
		results = append(results, check(file, opts)...)
	}

	return results
}

func checkModulePath(file *modfile.File, opts Options) []Result {
	if file.Module == nil || !opts.CheckModulePath {
		return nil
	}

	err := module.CheckPath(file.Module.Mod.Path)
	if err != nil {
		return []Result{NewResult(file, file.Module.Syntax, err.Error())}
	}

	return nil
}

func checkGoVersionDirectives(file *modfile.File, opts Options) []Result {
	if file == nil || file.Go == nil || opts.GoVersionPattern == nil || opts.GoVersionPattern.MatchString(file.Go.Version) {
		return nil
	}

	return []Result{NewResult(file, file.Go.Syntax, fmt.Sprintf(reasonGoVersion, file.Go.Version, opts.GoVersionPattern.String()))}
}

func checkToolchainDirective(file *modfile.File, opts Options) []Result {
	if file.Toolchain == nil {
		return nil
	}

	if opts.ToolchainForbidden {
		return []Result{NewResult(file, file.Toolchain.Syntax, reasonToolchain)}
	}

	if opts.ToolchainPattern == nil {
		return nil
	}

	if !opts.ToolchainPattern.MatchString(file.Toolchain.Name) {
		return []Result{NewResult(file, file.Toolchain.Syntax, fmt.Sprintf(reasonToolchainPattern, file.Toolchain.Name, opts.ToolchainPattern.String()))}
	}

	return nil
}

func checkRetractDirectives(file *modfile.File, opts Options) []Result {
	if opts.RetractAllowNoExplanation {
		return nil
	}

	var results []Result

	for _, retract := range file.Retract {
		if retract.Rationale != "" {
			continue
		}

		results = append(results, NewResult(file, retract.Syntax, reasonRetract))
	}

	return results
}

func checkExcludeDirectives(file *modfile.File, opts Options) []Result {
	if !opts.ExcludeForbidden {
		return nil
	}

	var results []Result

	for _, exclude := range file.Exclude {
		results = append(results, NewResult(file, exclude.Syntax, reasonExclude))
	}

	return results
}

func checkIgnoreDirectives(file *modfile.File, opts Options) []Result {
	if !opts.IgnoreForbidden {
		return nil
	}

	var results []Result

	for _, exclude := range file.Ignore {
		results = append(results, NewResult(file, exclude.Syntax, reasonIgnore))
	}

	return results
}

func checkToolDirectives(file *modfile.File, opts Options) []Result {
	if !opts.ToolForbidden {
		return nil
	}

	var results []Result

	for _, tool := range file.Tool {
		results = append(results, NewResult(file, tool.Syntax, reasonTool))
	}

	return results
}

func checkReplaceDirectives(file *modfile.File, opts Options) []Result {
	var results []Result

	uniqReplace := map[string]struct{}{}

	for _, replace := range file.Replace {
		reason := checkReplaceDirective(opts, replace)
		if reason != "" {
			results = append(results, NewResult(file, replace.Syntax, reason))
			continue
		}

		if replace.Old.Path == replace.New.Path && replace.Old.Version == replace.New.Version {
			results = append(results, NewResult(file, replace.Syntax, reasonReplaceIdentical))
			continue
		}

		if _, ok := uniqReplace[replace.Old.Path+replace.Old.Version]; ok {
			results = append(results, NewResult(file, replace.Syntax, reasonReplaceDuplicate))
		}

		uniqReplace[replace.Old.Path+replace.Old.Version] = struct{}{}
	}

	return results
}

func checkReplaceDirective(opts Options, r *modfile.Replace) string {
	if isLocal(r) {
		if opts.ReplaceAllowLocal {
			return ""
		}

		return fmt.Sprintf("%s: %s", reasonReplaceLocal, r.Old.Path)
	}

	if slices.Contains(opts.ReplaceAllowList, r.Old.Path) {
		return ""
	}

	return fmt.Sprintf("%s: %s", reasonReplace, r.Old.Path)
}

func checkGoDebugDirectives(file *modfile.File, opts Options) []Result {
	if !opts.GoDebugForbidden {
		return nil
	}

	var results []Result

	for _, goDebug := range file.Godebug {
		results = append(results, NewResult(file, goDebug.Syntax, reasonGoDebug))
	}

	return results
}

// Filesystem paths found in "replace" directives are represented by a path with an empty version.
// https://github.com/golang/mod/blob/bc388b264a244501debfb9caea700c6dcaff10e2/module/module.go#L122-L124
func isLocal(r *modfile.Replace) bool {
	return strings.TrimSpace(r.New.Version) == ""
}
