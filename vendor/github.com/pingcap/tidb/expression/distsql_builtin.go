// Copyright 2017 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package expression

import (
	"fmt"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/parser/model"
	"github.com/pingcap/parser/mysql"
	"github.com/pingcap/tidb/sessionctx"
	"github.com/pingcap/tidb/sessionctx/stmtctx"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/util/codec"
	"github.com/pingcap/tidb/util/mock"
	"github.com/pingcap/tipb/go-tipb"
)

func pbTypeToFieldType(tp *tipb.FieldType) *types.FieldType {
	return &types.FieldType{
		Tp:      byte(tp.Tp),
		Flag:    uint(tp.Flag),
		Flen:    int(tp.Flen),
		Decimal: int(tp.Decimal),
		Charset: tp.Charset,
		Collate: mysql.Collations[uint8(tp.Collate)],
	}
}

func getSignatureByPB(ctx sessionctx.Context, sigCode tipb.ScalarFuncSig, tp *tipb.FieldType, args []Expression) (f builtinFunc, e error) {
	fieldTp := pbTypeToFieldType(tp)
	base := newBaseBuiltinFunc(ctx, args)
	base.tp = fieldTp
	switch sigCode {
	case tipb.ScalarFuncSig_CastIntAsInt:
		f = &builtinCastIntAsIntSig{newBaseBuiltinCastFunc(base, false)}
	case tipb.ScalarFuncSig_CastRealAsInt:
		f = &builtinCastRealAsIntSig{newBaseBuiltinCastFunc(base, false)}
	case tipb.ScalarFuncSig_CastDecimalAsInt:
		f = &builtinCastDecimalAsIntSig{newBaseBuiltinCastFunc(base, false)}
	case tipb.ScalarFuncSig_CastDurationAsInt:
		f = &builtinCastDurationAsIntSig{newBaseBuiltinCastFunc(base, false)}
	case tipb.ScalarFuncSig_CastTimeAsInt:
		f = &builtinCastTimeAsIntSig{newBaseBuiltinCastFunc(base, false)}
	case tipb.ScalarFuncSig_CastStringAsInt:
		f = &builtinCastStringAsIntSig{newBaseBuiltinCastFunc(base, false)}
	case tipb.ScalarFuncSig_CastJsonAsInt:
		f = &builtinCastJSONAsIntSig{newBaseBuiltinCastFunc(base, false)}

	case tipb.ScalarFuncSig_CastIntAsReal:
		f = &builtinCastIntAsRealSig{newBaseBuiltinCastFunc(base, false)}
	case tipb.ScalarFuncSig_CastRealAsReal:
		f = &builtinCastRealAsRealSig{newBaseBuiltinCastFunc(base, false)}
	case tipb.ScalarFuncSig_CastDecimalAsReal:
		f = &builtinCastDecimalAsRealSig{newBaseBuiltinCastFunc(base, false)}
	case tipb.ScalarFuncSig_CastDurationAsReal:
		f = &builtinCastDurationAsRealSig{newBaseBuiltinCastFunc(base, false)}
	case tipb.ScalarFuncSig_CastTimeAsReal:
		f = &builtinCastTimeAsRealSig{newBaseBuiltinCastFunc(base, false)}
	case tipb.ScalarFuncSig_CastStringAsReal:
		f = &builtinCastStringAsRealSig{newBaseBuiltinCastFunc(base, false)}
	case tipb.ScalarFuncSig_CastJsonAsReal:
		f = &builtinCastJSONAsRealSig{newBaseBuiltinCastFunc(base, false)}

	case tipb.ScalarFuncSig_CastIntAsDecimal:
		f = &builtinCastIntAsDecimalSig{newBaseBuiltinCastFunc(base, false)}
	case tipb.ScalarFuncSig_CastRealAsDecimal:
		f = &builtinCastRealAsDecimalSig{newBaseBuiltinCastFunc(base, false)}
	case tipb.ScalarFuncSig_CastDecimalAsDecimal:
		f = &builtinCastDecimalAsDecimalSig{newBaseBuiltinCastFunc(base, false)}
	case tipb.ScalarFuncSig_CastDurationAsDecimal:
		f = &builtinCastDurationAsDecimalSig{newBaseBuiltinCastFunc(base, false)}
	case tipb.ScalarFuncSig_CastTimeAsDecimal:
		f = &builtinCastTimeAsDecimalSig{newBaseBuiltinCastFunc(base, false)}
	case tipb.ScalarFuncSig_CastStringAsDecimal:
		f = &builtinCastStringAsDecimalSig{newBaseBuiltinCastFunc(base, false)}
	case tipb.ScalarFuncSig_CastJsonAsDecimal:
		f = &builtinCastJSONAsDecimalSig{newBaseBuiltinCastFunc(base, false)}

	case tipb.ScalarFuncSig_CastIntAsTime:
		f = &builtinCastIntAsTimeSig{base}
	case tipb.ScalarFuncSig_CastRealAsTime:
		f = &builtinCastRealAsTimeSig{base}
	case tipb.ScalarFuncSig_CastDecimalAsTime:
		f = &builtinCastDecimalAsTimeSig{base}
	case tipb.ScalarFuncSig_CastDurationAsTime:
		f = &builtinCastDurationAsTimeSig{base}
	case tipb.ScalarFuncSig_CastTimeAsTime:
		f = &builtinCastTimeAsTimeSig{base}
	case tipb.ScalarFuncSig_CastStringAsTime:
		f = &builtinCastStringAsTimeSig{base}
	case tipb.ScalarFuncSig_CastJsonAsTime:
		f = &builtinCastJSONAsTimeSig{base}

	case tipb.ScalarFuncSig_CastIntAsString:
		f = &builtinCastIntAsStringSig{base}
	case tipb.ScalarFuncSig_CastRealAsString:
		f = &builtinCastRealAsStringSig{base}
	case tipb.ScalarFuncSig_CastDecimalAsString:
		f = &builtinCastDecimalAsStringSig{base}
	case tipb.ScalarFuncSig_CastDurationAsString:
		f = &builtinCastDurationAsStringSig{base}
	case tipb.ScalarFuncSig_CastTimeAsString:
		f = &builtinCastTimeAsStringSig{base}
	case tipb.ScalarFuncSig_CastStringAsString:
		f = &builtinCastStringAsStringSig{base}
	case tipb.ScalarFuncSig_CastJsonAsString:
		f = &builtinCastJSONAsStringSig{base}

	case tipb.ScalarFuncSig_CastIntAsDuration:
		f = &builtinCastIntAsDurationSig{base}
	case tipb.ScalarFuncSig_CastRealAsDuration:
		f = &builtinCastRealAsDurationSig{base}
	case tipb.ScalarFuncSig_CastDecimalAsDuration:
		f = &builtinCastDecimalAsDurationSig{base}
	case tipb.ScalarFuncSig_CastDurationAsDuration:
		f = &builtinCastDurationAsDurationSig{base}
	case tipb.ScalarFuncSig_CastTimeAsDuration:
		f = &builtinCastTimeAsDurationSig{base}
	case tipb.ScalarFuncSig_CastStringAsDuration:
		f = &builtinCastStringAsDurationSig{base}
	case tipb.ScalarFuncSig_CastJsonAsDuration:
		f = &builtinCastJSONAsDurationSig{base}

	case tipb.ScalarFuncSig_CastIntAsJson:
		f = &builtinCastIntAsJSONSig{base}
	case tipb.ScalarFuncSig_CastRealAsJson:
		f = &builtinCastRealAsJSONSig{base}
	case tipb.ScalarFuncSig_CastDecimalAsJson:
		f = &builtinCastDecimalAsJSONSig{base}
	case tipb.ScalarFuncSig_CastTimeAsJson:
		f = &builtinCastTimeAsJSONSig{base}
	case tipb.ScalarFuncSig_CastDurationAsJson:
		f = &builtinCastDurationAsJSONSig{base}
	case tipb.ScalarFuncSig_CastStringAsJson:
		f = &builtinCastStringAsJSONSig{base}
	case tipb.ScalarFuncSig_CastJsonAsJson:
		f = &builtinCastJSONAsJSONSig{base}

	case tipb.ScalarFuncSig_GTInt:
		f = &builtinGTIntSig{base}
	case tipb.ScalarFuncSig_GEInt:
		f = &builtinGEIntSig{base}
	case tipb.ScalarFuncSig_LTInt:
		f = &builtinLTIntSig{base}
	case tipb.ScalarFuncSig_LEInt:
		f = &builtinLEIntSig{base}
	case tipb.ScalarFuncSig_EQInt:
		f = &builtinEQIntSig{base}
	case tipb.ScalarFuncSig_NEInt:
		f = &builtinNEIntSig{base}
	case tipb.ScalarFuncSig_NullEQInt:
		f = &builtinNullEQIntSig{base}

	case tipb.ScalarFuncSig_GTReal:
		f = &builtinGTRealSig{base}
	case tipb.ScalarFuncSig_GEReal:
		f = &builtinGERealSig{base}
	case tipb.ScalarFuncSig_LTReal:
		f = &builtinLTRealSig{base}
	case tipb.ScalarFuncSig_LEReal:
		f = &builtinLERealSig{base}
	case tipb.ScalarFuncSig_EQReal:
		f = &builtinEQRealSig{base}
	case tipb.ScalarFuncSig_NEReal:
		f = &builtinNERealSig{base}
	case tipb.ScalarFuncSig_NullEQReal:
		f = &builtinNullEQRealSig{base}

	case tipb.ScalarFuncSig_GTDecimal:
		f = &builtinGTDecimalSig{base}
	case tipb.ScalarFuncSig_GEDecimal:
		f = &builtinGEDecimalSig{base}
	case tipb.ScalarFuncSig_LTDecimal:
		f = &builtinLTDecimalSig{base}
	case tipb.ScalarFuncSig_LEDecimal:
		f = &builtinLEDecimalSig{base}
	case tipb.ScalarFuncSig_EQDecimal:
		f = &builtinEQDecimalSig{base}
	case tipb.ScalarFuncSig_NEDecimal:
		f = &builtinNEDecimalSig{base}
	case tipb.ScalarFuncSig_NullEQDecimal:
		f = &builtinNullEQDecimalSig{base}

	case tipb.ScalarFuncSig_GTTime:
		f = &builtinGTTimeSig{base}
	case tipb.ScalarFuncSig_GETime:
		f = &builtinGETimeSig{base}
	case tipb.ScalarFuncSig_LTTime:
		f = &builtinLTTimeSig{base}
	case tipb.ScalarFuncSig_LETime:
		f = &builtinLETimeSig{base}
	case tipb.ScalarFuncSig_EQTime:
		f = &builtinEQTimeSig{base}
	case tipb.ScalarFuncSig_NETime:
		f = &builtinNETimeSig{base}
	case tipb.ScalarFuncSig_NullEQTime:
		f = &builtinNullEQTimeSig{base}

	case tipb.ScalarFuncSig_GTDuration:
		f = &builtinGTDurationSig{base}
	case tipb.ScalarFuncSig_GEDuration:
		f = &builtinGEDurationSig{base}
	case tipb.ScalarFuncSig_LTDuration:
		f = &builtinLTDurationSig{base}
	case tipb.ScalarFuncSig_LEDuration:
		f = &builtinLEDurationSig{base}
	case tipb.ScalarFuncSig_EQDuration:
		f = &builtinEQDurationSig{base}
	case tipb.ScalarFuncSig_NEDuration:
		f = &builtinNEDurationSig{base}
	case tipb.ScalarFuncSig_NullEQDuration:
		f = &builtinNullEQDurationSig{base}

	case tipb.ScalarFuncSig_GTString:
		f = &builtinGTStringSig{base}
	case tipb.ScalarFuncSig_GEString:
		f = &builtinGEStringSig{base}
	case tipb.ScalarFuncSig_LTString:
		f = &builtinLTStringSig{base}
	case tipb.ScalarFuncSig_LEString:
		f = &builtinLEStringSig{base}
	case tipb.ScalarFuncSig_EQString:
		f = &builtinEQStringSig{base}
	case tipb.ScalarFuncSig_NEString:
		f = &builtinNEStringSig{base}
	case tipb.ScalarFuncSig_NullEQString:
		f = &builtinNullEQStringSig{base}

	case tipb.ScalarFuncSig_GTJson:
		f = &builtinGTJSONSig{base}
	case tipb.ScalarFuncSig_GEJson:
		f = &builtinGEJSONSig{base}
	case tipb.ScalarFuncSig_LTJson:
		f = &builtinLTJSONSig{base}
	case tipb.ScalarFuncSig_LEJson:
		f = &builtinLEJSONSig{base}
	case tipb.ScalarFuncSig_EQJson:
		f = &builtinEQJSONSig{base}
	case tipb.ScalarFuncSig_NEJson:
		f = &builtinNEJSONSig{base}
	case tipb.ScalarFuncSig_NullEQJson:
		f = &builtinNullEQJSONSig{base}

	case tipb.ScalarFuncSig_PlusInt:
		f = &builtinArithmeticPlusIntSig{base}
	case tipb.ScalarFuncSig_PlusDecimal:
		f = &builtinArithmeticPlusDecimalSig{base}
	case tipb.ScalarFuncSig_PlusReal:
		f = &builtinArithmeticPlusRealSig{base}
	case tipb.ScalarFuncSig_MinusInt:
		f = &builtinArithmeticMinusIntSig{base}
	case tipb.ScalarFuncSig_MinusDecimal:
		f = &builtinArithmeticMinusDecimalSig{base}
	case tipb.ScalarFuncSig_MinusReal:
		f = &builtinArithmeticMinusRealSig{base}
	case tipb.ScalarFuncSig_MultiplyInt:
		f = &builtinArithmeticMultiplyIntSig{base}
	case tipb.ScalarFuncSig_MultiplyDecimal:
		f = &builtinArithmeticMultiplyDecimalSig{base}
	case tipb.ScalarFuncSig_MultiplyReal:
		f = &builtinArithmeticMultiplyRealSig{base}
	case tipb.ScalarFuncSig_DivideDecimal:
		f = &builtinArithmeticDivideDecimalSig{base}
	case tipb.ScalarFuncSig_DivideReal:
		f = &builtinArithmeticDivideRealSig{base}
	case tipb.ScalarFuncSig_AbsInt:
		f = &builtinAbsIntSig{base}
	case tipb.ScalarFuncSig_AbsUInt:
		f = &builtinAbsUIntSig{base}
	case tipb.ScalarFuncSig_AbsReal:
		f = &builtinAbsRealSig{base}
	case tipb.ScalarFuncSig_AbsDecimal:
		f = &builtinAbsDecSig{base}
	case tipb.ScalarFuncSig_CeilIntToInt:
		f = &builtinCeilIntToIntSig{base}
	case tipb.ScalarFuncSig_CeilIntToDec:
		f = &builtinCeilIntToDecSig{base}
	case tipb.ScalarFuncSig_CeilDecToInt:
		f = &builtinCeilDecToIntSig{base}
	case tipb.ScalarFuncSig_CeilDecToDec:
		f = &builtinCeilDecToDecSig{base}
	case tipb.ScalarFuncSig_CeilReal:
		f = &builtinCeilRealSig{base}
	case tipb.ScalarFuncSig_FloorIntToInt:
		f = &builtinFloorIntToIntSig{base}
	case tipb.ScalarFuncSig_FloorIntToDec:
		f = &builtinFloorIntToDecSig{base}
	case tipb.ScalarFuncSig_FloorDecToInt:
		f = &builtinFloorDecToIntSig{base}
	case tipb.ScalarFuncSig_FloorDecToDec:
		f = &builtinFloorDecToDecSig{base}
	case tipb.ScalarFuncSig_FloorReal:
		f = &builtinFloorRealSig{base}

	case tipb.ScalarFuncSig_LogicalAnd:
		f = &builtinLogicAndSig{base}
	case tipb.ScalarFuncSig_LogicalOr:
		f = &builtinLogicOrSig{base}
	case tipb.ScalarFuncSig_LogicalXor:
		f = &builtinLogicXorSig{base}
	case tipb.ScalarFuncSig_BitAndSig:
		f = &builtinBitAndSig{base}
	case tipb.ScalarFuncSig_BitOrSig:
		f = &builtinBitOrSig{base}
	case tipb.ScalarFuncSig_BitXorSig:
		f = &builtinBitXorSig{base}
	case tipb.ScalarFuncSig_BitNegSig:
		f = &builtinBitNegSig{base}

	case tipb.ScalarFuncSig_UnaryNot:
		f = &builtinUnaryNotSig{base}
	case tipb.ScalarFuncSig_UnaryMinusInt:
		f = &builtinUnaryMinusIntSig{base}
	case tipb.ScalarFuncSig_UnaryMinusReal:
		f = &builtinUnaryMinusRealSig{base}
	case tipb.ScalarFuncSig_UnaryMinusDecimal:
		f = &builtinUnaryMinusDecimalSig{base, false}

	case tipb.ScalarFuncSig_DecimalIsNull:
		f = &builtinDecimalIsNullSig{base}
	case tipb.ScalarFuncSig_DurationIsNull:
		f = &builtinDurationIsNullSig{base}
	case tipb.ScalarFuncSig_RealIsNull:
		f = &builtinRealIsNullSig{base}
	case tipb.ScalarFuncSig_TimeIsNull:
		f = &builtinTimeIsNullSig{base}
	case tipb.ScalarFuncSig_StringIsNull:
		f = &builtinStringIsNullSig{base}
	case tipb.ScalarFuncSig_IntIsNull:
		f = &builtinIntIsNullSig{base}

	case tipb.ScalarFuncSig_CoalesceDecimal:
		f = &builtinCoalesceDecimalSig{base}
	case tipb.ScalarFuncSig_CoalesceDuration:
		f = &builtinCoalesceDurationSig{base}
	case tipb.ScalarFuncSig_CoalesceReal:
		f = &builtinCoalesceRealSig{base}
	case tipb.ScalarFuncSig_CoalesceTime:
		f = &builtinCoalesceTimeSig{base}
	case tipb.ScalarFuncSig_CoalesceString:
		f = &builtinCoalesceStringSig{base}
	case tipb.ScalarFuncSig_CoalesceInt:
		f = &builtinCoalesceIntSig{base}

	case tipb.ScalarFuncSig_CaseWhenDecimal:
		f = &builtinCaseWhenDecimalSig{base}
	case tipb.ScalarFuncSig_CaseWhenDuration:
		f = &builtinCaseWhenDurationSig{base}
	case tipb.ScalarFuncSig_CaseWhenReal:
		f = &builtinCaseWhenRealSig{base}
	case tipb.ScalarFuncSig_CaseWhenTime:
		f = &builtinCaseWhenTimeSig{base}
	case tipb.ScalarFuncSig_CaseWhenString:
		f = &builtinCaseWhenStringSig{base}
	case tipb.ScalarFuncSig_CaseWhenInt:
		f = &builtinCaseWhenIntSig{base}

	case tipb.ScalarFuncSig_IntIsFalse:
		f = &builtinIntIsFalseSig{base}
	case tipb.ScalarFuncSig_RealIsFalse:
		f = &builtinRealIsFalseSig{base}
	case tipb.ScalarFuncSig_DecimalIsFalse:
		f = &builtinDecimalIsFalseSig{base}
	case tipb.ScalarFuncSig_IntIsTrue:
		f = &builtinIntIsTrueSig{base}
	case tipb.ScalarFuncSig_RealIsTrue:
		f = &builtinRealIsTrueSig{base}
	case tipb.ScalarFuncSig_DecimalIsTrue:
		f = &builtinDecimalIsTrueSig{base}

	case tipb.ScalarFuncSig_IfNullReal:
		f = &builtinIfNullRealSig{base}
	case tipb.ScalarFuncSig_IfNullInt:
		f = &builtinIfNullIntSig{base}
	case tipb.ScalarFuncSig_IfNullDecimal:
		f = &builtinIfNullDecimalSig{base}
	case tipb.ScalarFuncSig_IfNullString:
		f = &builtinIfNullStringSig{base}
	case tipb.ScalarFuncSig_IfNullTime:
		f = &builtinIfNullTimeSig{base}
	case tipb.ScalarFuncSig_IfNullDuration:
		f = &builtinIfNullDurationSig{base}
	case tipb.ScalarFuncSig_IfNullJson:
		f = &builtinIfNullJSONSig{base}
	case tipb.ScalarFuncSig_IfReal:
		f = &builtinIfRealSig{base}
	case tipb.ScalarFuncSig_IfInt:
		f = &builtinIfIntSig{base}
	case tipb.ScalarFuncSig_IfDecimal:
		f = &builtinIfDecimalSig{base}
	case tipb.ScalarFuncSig_IfString:
		f = &builtinIfStringSig{base}
	case tipb.ScalarFuncSig_IfTime:
		f = &builtinIfTimeSig{base}
	case tipb.ScalarFuncSig_IfDuration:
		f = &builtinIfDurationSig{base}
	case tipb.ScalarFuncSig_IfJson:
		f = &builtinIfJSONSig{base}

	case tipb.ScalarFuncSig_JsonTypeSig:
		f = &builtinJSONTypeSig{base}
	case tipb.ScalarFuncSig_JsonUnquoteSig:
		f = &builtinJSONUnquoteSig{base}
	case tipb.ScalarFuncSig_JsonArraySig:
		f = &builtinJSONArraySig{base}
	case tipb.ScalarFuncSig_JsonObjectSig:
		f = &builtinJSONObjectSig{base}
	case tipb.ScalarFuncSig_JsonExtractSig:
		f = &builtinJSONExtractSig{base}
	case tipb.ScalarFuncSig_JsonSetSig:
		f = &builtinJSONSetSig{base}
	case tipb.ScalarFuncSig_JsonInsertSig:
		f = &builtinJSONInsertSig{base}
	case tipb.ScalarFuncSig_JsonReplaceSig:
		f = &builtinJSONReplaceSig{base}
	case tipb.ScalarFuncSig_JsonRemoveSig:
		f = &builtinJSONRemoveSig{base}
	case tipb.ScalarFuncSig_JsonMergeSig:
		f = &builtinJSONMergeSig{base}
	case tipb.ScalarFuncSig_JsonContainsSig:
		f = &builtinJSONContainsSig{base}
	case tipb.ScalarFuncSig_LikeSig:
		f = &builtinLikeSig{base}
	case tipb.ScalarFuncSig_JsonLengthSig:
		f = &builtinJSONLengthSig{base}

	case tipb.ScalarFuncSig_InInt:
		f = &builtinInIntSig{base}
	case tipb.ScalarFuncSig_InReal:
		f = &builtinInRealSig{base}
	case tipb.ScalarFuncSig_InDecimal:
		f = &builtinInDecimalSig{base}
	case tipb.ScalarFuncSig_InString:
		f = &builtinInStringSig{base}
	case tipb.ScalarFuncSig_InTime:
		f = &builtinInTimeSig{base}
	case tipb.ScalarFuncSig_InDuration:
		f = &builtinInDurationSig{base}
	case tipb.ScalarFuncSig_InJson:
		f = &builtinInJSONSig{base}

	case tipb.ScalarFuncSig_DateFormatSig:
		f = &builtinDateFormatSig{base}

	default:
		e = errFunctionNotExists.GenWithStackByArgs("FUNCTION", sigCode)
		return nil, errors.Trace(e)
	}
	return f, nil
}

