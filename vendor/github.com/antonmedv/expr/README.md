# Expr 
[![Build Status](https://travis-ci.org/antonmedv/expr.svg?branch=master)](https://travis-ci.org/antonmedv/expr) 
[![Go Report Card](https://goreportcard.com/badge/github.com/antonmedv/expr)](https://goreportcard.com/report/github.com/antonmedv/expr) 
[![GoDoc](https://godoc.org/github.com/antonmedv/expr?status.svg)](https://godoc.org/github.com/antonmedv/expr)

<img src="docs/images/logo-small.png" width="150" alt="expr logo" align="right">

**Expr** package provides an engine that can compile and evaluate expressions. 
An expression is a one-liner that returns a value (mostly, but not limited to, booleans).
It is designed for simplicity, speed and safety.

The purpose of the package is to allow users to use expressions inside configuration for more complex logic. 
It is a perfect candidate for the foundation of a _business rule engine_. 
The idea is to let configure things in a dynamic way without recompile of a program:

```coffeescript
# Get the special price if
user.Group in ["good_customers", "collaborator"]

# Promote article to the homepage when
len(article.Comments) > 100 and article.Category not in ["misc"]

# Send an alert when
product.Stock < 15
```

## Features

* Seamless integration with Go (no need to redefine types)
* Static typing ([example](https://godoc.org/github.com/antonmedv/expr#example-Env)).
  ```go
  out, err := expr.Compile(`name + age`)
  // err: invalid operation + (mismatched types string and int)
  // | name + age
  // | .....^
  ```
* User-friendly error messages.
* Reasonable set of basic operators.
* Builtins `all`, `none`, `any`, `one`, `filter`, `map`.
  ```coffeescript
  all(Tweets, {.Size <= 280})
  ```
* Fast ([benchmarks](https://github.com/antonmedv/golang-expression-evaluation-comparison#readme)): uses bytecode virtual machine and optimizing compiler.

## Install

```
go get github.com/antonmedv/expr
```

## Documentation

* See [Getting Started](docs/Getting-Started.md) page for developer documentation.
* See [Language Definition](docs/Language-Definition.md) page to learn the syntax.

## Expr Code Editor

<a href="http://bit.ly/expr-code-editor">
	<img src="https://antonmedv.github.io/expr/ogimage.png" align="center" alt="Expr Code Editor" width="1200">
</a>

Also, I have an embeddable code editor written in JavaScript which allows editing expressions with syntax highlighting and autocomplete based on your types declaration.

[Learn more →](https://antonmedv.github.io/expr/)

## Examples

[Play Online](https://play.golang.org/p/z7T8ytJ1T1d)

```go
package main

import (
	"fmt"
	"github.com/antonmedv/expr"
)

func main() {
	env := map[string]interface{}{
		"greet":   "Hello, %v!",
		"names":   []string{"world", "you"},
		"sprintf": fmt.Sprintf,
	}

	code := `sprintf(greet, names[0])`

	program, err := expr.Compile(code, expr.Env(env))
	if err != nil {
		panic(err)
	}

	output, err := expr.Run(program, env)
	if err != nil {
		panic(err)
	}

	fmt.Println(output)
}
```

[Play Online](https://play.golang.org/p/4S4brsIvU4i)

```go
package main

import (
	"fmt"
	"github.com/antonmedv/expr"
)

type Tweet struct {
	Len int
}

type Env struct {
	Tweets []Tweet
}

func main() {
	code := `all(Tweets, {.Len <= 240})`

	program, err := expr.Compile(code, expr.Env(Env{}))
	if err != nil {
		panic(err)
	}

	env := Env{
		Tweets: []Tweet{{42}, {98}, {69}},
	}
	output, err := expr.Run(program, env)
	if err != nil {
		panic(err)
	}

	fmt.Println(output)
}
```

## Contributing

**Expr** consist of a few packages for parsing source code to AST, type checking AST, compiling to bytecode and VM for running bytecode program.

Also expr provides powerful tool [exe](cmd/exe) for debugging. It has interactive terminal debugger for our bytecode virtual machine.

<p align="center">
    <img src="docs/images/debug.gif" alt="debugger" width="605">
</p>
    

## Who is using Expr?

<table>
	<tr>
		<td>
			<p align="center"><a href="https://aviasales.ru"><img alt="Aviasales Logo" height="100" src="https://cdn.worldvectorlogo.com/logos/aviasales-4.svg"></a></p>
			<p><a href="https://aviasales.ru">Aviasales</a> are actively using Expr for different parts of the search engine.</p>
		</td>
		<td>
			<p align="center"><a href="https://www.mysteryminds.com/en/"><img alt="Mystery Minds Logo" height="100" src="https://www.mysteryminds.com/wp-content/uploads/2018/07/MM_logo_colours.png"></a></p>
			<p><a href="https://www.mysteryminds.com/en/">Mystery Minds</a> uses Expr to allow easy yet powerful customization of its matching algorithm.</p>
		</td>
	</tr>
</table>

[Add you company too](https://github.com/antonmedv/expr/edit/master/README.md)

## License

[MIT](LICENSE)
