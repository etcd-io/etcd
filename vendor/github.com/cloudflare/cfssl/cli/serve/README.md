## Compiling and serving static files using esc

```
go install github.com/mjibson/esc

# Compile changes to static files 
esc -pkg serve -prefix cli/serve/static cli/serve/static > cli/serve/static.go

# Build and run CFSSL
go build ./cmd/cfssl/...
./cfssl serve
```
