// Copyright 2017-2018 Auburn University and others. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The godoctor command refactors Go code.
package main

import (
	"fmt"
	"os"

	"github.com/godoctor/godoctor/engine"
	"github.com/godoctor/godoctor/engine/cli"
)

// Name of the refactoring tool (Go Doctor).  This can be overridden using:
// go build -ldflags "-X main.name 'Go Doctor'" github.com/godoctor/godoctor
var name string = "Go Doctor"

// Go Doctor version number.  This can be overridden using:
// go build -ldflags "-X main.version 0.6" github.com/godoctor/godoctor
var version string = "0.6 (Beta)"

func main() {
	aboutText := fmt.Sprintf("%s %s", name, version)
	engine.AddDefaultRefactorings()
	os.Exit(cli.Run(aboutText, os.Stdin, os.Stdout, os.Stderr, os.Args))
}
