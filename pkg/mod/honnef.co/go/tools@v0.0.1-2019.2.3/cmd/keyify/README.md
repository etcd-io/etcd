Keyify turns unkeyed struct literals (`T{1, 2, 3}`) into keyed
ones (`T{A: 1, B: 2, C: 3}`)

## Installation

See [the main README](https://github.com/dominikh/go-tools#installation) for installation instructions.

## Usage

Call keyify with a position such as `/some/file.go:#5`, where #5 is
the byte offset in the file and has to point at or into a struct
literal.

By default, keyify will print the new literal on stdout, formatted as
Go code. By using the `-json` flag, it will print a JSON object
denoting the start and end of the original literal, and its
replacement. This is useful for integration with editors.

For a description of all available flags, see `keyify -help`.

### Emacs

For Emacs integration, add the following to your `.emacs` file:

```
(add-to-list 'load-path "/your/gopath/src/honnef.co/go/tools/cmd/keyify)
(eval-after-load 'go-mode
  (lambda ()
    (require 'go-keyify)))
```

With point on or in a struct literal, call the `go-keyify` command.

![gif of keyify](http://stuff.fork-bomb.org/keyify.gif)
