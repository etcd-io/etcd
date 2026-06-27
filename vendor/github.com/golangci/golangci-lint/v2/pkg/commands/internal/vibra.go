package internal

import (
	"fmt"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type FlagFunc[T any] func(name string, value T, usage string) *T

type FlagPFunc[T any] func(name, shorthand string, value T, usage string) *T

// AddFlagAndBind adds a Cobra/pflag flag and binds it with Viper.
func AddFlagAndBind[T any](v *viper.Viper, fs *pflag.FlagSet, pfn FlagFunc[T], name, bind string, value T, usage string) {
	pfn(name, value, usage)

	err := v.BindPFlag(bind, fs.Lookup(name))
	if err != nil {
		panic(fmt.Sprintf("failed to bind flag %s: %v", name, err))
	}
}

// AddFlagAndBindP adds a Cobra/pflag flag and binds it with Viper.
func AddFlagAndBindP[T any](v *viper.Viper, fs *pflag.FlagSet, pfn FlagPFunc[T], name, shorthand, bind string, value T, usage string) {
	pfn(name, shorthand, value, usage)

	err := v.BindPFlag(bind, fs.Lookup(name))
	if err != nil {
		panic(fmt.Sprintf("failed to bind flag %s: %v", name, err))
	}
}

// AddDeprecatedFlagAndBind similar to AddFlagAndBind but deprecate the flag.
func AddDeprecatedFlagAndBind[T any](v *viper.Viper, fs *pflag.FlagSet, pfn FlagFunc[T], name, bind string, value T, usage string) {
	AddFlagAndBind(v, fs, pfn, name, bind, value, usage)
	deprecateFlag(fs, name)
}

// AddHackedStringSliceP Hack for slice, see Loader.applyStringSliceHack.
func AddHackedStringSliceP(fs *pflag.FlagSet, name, shorthand, usage string) {
	fs.StringSliceP(name, shorthand, nil, usage)
}

// AddHackedStringSlice Hack for slice, see Loader.applyStringSliceHack.
func AddHackedStringSlice(fs *pflag.FlagSet, name, usage string) {
	AddHackedStringSliceP(fs, name, "", usage)
}

// AddDeprecatedHackedStringSlice similar to AddHackedStringSlice but deprecate the flag.
func AddDeprecatedHackedStringSlice(fs *pflag.FlagSet, name, usage string) {
	AddHackedStringSlice(fs, name, usage)
	deprecateFlag(fs, name)
}

func deprecateFlag(fs *pflag.FlagSet, name string) {
	_ = fs.MarkHidden(name)
	_ = fs.MarkDeprecated(name, "check the documentation for more information.")
}
