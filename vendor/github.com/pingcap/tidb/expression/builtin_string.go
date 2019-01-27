// Copyright 2013 The ql Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSES/QL-LICENSE file.

// Copyright 2015 PingCAP, Inc.
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
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/pingcap/errors"
	"github.com/pingcap/parser/ast"
	"github.com/pingcap/parser/charset"
	"github.com/pingcap/parser/mysql"
	"github.com/pingcap/tidb/sessionctx"
	"github.com/pingcap/tidb/sessionctx/variable"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/util/chunk"
	"github.com/pingcap/tidb/util/hack"
	log "github.com/sirupsen/logrus"
	"golang.org/x/text/transform"
)

var (
	_ functionClass = &lengthFunctionClass{}
	_ functionClass = &asciiFunctionClass{}
	_ functionClass = &concatFunctionClass{}
	_ functionClass = &concatWSFunctionClass{}
	_ functionClass = &leftFunctionClass{}
	_ functionClass = &repeatFunctionClass{}
	_ functionClass = &lowerFunctionClass{}
	_ functionClass = &reverseFunctionClass{}
	_ functionClass = &spaceFunctionClass{}
	_ functionClass = &upperFunctionClass{}
	_ functionClass = &strcmpFunctionClass{}
	_ functionClass = &replaceFunctionClass{}
	_ functionClass = &convertFunctionClass{}
	_ functionClass = &substringFunctionClass{}
	_ functionClass = &substringIndexFunctionClass{}
	_ functionClass = &locateFunctionClass{}
	_ functionClass = &hexFunctionClass{}
	_ functionClass = &unhexFunctionClass{}
	_ functionClass = &trimFunctionClass{}
	_ functionClass = &lTrimFunctionClass{}
	_ functionClass = &rTrimFunctionClass{}
	_ functionClass = &lpadFunctionClass{}
	_ functionClass = &rpadFunctionClass{}
	_ functionClass = &bitLengthFunctionClass{}
	_ functionClass = &charFunctionClass{}
	_ functionClass = &charLengthFunctionClass{}
	_ functionClass = &findInSetFunctionClass{}
	_ functionClass = &fieldFunctionClass{}
	_ functionClass = &makeSetFunctionClass{}
	_ functionClass = &octFunctionClass{}
	_ functionClass = &ordFunctionClass{}
	_ functionClass = &quoteFunctionClass{}
	_ functionClass = &binFunctionClass{}
	_ functionClass = &eltFunctionClass{}
	_ functionClass = &exportSetFunctionClass{}
	_ functionClass = &formatFunctionClass{}
	_ functionClass = &fromBase64FunctionClass{}
	_ functionClass = &toBase64FunctionClass{}
	_ functionClass = &insertFunctionClass{}
	_ functionClass = &instrFunctionClass{}
	_ functionClass = &loadFileFunctionClass{}
)

var (
	_ builtinFunc = &builtinLengthSig{}
	_ builtinFunc = &builtinASCIISig{}
	_ builtinFunc = &builtinConcatSig{}
	_ builtinFunc = &builtinConcatWSSig{}
	_ builtinFunc = &builtinLeftBinarySig{}
	_ builtinFunc = &builtinLeftSig{}
	_ builtinFunc = &builtinRightBinarySig{}
	_ builtinFunc = &builtinRightSig{}
	_ builtinFunc = &builtinRepeatSig{}
	_ builtinFunc = &builtinLowerSig{}
	_ builtinFunc = &builtinReverseSig{}
	_ builtinFunc = &builtinReverseBinarySig{}
	_ builtinFunc = &builtinSpaceSig{}
	_ builtinFunc = &builtinUpperSig{}
	_ builtinFunc = &builtinStrcmpSig{}
	_ builtinFunc = &builtinReplaceSig{}
	_ builtinFunc = &builtinConvertSig{}
	_ builtinFunc = &builtinSubstringBinary2ArgsSig{}
	_ builtinFunc = &builtinSubstringBinary3ArgsSig{}
	_ builtinFunc = &builtinSubstring2ArgsSig{}
	_ builtinFunc = &builtinSubstring3ArgsSig{}
	_ builtinFunc = &builtinSubstringIndexSig{}
	_ builtinFunc = &builtinLocate2ArgsSig{}
	_ builtinFunc = &builtinLocate3ArgsSig{}
	_ builtinFunc = &builtinLocateBinary2ArgsSig{}
	_ builtinFunc = &builtinLocateBinary3ArgsSig{}
	_ builtinFunc = &builtinHexStrArgSig{}
	_ builtinFunc = &builtinHexIntArgSig{}
	_ builtinFunc = &builtinUnHexSig{}
	_ builtinFunc = &builtinTrim1ArgSig{}
	_ builtinFunc = &builtinTrim2ArgsSig{}
	_ builtinFunc = &builtinTrim3ArgsSig{}
	_ builtinFunc = &builtinLTrimSig{}
	_ builtinFunc = &builtinRTrimSig{}
	_ builtinFunc = &builtinLpadSig{}
	_ builtinFunc = &builtinLpadBinarySig{}
	_ builtinFunc = &builtinRpadSig{}
	_ builtinFunc = &builtinRpadBinarySig{}
	_ builtinFunc = &builtinBitLengthSig{}
	_ builtinFunc = &builtinCharSig{}
	_ builtinFunc = &builtinCharLengthSig{}
	_ builtinFunc = &builtinFindInSetSig{}
	_ builtinFunc = &builtinMakeSetSig{}
	_ builtinFunc = &builtinOctIntSig{}
	_ builtinFunc = &builtinOctStringSig{}
	_ builtinFunc = &builtinOrdSig{}
	_ builtinFunc = &builtinQuoteSig{}
	_ builtinFunc = &builtinBinSig{}
	_ builtinFunc = &builtinEltSig{}
	_ builtinFunc = &builtinExportSet3ArgSig{}
	_ builtinFunc = &builtinExportSet4ArgSig{}
	_ builtinFunc = &builtinExportSet5ArgSig{}
	_ builtinFunc = &builtinFormatWithLocaleSig{}
	_ builtinFunc = &builtinFormatSig{}
	_ builtinFunc = &builtinFromBase64Sig{}
	_ builtinFunc = &builtinToBase64Sig{}
	_ builtinFunc = &builtinInsertBinarySig{}
	_ builtinFunc = &builtinInsertSig{}
	_ builtinFunc = &builtinInstrSig{}
	_ builtinFunc = &builtinInstrBinarySig{}
	_ builtinFunc = &builtinFieldRealSig{}
	_ builtinFunc = &builtinFieldIntSig{}
	_ builtinFunc = &builtinFieldStringSig{}
)

func reverseBytes(origin []byte) []byte {
	for i, length := 0, len(origin); i < length/2; i++ {
		origin[i], origin[length-i-1] = origin[length-i-1], origin[i]
	}
	return origin
}

func reverseRunes(origin []rune) []rune {
	for i, length := 0, len(origin); i < length/2; i++ {
		origin[i], origin[length-i-1] = origin[length-i-1], origin[i]
	}
	return origin
}

// SetBinFlagOrBinStr sets resTp to binary string if argTp is a binary string,
// if not, sets the binary flag of resTp to true if argTp has binary flag.
func SetBinFlagOrBinStr(argTp *types.FieldType, resTp *types.FieldType) {
	if types.IsBinaryStr(argTp) {
		types.SetBinChsClnFlag(resTp)
	} else if mysql.HasBinaryFlag(argTp.Flag) || !types.IsNonBinaryStr(argTp) {
		resTp.Flag |= mysql.BinaryFlag
	}
}

type lengthFunctionClass struct {
	baseFunctionClass
}

func (c *lengthFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETInt, types.ETString)
	bf.tp.Flen = 10
	sig := &builtinLengthSig{bf}
	return sig, nil
}

type builtinLengthSig struct {
	baseBuiltinFunc
}

func (b *builtinLengthSig) Clone() builtinFunc {
	newSig := &builtinLengthSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalInt evaluates a builtinLengthSig.
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html
func (b *builtinLengthSig) evalInt(row chunk.Row) (int64, bool, error) {
	val, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return 0, isNull, errors.Trace(err)
	}
	return int64(len([]byte(val))), false, nil
}

type asciiFunctionClass struct {
	baseFunctionClass
}

func (c *asciiFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETInt, types.ETString)
	bf.tp.Flen = 3
	sig := &builtinASCIISig{bf}
	return sig, nil
}

type builtinASCIISig struct {
	baseBuiltinFunc
}

func (b *builtinASCIISig) Clone() builtinFunc {
	newSig := &builtinASCIISig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalInt evals a builtinASCIISig.
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_ascii
func (b *builtinASCIISig) evalInt(row chunk.Row) (int64, bool, error) {
	val, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return 0, isNull, errors.Trace(err)
	}
	if len(val) == 0 {
		return 0, false, nil
	}
	return int64(val[0]), false, nil
}

type concatFunctionClass struct {
	baseFunctionClass
}

func (c *concatFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	argTps := make([]types.EvalType, 0, len(args))
	for i := 0; i < len(args); i++ {
		argTps = append(argTps, types.ETString)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETString, argTps...)
	for i := range args {
		argType := args[i].GetType()
		SetBinFlagOrBinStr(argType, bf.tp)

		if argType.Flen < 0 {
			bf.tp.Flen = mysql.MaxBlobWidth
			log.Warningf("Not Expected: `Flen` of arg[%v] in CONCAT is -1.", i)
		}
		bf.tp.Flen += argType.Flen
	}
	if bf.tp.Flen >= mysql.MaxBlobWidth {
		bf.tp.Flen = mysql.MaxBlobWidth
	}
	sig := &builtinConcatSig{bf}
	return sig, nil
}

type builtinConcatSig struct {
	baseBuiltinFunc
}

func (b *builtinConcatSig) Clone() builtinFunc {
	newSig := &builtinConcatSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString evals a builtinConcatSig
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_concat
func (b *builtinConcatSig) evalString(row chunk.Row) (d string, isNull bool, err error) {
	var s []byte
	for _, a := range b.getArgs() {
		d, isNull, err = a.EvalString(b.ctx, row)
		if isNull || err != nil {
			return d, isNull, errors.Trace(err)
		}
		s = append(s, []byte(d)...)
	}
	return string(s), false, nil
}

type concatWSFunctionClass struct {
	baseFunctionClass
}

func (c *concatWSFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	argTps := make([]types.EvalType, 0, len(args))
	for i := 0; i < len(args); i++ {
		argTps = append(argTps, types.ETString)
	}

	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETString, argTps...)

	for i := range args {
		argType := args[i].GetType()
		SetBinFlagOrBinStr(argType, bf.tp)

		// skip separator param
		if i != 0 {
			if argType.Flen < 0 {
				bf.tp.Flen = mysql.MaxBlobWidth
				log.Warningf("Not Expected: `Flen` of arg[%v] in CONCAT_WS is -1.", i)
			}
			bf.tp.Flen += argType.Flen
		}
	}

	// add separator
	argsLen := len(args) - 1
	bf.tp.Flen += argsLen - 1

	if bf.tp.Flen >= mysql.MaxBlobWidth {
		bf.tp.Flen = mysql.MaxBlobWidth
	}

	sig := &builtinConcatWSSig{bf}
	return sig, nil
}

