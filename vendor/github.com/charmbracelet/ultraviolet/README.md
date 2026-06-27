# Ultraviolet

<img width="400" alt="Charm Ultraviolet" src="https://github.com/user-attachments/assets/3484e4b0-3741-4e8c-bebf-9ea51f5bb49c" />

<p>
    <a href="https://pkg.go.dev/github.com/charmbracelet/ultraviolet?tab=doc"><img src="https://godoc.org/github.com/charmbracelet/ultraviolet?status.svg" alt="GoDoc"></a>
    <a href="https://github.com/charmbracelet/ultraviolet/actions"><img src="https://github.com/charmbracelet/ultraviolet/actions/workflows/build.yml/badge.svg" alt="Build Status"></a>
</p>

Ultraviolet is a set of primitives for manipulating terminal emulators, with a focus on terminal user interfaces (TUIs). It provides a set of tools and abstractions for interaction that can handle user input and display dynamic, cell-based content. It’s the product of many years of research, development, collaboration and ingenuity.

Ultraviolet is not a framework by design, however it can be used standalone to create powerful terminal applications. It’s in use in production and powers critical portions of [Bubble Tea v2][bbt] and [Lip Gloss v2][lg], and was instrumental in the development of [Crush][crush].

[crush]: https://github.com/charmbracelet/crush
[bbt]: https://github.com/charmbracelet/bubbletea
[lg]: https://github.com/charmbracelet/lipgloss

> [!CAUTION]
> This project currently exists to serve internal use cases. API stability is a goal, but expect no stability guarantees as of now.

## Features

Ultraviolet is built with several core features in mind to make terminal
application development easy and performant:

### 👺 The Cursed Renderer

The cell-based rendering model—called _The Cursed Render_—was inspired by the infamous
[ncurses](https://invisible-island.net/ncurses/) library, which has been an
essential part of terminal applications for decades. Ultraviolet takes this
concept and modernizes it for the Go programming language, providing a more
ergonomic and efficient way to work with terminal cells without the need for
archaic technologies like `terminfo` or `termcap` databases.

Unlike ncurses, it supports both full-window and inline use-cases as we see inline TUIs as important in maintaining user context and flow.

### 🏎️ High Speeds and Low Bandwidth

The built-in terminal renderer efficiently handles content updates by utilizing
a powerful cell-based diffing algorithm that minimizes the amount of data
written to the terminal using various ANSI escape sequences to accomplish this.
This allows applications to update only the parts of the terminal that have
changed, significantly improving performance and responsiveness.

In practical terms, Ultraviolet optimizes for fast redraws that use minimal data transfer. This is very important locally and critically important over the network (for example, via SSH).

### 💬 Universal Input

Input handling in terminals can be complex, especially when dealing with
multiple input sources, different platforms, and ancient terminal baggage.
Ultraviolet simplifies this by providing a unified interface for handling user
input, allowing developers to focus on building their applications without
getting bogged down in the intricacies of terminal input handling.

### 🎮 Cross-Platform Compatibility

Ultraviolet is designed to work seamlessly across different platforms and
terminal emulators. It abstracts away the differences in terminal capabilities
and provides a consistent API for developers to work with, ensuring that
applications built with Ultraviolet will run smoothly on various systems.

On Windows, it uses the [Windows Console API](https://learn.microsoft.com/en-us/windows/console/console-functions) to
provide a consistent experience, while on Unix-like systems, it relies on the
standard Termios API along with ANSI escape sequences to manipulate the
terminal.

In short: Ultraviolet provides first-class support for both Unix and Windows-based systems.

### 🧩 Extensible Architecture

Ultraviolet is built with extensibility in mind, providing a solid API that can
be embedded into other applications or used as a foundation for building custom
terminal user interfaces. It allows developers to create their own components,
styles, and behaviors, making it a versatile tool for building terminal
applications.

## FAQ

### 🐈 What about other Charm libraries?

Ultraviolet is not a replacement for existing libraries like [Bubble Tea](https://github.com/charmbracelet/bubbletea) or [Lip
Gloss](https://github.com/charmbracelet/lipgloss). Instead, it serves as a
foundation for the latest versions of both of these libraries and others like them, providing the
underlying primitives and abstractions needed to build terminal user interfaces
applications and frameworks.

### 🛁 How is it different from Bubble Tea?

Ultraviolet is a lower-level library that focuses on the core primitives of
terminal manipulation, rendering, and input handling. It provides the building
blocks for creating terminal applications, while Bubble Tea is a higher-level
framework that builds on top of Ultraviolet to provide a more structured and
opinionated way to build terminal user interfaces.

### 💋 Is it a replacement for Lip Gloss?

Simply put, no. Ultraviolet is not a replacement for Lip Gloss. Instead, it
provides the underlying rendering capabilities that Lip Gloss can use to create
styled terminal content. Lip Gloss is a higher-level library that builds on top
of Ultraviolet by utilizing the cell-based rendering model to provide a
simplified and ergonomic way to create styled terminal content and composition
of terminal user interfaces.

## ✏️ Tutorial

You can find a simple tutorial on how to create a UV application that displays
"Hello, World!" on the screen in the [TUTORIAL.md](./TUTORIAL.md) file.

## Whatcha think?

We’d love to hear your thoughts on this project. Feel free to drop us a note!

- [Twitter](https://twitter.com/charmcli)
- [Discord](https://charm.land/discord)
- [Slack](https://charm.land/slack)
- [The Fediverse](https://mastodon.social/@charmcli)

## License

[MIT](./LICENSE)

---

Part of [Charm](https://charm.land).

<a href="https://charm.sh/"><img alt="The Charm logo" width="400" src="https://stuff.charm.sh/charm-banner-next.jpg" /></a>

Charm热爱开源 • Charm loves open source • نحنُ نحب المصادر المفتوحة
