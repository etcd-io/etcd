// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// Altered copy of https://github.com/golang/tools/blob/v0.43.0/go/analysis/checker/checker.go

package goanalysis

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"go/types"
	"os"
	"reflect"
	"time"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"

	"github.com/golangci/golangci-lint/v2/internal/x/tools/driverutil"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis/pkgerrors"
)

// NOTE(ldez) altered: custom fields; remove 'once' and 'duration'.
// An action represents one unit of analysis work: the application of
// one analysis to one package. Actions form a DAG, both within a
// package (as different analyzers are applied, either in sequence or
// parallel), and across packages (as dependencies are analyzed).
type action struct {
	Analyzer    *analysis.Analyzer
	Package     *packages.Package
	IsRoot      bool // whether this is a root node of the graph
	Deps        []*action
	Result      any   // computed result of Analyzer.run, if any (and if IsRoot)
	Err         error // error result of Analyzer.run
	Diagnostics []analysis.Diagnostic
	Duration    time.Duration // execution time of this step

	pass         *analysis.Pass
	objectFacts  map[objectFactKey]analysis.Fact
	packageFacts map[packageFactKey]analysis.Fact

	// NOTE(ldez) custom fields.
	runner              *runner
	analysisDoneCh      chan struct{}
	loadCachedFactsDone bool
	loadCachedFactsOk   bool
	isInitialPkg        bool
	needAnalyzeSource   bool
}

// NOTE(ldez) no alteration.
type objectFactKey struct {
	obj types.Object
	typ reflect.Type
}

// NOTE(ldez) no alteration.
type packageFactKey struct {
	pkg *types.Package
	typ reflect.Type
}

// NOTE(ldez) no alteration.
func (act *action) String() string {
	return fmt.Sprintf("%s@%s", act.Analyzer, act.Package)
}

