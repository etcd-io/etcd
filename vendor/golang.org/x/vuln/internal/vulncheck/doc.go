// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package vulncheck detects uses of known vulnerabilities
in Go programs.

Vulncheck identifies vulnerability uses in Go programs
at the level of call graph, package import graph, and module
requires graph. For instance, vulncheck identifies which
vulnerable functions and methods are transitively called
from the program entry points. vulncheck also detects
transitively imported packages and required modules that
contain known vulnerable functions and methods.

We recommend using the command line tool [govulncheck] to
detect vulnerabilities in your code.

# Usage

The two main APIs of vulncheck, [Source] and [Binary], allow vulnerability
detection in Go source code and binaries, respectively.

[Source] accepts a list of [Package] objects, which
are a trimmed version of [golang.org/x/tools/go/packages.Package] objects to
reduce memory consumption. [Binary] accepts a path to a Go binary file.

Both [Source] and [Binary] require information about known
vulnerabilities in the form of a vulnerability database,
specifically a [golang.org/x/vuln/internal/client.Client].
The vulnerabilities
are modeled using the [golang.org/x/vuln/internal/osv] format.

# Results

The results of vulncheck are slices of the call graph, package imports graph,
and module requires graph leading to the use of an identified vulnerability.
The parts of these graphs not related to any vulnerabilities are omitted.

The [CallStacks] and [ImportChains] functions search the returned slices for
user-friendly representative call stacks and import chains. These call stacks
and import chains are provided as examples of vulnerability uses in the client
code.

# Limitations

There are some limitations with vulncheck. Please see the
[documented limitations] for more information.

[govulncheck]: https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck
[documented limitations]: https://go.dev/security/vulncheck#limitations.
*/
package vulncheck
