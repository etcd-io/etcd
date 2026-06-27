package rule

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"strconv"
	"strings"
	"time"

	"github.com/mgechev/revive/internal/astutils"
	"github.com/mgechev/revive/lint"
	"github.com/mgechev/revive/logging"
)

// TimeDateRule lints the way [time.Date] is used.
type TimeDateRule struct{}

// Apply applies the rule to given file.
func (*TimeDateRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure

	onFailure := func(failure lint.Failure) {
		failures = append(failures, failure)
	}

	w := &lintTimeDate{file, onFailure}

	ast.Walk(w, file.AST)
	return failures
}

// Name returns the rule name.
func (*TimeDateRule) Name() string {
	return "time-date"
}

type lintTimeDate struct {
	file      *lint.File
	onFailure func(lint.Failure)
}

// timeDateArgument is a type for the arguments of [time.Date] function.
type timeDateArgument string

const (
	timeDateArgYear       timeDateArgument = "year"
	timeDateArgMonth      timeDateArgument = "month"
	timeDateArgDay        timeDateArgument = "day"
	timeDateArgHour       timeDateArgument = "hour"
	timeDateArgMinute     timeDateArgument = "minute"
	timeDateArgSecond     timeDateArgument = "second"
	timeDateArgNanosecond timeDateArgument = "nanosecond"
	timeDateArgTimezone   timeDateArgument = "timezone"
)

var (
	// timeDateArgumentNames are the names of the arguments of [time.Date].
	timeDateArgumentNames = []timeDateArgument{
		timeDateArgYear,
		timeDateArgMonth,
		timeDateArgDay,
		timeDateArgHour,
		timeDateArgMinute,
		timeDateArgSecond,
		timeDateArgNanosecond,
		timeDateArgTimezone,
	}

	// timeDateArity is the number of arguments of [time.Date].
	timeDateArity = len(timeDateArgumentNames)
)

var timeDateArgumentBoundaries = map[timeDateArgument][2]int64{
	// year is not validated
	timeDateArgMonth:      {1, 12},
	timeDateArgDay:        {1, 31}, // there is a special check for this field, this is just a fallback
	timeDateArgHour:       {0, 23},
	timeDateArgMinute:     {0, 59},
	timeDateArgSecond:     {0, 60},      // 60 is for leap second
	timeDateArgNanosecond: {0, 1e9 - 1}, // 1e9 is not allowed, as it means 1 second
}

type timeDateMonthYear struct {
	year, month int64
}

