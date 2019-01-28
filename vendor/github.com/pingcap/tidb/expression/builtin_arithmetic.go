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
	"math"

	"github.com/cznic/mathutil"
	"github.com/pingcap/errors"
	"github.com/pingcap/parser/mysql"
	"github.com/pingcap/parser/terror"
	"github.com/pingcap/tidb/sessionctx"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/util/chunk"
	"github.com/pingcap/tipb/go-tipb"
)

var (
	_ functionClass = &arithmeticPlusFunctionClass{}
	_ functionClass = &arithmeticMinusFunctionClass{}
	_ functionClass = &arithmeticDivideFunctionClass{}
	_ functionClass = &arithmeticMultiplyFunctionClass{}
	_ functionClass = &arithmeticIntDivideFunctionClass{}
	_ functionClass = &arithmeticModFunctionClass{}
)

var (
	_ builtinFunc = &builtinArithmeticPlusRealSig{}
	_ builtinFunc = &builtinArithmeticPlusDecimalSig{}
	_ builtinFunc = &builtinArithmeticPlusIntSig{}
	_ builtinFunc = &builtinArithmeticMinusRealSig{}
	_ builtinFunc = &builtinArithmeticMinusDecimalSig{}
	_ builtinFunc = &builtinArithmeticMinusIntSig{}
	_ builtinFunc = &builtinArithmeticDivideRealSig{}
	_ builtinFunc = &builtinArithmeticDivideDecimalSig{}
	_ builtinFunc = &builtinArithmeticMultiplyRealSig{}
	_ builtinFunc = &builtinArithmeticMultiplyDecimalSig{}
	_ builtinFunc = &builtinArithmeticMultiplyIntUnsignedSig{}
	_ builtinFunc = &builtinArithmeticMultiplyIntSig{}
	_ builtinFunc = &builtinArithmeticIntDivideIntSig{}
	_ builtinFunc = &builtinArithmeticIntDivideDecimalSig{}
	_ builtinFunc = &builtinArithmeticModIntSig{}
	_ builtinFunc = &builtinArithmeticModRealSig{}
	_ builtinFunc = &builtinArithmeticModDecimalSig{}
)

// precIncrement indicates the number of digits by which to increase the scale of the result of division operations
// performed with the / operator.
const precIncrement = 4

// numericContextResultType returns types.EvalType for numeric function's parameters.
// the returned types.EvalType should be one of: types.ETInt, types.ETDecimal, types.ETReal
func numericContextResultType(ft *types.FieldType) types.EvalType {
	if types.IsTypeTemporal(ft.Tp) {
		if ft.Decimal > 0 {
			return types.ETDecimal
		}
		return types.ETInt
	}
	if types.IsBinaryStr(ft) {
		return types.ETInt
	}
	evalTp4Ft := types.ETReal
	if !ft.Hybrid() {
		evalTp4Ft = ft.EvalType()
		if evalTp4Ft != types.ETDecimal && evalTp4Ft != types.ETInt {
			evalTp4Ft = types.ETReal
		}
	}
	return evalTp4Ft
}

// setFlenDecimal4Int is called to set proper `Flen` and `Decimal` of return
// type according to the two input parameter's types.
func setFlenDecimal4Int(retTp, a, b *types.FieldType) {
	retTp.Decimal = 0
	retTp.Flen = mysql.MaxIntWidth
}

// setFlenDecimal4RealOrDecimal is called to set proper `Flen` and `Decimal` of return
// type according to the two input parameter's types.
func setFlenDecimal4RealOrDecimal(retTp, a, b *types.FieldType, isReal bool) {
	if a.Decimal != types.UnspecifiedLength && b.Decimal != types.UnspecifiedLength {
		retTp.Decimal = a.Decimal + b.Decimal
		if !isReal && retTp.Decimal > mysql.MaxDecimalScale {
			retTp.Decimal = mysql.MaxDecimalScale
		}
		if a.Flen == types.UnspecifiedLength || b.Flen == types.UnspecifiedLength {
			retTp.Flen = types.UnspecifiedLength
			return
		}
		digitsInt := mathutil.Max(a.Flen-a.Decimal, b.Flen-b.Decimal)
		retTp.Flen = digitsInt + retTp.Decimal + 3
		if isReal {
			retTp.Flen = mathutil.Min(retTp.Flen, mysql.MaxRealWidth)
			return
		}
		retTp.Flen = mathutil.Min(retTp.Flen, mysql.MaxDecimalWidth)
		return
	}
	if isReal {
		retTp.Flen, retTp.Decimal = types.UnspecifiedLength, types.UnspecifiedLength
	} else {
		retTp.Flen, retTp.Decimal = mysql.MaxDecimalWidth, mysql.MaxDecimalScale
	}
}

