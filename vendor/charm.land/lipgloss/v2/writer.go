package lipgloss

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/colorprofile"
)

// Writer is the default writer that prints to stdout, automatically
// downsampling colors when necessary.
var Writer = colorprofile.NewWriter(os.Stdout, os.Environ())

// Println to stdout, automatically downsampling colors when necessary, ending
// with a trailing newline.
//
// Example:
//
//	str := NewStyle().
//	    Foreground(lipgloss.Color("#6a00ff")).
//	    Render("breakfast")
//
//	Println("Time for a", str, "sandwich!")
func Println(v ...any) (int, error) {
	return fmt.Fprintln(Writer, v...) //nolint:wrapcheck
}

// Printf prints formatted text to stdout, automatically downsampling colors
// when necessary.
//
// Example:
//
//	str := NewStyle().
//	  Foreground(lipgloss.Color("#6a00ff")).
//	  Render("knuckle")
//
//	Printf("Time for a %s sandwich!\n", str)
func Printf(format string, v ...any) (int, error) {
	return fmt.Fprintf(Writer, format, v...) //nolint:wrapcheck
}

// Print to stdout, automatically downsampling colors when necessary.
//
// Example:
//
//	str := NewStyle().
//	    Foreground(lipgloss.Color("#6a00ff")).
//	    Render("Who wants marmalade?\n")
//
//	Print(str)
func Print(v ...any) (int, error) {
	return fmt.Fprint(Writer, v...) //nolint:wrapcheck
}

// Fprint pritnts to the given writer, automatically downsampling colors when
// necessary.
//
// Example:
//
//	str := NewStyle().
//	    Foreground(lipgloss.Color("#6a00ff")).
//	    Render("guzzle")
//
//	Fprint(os.Stderr, "I %s horchata pretty much all the time.\n", str)
func Fprint(w io.Writer, v ...any) (int, error) {
	return fmt.Fprint(colorprofile.NewWriter(w, os.Environ()), v...) //nolint:wrapcheck
}

// Fprintln prints to the given writer, automatically downsampling colors when
// necessary, and ending with a trailing newline.
//
// Example:
//
//	str := NewStyle().
//	    Foreground(lipgloss.Color("#6a00ff")).
//	    Render("Sandwich time!")
//
//	Fprintln(os.Stderr, str)
func Fprintln(w io.Writer, v ...any) (int, error) {
	return fmt.Fprintln(colorprofile.NewWriter(w, os.Environ()), v...) //nolint:wrapcheck
}

// Fprintf prints text to a writer, against the given format, automatically
// downsampling colors when necessary.
//
// Example:
//
//	str := NewStyle().
//	    Foreground(lipgloss.Color("#6a00ff")).
//	    Render("artichokes")
//
//	Fprintf(os.Stderr, "I really love %s!\n", food)
func Fprintf(w io.Writer, format string, v ...any) (int, error) {
	return fmt.Fprintf(colorprofile.NewWriter(w, os.Environ()), format, v...) //nolint:wrapcheck
}

// Sprint returns a string for stdout, automatically downsampling colors when
// necessary.
//
// Example:
//
//	str := NewStyle().
//		Faint(true).
//	    Foreground(lipgloss.Color("#6a00ff")).
//	    Render("I love to eat")
//
//	str = Sprint(str)
func Sprint(v ...any) string {
	var buf bytes.Buffer
	w := colorprofile.Writer{
		Forward: &buf,
		Profile: Writer.Profile,
	}
	fmt.Fprint(&w, v...) //nolint:errcheck
	return buf.String()
}

// Sprintln returns a string for stdout, automatically downsampling colors when
// necessary, and ending with a trailing newline.
//
// Example:
//
//	str := NewStyle().
//		Bold(true).
//		Foreground(lipgloss.Color("#6a00ff")).
//		Render("Yummy!")
//
//	str = Sprintln(str)
func Sprintln(v ...any) string {
	var buf bytes.Buffer
	w := colorprofile.Writer{
		Forward: &buf,
		Profile: Writer.Profile,
	}
	fmt.Fprintln(&w, v...) //nolint:errcheck
	return buf.String()
}

// Sprintf returns a formatted string for stdout, automatically downsampling
// colors when necessary.
//
// Example:
//
//	str := NewStyle().
//		Bold(true).
//		Foreground(lipgloss.Color("#fccaee")).
//		Render("Cantaloupe")
//
//	str = Sprintf("I really love %s!", str)
func Sprintf(format string, v ...any) string {
	var buf bytes.Buffer
	w := colorprofile.Writer{
		Forward: &buf,
		Profile: Writer.Profile,
	}
	fmt.Fprintf(&w, format, v...) //nolint:errcheck
	return buf.String()
}
