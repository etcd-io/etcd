// Code generated "gen_operations.go"; DO NOT EDIT.

package gogrep

import (
	"github.com/quasilyte/gogrep/nodetag"
)

//go:generate stringer -type=operation -trimprefix=op
type operation uint8

const (
	opInvalid operation = 0

	// Tag: Node
	opNode operation = 1

	// Tag: Node
	// ValueIndex: strings | wildcard name
	opNamedNode operation = 2

	// Tag: Unknown
	opNodeSeq operation = 3

	// Tag: Unknown
	// ValueIndex: strings | wildcard name
	opNamedNodeSeq operation = 4

	// Tag: Unknown
	opOptNode operation = 5

	// Tag: Unknown
	// ValueIndex: strings | wildcard name
	opNamedOptNode operation = 6

	// Tag: Node
	opFieldNode operation = 7

	// Tag: Node
	// ValueIndex: strings | wildcard name
	opNamedFieldNode operation = 8

	// Tag: StmtList
	// Args: stmts...
	// Example: f(); g()
	opMultiStmt operation = 9

	// Tag: ExprList
	// Args: exprs...
	// Example: f(), g()
	opMultiExpr operation = 10

	// Tag: DeclList
	// Args: exprs...
	// Example: f(), g()
	opMultiDecl operation = 11

	// Tag: Unknown
	opEnd operation = 12

	// Tag: BasicLit
	// ValueIndex: ifaces | parsed literal value
	opBasicLit operation = 13

	// Tag: BasicLit
	// ValueIndex: strings | raw literal value
	opStrictIntLit operation = 14

	// Tag: BasicLit
	// ValueIndex: strings | raw literal value
	opStrictFloatLit operation = 15

	// Tag: BasicLit
	// ValueIndex: strings | raw literal value
	opStrictCharLit operation = 16

	// Tag: BasicLit
	// ValueIndex: strings | raw literal value
	opStrictStringLit operation = 17

	// Tag: BasicLit
	// ValueIndex: strings | raw literal value
	opStrictComplexLit operation = 18

	// Tag: Ident
	// ValueIndex: strings | ident name
	opIdent operation = 19

	// Tag: Ident
	// ValueIndex: strings | package path
	opPkg operation = 20

	// Tag: IndexExpr
	// Args: x expr
	opIndexExpr operation = 21

	// Tag: IndexListExpr
	// Args: x exprs...
	opIndexListExpr operation = 22

	// Tag: SliceExpr
	// Args: x
	opSliceExpr operation = 23

	// Tag: SliceExpr
	// Args: x from
	// Example: x[from:]
	opSliceFromExpr operation = 24

	// Tag: SliceExpr
	// Args: x to
	// Example: x[:to]
	opSliceToExpr operation = 25

	// Tag: SliceExpr
	// Args: x from to
	// Example: x[from:to]
	opSliceFromToExpr operation = 26

	// Tag: SliceExpr
	// Args: x from cap
	// Example: x[:from:cap]
	opSliceToCapExpr operation = 27

	// Tag: SliceExpr
	// Args: x from to cap
	// Example: x[from:to:cap]
	opSliceFromToCapExpr operation = 28

	// Tag: FuncLit
	// Args: type block
	opFuncLit operation = 29

	// Tag: CompositeLit
	// Args: elts...
	// Example: {elts...}
	opCompositeLit operation = 30

	// Tag: CompositeLit
	// Args: typ elts...
	// Example: typ{elts...}
	opTypedCompositeLit operation = 31

	// Tag: SelectorExpr
	// Args: x
	// ValueIndex: strings | selector name
	opSimpleSelectorExpr operation = 32

	// Tag: SelectorExpr
	// Args: x sel
	opSelectorExpr operation = 33

	// Tag: TypeAssertExpr
	// Args: x typ
	opTypeAssertExpr operation = 34

	// Tag: TypeAssertExpr
	// Args: x
	opTypeSwitchAssertExpr operation = 35

	// Tag: StructType
	// Args: fields
	opStructType operation = 36

	// Tag: InterfaceType
	// Args: fields
	opInterfaceType operation = 37

	// Tag: InterfaceType
	opEfaceType operation = 38

	// Tag: FuncType
	// Args: params
	opVoidFuncType operation = 39

	// Tag: FuncType
	// Args: typeparams params
	opGenericVoidFuncType operation = 40

	// Tag: FuncType
	// Args: params results
	opFuncType operation = 41

	// Tag: FuncType
	// Args: typeparams params results
	opGenericFuncType operation = 42

	// Tag: ArrayType
	// Args: length elem
	opArrayType operation = 43

	// Tag: ArrayType
	// Args: elem
	opSliceType operation = 44

	// Tag: MapType
	// Args: key value
	opMapType operation = 45

	// Tag: ChanType
	// Args: value
	// Value: ast.ChanDir | channel direction
	opChanType operation = 46

	// Tag: KeyValueExpr
	// Args: key value
	opKeyValueExpr operation = 47

	// Tag: Ellipsis
	opEllipsis operation = 48

	// Tag: Ellipsis
	// Args: type
	opTypedEllipsis operation = 49

	// Tag: StarExpr
	// Args: x
	opStarExpr operation = 50

	// Tag: UnaryExpr
	// Args: x
	// Value: token.Token | unary operator
	opUnaryExpr operation = 51

	// Tag: BinaryExpr
	// Args: x y
	// Value: token.Token | binary operator
	opBinaryExpr operation = 52

	// Tag: ParenExpr
	// Args: x
	opParenExpr operation = 53

	// Tag: Unknown
	// Args: exprs...
	// Example: 1, 2, 3
	opArgList operation = 54

	// Tag: Unknown
	// Like ArgList, but pattern contains no $*
	// Args: exprs[]
	// Example: 1, 2, 3
	// Value: int | slice len
	opSimpleArgList operation = 55

	// Tag: CallExpr
	// Args: fn args
	// Example: f(1, xs...)
	opVariadicCallExpr operation = 56

	// Tag: CallExpr
	// Args: fn args
	// Example: f(1, xs)
	opNonVariadicCallExpr operation = 57

	// Tag: CallExpr
	// Args: fn args
	// Example: f(1, xs) or f(1, xs...)
	// Value: int | can be variadic if len(args)>value
	opMaybeVariadicCallExpr operation = 58

	// Tag: CallExpr
	// Args: fn args
	// Example: f(1, xs) or f(1, xs...)
	opCallExpr operation = 59

	// Tag: AssignStmt
	// Args: lhs rhs
	// Example: lhs := rhs()
	// Value: token.Token | ':=' or '='
	opAssignStmt operation = 60

	// Tag: AssignStmt
	// Args: lhs... rhs...
	// Example: lhs1, lhs2 := rhs()
	// Value: token.Token | ':=' or '='
	opMultiAssignStmt operation = 61

	// Tag: BranchStmt
	// Args: x
	// Value: token.Token | branch kind
	opBranchStmt operation = 62

	// Tag: BranchStmt
	// Args: x
	// Value: token.Token | branch kind
	// ValueIndex: strings | label name
	opSimpleLabeledBranchStmt operation = 63

	// Tag: BranchStmt
	// Args: label x
	// Value: token.Token | branch kind
	opLabeledBranchStmt operation = 64

	// Tag: LabeledStmt
	// Args: x
	// ValueIndex: strings | label name
	opSimpleLabeledStmt operation = 65

	// Tag: LabeledStmt
	// Args: label x
	opLabeledStmt operation = 66

	// Tag: BlockStmt
	// Args: body...
	opBlockStmt operation = 67

	// Tag: ExprStmt
	// Args: x
	opExprStmt operation = 68

	// Tag: GoStmt
	// Args: x
	opGoStmt operation = 69

	// Tag: DeferStmt
	// Args: x
	opDeferStmt operation = 70

	// Tag: SendStmt
	// Args: ch value
	opSendStmt operation = 71

	// Tag: EmptyStmt
	opEmptyStmt operation = 72

	// Tag: IncDecStmt
	// Args: x
	// Value: token.Token | '++' or '--'
	opIncDecStmt operation = 73

	// Tag: ReturnStmt
	// Args: results...
	opReturnStmt operation = 74

	// Tag: IfStmt
	// Args: cond block
	// Example: if cond {}
	opIfStmt operation = 75

	// Tag: IfStmt
	// Args: init cond block
	// Example: if init; cond {}
	opIfInitStmt operation = 76

	// Tag: IfStmt
	// Args: cond block else
	// Example: if cond {} else ...
	opIfElseStmt operation = 77

	// Tag: IfStmt
	// Args: init cond block else
	// Example: if init; cond {} else ...
	opIfInitElseStmt operation = 78

	// Tag: IfStmt
	// Args: block
	// Example: if $*x {}
	// ValueIndex: strings | wildcard name
	opIfNamedOptStmt operation = 79

	// Tag: IfStmt
	// Args: block else
	// Example: if $*x {} else ...
	// ValueIndex: strings | wildcard name
	opIfNamedOptElseStmt operation = 80

	// Tag: SwitchStmt
	// Args: body...
	// Example: switch {}
	opSwitchStmt operation = 81

	// Tag: SwitchStmt
	// Args: tag body...
	// Example: switch tag {}
	opSwitchTagStmt operation = 82

	// Tag: SwitchStmt
	// Args: init body...
	// Example: switch init; {}
	opSwitchInitStmt operation = 83

	// Tag: SwitchStmt
	// Args: init tag body...
	// Example: switch init; tag {}
	opSwitchInitTagStmt operation = 84

	// Tag: SelectStmt
	// Args: body...
	opSelectStmt operation = 85

	// Tag: TypeSwitchStmt
	// Args: x block
	// Example: switch x.(type) {}
	opTypeSwitchStmt operation = 86

	// Tag: TypeSwitchStmt
	// Args: init x block
	// Example: switch init; x.(type) {}
	opTypeSwitchInitStmt operation = 87

	// Tag: CaseClause
	// Args: values... body...
	opCaseClause operation = 88

	// Tag: CaseClause
	// Args: body...
	opDefaultCaseClause operation = 89

	// Tag: CommClause
	// Args: comm body...
	opCommClause operation = 90

	// Tag: CommClause
	// Args: body...
	opDefaultCommClause operation = 91

	// Tag: ForStmt
	// Args: blocl
	// Example: for {}
	opForStmt operation = 92

	// Tag: ForStmt
	// Args: post block
	// Example: for ; ; post {}
	opForPostStmt operation = 93

	// Tag: ForStmt
	// Args: cond block
	// Example: for ; cond; {}
	opForCondStmt operation = 94

	// Tag: ForStmt
	// Args: cond post block
	// Example: for ; cond; post {}
	opForCondPostStmt operation = 95

	// Tag: ForStmt
	// Args: init block
	// Example: for init; ; {}
	opForInitStmt operation = 96

	// Tag: ForStmt
	// Args: init post block
	// Example: for init; ; post {}
	opForInitPostStmt operation = 97

	// Tag: ForStmt
	// Args: init cond block
	// Example: for init; cond; {}
	opForInitCondStmt operation = 98

	// Tag: ForStmt
	// Args: init cond post block
	// Example: for init; cond; post {}
	opForInitCondPostStmt operation = 99

	// Tag: RangeStmt
	// Args: x block
	// Example: for range x {}
	opRangeStmt operation = 100

	// Tag: RangeStmt
	// Args: key x block
	// Example: for key := range x {}
	// Value: token.Token | ':=' or '='
	opRangeKeyStmt operation = 101

	// Tag: RangeStmt
	// Args: key value x block
	// Example: for key, value := range x {}
	// Value: token.Token | ':=' or '='
	opRangeKeyValueStmt operation = 102

	// Tag: RangeStmt
	// Args: x
	// Example: range x
	opRangeClause operation = 103

	// Tag: RangeStmt
	// Args: x
	// Example: for range x
	opRangeHeader operation = 104

	// Tag: RangeStmt
	// Args: key x
	// Example: for key := range x
	// Value: token.Token | ':=' or '='
	opRangeKeyHeader operation = 105

	// Tag: RangeStmt
	// Args: key value x
	// Example: for key, value := range x
	// Value: token.Token | ':=' or '='
	opRangeKeyValueHeader operation = 106

	// Tag: Unknown
	// Args: fields...
	opFieldList operation = 107

	// Tag: Unknown
	// Args: typ
	// Example: type
	opUnnamedField operation = 108

	// Tag: Unknown
	// Args: typ
	// Example: name type
	// ValueIndex: strings | field name
	opSimpleField operation = 109

	// Tag: Unknown
	// Args: name typ
	// Example: $name type
	opField operation = 110

	// Tag: Unknown
	// Args: names... typ
	// Example: name1, name2 type
	opMultiField operation = 111

	// Tag: ValueSpec
	// Args: value
	opValueSpec operation = 112

	// Tag: ValueSpec
	// Args: lhs... rhs...
	// Example: lhs = rhs
	opValueInitSpec operation = 113

	// Tag: ValueSpec
	// Args: lhs... type rhs...
	// Example: lhs typ = rhs
	opTypedValueInitSpec operation = 114

	// Tag: ValueSpec
	// Args: lhs... type
	// Example: lhs typ
	opTypedValueSpec operation = 115

	// Tag: TypeSpec
	// Args: type
	// Example: name type
	// ValueIndex: strings | type name
	opSimpleTypeSpec operation = 116

	// Tag: TypeSpec
	// Args: name type
	// Example: name type
	opTypeSpec operation = 117

	// Tag: TypeSpec
	// Args: name typeparasm type
	// Example: name[typeparams] type
	opGenericTypeSpec operation = 118

	// Tag: TypeSpec
	// Args: name type
	// Example: name = type
	opTypeAliasSpec operation = 119

	// Tag: FuncDecl
	// Args: type block
	// ValueIndex: strings | field name
	opSimpleFuncDecl operation = 120

	// Tag: FuncDecl
	// Args: name type block
	opFuncDecl operation = 121

	// Tag: FuncDecl
	// Args: recv name type block
	opMethodDecl operation = 122

	// Tag: FuncDecl
	// Args: name type
	opFuncProtoDecl operation = 123

	// Tag: FuncDecl
	// Args: recv name type
	opMethodProtoDecl operation = 124

	// Tag: DeclStmt
	// Args: decl
	opDeclStmt operation = 125

	// Tag: GenDecl
	// Args: valuespecs...
	opConstDecl operation = 126

	// Tag: GenDecl
	// Args: valuespecs...
	opVarDecl operation = 127

	// Tag: GenDecl
	// Args: typespecs...
	opTypeDecl operation = 128

	// Tag: GenDecl
	opAnyImportDecl operation = 129

	// Tag: GenDecl
	// Args: importspecs...
	opImportDecl operation = 130

	// Tag: File
	// Args: name
	opEmptyPackage operation = 131
)