type builtinConcatWSSig struct {
	baseBuiltinFunc
}

func (b *builtinConcatWSSig) Clone() builtinFunc {
	newSig := &builtinConcatWSSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString evals a builtinConcatWSSig.
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_concat-ws
func (b *builtinConcatWSSig) evalString(row chunk.Row) (string, bool, error) {
	args := b.getArgs()
	strs := make([]string, 0, len(args))
	var sep string
	for i, arg := range args {
		val, isNull, err := arg.EvalString(b.ctx, row)
		if err != nil {
			return val, isNull, errors.Trace(err)
		}

		if isNull {
			// If the separator is NULL, the result is NULL.
			if i == 0 {
				return val, isNull, nil
			}
			// CONCAT_WS() does not skip empty strings. However,
			// it does skip any NULL values after the separator argument.
			continue
		}

		if i == 0 {
			sep = val
			continue
		}
		strs = append(strs, val)
	}

	// TODO: check whether the length of result is larger than Flen
	return strings.Join(strs, sep), false, nil
}

type leftFunctionClass struct {
	baseFunctionClass
}

func (c *leftFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETString, types.ETString, types.ETInt)
	argType := args[0].GetType()
	bf.tp.Flen = argType.Flen
	SetBinFlagOrBinStr(argType, bf.tp)
	if types.IsBinaryStr(argType) {
		sig := &builtinLeftBinarySig{bf}
		return sig, nil
	}
	sig := &builtinLeftSig{bf}
	return sig, nil
}

type builtinLeftBinarySig struct {
	baseBuiltinFunc
}

func (b *builtinLeftBinarySig) Clone() builtinFunc {
	newSig := &builtinLeftBinarySig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString evals LEFT(str,len).
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_left
func (b *builtinLeftBinarySig) evalString(row chunk.Row) (string, bool, error) {
	str, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}
	left, isNull, err := b.args[1].EvalInt(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}
	leftLength := int(left)
	if strLength := len(str); leftLength > strLength {
		leftLength = strLength
	} else if leftLength < 0 {
		leftLength = 0
	}
	return str[:leftLength], false, nil
}

type builtinLeftSig struct {
	baseBuiltinFunc
}

func (b *builtinLeftSig) Clone() builtinFunc {
	newSig := &builtinLeftSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString evals LEFT(str,len).
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_left
func (b *builtinLeftSig) evalString(row chunk.Row) (string, bool, error) {
	str, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}
	left, isNull, err := b.args[1].EvalInt(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}
	runes, leftLength := []rune(str), int(left)
	if runeLength := len(runes); leftLength > runeLength {
		leftLength = runeLength
	} else if leftLength < 0 {
		leftLength = 0
	}
	return string(runes[:leftLength]), false, nil
}

type rightFunctionClass struct {
	baseFunctionClass
}

func (c *rightFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETString, types.ETString, types.ETInt)
	argType := args[0].GetType()
	bf.tp.Flen = argType.Flen
	SetBinFlagOrBinStr(argType, bf.tp)
	if types.IsBinaryStr(argType) {
		sig := &builtinRightBinarySig{bf}
		return sig, nil
	}
	sig := &builtinRightSig{bf}
	return sig, nil
}

type builtinRightBinarySig struct {
	baseBuiltinFunc
}

func (b *builtinRightBinarySig) Clone() builtinFunc {
	newSig := &builtinRightBinarySig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString evals RIGHT(str,len).
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_right
func (b *builtinRightBinarySig) evalString(row chunk.Row) (string, bool, error) {
	str, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}
	right, isNull, err := b.args[1].EvalInt(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}
	strLength, rightLength := len(str), int(right)
	if rightLength > strLength {
		rightLength = strLength
	} else if rightLength < 0 {
		rightLength = 0
	}
	return str[strLength-rightLength:], false, nil
}

type builtinRightSig struct {
	baseBuiltinFunc
}

func (b *builtinRightSig) Clone() builtinFunc {
	newSig := &builtinRightSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString evals RIGHT(str,len).
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_right
func (b *builtinRightSig) evalString(row chunk.Row) (string, bool, error) {
	str, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}
	right, isNull, err := b.args[1].EvalInt(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}
	runes := []rune(str)
	strLength, rightLength := len(runes), int(right)
	if rightLength > strLength {
		rightLength = strLength
	} else if rightLength < 0 {
		rightLength = 0
	}
	return string(runes[strLength-rightLength:]), false, nil
}

type repeatFunctionClass struct {
	baseFunctionClass
}

func (c *repeatFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETString, types.ETString, types.ETInt)
	bf.tp.Flen = mysql.MaxBlobWidth
	SetBinFlagOrBinStr(args[0].GetType(), bf.tp)
	valStr, _ := ctx.GetSessionVars().GetSystemVar(variable.MaxAllowedPacket)
	maxAllowedPacket, err := strconv.ParseUint(valStr, 10, 64)
	if err != nil {
		return nil, errors.Trace(err)
	}
	sig := &builtinRepeatSig{bf, maxAllowedPacket}
	return sig, nil
}

type builtinRepeatSig struct {
	baseBuiltinFunc
	maxAllowedPacket uint64
}

func (b *builtinRepeatSig) Clone() builtinFunc {
	newSig := &builtinRepeatSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	newSig.maxAllowedPacket = b.maxAllowedPacket
	return newSig
}

// evalString evals a builtinRepeatSig.
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_repeat
func (b *builtinRepeatSig) evalString(row chunk.Row) (d string, isNull bool, err error) {
	str, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", isNull, errors.Trace(err)
	}
	byteLength := len(str)

	num, isNull, err := b.args[1].EvalInt(b.ctx, row)
	if isNull || err != nil {
		return "", isNull, errors.Trace(err)
	}
	if num < 1 {
		return "", false, nil
	}
	if num > math.MaxInt32 {
		num = math.MaxInt32
	}

	if uint64(byteLength)*uint64(num) > b.maxAllowedPacket {
		b.ctx.GetSessionVars().StmtCtx.AppendWarning(errWarnAllowedPacketOverflowed.GenWithStackByArgs("repeat", b.maxAllowedPacket))
		return "", true, nil
	}

	if int64(byteLength) > int64(b.tp.Flen)/num {
		return "", true, nil
	}
	return strings.Repeat(str, int(num)), false, nil
}

type lowerFunctionClass struct {
	baseFunctionClass
}

func (c *lowerFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETString, types.ETString)
	argTp := args[0].GetType()
	bf.tp.Flen = argTp.Flen
	SetBinFlagOrBinStr(argTp, bf.tp)
	sig := &builtinLowerSig{bf}
	return sig, nil
}

type builtinLowerSig struct {
	baseBuiltinFunc
}

func (b *builtinLowerSig) Clone() builtinFunc {
	newSig := &builtinLowerSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString evals a builtinLowerSig.
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_lower
func (b *builtinLowerSig) evalString(row chunk.Row) (d string, isNull bool, err error) {
	d, isNull, err = b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return d, isNull, errors.Trace(err)
	}

	if types.IsBinaryStr(b.args[0].GetType()) {
		return d, false, nil
	}

	return strings.ToLower(d), false, nil
}

type reverseFunctionClass struct {
	baseFunctionClass
}

func (c *reverseFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETString, types.ETString)
	retTp := *args[0].GetType()
	retTp.Tp = mysql.TypeVarString
	retTp.Decimal = types.UnspecifiedLength
	bf.tp = &retTp
	var sig builtinFunc
	if types.IsBinaryStr(bf.tp) {
		sig = &builtinReverseBinarySig{bf}
	} else {
		sig = &builtinReverseSig{bf}
	}
	return sig, nil
}

type builtinReverseBinarySig struct {
	baseBuiltinFunc
}

func (b *builtinReverseBinarySig) Clone() builtinFunc {
	newSig := &builtinReverseBinarySig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString evals a REVERSE(str).
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_reverse
func (b *builtinReverseBinarySig) evalString(row chunk.Row) (string, bool, error) {
	str, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}
	reversed := reverseBytes([]byte(str))
	return string(reversed), false, nil
}

type builtinReverseSig struct {
	baseBuiltinFunc
}

func (b *builtinReverseSig) Clone() builtinFunc {
	newSig := &builtinReverseSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString evals a REVERSE(str).
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_reverse
func (b *builtinReverseSig) evalString(row chunk.Row) (string, bool, error) {
	str, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}
	reversed := reverseRunes([]rune(str))
	return string(reversed), false, nil
}

type spaceFunctionClass struct {
	baseFunctionClass
}

func (c *spaceFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETString, types.ETInt)
	bf.tp.Flen = mysql.MaxBlobWidth
	valStr, _ := ctx.GetSessionVars().GetSystemVar(variable.MaxAllowedPacket)
	maxAllowedPacket, err := strconv.ParseUint(valStr, 10, 64)
	if err != nil {
		return nil, errors.Trace(err)
	}
	sig := &builtinSpaceSig{bf, maxAllowedPacket}
	return sig, nil
}

type builtinSpaceSig struct {
	baseBuiltinFunc
	maxAllowedPacket uint64
}

func (b *builtinSpaceSig) Clone() builtinFunc {
	newSig := &builtinSpaceSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	newSig.maxAllowedPacket = b.maxAllowedPacket
	return newSig
}

// evalString evals a builtinSpaceSig.
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_space
func (b *builtinSpaceSig) evalString(row chunk.Row) (d string, isNull bool, err error) {
	var x int64

	x, isNull, err = b.args[0].EvalInt(b.ctx, row)
	if isNull || err != nil {
		return d, isNull, errors.Trace(err)
	}
	if x < 0 {
		x = 0
	}
	if uint64(x) > b.maxAllowedPacket {
		b.ctx.GetSessionVars().StmtCtx.AppendWarning(errWarnAllowedPacketOverflowed.GenWithStackByArgs("space", b.maxAllowedPacket))
		return d, true, nil
	}
	if x > mysql.MaxBlobWidth {
		return d, true, nil
	}
	return strings.Repeat(" ", int(x)), false, nil
}

type upperFunctionClass struct {
	baseFunctionClass
}

func (c *upperFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETString, types.ETString)
	argTp := args[0].GetType()
	bf.tp.Flen = argTp.Flen
	SetBinFlagOrBinStr(argTp, bf.tp)
	sig := &builtinUpperSig{bf}
	return sig, nil
}

type builtinUpperSig struct {
	baseBuiltinFunc
}

func (b *builtinUpperSig) Clone() builtinFunc {
	newSig := &builtinUpperSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString evals a builtinUpperSig.
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_upper
func (b *builtinUpperSig) evalString(row chunk.Row) (d string, isNull bool, err error) {
	d, isNull, err = b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return d, isNull, errors.Trace(err)
	}

	if types.IsBinaryStr(b.args[0].GetType()) {
		return d, false, nil
	}

	return strings.ToUpper(d), false, nil
}

type strcmpFunctionClass struct {
	baseFunctionClass
}

func (c *strcmpFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETInt, types.ETString, types.ETString)
	bf.tp.Flen = 2
	types.SetBinChsClnFlag(bf.tp)
	sig := &builtinStrcmpSig{bf}
	return sig, nil
}

