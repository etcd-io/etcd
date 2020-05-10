# honnef.co/go/tools

`honnef.co/go/tools/...` is a collection of tools and libraries for
working with Go code, including linters and static analysis, most
prominently staticcheck.

**These tools are financially supported by [private and corporate sponsors](http://staticcheck.io/sponsors) to ensure its continued development.
Please consider [becoming a sponsor](https://github.com/users/dominikh/sponsorship) if you or your company relies on the tools.**


## Documentation

You can find extensive documentation on these tools, in particular staticcheck, on [staticcheck.io](https://staticcheck.io/docs/).


## Installation

### Releases

It is recommended that you run released versions of the tools. These
releases can be found as git tags (e.g. `2019.1`) as well as prebuilt
binaries in the [releases tab](https://github.com/dominikh/go-tools/releases).

The easiest way of using the releases from source is to use a Go
package manager such as Godep or Go modules. Alternatively you can use
a combination of `git clone -b` and `go get` to check out the
appropriate tag and download its dependencies.


### Master

You can also run the master branch instead of a release. Note that
while the master branch is usually stable, it may still contain new
checks or backwards incompatible changes that break your build. By
using the master branch you agree to become a beta tester.

## Tools

All of the following tools can be found in the cmd/ directory. Each
tool is accompanied by its own README, describing it in more detail.

| Tool                                               | Description                                                             |
|----------------------------------------------------|-------------------------------------------------------------------------|
| [keyify](cmd/keyify/)                              | Transforms an unkeyed struct literal into a keyed one.                  |
| [rdeps](cmd/rdeps/)                                | Find all reverse dependencies of a set of packages                      |
| [staticcheck](cmd/staticcheck/)                    | Go static analysis, detecting bugs, performance issues, and much more. |
| [structlayout](cmd/structlayout/)                  | Displays the layout (field sizes and padding) of structs.               |
| [structlayout-optimize](cmd/structlayout-optimize) | Reorders struct fields to minimize the amount of padding.               |
| [structlayout-pretty](cmd/structlayout-pretty)     | Formats the output of structlayout with ASCII art.                      |

## Libraries

In addition to the aforementioned tools, this repository contains the
libraries necessary to implement these tools.

Unless otherwise noted, none of these libraries have stable APIs.
Their main purpose is to aid the implementation of the tools. If you
decide to use these libraries, please vendor them and expect regular
backwards-incompatible changes.

## System requirements

We support the last two versions of Go.

## Sponsors

This project is sponsored by the following companies

[<img src="images/sponsors/fastly.png" alt="Fastly" height="55"></img>](https://fastly.com)  
[<img src="images/sponsors/uber.png" alt="Uber" height="35"></img>](https://uber.com)

as well as many generous people.