func (c *arithmeticDivideFunctionClass) setType4DivDecimal(retTp, a, b *types.FieldType) {
	var deca, decb = a.Decimal, b.Decimal
	if deca == types.UnspecifiedFsp {
		deca = 0
	}
	if decb == types.UnspecifiedFsp {
		decb = 0
	}
	retTp.Decimal = deca + precIncrement
	if retTp.Decimal > mysql.MaxDecimalScale {
		retTp.Decimal = mysql.MaxDecimalScale
	}
	if a.Flen == types.UnspecifiedLength {
		retTp.Flen = mysql.MaxDecimalWidth
		return
	}
	retTp.Flen = a.Flen + decb + precIncrement
	if retTp.Flen > mysql.MaxDecimalWidth {
		retTp.Flen = mysql.MaxDecimalWidth
	}
}

func (c *arithmeticDivideFunctionClass) setType4DivReal(retTp *types.FieldType) {
	retTp.Decimal = mysql.NotFixedDec
	retTp.Flen = mysql.MaxRealWidth
}

type arithmeticPlusFunctionClass struct {
	baseFunctionClass
}

func (c *arithmeticPlusFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	lhsTp, rhsTp := args[0].GetType(), args[1].GetType()
	lhsEvalTp, rhsEvalTp := numericContextResultType(lhsTp), numericContextResultType(rhsTp)
	if lhsEvalTp == types.ETReal || rhsEvalTp == types.ETReal {
		bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETReal, types.ETReal, types.ETReal)
		setFlenDecimal4RealOrDecimal(bf.tp, args[0].GetType(), args[1].GetType(), true)
		sig := &builtinArithmeticPlusRealSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_PlusReal)
		return sig, nil
	} else if lhsEvalTp == types.ETDecimal || rhsEvalTp == types.ETDecimal {
		bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETDecimal, types.ETDecimal, types.ETDecimal)
		setFlenDecimal4RealOrDecimal(bf.tp, args[0].GetType(), args[1].GetType(), false)
		sig := &builtinArithmeticPlusDecimalSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_PlusDecimal)
		return sig, nil
	} else {
		bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETInt, types.ETInt, types.ETInt)
		if mysql.HasUnsignedFlag(args[0].GetType().Flag) || mysql.HasUnsignedFlag(args[1].GetType().Flag) {
			bf.tp.Flag |= mysql.UnsignedFlag
		}
		setFlenDecimal4Int(bf.tp, args[0].GetType(), args[1].GetType())
		sig := &builtinArithmeticPlusIntSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_PlusInt)
		return sig, nil
	}
}

type builtinArithmeticPlusIntSig struct {
	baseBuiltinFunc
}

func (s *builtinArithmeticPlusIntSig) Clone() builtinFunc {
	newSig := &builtinArithmeticPlusIntSig{}
	newSig.cloneFrom(&s.baseBuiltinFunc)
	return newSig
}

func (s *builtinArithmeticPlusIntSig) evalInt(row chunk.Row) (val int64, isNull bool, err error) {
	a, isNull, err := s.args[0].EvalInt(s.ctx, row)
	if isNull || err != nil {
		return 0, isNull, errors.Trace(err)
	}

	b, isNull, err := s.args[1].EvalInt(s.ctx, row)
	if isNull || err != nil {
		return 0, isNull, errors.Trace(err)
	}

	isLHSUnsigned := mysql.HasUnsignedFlag(s.args[0].GetType().Flag)
	isRHSUnsigned := mysql.HasUnsignedFlag(s.args[1].GetType().Flag)

	switch {
	case isLHSUnsigned && isRHSUnsigned:
		if uint64(a) > math.MaxUint64-uint64(b) {
			return 0, true, types.ErrOverflow.GenWithStackByArgs("BIGINT UNSIGNED", fmt.Sprintf("(%s + %s)", s.args[0].String(), s.args[1].String()))
		}
	case isLHSUnsigned && !isRHSUnsigned:
		if b < 0 && uint64(-b) > uint64(a) {
			return 0, true, types.ErrOverflow.GenWithStackByArgs("BIGINT UNSIGNED", fmt.Sprintf("(%s + %s)", s.args[0].String(), s.args[1].String()))
		}
		if b > 0 && uint64(a) > math.MaxUint64-uint64(b) {
			return 0, true, types.ErrOverflow.GenWithStackByArgs("BIGINT UNSIGNED", fmt.Sprintf("(%s + %s)", s.args[0].String(), s.args[1].String()))
		}
	case !isLHSUnsigned && isRHSUnsigned:
		if a < 0 && uint64(-a) > uint64(b) {
			return 0, true, types.ErrOverflow.GenWithStackByArgs("BIGINT UNSIGNED", fmt.Sprintf("(%s + %s)", s.args[0].String(), s.args[1].String()))
		}
		if a > 0 && uint64(b) > math.MaxInt64-uint64(a) {
			return 0, true, types.ErrOverflow.GenWithStackByArgs("BIGINT UNSIGNED", fmt.Sprintf("(%s + %s)", s.args[0].String(), s.args[1].String()))
		}
	case !isLHSUnsigned && !isRHSUnsigned:
		if (a > 0 && b > math.MaxInt64-a) || (a < 0 && b < math.MinInt64-a) {
			return 0, true, types.ErrOverflow.GenWithStackByArgs("BIGINT", fmt.Sprintf("(%s + %s)", s.args[0].String(), s.args[1].String()))
		}
	}

	return a + b, false, nil
}

