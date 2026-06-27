package sloglint

import (
	"errors"
	"flag"
	"fmt"
	"strings"
)

// Func describes a function to analyze, e.g. [slog.Info].
type Func struct {
	// The full name of the function, including the package, e.g. "log/slog.Info".
	// If the function is a method, the receiver type must be wrapped in parentheses, e.g. "(*log/slog.Logger).Info".
	FullName string
	// The position of the "msg string" argument in the function signature, starting from 0.
	// If there is no message in the function, a negative value must be passed.
	MessagePos int
	// The position of the "args ...any" argument in the function signature, starting from 0.
	// If there are no arguments in the function, a negative value must be passed.
	ArgumentsPos int
}

// Options contains options for the sloglint analyzer.
type Options struct {
	// Report the use of global loggers ("all" or "default").
	NoGlobalLogger string
	// Report the use of functions without a [context.Context] ("all" or "scope").
	ContextOnly string

	// Report dynamic log messages, such as those that are built with [fmt.Sprintf].
	StaticMessage bool
	// Report log messages that do not match a particular style ("lowercased" or "capitalized").
	MessageStyle string

	// Report the use of both key-value pairs and attributes within a single function call (default true).
	NoMixedArguments bool
	// Report any use of attributes as function call arguments.
	KeyValuePairsOnly bool
	// Report any use of key-value pairs as function call arguments.
	AttributesOnly bool
	// Report two or more arguments on the same line.
	ArgumentsOnSeparateLines bool

	// Report the use of string literals as log keys.
	ConstantKeys bool
	// Report the use of log keys that are not explicitly allowed.
	AllowedKeys []string
	// Report the use of forbidden log keys.
	ForbiddenKeys []string
	// Report log keys that do not match a particular naming case ("snake", "kebab", "camel", or "pascal").
	KeyNamingCase string

	// Analyze custom functions in addition to the standard [log/slog] functions.
	CustomFuncs []Func
}

// Possible values for [Options.NoGlobalLogger].
const (
	noGlobalLoggerAll     = "all"
	noGlobalLoggerDefault = "default"
)

// Possible values for [Options.ContextOnly].
const (
	contextOnlyAll   = "all"
	contextOnlyScope = "scope"
)

// Possible values for [Options.MessageStyle].
const (
	messageStyleLowercased  = "lowercased"
	messageStyleCapitalized = "capitalized"
)

// Possible values for [Options.KeyNamingCase].
const (
	keyNamingCaseSnake  = "snake"
	keyNamingCaseKebab  = "kebab"
	keyNamingCaseCamel  = "camel"
	keyNamingCasePascal = "pascal"
)

var (
	errIncompatible = errors.New("incompatible")
	errInvalidValue = errors.New("invalid value")
)

func (opts *Options) validate() error {
	switch opts.NoGlobalLogger {
	case "", noGlobalLoggerAll, noGlobalLoggerDefault:
	default:
		return fmt.Errorf("sloglint: Options.NoGlobalLogger has an %w %q", errInvalidValue, opts.NoGlobalLogger)
	}

	switch opts.ContextOnly {
	case "", contextOnlyAll, contextOnlyScope:
	default:
		return fmt.Errorf("sloglint: Options.ContextOnly has an %w %q", errInvalidValue, opts.ContextOnly)
	}

	switch opts.MessageStyle {
	case "", messageStyleLowercased, messageStyleCapitalized:
	default:
		return fmt.Errorf("sloglint: Options.MessageStyle has an %w %q", errInvalidValue, opts.MessageStyle)
	}

	if opts.KeyValuePairsOnly && opts.AttributesOnly {
		return fmt.Errorf("sloglint: Options.KeyValuePairsOnly and Options.AttributesOnly are %w", errIncompatible)
	}

	switch opts.KeyNamingCase {
	case "", keyNamingCaseSnake, keyNamingCaseKebab, keyNamingCaseCamel, keyNamingCasePascal:
	default:
		return fmt.Errorf("sloglint: Options.KeyNamingCase has an %w %q", errInvalidValue, opts.KeyNamingCase)
	}

	return nil
}

func flags(opts *Options) flag.FlagSet {
	fs := flag.NewFlagSet("sloglint", flag.ContinueOnError)

	listVar := func(p *[]string, name, usage string) {
		fs.Func(name, usage+" (comma-separated)", func(s string) error {
			*p = append(*p, strings.Split(s, ",")...)
			return nil
		})
	}

	fs.StringVar(&opts.NoGlobalLogger, "no-global", opts.NoGlobalLogger, `report the use of global loggers ("all" or "default")`)
	fs.StringVar(&opts.ContextOnly, "ctx-only", opts.ContextOnly, `report the use of functions without a context.Context ("all" or "scope")`)
	fs.BoolVar(&opts.StaticMessage, "static-msg", opts.StaticMessage, `report dynamic log messages, such as those that are built with fmt.Sprintf`)
	fs.StringVar(&opts.MessageStyle, "msg-style", opts.MessageStyle, `report log messages that do not match a particular style ("lowercased" or "capitalized")`)
	fs.BoolVar(&opts.NoMixedArguments, "no-mixed-args", opts.NoMixedArguments, `report the use of both key-value pairs and attributes within a single function call (default true)`)
	fs.BoolVar(&opts.KeyValuePairsOnly, "kv-only", opts.KeyValuePairsOnly, `report any use of attributes as function call arguments`)
	fs.BoolVar(&opts.AttributesOnly, "attr-only", opts.AttributesOnly, `report any use of key-value pairs as function call arguments`)
	fs.BoolVar(&opts.ArgumentsOnSeparateLines, "args-on-sep-lines", opts.ArgumentsOnSeparateLines, `report two or more arguments on the same line`)
	fs.BoolVar(&opts.ConstantKeys, "const-keys", opts.ConstantKeys, `report the use of string literal as log keys`)
	listVar(&opts.AllowedKeys, "allowed-keys", `report the use of log keys that are not explicitly allowed`)
	listVar(&opts.ForbiddenKeys, "forbidden-keys", `report the use of forbidden log keys`)
	fs.StringVar(&opts.KeyNamingCase, "key-naming-case", opts.KeyNamingCase, `report log keys that do not match a particular naming case ("snake", "kebab", "camel", or "pascal")`)

	fs.Func("fn", `analyze a custom function (format: "full-name:msg-pos:args-pos")`, func(s string) error {
		name, rest, _ := strings.Cut(s, ":")
		fn := Func{FullName: name}
		_, err := fmt.Sscanf(rest, "%d:%d", &fn.MessagePos, &fn.ArgumentsPos)
		opts.CustomFuncs = append(opts.CustomFuncs, fn)
		return err
	})

	return *fs
}
