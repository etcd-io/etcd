# structlayout

The _structlayout_ utility prints the layout of a struct – that is the
byte offset and size of each field, respecting alignment/padding.

The information is printed in human-readable form by default, but can
be emitted as JSON with the `-json` flag. This makes it easy to
consume this information in other tools.

A utility called _structlayout-pretty_ takes this JSON and prints an
ASCII graphic representing the memory layout.

_structlayout-optimize_ is another tool. Inspired by
[maligned](https://github.com/mdempsky/maligned), it reads
_structlayout_ JSON on stdin and reorders fields to minimize the
amount of padding. The tool can itself emit JSON and feed into e.g.
_structlayout-pretty_.

_structlayout-svg_ is a third-party tool that, similarly to
_structlayout-pretty_, visualises struct layouts. It does so by
generating a fancy-looking SVG graphic. You can install it via

```
go get github.com/ajstarks/svgo/structlayout-svg
```

## Installation

See [the main README](https://github.com/dominikh/go-tools#installation) for installation instructions.

## Examples

```
$ structlayout bufio Reader
Reader.buf []byte: 0-24 (24 bytes)
Reader.rd io.Reader: 24-40 (16 bytes)
Reader.r int: 40-48 (8 bytes)
Reader.w int: 48-56 (8 bytes)
Reader.err error: 56-72 (16 bytes)
Reader.lastByte int: 72-80 (8 bytes)
Reader.lastRuneSize int: 80-88 (8 bytes)
```

```
$ structlayout -json bufio Reader | jq .
[
  {
    "name": "Reader.buf",
    "type": "[]byte",
    "start": 0,
    "end": 24,
    "size": 24,
    "is_padding": false
  },
  {
    "name": "Reader.rd",
    "type": "io.Reader",
    "start": 24,
    "end": 40,
    "size": 16,
    "is_padding": false
  },
  {
    "name": "Reader.r",
    "type": "int",
    "start": 40,
    "end": 48,
    "size": 8,
    "is_padding": false
  },
...
```

```
$ structlayout -json bufio Reader | structlayout-pretty 
    +--------+
  0 |        | <- Reader.buf []byte
    +--------+
    -........-
    +--------+
 23 |        |
    +--------+
 24 |        | <- Reader.rd io.Reader
    +--------+
    -........-
    +--------+
 39 |        |
    +--------+
 40 |        | <- Reader.r int
    +--------+
    -........-
    +--------+
 47 |        |
    +--------+
 48 |        | <- Reader.w int
    +--------+
    -........-
    +--------+
 55 |        |
    +--------+
 56 |        | <- Reader.err error
    +--------+
    -........-
    +--------+
 71 |        |
    +--------+
 72 |        | <- Reader.lastByte int
    +--------+
    -........-
    +--------+
 79 |        |
    +--------+
 80 |        | <- Reader.lastRuneSize int
    +--------+
    -........-
    +--------+
 87 |        |
    +--------+
```

```
$ structlayout -json bytes Buffer | structlayout-svg -t "bytes.Buffer" > /tmp/struct.svg
```

![memory layout of bytes.Buffer](/images/screenshots/struct.png)
