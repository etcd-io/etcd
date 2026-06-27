# Fork of swaggo/swag

This is a hard fork of [swaggo/swag](https://github.com/swaggo/swag) to be usable as a library.

I considered other options before deciding to fork, but there are no straightforward or non-invasive changes.

Issues should be open either on the original [swaggo/swag repository](https://github.com/swaggo/swag) or on [golangci-lint repository](https://github.com/golangci/golangci-lint).

**No modifications will be accepted other than the synchronization of the fork.**

The synchronization of the fork will be done by the golangci-lint maintainers only.

## Modifications

- All the files have been removed except:
  - `formatter.go` (the unused field `debug Debugger` is removed)
  - `formatter_test.go`
  - `parser.go` (only the constants are kept.)
  - `license`
  - `.gitignore`
- The module name has been changed to `github.com/golangci/swaggoswag` to avoid replacement directives inside golangci-lint.
  - The package name has been changed from `swag` to `swaggoswag`.

## History

- sync with 93e86851e9f22f1f2db57812cf71fc004c02159c (after v1.16.4)