func newDistSQLFunctionBySig(sc *stmtctx.StatementContext, sigCode tipb.ScalarFuncSig, tp *tipb.FieldType, args []Expression) (Expression, error) {
	ctx := mock.NewContext()
	ctx.GetSessionVars().StmtCtx = sc
	f, err := getSignatureByPB(ctx, sigCode, tp, args)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return &ScalarFunction{
		FuncName: model.NewCIStr(fmt.Sprintf("sig_%T", f)),
		Function: f,
		RetType:  f.getRetTp(),
	}, nil
}

// PBToExpr converts pb structure to expression.
func PBToExpr(expr *tipb.Expr, tps []*types.FieldType, sc *stmtctx.StatementContext) (Expression, error) {
	switch expr.Tp {
	case tipb.ExprType_ColumnRef:
		_, offset, err := codec.DecodeInt(expr.Val)
		if err != nil {
			return nil, errors.Trace(err)
		}
		return &Column{Index: int(offset), RetType: tps[offset]}, nil
	case tipb.ExprType_Null:
		return &Constant{Value: types.Datum{}, RetType: types.NewFieldType(mysql.TypeNull)}, nil
	case tipb.ExprType_Int64:
		return convertInt(expr.Val)
	case tipb.ExprType_Uint64:
		return convertUint(expr.Val)
	case tipb.ExprType_String:
		return convertString(expr.Val)
	case tipb.ExprType_Bytes:
		return &Constant{Value: types.NewBytesDatum(expr.Val), RetType: types.NewFieldType(mysql.TypeString)}, nil
	case tipb.ExprType_Float32:
		return convertFloat(expr.Val, true)
	case tipb.ExprType_Float64:
		return convertFloat(expr.Val, false)
	case tipb.ExprType_MysqlDecimal:
		return convertDecimal(expr.Val)
	case tipb.ExprType_MysqlDuration:
		return convertDuration(expr.Val)
	case tipb.ExprType_MysqlTime:
		return convertTime(expr.Val, expr.FieldType, sc.TimeZone)
	}
	if expr.Tp != tipb.ExprType_ScalarFunc {
		panic("should be a tipb.ExprType_ScalarFunc")
	}
	// Then it must be a scalar function.
	args := make([]Expression, 0, len(expr.Children))
	for _, child := range expr.Children {
		if child.Tp == tipb.ExprType_ValueList {
			results, err := decodeValueList(child.Val)
			if err != nil {
				return nil, errors.Trace(err)
			}
			if len(results) == 0 {
				return &Constant{Value: types.NewDatum(false), RetType: types.NewFieldType(mysql.TypeLonglong)}, nil
			}
			args = append(args, results...)
			continue
		}
		arg, err := PBToExpr(child, tps, sc)
		if err != nil {
			return nil, errors.Trace(err)
		}
		args = append(args, arg)
	}
	return newDistSQLFunctionBySig(sc, expr.Sig, expr.FieldType, args)
}

