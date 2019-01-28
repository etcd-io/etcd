package cli

import (
	"errors"
	"flag"
	"strconv"
	"strings"
	"time"
)

// Context is a type that is passed through to
// each Handler action in a cli application. Context
// can be used to retrieve context-specific Args and
// parsed command-line options.
type Context struct {
	App            *App
	Command        Command
	flagSet        *flag.FlagSet
	setFlags       map[string]bool
	globalSetFlags map[string]bool
	parentContext  *Context
}

// NewContext creates a new context. For use in when invoking an App or Command action.
func NewContext(app *App, set *flag.FlagSet, parentCtx *Context) *Context {
	return &Context{App: app, flagSet: set, parentContext: parentCtx}
}

// Int looks up the value of a local int flag, returns 0 if no int flag exists
func (c *Context) Int(name string) int {
	return lookupInt(name, c.flagSet)
}

// Int64 looks up the value of a local int flag, returns 0 if no int flag exists
func (c *Context) Int64(name string) int64 {
	return lookupInt64(name, c.flagSet)
}

// Uint looks up the value of a local int flag, returns 0 if no int flag exists
func (c *Context) Uint(name string) uint {
	return lookupUint(name, c.flagSet)
}

// Uint64 looks up the value of a local int flag, returns 0 if no int flag exists
func (c *Context) Uint64(name string) uint64 {
	return lookupUint64(name, c.flagSet)
}

// Duration looks up the value of a local time.Duration flag, returns 0 if no
// time.Duration flag exists
func (c *Context) Duration(name string) time.Duration {
	return lookupDuration(name, c.flagSet)
}

// Float64 looks up the value of a local float64 flag, returns 0 if no float64
// flag exists
func (c *Context) Float64(name string) float64 {
	return lookupFloat64(name, c.flagSet)
}

// Bool looks up the value of a local bool flag, returns false if no bool flag exists
func (c *Context) Bool(name string) bool {
	return lookupBool(name, c.flagSet)
}

// BoolT looks up the value of a local boolT flag, returns false if no bool flag exists
func (c *Context) BoolT(name string) bool {
	return lookupBoolT(name, c.flagSet)
}

// String looks up the value of a local string flag, returns "" if no string flag exists
func (c *Context) String(name string) string {
	return lookupString(name, c.flagSet)
}

// StringSlice looks up the value of a local string slice flag, returns nil if no
// string slice flag exists
func (c *Context) StringSlice(name string) []string {
	return lookupStringSlice(name, c.flagSet)
}

// IntSlice looks up the value of a local int slice flag, returns nil if no int
// slice flag exists
func (c *Context) IntSlice(name string) []int {
	return lookupIntSlice(name, c.flagSet)
}

// Int64Slice looks up the value of a local int slice flag, returns nil if no int
// slice flag exists
func (c *Context) Int64Slice(name string) []int64 {
	return lookupInt64Slice(name, c.flagSet)
}

// Generic looks up the value of a local generic flag, returns nil if no generic
// flag exists
func (c *Context) Generic(name string) interface{} {
	return lookupGeneric(name, c.flagSet)
}

// GlobalInt looks up the value of a global int flag, returns 0 if no int flag exists
func (c *Context) GlobalInt(name string) int {
	if fs := lookupGlobalFlagSet(name, c); fs != nil {
		return lookupInt(name, fs)
	}
	return 0
}

// GlobalInt64 looks up the value of a global int flag, returns 0 if no int flag exists
func (c *Context) GlobalInt64(name string) int64 {
	if fs := lookupGlobalFlagSet(name, c); fs != nil {
		return lookupInt64(name, fs)
	}
	return 0
}

// GlobalUint looks up the value of a global int flag, returns 0 if no int flag exists
func (c *Context) GlobalUint(name string) uint {
	if fs := lookupGlobalFlagSet(name, c); fs != nil {
		return lookupUint(name, fs)
	}
	return 0
}

// GlobalUint64 looks up the value of a global int flag, returns 0 if no int flag exists
func (c *Context) GlobalUint64(name string) uint64 {
	if fs := lookupGlobalFlagSet(name, c); fs != nil {
		return lookupUint64(name, fs)
	}
	return 0
}

// GlobalFloat64 looks up the value of a global float64 flag, returns float64(0)
// if no float64 flag exists
func (c *Context) GlobalFloat64(name string) float64 {
	if fs := lookupGlobalFlagSet(name, c); fs != nil {
		return lookupFloat64(name, fs)
	}
	return float64(0)
}

// GlobalDuration looks up the value of a global time.Duration flag, returns 0
// if no time.Duration flag exists
func (c *Context) GlobalDuration(name string) time.Duration {
	if fs := lookupGlobalFlagSet(name, c); fs != nil {
		return lookupDuration(name, fs)
	}
	return 0
}

