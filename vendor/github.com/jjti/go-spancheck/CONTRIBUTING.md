# Contributing guideline

Contributions are welcome + appreciated.

## Open Requests

These are a couple contributions I would especially appreciate:

1. Add check for SetAttributes: https://github.com/jjti/go-spancheck/issues/1
1. Add SuggestedFix(es): https://github.com/jjti/go-spancheck/issues/2

## Steps

### 1. Create an Issue

If one does not exist already, open a bug report or feature request in [https://github.com/jjti/go-spancheck/issues](https://github.com/jjti/go-spancheck/issues).

### 2. Add a test case

Test cases are in `/testdata`.

If fixing a bug, you can add it to `testdata/enableall/enable_all.go` (for example):

```go
func _() {
	ctx, span := otel.Tracer("foo").Start(context.Background(), "bar") // want "span.End is not called on all paths, possible memory leak"
	print(ctx.Done(), span.IsRecording())
} // want "return can be reached without calling span.End"
```

If adding a new feature with a new combination of flags, create a new module within `testdata`:

1. Create a new module, eg `testdata/setattributes`
1. Copy/paste go.mod/sum into the new module directory and update the module definition, eg `module github.com/jjti/go-spancheck/testdata/setattributes`
1. Add the module to the workspace in [go.work](./go.work)
1. Add the module's directory to the `testvendor` Make target in [Makefile](./Makefile)

### 3. Run tests

```bash
make test
```

### 4. Open a PR

Eg of a GitHub snippet for PRs:

```bash
alias gpr='gh pr view --web 2>/dev/null || gh pr create --web --fill'
gpr
```
