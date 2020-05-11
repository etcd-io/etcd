package altsrc

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/urfave/cli"
)

// FlagInputSourceExtension is an extension interface of cli.Flag that
// allows a value to be set on the existing parsed flags.
type FlagInputSourceExtension interface {
	cli.Flag
	ApplyInputSourceValue(context *cli.Context, isc InputSourceContext) error
}

// ApplyInputSourceValues iterates over all provided flags and
// executes ApplyInputSourceValue on flags implementing the
// FlagInputSourceExtension interface to initialize these flags
// to an alternate input source.
func ApplyInputSourceValues(context *cli.Context, inputSourceContext InputSourceContext, flags []cli.Flag) error {
	for _, f := range flags {
		inputSourceExtendedFlag, isType := f.(FlagInputSourceExtension)
		if isType {
			err := inputSourceExtendedFlag.ApplyInputSourceValue(context, inputSourceContext)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// InitInputSource is used to to setup an InputSourceContext on a cli.Command Before method. It will create a new
// input source based on the func provided. If there is no error it will then apply the new input source to any flags
// that are supported by the input source
func InitInputSource(flags []cli.Flag, createInputSource func() (InputSourceContext, error)) cli.BeforeFunc {
	return func(context *cli.Context) error {
		inputSource, err := createInputSource()
		if err != nil {
			return fmt.Errorf("Unable to create input source: inner error: \n'%v'", err.Error())
		}

		return ApplyInputSourceValues(context, inputSource, flags)
	}
}

// InitInputSourceWithContext is used to to setup an InputSourceContext on a cli.Command Before method. It will create a new
// input source based on the func provided with potentially using existing cli.Context values to initialize itself. If there is
// no error it will then apply the new input source to any flags that are supported by the input source
func InitInputSourceWithContext(flags []cli.Flag, createInputSource func(context *cli.Context) (InputSourceContext, error)) cli.BeforeFunc {
	return func(context *cli.Context) error {
		inputSource, err := createInputSource(context)
		if err != nil {
			return fmt.Errorf("Unable to create input source with context: inner error: \n'%v'", err.Error())
		}

		return ApplyInputSourceValues(context, inputSource, flags)
	}
}

// GenericFlag is the flag type that wraps cli.GenericFlag to allow
// for other values to be specified
type GenericFlag struct {
	cli.GenericFlag
	set *flag.FlagSet
}

// NewGenericFlag creates a new GenericFlag
func NewGenericFlag(flag cli.GenericFlag) *GenericFlag {
	return &GenericFlag{GenericFlag: flag, set: nil}
}

// ApplyInputSourceValue applies a generic value to the flagSet if required
func (f *GenericFlag) ApplyInputSourceValue(context *cli.Context, isc InputSourceContext) error {
	if f.set != nil {
		if !context.IsSet(f.Name) && !isEnvVarSet(f.EnvVar) {
			value, err := isc.Generic(f.GenericFlag.Name)
			if err != nil {
				return err
			}
			if value != nil {
				eachName(f.Name, func(name string) {
					f.set.Set(f.Name, value.String())
				})
			}
		}
	}

	return nil
}

// Apply saves the flagSet for later usage then calls
// the wrapped GenericFlag.Apply
func (f *GenericFlag) Apply(set *flag.FlagSet) {
	f.set = set
	f.GenericFlag.Apply(set)
}

// StringSliceFlag is the flag type that wraps cli.StringSliceFlag to allow
// for other values to be specified
type StringSliceFlag struct {
	cli.StringSliceFlag
	set *flag.FlagSet
}

// NewStringSliceFlag creates a new StringSliceFlag
func NewStringSliceFlag(flag cli.StringSliceFlag) *StringSliceFlag {
	return &StringSliceFlag{StringSliceFlag: flag, set: nil}
}

// ApplyInputSourceValue applies a StringSlice value to the flagSet if required
func (f *StringSliceFlag) ApplyInputSourceValue(context *cli.Context, isc InputSourceContext) error {
	if f.set != nil {
		if !context.IsSet(f.Name) && !isEnvVarSet(f.EnvVar) {
			value, err := isc.StringSlice(f.StringSliceFlag.Name)
			if err != nil {
				return err
			}
			if value != nil {
				var sliceValue cli.StringSlice = value
				eachName(f.Name, func(name string) {
					underlyingFlag := f.set.Lookup(f.Name)
					if underlyingFlag != nil {
						underlyingFlag.Value = &sliceValue
					}
				})
			}
		}
	}
	return nil
}

// Apply saves the flagSet for later usage then calls
// the wrapped StringSliceFlag.Apply
func (f *StringSliceFlag) Apply(set *flag.FlagSet) {
	f.set = set
	f.StringSliceFlag.Apply(set)
}

// IntSliceFlag is the flag type that wraps cli.IntSliceFlag to allow
// for other values to be specified
type IntSliceFlag struct {
	cli.IntSliceFlag
	set *flag.FlagSet
}

// NewIntSliceFlag creates a new IntSliceFlag
func NewIntSliceFlag(flag cli.IntSliceFlag) *IntSliceFlag {
	return &IntSliceFlag{IntSliceFlag: flag, set: nil}
}

// ApplyInputSourceValue applies a IntSlice value if required
func (f *IntSliceFlag) ApplyInputSourceValue(context *cli.Context, isc InputSourceContext) error {
	if f.set != nil {
		if !context.IsSet(f.Name) && !isEnvVarSet(f.EnvVar) {
			value, err := isc.IntSlice(f.IntSliceFlag.Name)
			if err != nil {
				return err
			}
			if value != nil {
				var sliceValue cli.IntSlice = value
				eachName(f.Name, func(name string) {
					underlyingFlag := f.set.Lookup(f.Name)
					if underlyingFlag != nil {
						underlyingFlag.Value = &sliceValue
					}
				})
			}
		}
	}
	return nil
}

// Apply saves the flagSet for later usage then calls
// the wrapped IntSliceFlag.Apply
func (f *IntSliceFlag) Apply(set *flag.FlagSet) {
	f.set = set
	f.IntSliceFlag.Apply(set)
}

// BoolFlag is the flag type that wraps cli.BoolFlag to allow
// for other values to be specified
type BoolFlag struct {
	cli.BoolFlag
	set *flag.FlagSet
}

// NewBoolFlag creates a new BoolFlag
func NewBoolFlag(flag cli.BoolFlag) *BoolFlag {
	return &BoolFlag{BoolFlag: flag, set: nil}
}

// ApplyInputSourceValue applies a Bool value to the flagSet if required
func (f *BoolFlag) ApplyInputSourceValue(context *cli.Context, isc InputSourceContext) error {
	if f.set != nil {
		if !context.IsSet(f.Name) && !isEnvVarSet(f.EnvVar) {
			value, err := isc.Bool(f.BoolFlag.Name)
			if err != nil {
				return err
			}
			if value {
				eachName(f.Name, func(name string) {
					f.set.Set(f.Name, strconv.FormatBool(value))
				})
			}
		}
	}
	return nil
}

// Apply saves the flagSet for later usage then calls
// the wrapped BoolFlag.Apply
func (f *BoolFlag) Apply(set *flag.FlagSet) {
	f.set = set
	f.BoolFlag.Apply(set)
}

// BoolTFlag is the flag type that wraps cli.BoolTFlag to allow
// for other values to be specified
type BoolTFlag struct {
	cli.BoolTFlag
	set *flag.FlagSet
}

// NewBoolTFlag creates a new BoolTFlag
func NewBoolTFlag(flag cli.BoolTFlag) *BoolTFlag {
	return &BoolTFlag{BoolTFlag: flag, set: nil}
}

// ApplyInputSourceValue applies a BoolT value to the flagSet if required
func (f *BoolTFlag) ApplyInputSourceValue(context *cli.Context, isc InputSourceContext) error {
	if f.set != nil {
		if !context.IsSet(f.Name) && !isEnvVarSet(f.EnvVar) {
			value, err := isc.BoolT(f.BoolTFlag.Name)
			if err != nil {
				return err
			}
			if !value {
				eachName(f.Name, func(name string) {
					f.set.Set(f.Name, strconv.FormatBool(value))
				})
			}
		}
	}
	return nil
}

// Apply saves the flagSet for later usage then calls
// the wrapped BoolTFlag.Apply
func (f *BoolTFlag) Apply(set *flag.FlagSet) {
	f.set = set

	f.BoolTFlag.Apply(set)
}

// StringFlag is the flag type that wraps cli.StringFlag to allow
// for other values to be specified
type StringFlag struct {
	cli.StringFlag
	set *flag.FlagSet
}

// NewStringFlag creates a new StringFlag
func NewStringFlag(flag cli.StringFlag) *StringFlag {
	return &StringFlag{StringFlag: flag, set: nil}
}

// ApplyInputSourceValue applies a String value to the flagSet if required
func (f *StringFlag) ApplyInputSourceValue(context *cli.Context, isc InputSourceContext) error {
	if f.set != nil {
		if !(context.IsSet(f.Name) || isEnvVarSet(f.EnvVar)) {
			value, err := isc.String(f.StringFlag.Name)
			if err != nil {
				return err
			}
			if value != "" {
				eachName(f.Name, func(name string) {
					f.set.Set(f.Name, value)
				})
			}
		}
	}
	return nil
}

// Apply saves the flagSet for later usage then calls
// the wrapped StringFlag.Apply
func (f *StringFlag) Apply(set *flag.FlagSet) {
	f.set = set

	f.StringFlag.Apply(set)
}

// IntFlag is the flag type that wraps cli.IntFlag to allow
// for other values to be specified
type IntFlag struct {
	cli.IntFlag
	set *flag.FlagSet
}

// NewIntFlag creates a new IntFlag
func NewIntFlag(flag cli.IntFlag) *IntFlag {
	return &IntFlag{IntFlag: flag, set: nil}
}

// ApplyInputSourceValue applies a int value to the flagSet if required
func (f *IntFlag) ApplyInputSourceValue(context *cli.Context, isc InputSourceContext) error {
	if f.set != nil {
		if !(context.IsSet(f.Name) || isEnvVarSet(f.EnvVar)) {
			value, err := isc.Int(f.IntFlag.Name)
			if err != nil {
				return err
			}
			if value > 0 {
				eachName(f.Name, func(name string) {
					f.set.Set(f.Name, strconv.FormatInt(int64(value), 10))
				})
			}
		}
	}
	return nil
}

// Apply saves the flagSet for later usage then calls
// the wrapped IntFlag.Apply
func (f *IntFlag) Apply(set *flag.FlagSet) {
	f.set = set
	f.IntFlag.Apply(set)
}

// DurationFlag is the flag type that wraps cli.DurationFlag to allow
// for other values to be specified
type DurationFlag struct {
	cli.DurationFlag
	set *flag.FlagSet
}

// NewDurationFlag creates a new DurationFlag
func NewDurationFlag(flag cli.DurationFlag) *DurationFlag {
	return &DurationFlag{DurationFlag: flag, set: nil}
}

// ApplyInputSourceValue applies a Duration value to the flagSet if required
func (f *DurationFlag) ApplyInputSourceValue(context *cli.Context, isc InputSourceContext) error {
	if f.set != nil {
		if !(context.IsSet(f.Name) || isEnvVarSet(f.EnvVar)) {
			value, err := isc.Duration(f.DurationFlag.Name)
			if err != nil {
				return err
			}
			if value > 0 {
				eachName(f.Name, func(name string) {
					f.set.Set(f.Name, value.String())
				})
			}
		}
	}
	return nil
}

// Apply saves the flagSet for later usage then calls
// the wrapped DurationFlag.Apply
func (f *DurationFlag) Apply(set *flag.FlagSet) {
	f.set = set

	f.DurationFlag.Apply(set)
}

// Float64Flag is the flag type that wraps cli.Float64Flag to allow
// for other values to be specified
type Float64Flag struct {
	cli.Float64Flag
	set *flag.FlagSet
}

// NewFloat64Flag creates a new Float64Flag
func NewFloat64Flag(flag cli.Float64Flag) *Float64Flag {
	return &Float64Flag{Float64Flag: flag, set: nil}
}

// ApplyInputSourceValue applies a Float64 value to the flagSet if required
func (f *Float64Flag) ApplyInputSourceValue(context *cli.Context, isc InputSourceContext) error {
	if f.set != nil {
		if !(context.IsSet(f.Name) || isEnvVarSet(f.EnvVar)) {
			value, err := isc.Float64(f.Float64Flag.Name)
			if err != nil {
				return err
			}
			if value > 0 {
				floatStr := float64ToString(value)
				eachName(f.Name, func(name string) {
					f.set.Set(f.Name, floatStr)
				})
			}
		}
	}
	return nil
}

// Apply saves the flagSet for later usage then calls
// the wrapped Float64Flag.Apply
func (f *Float64Flag) Apply(set *flag.FlagSet) {
	f.set = set

	f.Float64Flag.Apply(set)
}

func isEnvVarSet(envVars string) bool {
	for _, envVar := range strings.Split(envVars, ",") {
		envVar = strings.TrimSpace(envVar)
		if envVal := os.Getenv(envVar); envVal != "" {
			// TODO: Can't use this for bools as
			// set means that it was true or false based on
			// Bool flag type, should work for other types
			if len(envVal) > 0 {
				return true
			}
		}
	}

	return false
}

func float64ToString(f float64) string {
	return fmt.Sprintf("%v", f)
}

func eachName(longName string, fn func(string)) {
	parts := strings.Split(longName, ",")
	for _, name := range parts {
		name = strings.Trim(name, " ")
		fn(name)
	}
}
