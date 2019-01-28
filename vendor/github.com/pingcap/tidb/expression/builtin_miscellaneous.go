// Copyright 2017 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
// // Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package expression

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"strings"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/parser/mysql"
	"github.com/pingcap/tidb/sessionctx"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/types/json"
	"github.com/pingcap/tidb/util/chunk"
	"github.com/twinj/uuid"
)

var (
	_ functionClass = &sleepFunctionClass{}
	_ functionClass = &lockFunctionClass{}
	_ functionClass = &releaseLockFunctionClass{}
	_ functionClass = &anyValueFunctionClass{}
	_ functionClass = &defaultFunctionClass{}
	_ functionClass = &inetAtonFunctionClass{}
	_ functionClass = &inetNtoaFunctionClass{}
	_ functionClass = &inet6AtonFunctionClass{}
	_ functionClass = &inet6NtoaFunctionClass{}
	_ functionClass = &isFreeLockFunctionClass{}
	_ functionClass = &isIPv4FunctionClass{}
	_ functionClass = &isIPv4CompatFunctionClass{}
	_ functionClass = &isIPv4MappedFunctionClass{}
	_ functionClass = &isIPv6FunctionClass{}
	_ functionClass = &isUsedLockFunctionClass{}
	_ functionClass = &masterPosWaitFunctionClass{}
	_ functionClass = &nameConstFunctionClass{}
	_ functionClass = &releaseAllLocksFunctionClass{}
	_ functionClass = &uuidFunctionClass{}
	_ functionClass = &uuidShortFunctionClass{}
)

var (
	_ builtinFunc = &builtinSleepSig{}
	_ builtinFunc = &builtinLockSig{}
	_ builtinFunc = &builtinReleaseLockSig{}
	_ builtinFunc = &builtinDecimalAnyValueSig{}
	_ builtinFunc = &builtinDurationAnyValueSig{}
	_ builtinFunc = &builtinIntAnyValueSig{}
	_ builtinFunc = &builtinJSONAnyValueSig{}
	_ builtinFunc = &builtinRealAnyValueSig{}
	_ builtinFunc = &builtinStringAnyValueSig{}
	_ builtinFunc = &builtinTimeAnyValueSig{}
	_ builtinFunc = &builtinInetAtonSig{}
	_ builtinFunc = &builtinInetNtoaSig{}
	_ builtinFunc = &builtinInet6AtonSig{}
	_ builtinFunc = &builtinInet6NtoaSig{}
	_ builtinFunc = &builtinIsIPv4Sig{}
	_ builtinFunc = &builtinIsIPv4CompatSig{}
	_ builtinFunc = &builtinIsIPv4MappedSig{}
	_ builtinFunc = &builtinIsIPv6Sig{}
	_ builtinFunc = &builtinUUIDSig{}
)

type sleepFunctionClass struct {
	baseFunctionClass
}

func (c *sleepFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETInt, types.ETReal)
	bf.tp.Flen = 21
	sig := &builtinSleepSig{bf}
	return sig, nil
}

type builtinSleepSig struct {
	baseBuiltinFunc
}

func (b *builtinSleepSig) Clone() builtinFunc {
	newSig := &builtinSleepSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalInt evals a builtinSleepSig.
// See https://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_sleep
func (b *builtinSleepSig) evalInt(row chunk.Row) (int64, bool, error) {
	val, isNull, err := b.args[0].EvalReal(b.ctx, row)
	if err != nil {
		return 0, isNull, errors.Trace(err)
	}
	sessVars := b.ctx.GetSessionVars()
	if isNull {
		if sessVars.StrictSQLMode {
			return 0, true, errIncorrectArgs.GenWithStackByArgs("sleep")
		}
		return 0, true, nil
	}
	// processing argument is negative
	if val < 0 {
		if sessVars.StrictSQLMode {
			return 0, false, errIncorrectArgs.GenWithStackByArgs("sleep")
		}
		return 0, false, nil
	}

	if val > math.MaxFloat64/float64(time.Second.Nanoseconds()) {
		return 0, false, errIncorrectArgs.GenWithStackByArgs("sleep")
	}
	dur := time.Duration(val * float64(time.Second.Nanoseconds()))
	select {
	case <-time.After(dur):
		// TODO: Handle Ctrl-C is pressed in `mysql` client.
		// return 1 when SLEEP() is KILLed
	}
	return 0, false, nil
}

type lockFunctionClass struct {
	baseFunctionClass
}

func (c *lockFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETInt, types.ETString, types.ETInt)
	sig := &builtinLockSig{bf}
	bf.tp.Flen = 1
	return sig, nil
}

