# fillswitch [![Build Status](https://travis-ci.org/davidrjenni/reftools.svg?branch=master)](https://travis-ci.org/davidrjenni/reftools) [![Coverage Status](https://coveralls.io/repos/github/davidrjenni/reftools/badge.svg)](https://coveralls.io/github/davidrjenni/reftools) [![GoDoc](https://godoc.org/github.com/davidrjenni/reftools?status.svg)](https://godoc.org/github.com/davidrjenni/reftools/cmd/fillswitch) [![Go Report Card](https://goreportcard.com/badge/github.com/davidrjenni/reftools)](https://goreportcard.com/report/github.com/davidrjenni/reftools)

fillswitch - fills (type) switches with case statements

---

For example, the following (type) switches,
```
var stmt ast.Stmt
switch stmt := stmt.(type) {
}

var kind ast.ObjKind
switch kind {
}
```
become:
```
var stmt ast.Stmt
switch stmt := stmt.(type) {
case *ast.AssignStmt:
case *ast.BadStmt:
case *ast.BlockStmt:
case *ast.BranchStmt:
case *ast.CaseClause:
case *ast.CommClause:
case *ast.DeclStmt:
case *ast.DeferStmt:
case *ast.EmptyStmt:
case *ast.ExprStmt:
case *ast.ForStmt:
case *ast.GoStmt:
case *ast.IfStmt:
case *ast.IncDecStmt:
case *ast.LabeledStmt:
case *ast.RangeStmt:
case *ast.ReturnStmt:
case *ast.SelectStmt:
case *ast.SendStmt:
case *ast.SwitchStmt:
case *ast.TypeSwitchStmt:
}

var kind ast.ObjKind
switch kind {
case ast.Bad:
case ast.Con:
case ast.Fun:
case ast.Lbl:
case ast.Pkg:
case ast.Typ:
case ast.Var:
}
```
after applying fillswitch for the (type) switch statements.

## Installation

```
% go get -u github.com/davidrjenni/reftools/cmd/fillswitch
```

## Usage

```
% fillswitch [-modified] -file=<filename> -offset=<byte offset> -line=<line number>
```

Flags:

	-file:     filename
	-modified: read an archive of modified files from stdin
	-offset:   byte offset of the (type) switch, optional if -line is present
	-line:     line number of the (type) switch, optional if -offset is present

If -offset as well as -line are present, then the tool first uses the
more specific offset information. If there was no (type) switch found
at the given offset, then the line information is used.