func fieldTypeFromPB(ft *tipb.FieldType) *types.FieldType {
	return &types.FieldType{
		Tp:      byte(ft.GetTp()),
		Flag:    uint(ft.GetFlag()),
		Flen:    int(ft.GetFlen()),
		Decimal: int(ft.GetDecimal()),
		Collate: mysql.Collations[uint8(ft.GetCollate())],
	}
}

func convertTime(data []byte, ftPB *tipb.FieldType, tz *time.Location) (*Constant, error) {
	ft := fieldTypeFromPB(ftPB)
	_, v, err := codec.DecodeUint(data)
	if err != nil {
		return nil, errors.Trace(err)
	}
	var t types.Time
	t.Type = ft.Tp
	t.Fsp = ft.Decimal
	err = t.FromPackedUint(v)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if ft.Tp == mysql.TypeTimestamp && !t.IsZero() {
		err = t.ConvertTimeZone(time.UTC, tz)
		if err != nil {
			return nil, errors.Trace(err)
		}
	}
	return &Constant{Value: types.NewTimeDatum(t), RetType: ft}, nil
}

func decodeValueList(data []byte) ([]Expression, error) {
	if len(data) == 0 {
		return nil, nil
	}
	list, err := codec.Decode(data, 1)
	if err != nil {
		return nil, errors.Trace(err)
	}
	result := make([]Expression, 0, len(list))
	for _, value := range list {
		result = append(result, &Constant{Value: value})
	}
	return result, nil
}

