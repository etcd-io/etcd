// Copyright ©2020 The go-latex Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tex

import (
	"codeberg.org/go-latex/latex/font"
)

type State struct {
	be   font.Backend
	Font font.Font
	DPI  float64
}

func NewState(be font.Backend, font font.Font, dpi float64) State {
	return State{
		be:   be,
		Font: font,
		DPI:  dpi,
	}
}

func (state State) Backend() font.Backend { return state.be }