// GlobalBool looks up the value of a global bool flag, returns false if no bool
// flag exists
func (c *Context) GlobalBool(name string) bool {
	if fs := lookupGlobalFlagSet(name, c); fs != nil {
		return lookupBool(name, fs)
	}
	return false
}

// GlobalBoolT looks up the value of a global bool flag, returns true if no bool
// flag exists
func (c *Context) GlobalBoolT(name string) bool {
	if fs := lookupGlobalFlagSet(name, c); fs != nil {
		return lookupBoolT(name, fs)
	}
	return false
}

// GlobalString looks up the value of a global string flag, returns "" if no
// string flag exists
func (c *Context) GlobalString(name string) string {
	if fs := lookupGlobalFlagSet(name, c); fs != nil {
		return lookupString(name, fs)
	}
	return ""
}

// GlobalStringSlice looks up the value of a global string slice flag, returns
// nil if no string slice flag exists
func (c *Context) GlobalStringSlice(name string) []string {
	if fs := lookupGlobalFlagSet(name, c); fs != nil {
		return lookupStringSlice(name, fs)
	}
	return nil
}

// GlobalIntSlice looks up the value of a global int slice flag, returns nil if
// no int slice flag exists
func (c *Context) GlobalIntSlice(name string) []int {
	if fs := lookupGlobalFlagSet(name, c); fs != nil {
		return lookupIntSlice(name, fs)
	}
	return nil
}

// GlobalInt64Slice looks up the value of a global int slice flag, returns nil if
// no int slice flag exists
func (c *Context) GlobalInt64Slice(name string) []int64 {
	if fs := lookupGlobalFlagSet(name, c); fs != nil {
		return lookupInt64Slice(name, fs)
	}
	return nil
}

// GlobalGeneric looks up the value of a global generic flag, returns nil if no
// generic flag exists
func (c *Context) GlobalGeneric(name string) interface{} {
	if fs := lookupGlobalFlagSet(name, c); fs != nil {
		return lookupGeneric(name, fs)
	}
	return nil
}

// NumFlags returns the number of flags set
func (c *Context) NumFlags() int {
	return c.flagSet.NFlag()
}

// Set sets a context flag to a value.
func (c *Context) Set(name, value string) error {
	return c.flagSet.Set(name, value)
}

// GlobalSet sets a context flag to a value on the global flagset
func (c *Context) GlobalSet(name, value string) error {
	return globalContext(c).flagSet.Set(name, value)
}

// IsSet determines if the flag was actually set
func (c *Context) IsSet(name string) bool {
	if c.setFlags == nil {
		c.setFlags = make(map[string]bool)
		c.flagSet.Visit(func(f *flag.Flag) {
			c.setFlags[f.Name] = true
		})
	}
	return c.setFlags[name] == true
}

// GlobalIsSet determines if the global flag was actually set
func (c *Context) GlobalIsSet(name string) bool {
	if c.globalSetFlags == nil {
		c.globalSetFlags = make(map[string]bool)
		ctx := c
		if ctx.parentContext != nil {
			ctx = ctx.parentContext
		}
		for ; ctx != nil && c.globalSetFlags[name] == false; ctx = ctx.parentContext {
			ctx.flagSet.Visit(func(f *flag.Flag) {
				c.globalSetFlags[f.Name] = true
			})
		}
	}
	return c.globalSetFlags[name]
}

// FlagNames returns a slice of flag names used in this context.
func (c *Context) FlagNames() (names []string) {
	for _, flag := range c.Command.Flags {
		name := strings.Split(flag.GetName(), ",")[0]
		if name == "help" {
			continue
		}
		names = append(names, name)
	}
	return
}

// GlobalFlagNames returns a slice of global flag names used by the app.
func (c *Context) GlobalFlagNames() (names []string) {
	for _, flag := range c.App.Flags {
		name := strings.Split(flag.GetName(), ",")[0]
		if name == "help" || name == "version" {
			continue
		}
		names = append(names, name)
	}
	return
}

// Parent returns the parent context, if any
func (c *Context) Parent() *Context {
	return c.parentContext
}

// Args contains apps console arguments
type Args []string

// Args returns the command line arguments associated with the context.
func (c *Context) Args() Args {
	args := Args(c.flagSet.Args())
	return args
}

// NArg returns the number of the command line arguments.
func (c *Context) NArg() int {
	return len(c.Args())
}

// Get returns the nth argument, or else a blank string
func (a Args) Get(n int) string {
	if len(a) > n {
		return a[n]
	}
	return ""
}

// First returns the first argument, or else a blank string
func (a Args) First() string {
	return a.Get(0)
}

// Tail returns the rest of the arguments (not the first one)
// or else an empty string slice
func (a Args) Tail() []string {
	if len(a) >= 2 {
		return []string(a)[1:]
	}
	return []string{}
}

// Present checks if there are any arguments present
func (a Args) Present() bool {
	return len(a) != 0
}

