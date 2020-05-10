[![Build Status](https://travis-ci.org/uudashr/gopkgs.svg?branch=master)](https://travis-ci.org/uudashr/gopkgs)
[![GoDoc](https://godoc.org/github.com/uudashr/gopkgs?status.svg)](https://godoc.org/github.com/uudashr/gopkgs)
# gopkgs

Gopkgs is tools that provide list of available Go packages that can be imported.

This are alternative for `go list all`, it just faster.

## Installation

`$ go get -u github.com/uudashr/gopkgs/cmd/gopkgs`

## Usage
```
$ gopkgs -help
  -format string
    	custom output format (default "{{.ImportPath}}")
  -help
    	show this message
  -no-vendor
    	exclude vendor dependencies except under workDir (if specified)
  -workDir string
    	importable packages only for workDir


Use -format to custom the output using template syntax. The struct being passed to template is:
    type Pkg struct {
        Dir        string // directory containing package sources
        ImportPath string // import path of package in dir
        Name       string // package name
    }

Use -workDir={path} to speed up the package search. This will ignore any vendor package outside the workDir.
```

### Example
Get package name along with the import path.
```
$ gopkgs -format "{{.Name}};{{.ImportPath}}"
testing;github.com/mattes/migrate/source/testing
http;github.com/stretchr/testify/http
ql;github.com/mattes/migrate/database/ql
pkgtree;github.com/golang/dep/internal/gps/pkgtree
sqlite3;github.com/mattes/migrate/database/sqlite3
gps;github.com/golang/dep/internal/gps
spanner;github.com/mattes/migrate/database/spanner
dep;github.com/golang/dep
shortener;github.com/uudashr/shortener
bindata;github.com/mattes/migrate/source/go-bindata
postgres;github.com/mattes/migrate/database/postgres
test;github.com/vektra/mockery/mockery/fixtures
awss3;github.com/mattes/migrate/source/aws-s3
```

### Tips

Use `-workDir={path}` flag, it will speed up the package search by ignoring the exernal vendor.

## Related Project

This is based on https://github.com/haya14busa/gopkgs but taking slightly different path by simplifying its implementation.