type builtinStrcmpSig struct {
	baseBuiltinFunc
}

func (b *builtinStrcmpSig) Clone() builtinFunc {
	newSig := &builtinStrcmpSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalInt evals a builtinStrcmpSig.
// See https://dev.mysql.com/doc/refman/5.7/en/string-comparison-functions.html
func (b *builtinStrcmpSig) evalInt(row chunk.Row) (int64, bool, error) {
	var (
		left, right string
		isNull      bool
		err         error
	)

	left, isNull, err = b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return 0, isNull, errors.Trace(err)
	}
	right, isNull, err = b.args[1].EvalString(b.ctx, row)
	if isNull || err != nil {
		return 0, isNull, errors.Trace(err)
	}
	res := types.CompareString(left, right)
	return int64(res), false, nil
}

type replaceFunctionClass struct {
	baseFunctionClass
}

func (c *replaceFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETString, types.ETString, types.ETString, types.ETString)
	bf.tp.Flen = c.fixLength(args)
	for _, a := range args {
		SetBinFlagOrBinStr(a.GetType(), bf.tp)
	}
	sig := &builtinReplaceSig{bf}
	return sig, nil
}

// fixLength calculate the Flen of the return type.
func (c *replaceFunctionClass) fixLength(args []Expression) int {
	charLen := args[0].GetType().Flen
	oldStrLen := args[1].GetType().Flen
	diff := args[2].GetType().Flen - oldStrLen
	if diff > 0 && oldStrLen > 0 {
		charLen += (charLen / oldStrLen) * diff
	}
	return charLen
}

type builtinReplaceSig struct {
	baseBuiltinFunc
}

func (b *builtinReplaceSig) Clone() builtinFunc {
	newSig := &builtinReplaceSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString evals a builtinReplaceSig.
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_replace
func (b *builtinReplaceSig) evalString(row chunk.Row) (d string, isNull bool, err error) {
	var str, oldStr, newStr string

	str, isNull, err = b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return d, isNull, errors.Trace(err)
	}
	oldStr, isNull, err = b.args[1].EvalString(b.ctx, row)
	if isNull || err != nil {
		return d, isNull, errors.Trace(err)
	}
	newStr, isNull, err = b.args[2].EvalString(b.ctx, row)
	if isNull || err != nil {
		return d, isNull, errors.Trace(err)
	}
	if oldStr == "" {
		return str, false, nil
	}
	return strings.Replace(str, oldStr, newStr, -1), false, nil
}

type convertFunctionClass struct {
	baseFunctionClass
}

func (c *convertFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETString, types.ETString, types.ETString)
	// TODO: issue #4436: The second parameter should be a constant.
	// TODO: issue #4474: Charset supported by TiDB and MySQL is not the same.
	// TODO: Fix #4436 && #4474, set the correct charset and flag of `bf.tp`.
	bf.tp.Flen = mysql.MaxBlobWidth
	sig := &builtinConvertSig{bf}
	return sig, nil
}

type builtinConvertSig struct {
	baseBuiltinFunc
}

func (b *builtinConvertSig) Clone() builtinFunc {
	newSig := &builtinConvertSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString evals CONVERT(expr USING transcoding_name).
// See https://dev.mysql.com/doc/refman/5.7/en/cast-functions.html#function_convert
func (b *builtinConvertSig) evalString(row chunk.Row) (string, bool, error) {
	expr, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}

	charsetName, isNull, err := b.args[1].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}

	encoding, _ := charset.Lookup(charsetName)
	if encoding == nil {
		return "", true, errUnknownCharacterSet.GenWithStackByArgs(charsetName)
	}

	target, _, err := transform.String(encoding.NewDecoder(), expr)
	return target, err != nil, errors.Trace(err)
}

type substringFunctionClass struct {
	baseFunctionClass
}

func (c *substringFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	argTps := []types.EvalType{types.ETString, types.ETInt}
	if len(args) == 3 {
		argTps = append(argTps, types.ETInt)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETString, argTps...)

	argType := args[0].GetType()
	bf.tp.Flen = argType.Flen
	SetBinFlagOrBinStr(argType, bf.tp)

	var sig builtinFunc
	switch {
	case len(args) == 3 && types.IsBinaryStr(argType):
		sig = &builtinSubstringBinary3ArgsSig{bf}
	case len(args) == 3:
		sig = &builtinSubstring3ArgsSig{bf}
	case len(args) == 2 && types.IsBinaryStr(argType):
		sig = &builtinSubstringBinary2ArgsSig{bf}
	case len(args) == 2:
		sig = &builtinSubstring2ArgsSig{bf}
	default:
		// Should never happens.
		return nil, errors.Errorf("SUBSTR invalid arg length, expect 2 or 3 but got: %v", len(args))
	}
	return sig, nil
}

type builtinSubstringBinary2ArgsSig struct {
	baseBuiltinFunc
}

func (b *builtinSubstringBinary2ArgsSig) Clone() builtinFunc {
	newSig := &builtinSubstringBinary2ArgsSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString evals SUBSTR(str,pos), SUBSTR(str FROM pos), SUBSTR() is a synonym for SUBSTRING().
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_substr
func (b *builtinSubstringBinary2ArgsSig) evalString(row chunk.Row) (string, bool, error) {
	str, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}
	pos, isNull, err := b.args[1].EvalInt(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}
	length := int64(len(str))
	if pos < 0 {
		pos += length
	} else {
		pos--
	}
	if pos > length || pos < 0 {
		pos = length
	}
	return str[pos:], false, nil
}

type builtinSubstring2ArgsSig struct {
	baseBuiltinFunc
}

func (b *builtinSubstring2ArgsSig) Clone() builtinFunc {
	newSig := &builtinSubstring2ArgsSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString evals SUBSTR(str,pos), SUBSTR(str FROM pos), SUBSTR() is a synonym for SUBSTRING().
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_substr
func (b *builtinSubstring2ArgsSig) evalString(row chunk.Row) (string, bool, error) {
	str, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}
	pos, isNull, err := b.args[1].EvalInt(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}
	runes := []rune(str)
	length := int64(len(runes))
	if pos < 0 {
		pos += length
	} else {
		pos--
	}
	if pos > length || pos < 0 {
		pos = length
	}
	return string(runes[pos:]), false, nil
}

type builtinSubstringBinary3ArgsSig struct {
	baseBuiltinFunc
}

func (b *builtinSubstringBinary3ArgsSig) Clone() builtinFunc {
	newSig := &builtinSubstringBinary3ArgsSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString evals SUBSTR(str,pos,len), SUBSTR(str FROM pos FOR len), SUBSTR() is a synonym for SUBSTRING().
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_substr
func (b *builtinSubstringBinary3ArgsSig) evalString(row chunk.Row) (string, bool, error) {
	str, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}
	pos, isNull, err := b.args[1].EvalInt(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}
	length, isNull, err := b.args[2].EvalInt(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}
	byteLen := int64(len(str))
	if pos < 0 {
		pos += byteLen
	} else {
		pos--
	}
	if pos > byteLen || pos < 0 {
		pos = byteLen
	}
	end := pos + length
	if end < pos {
		return "", false, nil
	} else if end < byteLen {
		return str[pos:end], false, nil
	}
	return str[pos:], false, nil
}

type builtinSubstring3ArgsSig struct {
	baseBuiltinFunc
}

func (b *builtinSubstring3ArgsSig) Clone() builtinFunc {
	newSig := &builtinSubstring3ArgsSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString evals SUBSTR(str,pos,len), SUBSTR(str FROM pos FOR len), SUBSTR() is a synonym for SUBSTRING().
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_substr
func (b *builtinSubstring3ArgsSig) evalString(row chunk.Row) (string, bool, error) {
	str, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}
	pos, isNull, err := b.args[1].EvalInt(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}
	length, isNull, err := b.args[2].EvalInt(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}
	runes := []rune(str)
	numRunes := int64(len(runes))
	if pos < 0 {
		pos += numRunes
	} else {
		pos--
	}
	if pos > numRunes || pos < 0 {
		pos = numRunes
	}
	end := pos + length
	if end < pos {
		return "", false, nil
	} else if end < numRunes {
		return string(runes[pos:end]), false, nil
	}
	return string(runes[pos:]), false, nil
}

type substringIndexFunctionClass struct {
	baseFunctionClass
}

func (c *substringIndexFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETString, types.ETString, types.ETString, types.ETInt)
	argType := args[0].GetType()
	bf.tp.Flen = argType.Flen
	SetBinFlagOrBinStr(argType, bf.tp)
	sig := &builtinSubstringIndexSig{bf}
	return sig, nil
}

type builtinSubstringIndexSig struct {
	baseBuiltinFunc
}

func (b *builtinSubstringIndexSig) Clone() builtinFunc {
	newSig := &builtinSubstringIndexSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString evals a builtinSubstringIndexSig.
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_substring-index
func (b *builtinSubstringIndexSig) evalString(row chunk.Row) (d string, isNull bool, err error) {
	var (
		str, delim string
		count      int64
	)
	str, isNull, err = b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return d, isNull, errors.Trace(err)
	}
	delim, isNull, err = b.args[1].EvalString(b.ctx, row)
	if isNull || err != nil {
		return d, isNull, errors.Trace(err)
	}
	count, isNull, err = b.args[2].EvalInt(b.ctx, row)
	if isNull || err != nil {
		return d, isNull, errors.Trace(err)
	}
	if len(delim) == 0 {
		return "", false, nil
	}

	strs := strings.Split(str, delim)
	start, end := int64(0), int64(len(strs))
	if count > 0 {
		// If count is positive, everything to the left of the final delimiter (counting from the left) is returned.
		if count < end {
			end = count
		}
	} else {
		// If count is negative, everything to the right of the final delimiter (counting from the right) is returned.
		count = -count
		if count < 0 {
			// -count overflows max int64, returns an empty string.
			return "", false, nil
		}

		if count < end {
			start = end - count
		}
	}
	substrs := strs[start:end]
	return strings.Join(substrs, delim), false, nil
}

type locateFunctionClass struct {
	baseFunctionClass
}

func (c *locateFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	hasStartPos, argTps := len(args) == 3, []types.EvalType{types.ETString, types.ETString}
	if hasStartPos {
		argTps = append(argTps, types.ETInt)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETInt, argTps...)
	var sig builtinFunc
	// Loacte is multibyte safe, and is case-sensitive only if at least one argument is a binary string.
	hasBianryInput := types.IsBinaryStr(args[0].GetType()) || types.IsBinaryStr(args[1].GetType())
	switch {
	case hasStartPos && hasBianryInput:
		sig = &builtinLocateBinary3ArgsSig{bf}
	case hasStartPos:
		sig = &builtinLocate3ArgsSig{bf}
	case hasBianryInput:
		sig = &builtinLocateBinary2ArgsSig{bf}
	default:
		sig = &builtinLocate2ArgsSig{bf}
	}
	return sig, nil
}

type builtinLocateBinary2ArgsSig struct {
	baseBuiltinFunc
}

