package colorprofile

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/term"
	"github.com/xo/terminfo"
)

const dumbTerm = "dumb"

// Detect returns the color profile based on the terminal output, and
// environment variables. This respects NO_COLOR, CLICOLOR, and CLICOLOR_FORCE
// environment variables.
//
// The rules as follows:
//   - TERM=dumb is always treated as NoTTY unless CLICOLOR_FORCE=1 is set.
//   - If COLORTERM=truecolor, and the profile is not NoTTY, it gest upgraded to TrueColor.
//   - Using any 256 color terminal (e.g. TERM=xterm-256color) will set the profile to ANSI256.
//   - Using any color terminal (e.g. TERM=xterm-color) will set the profile to ANSI.
//   - Using CLICOLOR=1 without TERM defined should be treated as ANSI if the
//     output is a terminal.
//   - NO_COLOR takes precedence over CLICOLOR/CLICOLOR_FORCE, and will disable
//     colors but not text decoration, i.e. bold, italic, faint, etc.
//
// See https://no-color.org/ and https://bixense.com/clicolors/ for more information.
func Detect(output io.Writer, env []string) Profile {
	out, ok := output.(term.File)
	environ := newEnviron(env)
	isatty := isTTYForced(environ) || (ok && term.IsTerminal(out.Fd()))
	term, ok := environ.lookup("TERM")
	isDumb := !ok || term == dumbTerm
	envp := colorProfile(isatty, environ)
	if envp == TrueColor || envNoColor(environ) {
		// We already know we have TrueColor, or NO_COLOR is set.
		return envp
	}

	if isatty && !isDumb {
		tip := Terminfo(term)
		tmuxp := tmux(environ)

		// Color profile is the maximum of env, terminfo, and tmux.
		return max(envp, max(tip, tmuxp))
	}

	return envp
}

// Env returns the color profile based on the terminal environment variables.
// This respects NO_COLOR, CLICOLOR, and CLICOLOR_FORCE environment variables.
//
// The rules as follows:
//   - TERM=dumb is always treated as NoTTY unless CLICOLOR_FORCE=1 is set.
//   - If COLORTERM=truecolor, and the profile is not NoTTY, it gest upgraded to TrueColor.
//   - Using any 256 color terminal (e.g. TERM=xterm-256color) will set the profile to ANSI256.
//   - Using any color terminal (e.g. TERM=xterm-color) will set the profile to ANSI.
//   - Using CLICOLOR=1 without TERM defined should be treated as ANSI if the
//     output is a terminal.
//   - NO_COLOR takes precedence over CLICOLOR/CLICOLOR_FORCE, and will disable
//     colors but not text decoration, i.e. bold, italic, faint, etc.
//
// See https://no-color.org/ and https://bixense.com/clicolors/ for more information.
func Env(env []string) (p Profile) {
	return colorProfile(true, newEnviron(env))
}

func colorProfile(isatty bool, env environ) (p Profile) {
	term, ok := env.lookup("TERM")
	isDumb := (!ok && runtime.GOOS != "windows") || term == dumbTerm
	envp := envColorProfile(env)
	if !isatty || isDumb {
		// Check if the output is a terminal.
		// Treat dumb terminals as NoTTY
		p = NoTTY
	} else {
		p = envp
	}

	if envNoColor(env) && isatty {
		if p > ASCII {
			p = ASCII
		}
		return //nolint:nakedret
	}

	if cliColorForced(env) {
		if p < ANSI {
			p = ANSI
		}
		if envp > p {
			p = envp
		}

		return //nolint:nakedret
	}

	if cliColor(env) {
		if isatty && !isDumb && p < ANSI {
			p = ANSI
		}
	}

	return p
}

// envNoColor returns true if the environment variables explicitly disable color output
// by setting NO_COLOR (https://no-color.org/).
func envNoColor(env environ) bool {
	noColor, _ := strconv.ParseBool(env.get("NO_COLOR"))
	return noColor
}

func cliColor(env environ) bool {
	cliColor, _ := strconv.ParseBool(env.get("CLICOLOR"))
	return cliColor
}

func cliColorForced(env environ) bool {
	cliColorForce, _ := strconv.ParseBool(env.get("CLICOLOR_FORCE"))
	return cliColorForce
}

func isTTYForced(env environ) bool {
	skip, _ := strconv.ParseBool(env.get("TTY_FORCE"))
	return skip
}

