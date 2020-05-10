# gotests [![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](https://github.com/cweill/gotests/blob/master/LICENSE) [![godoc](https://img.shields.io/badge/go-documentation-blue.svg)](https://godoc.org/github.com/cweill/gotests) [![Build Status](https://travis-ci.org/cweill/gotests.svg?branch=master)](https://travis-ci.org/cweill/gotests) [![Coverage Status](https://coveralls.io/repos/github/cweill/gotests/badge.svg?branch=master)](https://coveralls.io/github/cweill/gotests?branch=master) [![codebeat badge](https://codebeat.co/badges/7ef052e3-35ff-4cab-88f9-e13393c8ab35)](https://codebeat.co/projects/github-com-cweill-gotests) [![Go Report Card](https://goreportcard.com/badge/github.com/cweill/gotests)](https://goreportcard.com/report/github.com/cweill/gotests)

`gotests` makes writing Go tests easy. It's a Golang commandline tool that generates [table driven tests](https://github.com/golang/go/wiki/TableDrivenTests) based on its target source files' function and method signatures. Any new dependencies in the test files are automatically imported.

## Demo

The following shows `gotests` in action using the [official Sublime Text 3 plugin](https://github.com/cweill/GoTests-Sublime). Plugins also exist for [Emacs](https://github.com/damienlevin/GoTests-Emacs), also [Emacs](https://github.com/s-kostyaev/go-gen-test), [Vim](https://github.com/buoto/gotests-vim), [Atom Editor](https://atom.io/packages/gotests), [Visual Studio Code](https://github.com/Microsoft/vscode-go), and [IntelliJ Goland](https://www.jetbrains.com/help/go/run-debug-configuration-for-go-test.html).

![demo](https://github.com/cweill/GoTests-Sublime/blob/master/gotests.gif)

## Installation

__Minimum Go version:__ Go 1.6

Use [`go get`](https://golang.org/cmd/go/#hdr-Download_and_install_packages_and_dependencies) to install and update:

```sh
$ go get -u github.com/cweill/gotests/...
```

## Usage

From the commandline, `gotests` can generate Go tests for specific source files or an entire directory. By default, it prints its output to `stdout`.

```sh
$ gotests [options] PATH ...
```

Available options:

```
  -all           generate go tests for all functions and methods

  -excl          regexp. generate go tests for functions and methods that don't
                 match. Takes precedence over -only, -exported, and -all

  -exported      generate go tests for exported functions and methods. Takes
                 precedence over -only and -all

  -i             print test inputs in error messages

  -only          regexp. generate go tests for functions and methods that match only.
                 Takes precedence over -all

  -w             write output to (test) files instead of stdout

  -nosubtests    disable subtest generation. Only available for Go 1.7+

  -template_dir  optional. Path to a directory containing custom test code templates
```

## Contributions

Contributing guidelines are in [CONTRIBUTING.md](CONTRIBUTING.md).

## License

`gotests` is released under the [Apache 2.0 License](http://www.apache.org/licenses/LICENSE-2.0).