type builtinArithmeticPlusDecimalSig struct {
	baseBuiltinFunc
}

func (s *builtinArithmeticPlusDecimalSig) Clone() builtinFunc {
	newSig := &builtinArithmeticPlusDecimalSig{}
	newSig.cloneFrom(&s.baseBuiltinFunc)
	return newSig
}

func (s *builtinArithmeticPlusDecimalSig) evalDecimal(row chunk.Row) (*types.MyDecimal, bool, error) {
	a, isNull, err := s.args[0].EvalDecimal(s.ctx, row)
	if isNull || err != nil {
		return nil, isNull, errors.Trace(err)
	}
	b, isNull, err := s.args[1].EvalDecimal(s.ctx, row)
	if isNull || err != nil {
		return nil, isNull, errors.Trace(err)
	}
	c := &types.MyDecimal{}
	err = types.DecimalAdd(a, b, c)
	if err != nil {
		return nil, true, errors.Trace(err)
	}
	return c, false, nil
}

type builtinArithmeticPlusRealSig struct {
	baseBuiltinFunc
}

func (s *builtinArithmeticPlusRealSig) Clone() builtinFunc {
	newSig := &builtinArithmeticPlusRealSig{}
	newSig.cloneFrom(&s.baseBuiltinFunc)
	return newSig
}

func (s *builtinArithmeticPlusRealSig) evalReal(row chunk.Row) (float64, bool, error) {
	a, isNull, err := s.args[0].EvalReal(s.ctx, row)
	if isNull || err != nil {
		return 0, isNull, errors.Trace(err)
	}
	b, isNull, err := s.args[1].EvalReal(s.ctx, row)
	if isNull || err != nil {
		return 0, isNull, errors.Trace(err)
	}
	if (a > 0 && b > math.MaxFloat64-a) || (a < 0 && b < -math.MaxFloat64-a) {
		return 0, true, types.ErrOverflow.GenWithStackByArgs("DOUBLE", fmt.Sprintf("(%s + %s)", s.args[0].String(), s.args[1].String()))
	}
	return a + b, false, nil
}

type arithmeticMinusFunctionClass struct {
	baseFunctionClass
}

func (c *arithmeticMinusFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	lhsTp, rhsTp := args[0].GetType(), args[1].GetType()
	lhsEvalTp, rhsEvalTp := numericContextResultType(lhsTp), numericContextResultType(rhsTp)
	if lhsEvalTp == types.ETReal || rhsEvalTp == types.ETReal {
		bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETReal, types.ETReal, types.ETReal)
		setFlenDecimal4RealOrDecimal(bf.tp, args[0].GetType(), args[1].GetType(), true)
		sig := &builtinArithmeticMinusRealSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_MinusReal)
		return sig, nil
	} else if lhsEvalTp == types.ETDecimal || rhsEvalTp == types.ETDecimal {
		bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETDecimal, types.ETDecimal, types.ETDecimal)
		setFlenDecimal4RealOrDecimal(bf.tp, args[0].GetType(), args[1].GetType(), false)
		sig := &builtinArithmeticMinusDecimalSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_MinusDecimal)
		return sig, nil
	} else {

		bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETInt, types.ETInt, types.ETInt)
		setFlenDecimal4Int(bf.tp, args[0].GetType(), args[1].GetType())
		if (mysql.HasUnsignedFlag(args[0].GetType().Flag) || mysql.HasUnsignedFlag(args[1].GetType().Flag)) && !ctx.GetSessionVars().SQLMode.HasNoUnsignedSubtractionMode() {
			bf.tp.Flag |= mysql.UnsignedFlag
		}
		sig := &builtinArithmeticMinusIntSig{baseBuiltinFunc: bf}
		sig.setPbCode(tipb.ScalarFuncSig_MinusInt)
		return sig, nil
	}
}

type builtinArithmeticMinusRealSig struct {
	baseBuiltinFunc
}

func (s *builtinArithmeticMinusRealSig) Clone() builtinFunc {
	newSig := &builtinArithmeticMinusRealSig{}
	newSig.cloneFrom(&s.baseBuiltinFunc)
	return newSig
}

func (s *builtinArithmeticMinusRealSig) evalReal(row chunk.Row) (float64, bool, error) {
	a, isNull, err := s.args[0].EvalReal(s.ctx, row)
	if isNull || err != nil {
		return 0, isNull, errors.Trace(err)
	}
	b, isNull, err := s.args[1].EvalReal(s.ctx, row)
	if isNull || err != nil {
		return 0, isNull, errors.Trace(err)
	}
	if (a > 0 && -b > math.MaxFloat64-a) || (a < 0 && -b < -math.MaxFloat64-a) {
		return 0, true, types.ErrOverflow.GenWithStackByArgs("DOUBLE", fmt.Sprintf("(%s - %s)", s.args[0].String(), s.args[1].String()))
	}
	return a - b, false, nil
}

type builtinArithmeticMinusDecimalSig struct {
	baseBuiltinFunc
}

