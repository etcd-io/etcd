package cli

import (
	"bytes"
	"flag"
	"strings"
	"testing"
)

func Test_ShowAppHelp_NoAuthor(t *testing.T) {
	output := new(bytes.Buffer)
	app := NewApp()
	app.Writer = output

	c := NewContext(app, nil, nil)

	ShowAppHelp(c)

	if bytes.Index(output.Bytes(), []byte("AUTHOR(S):")) != -1 {
		t.Errorf("expected\n%snot to include %s", output.String(), "AUTHOR(S):")
	}
}

func Test_ShowAppHelp_NoVersion(t *testing.T) {
	output := new(bytes.Buffer)
	app := NewApp()
	app.Writer = output

	app.Version = ""

	c := NewContext(app, nil, nil)

	ShowAppHelp(c)

	if bytes.Index(output.Bytes(), []byte("VERSION:")) != -1 {
		t.Errorf("expected\n%snot to include %s", output.String(), "VERSION:")
	}
}

func Test_ShowAppHelp_HideVersion(t *testing.T) {
	output := new(bytes.Buffer)
	app := NewApp()
	app.Writer = output

	app.HideVersion = true

	c := NewContext(app, nil, nil)

	ShowAppHelp(c)

	if bytes.Index(output.Bytes(), []byte("VERSION:")) != -1 {
		t.Errorf("expected\n%snot to include %s", output.String(), "VERSION:")
	}
}

func Test_Help_Custom_Flags(t *testing.T) {
	oldFlag := HelpFlag
	defer func() {
		HelpFlag = oldFlag
	}()

	HelpFlag = BoolFlag{
		Name:  "help, x",
		Usage: "show help",
	}

	app := App{
		Flags: []Flag{
			BoolFlag{Name: "foo, h"},
		},
		Action: func(ctx *Context) error {
			if ctx.Bool("h") != true {
				t.Errorf("custom help flag not set")
			}
			return nil
		},
	}
	output := new(bytes.Buffer)
	app.Writer = output
	app.Run([]string{"test", "-h"})
	if output.Len() > 0 {
		t.Errorf("unexpected output: %s", output.String())
	}
}

func Test_Version_Custom_Flags(t *testing.T) {
	oldFlag := VersionFlag
	defer func() {
		VersionFlag = oldFlag
	}()

	VersionFlag = BoolFlag{
		Name:  "version, V",
		Usage: "show version",
	}

	app := App{
		Flags: []Flag{
			BoolFlag{Name: "foo, v"},
		},
		Action: func(ctx *Context) error {
			if ctx.Bool("v") != true {
				t.Errorf("custom version flag not set")
			}
			return nil
		},
	}
	output := new(bytes.Buffer)
	app.Writer = output
	app.Run([]string{"test", "-v"})
	if output.Len() > 0 {
		t.Errorf("unexpected output: %s", output.String())
	}
}

func Test_helpCommand_Action_ErrorIfNoTopic(t *testing.T) {
	app := NewApp()

	set := flag.NewFlagSet("test", 0)
	set.Parse([]string{"foo"})

	c := NewContext(app, set, nil)

	err := helpCommand.Action.(func(*Context) error)(c)

	if err == nil {
		t.Fatalf("expected error from helpCommand.Action(), but got nil")
	}

	exitErr, ok := err.(*ExitError)
	if !ok {
		t.Fatalf("expected ExitError from helpCommand.Action(), but instead got: %v", err.Error())
	}

	if !strings.HasPrefix(exitErr.Error(), "No help topic for") {
		t.Fatalf("expected an unknown help topic error, but got: %v", exitErr.Error())
	}

	if exitErr.exitCode != 3 {
		t.Fatalf("expected exit value = 3, got %d instead", exitErr.exitCode)
	}
}

