# gomodifytags 

Go tool to modify/update field tags in structs. `gomodifytags` makes it easy to
update, add or delete the tags in a struct field. You can easily add new tags,
update existing tags (such as appending a new key, i.e: `db`, `xml`, etc..) or
remove existing tags. It also allows you to add and remove tag options. It's
intended to be used by an editor, but also has modes to run it from the
terminal. Read the usage section below for more information.

![gomodifytags](https://user-images.githubusercontent.com/438920/32691304-a1c7e47c-c716-11e7-977c-f4d0f8c616be.gif)


# Install

```bash
go get github.com/fatih/gomodifytags
```

# Supported editors

* [vim-go](https://github.com/fatih/vim-go) with `:GoAddTags` and `:GoRemoveTags`
* [go-plus (atom)](https://github.com/joefitzgerald/go-plus) with commands `golang:add-tags` and `golang:remove-tags`
* [vscode-go](https://github.com/Microsoft/vscode-go) with commands `Go: Add Tags` and `Go: Remove Tags`
* [A (Acme)](https://github.com/davidrjenni/A) with commands `addtags` and `rmtags`
* [emacs-go-tag](https://github.com/brantou/emacs-go-tag) with commands `go-tag-add` and `go-tag-remove`

# Usage

`gomodifytags` has multiple ways to modify a tag. Let's start with an example package:

```go
package main

type Server struct {
	Name        string
	Port        int
	EnableLogs  bool
	BaseDomain  string
	Credentials struct {
		Username string
		Password string
	}
}
```

We have to first pass a file. For that we can use the `-file` flag:

```sh
$ gomodifytags -file demo.go
-line, -offset or -struct is not passed
```

What are these? There are three different ways of defining **which** field tags
to change:

* `-struct`: This accepts the struct name. i.e: `-struct Server`. The name
  should be a valid type name. The `-struct` flag selects the whole struct, and
  thus it will operate on all fields.
* `-offset`: This accepts a byte offset of the file. Useful for editors to pass
  the position under the cursor. i.e: `-offset 548`. The offset has to be
  inside a valid struct. The `-offset` selects the whole struct. If you need
  more granular option see `-line`
* `-line`: This accepts a string that defines the line or lines of which fields
  should be changed. I.e: `-line 4` or `-line 5,8`

Let's continue by using the `-struct` tag:

```
$ gomodifytags -file demo.go -struct Server
one of [-add-tags, -add-options, -remove-tags, -remove-options, -clear-tags, -clear-options] should be defined
```

## Adding tags & options

There are many options on how you can change the struct. Let us start by adding
tags. The following will add the `json` key to all fields. The value will be
automatically inherited from the field name and transformed to `snake_case`:

```
$ gomodifytags -file demo.go -struct Server -add-tags json
```
```go
package main

type Server struct {
	Name        string `json:"name"`
	Port        int    `json:"port"`
	EnableLogs  bool   `json:"enable_logs"`
	BaseDomain  string `json:"base_domain"`
	Credentials struct {
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"credentials"`
}
```

By default every change will be printed to stdout. So it's safe to run it and
see the results of it. If you want to change it permanently, pass the `-w`
(write) flag.

```
$ gomodifytags -file demo.go -struct Server -add-tags json -w
```

You can pass multiple keys to add tags. The following will add `json` and `xml`
keys:

```
$ gomodifytags -file demo.go -struct Server -add-tags json,xml
```
```go
package main

type Server struct {
	Name        string `json:"name" xml:"name"`
	Port        int    `json:"port" xml:"port"`
	EnableLogs  bool   `json:"enable_logs" xml:"enable_logs"`
	BaseDomain  string `json:"base_domain" xml:"base_domain"`
	Credentials struct {
		Username string `json:"username" xml:"username"`
		Password string `json:"password" xml:"password"`
	} `json:"credentials" xml:"credentials"`
}
```

If you prefer to use `camelCase` instead of `snake_case` for the values, you
can use the `-transform` flag to define a different transformation rule. The
following example uses the `camelcase` transformation rule:


```
$ gomodifytags -file demo.go -struct Server -add-tags json,xml -transform camelcase
```
```go
package main

type Server struct {
	Name        string `json:"name" xml:"name"`
	Port        int    `json:"port" xml:"port"`
	EnableLogs  bool   `json:"enableLogs" xml:"enableLogs"`
	BaseDomain  string `json:"baseDomain" xml:"baseDomain"`
	Credentials struct {
		Username string `json:"username" xml:"username"`
		Password string `json:"password" xml:"password"`
	} `json:"credentials" xml:"credentials"`
}
```

We currently support the following transformations:

* `snakecase`: `"BaseDomain"` -> `"base_domain"`
* `camelcase`: `"BaseDomain"` -> `"baseDomain"`
* `lispcase`:  `"BaseDomain"` -> `"base-domain"`
* `pascalcase`:  `"BaseDomain"` -> `"BaseDomain"`
* `keep`:  keeps the original field name

You can also pass a static value for each fields. This is useful if you use Go
packages that validates the struct fields or extract values for certain
operations. The following example adds the `json` key, a `validate` key with
the value set to `gt=1` and the `scope` key with the value `read-only`:

```
$ gomodifytags -file demo.go -struct Server -add-tags json,validate:gt=1,scope:read-only
```
```go
package main

type Server struct {
	Name        string `json:"name" validate:"gt=1" scope:"read-only"`
	Port        int    `json:"port" validate:"gt=1" scope:"read-only"`
	EnableLogs  bool   `json:"enable_logs" validate:"gt=1" scope:"read-only"`
	BaseDomain  string `json:"base_domain" validate:"gt=1" scope:"read-only"`
	Credentials struct {
		Username string `json:"username" validate:"gt=1" scope:"read-only"`
		Password string `json:"password" validate:"gt=1" scope:"read-only"`
	} `json:"credentials" validate:"gt=1" scope:"read-only"`
}
```

To add `options` to for a given key, we use the `-add-options` flag. In the
example below we're going to add the `json` key and the `omitempty` option to
all json keys:

```
$ gomodifytags -file demo.go -struct Server -add-tags json -add-options json=omitempty
```
```go
package main

type Server struct {
	Name        string `json:"name,omitempty"`
	Port        int    `json:"port,omitempty"`
	EnableLogs  bool   `json:"enable_logs,omitempty"`
	BaseDomain  string `json:"base_domain,omitempty"`
	Credentials struct {
		Username string `json:"username,omitempty"`
		Password string `json:"password,omitempty"`
	} `json:"credentials,omitempty"`
}
```

If the key already exists you don't have to use `-add-tags`


### Skipping unexported fields

By default all fields are processed. This main reason for this is to allow
structs to evolve with time and be ready in case a field is exported in the
future. However if you don't like this behavior, you can skip it by passing the
`--skip-unexported` flag:

```
$ gomodifytags -file demo.go -struct Server -add-tags json --skip-unexported
```
```go
package main

type Server struct {
        Name       string `json:"name"`
        Port       int    `json:"port"`
        enableLogs bool
        baseDomain string
}
```

## Removing tags & options

Let's continue with removing tags. We're going to use the following simple package:

```go
package main

type Server struct {
	Name        string `json:"name,omitempty" xml:"name,attr,cdata"`
	Port        int    `json:"port,omitempty" xml:"port,attr,cdata"`
	EnableLogs  bool   `json:"enable_logs,omitempty" xml:"enable_logs,attr,cdata"`
	BaseDomain  string `json:"base_domain,omitempty" xml:"base_domain,attr,cdata"`
	Credentials struct {
		Username string `json:"username,omitempty" xml:"username,attr,cdata"`
		Password string `json:"password,omitempty" xml:"password,attr,cdata"`
	} `json:"credentials,omitempty" xml:"credentials,attr,cdata"`
}
```

To remove the xml tags, we're going to use the `-remove-tags` flag:

```
$ gomodifytags -file demo.go -struct Server -remove-tags xml
```
```go
package main

type Server struct {
	Name        string `json:"name"`
	Port        int    `json:"port"`
	EnableLogs  bool   `json:"enable_logs"`
	BaseDomain  string `json:"base_domain"`
	Credentials struct {
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"credentials"`
}
```

You can also remove multiple tags. The example below removs `json` and `xml`:

```
$ gomodifytags -file demo.go -struct Server -remove-tags json,xml
```
```go
package main

type Server struct {
	Name        string
	Port        int
	EnableLogs  bool
	BaseDomain  string
	Credentials struct {
		Username string
		Password string
	}
}
```

If you want to remove all keys, we can also use the `-clear-tags` flag. This
flag removes all tags and doesn't require to explicitly pass the key names:

```
$ gomodifytags -file demo.go -struct Server -clear-tags
```
```go
package main

type Server struct {
	Name        string
	Port        int
	EnableLogs  bool
	BaseDomain  string
	Credentials struct {
		Username string
		Password string
	}
}
```

To remove any option, we can use the `-remove-options` flag. The following will
remove all `omitempty` flags from the `json` key:

```
$ gomodifytags -file demo.go -struct Server -remove-options json=omitempty
```
```go
package main

type Server struct {
	Name        string `json:"name" xml:"name,attr,cdata"`
	Port        int    `json:"port" xml:"port,attr,cdata"`
	EnableLogs  bool   `json:"enable_logs" xml:"enable_logs,attr,cdata"`
	BaseDomain  string `json:"base_domain" xml:"base_domain,attr,cdata"`
	Credentials struct {
		Username string `json:"username" xml:"username,attr,cdata"`
		Password string `json:"password" xml:"password,attr,cdata"`
	} `json:"credentials" xml:"credentials,attr,cdata"`
}
```

To remove multiple options from multiple tags just add another options:

```
$ gomodifytags -file demo.go -struct Server -remove-options json=omitempty,xml=cdata
```
```go
package main

type Server struct {
	Name        string `json:"name" xml:"name,attr"`
	Port        int    `json:"port" xml:"port,attr"`
	EnableLogs  bool   `json:"enable_logs" xml:"enable_logs,attr"`
	BaseDomain  string `json:"base_domain" xml:"base_domain,attr"`
	Credentials struct {
		Username string `json:"username" xml:"username,attr"`
		Password string `json:"password" xml:"password,attr"`
	} `json:"credentials" xml:"credentials,attr"`
}
```

Lastly, to remove all options without explicitly defining the keys and names,
we can use the `-clear-options` flag. The following example will remove all
options for the given struct:

```
$ gomodifytags -file demo.go -struct Server -clear-options
```
```go
package main

type Server struct {
	Name        string `json:"name" xml:"name"`
	Port        int    `json:"port" xml:"port"`
	EnableLogs  bool   `json:"enable_logs" xml:"enable_logs"`
	BaseDomain  string `json:"base_domain" xml:"base_domain"`
	Credentials struct {
		Username string `json:"username" xml:"username"`
		Password string `json:"password" xml:"password"`
	} `json:"credentials" xml:"credentials"`
}
```

## Line based modification

So far all examples used the `-struct` flag. However we also can pass the line
numbers to only change certain files. Suppose we only want to remove the tags
for the `Credentials` struct (including the fields) for the following code (lines are included):

```go
01  package main
02  
03  type Server struct {
04  	Name        string `json:"name" xml:"name"`
05  	Port        int    `json:"port" xml:"port"`
06  	EnableLogs  bool   `json:"enable_logs" xml:"enable_logs"`
07  	BaseDomain  string `json:"base_domain" xml:"base_domain"`
08  	Credentials struct {
09  		Username string `json:"username" xml:"username"`
10  		Password string `json:"password" xml:"password"`
11  	} `json:"credentials" xml:"credentials"`
12  }
```

To remove the tags for the credentials we're going to pass the `-line` flag:

```
$ gomodifytags -file demo.go -line 8,11 -clear-tags xml
```
```go
package main

type Server struct {
	Name        string `json:"name" xml:"name"`
	Port        int    `json:"port" xml:"port"`
	EnableLogs  bool   `json:"enable_logs" xml:"enable_logs"`
	BaseDomain  string `json:"base_domain" xml:"base_domain"`
	Credentials struct {
		Username string
		Password string
	}
}
```

For removing the xml tags for certain lines, we can use the `-remove-tags`
field. The following example will remove the `xml` tags for the lines 6 and 7
(fields with names of `EnableLogs` and `BaseDomain`):

```
$ gomodifytags -file demo.go -line 6,7 -remove-tags xml
```
```go
package main

type Server struct {
	Name        string `json:"name" xml:"name"`
	Port        int    `json:"port" xml:"port"`
	EnableLogs  bool   `json:"enable_logs"`
	BaseDomain  string `json:"base_domain"`
	Credentials struct {
		Username string `json:"username" xml:"username"`
		Password string `json:"password" xml:"password"`
	} `json:"credentials" xml:"credentials"`
}
```

The same logic applies to adding tags or any other option as well. To add the
`bson` tag to the lines between 5 and 7, we can use the following example:

```
$ gomodifytags -file demo.go -line 5,7 -add-tags bson
```
```go
package main

type Server struct {
	Name        string `json:"name" xml:"name"`
	Port        int    `json:"port" xml:"port" bson:"port"`
	EnableLogs  bool   `json:"enable_logs" xml:"enable_logs" bson:"enable_logs"`
	BaseDomain  string `json:"base_domain" xml:"base_domain" bson:"base_domain"`
	Credentials struct {
		Username string `json:"username" xml:"username"`
		Password string `json:"password" xml:"password"`
	} `json:"credentials" xml:"credentials"`
}
```

## Editor integration

Editors can use the tool by calling the tool and then either replace the buffer
with the stdout or use the `-w` flag.

Also `-line` and `-offset` flags should be preferred to be used with editors.
An editor can select a range of lines and then pass it to `-line` flag. The
editor also can pass the offset under the cursor if it's inside the struct to
`-offset`

Editors also can use the `-format` flag to output a json output with the
changed lines. This is useful if you want to explicitly replace the buffer with
the given lines. For the file below:


```go
package main

type Server struct {
	Name        string
	Port        int
	EnableLogs  bool
	BaseDomain  string
	Credentials struct {
		Username string
		Password string
	}
}
```

If we add the `xml` tag and tell to output the format in json  with the
`-format` flag, the following will be outputed:

```
$ gomodifytags -file demo.go -struct Server -add-tags xml -format json
```
```json
{
  "start": 3,
  "end": 12,
  "lines": [
    "type Server struct {",
    "\tName        string `xml:\"name\"`",
    "\tPort        int    `xml:\"port\"`",
    "\tEnableLogs  bool   `xml:\"enable_logs\"`",
    "\tBaseDomain  string `xml:\"base_domain\"`",
    "\tCredentials struct {",
    "\t\tUsername string `xml:\"username\"`",
    "\t\tPassword string `xml:\"password\"`",
    "\t} `xml:\"credentials\"`",
    "}"
  ]
}
```

The output is defined with the following Go struct:

```go
type output struct {
	Start int      `json:"start"`
	End   int      `json:"end"`
	Lines []string `json:"lines"`
}
```

The `start` and `end` specifices the positions in the file the `lines` will
apply.  With this information, you can replace the editor buffer by iterating
over the `lines` and set it for the given range. An example how it's done in
vim-go in Vimscript is:

```viml
let index = 0
for line_number in range(start, end)
  call setline(line_number, lines[index])
  let index += 1
endfor
```

### Unsaved files

Editors can supply `gomodifytags` with the contents of unsaved buffers by
using the `-modified` flag and writing an archive to stdin.  
Files in the archive will be preferred over those on disk.

Each archive entry consists of:
 - the file name, followed by a newline
 - the (decimal) file size, followed by a newline
 - the contents of the file

# Development

At least Go `v1.11.x` is required. Older versions might work, but it's not
recommended.

`gomodifytags` uses [Go modules](https://github.com/golang/go/wiki/Modules) for
dependency management. This means that you don't have to `go get` it into a
GOPATH anymore. Checkout the repository:

```
git clone https://github.com/fatih/gomodifytags.git
```

Start developing the code. To build a binary, execute:

```
GO111MODULE=on go build -mod=vendor
```

This will create a `gomodifytags` binary in the current directory. To test the
package, run the following:

```
GO111MODULE=on go test -v -mod=vendor
```

If everything works fine, feel free to open a pull request with your changes.
