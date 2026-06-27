// Package tagliatelle a linter that handle struct tags.
package tagliatelle

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"maps"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	iradix "github.com/hashicorp/go-immutable-radix/v2"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Config the tagliatelle configuration.
type Config struct {
	Base

	Overrides []Overrides
}

// Overrides applies configuration overrides by package.
type Overrides struct {
	Base

	Package string
}

// Base shared configuration between rules.
type Base struct {
	Rules         map[string]string
	ExtendedRules map[string]ExtendedRule
	UseFieldName  bool
	IgnoredFields []string
	Ignore        bool
}

// ExtendedRule allows to customize rules.
type ExtendedRule struct {
	Case                string
	ExtraInitialisms    bool
	InitialismOverrides map[string]bool
}

// New creates an analyzer.
func New(config Config) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name: "tagliatelle",
		Doc:  "Checks the struct tags.",
		Run: func(pass *analysis.Pass) (any, error) {
			if len(config.Rules) == 0 && len(config.ExtendedRules) == 0 && len(config.Overrides) == 0 {
				return nil, nil
			}

			return run(pass, config)
		},
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	}
}

func run(pass *analysis.Pass, config Config) (any, error) {
	isp, ok := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if !ok {
		return nil, errors.New("missing inspect analyser")
	}

	nodeFilter := []ast.Node{
		(*ast.StructType)(nil),
	}

	cfg := config.Base

	if pass.Module != nil {
		radixTree := createRadixTree(config, pass.Module.Path)
		_, cfg, _ = radixTree.Root().LongestPrefix([]byte(pass.Pkg.Path()))
	}

	if cfg.Ignore {
		return nil, nil
	}

	isp.Preorder(nodeFilter, func(n ast.Node) {
		node, ok := n.(*ast.StructType)
		if !ok {
			return
		}

		for _, field := range node.Fields.List {
			analyze(pass, cfg, node, field)
		}
	})

	return nil, nil
}

func analyze(pass *analysis.Pass, config Base, n *ast.StructType, field *ast.Field) {
	if n.Fields == nil || n.Fields.NumFields() < 1 {
		// skip empty structs
		return
	}

	if field.Tag == nil {
		// skip when no struct tag
		return
	}

	fieldName, err := getFieldName(field)
	if err != nil {
		pass.Reportf(n.Pos(), "unable to get field name: %v", err)
		return
	}

	cleanRules(config)

	if slices.Contains(config.IgnoredFields, fieldName) {
		return
	}

	for key, extRule := range config.ExtendedRules {
		report(pass, config, key, extRule.Case, fieldName, n, field, func() (Converter, error) {
			return ruleToConverter(extRule)
		})
	}

	for key, convName := range config.Rules {
		report(pass, config, key, convName, fieldName, n, field, func() (Converter, error) {
			return getSimpleConverter(convName)
		})
	}
}

func report(pass *analysis.Pass, config Base, key, convName, fieldName string, n *ast.StructType, field *ast.Field, fn ConverterCallback) {
	if convName == "" {
		return
	}

	value, flags, ok := lookupTagValue(field.Tag, key)
	if !ok {
		// skip when no struct tag for the key
		return
	}

	if value == "-" {
		// skip when skipped :)
		return
	}

	// TODO(ldez): need to be rethink.
	// tagliatelle should try to remain neutral in terms of format.
	if key == "xml" && strings.ContainsAny(value, ">:") {
		// ignore XML names than contains path
		return
	}

	// TODO(ldez): need to be rethink.
	// This is an exception because of a bug.
	// https://github.com/ldez/tagliatelle/issues/8
	// For now, tagliatelle should try to remain neutral in terms of format.
	if slices.Contains(flags, "inline") {
		// skip for inline children (no name to lint)
		return
	}

	if value == "" {
		value = fieldName
	}

	converter, err := fn()
	if err != nil {
		pass.Reportf(n.Pos(), "%s(%s): %v", key, convName, err)
		return
	}

	expected := value
	if config.UseFieldName {
		expected = fieldName
	}

	if value != converter(expected) {
		pass.Reportf(field.Tag.Pos(), "%s(%s): got '%s' want '%s'", key, convName, value, converter(expected))
	}
}

func getFieldName(field *ast.Field) (string, error) {
	var name string

	for _, n := range field.Names {
		if n.Name != "" {
			name = n.Name
		}
	}

	if name != "" {
		return name, nil
	}

	return getTypeName(field.Type)
}

func getTypeName(exp ast.Expr) (string, error) {
	switch typ := exp.(type) {
	case *ast.Ident:
		return typ.Name, nil
	case *ast.StarExpr:
		return getTypeName(typ.X)
	case *ast.SelectorExpr:
		return getTypeName(typ.Sel)
	default:
		bytes, _ := json.Marshal(exp)
		return "", fmt.Errorf("unexpected error: type %T: %s", typ, string(bytes))
	}
}

func lookupTagValue(tag *ast.BasicLit, key string) (name string, flags []string, ok bool) {
	raw := strings.Trim(tag.Value, "`")

	value, ok := reflect.StructTag(raw).Lookup(key)
	if !ok {
		return value, nil, ok
	}

	values := strings.Split(value, ",")

	if len(values) < 1 {
		return "", nil, true
	}

	return values[0], values[1:], true
}

func createRadixTree(config Config, modPath string) *iradix.Tree[Base] {
	r := iradix.New[Base]()

	defaultRule := Base{
		Rules:         maps.Clone(config.Rules),
		ExtendedRules: maps.Clone(config.ExtendedRules),
		UseFieldName:  config.UseFieldName,
		Ignore:        config.Ignore,
	}

	defaultRule.IgnoredFields = append(defaultRule.IgnoredFields, config.IgnoredFields...)

	r, _, _ = r.Insert([]byte(""), defaultRule)

	for _, override := range config.Overrides {
		c := Base{
			UseFieldName: override.UseFieldName,
			Ignore:       override.Ignore,
		}

		// If there is an override, the base configuration is ignored.
		if len(override.IgnoredFields) == 0 {
			c.IgnoredFields = append(c.IgnoredFields, config.IgnoredFields...)
		} else {
			c.IgnoredFields = append(c.IgnoredFields, override.IgnoredFields...)
		}

		// Copy the rules from the base.
		c.Rules = maps.Clone(config.Rules)

		// Overrides the rule from the base.
		for k, v := range override.Rules {
			c.Rules[k] = v
		}

		// Copy the extended rules from the base.
		c.ExtendedRules = maps.Clone(config.ExtendedRules)

		// Overrides the extended rule from the base.
		for k, v := range override.ExtendedRules {
			c.ExtendedRules[k] = v
		}

		key := path.Join(modPath, override.Package)
		if filepath.Base(modPath) == override.Package {
			key = modPath
		}

		r, _, _ = r.Insert([]byte(key), c)
	}

	return r
}

func cleanRules(config Base) {
	for k := range config.ExtendedRules {
		delete(config.Rules, k)
	}
}