func Test_helpCommand_InHelpOutput(t *testing.T) {
	app := NewApp()
	output := &bytes.Buffer{}
	app.Writer = output
	app.Run([]string{"test", "--help"})

	s := output.String()

	if strings.Contains(s, "\nCOMMANDS:\nGLOBAL OPTIONS:\n") {
		t.Fatalf("empty COMMANDS section detected: %q", s)
	}

	if !strings.Contains(s, "help, h") {
		t.Fatalf("missing \"help, h\": %q", s)
	}
}

func Test_helpSubcommand_Action_ErrorIfNoTopic(t *testing.T) {
	app := NewApp()

	set := flag.NewFlagSet("test", 0)
	set.Parse([]string{"foo"})

	c := NewContext(app, set, nil)

	err := helpSubcommand.Action.(func(*Context) error)(c)

	if err == nil {
		t.Fatalf("expected error from helpCommand.Action(), but got nil")
	}

	exitErr, ok := err.(*ExitError)
	if !ok {
		t.Fatalf("expected ExitError from helpCommand.Action(), but instead got: %v", err.Error())
	}

	if !strings.HasPrefix(exitErr.Error(), "No help topic for") {
		t.Fatalf("expected an unknown help topic error, but got: %v", exitErr.Error())
	}

	if exitErr.exitCode != 3 {
		t.Fatalf("expected exit value = 3, got %d instead", exitErr.exitCode)
	}
}

func TestShowAppHelp_CommandAliases(t *testing.T) {
	app := &App{
		Commands: []Command{
			{
				Name:    "frobbly",
				Aliases: []string{"fr", "frob"},
				Action: func(ctx *Context) error {
					return nil
				},
			},
		},
	}

	output := &bytes.Buffer{}
	app.Writer = output
	app.Run([]string{"foo", "--help"})

	if !strings.Contains(output.String(), "frobbly, fr, frob") {
		t.Errorf("expected output to include all command aliases; got: %q", output.String())
	}
}

func TestShowCommandHelp_CommandAliases(t *testing.T) {
	app := &App{
		Commands: []Command{
			{
				Name:    "frobbly",
				Aliases: []string{"fr", "frob", "bork"},
				Action: func(ctx *Context) error {
					return nil
				},
			},
		},
	}

	output := &bytes.Buffer{}
	app.Writer = output
	app.Run([]string{"foo", "help", "fr"})

	if !strings.Contains(output.String(), "frobbly") {
		t.Errorf("expected output to include command name; got: %q", output.String())
	}

	if strings.Contains(output.String(), "bork") {
		t.Errorf("expected output to exclude command aliases; got: %q", output.String())
	}
}

func TestShowSubcommandHelp_CommandAliases(t *testing.T) {
	app := &App{
		Commands: []Command{
			{
				Name:    "frobbly",
				Aliases: []string{"fr", "frob", "bork"},
				Action: func(ctx *Context) error {
					return nil
				},
			},
		},
	}

	output := &bytes.Buffer{}
	app.Writer = output
	app.Run([]string{"foo", "help"})

	if !strings.Contains(output.String(), "frobbly, fr, frob, bork") {
		t.Errorf("expected output to include all command aliases; got: %q", output.String())
	}
}

func TestShowAppHelp_HiddenCommand(t *testing.T) {
	app := &App{
		Commands: []Command{
			{
				Name: "frobbly",
				Action: func(ctx *Context) error {
					return nil
				},
			},
			{
				Name:   "secretfrob",
				Hidden: true,
				Action: func(ctx *Context) error {
					return nil
				},
			},
		},
	}

	output := &bytes.Buffer{}
	app.Writer = output
	app.Run([]string{"app", "--help"})

	if strings.Contains(output.String(), "secretfrob") {
		t.Errorf("expected output to exclude \"secretfrob\"; got: %q", output.String())
	}

	if !strings.Contains(output.String(), "frobbly") {
		t.Errorf("expected output to include \"frobbly\"; got: %q", output.String())
	}
}
