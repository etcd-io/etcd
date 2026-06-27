// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package internal contains functionality for x/vuln.
package internal

// IDDirectory is the name of the directory that contains entries
// listed by their IDs.
const IDDirectory = "ID"

// Pseudo-module paths used for parts of the Go system.
// These are technically not valid module paths, so we
// mustn't pass them to module.EscapePath.
// Keep in sync with vulndb/internal/database/generate.go.
const (
	// GoStdModulePath is the internal Go module path string used
	// when listing vulnerabilities in standard library.
	GoStdModulePath = "stdlib"

	// GoCmdModulePath is the internal Go module path string used
	// when listing vulnerabilities in the go command.
	GoCmdModulePath = "toolchain"

	// UnknownModulePath is a special module path for when we cannot work out
	// the module for a package.
	UnknownModulePath = "unknown-module"

	// UnknownPackagePath is a special package path for when we cannot work out
	// the packagUnknownModulePath = "unknown"
	UnknownPackagePath = "unknown-package"
)
