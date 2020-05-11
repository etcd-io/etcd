package cli

import (
	"flag"
	"testing"
	"time"
)

func TestNewContext(t *testing.T) {
	set := flag.NewFlagSet("test", 0)
	set.Int("myflag", 12, "doc")
	set.Int64("myflagInt64", int64(12), "doc")
	set.Uint("myflagUint", uint(93), "doc")
	set.Uint64("myflagUint64", uint64(93), "doc")
	set.Float64("myflag64", float64(17), "doc")
	globalSet := flag.NewFlagSet("test", 0)
	globalSet.Int("myflag", 42, "doc")
	globalSet.Int64("myflagInt64", int64(42), "doc")
	globalSet.Uint("myflagUint", uint(33), "doc")
	globalSet.Uint64("myflagUint64", uint64(33), "doc")
	globalSet.Float64("myflag64", float64(47), "doc")
	globalCtx := NewContext(nil, globalSet, nil)
	command := Command{Name: "mycommand"}
	c := NewContext(nil, set, globalCtx)
	c.Command = command
	expect(t, c.Int("myflag"), 12)
	expect(t, c.Int64("myflagInt64"), int64(12))
	expect(t, c.Uint("myflagUint"), uint(93))
	expect(t, c.Uint64("myflagUint64"), uint64(93))
	expect(t, c.Float64("myflag64"), float64(17))
	expect(t, c.GlobalInt("myflag"), 42)
	expect(t, c.GlobalInt64("myflagInt64"), int64(42))
	expect(t, c.GlobalUint("myflagUint"), uint(33))
	expect(t, c.GlobalUint64("myflagUint64"), uint64(33))
	expect(t, c.GlobalFloat64("myflag64"), float64(47))
	expect(t, c.Command.Name, "mycommand")
}

func TestContext_Int(t *testing.T) {
	set := flag.NewFlagSet("test", 0)
	set.Int("myflag", 12, "doc")
	c := NewContext(nil, set, nil)
	expect(t, c.Int("myflag"), 12)
}

func TestContext_Int64(t *testing.T) {
	set := flag.NewFlagSet("test", 0)
	set.Int64("myflagInt64", 12, "doc")
	c := NewContext(nil, set, nil)
	expect(t, c.Int64("myflagInt64"), int64(12))
}

func TestContext_Uint(t *testing.T) {
	set := flag.NewFlagSet("test", 0)
	set.Uint("myflagUint", uint(13), "doc")
	c := NewContext(nil, set, nil)
	expect(t, c.Uint("myflagUint"), uint(13))
}

func TestContext_Uint64(t *testing.T) {
	set := flag.NewFlagSet("test", 0)
	set.Uint64("myflagUint64", uint64(9), "doc")
	c := NewContext(nil, set, nil)
	expect(t, c.Uint64("myflagUint64"), uint64(9))
}

func TestContext_GlobalInt(t *testing.T) {
	set := flag.NewFlagSet("test", 0)
	set.Int("myflag", 12, "doc")
	c := NewContext(nil, set, nil)
	expect(t, c.GlobalInt("myflag"), 12)
	expect(t, c.GlobalInt("nope"), 0)
}

func TestContext_GlobalInt64(t *testing.T) {
	set := flag.NewFlagSet("test", 0)
	set.Int64("myflagInt64", 12, "doc")
	c := NewContext(nil, set, nil)
	expect(t, c.GlobalInt64("myflagInt64"), int64(12))
	expect(t, c.GlobalInt64("nope"), int64(0))
}

func TestContext_Float64(t *testing.T) {
	set := flag.NewFlagSet("test", 0)
	set.Float64("myflag", float64(17), "doc")
	c := NewContext(nil, set, nil)
	expect(t, c.Float64("myflag"), float64(17))
}

func TestContext_GlobalFloat64(t *testing.T) {
	set := flag.NewFlagSet("test", 0)
	set.Float64("myflag", float64(17), "doc")
	c := NewContext(nil, set, nil)
	expect(t, c.GlobalFloat64("myflag"), float64(17))
	expect(t, c.GlobalFloat64("nope"), float64(0))
}

func TestContext_Duration(t *testing.T) {
	set := flag.NewFlagSet("test", 0)
	set.Duration("myflag", time.Duration(12*time.Second), "doc")
	c := NewContext(nil, set, nil)
	expect(t, c.Duration("myflag"), time.Duration(12*time.Second))
}

func TestContext_String(t *testing.T) {
	set := flag.NewFlagSet("test", 0)
	set.String("myflag", "hello world", "doc")
	c := NewContext(nil, set, nil)
	expect(t, c.String("myflag"), "hello world")
}

func TestContext_Bool(t *testing.T) {
	set := flag.NewFlagSet("test", 0)
	set.Bool("myflag", false, "doc")
	c := NewContext(nil, set, nil)
	expect(t, c.Bool("myflag"), false)
}

func TestContext_BoolT(t *testing.T) {
	set := flag.NewFlagSet("test", 0)
	set.Bool("myflag", true, "doc")
	c := NewContext(nil, set, nil)
	expect(t, c.BoolT("myflag"), true)
}

func TestContext_GlobalBool(t *testing.T) {
	set := flag.NewFlagSet("test", 0)

	globalSet := flag.NewFlagSet("test-global", 0)
	globalSet.Bool("myflag", false, "doc")
	globalCtx := NewContext(nil, globalSet, nil)

	c := NewContext(nil, set, globalCtx)
	expect(t, c.GlobalBool("myflag"), false)
	expect(t, c.GlobalBool("nope"), false)
}