type operationInfo struct {
	Tag            nodetag.Value
	NumArgs        int
	ValueKind      valueKind
	ExtraValueKind valueKind
	VariadicMap    bitmap64
	SliceIndex     int
}

var operationInfoTable = [256]operationInfo{
	opInvalid: {},

	opNode: {
		Tag:            nodetag.Node,
		NumArgs:        0,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opNamedNode: {
		Tag:            nodetag.Node,
		NumArgs:        0,
		ValueKind:      emptyValue,
		ExtraValueKind: stringValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opNodeSeq: {
		Tag:            nodetag.Unknown,
		NumArgs:        0,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opNamedNodeSeq: {
		Tag:            nodetag.Unknown,
		NumArgs:        0,
		ValueKind:      emptyValue,
		ExtraValueKind: stringValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opOptNode: {
		Tag:            nodetag.Unknown,
		NumArgs:        0,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opNamedOptNode: {
		Tag:            nodetag.Unknown,
		NumArgs:        0,
		ValueKind:      emptyValue,
		ExtraValueKind: stringValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opFieldNode: {
		Tag:            nodetag.Node,
		NumArgs:        0,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opNamedFieldNode: {
		Tag:            nodetag.Node,
		NumArgs:        0,
		ValueKind:      emptyValue,
		ExtraValueKind: stringValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opMultiStmt: {
		Tag:            nodetag.StmtList,
		NumArgs:        1,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    1, // 1
		SliceIndex:     -1,
	},
	opMultiExpr: {
		Tag:            nodetag.ExprList,
		NumArgs:        1,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    1, // 1
		SliceIndex:     -1,
	},
	opMultiDecl: {
		Tag:            nodetag.DeclList,
		NumArgs:        1,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    1, // 1
		SliceIndex:     -1,
	},
	opEnd: {
		Tag:            nodetag.Unknown,
		NumArgs:        0,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opBasicLit: {
		Tag:            nodetag.BasicLit,
		NumArgs:        0,
		ValueKind:      emptyValue,
		ExtraValueKind: ifaceValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opStrictIntLit: {
		Tag:            nodetag.BasicLit,
		NumArgs:        0,
		ValueKind:      emptyValue,
		ExtraValueKind: stringValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opStrictFloatLit: {
		Tag:            nodetag.BasicLit,
		NumArgs:        0,
		ValueKind:      emptyValue,
		ExtraValueKind: stringValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opStrictCharLit: {
		Tag:            nodetag.BasicLit,
		NumArgs:        0,
		ValueKind:      emptyValue,
		ExtraValueKind: stringValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opStrictStringLit: {
		Tag:            nodetag.BasicLit,
		NumArgs:        0,
		ValueKind:      emptyValue,
		ExtraValueKind: stringValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opStrictComplexLit: {
		Tag:            nodetag.BasicLit,
		NumArgs:        0,
		ValueKind:      emptyValue,
		ExtraValueKind: stringValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opIdent: {
		Tag:            nodetag.Ident,
		NumArgs:        0,
		ValueKind:      emptyValue,
		ExtraValueKind: stringValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opPkg: {
		Tag:            nodetag.Ident,
		NumArgs:        0,
		ValueKind:      emptyValue,
		ExtraValueKind: stringValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opIndexExpr: {
		Tag:            nodetag.IndexExpr,
		NumArgs:        2,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opIndexListExpr: {
		Tag:            nodetag.IndexListExpr,
		NumArgs:        2,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    2, // 10
		SliceIndex:     -1,
	},
	opSliceExpr: {
		Tag:            nodetag.SliceExpr,
		NumArgs:        1,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opSliceFromExpr: {
		Tag:            nodetag.SliceExpr,
		NumArgs:        2,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opSliceToExpr: {
		Tag:            nodetag.SliceExpr,
		NumArgs:        2,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opSliceFromToExpr: {
		Tag:            nodetag.SliceExpr,
		NumArgs:        3,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opSliceToCapExpr: {
		Tag:            nodetag.SliceExpr,
		NumArgs:        3,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opSliceFromToCapExpr: {
		Tag:            nodetag.SliceExpr,
		NumArgs:        4,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opFuncLit: {
		Tag:            nodetag.FuncLit,
		NumArgs:        2,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opCompositeLit: {
		Tag:            nodetag.CompositeLit,
		NumArgs:        1,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    1, // 1
		SliceIndex:     -1,
	},
	opTypedCompositeLit: {
		Tag:            nodetag.CompositeLit,
		NumArgs:        2,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    2, // 10
		SliceIndex:     -1,
	},
	opSimpleSelectorExpr: {
		Tag:            nodetag.SelectorExpr,
		NumArgs:        1,
		ValueKind:      emptyValue,
		ExtraValueKind: stringValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opSelectorExpr: {
		Tag:            nodetag.SelectorExpr,
		NumArgs:        2,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opTypeAssertExpr: {
		Tag:            nodetag.TypeAssertExpr,
		NumArgs:        2,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opTypeSwitchAssertExpr: {
		Tag:            nodetag.TypeAssertExpr,
		NumArgs:        1,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opStructType: {
		Tag:            nodetag.StructType,
		NumArgs:        1,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opInterfaceType: {
		Tag:            nodetag.InterfaceType,
		NumArgs:        1,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opEfaceType: {
		Tag:            nodetag.InterfaceType,
		NumArgs:        0,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opVoidFuncType: {
		Tag:            nodetag.FuncType,
		NumArgs:        1,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opGenericVoidFuncType: {
		Tag:            nodetag.FuncType,
		NumArgs:        2,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opFuncType: {
		Tag:            nodetag.FuncType,
		NumArgs:        2,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opGenericFuncType: {
		Tag:            nodetag.FuncType,
		NumArgs:        3,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opArrayType: {
		Tag:            nodetag.ArrayType,
		NumArgs:        2,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opSliceType: {
		Tag:            nodetag.ArrayType,
		NumArgs:        1,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opMapType: {
		Tag:            nodetag.MapType,
		NumArgs:        2,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opChanType: {
		Tag:            nodetag.ChanType,
		NumArgs:        1,
		ValueKind:      chandirValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opKeyValueExpr: {
		Tag:            nodetag.KeyValueExpr,
		NumArgs:        2,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opEllipsis: {
		Tag:            nodetag.Ellipsis,
		NumArgs:        0,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opTypedEllipsis: {
		Tag:            nodetag.Ellipsis,
		NumArgs:        1,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opStarExpr: {
		Tag:            nodetag.StarExpr,
		NumArgs:        1,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opUnaryExpr: {
		Tag:            nodetag.UnaryExpr,
		NumArgs:        1,
		ValueKind:      tokenValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opBinaryExpr: {
		Tag:            nodetag.BinaryExpr,
		NumArgs:        2,
		ValueKind:      tokenValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opParenExpr: {
		Tag:            nodetag.ParenExpr,
		NumArgs:        1,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opArgList: {
		Tag:            nodetag.Unknown,
		NumArgs:        1,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    1, // 1
		SliceIndex:     -1,
	},
	opSimpleArgList: {
		Tag:            nodetag.Unknown,
		NumArgs:        1,
		ValueKind:      intValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     0,
	},
	opVariadicCallExpr: {
		Tag:            nodetag.CallExpr,
		NumArgs:        2,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opNonVariadicCallExpr: {
		Tag:            nodetag.CallExpr,
		NumArgs:        2,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opMaybeVariadicCallExpr: {
		Tag:            nodetag.CallExpr,
		NumArgs:        2,
		ValueKind:      intValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opCallExpr: {
		Tag:            nodetag.CallExpr,
		NumArgs:        2,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opAssignStmt: {
		Tag:            nodetag.AssignStmt,
		NumArgs:        2,
		ValueKind:      tokenValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opMultiAssignStmt: {
		Tag:            nodetag.AssignStmt,
		NumArgs:        2,
		ValueKind:      tokenValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    3, // 11
		SliceIndex:     -1,
	},
	opBranchStmt: {
		Tag:            nodetag.BranchStmt,
		NumArgs:        1,
		ValueKind:      tokenValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opSimpleLabeledBranchStmt: {
		Tag:            nodetag.BranchStmt,
		NumArgs:        1,
		ValueKind:      tokenValue,
		ExtraValueKind: stringValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opLabeledBranchStmt: {
		Tag:            nodetag.BranchStmt,
		NumArgs:        2,
		ValueKind:      tokenValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opSimpleLabeledStmt: {
		Tag:            nodetag.LabeledStmt,
		NumArgs:        1,
		ValueKind:      emptyValue,
		ExtraValueKind: stringValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opLabeledStmt: {
		Tag:            nodetag.LabeledStmt,
		NumArgs:        2,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opBlockStmt: {
		Tag:            nodetag.BlockStmt,
		NumArgs:        1,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    1, // 1
		SliceIndex:     -1,
	},
	opExprStmt: {
		Tag:            nodetag.ExprStmt,
		NumArgs:        1,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opGoStmt: {
		Tag:            nodetag.GoStmt,
		NumArgs:        1,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opDeferStmt: {
		Tag:            nodetag.DeferStmt,
		NumArgs:        1,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opSendStmt: {
		Tag:            nodetag.SendStmt,
		NumArgs:        2,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opEmptyStmt: {
		Tag:            nodetag.EmptyStmt,
		NumArgs:        0,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opIncDecStmt: {
		Tag:            nodetag.IncDecStmt,
		NumArgs:        1,
		ValueKind:      tokenValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opReturnStmt: {
		Tag:            nodetag.ReturnStmt,
		NumArgs:        1,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    1, // 1
		SliceIndex:     -1,
	},
	opIfStmt: {
		Tag:            nodetag.IfStmt,
		NumArgs:        2,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opIfInitStmt: {
		Tag:            nodetag.IfStmt,
		NumArgs:        3,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opIfElseStmt: {
		Tag:            nodetag.IfStmt,
		NumArgs:        3,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opIfInitElseStmt: {
		Tag:            nodetag.IfStmt,
		NumArgs:        4,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opIfNamedOptStmt: {
		Tag:            nodetag.IfStmt,
		NumArgs:        1,
		ValueKind:      emptyValue,
		ExtraValueKind: stringValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opIfNamedOptElseStmt: {
		Tag:            nodetag.IfStmt,
		NumArgs:        2,
		ValueKind:      emptyValue,
		ExtraValueKind: stringValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opSwitchStmt: {
		Tag:            nodetag.SwitchStmt,
		NumArgs:        1,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    1, // 1
		SliceIndex:     -1,
	},
	opSwitchTagStmt: {
		Tag:            nodetag.SwitchStmt,
		NumArgs:        2,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    2, // 10
		SliceIndex:     -1,
	},
	opSwitchInitStmt: {
		Tag:            nodetag.SwitchStmt,
		NumArgs:        2,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    2, // 10
		SliceIndex:     -1,
	},
	opSwitchInitTagStmt: {
		Tag:            nodetag.SwitchStmt,
		NumArgs:        3,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    4, // 100
		SliceIndex:     -1,
	},
	opSelectStmt: {
		Tag:            nodetag.SelectStmt,
		NumArgs:        1,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    1, // 1
		SliceIndex:     -1,
	},
	opTypeSwitchStmt: {
		Tag:            nodetag.TypeSwitchStmt,
		NumArgs:        2,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opTypeSwitchInitStmt: {
		Tag:            nodetag.TypeSwitchStmt,
		NumArgs:        3,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opCaseClause: {
		Tag:            nodetag.CaseClause,
		NumArgs:        2,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    3, // 11
		SliceIndex:     -1,
	},
	opDefaultCaseClause: {
		Tag:            nodetag.CaseClause,
		NumArgs:        1,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    1, // 1
		SliceIndex:     -1,
	},
	opCommClause: {
		Tag:            nodetag.CommClause,
		NumArgs:        2,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    2, // 10
		SliceIndex:     -1,
	},
	opDefaultCommClause: {
		Tag:            nodetag.CommClause,
		NumArgs:        1,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    1, // 1
		SliceIndex:     -1,
	},
	opForStmt: {
		Tag:            nodetag.ForStmt,
		NumArgs:        1,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opForPostStmt: {
		Tag:            nodetag.ForStmt,
		NumArgs:        2,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opForCondStmt: {
		Tag:            nodetag.ForStmt,
		NumArgs:        2,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opForCondPostStmt: {
		Tag:            nodetag.ForStmt,
		NumArgs:        3,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opForInitStmt: {
		Tag:            nodetag.ForStmt,
		NumArgs:        2,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opForInitPostStmt: {
		Tag:            nodetag.ForStmt,
		NumArgs:        3,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opForInitCondStmt: {
		Tag:            nodetag.ForStmt,
		NumArgs:        3,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opForInitCondPostStmt: {
		Tag:            nodetag.ForStmt,
		NumArgs:        4,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opRangeStmt: {
		Tag:            nodetag.RangeStmt,
		NumArgs:        2,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opRangeKeyStmt: {
		Tag:            nodetag.RangeStmt,
		NumArgs:        3,
		ValueKind:      tokenValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opRangeKeyValueStmt: {
		Tag:            nodetag.RangeStmt,
		NumArgs:        4,
		ValueKind:      tokenValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opRangeClause: {
		Tag:            nodetag.RangeStmt,
		NumArgs:        1,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opRangeHeader: {
		Tag:            nodetag.RangeStmt,
		NumArgs:        1,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opRangeKeyHeader: {
		Tag:            nodetag.RangeStmt,
		NumArgs:        2,
		ValueKind:      tokenValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opRangeKeyValueHeader: {
		Tag:            nodetag.RangeStmt,
		NumArgs:        3,
		ValueKind:      tokenValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opFieldList: {
		Tag:            nodetag.Unknown,
		NumArgs:        1,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    1, // 1
		SliceIndex:     -1,
	},
	opUnnamedField: {
		Tag:            nodetag.Unknown,
		NumArgs:        1,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opSimpleField: {
		Tag:            nodetag.Unknown,
		NumArgs:        1,
		ValueKind:      emptyValue,
		ExtraValueKind: stringValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opField: {
		Tag:            nodetag.Unknown,
		NumArgs:        2,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opMultiField: {
		Tag:            nodetag.Unknown,
		NumArgs:        2,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    1, // 1
		SliceIndex:     -1,
	},
	opValueSpec: {
		Tag:            nodetag.ValueSpec,
		NumArgs:        1,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opValueInitSpec: {
		Tag:            nodetag.ValueSpec,
		NumArgs:        2,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    3, // 11
		SliceIndex:     -1,
	},
	opTypedValueInitSpec: {
		Tag:            nodetag.ValueSpec,
		NumArgs:        3,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    5, // 101
		SliceIndex:     -1,
	},
	opTypedValueSpec: {
		Tag:            nodetag.ValueSpec,
		NumArgs:        2,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    1, // 1
		SliceIndex:     -1,
	},
	opSimpleTypeSpec: {
		Tag:            nodetag.TypeSpec,
		NumArgs:        1,
		ValueKind:      emptyValue,
		ExtraValueKind: stringValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opTypeSpec: {
		Tag:            nodetag.TypeSpec,
		NumArgs:        2,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opGenericTypeSpec: {
		Tag:            nodetag.TypeSpec,
		NumArgs:        3,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opTypeAliasSpec: {
		Tag:            nodetag.TypeSpec,
		NumArgs:        2,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opSimpleFuncDecl: {
		Tag:            nodetag.FuncDecl,
		NumArgs:        2,
		ValueKind:      emptyValue,
		ExtraValueKind: stringValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opFuncDecl: {
		Tag:            nodetag.FuncDecl,
		NumArgs:        3,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opMethodDecl: {
		Tag:            nodetag.FuncDecl,
		NumArgs:        4,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opFuncProtoDecl: {
		Tag:            nodetag.FuncDecl,
		NumArgs:        2,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opMethodProtoDecl: {
		Tag:            nodetag.FuncDecl,
		NumArgs:        3,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opDeclStmt: {
		Tag:            nodetag.DeclStmt,
		NumArgs:        1,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opConstDecl: {
		Tag:            nodetag.GenDecl,
		NumArgs:        1,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    1, // 1
		SliceIndex:     -1,
	},
	opVarDecl: {
		Tag:            nodetag.GenDecl,
		NumArgs:        1,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    1, // 1
		SliceIndex:     -1,
	},
	opTypeDecl: {
		Tag:            nodetag.GenDecl,
		NumArgs:        1,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    1, // 1
		SliceIndex:     -1,
	},
	opAnyImportDecl: {
		Tag:            nodetag.GenDecl,
		NumArgs:        0,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
	opImportDecl: {
		Tag:            nodetag.GenDecl,
		NumArgs:        1,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    1, // 1
		SliceIndex:     -1,
	},
	opEmptyPackage: {
		Tag:            nodetag.File,
		NumArgs:        1,
		ValueKind:      emptyValue,
		ExtraValueKind: emptyValue,
		VariadicMap:    0, // 0
		SliceIndex:     -1,
	},
}