func convertInt(val []byte) (*Constant, error) {
	var d types.Datum
	_, i, err := codec.DecodeInt(val)
	if err != nil {
		return nil, errors.Errorf("invalid int % x", val)
	}
	d.SetInt64(i)
	return &Constant{Value: d, RetType: types.NewFieldType(mysql.TypeLonglong)}, nil
}

func convertUint(val []byte) (*Constant, error) {
	var d types.Datum
	_, u, err := codec.DecodeUint(val)
	if err != nil {
		return nil, errors.Errorf("invalid uint % x", val)
	}
	d.SetUint64(u)
	return &Constant{Value: d, RetType: &types.FieldType{Tp: mysql.TypeLonglong, Flag: mysql.UnsignedFlag}}, nil
}

func convertString(val []byte) (*Constant, error) {
	var d types.Datum
	d.SetBytesAsString(val)
	return &Constant{Value: d, RetType: types.NewFieldType(mysql.TypeVarString)}, nil
}

func convertFloat(val []byte, f32 bool) (*Constant, error) {
	var d types.Datum
	_, f, err := codec.DecodeFloat(val)
	if err != nil {
		return nil, errors.Errorf("invalid float % x", val)
	}
	if f32 {
		d.SetFloat32(float32(f))
	} else {
		d.SetFloat64(f)
	}
	return &Constant{Value: d, RetType: types.NewFieldType(mysql.TypeDouble)}, nil
}

func convertDecimal(val []byte) (*Constant, error) {
	_, dec, precision, frac, err := codec.DecodeDecimal(val)
	var d types.Datum
	d.SetMysqlDecimal(dec)
	d.SetLength(precision)
	d.SetFrac(frac)
	if err != nil {
		return nil, errors.Errorf("invalid decimal % x", val)
	}
	return &Constant{Value: d, RetType: types.NewFieldType(mysql.TypeNewDecimal)}, nil
}

func convertDuration(val []byte) (*Constant, error) {
	var d types.Datum
	_, i, err := codec.DecodeInt(val)
	if err != nil {
		return nil, errors.Errorf("invalid duration %d", i)
	}
	d.SetMysqlDuration(types.Duration{Duration: time.Duration(i), Fsp: types.MaxFsp})
	return &Constant{Value: d, RetType: types.NewFieldType(mysql.TypeDuration)}, nil
}