func (w *lintTimeDate) Visit(n ast.Node) ast.Visitor {
	ce, ok := n.(*ast.CallExpr)
	if !ok || len(ce.Args) != timeDateArity {
		return w
	}
	if !astutils.IsPkgDotName(ce.Fun, "time", "Date") {
		return w
	}

	// The last argument is a timezone, check it
	tzArg := ce.Args[timeDateArity-1]
	if astutils.IsIdent(tzArg, "nil") {
		w.onFailure(lint.Failure{
			Category:   "time",
			Node:       tzArg,
			Confidence: 1,
			Failure:    "time.Date timezone argument cannot be nil, it would panic on runtime",
		})
	}

	var parsedDate timeDateMonthYear
	// All the other arguments should be decimal integers.
	for pos, arg := range ce.Args[:timeDateArity-1] {
		fieldName := timeDateArgumentNames[pos]

		bl, ok := w.checkArgSign(arg, fieldName)
		if !ok {
			// either it is not a basic literal
			// or it is a unary expression with a sign that was reported as a failure
			continue
		}

		parsedValue, err := parseDecimalInteger(bl)
		if err == nil {
			if fieldName == timeDateArgYear {
				// store the year value for further checks with day
				parsedDate.year = parsedValue

				// no checks for year, as it can be any value
				// a year can be negative, zero, or positive
				continue
			}

			boundaries, ok := timeDateArgumentBoundaries[fieldName]
			if !ok {
				// no boundaries for this field, skip it
				continue
			}
			minValue, maxValue := boundaries[0], boundaries[1]

			switch fieldName {
			case timeDateArgMonth:
				parsedDate.month = parsedValue

				if parsedValue == 0 {
					// Special case: month is 0.
					// Go treats it as January, but we still report it as a failure.
					w.onFailure(lint.Failure{
						Category:   "time",
						Node:       arg,
						Confidence: 1,
						Failure:    "time.Date month argument should not be zero",
					})
					continue
				}

			case timeDateArgDay:

				switch {
				case parsedValue == 0:
					// Special case: day is 0.
					// Go treats it as the first day of the month, but we still report it as a failure.
					w.onFailure(lint.Failure{
						Category:   "time",
						Node:       arg,
						Confidence: 1,
						Failure:    "time.Date day argument should not be zero",
					})
					continue

				// the month is valid, check the day
				case parsedDate.month >= 1 && parsedDate.month <= 12:
					month := time.Month(parsedDate.month)

					maxValue = w.daysInMonth(parsedDate.year, month)

					monthName := month.String()
					if month == time.February {
						// because of leap years, we need to provide the year in the error message
						monthName += " " + strconv.FormatInt(parsedDate.year, 10)
					}

					if parsedValue > maxValue {
						// we can provide a more detailed error message
						w.onFailure(lint.Failure{
							Category:   "time",
							Node:       arg,
							Confidence: 0.8,
							Failure: fmt.Sprintf(
								"time.Date day argument is %d, but %s has only %d days",
								parsedValue, monthName, maxValue,
							),
						})
						continue
					}

				// We know, the month is >12, let's try to detect possible day and month swap in arguments.
				// for example: time.Date(2023, 31, 6, 0, 0, 0, 0, time.UTC)
				case parsedDate.month > 12 && parsedDate.month <= 31 && parsedValue <= 12:

					// Invert the month and day values
					realMonth, realDay := parsedValue, parsedDate.month

					// Check if the real month is valid.
					if realDay <= w.daysInMonth(parsedDate.year, time.Month(realMonth)) {
						w.onFailure(lint.Failure{
							Category:   "time",
							Node:       arg,
							Confidence: 0.5,
							Failure: fmt.Sprintf(
								"time.Date month and day arguments appear to be swapped: %d-%02d-%02d vs %d-%02d-%02d",
								parsedDate.year, realMonth, realDay,
								parsedDate.year, parsedDate.month, parsedValue,
							),
						})
					}
				}
			}

			if parsedValue < minValue || parsedValue > maxValue {
				w.onFailure(lint.Failure{
					Category:   "time",
					Node:       arg,
					Confidence: 0.8,
					Failure: fmt.Sprintf(
						"time.Date %s argument should be between %d and %d: %s",
						fieldName, minValue, maxValue, astutils.GoFmt(arg),
					),
				})
			}

			continue
		}

		if errors.Is(err, errParsedInvalid) {
			// This is not supposed to happen, let's be defensive
			// log the error, but continue

			logger, errLogger := logging.GetLogger()
			if errLogger != nil {
				// This is not supposed to happen, discard both errors
				continue
			}
			logger.With(
				"value", bl.Value,
				"kind", bl.Kind,
				"error", err.Error(),
			).Error("failed to parse time.Date argument")

			continue
		}

		confidence := 0.8 // default confidence
		errMessage := err.Error()
		replacedValue := strconv.FormatInt(parsedValue, 10)
		instructions := fmt.Sprintf("use %s instead of %s", replacedValue, astutils.GoFmt(arg))
		switch {
		case errors.Is(err, errParsedOctalWithZero):
			// people can use 00, 01, 02, 03, 04, 05, 06, and 07 if they want.
			confidence = 0.5

		case errors.Is(err, errParsedOctalWithPaddingZeroes):
			// This is a clear mistake.
			// example with 000123456 (octal) is about 123456 or 42798 ?
			confidence = 1

			strippedValue := strings.TrimLeft(bl.Value, "0")
			if strippedValue == "" {
				// avoid issue with 00000000
				strippedValue = "0"
			}

			if strippedValue != replacedValue {
				instructions = fmt.Sprintf(
					"choose between %s and %s (decimal value of %s octal value)",
					strippedValue, replacedValue, strippedValue,
				)
			}
		}

		w.onFailure(lint.Failure{
			Category:   "time",
			Node:       bl,
			Confidence: confidence,
			Failure: fmt.Sprintf(
				"use decimal digits for time.Date %s argument: %s found: %s",
				fieldName, errMessage, instructions),
		})
	}

	return w
}

func (w *lintTimeDate) checkArgSign(arg ast.Node, fieldName timeDateArgument) (*ast.BasicLit, bool) {
	if bl, ok := arg.(*ast.BasicLit); ok {
		// it is an unsigned basic literal
		// we can use it as is
		return bl, true
	}

	// We can have an unary expression like -1, -a, +a, +1...
	node, ok := arg.(*ast.UnaryExpr)
	if !ok {
		// Any other expression is not supported.
		// It could be something like this:
		// time.Date(2023, 2 * a, 3 + b, 4, 5, 6, 7, time.UTC)
		return nil, false
	}

	// But we expect the unary expression to be followed by a basic literal
	bl, ok := node.X.(*ast.BasicLit)
	if !ok {
		// This is not a basic literal, it could be an identifier, a function call, etc.
		// -a
		// ^b
		// -foo()
		//
		// It's out of scope of this rule.
		return nil, false
	}
	// So now, we have an unary expression like -500, +2023, -0x1234 ...

	if fieldName == timeDateArgYear && node.Op == token.SUB {
		// The year can be negative, like referring to BC years.
		// We can return it as is, without reporting a failure
		return bl, true
	}

	switch node.Op {
	case token.SUB:
		// This is a negative number, it is supported, but it's uncommon.
		w.onFailure(lint.Failure{
			Category:   "time",
			Node:       arg,
			Confidence: 0.5,
			Failure: fmt.Sprintf(
				"time.Date %s argument is negative: %s",
				fieldName, astutils.GoFmt(arg),
			),
		})
	case token.ADD:
		// There is a positive sign, but it is not necessary to have a positive sign
		w.onFailure(lint.Failure{
			Category:   "time",
			Node:       arg,
			Confidence: 0.8,
			Failure: fmt.Sprintf(
				"time.Date %s argument contains a useless plus sign: %s",
				fieldName, astutils.GoFmt(arg),
			),
		})
	default:
		// Other unary expressions are not supported.
		//
		// It could be something like this:
		// ^1, ^0x1234
		// but these are unlikely to be used with time.Date
		// We ignore them, to avoid false positives.
	}

	return nil, false
}

