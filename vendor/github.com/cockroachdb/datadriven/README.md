# Data-Driven Tests for Go

This repository implements an extension of [Table-Driven Testing]. Instead of
building and iterating over a table in the test code, the input is further
separated into files (or inline strings). For certain classes of tests, this
can significantly reduce the friction involved in writing and reading these
tests.

[Table-Driven Testing]: https://github.com/golang/go/wiki/TableDrivenTests