func TestContext_GlobalBoolT(t *testing.T) {
	set := flag.NewFlagSet("test", 0)

	globalSet := flag.NewFlagSet("test-global", 0)
	globalSet.Bool("myflag", true, "doc")
	globalCtx := NewContext(nil, globalSet, nil)

	c := NewContext(nil, set, globalCtx)
	expect(t, c.GlobalBoolT("myflag"), true)
	expect(t, c.GlobalBoolT("nope"), false)
}

func TestContext_Args(t *testing.T) {
	set := flag.NewFlagSet("test", 0)
	set.Bool("myflag", false, "doc")
	c := NewContext(nil, set, nil)
	set.Parse([]string{"--myflag", "bat", "baz"})
	expect(t, len(c.Args()), 2)
	expect(t, c.Bool("myflag"), true)
}

func TestContext_NArg(t *testing.T) {
	set := flag.NewFlagSet("test", 0)
	set.Bool("myflag", false, "doc")
	c := NewContext(nil, set, nil)
	set.Parse([]string{"--myflag", "bat", "baz"})
	expect(t, c.NArg(), 2)
}

func TestContext_IsSet(t *testing.T) {
	set := flag.NewFlagSet("test", 0)
	set.Bool("myflag", false, "doc")
	set.String("otherflag", "hello world", "doc")
	globalSet := flag.NewFlagSet("test", 0)
	globalSet.Bool("myflagGlobal", true, "doc")
	globalCtx := NewContext(nil, globalSet, nil)
	c := NewContext(nil, set, globalCtx)
	set.Parse([]string{"--myflag", "bat", "baz"})
	globalSet.Parse([]string{"--myflagGlobal", "bat", "baz"})
	expect(t, c.IsSet("myflag"), true)
	expect(t, c.IsSet("otherflag"), false)
	expect(t, c.IsSet("bogusflag"), false)
	expect(t, c.IsSet("myflagGlobal"), false)
}

func TestContext_GlobalIsSet(t *testing.T) {
	set := flag.NewFlagSet("test", 0)
	set.Bool("myflag", false, "doc")
	set.String("otherflag", "hello world", "doc")
	globalSet := flag.NewFlagSet("test", 0)
	globalSet.Bool("myflagGlobal", true, "doc")
	globalSet.Bool("myflagGlobalUnset", true, "doc")
	globalCtx := NewContext(nil, globalSet, nil)
	c := NewContext(nil, set, globalCtx)
	set.Parse([]string{"--myflag", "bat", "baz"})
	globalSet.Parse([]string{"--myflagGlobal", "bat", "baz"})
	expect(t, c.GlobalIsSet("myflag"), false)
	expect(t, c.GlobalIsSet("otherflag"), false)
	expect(t, c.GlobalIsSet("bogusflag"), false)
	expect(t, c.GlobalIsSet("myflagGlobal"), true)
	expect(t, c.GlobalIsSet("myflagGlobalUnset"), false)
	expect(t, c.GlobalIsSet("bogusGlobal"), false)
}

func TestContext_NumFlags(t *testing.T) {
	set := flag.NewFlagSet("test", 0)
	set.Bool("myflag", false, "doc")
	set.String("otherflag", "hello world", "doc")
	globalSet := flag.NewFlagSet("test", 0)
	globalSet.Bool("myflagGlobal", true, "doc")
	globalCtx := NewContext(nil, globalSet, nil)
	c := NewContext(nil, set, globalCtx)
	set.Parse([]string{"--myflag", "--otherflag=foo"})
	globalSet.Parse([]string{"--myflagGlobal"})
	expect(t, c.NumFlags(), 2)
}

func TestContext_GlobalFlag(t *testing.T) {
	var globalFlag string
	var globalFlagSet bool
	app := NewApp()
	app.Flags = []Flag{
		StringFlag{Name: "global, g", Usage: "global"},
	}
	app.Action = func(c *Context) error {
		globalFlag = c.GlobalString("global")
		globalFlagSet = c.GlobalIsSet("global")
		return nil
	}
	app.Run([]string{"command", "-g", "foo"})
	expect(t, globalFlag, "foo")
	expect(t, globalFlagSet, true)

}

func TestContext_GlobalFlagsInSubcommands(t *testing.T) {
	subcommandRun := false
	parentFlag := false
	app := NewApp()

	app.Flags = []Flag{
		BoolFlag{Name: "debug, d", Usage: "Enable debugging"},
	}

	app.Commands = []Command{
		{
			Name: "foo",
			Flags: []Flag{
				BoolFlag{Name: "parent, p", Usage: "Parent flag"},
			},
			Subcommands: []Command{
				{
					Name: "bar",
					Action: func(c *Context) error {
						if c.GlobalBool("debug") {
							subcommandRun = true
						}
						if c.GlobalBool("parent") {
							parentFlag = true
						}
						return nil
					},
				},
			},
		},
	}

	app.Run([]string{"command", "-d", "foo", "-p", "bar"})

	expect(t, subcommandRun, true)
	expect(t, parentFlag, true)
}

func TestContext_Set(t *testing.T) {
	set := flag.NewFlagSet("test", 0)
	set.Int("int", 5, "an int")
	c := NewContext(nil, set, nil)

	c.Set("int", "1")
	expect(t, c.Int("int"), 1)
}

func TestContext_GlobalSet(t *testing.T) {
	gSet := flag.NewFlagSet("test", 0)
	gSet.Int("int", 5, "an int")

	set := flag.NewFlagSet("sub", 0)
	set.Int("int", 3, "an int")

	pc := NewContext(nil, gSet, nil)
	c := NewContext(nil, set, pc)

	c.Set("int", "1")
	expect(t, c.Int("int"), 1)
	expect(t, c.GlobalInt("int"), 5)

	c.GlobalSet("int", "1")
	expect(t, c.Int("int"), 1)
	expect(t, c.GlobalInt("int"), 1)
}