func (s *builtinArithmeticMinusDecimalSig) Clone() builtinFunc {
	newSig := &builtinArithmeticMinusDecimalSig{}
	newSig.cloneFrom(&s.baseBuiltinFunc)
	return newSig
}

func (s *builtinArithmeticMinusDecimalSig) evalDecimal(row chunk.Row) (*types.MyDecimal, bool, error) {
	a, isNull, err := s.args[0].EvalDecimal(s.ctx, row)
	if isNull || err != nil {
		return nil, isNull, errors.Trace(err)
	}
	b, isNull, err := s.args[1].EvalDecimal(s.ctx, row)
	if isNull || err != nil {
		return nil, isNull, errors.Trace(err)
	}
	c := &types.MyDecimal{}
	err = types.DecimalSub(a, b, c)
	if err != nil {
		return nil, true, errors.Trace(err)
	}
	return c, false, nil
}

type builtinArithmeticMinusIntSig struct {
	baseBuiltinFunc
}

func (s *builtinArithmeticMinusIntSig) Clone() builtinFunc {
	newSig := &builtinArithmeticMinusIntSig{}
	newSig.cloneFrom(&s.baseBuiltinFunc)
	return newSig
}

func (s *builtinArithmeticMinusIntSig) evalInt(row chunk.Row) (val int64, isNull bool, err error) {
	a, isNull, err := s.args[0].EvalInt(s.ctx, row)
	if isNull || err != nil {
		return 0, isNull, errors.Trace(err)
	}

	b, isNull, err := s.args[1].EvalInt(s.ctx, row)
	if isNull || err != nil {
		return 0, isNull, errors.Trace(err)
	}
	forceToSigned := s.ctx.GetSessionVars().SQLMode.HasNoUnsignedSubtractionMode()
	isLHSUnsigned := !forceToSigned && mysql.HasUnsignedFlag(s.args[0].GetType().Flag)
	isRHSUnsigned := !forceToSigned && mysql.HasUnsignedFlag(s.args[1].GetType().Flag)

	if forceToSigned && mysql.HasUnsignedFlag(s.args[0].GetType().Flag) {
		if a < 0 || (a > math.MaxInt64) {
			return 0, true, types.ErrOverflow.GenWithStackByArgs("BIGINT UNSIGNED", fmt.Sprintf("(%s - %s)", s.args[0].String(), s.args[1].String()))
		}
	}
	if forceToSigned && mysql.HasUnsignedFlag(s.args[1].GetType().Flag) {
		if b < 0 || (b > math.MaxInt64) {
			return 0, true, types.ErrOverflow.GenWithStackByArgs("BIGINT UNSIGNED", fmt.Sprintf("(%s - %s)", s.args[0].String(), s.args[1].String()))
		}
	}

	switch {
	case isLHSUnsigned && isRHSUnsigned:
		if uint64(a) < uint64(b) {
			return 0, true, types.ErrOverflow.GenWithStackByArgs("BIGINT UNSIGNED", fmt.Sprintf("(%s - %s)", s.args[0].String(), s.args[1].String()))
		}
	case isLHSUnsigned && !isRHSUnsigned:
		if b >= 0 && uint64(a) < uint64(b) {
			return 0, true, types.ErrOverflow.GenWithStackByArgs("BIGINT UNSIGNED", fmt.Sprintf("(%s - %s)", s.args[0].String(), s.args[1].String()))
		}
		if b < 0 && uint64(a) > math.MaxUint64-uint64(-b) {
			return 0, true, types.ErrOverflow.GenWithStackByArgs("BIGINT UNSIGNED", fmt.Sprintf("(%s - %s)", s.args[0].String(), s.args[1].String()))
		}
	case !isLHSUnsigned && isRHSUnsigned:
		if uint64(a-math.MinInt64) < uint64(b) {
			return 0, true, types.ErrOverflow.GenWithStackByArgs("BIGINT UNSIGNED", fmt.Sprintf("(%s - %s)", s.args[0].String(), s.args[1].String()))
		}
	case !isLHSUnsigned && !isRHSUnsigned:
		if (a > 0 && -b > math.MaxInt64-a) || (a < 0 && -b < math.MinInt64-a) {
			return 0, true, types.ErrOverflow.GenWithStackByArgs("BIGINT", fmt.Sprintf("(%s - %s)", s.args[0].String(), s.args[1].String()))
		}
	}
	return a - b, false, nil
}

type arithmeticMultiplyFunctionClass struct {
	baseFunctionClass
}

