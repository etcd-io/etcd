// Package sloglint implements the sloglint analyzer.
package sloglint

import (
	"go/ast"
	"go/version"
	"slices"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/types/typeutil"
)

// New creates a new sloglint analyzer.
func New(opts *Options) *analysis.Analyzer {
	if opts == nil {
		opts = &Options{NoMixedArguments: true}
	}

	return &analysis.Analyzer{
		Name:     "sloglint",
		Doc:      "Ensures consistent code style when using log/slog.",
		URL:      "https://go-simpler.org/sloglint",
		Flags:    flags(opts),
		Requires: []*analysis.Analyzer{inspect.Analyzer},
		Run: func(pass *analysis.Pass) (any, error) {
			if err := opts.validate(); err != nil {
				return nil, err
			}

			root := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector).Root()
			for cursor := range root.Preorder(new(ast.CallExpr), new(ast.CompositeLit)) {
				analyzeNode(pass, opts, cursor)
			}

			return nil, nil
		},
	}
}

var slogFuncs = []Func{
	{"log/slog.Log", 2, 3},
	{"log/slog.LogAttrs", 2, 3},
	{"log/slog.Debug", 0, 1},
	{"log/slog.Info", 0, 1},
	{"log/slog.Warn", 0, 1},
	{"log/slog.Error", 0, 1},
	{"log/slog.DebugContext", 1, 2},
	{"log/slog.InfoContext", 1, 2},
	{"log/slog.WarnContext", 1, 2},
	{"log/slog.ErrorContext", 1, 2},
	{"log/slog.With", -1, 0},
	{"log/slog.Group", -1, 1},
	{"log/slog.NewTextHandler", -1, -1},
	{"log/slog.NewJSONHandler", -1, -1},
	{"(*log/slog.Logger).Log", 2, 3},
	{"(*log/slog.Logger).LogAttrs", 2, 3},
	{"(*log/slog.Logger).Debug", 0, 1},
	{"(*log/slog.Logger).Info", 0, 1},
	{"(*log/slog.Logger).Warn", 0, 1},
	{"(*log/slog.Logger).Error", 0, 1},
	{"(*log/slog.Logger).DebugContext", 1, 2},
	{"(*log/slog.Logger).InfoContext", 1, 2},
	{"(*log/slog.Logger).WarnContext", 1, 2},
	{"(*log/slog.Logger).ErrorContext", 1, 2},
	{"(*log/slog.Logger).With", -1, 0},
}

func analyzeNode(pass *analysis.Pass, opts *Options, cursor inspector.Cursor) {
	node := cursor.Node()
	if cl, ok := node.(*ast.CompositeLit); ok && typeName(pass.TypesInfo, cl) == "log/slog.Attr" {
		analyzeAttrKey(pass, opts, cl)
		return
	}

	call, ok := node.(*ast.CallExpr)
	if !ok {
		return
	}

	fn := typeutil.StaticCallee(pass.TypesInfo, call)
	if fn == nil {
		return
	}

	switch fn.FullName() {
	case "log/slog.Int",
		"log/slog.Int64",
		"log/slog.Uint64",
		"log/slog.Float64",
		"log/slog.String",
		"log/slog.Bool",
		"log/slog.Time",
		"log/slog.Duration",
		"log/slog.Any":
		analyzeKey(pass, opts, call.Args[0])
		return
	case "log/slog.Group":
		analyzeKey(pass, opts, call.Args[0])
		// Special case: don't return here, we also need to analyze the group's arguments.
	}

	funcs := slices.Concat(slogFuncs, opts.CustomFuncs)
	idx := slices.IndexFunc(funcs, func(f Func) bool {
		return f.FullName == fn.FullName()
	})
	if idx == -1 {
		return
	}

	if idx < len(slogFuncs) {
		analyzeFunction(pass, opts, call, cursor)
	}
	if pos := funcs[idx].MessagePos; pos >= 0 && len(call.Args) > pos {
		analyzeMessage(pass, opts, call.Args[pos])
	}
	if pos := funcs[idx].ArgumentsPos; pos >= 0 && len(call.Args) > pos {
		analyzeArguments(pass, opts, call.Args[pos:])
	}
}

func analyzeFunction(pass *analysis.Pass, opts *Options, call *ast.CallExpr, cursor inspector.Cursor) {
	if opts.NoGlobalLogger != "" {
		noGlobalLogger(pass, call, opts.NoGlobalLogger == noGlobalLoggerDefault)
	}
	if opts.ContextOnly != "" {
		contextOnly(pass, call, cursor, opts.ContextOnly == contextOnlyScope)
	}
	v := pass.Module.GoVersion // Empty in test runs.
	if v == "" || version.Compare("go"+v, "go1.24") >= 0 {
		discardHandler(pass, call)
	}
}

func analyzeMessage(pass *analysis.Pass, opts *Options, msg ast.Expr) {
	if opts.StaticMessage {
		staticMessage(pass, msg)
	}
	if opts.MessageStyle != "" {
		messageStyle(pass, msg, opts.MessageStyle)
	}
}

func analyzeArguments(pass *analysis.Pass, opts *Options, args []ast.Expr) {
	var keys, attrs []ast.Expr

	for i := 0; i < len(args); i++ {
		typ := pass.TypesInfo.TypeOf(args[i])
		if typ == nil {
			continue
		}
		switch typ.String() {
		case "string":
			keys = append(keys, args[i])
			analyzeKey(pass, opts, args[i])
			i++ // Skip the value.
		case "log/slog.Attr":
			attrs = append(attrs, args[i])
		case "[]any", "[]log/slog.Attr":
			continue // The last argument may be an unpacked slice, skip it.
		}
	}

	if opts.NoMixedArguments {
		noMixedArguments(pass, keys, attrs)
	}
	if opts.KeyValuePairsOnly {
		keyValuePairsOnly(pass, attrs)
	}
	if opts.AttributesOnly {
		attributesOnly(pass, keys)
	}
	if opts.ArgumentsOnSeparateLines {
		argumentsOnSeparateLines(pass, keys, attrs)
	}
}

func analyzeKey(pass *analysis.Pass, opts *Options, key ast.Expr) {
	if opts.ConstantKeys {
		constantKeys(pass, key)
	}
	if opts.KeyNamingCase != "" {
		keyNamingCase(pass, key, opts.KeyNamingCase)
	}
	if len(opts.AllowedKeys) > 0 {
		allowedKeys(pass, key, opts.AllowedKeys)
	}
	if len(opts.ForbiddenKeys) > 0 {
		forbiddenKeys(pass, key, opts.ForbiddenKeys)
	}
}

func analyzeAttrKey(pass *analysis.Pass, opts *Options, attr *ast.CompositeLit) {
	switch len(attr.Elts) {
	case 1:
		if kv := attr.Elts[0].(*ast.KeyValueExpr); kv.Key.(*ast.Ident).Name == "Key" {
			analyzeKey(pass, opts, kv.Value) // slog.Attr{Key: ...}
		}
	case 2:
		if kv, ok := attr.Elts[0].(*ast.KeyValueExpr); ok && kv.Key.(*ast.Ident).Name == "Key" {
			analyzeKey(pass, opts, kv.Value) // slog.Attr{Key: ..., Value: ...}
		} else if kv, ok := attr.Elts[1].(*ast.KeyValueExpr); ok && kv.Key.(*ast.Ident).Name == "Key" {
			analyzeKey(pass, opts, kv.Value) // slog.Attr{Value: ..., Key: ...}
		} else {
			analyzeKey(pass, opts, attr.Elts[0]) // slog.Attr{..., ...}
		}
	}
}
