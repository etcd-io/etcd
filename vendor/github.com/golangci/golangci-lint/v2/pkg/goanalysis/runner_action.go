package goanalysis

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/golangci/golangci-lint/v2/internal/errorutil"
)

type actionAllocator struct {
	allocatedActions []action
	nextFreeIndex    int
}

func newActionAllocator(maxCount int) *actionAllocator {
	return &actionAllocator{
		allocatedActions: make([]action, maxCount),
		nextFreeIndex:    0,
	}
}

func (actAlloc *actionAllocator) alloc() *action {
	if actAlloc.nextFreeIndex == len(actAlloc.allocatedActions) {
		panic(fmt.Sprintf("Made too many allocations of actions: %d allowed", len(actAlloc.allocatedActions)))
	}
	act := &actAlloc.allocatedActions[actAlloc.nextFreeIndex]
	actAlloc.nextFreeIndex++
	return act
}

func (act *action) waitUntilDependingAnalyzersWorked(ctx context.Context) {
	for _, dep := range act.Deps {
		if dep.Package == act.Package {
			select {
			case <-ctx.Done():
				return
			case <-dep.analysisDoneCh:
			}
		}
	}
}

func (act *action) analyzeSafe() {
	defer func() {
		if p := recover(); p != nil {
			if !act.IsRoot {
				// This line allows to display "hidden" panic with analyzers like buildssa.
				// Some linters are dependent of sub-analyzers but when a sub-analyzer fails the linter is not aware of that,
				// this results to another panic (ex: "interface conversion: interface {} is nil, not *buildssa.SSA").
				act.runner.log.Errorf("%s: panic during analysis: %v, %s", act.Analyzer.Name, p, string(debug.Stack()))
			}

			act.Err = errorutil.NewPanicError(fmt.Sprintf("%s: package %q (isInitialPkg: %t, needAnalyzeSource: %t): %s",
				act.Analyzer.Name, act.Package.Name, act.isInitialPkg, act.needAnalyzeSource, p), debug.Stack())
		}
	}()

	act.runner.sw.TrackStage(act.Analyzer.Name, act.analyze)
}

func (act *action) markDepsForAnalyzingSource() {
	// Horizontal deps (analyzer.Requires) must be loaded from source and analyzed before analyzing
	// this action.
	for _, dep := range act.Deps {
		if dep.Package == act.Package && !dep.needAnalyzeSource {
			// Analyze source only for horizontal dependencies, e.g. from "buildssa".
			dep.needAnalyzeSource = true // can't be set in parallel
			dep.markDepsForAnalyzingSource()
		}
	}
}
