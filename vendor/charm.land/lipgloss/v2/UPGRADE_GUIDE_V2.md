# Lip Gloss v2 Upgrade Guide

This guide covers migrating from Lip Gloss v1 (`github.com/charmbracelet/lipgloss`)
to Lip Gloss v2 (`charm.land/lipgloss/v2`). It is written for both humans and
LLMs performing automated migrations.

---

## Table of Contents

1. [Quick Start](#quick-start)
2. [Module Path](#module-path)
3. [Color System](#color-system)
4. [Renderer Removal](#renderer-removal)
5. [Printing and Color Downsampling](#printing-and-color-downsampling)
6. [Background Detection and Adaptive Colors](#background-detection-and-adaptive-colors)
7. [Whitespace Options](#whitespace-options)
8. [Underline](#underline)
9. [Style API Changes](#style-api-changes)
10. [Tree Subpackage](#tree-subpackage)
11. [Removed APIs](#removed-apis)
12. [Quick Reference Table](#quick-reference-table)

---

## Quick Start

For the fastest possible upgrade, do these two things:

### 1. Use the `compat` package for adaptive/complete colors

```go
import "charm.land/lipgloss/v2/compat"

// v1
color := lipgloss.AdaptiveColor{Light: "#f1f1f1", Dark: "#cccccc"}

// v2
color := compat.AdaptiveColor{Light: lipgloss.Color("#f1f1f1"), Dark: lipgloss.Color("#cccccc")}
```

The `compat` package reads `stdin`/`stdout` globally, just like v1. To
customize:

```go
import (
    "charm.land/lipgloss/v2/compat"
    "github.com/charmbracelet/colorprofile"
)

func init() {
    compat.HasDarkBackground = lipgloss.HasDarkBackground(os.Stdin, os.Stderr)
    compat.Profile = colorprofile.Detect(os.Stderr, os.Environ())
}
```

### 2. Use Lip Gloss writers for output

```go
// v1
fmt.Println(s)

// v2
lipgloss.Println(s)
```

This ensures colors are automatically downsampled. If you're using Bubble Tea
v2, this step is unnecessary — Bubble Tea handles it for you.

**That's the quick path.** Read on for the full migration details.

---

## Module Path

The import path has changed.

```go
// v1
import "github.com/charmbracelet/lipgloss"

// v2
import "charm.land/lipgloss/v2"
```

**Install:**

```bash
go get charm.land/lipgloss/v2
```

All subpackages follow the same pattern:

```go
// v1
import "github.com/charmbracelet/lipgloss/table"
import "github.com/charmbracelet/lipgloss/tree"
import "github.com/charmbracelet/lipgloss/list"

// v2
import "charm.land/lipgloss/v2/table"
import "charm.land/lipgloss/v2/tree"
import "charm.land/lipgloss/v2/list"
```

**Search-and-replace pattern:**

```
github.com/charmbracelet/lipgloss → charm.land/lipgloss/v2
```

---

## Color System

This is the most significant API change.

### `Color` is now a function, not a type

```go
// v1 — Color is a string type
var c lipgloss.Color = "21"
var c lipgloss.Color = "#ff00ff"

// v2 — Color is a function returning color.Color
var c color.Color = lipgloss.Color("21")
var c color.Color = lipgloss.Color("#ff00ff")
```

The return type is `image/color.Color` (from the standard library).

### `TerminalColor` interface is removed

All methods that accepted `lipgloss.TerminalColor` now accept
`image/color.Color`:

```go
// v1
func (s Style) Foreground(c TerminalColor) Style
func (s Style) Background(c TerminalColor) Style
func (s Style) BorderForeground(c ...TerminalColor) Style

// v2
func (s Style) Foreground(c color.Color) Style
func (s Style) Background(c color.Color) Style
func (s Style) BorderForeground(c ...color.Color) Style
```

**Migration:** Replace every `lipgloss.TerminalColor` with `color.Color` and
add `import "image/color"`.

### `ANSIColor` is now an alias

```go
// v1 — custom uint type
type ANSIColor uint

// v2 — alias for ansi.IndexedColor
type ANSIColor = ansi.IndexedColor
```

v2 also exports named constants for the 16 basic ANSI colors:

```go
lipgloss.Black, lipgloss.Red, lipgloss.Green, lipgloss.Yellow,
lipgloss.Blue, lipgloss.Magenta, lipgloss.Cyan, lipgloss.White,
lipgloss.BrightBlack, lipgloss.BrightRed, lipgloss.BrightGreen,
lipgloss.BrightYellow, lipgloss.BrightBlue, lipgloss.BrightMagenta,
lipgloss.BrightCyan, lipgloss.BrightWhite
```

### `AdaptiveColor`, `CompleteColor`, `CompleteAdaptiveColor`

These types have been moved out of the root package. Use the `compat` package
for a drop-in replacement, or use the new `LightDark` and `Complete` helpers
for explicit control:

```go
// v1
color := lipgloss.AdaptiveColor{Light: "#0000ff", Dark: "#000099"}

// v2 — using compat (quick path)
color := compat.AdaptiveColor{
    Light: lipgloss.Color("#0000ff"),
    Dark:  lipgloss.Color("#000099"),
}

// v2 — using LightDark (recommended)
hasDark := lipgloss.HasDarkBackground(os.Stdin, os.Stdout)
lightDark := lipgloss.LightDark(hasDark)
color := lightDark(lipgloss.Color("#0000ff"), lipgloss.Color("#000099"))
```

```go
// v1
color := lipgloss.CompleteColor{TrueColor: "#ff00ff", ANSI256: "200", ANSI: "5"}

// v2 — using compat
color := compat.CompleteColor{
    TrueColor: lipgloss.Color("#ff00ff"),
    ANSI256:   lipgloss.Color("200"),
    ANSI:      lipgloss.Color("5"),
}

// v2 — using Complete (recommended)
profile := colorprofile.Detect(os.Stdout, os.Environ())
complete := lipgloss.Complete(profile)
color := complete(lipgloss.Color("5"), lipgloss.Color("200"), lipgloss.Color("#ff00ff"))
```

Note that `compat.AdaptiveColor` and friends take `color.Color` values for
their fields, not strings.

---

## Renderer Removal

The `Renderer` type and all associated functions are removed. In v1, every
`Style` carried a `*Renderer` pointer and the package maintained a global
default renderer.

```go
// v1 — these no longer exist
lipgloss.DefaultRenderer()
lipgloss.SetDefaultRenderer(r)
lipgloss.NewRenderer(w, opts...)
lipgloss.ColorProfile()
lipgloss.SetColorProfile(p)
renderer.NewStyle()
```

**In v2, `Style` is a plain value type.** There is no renderer. Color
downsampling is handled at the output layer (see next section).

**Migration:**

- Replace `lipgloss.DefaultRenderer().NewStyle()` with `lipgloss.NewStyle()`.
- Replace `renderer.NewStyle()` with `lipgloss.NewStyle()`.
- Remove any `*Renderer` fields from your types.
- Remove calls to `SetColorProfile` — use `colorprofile.Detect` at the output
  layer instead.

---

## Printing and Color Downsampling

In v1, color downsampling happened inside `Style.Render()` via the renderer. In
v2, `Render()` always emits full-fidelity ANSI. Downsampling happens when you
print.

### Standalone Usage

Use the Lip Gloss writer functions:

```go
s := someStyle.Render("Hello!")

// Print to stdout with automatic downsampling
lipgloss.Println(s)

// Print to stderr
lipgloss.Fprintln(os.Stderr, s)

// Render to a string (downsampled for stdout's profile)
str := lipgloss.Sprint(s)
```

The default writer targets `stdout`. To customize:

```go
lipgloss.Writer = colorprofile.NewWriter(os.Stderr, os.Environ())
```

### With Bubble Tea

No changes needed. Bubble Tea v2 handles downsampling internally.

---

## Background Detection and Adaptive Colors

### Standalone

v1 detected the background color automatically via the global renderer. v2
requires explicit queries:

```go
// v1
hasDark := lipgloss.HasDarkBackground()

// v2 — specify the input and output
hasDark := lipgloss.HasDarkBackground(os.Stdin, os.Stdout)
```

Then use `LightDark` to pick colors:

```go
lightDark := lipgloss.LightDark(hasDark)
fg := lightDark(lipgloss.Color("#333333"), lipgloss.Color("#f1f1f1"))

s := lipgloss.NewStyle().Foreground(fg)
```

### With Bubble Tea

Request the background color in `Init` and listen for the response:

```go
func (m model) Init() tea.Cmd {
    return tea.RequestBackgroundColor
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.BackgroundColorMsg:
        m.styles = newStyles(msg.IsDark())
    }
    // ...
}

func newStyles(bgIsDark bool) styles {
    lightDark := lipgloss.LightDark(bgIsDark)
    return styles{
        title: lipgloss.NewStyle().Foreground(lightDark(
            lipgloss.Color("#333333"),
            lipgloss.Color("#f1f1f1"),
        )),
    }
}
```

---

## Whitespace Options

The separate foreground/background whitespace options have been replaced by a
single style option:

```go
// v1
lipgloss.Place(width, height, hPos, vPos, str,
    lipgloss.WithWhitespaceForeground(lipgloss.Color("#333")),
    lipgloss.WithWhitespaceBackground(lipgloss.Color("#000")),
)

// v2
lipgloss.Place(width, height, hPos, vPos, str,
    lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().
        Foreground(lipgloss.Color("#333")).
        Background(lipgloss.Color("#000")),
    ),
)
```

---

## Underline

`Underline(bool)` still works for basic on/off. v2 adds fine-grained control:

```go
// v1
s := lipgloss.NewStyle().Underline(true)

// v2 — still works
s := lipgloss.NewStyle().Underline(true)

// v2 — new: specific styles
s := lipgloss.NewStyle().UnderlineStyle(lipgloss.UnderlineCurly)

// v2 — new: colored underlines
s := lipgloss.NewStyle().
    UnderlineStyle(lipgloss.UnderlineSingle).
    UnderlineColor(lipgloss.Color("#FF0000"))
```

Internally, `Underline(true)` is equivalent to `UnderlineStyle(UnderlineSingle)`
and `Underline(false)` is equivalent to `UnderlineStyle(UnderlineNone)`.

---

## Style API Changes

### `NewStyle()` is no longer tied to a Renderer

```go
// v1
s := lipgloss.NewStyle()         // uses global renderer
s := renderer.NewStyle()          // uses specific renderer

// v2
s := lipgloss.NewStyle()         // pure value, no renderer
```

### Color getters return `color.Color`

```go
// v1
fg := s.GetForeground() // returns TerminalColor

// v2
fg := s.GetForeground() // returns color.Color
```

### New style methods

| Method | Description |
|---|---|
| `UnderlineStyle(Underline)` | Set underline style (single, double, curly, etc.) |
| `UnderlineColor(color.Color)` | Set underline color |
| `PaddingChar(rune)` | Set the character used for padding fill |
| `MarginChar(rune)` | Set the character used for margin fill |
| `Hyperlink(link, params...)` | Set a clickable hyperlink |
| `BorderForegroundBlend(...color.Color)` | Apply gradient colors to borders |
| `BorderForegroundBlendOffset(int)` | Set the offset for border gradient |

Each has a corresponding `Get*`, `Unset*`, and where applicable `Get*`
accessor.

---

## Tree Subpackage

The import path changes and there are new styling options:

```go
// v1
import "github.com/charmbracelet/lipgloss/tree"

// v2
import "charm.land/lipgloss/v2/tree"
```

New methods:

- `IndenterStyle(lipgloss.Style)` — set a static style for tree indentation.
- `IndenterStyleFunc(func(Children, int) lipgloss.Style)` — conditionally style
  indentation.
- `Width(int)` — set tree width for padding.

---

## Removed APIs

The following types and functions no longer exist in v2. This table shows each
removed symbol and its replacement.

| v1 Symbol | v2 Replacement |
|---|---|
| `type Renderer` | Removed entirely |
| `DefaultRenderer()` | Not needed |
| `SetDefaultRenderer(r)` | Not needed |
| `NewRenderer(w, opts...)` | Not needed |
| `ColorProfile()` | `colorprofile.Detect(w, env)` |
| `SetColorProfile(p)` | Set `lipgloss.Writer.Profile` |
| `HasDarkBackground()` (no args) | `lipgloss.HasDarkBackground(in, out)` |
| `SetHasDarkBackground(b)` | Not needed — pass bool to `LightDark` |
| `type TerminalColor` | `image/color.Color` |
| `type Color string` | `func Color(string) color.Color` |
| `type ANSIColor uint` | `type ANSIColor = ansi.IndexedColor` |
| `type AdaptiveColor` | `compat.AdaptiveColor` or `LightDark` |
| `type CompleteColor` | `compat.CompleteColor` or `Complete` |
| `type CompleteAdaptiveColor` | `compat.CompleteAdaptiveColor` |
| `WithWhitespaceForeground(c)` | `WithWhitespaceStyle(s)` |
| `WithWhitespaceBackground(c)` | `WithWhitespaceStyle(s)` |
| `renderer.NewStyle()` | `lipgloss.NewStyle()` |

---

## Quick Reference Table

A side-by-side summary for common patterns:

| Task | v1 | v2 |
|---|---|---|
| Import | `"github.com/charmbracelet/lipgloss"` | `"charm.land/lipgloss/v2"` |
| Create style | `lipgloss.NewStyle()` | `lipgloss.NewStyle()` |
| Hex color | `lipgloss.Color("#ff00ff")` | `lipgloss.Color("#ff00ff")` |
| ANSI color | `lipgloss.Color("5")` | `lipgloss.Color("5")` or `lipgloss.Magenta` |
| Adaptive color | `lipgloss.AdaptiveColor{Light: "#fff", Dark: "#000"}` | `compat.AdaptiveColor{Light: lipgloss.Color("#fff"), Dark: lipgloss.Color("#000")}` |
| Set foreground | `s.Foreground(lipgloss.Color("5"))` | `s.Foreground(lipgloss.Color("5"))` |
| Print with downsampling | `fmt.Println(s.Render("hi"))` | `lipgloss.Println(s.Render("hi"))` |
| Detect dark bg | `lipgloss.HasDarkBackground()` | `lipgloss.HasDarkBackground(os.Stdin, os.Stdout)` |
| Light/dark color | `lipgloss.AdaptiveColor{...}` | `lipgloss.LightDark(isDark)(light, dark)` |
| Whitespace styling | `WithWhitespaceForeground(c)` | `WithWhitespaceStyle(lipgloss.NewStyle().Foreground(c))` |
| Underline | `s.Underline(true)` | `s.Underline(true)` or `s.UnderlineStyle(lipgloss.UnderlineCurly)` |

---

## Feedback

Questions, issues, or feedback:

- [Discord](https://charm.land/discord)
- [Matrix](https://charm.land/matrix)
- [Email](mailto:vt100@charm.land)

---

Part of [Charm](https://charm.land).

<a href="https://charm.land/"><img alt="The Charm logo" src="https://stuff.charm.sh/charm-badge.jpg" width="400"></a>

Charm热爱开源 • Charm loves open source • نحنُ نحب المصادر المفتوحة