type builtinLockSig struct {
	baseBuiltinFunc
}

func (b *builtinLockSig) Clone() builtinFunc {
	newSig := &builtinLockSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalInt evals a builtinLockSig.
// See https://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_get-lock
// The lock function will do nothing.
// Warning: get_lock() function is parsed but ignored.
func (b *builtinLockSig) evalInt(_ chunk.Row) (int64, bool, error) {
	return 1, false, nil
}

type releaseLockFunctionClass struct {
	baseFunctionClass
}

func (c *releaseLockFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETInt, types.ETString)
	sig := &builtinReleaseLockSig{bf}
	bf.tp.Flen = 1
	return sig, nil
}

type builtinReleaseLockSig struct {
	baseBuiltinFunc
}

func (b *builtinReleaseLockSig) Clone() builtinFunc {
	newSig := &builtinReleaseLockSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalInt evals a builtinReleaseLockSig.
// See https://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_release-lock
// The release lock function will do nothing.
// Warning: release_lock() function is parsed but ignored.
func (b *builtinReleaseLockSig) evalInt(_ chunk.Row) (int64, bool, error) {
	return 1, false, nil
}

type anyValueFunctionClass struct {
	baseFunctionClass
}

func (c *anyValueFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	argTp := args[0].GetType().EvalType()
	bf := newBaseBuiltinFuncWithTp(ctx, args, argTp, argTp)
	args[0].GetType().Flag |= bf.tp.Flag
	*bf.tp = *args[0].GetType()
	var sig builtinFunc
	switch argTp {
	case types.ETDecimal:
		sig = &builtinDecimalAnyValueSig{bf}
	case types.ETDuration:
		sig = &builtinDurationAnyValueSig{bf}
	case types.ETInt:
		bf.tp.Decimal = 0
		sig = &builtinIntAnyValueSig{bf}
	case types.ETJson:
		sig = &builtinJSONAnyValueSig{bf}
	case types.ETReal:
		sig = &builtinRealAnyValueSig{bf}
	case types.ETString:
		bf.tp.Decimal = types.UnspecifiedLength
		sig = &builtinStringAnyValueSig{bf}
	case types.ETDatetime, types.ETTimestamp:
		bf.tp.Charset, bf.tp.Collate, bf.tp.Flag = mysql.DefaultCharset, mysql.DefaultCollationName, 0
		sig = &builtinTimeAnyValueSig{bf}
	default:
		panic("unexpected types.EvalType of builtin function ANY_VALUE")
	}
	return sig, nil
}

type builtinDecimalAnyValueSig struct {
	baseBuiltinFunc
}

func (b *builtinDecimalAnyValueSig) Clone() builtinFunc {
	newSig := &builtinDecimalAnyValueSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalDecimal evals a builtinDecimalAnyValueSig.
// See https://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_any-value
func (b *builtinDecimalAnyValueSig) evalDecimal(row chunk.Row) (*types.MyDecimal, bool, error) {
	return b.args[0].EvalDecimal(b.ctx, row)
}

type builtinDurationAnyValueSig struct {
	baseBuiltinFunc
}

func (b *builtinDurationAnyValueSig) Clone() builtinFunc {
	newSig := &builtinDurationAnyValueSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalDuration evals a builtinDurationAnyValueSig.
// See https://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_any-value
func (b *builtinDurationAnyValueSig) evalDuration(row chunk.Row) (types.Duration, bool, error) {
	return b.args[0].EvalDuration(b.ctx, row)
}

type builtinIntAnyValueSig struct {
	baseBuiltinFunc
}

func (b *builtinIntAnyValueSig) Clone() builtinFunc {
	newSig := &builtinIntAnyValueSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalInt evals a builtinIntAnyValueSig.
// See https://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_any-value
func (b *builtinIntAnyValueSig) evalInt(row chunk.Row) (int64, bool, error) {
	return b.args[0].EvalInt(b.ctx, row)
}

type builtinJSONAnyValueSig struct {
	baseBuiltinFunc
}

func (b *builtinJSONAnyValueSig) Clone() builtinFunc {
	newSig := &builtinJSONAnyValueSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalJSON evals a builtinJSONAnyValueSig.
// See https://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_any-value
func (b *builtinJSONAnyValueSig) evalJSON(row chunk.Row) (json.BinaryJSON, bool, error) {
	return b.args[0].EvalJSON(b.ctx, row)
}

type builtinRealAnyValueSig struct {
	baseBuiltinFunc
}

func (b *builtinRealAnyValueSig) Clone() builtinFunc {
	newSig := &builtinRealAnyValueSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalReal evals a builtinRealAnyValueSig.
// See https://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_any-value
func (b *builtinRealAnyValueSig) evalReal(row chunk.Row) (float64, bool, error) {
	return b.args[0].EvalReal(b.ctx, row)
}

type builtinStringAnyValueSig struct {
	baseBuiltinFunc
}

func (b *builtinStringAnyValueSig) Clone() builtinFunc {
	newSig := &builtinStringAnyValueSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString evals a builtinStringAnyValueSig.
// See https://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_any-value
func (b *builtinStringAnyValueSig) evalString(row chunk.Row) (string, bool, error) {
	return b.args[0].EvalString(b.ctx, row)
}

type builtinTimeAnyValueSig struct {
	baseBuiltinFunc
}

func (b *builtinTimeAnyValueSig) Clone() builtinFunc {
	newSig := &builtinTimeAnyValueSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalTime evals a builtinTimeAnyValueSig.
// See https://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_any-value
func (b *builtinTimeAnyValueSig) evalTime(row chunk.Row) (types.Time, bool, error) {
	return b.args[0].EvalTime(b.ctx, row)
}

type defaultFunctionClass struct {
	baseFunctionClass
}

func (c *defaultFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	return nil, errFunctionNotExists.GenWithStackByArgs("FUNCTION", "DEFAULT")
}

type inetAtonFunctionClass struct {
	baseFunctionClass
}

func (c *inetAtonFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETInt, types.ETString)
	bf.tp.Flen = 21
	bf.tp.Flag |= mysql.UnsignedFlag
	sig := &builtinInetAtonSig{bf}
	return sig, nil
}

type builtinInetAtonSig struct {
	baseBuiltinFunc
}

func (b *builtinInetAtonSig) Clone() builtinFunc {
	newSig := &builtinInetAtonSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalInt evals a builtinInetAtonSig.
// See https://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_inet-aton
func (b *builtinInetAtonSig) evalInt(row chunk.Row) (int64, bool, error) {
	val, isNull, err := b.args[0].EvalString(b.ctx, row)
	if err != nil || isNull {
		return 0, true, errors.Trace(err)
	}
	// ip address should not end with '.'.
	if len(val) == 0 || val[len(val)-1] == '.' {
		return 0, true, nil
	}

	var (
		byteResult, result uint64
		dotCount           int
	)
	for _, c := range val {
		if c >= '0' && c <= '9' {
			digit := uint64(c - '0')
			byteResult = byteResult*10 + digit
			if byteResult > 255 {
				return 0, true, nil
			}
		} else if c == '.' {
			dotCount++
			if dotCount > 3 {
				return 0, true, nil
			}
			result = (result << 8) + byteResult
			byteResult = 0
		} else {
			return 0, true, nil
		}
	}
	// 127 		-> 0.0.0.127
	// 127.255 	-> 127.0.0.255
	// 127.256	-> NULL
	// 127.2.1	-> 127.2.0.1
	switch dotCount {
	case 1:
		result <<= 8
		fallthrough
	case 2:
		result <<= 8
	}
	return int64((result << 8) + byteResult), false, nil
}

type inetNtoaFunctionClass struct {
	baseFunctionClass
}

func (c *inetNtoaFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETString, types.ETInt)
	bf.tp.Flen = 93
	bf.tp.Decimal = 0
	sig := &builtinInetNtoaSig{bf}
	return sig, nil
}

type builtinInetNtoaSig struct {
	baseBuiltinFunc
}

func (b *builtinInetNtoaSig) Clone() builtinFunc {
	newSig := &builtinInetNtoaSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString evals a builtinInetNtoaSig.
// See https://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_inet-ntoa
func (b *builtinInetNtoaSig) evalString(row chunk.Row) (string, bool, error) {
	val, isNull, err := b.args[0].EvalInt(b.ctx, row)
	if err != nil || isNull {
		return "", true, errors.Trace(err)
	}

	if val < 0 || uint64(val) > math.MaxUint32 {
		//not an IPv4 address.
		return "", true, nil
	}
	ip := make(net.IP, net.IPv4len)
	binary.BigEndian.PutUint32(ip, uint32(val))
	ipv4 := ip.To4()
	if ipv4 == nil {
		//Not a vaild ipv4 address.
		return "", true, nil
	}

	return ipv4.String(), false, nil
}

type inet6AtonFunctionClass struct {
	baseFunctionClass
}

func (c *inet6AtonFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETString, types.ETString)
	bf.tp.Flen = 16
	types.SetBinChsClnFlag(bf.tp)
	bf.tp.Decimal = 0
	sig := &builtinInet6AtonSig{bf}
	return sig, nil
}

type builtinInet6AtonSig struct {
	baseBuiltinFunc
}

func (b *builtinInet6AtonSig) Clone() builtinFunc {
	newSig := &builtinInet6AtonSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString evals a builtinInet6AtonSig.
// See https://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_inet6-aton
func (b *builtinInet6AtonSig) evalString(row chunk.Row) (string, bool, error) {
	val, isNull, err := b.args[0].EvalString(b.ctx, row)
	if err != nil || isNull {
		return "", true, errors.Trace(err)
	}

	if len(val) == 0 {
		return "", true, nil
	}

	ip := net.ParseIP(val)
	if ip == nil {
		return "", true, nil
	}

	var isMappedIpv6 bool
	if ip.To4() != nil && strings.Contains(val, ":") {
		//mapped ipv6 address.
		isMappedIpv6 = true
	}

	var result []byte
	if isMappedIpv6 || ip.To4() == nil {
		result = make([]byte, net.IPv6len)
	} else {
		result = make([]byte, net.IPv4len)
	}

	if isMappedIpv6 {
		copy(result[12:], ip.To4())
		result[11] = 0xff
		result[10] = 0xff
	} else if ip.To4() == nil {
		copy(result, ip.To16())
	} else {
		copy(result, ip.To4())
	}

	return string(result[:]), false, nil
}

type inet6NtoaFunctionClass struct {
	baseFunctionClass
}

func (c *inet6NtoaFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETString, types.ETString)
	bf.tp.Flen = 117
	bf.tp.Decimal = 0
	sig := &builtinInet6NtoaSig{bf}
	return sig, nil
}

type builtinInet6NtoaSig struct {
	baseBuiltinFunc
}

func (b *builtinInet6NtoaSig) Clone() builtinFunc {
	newSig := &builtinInet6NtoaSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString evals a builtinInet6NtoaSig.
// See https://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_inet6-ntoa
func (b *builtinInet6NtoaSig) evalString(row chunk.Row) (string, bool, error) {
	val, isNull, err := b.args[0].EvalString(b.ctx, row)
	if err != nil || isNull {
		return "", true, errors.Trace(err)
	}
	ip := net.IP([]byte(val)).String()
	if len(val) == net.IPv6len && !strings.Contains(ip, ":") {
		ip = fmt.Sprintf("::ffff:%s", ip)
	}

	if net.ParseIP(ip) == nil {
		return "", true, nil
	}

	return ip, false, nil
}

type isFreeLockFunctionClass struct {
	baseFunctionClass
}

func (c *isFreeLockFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	return nil, errFunctionNotExists.GenWithStackByArgs("FUNCTION", "IS_FREE_LOCK")
}

type isIPv4FunctionClass struct {
	baseFunctionClass
}

func (c *isIPv4FunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETInt, types.ETString)
	bf.tp.Flen = 1
	sig := &builtinIsIPv4Sig{bf}
	return sig, nil
}

type builtinIsIPv4Sig struct {
	baseBuiltinFunc
}

func (b *builtinIsIPv4Sig) Clone() builtinFunc {
	newSig := &builtinIsIPv4Sig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalInt evals a builtinIsIPv4Sig.
// See https://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_is-ipv4
func (b *builtinIsIPv4Sig) evalInt(row chunk.Row) (int64, bool, error) {
	val, isNull, err := b.args[0].EvalString(b.ctx, row)
	if err != nil || isNull {
		return 0, err != nil, errors.Trace(err)
	}
	if isIPv4(val) {
		return 1, false, nil
	}
	return 0, false, nil
}

// isIPv4 checks IPv4 address which satisfying the format A.B.C.D(0<=A/B/C/D<=255).
// Mapped IPv6 address like '::ffff:1.2.3.4' would return false.
func isIPv4(ip string) bool {
	// acc: keep the decimal value of each segment under check, which should between 0 and 255 for valid IPv4 address.
	// pd: sentinel for '.'
	dots, acc, pd := 0, 0, true
	for _, c := range ip {
		switch {
		case '0' <= c && c <= '9':
			acc = acc*10 + int(c-'0')
			pd = false
		case c == '.':
			dots++
			if dots > 3 || acc > 255 || pd {
				return false
			}
			acc, pd = 0, true
		default:
			return false
		}
	}
	if dots != 3 || acc > 255 || pd {
		return false
	}
	return true
}

type isIPv4CompatFunctionClass struct {
	baseFunctionClass
}

func (c *isIPv4CompatFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETInt, types.ETString)
	bf.tp.Flen = 1
	sig := &builtinIsIPv4CompatSig{bf}
	return sig, nil
}

type builtinIsIPv4CompatSig struct {
	baseBuiltinFunc
}

func (b *builtinIsIPv4CompatSig) Clone() builtinFunc {
	newSig := &builtinIsIPv4CompatSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalInt evals Is_IPv4_Compat
// See https://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_is-ipv4-compat
func (b *builtinIsIPv4CompatSig) evalInt(row chunk.Row) (int64, bool, error) {
	val, isNull, err := b.args[0].EvalString(b.ctx, row)
	if err != nil || isNull {
		return 0, err != nil, errors.Trace(err)
	}

	ipAddress := []byte(val)
	if len(ipAddress) != net.IPv6len {
		//Not an IPv6 address, return false
		return 0, false, nil
	}

	prefixCompat := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	if !bytes.HasPrefix(ipAddress, prefixCompat) {
		return 0, false, nil
	}
	return 1, false, nil
}

type isIPv4MappedFunctionClass struct {
	baseFunctionClass
}

func (c *isIPv4MappedFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETInt, types.ETString)
	bf.tp.Flen = 1
	sig := &builtinIsIPv4MappedSig{bf}
	return sig, nil
}

type builtinIsIPv4MappedSig struct {
	baseBuiltinFunc
}

func (b *builtinIsIPv4MappedSig) Clone() builtinFunc {
	newSig := &builtinIsIPv4MappedSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalInt evals Is_IPv4_Mapped
// See https://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_is-ipv4-mapped
func (b *builtinIsIPv4MappedSig) evalInt(row chunk.Row) (int64, bool, error) {
	val, isNull, err := b.args[0].EvalString(b.ctx, row)
	if err != nil || isNull {
		return 0, err != nil, errors.Trace(err)
	}

	ipAddress := []byte(val)
	if len(ipAddress) != net.IPv6len {
		//Not an IPv6 address, return false
		return 0, false, nil
	}

	prefixMapped := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff}
	if !bytes.HasPrefix(ipAddress, prefixMapped) {
		return 0, false, nil
	}
	return 1, false, nil
}

type isIPv6FunctionClass struct {
	baseFunctionClass
}

func (c *isIPv6FunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETInt, types.ETString)
	bf.tp.Flen = 1
	sig := &builtinIsIPv6Sig{bf}
	return sig, nil
}

type builtinIsIPv6Sig struct {
	baseBuiltinFunc
}

func (b *builtinIsIPv6Sig) Clone() builtinFunc {
	newSig := &builtinIsIPv6Sig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalInt evals a builtinIsIPv6Sig.
// See https://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_is-ipv6
func (b *builtinIsIPv6Sig) evalInt(row chunk.Row) (int64, bool, error) {
	val, isNull, err := b.args[0].EvalString(b.ctx, row)
	if err != nil || isNull {
		return 0, err != nil, errors.Trace(err)
	}
	ip := net.ParseIP(val)
	if ip != nil && !isIPv4(val) {
		return 1, false, nil
	}
	return 0, false, nil
}

type isUsedLockFunctionClass struct {
	baseFunctionClass
}

func (c *isUsedLockFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	return nil, errFunctionNotExists.GenWithStackByArgs("FUNCTION", "IS_USED_LOCK")
}

type masterPosWaitFunctionClass struct {
	baseFunctionClass
}

func (c *masterPosWaitFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	return nil, errFunctionNotExists.GenWithStackByArgs("FUNCTION", "MASTER_POS_WAIT")
}

type nameConstFunctionClass struct {
	baseFunctionClass
}

func (c *nameConstFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	return nil, errFunctionNotExists.GenWithStackByArgs("FUNCTION", "NAME_CONST")
}

type releaseAllLocksFunctionClass struct {
	baseFunctionClass
}

func (c *releaseAllLocksFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	return nil, errFunctionNotExists.GenWithStackByArgs("FUNCTION", "RELEASE_ALL_LOCKS")
}

type uuidFunctionClass struct {
	baseFunctionClass
}

func (c *uuidFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, errors.Trace(err)
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETString)
	bf.tp.Flen = 36
	sig := &builtinUUIDSig{bf}
	return sig, nil
}

type builtinUUIDSig struct {
	baseBuiltinFunc
}

func (b *builtinUUIDSig) Clone() builtinFunc {
	newSig := &builtinUUIDSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString evals a builtinUUIDSig.
// See https://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_uuid
func (b *builtinUUIDSig) evalString(_ chunk.Row) (d string, isNull bool, err error) {
	return uuid.NewV1().String(), false, nil
}

type uuidShortFunctionClass struct {
	baseFunctionClass
}

func (c *uuidShortFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error) {
	return nil, errFunctionNotExists.GenWithStackByArgs("FUNCTION", "UUID_SHORT")
}
