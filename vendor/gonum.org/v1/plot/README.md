# Gonum Plot

[![Build status](https://github.com/gonum/plot/workflows/CI/badge.svg)](https://github.com/gonum/plot/actions)
[![Build status](https://ci.appveyor.com/api/projects/status/6vtroet40gj5jhoe/branch/master?svg=true)](https://ci.appveyor.com/project/Gonum/plot/branch/master)
[![codecov.io](https://codecov.io/gh/gonum/plot/branch/master/graph/badge.svg)](https://codecov.io/gh/gonum/plot)
[![coveralls.io](https://coveralls.io/repos/gonum/plot/badge.svg?branch=master&service=github)](https://coveralls.io/github/gonum/plot?branch=master)
[![GoDoc](https://godoc.org/gonum.org/v1/plot?status.svg)](https://godoc.org/gonum.org/v1/plot)
[![go.dev reference](https://pkg.go.dev/badge/gonum.org/v1/plot)](https://pkg.go.dev/gonum.org/v1/plot)

`gonum/plot` is the new, official fork of code.google.com/p/plotinum.
It provides an API for building and drawing plots in Go.
*Note* that this new API is still in flux and may change.
See the wiki for some [example plots](http://github.com/gonum/plot/wiki/Example-plots).

For additional Plotters, see the [Community Plotters](https://github.com/gonum/plot/wiki/Community-Plotters) Wiki page.

There is a discussion list on Google Groups: gonum-dev@googlegroups.com.

`gonum/plot` is split into a few packages:

* The `plot` package provides simple interface for laying out a plot and provides primitives for drawing to it.
* The `plotter` package provides a standard set of `Plotter`s which use the primitives provided by the `plot` package for drawing lines, scatter plots, box plots, error bars, etc. to a plot. You do not need to use the `plotter` package to make use of `gonum/plot`, however: see the wiki for a tutorial on making your own custom plotters.
* The `plotutil` package contains a few routines that allow some common plot types to be made very easily. This package is quite new so it is not as well tested as the others and it is bound to change.
* The `vg` package provides a generic vector graphics API that sits on top of other vector graphics back-ends such as a custom EPS back-end, draw2d, SVGo, X-Window, gopdf, and [Gio](https://gioui.org).

## Documentation

Documentation is available at:

  https://godoc.org/gonum.org/v1/plot

## Installation

You can get `gonum/plot` using go get:

`go get gonum.org/v1/plot/...`

If you write a cool plotter that you think others may be interested in using, please post to the list so that we can link to it in the `gonum/plot` wiki or possibly integrate it into the `plotter` package.