func (b *builtinLocateBinary2ArgsSig) Clone() builtinFunc {
	newSig := &builtinLocateBinary2ArgsSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalInt evals LOCATE(substr,str), case-sensitive.
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_locate
func (b *builtinLocateBinary2ArgsSig) evalInt(row chunk.Row) (int64, bool, error) {
	subStr, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return 0, isNull, errors.Trace(err)
	}
	str, isNull, err := b.args[1].EvalString(b.ctx, row)
	if isNull || err != nil {
		return 0, isNull, errors.Trace(err)
	}
	subStrLen := len(subStr)
	if subStrLen == 0 {
		return 1, false, nil
	}
	ret, idx := 0, strings.Index(str, subStr)
	if idx != -1 {
		ret = idx + 1
	}
	return int64(ret), false, nil
}

type builtinLocate2ArgsSig struct {
	baseBuiltinFunc
}

func (b *builtinLocate2ArgsSig) Clone() builtinFunc {
	newSig := &builtinLocate2ArgsSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalInt evals LOCATE(substr,str), non case-sensitive.
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_locate
func (b *builtinLocate2ArgsSig) evalInt(row chunk.Row) (int64, bool, error) {
	subStr, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return 0, isNull, errors.Trace(err)
	}
	str, isNull, err := b.args[1].EvalString(b.ctx, row)
	if isNull || err != nil {
		return 0, isNull, errors.Trace(err)
	}
	if int64(len([]rune(subStr))) == 0 {
		return 1, false, nil
	}
	slice := string([]rune(strings.ToLower(str)))
	ret, idx := 0, strings.Index(slice, strings.ToLower(subStr))
	if idx != -1 {
		ret = utf8.RuneCountInString(slice[:idx]) + 1
	}
	return int64(ret), false, nil
}

type builtinLocateBinary3ArgsSig struct {
	baseBuiltinFunc
}

func (b *builtinLocateBinary3ArgsSig) Clone() builtinFunc {
	newSig := &builtinLocateBinary3ArgsSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalInt evals LOCATE(substr,str,pos), case-sensitive.
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_locate
func (b *builtinLocateBinary3ArgsSig) evalInt(row chunk.Row) (int64, bool, error) {
	subStr, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return 0, isNull, errors.Trace(err)
	}
	str, isNull, err := b.args[1].EvalString(b.ctx, row)
	if isNull || err != nil {
		return 0, isNull, errors.Trace(err)
	}
	pos, isNull, err := b.args[2].EvalInt(b.ctx, row)
	// Transfer the argument which starts from 1 to real index which starts from 0.
	pos--
	if isNull || err != nil {
		return 0, isNull, errors.Trace(err)
	}
	subStrLen := len(subStr)
	if pos < 0 || pos > int64(len(str)-subStrLen) {
		return 0, false, nil
	} else if subStrLen == 0 {
		return pos + 1, false, nil
	}
	slice := str[pos:]
	idx := strings.Index(slice, subStr)
	if idx != -1 {
		return pos + int64(idx) + 1, false, nil
	}
	return 0, false, nil
}

type builtinLocate3ArgsSig struct {
	baseBuiltinFunc
}

func (b *builtinLocate3ArgsSig) Clone() builtinFunc {
	newSig := &builtinLocate3ArgsSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalInt evals LOCATE(substr,str,pos), non case-sensitive.
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_locate
func (b *builtinLocate3ArgsSig) evalInt(row chunk.Row) (int64, bool, error) {
	subStr, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return 0, isNull, errors.Trace(err)
	}
	str, isNull, err := b.args[1].EvalString(b.ctx, row)
	if isNull || err != nil {
		return 0, isNull, errors.Trace(err)
	}
	pos, isNull, err := b.args[2].EvalInt(b.ctx, row)
	// Transfer the argument which starts from 1 to real index which starts from 0.
	pos--
	if isNull || err != nil {
		return 0, isNull, errors.Trace(err)
	}
	subStrLen := len([]rune(subStr))
	if pos < 0 || pos > int64(len([]rune(strings.ToLower(str)))-subStrLen) {
		return 0, false, nil
	} else if subStrLen == 0 {
		return pos + 1, false, nil
	}
	slice := string([]rune(strings.ToLower(str))[pos:])
	idx := strings.Index(slice, strings.ToLower(subStr))
	if idx != -1 {
		return pos + int64(utf8.RuneCountInString(slice[:idx])) + 1, false, nil
	}
	return 0, false, nil
}

type hexFunctionClass struct {
	baseFunctionClass
}

func (c *hexFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}

	argTp := args[0].GetType().EvalType()
	switch argTp {
	case types.ETString, types.ETDatetime, types.ETTimestamp, types.ETDuration, types.ETJson:
		bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETString, types.ETString)
		// Use UTF-8 as default
		bf.tp.Flen = args[0].GetType().Flen * 3 * 2
		sig := &builtinHexStrArgSig{bf}
		return sig, nil
	case types.ETInt, types.ETReal, types.ETDecimal:
		bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETString, types.ETInt)
		bf.tp.Flen = args[0].GetType().Flen * 2
		sig := &builtinHexIntArgSig{bf}
		return sig, nil
	default:
		return nil, errors.Errorf("Hex invalid args, need int or string but get %T", args[0].GetType())
	}
}

type builtinHexStrArgSig struct {
	baseBuiltinFunc
}

func (b *builtinHexStrArgSig) Clone() builtinFunc {
	newSig := &builtinHexStrArgSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString evals a builtinHexStrArgSig, corresponding to hex(str)
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_hex
func (b *builtinHexStrArgSig) evalString(row chunk.Row) (string, bool, error) {
	d, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return d, isNull, errors.Trace(err)
	}
	return strings.ToUpper(hex.EncodeToString(hack.Slice(d))), false, nil
}

type builtinHexIntArgSig struct {
	baseBuiltinFunc
}

func (b *builtinHexIntArgSig) Clone() builtinFunc {
	newSig := &builtinHexIntArgSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString evals a builtinHexIntArgSig, corresponding to hex(N)
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_hex
func (b *builtinHexIntArgSig) evalString(row chunk.Row) (string, bool, error) {
	x, isNull, err := b.args[0].EvalInt(b.ctx, row)
	if isNull || err != nil {
		return "", isNull, errors.Trace(err)
	}
	return strings.ToUpper(fmt.Sprintf("%x", uint64(x))), false, nil
}

type unhexFunctionClass struct {
	baseFunctionClass
}

func (c *unhexFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	var retFlen int

	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	argType := args[0].GetType()
	argEvalTp := argType.EvalType()
	switch argEvalTp {
	case types.ETString, types.ETDatetime, types.ETTimestamp, types.ETDuration, types.ETJson:
		// Use UTF-8 as default charset, so there're (Flen * 3 + 1) / 2 byte-pairs
		retFlen = (argType.Flen*3 + 1) / 2
	case types.ETInt, types.ETReal, types.ETDecimal:
		// For number value, there're (Flen + 1) / 2 byte-pairs
		retFlen = (argType.Flen + 1) / 2
	default:
		return nil, errors.Errorf("Unhex invalid args, need int or string but get %s", argType)
	}

	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETString, types.ETString)
	bf.tp.Flen = retFlen
	types.SetBinChsClnFlag(bf.tp)
	sig := &builtinUnHexSig{bf}
	return sig, nil
}

type builtinUnHexSig struct {
	baseBuiltinFunc
}

func (b *builtinUnHexSig) Clone() builtinFunc {
	newSig := &builtinUnHexSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString evals a builtinUnHexSig.
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_unhex
func (b *builtinUnHexSig) evalString(row chunk.Row) (string, bool, error) {
	var bs []byte

	d, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return d, isNull, errors.Trace(err)
	}
	// Add a '0' to the front, if the length is not the multiple of 2
	if len(d)%2 != 0 {
		d = "0" + d
	}
	bs, err = hex.DecodeString(d)
	if err != nil {
		return "", true, nil
	}
	return string(bs), false, nil
}

const spaceChars = " "

type trimFunctionClass struct {
	baseFunctionClass
}

// getFunction sets trim built-in function signature.
// The syntax of trim in mysql is 'TRIM([{BOTH | LEADING | TRAILING} [remstr] FROM] str), TRIM([remstr FROM] str)',
// but we wil convert it into trim(str), trim(str, remstr) and trim(str, remstr, direction) in AST.
func (c *trimFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}

	switch len(args) {
	case 1:
		bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETString, types.ETString)
		argType := args[0].GetType()
		bf.tp.Flen = argType.Flen
		SetBinFlagOrBinStr(argType, bf.tp)
		sig := &builtinTrim1ArgSig{bf}
		return sig, nil

	case 2:
		bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETString, types.ETString, types.ETString)
		argType := args[0].GetType()
		SetBinFlagOrBinStr(argType, bf.tp)
		sig := &builtinTrim2ArgsSig{bf}
		return sig, nil

	case 3:
		bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETString, types.ETString, types.ETString, types.ETInt)
		argType := args[0].GetType()
		bf.tp.Flen = argType.Flen
		SetBinFlagOrBinStr(argType, bf.tp)
		sig := &builtinTrim3ArgsSig{bf}
		return sig, nil

	default:
		return nil, errors.Trace(c.verifyArgs(args))
	}
}

type builtinTrim1ArgSig struct {
	baseBuiltinFunc
}

func (b *builtinTrim1ArgSig) Clone() builtinFunc {
	newSig := &builtinTrim1ArgSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString evals a builtinTrim1ArgSig, corresponding to trim(str)
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_trim
func (b *builtinTrim1ArgSig) evalString(row chunk.Row) (d string, isNull bool, err error) {
	d, isNull, err = b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return d, isNull, errors.Trace(err)
	}
	return strings.Trim(d, spaceChars), false, nil
}

type builtinTrim2ArgsSig struct {
	baseBuiltinFunc
}

func (b *builtinTrim2ArgsSig) Clone() builtinFunc {
	newSig := &builtinTrim2ArgsSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString evals a builtinTrim2ArgsSig, corresponding to trim(str, remstr)
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_trim
func (b *builtinTrim2ArgsSig) evalString(row chunk.Row) (d string, isNull bool, err error) {
	var str, remstr string

	str, isNull, err = b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return d, isNull, errors.Trace(err)
	}
	remstr, isNull, err = b.args[1].EvalString(b.ctx, row)
	if isNull || err != nil {
		return d, isNull, errors.Trace(err)
	}
	d = trimLeft(str, remstr)
	d = trimRight(d, remstr)
	return d, false, nil
}

type builtinTrim3ArgsSig struct {
	baseBuiltinFunc
}

