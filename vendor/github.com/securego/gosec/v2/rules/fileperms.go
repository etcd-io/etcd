// (c) Copyright 2016 Hewlett Packard Enterprise Development LP
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rules

import (
	"fmt"
	"go/ast"
	"strconv"

	"github.com/securego/gosec/v2"
	"github.com/securego/gosec/v2/issue"
)

type filePermissions struct {
	issue.MetaData
	mode  int64
	pkgs  []string
	calls []string
}

func getConfiguredMode(conf map[string]interface{}, configKey string, defaultMode int64) int64 {
	mode := defaultMode
	if value, ok := conf[configKey]; ok {
		switch value := value.(type) {
		case int64:
			mode = value
		case string:
			if m, e := strconv.ParseInt(value, 0, 64); e != nil {
				mode = defaultMode
			} else {
				mode = m
			}
		}
	}
	return mode
}

func modeIsSubset(subset int64, superset int64) bool {
	return (subset | superset) == superset
}

// Match checks if the rule is matched.
func (r *filePermissions) Match(n ast.Node, c *gosec.Context) (*issue.Issue, error) {
	for _, pkg := range r.pkgs {
		if callexpr, matched := gosec.MatchCallByPackage(n, c, pkg, r.calls...); matched {
			modeArg := callexpr.Args[len(callexpr.Args)-1]
			if mode, err := gosec.GetInt(modeArg); err == nil && !modeIsSubset(mode, r.mode) || isOsPerm(modeArg) {
				return c.NewIssue(n, r.ID(), r.What, r.Severity, r.Confidence), nil
			}
		}
	}
	return nil, nil
}

// isOsPerm check if the provide ast node contains a os.PermMode symbol
func isOsPerm(n ast.Node) bool {
	if node, ok := n.(*ast.SelectorExpr); ok {
		if identX, ok := node.X.(*ast.Ident); ok {
			if identX.Name == "os" && node.Sel != nil && node.Sel.Name == "ModePerm" {
				return true
			}
		}
	}
	return false
}

// NewWritePerms creates a rule to detect file Writes with bad permissions.
func NewWritePerms(id string, conf gosec.Config) (gosec.Rule, []ast.Node) {
	mode := getConfiguredMode(conf, id, 0o600)
	return &filePermissions{
		mode:     mode,
		pkgs:     []string{"io/ioutil", "os"},
		calls:    []string{"WriteFile"},
		MetaData: issue.NewMetaData(id, fmt.Sprintf("Expect WriteFile permissions to be %#o or less", mode), issue.Medium, issue.High),
	}, []ast.Node{(*ast.CallExpr)(nil)}
}

// NewFilePerms creates a rule to detect file creation with a more permissive than configured
// permission mask.
func NewFilePerms(id string, conf gosec.Config) (gosec.Rule, []ast.Node) {
	mode := getConfiguredMode(conf, id, 0o600)
	return &filePermissions{
		mode:     mode,
		pkgs:     []string{"os"},
		calls:    []string{"OpenFile", "Chmod"},
		MetaData: issue.NewMetaData(id, fmt.Sprintf("Expect file permissions to be %#o or less", mode), issue.Medium, issue.High),
	}, []ast.Node{(*ast.CallExpr)(nil)}
}

// NewMkdirPerms creates a rule to detect directory creation with more permissive than
// configured permission mask.
func NewMkdirPerms(id string, conf gosec.Config) (gosec.Rule, []ast.Node) {
	mode := getConfiguredMode(conf, id, 0o750)
	return &filePermissions{
		mode:     mode,
		pkgs:     []string{"os"},
		calls:    []string{"Mkdir", "MkdirAll"},
		MetaData: issue.NewMetaData(id, fmt.Sprintf("Expect directory permissions to be %#o or less", mode), issue.Medium, issue.High),
	}, []ast.Node{(*ast.CallExpr)(nil)}
}

type osCreatePermissions struct {
	issue.MetaData
	mode  int64
	pkgs  []string
	calls []string
}

const defaultOsCreateMode = 0o666

// Match checks if the rule is matched.
func (r *osCreatePermissions) Match(n ast.Node, c *gosec.Context) (*issue.Issue, error) {
	for _, pkg := range r.pkgs {
		if _, matched := gosec.MatchCallByPackage(n, c, pkg, r.calls...); matched {
			if !modeIsSubset(defaultOsCreateMode, r.mode) {
				return c.NewIssue(n, r.ID(), r.What, r.Severity, r.Confidence), nil
			}
		}
	}
	return nil, nil
}

// NewOsCreatePerms creates a rule to detect file creation with a more permissive than configured
// permission mask.
func NewOsCreatePerms(id string, conf gosec.Config) (gosec.Rule, []ast.Node) {
	mode := getConfiguredMode(conf, id, 0o666)
	return &osCreatePermissions{
		mode:  mode,
		pkgs:  []string{"os"},
		calls: []string{"Create"},
		MetaData: issue.NewMetaData(id, fmt.Sprintf("Expect file permissions to be %#o or less but os.Create used with default permissions %#o",
			mode, defaultOsCreateMode), issue.Medium, issue.High),
	}, []ast.Node{(*ast.CallExpr)(nil)}
}