func (c *arithmeticMultiplyFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	lhsTp, rhsTp := args[0].GetType(), args[1].GetType()
	lhsEvalTp, rhsEvalTp := numericContextResultType(lhsTp), numericContextResultType(rhsTp)
	if lhsEvalTp == types.ETReal || rhsEvalTp == types.ETReal {
		bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETReal, types.ETReal, types.ETReal)
		setFlenDecimal4RealOrDecimal(bf.tp, args[0].GetType(), args[1].GetType(), true)
		sig := &builtinArithmeticMultiplyRealSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_MultiplyReal)
		return sig, nil
	} else if lhsEvalTp == types.ETDecimal || rhsEvalTp == types.ETDecimal {
		bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETDecimal, types.ETDecimal, types.ETDecimal)
		setFlenDecimal4RealOrDecimal(bf.tp, args[0].GetType(), args[1].GetType(), false)
		sig := &builtinArithmeticMultiplyDecimalSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_MultiplyDecimal)
		return sig, nil
	} else {
		bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETInt, types.ETInt, types.ETInt)
		if mysql.HasUnsignedFlag(lhsTp.Flag) || mysql.HasUnsignedFlag(rhsTp.Flag) {
			bf.tp.Flag |= mysql.UnsignedFlag
			setFlenDecimal4Int(bf.tp, args[0].GetType(), args[1].GetType())
			sig := &builtinArithmeticMultiplyIntUnsignedSig{bf}
			sig.setPbCode(tipb.ScalarFuncSig_MultiplyInt)
			return sig, nil
		}
		setFlenDecimal4Int(bf.tp, args[0].GetType(), args[1].GetType())
		sig := &builtinArithmeticMultiplyIntSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_MultiplyInt)
		return sig, nil
	}
}

type builtinArithmeticMultiplyRealSig struct{ baseBuiltinFunc }

func (s *builtinArithmeticMultiplyRealSig) Clone() builtinFunc {
	newSig := &builtinArithmeticMultiplyRealSig{}
	newSig.cloneFrom(&s.baseBuiltinFunc)
	return newSig
}

type builtinArithmeticMultiplyDecimalSig struct{ baseBuiltinFunc }

func (s *builtinArithmeticMultiplyDecimalSig) Clone() builtinFunc {
	newSig := &builtinArithmeticMultiplyDecimalSig{}
	newSig.cloneFrom(&s.baseBuiltinFunc)
	return newSig
}

type builtinArithmeticMultiplyIntUnsignedSig struct{ baseBuiltinFunc }

func (s *builtinArithmeticMultiplyIntUnsignedSig) Clone() builtinFunc {
	newSig := &builtinArithmeticMultiplyIntUnsignedSig{}
	newSig.cloneFrom(&s.baseBuiltinFunc)
	return newSig
}

type builtinArithmeticMultiplyIntSig struct{ baseBuiltinFunc }

func (s *builtinArithmeticMultiplyIntSig) Clone() builtinFunc {
	newSig := &builtinArithmeticMultiplyIntSig{}
	newSig.cloneFrom(&s.baseBuiltinFunc)
	return newSig
}

func (s *builtinArithmeticMultiplyRealSig) evalReal(row chunk.Row) (float64, bool, error) {
	a, isNull, err := s.args[0].EvalReal(s.ctx, row)
	if isNull || err != nil {
		return 0, isNull, errors.Trace(err)
	}
	b, isNull, err := s.args[1].EvalReal(s.ctx, row)
	if isNull || err != nil {
		return 0, isNull, errors.Trace(err)
	}
	result := a * b
	if math.IsInf(result, 0) {
		return 0, true, types.ErrOverflow.GenWithStackByArgs("DOUBLE", fmt.Sprintf("(%s * %s)", s.args[0].String(), s.args[1].String()))
	}
	return result, false, nil
}

func (s *builtinArithmeticMultiplyDecimalSig) evalDecimal(row chunk.Row) (*types.MyDecimal, bool, error) {
	a, isNull, err := s.args[0].EvalDecimal(s.ctx, row)
	if isNull || err != nil {
		return nil, isNull, errors.Trace(err)
	}
	b, isNull, err := s.args[1].EvalDecimal(s.ctx, row)
	if isNull || err != nil {
		return nil, isNull, errors.Trace(err)
	}
	c := &types.MyDecimal{}
	err = types.DecimalMul(a, b, c)
	if err != nil && !terror.ErrorEqual(err, types.ErrTruncated) {
		return nil, true, errors.Trace(err)
	}
	return c, false, nil
}

func (s *builtinArithmeticMultiplyIntUnsignedSig) evalInt(row chunk.Row) (val int64, isNull bool, err error) {
	a, isNull, err := s.args[0].EvalInt(s.ctx, row)
	if isNull || err != nil {
		return 0, isNull, errors.Trace(err)
	}
	unsignedA := uint64(a)
	b, isNull, err := s.args[1].EvalInt(s.ctx, row)
	if isNull || err != nil {
		return 0, isNull, errors.Trace(err)
	}
	unsignedB := uint64(b)
	result := unsignedA * unsignedB
	if unsignedA != 0 && result/unsignedA != unsignedB {
		return 0, true, types.ErrOverflow.GenWithStackByArgs("BIGINT UNSIGNED", fmt.Sprintf("(%s * %s)", s.args[0].String(), s.args[1].String()))
	}
	return int64(result), false, nil
}

