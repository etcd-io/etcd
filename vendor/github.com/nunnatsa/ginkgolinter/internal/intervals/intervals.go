package intervals

import (
	"go/ast"
	"go/constant"
	"go/token"
	gotypes "go/types"
	"strconv"
	"time"

	"golang.org/x/tools/go/analysis"
)

func GetDuration(pass *analysis.Pass, argOffset int, origInterval, intervalClone ast.Expr, timePkg string) DurationValue {
	tv := pass.TypesInfo.Types[origInterval]
	argType := tv.Type
	if durType, ok := argType.(*gotypes.Named); ok {
		if durType.String() == "time.Duration" {
			if tv.Value != nil {
				if val, ok := constant.Int64Val(tv.Value); ok {
					return &RealDurationValue{
						dur:  time.Duration(val),
						expr: intervalClone,
					}
				}
			}
			return &UnknownDurationTypeValue{
				expr: intervalClone,
			}
		}
	}

	if basic, ok := argType.(*gotypes.Basic); ok && tv.Value != nil {
		if basic.Info()&gotypes.IsInteger != 0 {
			if num, ok := constant.Int64Val(tv.Value); ok {
				return &NumericDurationValue{
					timePkg:     timePkg,
					numSeconds:  num,
					offset:      argOffset,
					dur:         time.Duration(num) * time.Second,
					expr:        intervalClone,
					useOrigExpr: tv.Type.String() == "int",
				}
			}
		}

		if basic.Info()&gotypes.IsFloat != 0 {
			if num, ok := constant.Float64Val(tv.Value); ok {
				return &NumericDurationValue{
					timePkg:     timePkg,
					numSeconds:  int64(num),
					offset:      argOffset,
					dur:         time.Duration(num) * time.Second,
					expr:        intervalClone,
					useOrigExpr: false,
				}
			}
		}

		if basic.Info()&gotypes.IsString != 0 {
			val, err := strconv.Unquote(tv.Value.ExactString())
			if err != nil {
				val = tv.Value.String()
			}
			duration, err := time.ParseDuration(val)
			if err != nil {
				return &UnknownDurationValue{expr: intervalClone}
			}

			return &StringDurationValue{
				timePkg: timePkg,
				dur:     duration,
				expr:    intervalClone,
				offset:  argOffset,
			}
		}
	}

	return &UnknownDurationValue{expr: intervalClone}
}

func GetDurationFromValue(pass *analysis.Pass, orig, clone ast.Expr) DurationValue {
	tv := pass.TypesInfo.Types[orig]
	interval := tv.Value
	if interval != nil {
		if val, ok := constant.Int64Val(interval); ok {
			return RealDurationValue{
				dur:  time.Duration(val),
				expr: orig,
			}
		}
	}
	return UnknownDurationTypeValue{expr: clone}
}

type DurationValue interface {
	Duration() time.Duration
}

type NumericValue interface {
	GetOffset() int
	GetDurationExpr() ast.Expr
}
type RealDurationValue struct {
	dur  time.Duration
	expr ast.Expr
}

func (r RealDurationValue) Duration() time.Duration {
	return r.dur
}

type NumericDurationValue struct {
	timePkg     string
	numSeconds  int64
	offset      int
	dur         time.Duration
	expr        ast.Expr
	useOrigExpr bool
}

func (r *NumericDurationValue) Duration() time.Duration {
	return r.dur
}

func (r *NumericDurationValue) GetOffset() int {
	return r.offset
}

func (r *NumericDurationValue) GetDurationExpr() ast.Expr {
	var newArg ast.Expr
	second := getUnit(r.timePkg, "Second")

	var y ast.Expr
	if r.useOrigExpr {
		y = r.expr
	} else {
		y = &ast.BasicLit{Value: strconv.Itoa(int(r.numSeconds)), Kind: token.INT}
	}

	if r.numSeconds == 1 {
		newArg = second
	} else {
		newArg = &ast.BinaryExpr{
			X:  second,
			Op: token.MUL,
			Y:  y,
		}
	}

	return newArg
}

type UnknownDurationValue struct {
	expr ast.Expr
}

