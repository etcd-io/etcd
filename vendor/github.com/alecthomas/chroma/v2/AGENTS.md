Chroma is a syntax highlighting library, tool and web playground for Go. It is based on Pygments and includes importers for it, so most of the same concepts from Pygments apply to Chroma.

This project is written in Go, uses Hermit to manage tooling, and Just for helper commands. Helper scripts are in ./scripts.

Language definitions are XML files defined in ./lexers/embedded/*.xml.

Styles/themes are defined in ./styles/*.xml.

The CLI can be run with `chroma`.

The web playground can be run with `chromad --csrf-key=moo`. It blocks, so should generally be run in the background. It also does not hot reload, so has to be manually restarted. The playground has two modes - for local development it uses the server itself to render, while for production running `just chromad` will compile ./cmd/libchromawasm into a WASM module that is bundled into `chromad`.