func (s *builtinArithmeticMultiplyIntSig) evalInt(row chunk.Row) (val int64, isNull bool, err error) {
	a, isNull, err := s.args[0].EvalInt(s.ctx, row)
	if isNull || err != nil {
		return 0, isNull, errors.Trace(err)
	}
	b, isNull, err := s.args[1].EvalInt(s.ctx, row)
	if isNull || err != nil {
		return 0, isNull, errors.Trace(err)
	}
	result := a * b
	if a != 0 && result/a != b {
		return 0, true, types.ErrOverflow.GenWithStackByArgs("BIGINT", fmt.Sprintf("(%s * %s)", s.args[0].String(), s.args[1].String()))
	}
	return result, false, nil
}

type arithmeticDivideFunctionClass struct {
	baseFunctionClass
}

func (c *arithmeticDivideFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	lhsTp, rhsTp := args[0].GetType(), args[1].GetType()
	lhsEvalTp, rhsEvalTp := numericContextResultType(lhsTp), numericContextResultType(rhsTp)
	if lhsEvalTp == types.ETReal || rhsEvalTp == types.ETReal {
		bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETReal, types.ETReal, types.ETReal)
		c.setType4DivReal(bf.tp)
		sig := &builtinArithmeticDivideRealSig{bf}
		sig.setPbCode(tipb.ScalarFuncSig_DivideReal)
		return sig, nil
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETDecimal, types.ETDecimal, types.ETDecimal)
	c.setType4DivDecimal(bf.tp, lhsTp, rhsTp)
	sig := &builtinArithmeticDivideDecimalSig{bf}
	sig.setPbCode(tipb.ScalarFuncSig_DivideDecimal)
	return sig, nil
}

type builtinArithmeticDivideRealSig struct{ baseBuiltinFunc }

func (s *builtinArithmeticDivideRealSig) Clone() builtinFunc {
	newSig := &builtinArithmeticDivideRealSig{}
	newSig.cloneFrom(&s.baseBuiltinFunc)
	return newSig
}

type builtinArithmeticDivideDecimalSig struct{ baseBuiltinFunc }

func (s *builtinArithmeticDivideDecimalSig) Clone() builtinFunc {
	newSig := &builtinArithmeticDivideDecimalSig{}
	newSig.cloneFrom(&s.baseBuiltinFunc)
	return newSig
}

func (s *builtinArithmeticDivideRealSig) evalReal(row chunk.Row) (float64, bool, error) {
	a, isNull, err := s.args[0].EvalReal(s.ctx, row)
	if isNull || err != nil {
		return 0, isNull, errors.Trace(err)
	}
	b, isNull, err := s.args[1].EvalReal(s.ctx, row)
	if isNull || err != nil {
		return 0, isNull, errors.Trace(err)
	}
	if b == 0 {
		return 0, true, errors.Trace(handleDivisionByZeroError(s.ctx))
	}
	result := a / b
	if math.IsInf(result, 0) {
		return 0, true, types.ErrOverflow.GenWithStackByArgs("DOUBLE", fmt.Sprintf("(%s / %s)", s.args[0].String(), s.args[1].String()))
	}
	return result, false, nil
}

func (s *builtinArithmeticDivideDecimalSig) evalDecimal(row chunk.Row) (*types.MyDecimal, bool, error) {
	a, isNull, err := s.args[0].EvalDecimal(s.ctx, row)
	if isNull || err != nil {
		return nil, isNull, errors.Trace(err)
	}

	b, isNull, err := s.args[1].EvalDecimal(s.ctx, row)
	if isNull || err != nil {
		return nil, isNull, errors.Trace(err)
	}

	c := &types.MyDecimal{}
	err = types.DecimalDiv(a, b, c, types.DivFracIncr)
	if err == types.ErrDivByZero {
		return c, true, errors.Trace(handleDivisionByZeroError(s.ctx))
	} else if err == nil {
		_, frac := c.PrecisionAndFrac()
		if frac < s.baseBuiltinFunc.tp.Decimal {
			err = c.Round(c, s.baseBuiltinFunc.tp.Decimal, types.ModeHalfEven)
		}
	}
	return c, false, errors.Trace(err)
}

type arithmeticIntDivideFunctionClass struct {
	baseFunctionClass
}

func (c *arithmeticIntDivideFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}

	lhsTp, rhsTp := args[0].GetType(), args[1].GetType()
	lhsEvalTp, rhsEvalTp := numericContextResultType(lhsTp), numericContextResultType(rhsTp)
	if lhsEvalTp == types.ETInt && rhsEvalTp == types.ETInt {
		bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETInt, types.ETInt, types.ETInt)
		if mysql.HasUnsignedFlag(lhsTp.Flag) || mysql.HasUnsignedFlag(rhsTp.Flag) {
			bf.tp.Flag |= mysql.UnsignedFlag
		}
		sig := &builtinArithmeticIntDivideIntSig{bf}
		return sig, nil
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETInt, types.ETDecimal, types.ETDecimal)
	if mysql.HasUnsignedFlag(lhsTp.Flag) || mysql.HasUnsignedFlag(rhsTp.Flag) {
		bf.tp.Flag |= mysql.UnsignedFlag
	}
	sig := &builtinArithmeticIntDivideDecimalSig{bf}
	return sig, nil
}

type builtinArithmeticIntDivideIntSig struct{ baseBuiltinFunc }

