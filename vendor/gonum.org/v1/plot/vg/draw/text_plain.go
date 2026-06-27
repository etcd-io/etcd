// Copyright ©2020 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package draw // import "gonum.org/v1/plot/vg/draw"

import "gonum.org/v1/plot/text"

// PlainTextHandler is a text/plain handler.
type PlainTextHandler = text.Plain

var _ text.Handler = (*PlainTextHandler)(nil)
