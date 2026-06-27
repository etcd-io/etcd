package noctx

import (
	"errors"
	"go/types"
	"strings"

	"github.com/gostaticanalysis/analysisutil"
	"golang.org/x/tools/go/analysis"
)

var errNotFound = errors.New("function not found")

func typeFuncs(pass *analysis.Pass, funcs []string) []*types.Func {
	fs := make([]*types.Func, 0, len(funcs))

	for _, fn := range funcs {
		f, err := typeFunc(pass, fn)
		if err != nil {
			continue
		}

		fs = append(fs, f)
	}

	return fs
}

func typeFunc(pass *analysis.Pass, funcName string) (*types.Func, error) {
	nameParts := strings.Split(strings.TrimSpace(funcName), ".")

	switch len(nameParts) {
	case 2:
		// package function: pkgname.Func
		f, ok := analysisutil.ObjectOf(pass, nameParts[0], nameParts[1]).(*types.Func)
		if !ok || f == nil {
			return nil, errNotFound
		}

		return f, nil
	case 3:
		// method: (*pkgname.Type).Method
		pkgName := strings.TrimLeft(nameParts[0], "(")
		typeName := strings.TrimRight(nameParts[1], ")")

		if pkgName != "" && pkgName[0] == '*' {
			pkgName = pkgName[1:]
			typeName = "*" + typeName
		}

		typ := analysisutil.TypeOf(pass, pkgName, typeName)
		if typ == nil {
			return nil, errNotFound
		}

		m := analysisutil.MethodOf(typ, nameParts[2])
		if m == nil {
			return nil, errNotFound
		}

		return m, nil
	}

	return nil, errNotFound
}