func (s *builtinArithmeticIntDivideIntSig) Clone() builtinFunc {
	newSig := &builtinArithmeticIntDivideIntSig{}
	newSig.cloneFrom(&s.baseBuiltinFunc)
	return newSig
}

type builtinArithmeticIntDivideDecimalSig struct{ baseBuiltinFunc }

func (s *builtinArithmeticIntDivideDecimalSig) Clone() builtinFunc {
	newSig := &builtinArithmeticIntDivideDecimalSig{}
	newSig.cloneFrom(&s.baseBuiltinFunc)
	return newSig
}

func (s *builtinArithmeticIntDivideIntSig) evalInt(row chunk.Row) (int64, bool, error) {
	b, isNull, err := s.args[1].EvalInt(s.ctx, row)
	if isNull || err != nil {
		return 0, isNull, errors.Trace(err)
	}

	if b == 0 {
		return 0, true, errors.Trace(handleDivisionByZeroError(s.ctx))
	}

	a, isNull, err := s.args[0].EvalInt(s.ctx, row)
	if isNull || err != nil {
		return 0, isNull, errors.Trace(err)
	}

	var (
		ret int64
		val uint64
	)
	isLHSUnsigned := mysql.HasUnsignedFlag(s.args[0].GetType().Flag)
	isRHSUnsigned := mysql.HasUnsignedFlag(s.args[1].GetType().Flag)

	switch {
	case isLHSUnsigned && isRHSUnsigned:
		ret = int64(uint64(a) / uint64(b))
	case isLHSUnsigned && !isRHSUnsigned:
		val, err = types.DivUintWithInt(uint64(a), b)
		ret = int64(val)
	case !isLHSUnsigned && isRHSUnsigned:
		val, err = types.DivIntWithUint(a, uint64(b))
		ret = int64(val)
	case !isLHSUnsigned && !isRHSUnsigned:
		ret, err = types.DivInt64(a, b)
	}

	return ret, err != nil, errors.Trace(err)
}

func (s *builtinArithmeticIntDivideDecimalSig) evalInt(row chunk.Row) (ret int64, isNull bool, err error) {
	sc := s.ctx.GetSessionVars().StmtCtx
	var num [2]*types.MyDecimal
	for i, arg := range s.args {
		num[i], isNull, err = arg.EvalDecimal(s.ctx, row)
		// Its behavior is consistent with MySQL.
		if terror.ErrorEqual(err, types.ErrTruncated) {
			err = nil
		}
		if terror.ErrorEqual(err, types.ErrOverflow) {
			newErr := errTruncatedWrongValue.GenWithStackByArgs("DECIMAL", arg)
			err = sc.HandleOverflow(newErr, newErr)
		}
		if isNull || err != nil {
			return 0, isNull, errors.Trace(err)
		}
	}

	c := &types.MyDecimal{}
	err = types.DecimalDiv(num[0], num[1], c, types.DivFracIncr)
	if err == types.ErrDivByZero {
		return 0, true, errors.Trace(handleDivisionByZeroError(s.ctx))
	}
	if err == types.ErrTruncated {
		err = sc.HandleTruncate(errTruncatedWrongValue.GenWithStackByArgs("DECIMAL", c))
	}
	if err == types.ErrOverflow {
		newErr := errTruncatedWrongValue.GenWithStackByArgs("DECIMAL", c)
		err = sc.HandleOverflow(newErr, newErr)
	}
	if err != nil {
		return 0, true, errors.Trace(err)
	}

	ret, err = c.ToInt()
	// err returned by ToInt may be ErrTruncated or ErrOverflow, only handle ErrOverflow, ignore ErrTruncated.
	if err == types.ErrOverflow {
		return 0, true, types.ErrOverflow.GenWithStackByArgs("BIGINT", fmt.Sprintf("(%s DIV %s)", s.args[0].String(), s.args[1].String()))
	}
	return ret, false, nil
}

type arithmeticModFunctionClass struct {
	baseFunctionClass
}

func (c *arithmeticModFunctionClass) setType4ModRealOrDecimal(retTp, a, b *types.FieldType, isDecimal bool) {
	if a.Decimal == types.UnspecifiedLength || b.Decimal == types.UnspecifiedLength {
		retTp.Decimal = types.UnspecifiedLength
	} else {
		retTp.Decimal = mathutil.Max(a.Decimal, b.Decimal)
		if isDecimal && retTp.Decimal > mysql.MaxDecimalScale {
			retTp.Decimal = mysql.MaxDecimalScale
		}
	}

	if a.Flen == types.UnspecifiedLength || b.Flen == types.UnspecifiedLength {
		retTp.Flen = types.UnspecifiedLength
	} else {
		retTp.Flen = mathutil.Max(a.Flen, b.Flen)
		if isDecimal {
			retTp.Flen = mathutil.Min(retTp.Flen, mysql.MaxDecimalWidth)
			return
		}
		retTp.Flen = mathutil.Min(retTp.Flen, mysql.MaxRealWidth)
	}
}

