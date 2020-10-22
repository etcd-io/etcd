// Copyright 2018 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package pprofui

import "io"

// Storage exposes the methods for storing and accessing profiles.
type Storage interface {
	// ID generates a unique ID for use in Store.
	ID() string
	// Store invokes the passed-in closure with a writer that stores its input.
	Store(id string, write func(io.Writer) error) error
	// Get invokes the passed-in closure with a reader for the data at the given id.
	// An error is returned when no data is found.
	Get(id string, read func(io.Reader) error) error
}