// isLeapYear checks if the year is a leap year.
// This is used to check if the date is valid according to Go implementation.
func (*lintTimeDate) isLeapYear(year int64) bool {
	// We cannot use the classic formula of
	// year%4 == 0 && (year%100 != 0 || year%400 == 0)
	// because we want to ensure what time.Date will compute

	return time.Date(int(year), 2, 29, 0, 0, 0, 0, time.UTC).Format("01-02") == "02-29"
}

func (w *lintTimeDate) daysInMonth(year int64, month time.Month) int64 {
	switch month {
	case time.April, time.June, time.September, time.November:
		return 30
	case time.February:
		if w.isLeapYear(year) {
			return 29
		}
		return 28
	}

	return 31
}

var (
	errParsedOctal                  = errors.New("octal notation")
	errParsedOctalWithZero          = errors.New("octal notation with leading zero")
	errParsedOctalWithPaddingZeroes = errors.New("octal notation with padding zeroes")
	errParsedHexadecimal            = errors.New("hexadecimal notation")
	errParseBinary                  = errors.New("binary notation")
	errParsedFloat                  = errors.New("float literal")
	errParsedExponential            = errors.New("exponential notation")
	errParsedAlternative            = errors.New("alternative notation")
	errParsedInvalid                = errors.New("invalid notation")
)

func parseDecimalInteger(bl *ast.BasicLit) (int64, error) {
	currentValue := strings.ToLower(bl.Value)

	if currentValue == "0" {
		// skip 0 as it is a valid value for all the arguments
		return 0, nil
	}

	switch bl.Kind {
	case token.FLOAT:
		// someone used a float literal, while they should have used an integer literal.
		parsedValue, err := strconv.ParseFloat(currentValue, 64)
		if err != nil {
			// This is not supposed to happen
			return 0, fmt.Errorf(
				"%w: %s: %w",
				errParsedInvalid,
				"failed to parse number as float",
				err,
			)
		}

		// this will convert back the number to a string
		if strings.Contains(currentValue, "e") {
			return int64(parsedValue), errParsedExponential
		}

		return int64(parsedValue), errParsedFloat

	case token.INT:
		// we expect this format

	default:
		// This is not supposed to happen
		return 0, fmt.Errorf(
			"%w: %s",
			errParsedInvalid,
			"unexpected kind of literal",
		)
	}

	// Parse the number with base=0 that allows to accept all number formats and base
	parsedValue, err := strconv.ParseInt(currentValue, 0, 64)
	if err != nil {
		// This is not supposed to happen
		return 0, fmt.Errorf(
			"%w: %s: %w",
			errParsedInvalid,
			"failed to parse number as integer",
			err,
		)
	}

	// Let's figure out the notation to return an error
	switch {
	case strings.HasPrefix(currentValue, "0b"):
		return parsedValue, errParseBinary
	case strings.HasPrefix(currentValue, "0x"):
		return parsedValue, errParsedHexadecimal
	case strings.HasPrefix(currentValue, "0"):
		// this matches both "0" and "0o" octal notation.

		switch currentValue {
		// people can use 00, 01, 02, 03, 04, 05, 06, 07, if they want
		case "00", "01", "02", "03", "04", "05", "06", "07":
			return parsedValue, errParsedOctalWithZero
		}

		if strings.HasPrefix(currentValue, "00") {
			// 00123456 (octal) is about 123456 or 42798 ?
			return parsedValue, errParsedOctalWithPaddingZeroes
		}

		return parsedValue, errParsedOctal
	}

	// Convert back the number to a string, and compare it with the original one
	formattedValue := strconv.FormatInt(parsedValue, 10)
	if formattedValue != currentValue {
		// This can catch some edge cases like: 1_0 ...
		return parsedValue, errParsedAlternative
	}

	return parsedValue, nil
}