func colorTerm(env environ) bool {
	colorTerm := strings.ToLower(env.get("COLORTERM"))
	return colorTerm == "truecolor" || colorTerm == "24bit" ||
		colorTerm == "yes" || colorTerm == "true"
}

// envColorProfile returns infers the color profile from the environment.
func envColorProfile(env environ) (p Profile) {
	term, ok := env.lookup("TERM")
	if !ok || len(term) == 0 || term == dumbTerm {
		p = NoTTY
		if runtime.GOOS == "windows" {
			// Use Windows API to detect color profile. Windows Terminal and
			// cmd.exe don't define $TERM.
			if wcp, ok := windowsColorProfile(env); ok {
				p = wcp
			}
		}
	} else {
		p = ANSI
	}

	switch {
	case strings.Contains(term, "alacritty"),
		strings.Contains(term, "contour"),
		strings.Contains(term, "foot"),
		strings.Contains(term, "ghostty"),
		strings.Contains(term, "kitty"),
		strings.Contains(term, "rio"),
		strings.Contains(term, "st"),
		strings.Contains(term, "wezterm"):
		return TrueColor
	case strings.HasPrefix(term, "tmux"), strings.HasPrefix(term, "screen"):
		if p < ANSI256 {
			p = ANSI256
		}
	case strings.HasPrefix(term, "xterm"):
		if p < ANSI {
			p = ANSI
		}
	}

	if len(env["WT_SESSION"]) > 0 {
		// Windows Terminal supports TrueColor
		return TrueColor
	}

	if isCloudShell, _ := strconv.ParseBool(env.get("GOOGLE_CLOUD_SHELL")); isCloudShell {
		return TrueColor
	}

	// GNU Screen doesn't support TrueColor
	// Tmux doesn't support $COLORTERM
	if colorTerm(env) && !strings.HasPrefix(term, "screen") && !strings.HasPrefix(term, "tmux") {
		return TrueColor
	}

	if strings.HasSuffix(term, "256color") && p < ANSI256 {
		p = ANSI256
	}

	// Direct color terminals support true colors.
	if strings.HasSuffix(term, "direct") {
		return TrueColor
	}

	return //nolint:nakedret
}

// Terminfo returns the color profile based on the terminal's terminfo
// database. This relies on the Tc and RGB capabilities to determine if the
// terminal supports TrueColor.
// If term is empty or "dumb", it returns NoTTY.
func Terminfo(term string) (p Profile) {
	if len(term) == 0 || term == "dumb" {
		return NoTTY
	}

	p = ANSI
	ti, err := terminfo.Load(term)
	if err != nil {
		return
	}

	extbools := ti.ExtBoolCapsShort()
	if _, ok := extbools["Tc"]; ok {
		return TrueColor
	}

	if _, ok := extbools["RGB"]; ok {
		return TrueColor
	}

	return
}

// Tmux returns the color profile based on `tmux info` output. Tmux supports
// overriding the terminal's color capabilities, so this function will return
// the color profile based on the tmux configuration.
func Tmux(env []string) Profile {
	return tmux(newEnviron(env))
}

// tmux returns the color profile based on the tmux environment variables.
func tmux(env environ) (p Profile) {
	if tmux, ok := env.lookup("TMUX"); !ok || len(tmux) == 0 {
		// Not in tmux
		return NoTTY
	}

	// Check if tmux has either Tc or RGB capabilities. Otherwise, return
	// ANSI256.
	p = ANSI256
	cmd := exec.CommandContext(context.Background(), "tmux", "info")
	out, err := cmd.Output()
	if err != nil {
		return
	}

	for line := range bytes.SplitSeq(out, []byte("\n")) {
		if (bytes.Contains(line, []byte("Tc")) || bytes.Contains(line, []byte("RGB"))) &&
			bytes.Contains(line, []byte("true")) {
			return TrueColor
		}
	}

	return
}

// environ is a map of environment variables.
type environ map[string]string

// newEnviron returns a new environment map from a slice of environment
// variables.
func newEnviron(environ []string) environ {
	m := make(map[string]string, len(environ))
	for _, e := range environ {
		parts := strings.SplitN(e, "=", 2)
		var value string
		if len(parts) == 2 {
			value = parts[1]
		}
		m[parts[0]] = value
	}
	return m
}

// lookup returns the value of an environment variable and a boolean indicating
// if it exists.
func (e environ) lookup(key string) (string, bool) {
	v, ok := e[key]
	return v, ok
}

// get returns the value of an environment variable and empty string if it
// doesn't exist.
func (e environ) get(key string) string {
	v, _ := e.lookup(key)
	return v
}
