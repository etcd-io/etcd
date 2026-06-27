package checkers

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/token"
	"os"
	"sync"

	"github.com/go-critic/go-critic/checkers/rulesdata"
	"github.com/go-critic/go-critic/linter"

	"github.com/quasilyte/go-ruleguard/ruleguard"
)

//go:generate go run ./rules/precompile.go -rules ./rules/rules.go -o ./rulesdata/rulesdata.go

// cachedEngine holds a pre-initialized ruleguard engine for a specific rule group.
// The engine is created once and reused for all checker instances.
type cachedEngine struct {
	engine *ruleguard.Engine
	once   sync.Once
	err    error

	// Configuration needed to create the engine
	fset         *token.FileSet
	buildContext *build.Context
	groupName    string
	debug        bool
}

func (ce *cachedEngine) get() (*ruleguard.Engine, error) {
	ce.once.Do(func() {
		parseContext := &ruleguard.LoadContext{
			Fset: ce.fset,
			GroupFilter: func(gr *ruleguard.GoRuleGroup) bool {
				return gr.Name == ce.groupName
			},
			DebugImports: ce.debug,
			DebugPrint: func(s string) {
				fmt.Println("debug:", s)
			},
		}
		engine := ruleguard.NewEngine()
		engine.BuildContext = ce.buildContext
		ce.err = engine.LoadFromIR(parseContext, "rules/rules.go", rulesdata.PrecompiledRules)
		if ce.err == nil {
			ce.engine = engine
		}
	})
	return ce.engine, ce.err
}

func InitEmbeddedRules() error {
	filename := "rules/rules.go"

	fset := token.NewFileSet()
	var groups []ruleguard.GoRuleGroup

	var buildContext *build.Context

	ruleguardDebug := os.Getenv("GOCRITIC_RULEGUARD_DEBUG") != ""

	// First we create an Engine to parse all rules.
	// We need it to get the structured info about our rules
	// that will be used to generate checkers.
	// We introduce an extra scope in hope that rootEngine
	// will be garbage-collected after we don't need it.
	// LoadedGroups() returns a slice copy and that's all what we need.
	{
		rootEngine := ruleguard.NewEngine()
		rootEngine.InferBuildContext()
		buildContext = rootEngine.BuildContext

		loadContext := &ruleguard.LoadContext{
			Fset:         fset,
			DebugImports: ruleguardDebug,
			DebugPrint: func(s string) {
				fmt.Println("debug:", s)
			},
		}
		if err := rootEngine.LoadFromIR(loadContext, filename, rulesdata.PrecompiledRules); err != nil {
			return fmt.Errorf("load embedded ruleguard rules: %w", err)
		}
		groups = rootEngine.LoadedGroups()
	}

	// For every rules group we create a cached engine holder.
	// The engine will be created lazily on first use and then reused.
	for i := range groups {
		g := groups[i]
		info := &linter.CheckerInfo{
			Name:    g.Name,
			Summary: g.DocSummary,
			Before:  g.DocBefore,
			After:   g.DocAfter,
			Note:    g.DocNote,
			Tags:    g.DocTags,

			EmbeddedRuleguard: true,
		}

		// Create a cached engine for this rule group
		cache := &cachedEngine{
			fset:         fset,
			buildContext: buildContext,
			groupName:    g.Name,
			debug:        ruleguardDebug,
		}

		collection.AddChecker(info, func(ctx *linter.CheckerContext) (linter.FileWalker, error) {
			engine, err := cache.get()
			if err != nil {
				return nil, err
			}
			c := &embeddedRuleguardChecker{
				ctx:    ctx,
				engine: engine,
			}
			return c, nil
		})
	}

	return nil
}

type embeddedRuleguardChecker struct {
	ctx    *linter.CheckerContext
	engine *ruleguard.Engine
}

func (c *embeddedRuleguardChecker) WalkFile(f *ast.File) {
	runRuleguardEngine(c.ctx, f, c.engine, &ruleguard.RunContext{
		Pkg:         c.ctx.Pkg,
		Types:       c.ctx.TypesInfo,
		Sizes:       c.ctx.SizesInfo,
		GoVersion:   ruleguard.GoVersion(c.ctx.GoVersion),
		Fset:        c.ctx.FileSet,
		TruncateLen: 100,
	})
}