// NOTE(ldez) altered version of `func (act *action) execOnce()`.
func (act *action) analyze() {
	defer close(act.analysisDoneCh) // unblock actions depending on this action

	if !act.needAnalyzeSource {
		return
	}

	// Record time spent in this node but not its dependencies.
	// In parallel mode, due to GC/scheduler contention, the
	// time is 5x higher than in sequential mode, even with a
	// semaphore limiting the number of threads here.
	// So use -debug=tp.
	t0 := time.Now()
	defer func() {
		act.Duration = time.Since(t0)
		analyzeDebugf("go/analysis: %s: %s: analyzed package %q in %s", act.runner.prefix, act.Analyzer.Name, act.Package.Name, time.Since(t0))
	}()

	// Report an error if any dependency failures.
	var depErrors error
	for _, dep := range act.Deps {
		if dep.Err != nil {
			depErrors = errors.Join(depErrors, errors.Unwrap(dep.Err))
		}
	}
	if depErrors != nil {
		act.Err = fmt.Errorf("failed prerequisites: %w", depErrors)
		return
	}

	// Plumb the output values of the dependencies
	// into the inputs of this action.  Also facts.
	inputs := make(map[*analysis.Analyzer]any)
	act.objectFacts = make(map[objectFactKey]analysis.Fact)
	act.packageFacts = make(map[packageFactKey]analysis.Fact)
	for _, dep := range act.Deps {
		if dep.Package == act.Package {
			// Same package, different analysis (horizontal edge):
			// in-memory outputs of prerequisite analyzers
			// become inputs to this analysis pass.
			inputs[dep.Analyzer] = dep.Result

		} else if dep.Analyzer == act.Analyzer { // (always true)
			// Same analysis, different package (vertical edge):
			// serialized facts produced by prerequisite analysis
			// become available to this analysis pass.
			inheritFacts(act, dep)
		}
	}

	// NOTE(ldez) this is not compatible with our implementation.
	// Quick (nonexhaustive) check that the correct go/packages mode bits were used.
	// (If there were errors, all bets are off.)
	// if pkg := act.Package; pkg.Errors == nil {
	// 	if pkg.Name == "" || pkg.PkgPath == "" || pkg.Types == nil || pkg.Fset == nil || pkg.TypesSizes == nil {
	// 		panic(fmt.Sprintf("packages must be loaded with packages.LoadSyntax mode: Name: %v, PkgPath: %v, Types: %v, Fset: %v, TypesSizes: %v",
	// 			pkg.Name == "", pkg.PkgPath == "", pkg.Types == nil, pkg.Fset == nil, pkg.TypesSizes == nil))
	// 	}
	// }

	factsDebugf("%s: Inherited facts in %s", act, time.Since(t0))

	module := &analysis.Module{} // possibly empty (non nil) in go/analysis drivers.
	if mod := act.Package.Module; mod != nil {
		module = analysisModuleFromPackagesModule(mod)
	}

	// Run the analysis.
	pass := &analysis.Pass{
		Analyzer:     act.Analyzer,
		Fset:         act.Package.Fset,
		Files:        act.Package.Syntax,
		OtherFiles:   act.Package.OtherFiles,
		IgnoredFiles: act.Package.IgnoredFiles,
		Pkg:          act.Package.Types,
		TypesInfo:    act.Package.TypesInfo,
		TypesSizes:   act.Package.TypesSizes,
		TypeErrors:   act.Package.TypeErrors,
		Module:       module,

		ResultOf:          inputs,
		Report:            func(d analysis.Diagnostic) { act.Diagnostics = append(act.Diagnostics, d) },
		ImportObjectFact:  act.ObjectFact,
		ExportObjectFact:  act.exportObjectFact,
		ImportPackageFact: act.PackageFact,
		ExportPackageFact: act.exportPackageFact,
		AllObjectFacts:    act.AllObjectFacts,
		AllPackageFacts:   act.AllPackageFacts,
	}
	pass.ReadFile = driverutil.CheckedReadFile(pass, os.ReadFile)
	act.pass = pass

	act.runner.passToPkgGuard.Lock()
	act.runner.passToPkg[pass] = act.Package
	act.runner.passToPkgGuard.Unlock()

	act.Result, act.Err = func() (any, error) {
		// NOTE(golangci-lint):
		// It looks like there should be !pass.Analyzer.RunDespiteErrors
		// but govet's cgocall crashes on it.
		// Govet itself contains !pass.Analyzer.RunDespiteErrors condition here,
		// but it exits before it if packages.Load have failed.
		if act.Package.IllTyped {
			return nil, fmt.Errorf("analysis skipped: %w", &pkgerrors.IllTypedError{Pkg: act.Package})
		}

		t1 := time.Now()

		result, err := pass.Analyzer.Run(pass)
		if err != nil {
			return nil, err
		}

		analyzedIn := time.Since(t1)
		if analyzedIn > 10*time.Millisecond {
			debugf("%s: run analyzer in %s", act, analyzedIn)
		}

		// correct result type?
		if got, want := reflect.TypeOf(result), pass.Analyzer.ResultType; got != want {
			return nil, fmt.Errorf(
				"internal error: on package %s, analyzer %s returned a result of type %v, but declared ResultType %v",
				pass.Pkg.Path(), pass.Analyzer, got, want)
		}

		// resolve diagnostic URLs
		for i := range act.Diagnostics {
			url, err := driverutil.ResolveURL(act.Analyzer, act.Diagnostics[i])
			if err != nil {
				return nil, err
			}
			act.Diagnostics[i].URL = url
		}
		return result, nil
	}()

	// Help detect (disallowed) calls after Run.
	pass.ExportObjectFact = nil
	pass.ExportPackageFact = nil

	err := act.persistFactsToCache()
	if err != nil {
		act.runner.log.Warnf("Failed to persist facts to cache: %s", err)
	}
}