func (b *builtinTrim3ArgsSig) Clone() builtinFunc {
	newSig := &builtinTrim3ArgsSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString evals a builtinTrim3ArgsSig, corresponding to trim(str, remstr, direction)
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_trim
func (b *builtinTrim3ArgsSig) evalString(row chunk.Row) (d string, isNull bool, err error) {
	var (
		str, remstr  string
		x            int64
		direction    ast.TrimDirectionType
		isRemStrNull bool
	)
	str, isNull, err = b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return d, isNull, errors.Trace(err)
	}
	remstr, isRemStrNull, err = b.args[1].EvalString(b.ctx, row)
	if err != nil {
		return d, isNull, errors.Trace(err)
	}
	x, isNull, err = b.args[2].EvalInt(b.ctx, row)
	if isNull || err != nil {
		return d, isNull, errors.Trace(err)
	}
	direction = ast.TrimDirectionType(x)
	if direction == ast.TrimLeading {
		if isRemStrNull {
			d = strings.TrimLeft(str, spaceChars)
		} else {
			d = trimLeft(str, remstr)
		}
	} else if direction == ast.TrimTrailing {
		if isRemStrNull {
			d = strings.TrimRight(str, spaceChars)
		} else {
			d = trimRight(str, remstr)
		}
	} else {
		if isRemStrNull {
			d = strings.Trim(str, spaceChars)
		} else {
			d = trimLeft(str, remstr)
			d = trimRight(d, remstr)
		}
	}
	return d, false, nil
}

type lTrimFunctionClass struct {
	baseFunctionClass
}

func (c *lTrimFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETString, types.ETString)
	argType := args[0].GetType()
	bf.tp.Flen = argType.Flen
	SetBinFlagOrBinStr(argType, bf.tp)
	sig := &builtinLTrimSig{bf}
	return sig, nil
}

type builtinLTrimSig struct {
	baseBuiltinFunc
}

func (b *builtinLTrimSig) Clone() builtinFunc {
	newSig := &builtinLTrimSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString evals a builtinLTrimSig
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_ltrim
func (b *builtinLTrimSig) evalString(row chunk.Row) (d string, isNull bool, err error) {
	d, isNull, err = b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return d, isNull, errors.Trace(err)
	}
	return strings.TrimLeft(d, spaceChars), false, nil
}

type rTrimFunctionClass struct {
	baseFunctionClass
}

func (c *rTrimFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETString, types.ETString)
	argType := args[0].GetType()
	bf.tp.Flen = argType.Flen
	SetBinFlagOrBinStr(argType, bf.tp)
	sig := &builtinRTrimSig{bf}
	return sig, nil
}

type builtinRTrimSig struct {
	baseBuiltinFunc
}

func (b *builtinRTrimSig) Clone() builtinFunc {
	newSig := &builtinRTrimSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString evals a builtinRTrimSig
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_rtrim
func (b *builtinRTrimSig) evalString(row chunk.Row) (d string, isNull bool, err error) {
	d, isNull, err = b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return d, isNull, errors.Trace(err)
	}
	return strings.TrimRight(d, spaceChars), false, nil
}

func trimLeft(str, remstr string) string {
	for {
		x := strings.TrimPrefix(str, remstr)
		if len(x) == len(str) {
			return x
		}
		str = x
	}
}

func trimRight(str, remstr string) string {
	for {
		x := strings.TrimSuffix(str, remstr)
		if len(x) == len(str) {
			return x
		}
		str = x
	}
}

func getFlen4LpadAndRpad(ctx sessionctx.Context, arg Expression) int {
	if constant, ok := arg.(*Constant); ok {
		length, isNull, err := constant.EvalInt(ctx, chunk.Row{})
		if err != nil {
			log.Errorf("getFlen4LpadAndRpad with error: %v", err.Error())
		}
		if isNull || err != nil || length > mysql.MaxBlobWidth {
			return mysql.MaxBlobWidth
		}
		return int(length)
	}
	return mysql.MaxBlobWidth
}

type lpadFunctionClass struct {
	baseFunctionClass
}

func (c *lpadFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETString, types.ETString, types.ETInt, types.ETString)
	bf.tp.Flen = getFlen4LpadAndRpad(bf.ctx, args[1])
	SetBinFlagOrBinStr(args[0].GetType(), bf.tp)
	SetBinFlagOrBinStr(args[2].GetType(), bf.tp)

	valStr, _ := ctx.GetSessionVars().GetSystemVar(variable.MaxAllowedPacket)
	maxAllowedPacket, err := strconv.ParseUint(valStr, 10, 64)
	if err != nil {
		return nil, errors.Trace(err)
	}

	if types.IsBinaryStr(args[0].GetType()) || types.IsBinaryStr(args[2].GetType()) {
		sig := &builtinLpadBinarySig{bf, maxAllowedPacket}
		return sig, nil
	}
	if bf.tp.Flen *= 4; bf.tp.Flen > mysql.MaxBlobWidth {
		bf.tp.Flen = mysql.MaxBlobWidth
	}
	sig := &builtinLpadSig{bf, maxAllowedPacket}
	return sig, nil
}

type builtinLpadBinarySig struct {
	baseBuiltinFunc
	maxAllowedPacket uint64
}

func (b *builtinLpadBinarySig) Clone() builtinFunc {
	newSig := &builtinLpadBinarySig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	newSig.maxAllowedPacket = b.maxAllowedPacket
	return newSig
}

// evalString evals LPAD(str,len,padstr).
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_lpad
func (b *builtinLpadBinarySig) evalString(row chunk.Row) (string, bool, error) {
	str, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}
	byteLength := len(str)

	length, isNull, err := b.args[1].EvalInt(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}
	targetLength := int(length)

	if uint64(targetLength) > b.maxAllowedPacket {
		b.ctx.GetSessionVars().StmtCtx.AppendWarning(errWarnAllowedPacketOverflowed.GenWithStackByArgs("lpad", b.maxAllowedPacket))
		return "", true, nil
	}

	padStr, isNull, err := b.args[2].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}
	padLength := len(padStr)

	if targetLength < 0 || targetLength > b.tp.Flen || (byteLength < targetLength && padLength == 0) {
		return "", true, nil
	}

	if tailLen := targetLength - byteLength; tailLen > 0 {
		repeatCount := tailLen/padLength + 1
		str = strings.Repeat(padStr, repeatCount)[:tailLen] + str
	}
	return str[:targetLength], false, nil
}

type builtinLpadSig struct {
	baseBuiltinFunc
	maxAllowedPacket uint64
}

func (b *builtinLpadSig) Clone() builtinFunc {
	newSig := &builtinLpadSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	newSig.maxAllowedPacket = b.maxAllowedPacket
	return newSig
}

// evalString evals LPAD(str,len,padstr).
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_lpad
func (b *builtinLpadSig) evalString(row chunk.Row) (string, bool, error) {
	str, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}
	runeLength := len([]rune(str))

	length, isNull, err := b.args[1].EvalInt(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}
	targetLength := int(length)

	if uint64(targetLength)*uint64(mysql.MaxBytesOfCharacter) > b.maxAllowedPacket {
		b.ctx.GetSessionVars().StmtCtx.AppendWarning(errWarnAllowedPacketOverflowed.GenWithStackByArgs("lpad", b.maxAllowedPacket))
		return "", true, nil
	}

	padStr, isNull, err := b.args[2].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}
	padLength := len([]rune(padStr))

	if targetLength < 0 || targetLength*4 > b.tp.Flen || (runeLength < targetLength && padLength == 0) {
		return "", true, nil
	}

	if tailLen := targetLength - runeLength; tailLen > 0 {
		repeatCount := tailLen/padLength + 1
		str = string([]rune(strings.Repeat(padStr, repeatCount))[:tailLen]) + str
	}
	return string([]rune(str)[:targetLength]), false, nil
}

type rpadFunctionClass struct {
	baseFunctionClass
}

func (c *rpadFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETString, types.ETString, types.ETInt, types.ETString)
	bf.tp.Flen = getFlen4LpadAndRpad(bf.ctx, args[1])
	SetBinFlagOrBinStr(args[0].GetType(), bf.tp)
	SetBinFlagOrBinStr(args[2].GetType(), bf.tp)

	valStr, _ := ctx.GetSessionVars().GetSystemVar(variable.MaxAllowedPacket)
	maxAllowedPacket, err := strconv.ParseUint(valStr, 10, 64)
	if err != nil {
		return nil, errors.Trace(err)
	}

	if types.IsBinaryStr(args[0].GetType()) || types.IsBinaryStr(args[2].GetType()) {
		sig := &builtinRpadBinarySig{bf, maxAllowedPacket}
		return sig, nil
	}
	if bf.tp.Flen *= 4; bf.tp.Flen > mysql.MaxBlobWidth {
		bf.tp.Flen = mysql.MaxBlobWidth
	}
	sig := &builtinRpadSig{bf, maxAllowedPacket}
	return sig, nil
}

type builtinRpadBinarySig struct {
	baseBuiltinFunc
	maxAllowedPacket uint64
}

func (b *builtinRpadBinarySig) Clone() builtinFunc {
	newSig := &builtinRpadBinarySig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	newSig.maxAllowedPacket = b.maxAllowedPacket
	return newSig
}

// evalString evals RPAD(str,len,padstr).
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_rpad
func (b *builtinRpadBinarySig) evalString(row chunk.Row) (string, bool, error) {
	str, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}
	byteLength := len(str)

	length, isNull, err := b.args[1].EvalInt(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}
	targetLength := int(length)
	if uint64(targetLength) > b.maxAllowedPacket {
		b.ctx.GetSessionVars().StmtCtx.AppendWarning(errWarnAllowedPacketOverflowed.GenWithStackByArgs("rpad", b.maxAllowedPacket))
		return "", true, nil
	}

	padStr, isNull, err := b.args[2].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}
	padLength := len(padStr)

	if targetLength < 0 || targetLength > b.tp.Flen || (byteLength < targetLength && padLength == 0) {
		return "", true, nil
	}

	if tailLen := targetLength - byteLength; tailLen > 0 {
		repeatCount := tailLen/padLength + 1
		str = str + strings.Repeat(padStr, repeatCount)
	}
	return str[:targetLength], false, nil
}

type builtinRpadSig struct {
	baseBuiltinFunc
	maxAllowedPacket uint64
}

func (b *builtinRpadSig) Clone() builtinFunc {
	newSig := &builtinRpadSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	newSig.maxAllowedPacket = b.maxAllowedPacket
	return newSig
}

// evalString evals RPAD(str,len,padstr).
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_rpad
func (b *builtinRpadSig) evalString(row chunk.Row) (string, bool, error) {
	str, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}
	runeLength := len([]rune(str))

	length, isNull, err := b.args[1].EvalInt(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}
	targetLength := int(length)

	if uint64(targetLength)*uint64(mysql.MaxBytesOfCharacter) > b.maxAllowedPacket {
		b.ctx.GetSessionVars().StmtCtx.AppendWarning(errWarnAllowedPacketOverflowed.GenWithStackByArgs("rpad", b.maxAllowedPacket))
		return "", true, nil
	}

	padStr, isNull, err := b.args[2].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}
	padLength := len([]rune(padStr))

	if targetLength < 0 || targetLength*4 > b.tp.Flen || (runeLength < targetLength && padLength == 0) {
		return "", true, nil
	}

	if tailLen := targetLength - runeLength; tailLen > 0 {
		repeatCount := tailLen/padLength + 1
		str = str + strings.Repeat(padStr, repeatCount)
	}
	return string([]rune(str)[:targetLength]), false, nil
}

type bitLengthFunctionClass struct {
	baseFunctionClass
}

func (c *bitLengthFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETInt, types.ETString)
	bf.tp.Flen = 10
	sig := &builtinBitLengthSig{bf}
	return sig, nil
}