func (r UnknownDurationValue) Duration() time.Duration {
	return 0
}

type UnknownNumericValue struct {
	expr    ast.Expr
	offset  int
	timePkg string
}

func (r UnknownNumericValue) Duration() time.Duration {
	return 0
}

func (r UnknownNumericValue) GetDurationExpr() ast.Expr {
	return &ast.BinaryExpr{
		X:  getUnit(r.timePkg, "Second"),
		Op: token.MUL,
		Y:  r.expr,
	}
}

func (r UnknownNumericValue) GetOffset() int {
	return r.offset
}

type UnknownDurationTypeValue struct {
	expr ast.Expr
}

func (r UnknownDurationTypeValue) Duration() time.Duration {
	return 0
}

type StringDurationValue struct {
	timePkg string
	dur     time.Duration
	expr    ast.Expr
	offset  int
}

func (r StringDurationValue) Duration() time.Duration {
	return r.dur
}

func (r StringDurationValue) GetOffset() int {
	return r.offset
}

func (r StringDurationValue) GetDurationExpr() ast.Expr {
	return durationToExpr(r.dur, r.timePkg)
}

func durationToExpr(duration time.Duration, timePkg string) ast.Expr {
	var durationExpr ast.Expr

	if duration >= time.Hour {
		hours := duration / time.Hour
		duration -= hours * time.Hour
		durationExpr = getDurationExpression("Hour", timePkg, hours)
	}

	if duration >= time.Minute {
		minutes := duration / time.Minute
		duration -= minutes * time.Minute
		minExp := getDurationExpression("Minute", timePkg, minutes)

		if durationExpr == nil {
			durationExpr = minExp
		} else {
			durationExpr = &ast.BinaryExpr{
				X:  durationExpr,
				Op: token.ADD,
				Y:  minExp,
			}
		}
	}

	if duration >= time.Second {
		seconds := duration / time.Second
		duration -= seconds * time.Second
		secExpr := getDurationExpression("Second", timePkg, seconds)

		if durationExpr == nil {
			durationExpr = secExpr
		} else {
			durationExpr = &ast.BinaryExpr{
				X:  durationExpr,
				Op: token.ADD,
				Y:  secExpr,
			}
		}
	}

	if duration >= time.Millisecond {
		milliseconds := duration / time.Millisecond
		duration -= milliseconds * time.Millisecond
		millisecondsExpr := getDurationExpression("Millisecond", timePkg, milliseconds)

		if durationExpr == nil {
			durationExpr = millisecondsExpr
		} else {
			durationExpr = &ast.BinaryExpr{
				X:  durationExpr,
				Op: token.ADD,
				Y:  millisecondsExpr,
			}
		}
	}

	if duration >= time.Microsecond {
		microseconds := duration / time.Microsecond
		duration -= microseconds * time.Microsecond
		microsecondsExpr := getDurationExpression("Microsecond", timePkg, microseconds)

		if durationExpr == nil {
			durationExpr = microsecondsExpr
		} else {
			durationExpr = &ast.BinaryExpr{
				X:  durationExpr,
				Op: token.ADD,
				Y:  microsecondsExpr,
			}
		}
	}

	if duration > 0 {
		nanosecondsExpr := getDurationExpression("Nanosecond", timePkg, duration)

		if durationExpr == nil {
			durationExpr = nanosecondsExpr
		} else {
			durationExpr = &ast.BinaryExpr{
				X:  durationExpr,
				Op: token.ADD,
				Y:  nanosecondsExpr,
			}
		}
	}

	return durationExpr
}

func getDurationExpression(unitName, timePkg string, amount time.Duration) ast.Expr {
	unit := getUnit(timePkg, unitName)

	if amount == 1 {
		return unit
	}

	return &ast.BinaryExpr{
		X:  unit,
		Op: token.MUL,
		Y:  &ast.BasicLit{Value: strconv.FormatInt(int64(amount), 10), Kind: token.INT},
	}
}

func getUnit(timePkg, unitName string) ast.Expr {
	return &ast.SelectorExpr{
		X:   ast.NewIdent(timePkg),
		Sel: ast.NewIdent(unitName),
	}
}