// NOTE(ldez) altered: logger; sanityCheck.
// inheritFacts populates act.facts with
// those it obtains from its dependency, dep.
func inheritFacts(act, dep *action) {
	const sanityCheck = false

	for key, fact := range dep.objectFacts {
		// Filter out facts related to objects
		// that are irrelevant downstream
		// (equivalently: not in the compiler export data).
		if !exportedFrom(key.obj, dep.Package.Types) {
			factsInheritDebugf("%v: discarding %T fact from %s for %s: %s", act, fact, dep, key.obj, fact)
			continue
		}

		// Optionally serialize/deserialize fact
		// to verify that it works across address spaces.
		if sanityCheck {
			encodedFact, err := codeFact(fact)
			if err != nil {
				act.runner.log.Panicf("internal error: encoding of %T fact failed in %v: %v", fact, act, err)
			}
			fact = encodedFact
		}

		factsInheritDebugf("%v: inherited %T fact for %s: %s", act, fact, key.obj, fact)

		act.objectFacts[key] = fact
	}

	for key, fact := range dep.packageFacts {
		// TODO: filter out facts that belong to
		// packages not mentioned in the export data
		// to prevent side channels.
		//
		// The Pass.All{Object,Package}Facts accessors expose too much:
		// all facts, of all types, for all dependencies in the action
		// graph. Not only does the representation grow quadratically,
		// but it violates the separate compilation paradigm, allowing
		// analysis implementations to communicate with indirect
		// dependencies that are not mentioned in the export data.
		//
		// It's not clear how to fix this short of a rather expensive
		// filtering step after each action that enumerates all the
		// objects that would appear in export data, and deletes
		// facts associated with objects not in this set.

		// Optionally serialize/deserialize fact
		// to verify that it works across address spaces
		// and is deterministic.
		if sanityCheck {
			encodedFact, err := codeFact(fact)
			if err != nil {
				act.runner.log.Panicf("internal error: encoding of %T fact failed in %v", fact, act)
			}
			fact = encodedFact
		}

		factsInheritDebugf("%v: inherited %T fact for %s: %s", act, fact, key.pkg.Path(), fact)

		act.packageFacts[key] = fact
	}
}

// NOTE(ldez) altered: `new` is renamed to `newFact`.
// codeFact encodes then decodes a fact,
// just to exercise that logic.
func codeFact(fact analysis.Fact) (analysis.Fact, error) {
	// We encode facts one at a time.
	// A real modular driver would emit all facts
	// into one encoder to improve gob efficiency.
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(fact); err != nil {
		return nil, err
	}

	// Encode it twice and assert that we get the same bits.
	// This helps detect nondeterministic Gob encoding (e.g. of maps).
	var buf2 bytes.Buffer
	if err := gob.NewEncoder(&buf2).Encode(fact); err != nil {
		return nil, err
	}
	if !bytes.Equal(buf.Bytes(), buf2.Bytes()) {
		return nil, fmt.Errorf("encoding of %T fact is nondeterministic", fact)
	}

	newFact := reflect.New(reflect.TypeOf(fact).Elem()).Interface().(analysis.Fact)
	if err := gob.NewDecoder(&buf).Decode(newFact); err != nil {
		return nil, err
	}
	return newFact, nil
}

// NOTE(ldez) no alteration.
// exportedFrom reports whether obj may be visible to a package that imports pkg.
// This includes not just the exported members of pkg, but also unexported
// constants, types, fields, and methods, perhaps belonging to other packages,
// that find there way into the API.
// This is an overapproximation of the more accurate approach used by
// gc export data, which walks the type graph, but it's much simpler.
//
// TODO(adonovan): do more accurate filtering by walking the type graph.
func exportedFrom(obj types.Object, pkg *types.Package) bool {
	switch obj := obj.(type) {
	case *types.Func:
		return obj.Exported() && obj.Pkg() == pkg ||
			obj.Signature().Recv() != nil
	case *types.Var:
		if obj.IsField() {
			return true
		}
		// we can't filter more aggressively than this because we need
		// to consider function parameters exported, but have no way
		// of telling apart function parameters from local variables.
		return obj.Pkg() == pkg
	case *types.TypeName, *types.Const:
		return true
	}
	return false // Nil, Builtin, Label, or PkgName
}

// NOTE(ldez) altered: logger; `act.factType`.
// ObjectFact retrieves a fact associated with obj,
// and returns true if one was found.
// Given a value ptr of type *T, where *T satisfies Fact,
// ObjectFact copies the value to *ptr.
//
// See documentation at ImportObjectFact field of [analysis.Pass].
func (act *action) ObjectFact(obj types.Object, ptr analysis.Fact) bool {
	if obj == nil {
		panic("nil object")
	}
	key := objectFactKey{obj, act.factType(ptr)}
	if v, ok := act.objectFacts[key]; ok {
		reflect.ValueOf(ptr).Elem().Set(reflect.ValueOf(v).Elem())
		return true
	}
	return false
}

