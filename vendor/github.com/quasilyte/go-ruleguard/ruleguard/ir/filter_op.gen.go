// Code generated "gen_filter_op.go"; DO NOT EDIT.

package ir

const (
	FilterInvalidOp FilterOp = 0

	// !$Args[0]
	FilterNotOp FilterOp = 1

	// $Args[0] && $Args[1]
	FilterAndOp FilterOp = 2

	// $Args[0] || $Args[1]
	FilterOrOp FilterOp = 3

	// $Args[0] == $Args[1]
	FilterEqOp FilterOp = 4

	// $Args[0] != $Args[1]
	FilterNeqOp FilterOp = 5

	// $Args[0] > $Args[1]
	FilterGtOp FilterOp = 6

	// $Args[0] < $Args[1]
	FilterLtOp FilterOp = 7

	// $Args[0] >= $Args[1]
	FilterGtEqOp FilterOp = 8

	// $Args[0] <= $Args[1]
	FilterLtEqOp FilterOp = 9

	// m[$Value].Addressable
	// $Value type: string
	FilterVarAddressableOp FilterOp = 10

	// m[$Value].Comparable
	// $Value type: string
	FilterVarComparableOp FilterOp = 11

	// m[$Value].Pure
	// $Value type: string
	FilterVarPureOp FilterOp = 12

	// m[$Value].Const
	// $Value type: string
	FilterVarConstOp FilterOp = 13

	// m[$Value].ConstSlice
	// $Value type: string
	FilterVarConstSliceOp FilterOp = 14

	// m[$Value].Text
	// $Value type: string
	FilterVarTextOp FilterOp = 15

	// m[$Value].Line
	// $Value type: string
	FilterVarLineOp FilterOp = 16

	// m[$Value].Value.Int()
	// $Value type: string
	FilterVarValueIntOp FilterOp = 17

	// m[$Value].Type.Size
	// $Value type: string
	FilterVarTypeSizeOp FilterOp = 18

	// m[$Value].Type.HasPointers()
	// $Value type: string
	FilterVarTypeHasPointersOp FilterOp = 19

	// m[$Value].Filter($Args[0])
	// $Value type: string
	FilterVarFilterOp FilterOp = 20

	// m[$Value].Node.Is($Args[0])
	// $Value type: string
	FilterVarNodeIsOp FilterOp = 21

	// m[$Value].Object.Is($Args[0])
	// $Value type: string
	FilterVarObjectIsOp FilterOp = 22

	// m[$Value].Object.IsGlobal()
	// $Value type: string
	FilterVarObjectIsGlobalOp FilterOp = 23

	// m[$Value].Object.IsVariadicParam()
	// $Value type: string
	FilterVarObjectIsVariadicParamOp FilterOp = 24

	// m[$Value].Type.Is($Args[0])
	// $Value type: string
	FilterVarTypeIsOp FilterOp = 25

	// m[$Value].Type.IdenticalTo($Args[0])
	// $Value type: string
	FilterVarTypeIdenticalToOp FilterOp = 26

	// m[$Value].Type.Underlying().Is($Args[0])
	// $Value type: string
	FilterVarTypeUnderlyingIsOp FilterOp = 27

	// m[$Value].Type.OfKind($Args[0])
	// $Value type: string
	FilterVarTypeOfKindOp FilterOp = 28

	// m[$Value].Type.Underlying().OfKind($Args[0])
	// $Value type: string
	FilterVarTypeUnderlyingOfKindOp FilterOp = 29

	// m[$Value].Type.ConvertibleTo($Args[0])
	// $Value type: string
	FilterVarTypeConvertibleToOp FilterOp = 30

	// m[$Value].Type.AssignableTo($Args[0])
	// $Value type: string
	FilterVarTypeAssignableToOp FilterOp = 31

	// m[$Value].Type.Implements($Args[0])
	// $Value type: string
	FilterVarTypeImplementsOp FilterOp = 32

	// m[$Value].Type.HasMethod($Args[0])
	// $Value type: string
	FilterVarTypeHasMethodOp FilterOp = 33

	// m[$Value].Text.Matches($Args[0])
	// $Value type: string
	FilterVarTextMatchesOp FilterOp = 34

	// m[$Value].Contains($Args[0])
	// $Value type: string
	FilterVarContainsOp FilterOp = 35

	// m.Deadcode()
	FilterDeadcodeOp FilterOp = 36

	// m.GoVersion().Eq($Value)
	// $Value type: string
	FilterGoVersionEqOp FilterOp = 37

	// m.GoVersion().LessThan($Value)
	// $Value type: string
	FilterGoVersionLessThanOp FilterOp = 38

	// m.GoVersion().GreaterThan($Value)
	// $Value type: string
	FilterGoVersionGreaterThanOp FilterOp = 39

	// m.GoVersion().LessEqThan($Value)
	// $Value type: string
	FilterGoVersionLessEqThanOp FilterOp = 40

	// m.GoVersion().GreaterEqThan($Value)
	// $Value type: string
	FilterGoVersionGreaterEqThanOp FilterOp = 41

	// m.File.Imports($Value)
	// $Value type: string
	FilterFileImportsOp FilterOp = 42

	// m.File.PkgPath.Matches($Value)
	// $Value type: string
	FilterFilePkgPathMatchesOp FilterOp = 43

	// m.File.Name.Matches($Value)
	// $Value type: string
	FilterFileNameMatchesOp FilterOp = 44

	// $Value holds a function name
	// $Value type: string
	FilterFilterFuncRefOp FilterOp = 45

	// $Value holds a string constant
	// $Value type: string
	FilterStringOp FilterOp = 46

	// $Value holds an int64 constant
	// $Value type: int64
	FilterIntOp FilterOp = 47

	// m[`$$`].Node.Parent().Is($Args[0])
	FilterRootNodeParentIsOp FilterOp = 48

	// m[`$$`].SinkType.Is($Args[0])
	FilterRootSinkTypeIsOp FilterOp = 49
)