type builtinBitLengthSig struct {
	baseBuiltinFunc
}

func (b *builtinBitLengthSig) Clone() builtinFunc {
	newSig := &builtinBitLengthSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalInt evaluates a builtinBitLengthSig.
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_bit-length
func (b *builtinBitLengthSig) evalInt(row chunk.Row) (int64, bool, error) {
	val, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return 0, isNull, errors.Trace(err)
	}

	return int64(len(val) * 8), false, nil
}

type charFunctionClass struct {
	baseFunctionClass
}

func (c *charFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	argTps := make([]types.EvalType, 0, len(args))
	for i := 0; i < len(args)-1; i++ {
		argTps = append(argTps, types.ETInt)
	}
	argTps = append(argTps, types.ETString)
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETString, argTps...)
	bf.tp.Flen = 4 * (len(args) - 1)
	types.SetBinChsClnFlag(bf.tp)

	sig := &builtinCharSig{bf}
	return sig, nil
}

type builtinCharSig struct {
	baseBuiltinFunc
}

func (b *builtinCharSig) Clone() builtinFunc {
	newSig := &builtinCharSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

func (b *builtinCharSig) convertToBytes(ints []int64) []byte {
	buffer := bytes.NewBuffer([]byte{})
	for i := len(ints) - 1; i >= 0; i-- {
		for count, val := 0, ints[i]; count < 4; count++ {
			buffer.WriteByte(byte(val & 0xff))
			if val >>= 8; val == 0 {
				break
			}
		}
	}
	return reverseBytes(buffer.Bytes())
}

// evalString evals CHAR(N,... [USING charset_name]).
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_char.
func (b *builtinCharSig) evalString(row chunk.Row) (string, bool, error) {
	bigints := make([]int64, 0, len(b.args)-1)

	for i := 0; i < len(b.args)-1; i++ {
		val, IsNull, err := b.args[i].EvalInt(b.ctx, row)
		if err != nil {
			return "", true, errors.Trace(err)
		}
		if IsNull {
			continue
		}
		bigints = append(bigints, val)
	}
	// The last argument represents the charset name after "using".
	// Use default charset utf8 if it is nil.
	argCharset, IsNull, err := b.args[len(b.args)-1].EvalString(b.ctx, row)
	if err != nil {
		return "", true, errors.Trace(err)
	}

	result := string(b.convertToBytes(bigints))
	charsetLabel := strings.ToLower(argCharset)
	if IsNull || charsetLabel == "ascii" || strings.HasPrefix(charsetLabel, "utf8") {
		return result, false, nil
	}

	encoding, charsetName := charset.Lookup(charsetLabel)
	if encoding == nil {
		return "", true, errors.Errorf("unknown encoding: %s", argCharset)
	}

	oldStr := result
	result, _, err = transform.String(encoding.NewDecoder(), result)
	if err != nil {
		log.Errorf("Convert %s to %s with error: %v", oldStr, charsetName, err.Error())
		return "", true, errors.Trace(err)
	}
	return result, false, nil
}

type charLengthFunctionClass struct {
	baseFunctionClass
}

func (c *charLengthFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if argsErr := c.verifyArgs(args); argsErr != nil {
		return nil, errors.Trace(argsErr)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETInt, types.ETString)
	if types.IsBinaryStr(args[0].GetType()) {
		sig := &builtinCharLengthBinarySig{bf}
		return sig, nil
	}
	sig := &builtinCharLengthSig{bf}
	return sig, nil
}

type builtinCharLengthBinarySig struct {
	baseBuiltinFunc
}

func (b *builtinCharLengthBinarySig) Clone() builtinFunc {
	newSig := &builtinCharLengthBinarySig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalInt evals a builtinCharLengthSig for binary string type.
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_char-length
func (b *builtinCharLengthBinarySig) evalInt(row chunk.Row) (int64, bool, error) {
	val, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return 0, isNull, errors.Trace(err)
	}
	return int64(len(val)), false, nil
}

type builtinCharLengthSig struct {
	baseBuiltinFunc
}

func (b *builtinCharLengthSig) Clone() builtinFunc {
	newSig := &builtinCharLengthSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalInt evals a builtinCharLengthSig for non-binary string type.
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_char-length
func (b *builtinCharLengthSig) evalInt(row chunk.Row) (int64, bool, error) {
	val, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return 0, isNull, errors.Trace(err)
	}
	return int64(len([]rune(val))), false, nil
}

type findInSetFunctionClass struct {
	baseFunctionClass
}

func (c *findInSetFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETInt, types.ETString, types.ETString)
	bf.tp.Flen = 3
	sig := &builtinFindInSetSig{bf}
	return sig, nil
}

type builtinFindInSetSig struct {
	baseBuiltinFunc
}

func (b *builtinFindInSetSig) Clone() builtinFunc {
	newSig := &builtinFindInSetSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalInt evals FIND_IN_SET(str,strlist).
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_find-in-set
// TODO: This function can be optimized by using bit arithmetic when the first argument is
// a constant string and the second is a column of type SET.
func (b *builtinFindInSetSig) evalInt(row chunk.Row) (int64, bool, error) {
	str, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return 0, isNull, errors.Trace(err)
	}

	strlist, isNull, err := b.args[1].EvalString(b.ctx, row)
	if isNull || err != nil {
		return 0, isNull, errors.Trace(err)
	}

	if len(strlist) == 0 {
		return 0, false, nil
	}

	for i, strInSet := range strings.Split(strlist, ",") {
		if str == strInSet {
			return int64(i + 1), false, nil
		}
	}
	return 0, false, nil
}

type fieldFunctionClass struct {
	baseFunctionClass
}

func (c *fieldFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}

	isAllString, isAllNumber := true, true
	for i, length := 0, len(args); i < length; i++ {
		argTp := args[i].GetType().EvalType()
		isAllString = isAllString && (argTp == types.ETString)
		isAllNumber = isAllNumber && (argTp == types.ETInt)
	}

	argTps := make([]types.EvalType, len(args))
	argTp := types.ETReal
	if isAllString {
		argTp = types.ETString
	} else if isAllNumber {
		argTp = types.ETInt
	}
	for i, length := 0, len(args); i < length; i++ {
		argTps[i] = argTp
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETInt, argTps...)
	var sig builtinFunc
	switch argTp {
	case types.ETReal:
		sig = &builtinFieldRealSig{bf}
	case types.ETInt:
		sig = &builtinFieldIntSig{bf}
	case types.ETString:
		sig = &builtinFieldStringSig{bf}
	}
	return sig, nil
}

type builtinFieldIntSig struct {
	baseBuiltinFunc
}

func (b *builtinFieldIntSig) Clone() builtinFunc {
	newSig := &builtinFieldIntSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalInt evals FIELD(str,str1,str2,str3,...).
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_field
func (b *builtinFieldIntSig) evalInt(row chunk.Row) (int64, bool, error) {
	str, isNull, err := b.args[0].EvalInt(b.ctx, row)
	if isNull || err != nil {
		return 0, err != nil, errors.Trace(err)
	}
	for i, length := 1, len(b.args); i < length; i++ {
		stri, isNull, err := b.args[i].EvalInt(b.ctx, row)
		if err != nil {
			return 0, true, errors.Trace(err)
		}
		if !isNull && str == stri {
			return int64(i), false, nil
		}
	}
	return 0, false, nil
}

type builtinFieldRealSig struct {
	baseBuiltinFunc
}

func (b *builtinFieldRealSig) Clone() builtinFunc {
	newSig := &builtinFieldRealSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalInt evals FIELD(str,str1,str2,str3,...).
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_field
func (b *builtinFieldRealSig) evalInt(row chunk.Row) (int64, bool, error) {
	str, isNull, err := b.args[0].EvalReal(b.ctx, row)
	if isNull || err != nil {
		return 0, err != nil, errors.Trace(err)
	}
	for i, length := 1, len(b.args); i < length; i++ {
		stri, isNull, err := b.args[i].EvalReal(b.ctx, row)
		if err != nil {
			return 0, true, errors.Trace(err)
		}
		if !isNull && str == stri {
			return int64(i), false, nil
		}
	}
	return 0, false, nil
}

type builtinFieldStringSig struct {
	baseBuiltinFunc
}

func (b *builtinFieldStringSig) Clone() builtinFunc {
	newSig := &builtinFieldStringSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalInt evals FIELD(str,str1,str2,str3,...).
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_field
func (b *builtinFieldStringSig) evalInt(row chunk.Row) (int64, bool, error) {
	str, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return 0, err != nil, errors.Trace(err)
	}
	for i, length := 1, len(b.args); i < length; i++ {
		stri, isNull, err := b.args[i].EvalString(b.ctx, row)
		if err != nil {
			return 0, true, errors.Trace(err)
		}
		if !isNull && str == stri {
			return int64(i), false, nil
		}
	}
	return 0, false, nil
}

type makeSetFunctionClass struct {
	baseFunctionClass
}

func (c *makeSetFunctionClass) getFlen(ctx sessionctx.Context, args []Expression) int {
	flen, count := 0, 0
	if constant, ok := args[0].(*Constant); ok {
		bits, isNull, err := constant.EvalInt(ctx, chunk.Row{})
		if err == nil && !isNull {
			for i, length := 1, len(args); i < length; i++ {
				if (bits & (1 << uint(i-1))) != 0 {
					flen += args[i].GetType().Flen
					count++
				}
			}
			if count > 0 {
				flen += count - 1
			}
			return flen
		}
	}
	for i, length := 1, len(args); i < length; i++ {
		flen += args[i].GetType().Flen
	}
	return flen + len(args) - 1 - 1
}

func (c *makeSetFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	argTps := make([]types.EvalType, len(args))
	argTps[0] = types.ETInt
	for i, length := 1, len(args); i < length; i++ {
		argTps[i] = types.ETString
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETString, argTps...)
	for i, length := 0, len(args); i < length; i++ {
		SetBinFlagOrBinStr(args[i].GetType(), bf.tp)
	}
	bf.tp.Flen = c.getFlen(bf.ctx, args)
	if bf.tp.Flen > mysql.MaxBlobWidth {
		bf.tp.Flen = mysql.MaxBlobWidth
	}
	sig := &builtinMakeSetSig{bf}
	return sig, nil
}

type builtinMakeSetSig struct {
	baseBuiltinFunc
}

func (b *builtinMakeSetSig) Clone() builtinFunc {
	newSig := &builtinMakeSetSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString evals MAKE_SET(bits,str1,str2,...).
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_make-set
func (b *builtinMakeSetSig) evalString(row chunk.Row) (string, bool, error) {
	bits, isNull, err := b.args[0].EvalInt(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}

	sets := make([]string, 0, len(b.args)-1)
	for i, length := 1, len(b.args); i < length; i++ {
		if (bits & (1 << uint(i-1))) == 0 {
			continue
		}
		str, isNull, err := b.args[i].EvalString(b.ctx, row)
		if err != nil {
			return "", true, errors.Trace(err)
		}
		if !isNull {
			sets = append(sets, str)
		}
	}

	return strings.Join(sets, ","), false, nil
}

type octFunctionClass struct {
	baseFunctionClass
}

func (c *octFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	var sig builtinFunc
	if IsBinaryLiteral(args[0]) || args[0].GetType().EvalType() == types.ETInt {
		bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETString, types.ETInt)
		bf.tp.Flen, bf.tp.Decimal = 64, types.UnspecifiedLength
		sig = &builtinOctIntSig{bf}
	} else {
		bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETString, types.ETString)
		bf.tp.Flen, bf.tp.Decimal = 64, types.UnspecifiedLength
		sig = &builtinOctStringSig{bf}
	}

	return sig, nil
}

type builtinOctIntSig struct {
	baseBuiltinFunc
}

func (b *builtinOctIntSig) Clone() builtinFunc {
	newSig := &builtinOctIntSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString evals OCT(N).
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_oct
func (b *builtinOctIntSig) evalString(row chunk.Row) (string, bool, error) {
	val, isNull, err := b.args[0].EvalInt(b.ctx, row)
	if isNull || err != nil {
		return "", isNull, errors.Trace(err)
	}

	return strconv.FormatUint(uint64(val), 8), false, nil
}

type builtinOctStringSig struct {
	baseBuiltinFunc
}

func (b *builtinOctStringSig) Clone() builtinFunc {
	newSig := &builtinOctStringSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString evals OCT(N).
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_oct
func (b *builtinOctStringSig) evalString(row chunk.Row) (string, bool, error) {
	val, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", isNull, errors.Trace(err)
	}

	negative, overflow := false, false
	val = getValidPrefix(strings.TrimSpace(val), 10)
	if len(val) == 0 {
		return "0", false, nil
	}

	if val[0] == '-' {
		negative, val = true, val[1:]
	}
	numVal, err := strconv.ParseUint(val, 10, 64)
	if err != nil {
		numError, ok := err.(*strconv.NumError)
		if !ok || numError.Err != strconv.ErrRange {
			return "", true, errors.Trace(err)
		}
		overflow = true
	}
	if negative && !overflow {
		numVal = -numVal
	}
	return strconv.FormatUint(numVal, 8), false, nil
}

type ordFunctionClass struct {
	baseFunctionClass
}

func (c *ordFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETInt, types.ETString)
	bf.tp.Flen = 10
	sig := &builtinOrdSig{bf}
	return sig, nil
}

type builtinOrdSig struct {
	baseBuiltinFunc
}

func (b *builtinOrdSig) Clone() builtinFunc {
	newSig := &builtinOrdSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalInt evals a builtinOrdSig.
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_ord
func (b *builtinOrdSig) evalInt(row chunk.Row) (int64, bool, error) {
	str, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return 0, isNull, errors.Trace(err)
	}
	if len(str) == 0 {
		return 0, false, nil
	}

	_, size := utf8.DecodeRuneInString(str)
	leftMost := str[:size]
	var result int64
	var factor int64 = 1
	for i := len(leftMost) - 1; i >= 0; i-- {
		result += int64(leftMost[i]) * factor
		factor *= 256
	}

	return result, false, nil
}

type quoteFunctionClass struct {
	baseFunctionClass
}

func (c *quoteFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETString, types.ETString)
	SetBinFlagOrBinStr(args[0].GetType(), bf.tp)
	bf.tp.Flen = 2*args[0].GetType().Flen + 2
	if bf.tp.Flen > mysql.MaxBlobWidth {
		bf.tp.Flen = mysql.MaxBlobWidth
	}
	sig := &builtinQuoteSig{bf}
	return sig, nil
}