// NOTE(ldez) altered: logger; `act.factType`.
// exportObjectFact implements Pass.ExportObjectFact.
func (act *action) exportObjectFact(obj types.Object, fact analysis.Fact) {
	if act.pass.ExportObjectFact == nil {
		act.runner.log.Panicf("%s: Pass.ExportObjectFact(%s, %T) called after Run", act, obj, fact)
	}

	if obj.Pkg() != act.Package.Types {
		act.runner.log.Panicf("internal error: in analysis %s of package %s: Fact.Set(%s, %T): can't set facts on objects belonging another package",
			act.Analyzer, act.Package, obj, fact)
	}

	key := objectFactKey{obj, act.factType(fact)}
	act.objectFacts[key] = fact // clobber any existing entry
	if isFactsExportDebug {
		objstr := types.ObjectString(obj, (*types.Package).Name)

		factsExportDebugf("%s: object %s has fact %s\n",
			act.Package.Fset.Position(obj.Pos()), objstr, fact)
	}
}

// NOTE(ldez) no alteration.
// AllObjectFacts returns a new slice containing all object facts of
// the analysis's FactTypes in unspecified order.
//
// See documentation at AllObjectFacts field of [analysis.Pass].
func (act *action) AllObjectFacts() []analysis.ObjectFact {
	facts := make([]analysis.ObjectFact, 0, len(act.objectFacts))
	for k, fact := range act.objectFacts {
		facts = append(facts, analysis.ObjectFact{Object: k.obj, Fact: fact})
	}
	return facts
}

// NOTE(ldez) altered: `act.factType`.
// PackageFact retrieves a fact associated with package pkg,
// which must be this package or one of its dependencies.
//
// See documentation at ImportObjectFact field of [analysis.Pass].
func (act *action) PackageFact(pkg *types.Package, ptr analysis.Fact) bool {
	if pkg == nil {
		panic("nil package")
	}
	key := packageFactKey{pkg, act.factType(ptr)}
	if v, ok := act.packageFacts[key]; ok {
		reflect.ValueOf(ptr).Elem().Set(reflect.ValueOf(v).Elem())
		return true
	}
	return false
}

// NOTE(ldez) altered: logger; `act.factType`.
// exportPackageFact implements Pass.ExportPackageFact.
func (act *action) exportPackageFact(fact analysis.Fact) {
	if act.pass.ExportPackageFact == nil {
		act.runner.log.Panicf("%s: Pass.ExportPackageFact(%T) called after Run", act, fact)
	}

	key := packageFactKey{act.pass.Pkg, act.factType(fact)}
	act.packageFacts[key] = fact // clobber any existing entry

	factsDebugf("%s: package %s has fact %s\n",
		act.Package.Fset.Position(act.pass.Files[0].Pos()), act.pass.Pkg.Path(), fact)
}

// NOTE(ldez) altered: add receiver to handle logs.
func (act *action) factType(fact analysis.Fact) reflect.Type {
	t := reflect.TypeOf(fact)
	if t.Kind() != reflect.Pointer {
		act.runner.log.Fatalf("invalid Fact type: got %T, want pointer", fact)
	}
	return t
}

// NOTE(ldez) no alteration.
// AllPackageFacts returns a new slice containing all package
// facts of the analysis's FactTypes in unspecified order.
//
// See documentation at AllPackageFacts field of [analysis.Pass].
func (act *action) AllPackageFacts() []analysis.PackageFact {
	facts := make([]analysis.PackageFact, 0, len(act.packageFacts))
	for k, fact := range act.packageFacts {
		facts = append(facts, analysis.PackageFact{Package: k.pkg, Fact: fact})
	}
	return facts
}

// NOTE(ldez) no alteration.
func analysisModuleFromPackagesModule(mod *packages.Module) *analysis.Module {
	if mod == nil {
		return nil
	}

	var modErr *analysis.ModuleError
	if mod.Error != nil {
		modErr = &analysis.ModuleError{
			Err: mod.Error.Err,
		}
	}

	return &analysis.Module{
		Path:      mod.Path,
		Version:   mod.Version,
		Replace:   analysisModuleFromPackagesModule(mod.Replace),
		Time:      mod.Time,
		Main:      mod.Main,
		Indirect:  mod.Indirect,
		Dir:       mod.Dir,
		GoMod:     mod.GoMod,
		GoVersion: mod.GoVersion,
		Error:     modErr,
	}
}
