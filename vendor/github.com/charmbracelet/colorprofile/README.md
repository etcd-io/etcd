# Colorprofile

<p>
    <a href="https://github.com/charmbracelet/colorprofile/releases"><img src="https://img.shields.io/github/release/charmbracelet/colorprofile.svg" alt="Latest Release"></a>
    <a href="https://pkg.go.dev/github.com/charmbracelet/colorprofile?tab=doc"><img src="https://godoc.org/github.com/charmbracelet/colorprofile?status.svg" alt="GoDoc"></a>
    <a href="https://github.com/charmbracelet/colorprofile/actions"><img src="https://github.com/charmbracelet/colorprofile/actions/workflows/build.yml/badge.svg" alt="Build Status"></a>
</p>

A simple, powerful—and at times magical—package for detecting terminal color
profiles and performing color (and CSI) degradation.

## Detecting the terminal’s color profile

Detecting the terminal’s color profile is easy.

```go
import "github.com/charmbracelet/colorprofile"

// Detect the color profile. If you’re planning on writing to stderr you'd want
// to use os.Stderr instead.
p := colorprofile.Detect(os.Stdout, os.Environ())

// Comment on the profile.
fmt.Printf("You know, your colors are quite %s.", func() string {
    switch p {
    case colorprofile.TrueColor:
        return "fancy"
    case colorprofile.ANSI256:
        return "1990s fancy"
    case colorprofile.ANSI:
        return "normcore"
    case colorprofile.Ascii:
        return "ancient"
    case colorprofile.NoTTY:
        return "naughty!"
    }
    return "...IDK" // this should never happen
}())
```

## Downsampling colors

When necessary, colors can be downsampled to a given profile, or manually
downsampled to a specific profile.

```go
p := colorprofile.Detect(os.Stdout, os.Environ())
c := color.RGBA{0x6b, 0x50, 0xff, 0xff} // #6b50ff

// Downsample to the detected profile, when necessary.
convertedColor := p.Convert(c)

// Or manually convert to a given profile.
ansi256Color := colorprofile.ANSI256.Convert(c)
ansiColor := colorprofile.ANSI.Convert(c)
noColor := colorprofile.Ascii.Convert(c)
noANSI := colorprofile.NoTTY.Convert(c)
```

## Automatic downsampling with a Writer

You can also magically downsample colors in ANSI output, when necessary. If
output is not a TTY ANSI will be dropped entirely.

```go
myFancyANSI := "\x1b[38;2;107;80;255mCute \x1b[1;3mpuppy!!\x1b[m"

// Automatically downsample for the terminal at stdout.
w := colorprofile.NewWriter(os.Stdout, os.Environ())
fmt.Fprintf(w, myFancyANSI)

// Downsample to 4-bit ANSI.
w.Profile = colorprofile.ANSI
fmt.Fprintf(w, myFancyANSI)

// Ascii-fy, no colors.
w.Profile = colorprofile.Ascii
fmt.Fprintf(w, myFancyANSI)

// Strip ANSI altogether.
w.Profile = colorprofile.NoTTY
fmt.Fprintf(w, myFancyANSI) // not as fancy
```

## Contributing

See [contributing][contribute].

[contribute]: https://github.com/charmbracelet/colorprofile/contribute

## Feedback

We’d love to hear your thoughts on this project. Feel free to drop us a note!

- [Twitter](https://twitter.com/charmcli)
- [The Fediverse](https://mastodon.social/@charmcli)
- [Discord](https://charm.sh/chat)

## License

[MIT](https://github.com/charmbracelet/bubbletea/raw/master/LICENSE)

---

Part of [Charm](https://charm.sh).

<a href="https://charm.sh/"><img alt="The Charm logo" src="https://stuff.charm.sh/charm-badge.jpg" width="400"></a>

Charm热爱开源 • Charm loves open source • نحنُ نحب المصادر المفتوحة