type builtinQuoteSig struct {
	baseBuiltinFunc
}

func (b *builtinQuoteSig) Clone() builtinFunc {
	newSig := &builtinQuoteSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString evals QUOTE(str).
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_quote
func (b *builtinQuoteSig) evalString(row chunk.Row) (string, bool, error) {
	str, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}

	runes := []rune(str)
	buffer := bytes.NewBufferString("")
	buffer.WriteRune('\'')
	for i, runeLength := 0, len(runes); i < runeLength; i++ {
		switch runes[i] {
		case '\\', '\'':
			buffer.WriteRune('\\')
			buffer.WriteRune(runes[i])
		case 0:
			buffer.WriteRune('\\')
			buffer.WriteRune('0')
		case '\032':
			buffer.WriteRune('\\')
			buffer.WriteRune('Z')
		default:
			buffer.WriteRune(runes[i])
		}
	}
	buffer.WriteRune('\'')

	return buffer.String(), false, nil
}

type binFunctionClass struct {
	baseFunctionClass
}

func (c *binFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETString, types.ETInt)
	bf.tp.Flen = 64
	sig := &builtinBinSig{bf}
	return sig, nil
}

type builtinBinSig struct {
	baseBuiltinFunc
}

func (b *builtinBinSig) Clone() builtinFunc {
	newSig := &builtinBinSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString evals BIN(N).
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_bin
func (b *builtinBinSig) evalString(row chunk.Row) (string, bool, error) {
	val, isNull, err := b.args[0].EvalInt(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}
	return fmt.Sprintf("%b", uint64(val)), false, nil
}

type eltFunctionClass struct {
	baseFunctionClass
}

func (c *eltFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if argsErr := c.verifyArgs(args); argsErr != nil {
		return nil, errors.Trace(argsErr)
	}
	argTps := make([]types.EvalType, 0, len(args))
	argTps = append(argTps, types.ETInt)
	for i := 1; i < len(args); i++ {
		argTps = append(argTps, types.ETString)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETString, argTps...)
	for _, arg := range args[1:] {
		argType := arg.GetType()
		if types.IsBinaryStr(argType) {
			types.SetBinChsClnFlag(bf.tp)
		}
		if argType.Flen > bf.tp.Flen {
			bf.tp.Flen = argType.Flen
		}
	}
	sig := &builtinEltSig{bf}
	return sig, nil
}

type builtinEltSig struct {
	baseBuiltinFunc
}

func (b *builtinEltSig) Clone() builtinFunc {
	newSig := &builtinEltSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString evals a builtinEltSig.
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_elt
func (b *builtinEltSig) evalString(row chunk.Row) (string, bool, error) {
	idx, isNull, err := b.args[0].EvalInt(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}
	if idx < 1 || idx >= int64(len(b.args)) {
		return "", true, nil
	}
	arg, isNull, err := b.args[idx].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}
	return arg, false, nil
}

type exportSetFunctionClass struct {
	baseFunctionClass
}

func (c *exportSetFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (sig builtinFunc, err error) {
	if err = c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	argTps := make([]types.EvalType, 0, 5)
	argTps = append(argTps, types.ETInt, types.ETString, types.ETString)
	if len(args) > 3 {
		argTps = append(argTps, types.ETString)
	}
	if len(args) > 4 {
		argTps = append(argTps, types.ETInt)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETString, argTps...)
	bf.tp.Flen = mysql.MaxBlobWidth
	switch len(args) {
	case 3:
		sig = &builtinExportSet3ArgSig{bf}
	case 4:
		sig = &builtinExportSet4ArgSig{bf}
	case 5:
		sig = &builtinExportSet5ArgSig{bf}
	}
	return sig, nil
}

// exportSet evals EXPORT_SET(bits,on,off,separator,number_of_bits).
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_export-set
func exportSet(bits int64, on, off, separator string, numberOfBits int64) string {
	result := ""
	for i := uint64(0); i < uint64(numberOfBits); i++ {
		if (bits & (1 << i)) > 0 {
			result += on
		} else {
			result += off
		}
		if i < uint64(numberOfBits)-1 {
			result += separator
		}
	}
	return result
}

type builtinExportSet3ArgSig struct {
	baseBuiltinFunc
}

func (b *builtinExportSet3ArgSig) Clone() builtinFunc {
	newSig := &builtinExportSet3ArgSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString evals EXPORT_SET(bits,on,off).
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_export-set
func (b *builtinExportSet3ArgSig) evalString(row chunk.Row) (string, bool, error) {
	bits, isNull, err := b.args[0].EvalInt(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}

	on, isNull, err := b.args[1].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}

	off, isNull, err := b.args[2].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}

	return exportSet(bits, on, off, ",", 64), false, nil
}

type builtinExportSet4ArgSig struct {
	baseBuiltinFunc
}

func (b *builtinExportSet4ArgSig) Clone() builtinFunc {
	newSig := &builtinExportSet4ArgSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString evals EXPORT_SET(bits,on,off,separator).
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_export-set
func (b *builtinExportSet4ArgSig) evalString(row chunk.Row) (string, bool, error) {
	bits, isNull, err := b.args[0].EvalInt(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}

	on, isNull, err := b.args[1].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}

	off, isNull, err := b.args[2].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}

	separator, isNull, err := b.args[3].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}

	return exportSet(bits, on, off, separator, 64), false, nil
}

type builtinExportSet5ArgSig struct {
	baseBuiltinFunc
}

func (b *builtinExportSet5ArgSig) Clone() builtinFunc {
	newSig := &builtinExportSet5ArgSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString evals EXPORT_SET(bits,on,off,separator,number_of_bits).
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_export-set
func (b *builtinExportSet5ArgSig) evalString(row chunk.Row) (string, bool, error) {
	bits, isNull, err := b.args[0].EvalInt(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}

	on, isNull, err := b.args[1].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}

	off, isNull, err := b.args[2].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}

	separator, isNull, err := b.args[3].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}

	numberOfBits, isNull, err := b.args[4].EvalInt(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}
	if numberOfBits < 0 || numberOfBits > 64 {
		numberOfBits = 64
	}

	return exportSet(bits, on, off, separator, numberOfBits), false, nil
}

type formatFunctionClass struct {
	baseFunctionClass
}

func (c *formatFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	argTps := make([]types.EvalType, 2, 3)
	argTps[0], argTps[1] = types.ETString, types.ETString
	if len(args) == 3 {
		argTps = append(argTps, types.ETString)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETString, argTps...)
	bf.tp.Flen = mysql.MaxBlobWidth
	var sig builtinFunc
	if len(args) == 3 {
		sig = &builtinFormatWithLocaleSig{bf}
	} else {
		sig = &builtinFormatSig{bf}
	}
	return sig, nil
}

type builtinFormatWithLocaleSig struct {
	baseBuiltinFunc
}

func (b *builtinFormatWithLocaleSig) Clone() builtinFunc {
	newSig := &builtinFormatWithLocaleSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString evals FORMAT(X,D,locale).
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_format
func (b *builtinFormatWithLocaleSig) evalString(row chunk.Row) (string, bool, error) {
	x, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}

	d, isNull, err := b.args[1].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}

	locale, isNull, err := b.args[2].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}

	formatString, err := mysql.GetLocaleFormatFunction(locale)(x, d)
	return formatString, err != nil, errors.Trace(err)
}

type builtinFormatSig struct {
	baseBuiltinFunc
}

func (b *builtinFormatSig) Clone() builtinFunc {
	newSig := &builtinFormatSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString evals FORMAT(X,D).
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_format
func (b *builtinFormatSig) evalString(row chunk.Row) (string, bool, error) {
	x, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}

	d, isNull, err := b.args[1].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}

	formatString, err := mysql.GetLocaleFormatFunction("en_US")(x, d)
	return formatString, err != nil, errors.Trace(err)
}

