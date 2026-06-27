package knowledge

import (
	"go/token"
	"go/types"
)

var Signatures = map[string]*types.Signature{
	"(io.Seeker).Seek": types.NewSignatureType(nil, nil, nil,
		types.NewTuple(
			types.NewParam(token.NoPos, nil, "", types.Typ[types.Int64]),
			types.NewParam(token.NoPos, nil, "", types.Typ[types.Int]),
		),
		types.NewTuple(
			types.NewParam(token.NoPos, nil, "", types.Typ[types.Int64]),
			types.NewParam(token.NoPos, nil, "", types.Universe.Lookup("error").Type()),
		),
		false,
	),

	"(io.Writer).Write": types.NewSignatureType(nil, nil, nil,
		types.NewTuple(
			types.NewParam(token.NoPos, nil, "", types.NewSlice(types.Typ[types.Byte])),
		),
		types.NewTuple(
			types.NewParam(token.NoPos, nil, "", types.Typ[types.Int]),
			types.NewParam(token.NoPos, nil, "", types.Universe.Lookup("error").Type()),
		),
		false,
	),

	"(io.StringWriter).WriteString": types.NewSignatureType(nil, nil, nil,
		types.NewTuple(
			types.NewParam(token.NoPos, nil, "", types.Typ[types.String]),
		),
		types.NewTuple(
			types.NewParam(token.NoPos, nil, "", types.Typ[types.Int]),
			types.NewParam(token.NoPos, nil, "", types.Universe.Lookup("error").Type()),
		),
		false,
	),

	"(encoding.TextMarshaler).MarshalText": types.NewSignatureType(nil, nil, nil,
		types.NewTuple(),
		types.NewTuple(
			types.NewParam(token.NoPos, nil, "", types.NewSlice(types.Typ[types.Byte])),
			types.NewParam(token.NoPos, nil, "", types.Universe.Lookup("error").Type()),
		),
		false,
	),

	"(encoding/json.Marshaler).MarshalJSON": types.NewSignatureType(nil, nil, nil,
		types.NewTuple(),
		types.NewTuple(
			types.NewParam(token.NoPos, nil, "", types.NewSlice(types.Typ[types.Byte])),
			types.NewParam(token.NoPos, nil, "", types.Universe.Lookup("error").Type()),
		),
		false,
	),

	"(fmt.Stringer).String": types.NewSignatureType(nil, nil, nil,
		types.NewTuple(),
		types.NewTuple(
			types.NewParam(token.NoPos, nil, "", types.Typ[types.String]),
		),
		false,
	),
}

var Interfaces = map[string]*types.Interface{
	"fmt.Stringer": types.NewInterfaceType(
		[]*types.Func{
			types.NewFunc(token.NoPos, nil, "String", Signatures["(fmt.Stringer).String"]),
		},
		nil,
	).Complete(),

	"error": types.Universe.Lookup("error").Type().Underlying().(*types.Interface),

	"io.Writer": types.NewInterfaceType(
		[]*types.Func{
			types.NewFunc(token.NoPos, nil, "Write", Signatures["(io.Writer).Write"]),
		},
		nil,
	).Complete(),

	"io.StringWriter": types.NewInterfaceType(
		[]*types.Func{
			types.NewFunc(token.NoPos, nil, "WriteString", Signatures["(io.StringWriter).WriteString"]),
		},
		nil,
	).Complete(),

	"encoding.TextMarshaler": types.NewInterfaceType(
		[]*types.Func{
			types.NewFunc(token.NoPos, nil, "MarshalText", Signatures["(encoding.TextMarshaler).MarshalText"]),
		},
		nil,
	).Complete(),

	"encoding/json.Marshaler": types.NewInterfaceType(
		[]*types.Func{
			types.NewFunc(token.NoPos, nil, "MarshalJSON", Signatures["(encoding/json.Marshaler).MarshalJSON"]),
		},
		nil,
	).Complete(),
}
