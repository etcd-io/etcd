# godirwalk

`godirwalk` is a library for traversing a directory tree on a file
system.

[![GoDoc](https://godoc.org/github.com/karrick/godirwalk?status.svg)](https://godoc.org/github.com/karrick/godirwalk) [![Build Status](https://dev.azure.com/microsoft0235/microsoft/_apis/build/status/karrick.godirwalk?branchName=master)](https://dev.azure.com/microsoft0235/microsoft/_build/latest?definitionId=1&branchName=master)

In short, why do I use this library?

1. It's faster than `filepath.Walk`.
1. It's more correct on Windows than `filepath.Walk`.
1. It's more easy to use than `filepath.Walk`.
1. It's more flexible than `filepath.Walk`.

## Usage Example

Additional examples are provided in the `examples/` subdirectory.

This library will normalize the provided top level directory name
based on the os-specific path separator by calling `filepath.Clean` on
its first argument. However it always provides the pathname created by
using the correct os-specific path separator when invoking the
provided callback function.

```Go
    dirname := "some/directory/root"
    err := godirwalk.Walk(dirname, &godirwalk.Options{
        Callback: func(osPathname string, de *godirwalk.Dirent) error {
            fmt.Printf("%s %s\n", de.ModeType(), osPathname)
            return nil
        },
        Unsorted: true, // (optional) set true for faster yet non-deterministic enumeration (see godoc)
    })
```

This library not only provides functions for traversing a file system
directory tree, but also for obtaining a list of immediate descendants
of a particular directory, typically much more quickly than using
`os.ReadDir` or `os.ReadDirnames`.

## Description

Here's why I use `godirwalk` in preference to `filepath.Walk`,
`os.ReadDir`, and `os.ReadDirnames`.

### It's faster than `filepath.Walk`

When compared against `filepath.Walk` in benchmarks, it has been
observed to run between five and ten times the speed on darwin, at
speeds comparable to the that of the unix `find` utility; about twice
the speed on linux; and about four times the speed on Windows.

How does it obtain this performance boost? It does less work to give
you nearly the same output. This library calls the same `syscall`
functions to do the work, but it makes fewer calls, does not throw
away information that it might need, and creates less memory churn
along the way by reusing the same scratch buffer for reading from a
directory rather than reallocating a new buffer every time it reads
file system entry data from the operating system.

While traversing a file system directory tree, `filepath.Walk` obtains
the list of immediate descendants of a directory, and throws away the
file system node type information provided by the operating system
that comes with the node's name. Then, immediately prior to invoking
the callback function, `filepath.Walk` invokes `os.Stat` for each
node, and passes the returned `os.FileInfo` information to the
callback.

While the `os.FileInfo` information provided by `os.Stat` is extremely
helpful--and even includes the `os.FileMode` data--providing it
requires an additional system call for each node.

Because most callbacks only care about what the node type is, this
library does not throw the type information away, but rather provides
that information to the callback function in the form of a
`os.FileMode` value. Note that the provided `os.FileMode` value that
this library provides only has the node type information, and does not
have the permission bits, sticky bits, or other information from the
file's mode. If the callback does care about a particular node's
entire `os.FileInfo` data structure, the callback can easiy invoke
`os.Stat` when needed, and only when needed.

#### Benchmarks

##### macOS

```Bash
$ go test -bench=. -benchmem
goos: darwin
goarch: amd64
pkg: github.com/karrick/godirwalk
BenchmarkReadDirnamesStandardLibrary-12   50000       26250  ns/op       10360  B/op       16  allocs/op
BenchmarkReadDirnamesThisLibrary-12       50000       24372  ns/op        5064  B/op       20  allocs/op
BenchmarkFilepathWalk-12                      1  1099524875  ns/op   228415912  B/op   416952  allocs/op
BenchmarkGodirwalk-12                         2   526754589  ns/op   103110464  B/op   451442  allocs/op
BenchmarkGodirwalkUnsorted-12                 3   509219296  ns/op   100751400  B/op   378800  allocs/op
BenchmarkFlameGraphFilepathWalk-12            1  7478618820  ns/op  2284138176  B/op  4169453  allocs/op
BenchmarkFlameGraphGodirwalk-12               1  4977264058  ns/op  1031105328  B/op  4514423  allocs/op
PASS
ok  	github.com/karrick/godirwalk	21.219s
```

##### Linux

```Bash
$ go test -bench=. -benchmem
goos: linux
goarch: amd64
pkg: github.com/karrick/godirwalk
BenchmarkReadDirnamesStandardLibrary-12  100000       15458  ns/op       10360  B/op       16  allocs/op
BenchmarkReadDirnamesThisLibrary-12      100000       14646  ns/op        5064  B/op       20  allocs/op
BenchmarkFilepathWalk-12                      2   631034745  ns/op   228210216  B/op   416939  allocs/op
BenchmarkGodirwalk-12                         3   358714883  ns/op   102988664  B/op   451437  allocs/op
BenchmarkGodirwalkUnsorted-12                 3   355363915  ns/op   100629234  B/op   378796  allocs/op
BenchmarkFlameGraphFilepathWalk-12            1  6086913991  ns/op  2282104720  B/op  4169417  allocs/op
BenchmarkFlameGraphGodirwalk-12               1  3456398824  ns/op  1029886400  B/op  4514373  allocs/op
PASS
ok      github.com/karrick/godirwalk    19.179s
```

### It's more correct on Windows than `filepath.Walk`

I did not previously care about this either, but humor me. We all love
how we can write once and run everywhere. It is essential for the
language's adoption, growth, and success, that the software we create
can run unmodified on all architectures and operating systems
supported by Go.

When the traversed file system has a logical loop caused by symbolic
links to directories, on unix `filepath.Walk` ignores symbolic links
and traverses the entire directory tree without error. On Windows
however, `filepath.Walk` will continue following directory symbolic
links, even though it is not supposed to, eventually causing
`filepath.Walk` to terminate early and return an error when the
pathname gets too long from concatenating endless loops of symbolic
links onto the pathname. This error comes from Windows, passes through
`filepath.Walk`, and to the upstream client running `filepath.Walk`.

The takeaway is that behavior is different based on which platform
`filepath.Walk` is running. While this is clearly not intentional,
until it is fixed in the standard library, it presents a compatibility
problem.

This library correctly identifies symbolic links that point to
directories and will only follow them when `FollowSymbolicLinks` is
set to true. Behavior on Windows and other operating systems is
identical.

### It's more easy to use than `filepath.Walk`

Since this library does not invoke `os.Stat` on every file system node
it encounters, there is no possible error event for the callback
function to filter on. The third argument in the `filepath.WalkFunc`
function signature to pass the error from `os.Stat` to the callback
function is no longer necessary, and thus eliminated from signature of
the callback function from this library.

Also, `filepath.Walk` invokes the callback function with a solidus
delimited pathname regardless of the os-specific path separator. This
library invokes the callback function with the os-specific pathname
separator, obviating a call to `filepath.Clean` in the callback
function for each node prior to actually using the provided pathname.

In other words, even on Windows, `filepath.Walk` will invoke the
callback with `some/path/to/foo.txt`, requiring well written clients
to perform pathname normalization for every file prior to working with
the specified file. In truth, many clients developed on unix and not
tested on Windows neglect this subtlety, and will result in software
bugs when running on Windows. This library would invoke the callback
function with `some\path\to\foo.txt` for the same file when running on
Windows, eliminating the need to normalize the pathname by the client,
and lessen the likelyhood that a client will work on unix but not on
Windows.

### It's more flexible than `filepath.Walk`

#### Configurable Handling of Symbolic Links

The default behavior of this library is to ignore symbolic links to
directories when walking a directory tree, just like `filepath.Walk`
does. However, it does invoke the callback function with each node it
finds, including symbolic links. If a particular use case exists to
follow symbolic links when traversing a directory tree, this library
can be invoked in manner to do so, by setting the
`FollowSymbolicLinks` parameter to true.

#### Configurable Sorting of Directory Children

The default behavior of this library is to always sort the immediate
descendants of a directory prior to visiting each node, just like
`filepath.Walk` does. This is usually the desired behavior. However,
this does come at slight performance and memory penalties required to
sort the names when a directory node has many entries. Additionally if
caller specifies `Unsorted` enumeration, reading directories is lazily
performed as the caller consumes entries. If a particular use case
exists that does not require sorting the directory's immediate
descendants prior to visiting its nodes, this library will skip the
sorting step when the `Unsorted` parameter is set to true.

Here's an interesting read of the potential hazzards of traversing a
file system hierarchy in a non-deterministic order. If you know the
problem you are solving is not affected by the order files are
visited, then I encourage you to use `Unsorted`. Otherwise skip
setting this option.

[Researchers find bug in Python script may have affected hundreds of studies](https://arstechnica.com/information-technology/2019/10/chemists-discover-cross-platform-python-scripts-not-so-cross-platform/)

#### Configurable Post Children Callback

This library provides upstream code with the ability to specify a
callback to be invoked for each directory after its children are
processed. This has been used to recursively delete empty directories
after traversing the file system in a more efficient manner. See the
`examples/clean-empties` directory for an example of this usage.

#### Configurable Error Callback

This library provides upstream code with the ability to specify a
callback to be invoked for errors that the operating system returns,
allowing the upstream code to determine the next course of action to
take, whether to halt walking the hierarchy, as it would do were no
error callback provided, or skip the node that caused the error. See
the `examples/walk-fast` directory for an example of this usage.