// Swap swaps arguments at the given indexes
func (a Args) Swap(from, to int) error {
	if from >= len(a) || to >= len(a) {
		return errors.New("index out of range")
	}
	a[from], a[to] = a[to], a[from]
	return nil
}

func globalContext(ctx *Context) *Context {
	if ctx == nil {
		return nil
	}

	for {
		if ctx.parentContext == nil {
			return ctx
		}
		ctx = ctx.parentContext
	}
}

func lookupGlobalFlagSet(name string, ctx *Context) *flag.FlagSet {
	if ctx.parentContext != nil {
		ctx = ctx.parentContext
	}
	for ; ctx != nil; ctx = ctx.parentContext {
		if f := ctx.flagSet.Lookup(name); f != nil {
			return ctx.flagSet
		}
	}
	return nil
}

func lookupInt(name string, set *flag.FlagSet) int {
	f := set.Lookup(name)
	if f != nil {
		val, err := strconv.ParseInt(f.Value.String(), 0, 64)
		if err != nil {
			return 0
		}
		return int(val)
	}

	return 0
}

func lookupInt64(name string, set *flag.FlagSet) int64 {
	f := set.Lookup(name)
	if f != nil {
		val, err := strconv.ParseInt(f.Value.String(), 0, 64)
		if err != nil {
			return 0
		}
		return val
	}

	return 0
}

func lookupUint(name string, set *flag.FlagSet) uint {
	f := set.Lookup(name)
	if f != nil {
		val, err := strconv.ParseUint(f.Value.String(), 0, 64)
		if err != nil {
			return 0
		}
		return uint(val)
	}

	return 0
}

func lookupUint64(name string, set *flag.FlagSet) uint64 {
	f := set.Lookup(name)
	if f != nil {
		val, err := strconv.ParseUint(f.Value.String(), 0, 64)
		if err != nil {
			return 0
		}
		return val
	}

	return 0
}

func lookupDuration(name string, set *flag.FlagSet) time.Duration {
	f := set.Lookup(name)
	if f != nil {
		val, err := time.ParseDuration(f.Value.String())
		if err == nil {
			return val
		}
	}

	return 0
}

func lookupFloat64(name string, set *flag.FlagSet) float64 {
	f := set.Lookup(name)
	if f != nil {
		val, err := strconv.ParseFloat(f.Value.String(), 64)
		if err != nil {
			return 0
		}
		return val
	}

	return 0
}

func lookupString(name string, set *flag.FlagSet) string {
	f := set.Lookup(name)
	if f != nil {
		return f.Value.String()
	}

	return ""
}

func lookupStringSlice(name string, set *flag.FlagSet) []string {
	f := set.Lookup(name)
	if f != nil {
		return (f.Value.(*StringSlice)).Value()

	}

	return nil
}

func lookupIntSlice(name string, set *flag.FlagSet) []int {
	f := set.Lookup(name)
	if f != nil {
		return (f.Value.(*IntSlice)).Value()

	}

	return nil
}

func lookupInt64Slice(name string, set *flag.FlagSet) []int64 {
	f := set.Lookup(name)
	if f != nil {
		return (f.Value.(*Int64Slice)).Value()

	}

	return nil
}

func lookupGeneric(name string, set *flag.FlagSet) interface{} {
	f := set.Lookup(name)
	if f != nil {
		return f.Value
	}
	return nil
}

func lookupBool(name string, set *flag.FlagSet) bool {
	f := set.Lookup(name)
	if f != nil {
		val, err := strconv.ParseBool(f.Value.String())
		if err != nil {
			return false
		}
		return val
	}

	return false
}

func lookupBoolT(name string, set *flag.FlagSet) bool {
	f := set.Lookup(name)
	if f != nil {
		val, err := strconv.ParseBool(f.Value.String())
		if err != nil {
			return true
		}
		return val
	}

	return false
}

func copyFlag(name string, ff *flag.Flag, set *flag.FlagSet) {
	switch ff.Value.(type) {
	case *StringSlice:
	default:
		set.Set(name, ff.Value.String())
	}
}

func normalizeFlags(flags []Flag, set *flag.FlagSet) error {
	visited := make(map[string]bool)
	set.Visit(func(f *flag.Flag) {
		visited[f.Name] = true
	})
	for _, f := range flags {
		parts := strings.Split(f.GetName(), ",")
		if len(parts) == 1 {
			continue
		}
		var ff *flag.Flag
		for _, name := range parts {
			name = strings.Trim(name, " ")
			if visited[name] {
				if ff != nil {
					return errors.New("Cannot use two forms of the same flag: " + name + " " + ff.Name)
				}
				ff = set.Lookup(name)
			}
		}
		if ff == nil {
			continue
		}
		for _, name := range parts {
			name = strings.Trim(name, " ")
			if !visited[name] {
				copyFlag(name, ff, set)
			}
		}
	}
	return nil
}
