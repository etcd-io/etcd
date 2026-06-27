# CancelReader

[![Latest Release](https://img.shields.io/github/release/muesli/cancelreader.svg?style=for-the-badge)](https://github.com/muesli/cancelreader/releases)
[![Go Doc](https://img.shields.io/badge/godoc-reference-blue.svg?style=for-the-badge)](https://pkg.go.dev/github.com/muesli/cancelreader)
[![Software License](https://img.shields.io/badge/license-MIT-blue.svg?style=for-the-badge)](/LICENSE)
[![Build Status](https://img.shields.io/github/workflow/status/muesli/cancelreader/build?style=for-the-badge)](https://github.com/muesli/cancelreader/actions)
[![Go ReportCard](https://goreportcard.com/badge/github.com/muesli/cancelreader?style=for-the-badge)](https://goreportcard.com/report/muesli/cancelreader)

A cancelable reader for Go

This package is based on the fantastic work of [Erik Geiser](https://github.com/erikgeiser)
in Charm's [Bubble Tea](https://github.com/charmbracelet/bubbletea) framework.

## Usage

`NewReader` returns a reader with a `Cancel` function. If the input reader is a
`File`, the cancel function can be used to interrupt a blocking `Read` call.
In this case, the cancel function returns true if the call was canceled
successfully. If the input reader is not a `File`, the cancel function does
nothing and always returns false.

```go
r, err := cancelreader.NewReader(file)
if err != nil {
    // handle error
    ...
}

// cancel after five seconds
go func() {
    time.Sleep(5 * time.Second)
    r.Cancel()
}()

// keep reading
for {
    var buf [1024]byte
    _, err := r.Read(buf[:])

    if errors.Is(err, cancelreader.ErrCanceled) {
        fmt.Println("canceled!")
        break
    }
    if err != nil {
        // handle other errors
        ...
    }

    // handle data
    ...
}
```

## Implementations

- The Linux implementation is based on the epoll mechanism
- The BSD and macOS implementation is based on the kqueue mechanism
- The generic Unix implementation is based on the posix select syscall

## Caution

The Windows implementation is based on WaitForMultipleObject with overlapping
reads from CONIN$. At this point it only supports canceling reads from
`os.Stdin`.