type fromBase64FunctionClass struct {
	baseFunctionClass
}

func (c *fromBase64FunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETString, types.ETString)
	bf.tp.Flen = mysql.MaxBlobWidth

	valStr, _ := ctx.GetSessionVars().GetSystemVar(variable.MaxAllowedPacket)
	maxAllowedPacket, err := strconv.ParseUint(valStr, 10, 64)
	if err != nil {
		return nil, errors.Trace(err)
	}

	types.SetBinChsClnFlag(bf.tp)
	sig := &builtinFromBase64Sig{bf, maxAllowedPacket}
	return sig, nil
}

// base64NeededDecodedLength return the base64 decoded string length.
func base64NeededDecodedLength(n int) int {
	// Returns -1 indicate the result will overflow.
	if strconv.IntSize == 64 && n > math.MaxInt64/3 {
		return -1
	}
	if strconv.IntSize == 32 && n > math.MaxInt32/3 {
		return -1
	}
	return n * 3 / 4
}

type builtinFromBase64Sig struct {
	baseBuiltinFunc
	maxAllowedPacket uint64
}

func (b *builtinFromBase64Sig) Clone() builtinFunc {
	newSig := &builtinFromBase64Sig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	newSig.maxAllowedPacket = b.maxAllowedPacket
	return newSig
}

// evalString evals FROM_BASE64(str).
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_from-base64
func (b *builtinFromBase64Sig) evalString(row chunk.Row) (string, bool, error) {
	str, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}

	needDecodeLen := base64NeededDecodedLength(len(str))
	if needDecodeLen == -1 {
		return "", true, nil
	}
	if needDecodeLen > int(b.maxAllowedPacket) {
		b.ctx.GetSessionVars().StmtCtx.AppendWarning(errWarnAllowedPacketOverflowed.GenWithStackByArgs("from_base64", b.maxAllowedPacket))
		return "", true, nil
	}

	str = strings.Replace(str, "\t", "", -1)
	str = strings.Replace(str, " ", "", -1)
	result, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		// When error happens, take `from_base64("asc")` as an example, we should return NULL.
		return "", true, nil
	}
	return string(result), false, nil
}

type toBase64FunctionClass struct {
	baseFunctionClass
}

func (c *toBase64FunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETString, types.ETString)
	bf.tp.Flen = base64NeededEncodedLength(bf.args[0].GetType().Flen)

	valStr, _ := ctx.GetSessionVars().GetSystemVar(variable.MaxAllowedPacket)
	maxAllowedPacket, err := strconv.ParseUint(valStr, 10, 64)
	if err != nil {
		return nil, errors.Trace(err)
	}

	sig := &builtinToBase64Sig{bf, maxAllowedPacket}
	return sig, nil
}

type builtinToBase64Sig struct {
	baseBuiltinFunc
	maxAllowedPacket uint64
}

func (b *builtinToBase64Sig) Clone() builtinFunc {
	newSig := &builtinToBase64Sig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	newSig.maxAllowedPacket = b.maxAllowedPacket
	return newSig
}

// base64NeededEncodedLength return the base64 encoded string length.
func base64NeededEncodedLength(n int) int {
	// Returns -1 indicate the result will overflow.
	if strconv.IntSize == 64 {
		// len(arg)            -> len(to_base64(arg))
		// 6827690988321067803 -> 9223372036854775804
		// 6827690988321067804 -> -9223372036854775808
		if n > 6827690988321067803 {
			return -1
		}
	} else {
		// len(arg)   -> len(to_base64(arg))
		// 1589695686 -> 2147483645
		// 1589695687 -> -2147483646
		if n > 1589695686 {
			return -1
		}
	}

	length := (n + 2) / 3 * 4
	return length + (length-1)/76
}

// evalString evals a builtinToBase64Sig.
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_to-base64
func (b *builtinToBase64Sig) evalString(row chunk.Row) (d string, isNull bool, err error) {
	str, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", isNull, errors.Trace(err)
	}
	needEncodeLen := base64NeededEncodedLength(len(str))
	if needEncodeLen == -1 {
		return "", true, nil
	}
	if needEncodeLen > int(b.maxAllowedPacket) {
		b.ctx.GetSessionVars().StmtCtx.AppendWarning(errWarnAllowedPacketOverflowed.GenWithStackByArgs("to_base64", b.maxAllowedPacket))
		return "", true, nil
	}
	if b.tp.Flen == -1 || b.tp.Flen > mysql.MaxBlobWidth {
		return "", true, nil
	}

	//encode
	strBytes := []byte(str)
	result := base64.StdEncoding.EncodeToString(strBytes)
	//A newline is added after each 76 characters of encoded output to divide long output into multiple lines.
	count := len(result)
	if count > 76 {
		resultArr := splitToSubN(result, 76)
		result = strings.Join(resultArr, "\n")
	}

	return result, false, nil
}

// splitToSubN splits a string every n runes into a string[]
func splitToSubN(s string, n int) []string {
	subs := make([]string, 0, len(s)/n+1)
	for len(s) > n {
		subs = append(subs, s[:n])
		s = s[n:]
	}
	subs = append(subs, s)
	return subs
}

type insertFunctionClass struct {
	baseFunctionClass
}

func (c *insertFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (sig builtinFunc, err error) {
	if err = c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETString, types.ETString, types.ETInt, types.ETInt, types.ETString)
	bf.tp.Flen = mysql.MaxBlobWidth
	SetBinFlagOrBinStr(args[0].GetType(), bf.tp)
	SetBinFlagOrBinStr(args[3].GetType(), bf.tp)

	valStr, _ := ctx.GetSessionVars().GetSystemVar(variable.MaxAllowedPacket)
	maxAllowedPacket, err := strconv.ParseUint(valStr, 10, 64)
	if err != nil {
		return nil, errors.Trace(err)
	}

	if types.IsBinaryStr(args[0].GetType()) {
		sig = &builtinInsertBinarySig{bf, maxAllowedPacket}
	} else {
		sig = &builtinInsertSig{bf, maxAllowedPacket}
	}
	return sig, nil
}

type builtinInsertBinarySig struct {
	baseBuiltinFunc
	maxAllowedPacket uint64
}

func (b *builtinInsertBinarySig) Clone() builtinFunc {
	newSig := &builtinInsertBinarySig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	newSig.maxAllowedPacket = b.maxAllowedPacket
	return newSig
}

// evalString evals INSERT(str,pos,len,newstr).
// See https://dev.mysql.com/doc/refman/5.6/en/string-functions.html#function_insert
func (b *builtinInsertBinarySig) evalString(row chunk.Row) (string, bool, error) {
	str, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}
	strLength := int64(len(str))

	pos, isNull, err := b.args[1].EvalInt(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}
	if pos < 1 || pos > strLength {
		return str, false, nil
	}

	length, isNull, err := b.args[2].EvalInt(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}

	newstr, isNull, err := b.args[3].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}

	if length > strLength-pos+1 || length < 0 {
		length = strLength - pos + 1
	}

	if uint64(strLength-length+int64(len(newstr))) > b.maxAllowedPacket {
		b.ctx.GetSessionVars().StmtCtx.AppendWarning(errWarnAllowedPacketOverflowed.GenWithStackByArgs("insert", b.maxAllowedPacket))
		return "", true, nil
	}

	return str[0:pos-1] + newstr + str[pos+length-1:], false, nil
}

type builtinInsertSig struct {
	baseBuiltinFunc
	maxAllowedPacket uint64
}

func (b *builtinInsertSig) Clone() builtinFunc {
	newSig := &builtinInsertSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	newSig.maxAllowedPacket = b.maxAllowedPacket
	return newSig
}

// evalString evals INSERT(str,pos,len,newstr).
// See https://dev.mysql.com/doc/refman/5.6/en/string-functions.html#function_insert
func (b *builtinInsertSig) evalString(row chunk.Row) (string, bool, error) {
	str, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}
	runes := []rune(str)
	runeLength := int64(len(runes))

	pos, isNull, err := b.args[1].EvalInt(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}
	if pos < 1 || pos > runeLength {
		return str, false, nil
	}

	length, isNull, err := b.args[2].EvalInt(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}

	newstr, isNull, err := b.args[3].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", true, errors.Trace(err)
	}

	if length > runeLength-pos+1 || length < 0 {
		length = runeLength - pos + 1
	}

	strHead := string(runes[0 : pos-1])
	strTail := string(runes[pos+length-1:])
	if uint64(len(strHead)+len(newstr)+len(strTail)) > b.maxAllowedPacket {
		b.ctx.GetSessionVars().StmtCtx.AppendWarning(errWarnAllowedPacketOverflowed.GenWithStackByArgs("insert", b.maxAllowedPacket))
		return "", true, nil
	}
	return strHead + newstr + strTail, false, nil
}

type instrFunctionClass struct {
	baseFunctionClass
}

func (c *instrFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETInt, types.ETString, types.ETString)
	bf.tp.Flen = 11
	if types.IsBinaryStr(bf.args[0].GetType()) || types.IsBinaryStr(bf.args[1].GetType()) {
		sig := &builtinInstrBinarySig{bf}
		return sig, nil
	}
	sig := &builtinInstrSig{bf}
	return sig, nil
}

type builtinInstrSig struct{ baseBuiltinFunc }

func (b *builtinInstrSig) Clone() builtinFunc {
	newSig := &builtinInstrSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

type builtinInstrBinarySig struct{ baseBuiltinFunc }

func (b *builtinInstrBinarySig) Clone() builtinFunc {
	newSig := &builtinInstrBinarySig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalInt evals INSTR(str,substr), case insensitive
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_instr
func (b *builtinInstrSig) evalInt(row chunk.Row) (int64, bool, error) {
	str, IsNull, err := b.args[0].EvalString(b.ctx, row)
	if IsNull || err != nil {
		return 0, true, errors.Trace(err)
	}
	str = strings.ToLower(str)

	substr, IsNull, err := b.args[1].EvalString(b.ctx, row)
	if IsNull || err != nil {
		return 0, true, errors.Trace(err)
	}
	substr = strings.ToLower(substr)

	idx := strings.Index(str, substr)
	if idx == -1 {
		return 0, false, nil
	}
	return int64(utf8.RuneCountInString(str[:idx]) + 1), false, nil
}

// evalInt evals INSTR(str,substr), case sensitive
// See https://dev.mysql.com/doc/refman/5.7/en/string-functions.html#function_instr
func (b *builtinInstrBinarySig) evalInt(row chunk.Row) (int64, bool, error) {
	str, IsNull, err := b.args[0].EvalString(b.ctx, row)
	if IsNull || err != nil {
		return 0, true, errors.Trace(err)
	}

	substr, IsNull, err := b.args[1].EvalString(b.ctx, row)
	if IsNull || err != nil {
		return 0, true, errors.Trace(err)
	}

	idx := strings.Index(str, substr)
	if idx == -1 {
		return 0, false, nil
	}
	return int64(idx + 1), false, nil
}

type loadFileFunctionClass struct {
	baseFunctionClass
}

func (c *loadFileFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	return nil, errFunctionNotExists.GenWithStackByArgs("FUNCTION", "load_file")
}
