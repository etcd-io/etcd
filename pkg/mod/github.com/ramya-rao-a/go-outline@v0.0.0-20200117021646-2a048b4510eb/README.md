# Go Outline

Simple utility for extracting a JSON representation of the declarations in a 
Go source file.

## Installing

```bash
go get -u github.com/ramya-rao-a/go-outline
```

## Using
```bash
> go-outline -f file.go
[{"label":"proc","type":"package",<...>}]
```

To parse and return only imports
```bash
> go-outline -f file.go -imports-only
```

To parse unsaved file contents, use the `-modified` flag along with the `-f` flag and write an archive to stdin.  
File in the archive will be preferred over the one on disk.

The archive entry consists of:
 - the file name, followed by a newline
 - the (decimal) file size, followed by a newline
 - the contents of the file

### Schema
```go
type Declaration struct {
	Label        string        `json:"label"`
	Type         string        `json:"type"`
	ReceiverType string        `json:"receiverType,omitempty"`
	Start        token.Pos     `json:"start"`
	End          token.Pos     `json:"end"`
	Children     []Declaration `json:"children,omitempty"`
}
```
