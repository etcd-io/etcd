# copyloopvar

copyloopvar is a linter detects places where loop variables are copied.

cf. [Fixing For Loops in Go 1.22](https://go.dev/blog/loopvar-preview)

## Example

```go
for i, v := range []int{1, 2, 3} {
    i := i // The copy of the 'for' variable "i" can be deleted (Go 1.22+)
    v := v // The copy of the 'for' variable "v" can be deleted (Go 1.22+)
    _, _ = i, v
}

for i := 1; i <= 3; i++ {
    i := i // The copy of the 'for' variable "i" can be deleted (Go 1.22+)
    _ = i
}
```

## Install

```bash
go install github.com/karamaru-alpha/copyloopvar/cmd/copyloopvar@latest
go vet -vettool=`which copyloopvar` ./...
```