func (c *arithmeticModFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	lhsTp, rhsTp := args[0].GetType(), args[1].GetType()
	lhsEvalTp, rhsEvalTp := numericContextResultType(lhsTp), numericContextResultType(rhsTp)
	if lhsEvalTp == types.ETReal || rhsEvalTp == types.ETReal {
		bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETReal, types.ETReal, types.ETReal)
		c.setType4ModRealOrDecimal(bf.tp, lhsTp, rhsTp, false)
		if mysql.HasUnsignedFlag(lhsTp.Flag) {
			bf.tp.Flag |= mysql.UnsignedFlag
		}
		sig := &builtinArithmeticModRealSig{bf}
		return sig, nil
	} else if lhsEvalTp == types.ETDecimal || rhsEvalTp == types.ETDecimal {
		bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETDecimal, types.ETDecimal, types.ETDecimal)
		c.setType4ModRealOrDecimal(bf.tp, lhsTp, rhsTp, true)
		if mysql.HasUnsignedFlag(lhsTp.Flag) {
			bf.tp.Flag |= mysql.UnsignedFlag
		}
		sig := &builtinArithmeticModDecimalSig{bf}
		return sig, nil
	} else {
		bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETInt, types.ETInt, types.ETInt)
		if mysql.HasUnsignedFlag(lhsTp.Flag) {
			bf.tp.Flag |= mysql.UnsignedFlag
		}
		sig := &builtinArithmeticModIntSig{bf}
		return sig, nil
	}
}

type builtinArithmeticModRealSig struct {
	baseBuiltinFunc
}

func (s *builtinArithmeticModRealSig) Clone() builtinFunc {
	newSig := &builtinArithmeticModRealSig{}
	newSig.cloneFrom(&s.baseBuiltinFunc)
	return newSig
}

func (s *builtinArithmeticModRealSig) evalReal(row chunk.Row) (float64, bool, error) {
	b, isNull, err := s.args[1].EvalReal(s.ctx, row)
	if isNull || err != nil {
		return 0, isNull, errors.Trace(err)
	}

	if b == 0 {
		return 0, true, errors.Trace(handleDivisionByZeroError(s.ctx))
	}

	a, isNull, err := s.args[0].EvalReal(s.ctx, row)
	if isNull || err != nil {
		return 0, isNull, errors.Trace(err)
	}

	return math.Mod(a, b), false, nil
}

type builtinArithmeticModDecimalSig struct {
	baseBuiltinFunc
}

func (s *builtinArithmeticModDecimalSig) Clone() builtinFunc {
	newSig := &builtinArithmeticModDecimalSig{}
	newSig.cloneFrom(&s.baseBuiltinFunc)
	return newSig
}

func (s *builtinArithmeticModDecimalSig) evalDecimal(row chunk.Row) (*types.MyDecimal, bool, error) {
	a, isNull, err := s.args[0].EvalDecimal(s.ctx, row)
	if isNull || err != nil {
		return nil, isNull, errors.Trace(err)
	}
	b, isNull, err := s.args[1].EvalDecimal(s.ctx, row)
	if isNull || err != nil {
		return nil, isNull, errors.Trace(err)
	}
	c := &types.MyDecimal{}
	err = types.DecimalMod(a, b, c)
	if err == types.ErrDivByZero {
		return c, true, errors.Trace(handleDivisionByZeroError(s.ctx))
	}
	return c, err != nil, errors.Trace(err)
}

type builtinArithmeticModIntSig struct {
	baseBuiltinFunc
}

func (s *builtinArithmeticModIntSig) Clone() builtinFunc {
	newSig := &builtinArithmeticModIntSig{}
	newSig.cloneFrom(&s.baseBuiltinFunc)
	return newSig
}

func (s *builtinArithmeticModIntSig) evalInt(row chunk.Row) (val int64, isNull bool, err error) {
	b, isNull, err := s.args[1].EvalInt(s.ctx, row)
	if isNull || err != nil {
		return 0, isNull, errors.Trace(err)
	}

	if b == 0 {
		return 0, true, errors.Trace(handleDivisionByZeroError(s.ctx))
	}

	a, isNull, err := s.args[0].EvalInt(s.ctx, row)
	if isNull || err != nil {
		return 0, isNull, errors.Trace(err)
	}

	var ret int64
	isLHSUnsigned := mysql.HasUnsignedFlag(s.args[0].GetType().Flag)
	isRHSUnsigned := mysql.HasUnsignedFlag(s.args[1].GetType().Flag)

	switch {
	case isLHSUnsigned && isRHSUnsigned:
		ret = int64(uint64(a) % uint64(b))
	case isLHSUnsigned && !isRHSUnsigned:
		if b < 0 {
			ret = int64(uint64(a) % uint64(-b))
		} else {
			ret = int64(uint64(a) % uint64(b))
		}
	case !isLHSUnsigned && isRHSUnsigned:
		if a < 0 {
			ret = -int64(uint64(-a) % uint64(b))
		} else {
			ret = int64(uint64(a) % uint64(b))
		}
	case !isLHSUnsigned && !isRHSUnsigned:
		ret = a % b
	}

	return ret, false, nil
}