var filterOpNames = map[FilterOp]string{
	FilterInvalidOp:                  `Invalid`,
	FilterNotOp:                      `Not`,
	FilterAndOp:                      `And`,
	FilterOrOp:                       `Or`,
	FilterEqOp:                       `Eq`,
	FilterNeqOp:                      `Neq`,
	FilterGtOp:                       `Gt`,
	FilterLtOp:                       `Lt`,
	FilterGtEqOp:                     `GtEq`,
	FilterLtEqOp:                     `LtEq`,
	FilterVarAddressableOp:           `VarAddressable`,
	FilterVarComparableOp:            `VarComparable`,
	FilterVarPureOp:                  `VarPure`,
	FilterVarConstOp:                 `VarConst`,
	FilterVarConstSliceOp:            `VarConstSlice`,
	FilterVarTextOp:                  `VarText`,
	FilterVarLineOp:                  `VarLine`,
	FilterVarValueIntOp:              `VarValueInt`,
	FilterVarTypeSizeOp:              `VarTypeSize`,
	FilterVarTypeHasPointersOp:       `VarTypeHasPointers`,
	FilterVarFilterOp:                `VarFilter`,
	FilterVarNodeIsOp:                `VarNodeIs`,
	FilterVarObjectIsOp:              `VarObjectIs`,
	FilterVarObjectIsGlobalOp:        `VarObjectIsGlobal`,
	FilterVarObjectIsVariadicParamOp: `VarObjectIsVariadicParam`,
	FilterVarTypeIsOp:                `VarTypeIs`,
	FilterVarTypeIdenticalToOp:       `VarTypeIdenticalTo`,
	FilterVarTypeUnderlyingIsOp:      `VarTypeUnderlyingIs`,
	FilterVarTypeOfKindOp:            `VarTypeOfKind`,
	FilterVarTypeUnderlyingOfKindOp:  `VarTypeUnderlyingOfKind`,
	FilterVarTypeConvertibleToOp:     `VarTypeConvertibleTo`,
	FilterVarTypeAssignableToOp:      `VarTypeAssignableTo`,
	FilterVarTypeImplementsOp:        `VarTypeImplements`,
	FilterVarTypeHasMethodOp:         `VarTypeHasMethod`,
	FilterVarTextMatchesOp:           `VarTextMatches`,
	FilterVarContainsOp:              `VarContains`,
	FilterDeadcodeOp:                 `Deadcode`,
	FilterGoVersionEqOp:              `GoVersionEq`,
	FilterGoVersionLessThanOp:        `GoVersionLessThan`,
	FilterGoVersionGreaterThanOp:     `GoVersionGreaterThan`,
	FilterGoVersionLessEqThanOp:      `GoVersionLessEqThan`,
	FilterGoVersionGreaterEqThanOp:   `GoVersionGreaterEqThan`,
	FilterFileImportsOp:              `FileImports`,
	FilterFilePkgPathMatchesOp:       `FilePkgPathMatches`,
	FilterFileNameMatchesOp:          `FileNameMatches`,
	FilterFilterFuncRefOp:            `FilterFuncRef`,
	FilterStringOp:                   `String`,
	FilterIntOp:                      `Int`,
	FilterRootNodeParentIsOp:         `RootNodeParentIs`,
	FilterRootSinkTypeIsOp:           `RootSinkTypeIs`,
}
var filterOpFlags = map[FilterOp]uint64{
	FilterAndOp:                      flagIsBinaryExpr,
	FilterOrOp:                       flagIsBinaryExpr,
	FilterEqOp:                       flagIsBinaryExpr,
	FilterNeqOp:                      flagIsBinaryExpr,
	FilterGtOp:                       flagIsBinaryExpr,
	FilterLtOp:                       flagIsBinaryExpr,
	FilterGtEqOp:                     flagIsBinaryExpr,
	FilterLtEqOp:                     flagIsBinaryExpr,
	FilterVarAddressableOp:           flagHasVar,
	FilterVarComparableOp:            flagHasVar,
	FilterVarPureOp:                  flagHasVar,
	FilterVarConstOp:                 flagHasVar,
	FilterVarConstSliceOp:            flagHasVar,
	FilterVarTextOp:                  flagHasVar,
	FilterVarLineOp:                  flagHasVar,
	FilterVarValueIntOp:              flagHasVar,
	FilterVarTypeSizeOp:              flagHasVar,
	FilterVarTypeHasPointersOp:       flagHasVar,
	FilterVarFilterOp:                flagHasVar,
	FilterVarNodeIsOp:                flagHasVar,
	FilterVarObjectIsOp:              flagHasVar,
	FilterVarObjectIsGlobalOp:        flagHasVar,
	FilterVarObjectIsVariadicParamOp: flagHasVar,
	FilterVarTypeIsOp:                flagHasVar,
	FilterVarTypeIdenticalToOp:       flagHasVar,
	FilterVarTypeUnderlyingIsOp:      flagHasVar,
	FilterVarTypeOfKindOp:            flagHasVar,
	FilterVarTypeUnderlyingOfKindOp:  flagHasVar,
	FilterVarTypeConvertibleToOp:     flagHasVar,
	FilterVarTypeAssignableToOp:      flagHasVar,
	FilterVarTypeImplementsOp:        flagHasVar,
	FilterVarTypeHasMethodOp:         flagHasVar,
	FilterVarTextMatchesOp:           flagHasVar,
	FilterVarContainsOp:              flagHasVar,
	FilterStringOp:                   flagIsBasicLit,
	FilterIntOp:                      flagIsBasicLit,
}
