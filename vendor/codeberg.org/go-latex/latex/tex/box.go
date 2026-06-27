// Copyright ©2020 The go-latex Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tex

import (
	"fmt"
	"log"
	"math"

	"codeberg.org/go-latex/latex/font"
)

const (
	// How much text shrinks when going to the next-smallest level.  growFactor
	// must be the inverse of shrinkFactor.
	shrinkFactor = 0.7
	growFactor   = 1.0 / shrinkFactor

	// The number of different sizes of chars to use, beyond which they will not
	// get any smaller
	numSizeLevels = 6
)

// FontConstants is a set of magical values that control how certain things,
// such as sub- and superscripts are laid out.
// These are all metrics that can't be reliably retrieved from the font metrics
// in the font itself.
type FontConstants struct {
	// Percentage of x-height of additional horiz. space after sub/superscripts
	ScriptSpace float64

	// Percentage of x-height that sub/superscripts drop below the baseline
	SubDrop float64

	// Percentage of x-height that superscripts are raised from the baseline
	Sup1 float64

	// Percentage of x-height that subscripts drop below the baseline
	Sub1 float64

	// Percentage of x-height that subscripts drop below the baseline when a
	// superscript is present
	Sub2 float64

	// Percentage of x-height that sub/supercripts are offset relative to the
	// nucleus edge for non-slanted nuclei
	Delta float64

	// Additional percentage of last character height above 2/3 of the
	// x-height that supercripts are offset relative to the subscript
	// for slanted nuclei
	DeltaSlanted float64

	// Percentage of x-height that supercripts and subscripts are offset for
	// integrals
	DeltaIntegral float64
}

var DefaultFontConstants = FontConstants{
	ScriptSpace:   0.05,
	SubDrop:       0.4,
	Sup1:          0.7,
	Sub1:          0.3,
	Sub2:          0.5,
	Delta:         0.025,
	DeltaSlanted:  0.2,
	DeltaIntegral: 0.1,
}

// Node represents a node in the TeX box model.
type Node interface {
	// Kerning returns the amount of kerning between this and the next node.
	Kerning(next Node) float64

	// Shrinks one level smaller.
	// There are only three levels of sizes, after which things
	// will no longer get smaller.
	Shrink()

	// Grows one level larger.
	// There is no limit to how big something can get.
	Grow()

	// Render renders the node at (x,y) on the canvas.
	Render(x, y float64)

	// Width returns the width of this node.
	Width() float64

	// Height returns the height of this node.
	Height() float64

	// Depth returns the depth of this node.
	Depth() float64
}

// Box is a node with a physical location
type Box struct {
	size   int
	width  float64
	height float64
	depth  float64
}

func newBox(w, h, d float64) *Box {
	return &Box{width: w, height: h, depth: d}
}

func (*Box) Kerning(next Node) float64 { return 0 }

func (box *Box) Shrink() {
	box.size--
	if box.size < numSizeLevels {
		box.width *= shrinkFactor
		box.height *= shrinkFactor
		box.depth *= shrinkFactor
	}
}

func (box *Box) Grow() {
	box.size++
	box.width *= growFactor
	box.height *= growFactor
	box.depth *= growFactor
}

func (*Box) Render(x, y float64) {}

// Width returns the width of this node.
func (box *Box) Width() float64 { return box.width }

// Height returns the height of this node.
func (box *Box) Height() float64 { return box.height }

// Depth returns the depth of this node.
func (box *Box) Depth() float64 { return box.depth }

func (box *Box) hpackDims(width, height, depth *float64, stretch, shrink []float64) {
	*width += box.width
	if math.IsInf(box.height, 0) || math.IsInf(box.depth, 0) {
		return
	}
	*height = math.Max(*height, box.height)
	*depth = math.Max(*depth, box.depth)
}

func (box *Box) vpackDims(width, height, depth *float64, stretch, shrink []float64) {
	*height += *depth + box.height
	*depth = box.depth
	if math.IsInf(box.width, 0) {
		return
	}
	*width = math.Max(*width, box.width)
}

// VBox is a box with a height but no width.
func VBox(h, d float64) *Box {
	return newBox(0, h, d)
}

// HBox is a box with a width but no height nor depth.
func HBox(w float64) *Box {
	return newBox(w, 0, 0)
}

// Char is a single character.
//
// Unlike TeX, the font information and metrics are stored with each `Char`
// to make it easier to lookup the font metrics when needed.  Note that TeX
// boxes have a width, height, and depth, unlike Type1 and TrueType which use
// a full bounding box and an advance in the x-direction.  The metrics must
// be converted to the TeX model, and the advance (if different from width)
// must be converted into a `Kern` node when the `Char` is added to its parent
// `HList`.
type Char struct {
	c string

	size    int
	width   float64
	height  float64
	depth   float64
	metrics font.Metrics

	be   font.Backend
	font font.Font
	dpi  float64
	math bool
}

func NewChar(c string, state State, math bool) *Char {
	ch := &Char{
		c:    c,
		be:   state.Backend(),
		font: state.Font,
		dpi:  state.DPI,
		math: math,
	}
	ch.updateMetrics()
	return ch
}

func (ch *Char) updateMetrics() {
	ch.metrics = ch.be.Metrics(
		ch.c, ch.font, ch.dpi,
		ch.math,
	)
	switch ch.c {
	case " ":
		ch.width = ch.metrics.Advance
	default:
		ch.width = ch.metrics.Width
	}
	ch.height = ch.metrics.Iceberg
	ch.depth = -(ch.metrics.Iceberg - ch.metrics.Height)
}

func (c *Char) String() string { return c.c }

func (c *Char) Kerning(next Node) float64 {
	adv := c.metrics.Advance - c.Width()
	kern := 0.0
	switch next := next.(type) {
	case *Char:
		kern = c.be.Kern(c.font, c.c, next.font, next.c, c.dpi)
	case *Accent:
		kern = c.be.Kern(c.font, c.c, next.char.font, next.char.c, c.dpi)
	}
	return adv + kern
}

func (box *Char) Shrink() {
	box.size--
	if box.size < numSizeLevels {
		box.font.Size *= shrinkFactor
		box.width *= shrinkFactor
		box.height *= shrinkFactor
		box.depth *= shrinkFactor
	}
}

func (box *Char) Grow() {
	box.size++
	box.font.Size *= growFactor
	box.width *= growFactor
	box.height *= growFactor
	box.depth *= growFactor
}

func (c *Char) Render(x, y float64) {
	c.be.RenderGlyph(x, y, c.font, c.c, c.dpi)
}

// Width returns the width of this node.
func (c *Char) Width() float64 { return c.width }

// Height returns the height of this node.
func (c *Char) Height() float64 { return c.height }

// Depth returns the depth of this node.
func (c *Char) Depth() float64 { return c.depth }

func (c Char) hpackDims(width, height, depth *float64, stretch, shrink []float64) {
	*width += c.width
	*height = math.Max(*height, c.height)
	*depth = math.Max(*depth, c.depth)
}

func (*Char) vpackDims(width, height, depth *float64, stretch, shrink []float64) {
	panic("Char node in VList")
}

// Accent is a character with an accent.
// Accents need to be dealt with separately as they are already offset
// from the baseline in TrueType fonts.
type Accent struct {
	char Char
}

func NewAccent(c string, state State, math bool) *Accent {
	acc := &Accent{
		char: Char{
			c:    c,
			be:   state.Backend(),
			font: state.Font,
			dpi:  state.DPI,
			math: math,
		},
	}
	acc.updateMetrics()
	return acc
}

func (acc *Accent) updateMetrics() {
	acc.char.metrics = acc.char.be.Metrics(
		acc.char.c, acc.char.font, acc.char.dpi,
		acc.char.math,
	)
	acc.char.width = acc.char.metrics.XMax - acc.char.metrics.XMin
	acc.char.height = acc.char.metrics.YMax - acc.char.metrics.YMin
	acc.char.depth = 0
}

func (acc *Accent) String() string            { return acc.char.String() }
func (acc *Accent) Kerning(next Node) float64 { return acc.char.Kerning(next) }

func (acc *Accent) Shrink() {
	acc.char.Shrink()
	acc.updateMetrics()
}

func (acc *Accent) Grow() {
	acc.char.Grow()
	acc.updateMetrics()
}

func (acc *Accent) Render(x, y float64) {
	acc.char.be.RenderGlyph(
		x-acc.char.metrics.XMin,
		y+acc.char.metrics.YMin,
		acc.char.font,
		acc.char.c,
		acc.char.dpi,
	)
}

// Width returns the width of this node.
func (acc *Accent) Width() float64 { return acc.char.width }

// Height returns the height of this node.
func (acc *Accent) Height() float64 { return acc.char.height }

// Depth returns the depth of this node.
func (acc *Accent) Depth() float64 { return acc.char.depth }

func (acc *Accent) hpackDims(width, height, depth *float64, stretch, shrink []float64) {
	acc.char.hpackDims(width, height, depth, stretch, shrink)
}

func (*Accent) vpackDims(width, height, depth *float64, stretch, shrink []float64) {
	panic("Accent node in VList")
}

// List is a list of vertical or horizontal nodes.
type List struct {
	box      Box
	shift    float64 // shift is an arbitrary offset.
	children []Node  // children nodes of this list.

	glue struct {
		set   float64 // glue setting of this list
		sign  int     // 0: normal, -1: shrinking, 1: stretching
		order int     // the order of infinity (0 - 3) for the glue.
		ratio float64
	}
}

func ListOf(elements []Node) *List {
	list := &List{children: make([]Node, len(elements))}
	copy(list.children, elements)
	return list
}

// determineOrder determines the highest order of glue used by the members
// of a List.
//
// used by VPack and HPack.
func determineOrder(totals []float64) int {
	for i := len(totals) - 1; i >= 0; i-- {
		if totals[i] != 0 {
			return i
		}
	}
	return 0
}

func (lst *List) setGlue(x float64, sign int, totals []float64, errMsg, typ string) {
	o := determineOrder(totals)
	lst.glue.order = o
	lst.glue.sign = sign
	switch {
	case totals[o] != 0:
		lst.glue.set = x / totals[o]
	default:
		lst.glue.sign = 0
		lst.glue.ratio = 0
	}
	if o == 0 {
		if len(lst.children) > 0 {
			log.Printf("%s %s: %v", errMsg, typ, lst.children)
		}
	}
}

func (lst *List) Kerning(next Node) float64 {
	return lst.box.Kerning(next)
}

func (lst *List) Shrink() {
	for _, node := range lst.children {
		node.Shrink()
	}
	lst.box.Shrink()
	if lst.box.size < numSizeLevels {
		lst.shift *= shrinkFactor
		lst.glue.set *= shrinkFactor
	}
}

func (lst *List) Grow() {
	for _, node := range lst.children {
		node.Grow()
	}
	lst.box.Grow()
	lst.shift *= growFactor
	lst.glue.set *= growFactor
}

func (lst *List) Render(x, y float64) {
	lst.box.Render(x, y)
}

// Width returns the width of this node.
func (lst *List) Width() float64 { return lst.box.Width() }

// Height returns the height of this node.
func (lst *List) Height() float64 { return lst.box.Height() }

// Depth returns the depth of this node.
func (lst *List) Depth() float64 { return lst.box.Depth() }

func (lst *List) Nodes() []Node    { return lst.children }
func (lst *List) GlueOrder() int   { return lst.glue.order }
func (lst *List) GlueSign() int    { return lst.glue.sign }
func (lst *List) GlueSet() float64 { return lst.glue.set }

func (lst *List) hpackDims(width, height, depth *float64, stretch, shrink []float64) {
	*width += lst.box.width
	if math.IsInf(lst.box.height, 0) || math.IsInf(lst.box.depth, 0) {
		return
	}
	*height = math.Max(*height, lst.box.height-lst.shift)
	*depth = math.Max(*depth, lst.box.depth+lst.shift)
}

func (lst *List) vpackDims(width, height, depth *float64, stretch, shrink []float64) {
	*height += *depth + lst.box.height
	*depth = lst.box.depth
	if math.IsInf(lst.box.width, 0) {
		return
	}
	*width = math.Max(*width, lst.box.width)
}

// HList is a horizontal list of boxes.
type HList struct {
	lst List
}

func HListOf(elements []Node, doKern bool) *HList {
	lst := &HList{
		lst: *ListOf(elements),
	}
	if doKern {
		lst.kern()
	}
	const (
		width      = 0
		additional = true
	)
	lst.HPack(width, additional)
	return lst
}

// kern inserts Kern nodes between Char nodes to set kerning.
//
// The Char nodes themselves determine the amount of kerning they need.
// This method just creates the correct list.
func (lst *HList) kern() {
	if len(lst.lst.children) == 0 {
		return
	}
	var (
		n        = len(lst.lst.children)
		children = make([]Node, 0, n)
	)
	for i := range lst.lst.children {
		var (
			elem = lst.lst.children[i]
			next Node
			dist float64
		)
		if i < n-1 {
			next = lst.lst.children[i+1]
		}
		dist = elem.Kerning(next)
		children = append(children, elem)
		if dist != 0 {
			children = append(children, NewKern(dist))
		}
	}
	lst.lst.children = children
}

// HPack computes the dimensions of the resulting boxes, and adjusts the glue
// if one of those dimensions is pre-specified.
//
// The computed sizes normally enclose all of the material inside the new box;
// but some items may stick out if negative glue is used, if the box is
// overfull, or if a `\vbox` includes other boxes that have been shifted left.
//
// If additional is false, HPack will produce a box whose width is exactly as
// wide as the given 'width'.
// Otherwise, HPack will produce a box with the natural width of the contents,
// plus the given 'width'.
func (lst *HList) HPack(width float64, additional bool) {
	var (
		h float64
		d float64
		x float64

		totStretch = make([]float64, 4)
		totShrink  = make([]float64, 4)
	)

	for _, node := range lst.lst.children {
		switch node := node.(type) {
		case hpacker:
			node.hpackDims(&x, &h, &d, totStretch, totShrink)
		default:
			panic(fmt.Errorf("unknown node type %T", node))
		}
	}
	lst.lst.box.height = h
	lst.lst.box.depth = d

	if additional {
		width += x
	}
	lst.lst.box.width = width
	x = width - x
	switch {
	case x == 0:
		lst.lst.glue.sign = 0
		lst.lst.glue.order = 0
		lst.lst.glue.ratio = 0
	case x > 0:
		lst.lst.setGlue(x, 1, totStretch, "overfull", "HList")
	default:
		lst.lst.setGlue(x, -1, totShrink, "underfull", "HList")
	}
}

func (lst *HList) Kerning(next Node) float64 { return lst.lst.Kerning(next) }
func (lst *HList) Shrink()                   { lst.lst.Shrink() }
func (lst *HList) Grow()                     { lst.lst.Grow() }
func (lst *HList) Render(x, y float64)       { lst.lst.Render(x, x) }

// Width returns the width of this node.
func (lst *HList) Width() float64 { return lst.lst.Width() }

// Height returns the height of this node.
func (lst *HList) Height() float64 { return lst.lst.Height() }

// Depth returns the depth of this node.
func (lst *HList) Depth() float64 { return lst.lst.Depth() }

func (lst *HList) Nodes() []Node    { return lst.lst.Nodes() }
func (lst *HList) GlueOrder() int   { return lst.lst.GlueOrder() }
func (lst *HList) GlueSign() int    { return lst.lst.GlueSign() }
func (lst *HList) GlueSet() float64 { return lst.lst.GlueSet() }
func (lst *HList) Shift() float64   { return lst.lst.shift }

func (lst *HList) hpackDims(width, height, depth *float64, stretch, shrink []float64) {
	lst.lst.hpackDims(width, height, depth, stretch, shrink)
}

func (lst *HList) vpackDims(width, height, depth *float64, stretch, shrink []float64) {
	lst.lst.vpackDims(width, height, depth, stretch, shrink)
}

// VList is a vertical list of boxes.
type VList struct {
	lst List
}

func (lst *VList) SetShift(s float64) { lst.lst.shift = s }

func VListOf(elements []Node) *VList {
	lst := &VList{lst: *ListOf(elements)}
	var (
		height     float64
		additional = true
		max        = math.Inf(+1)
	)
	lst.VPack(height, additional, max)
	return lst
}

// VPack computes the dimensions of the resulting boxes, and adjusts the
// glue if one of those dimensions is pre-specified.
//
// If additional is false, VPack will produce a box whose height is exactly as
// tall as the given 'height'.
// Otherwise, VPack will produce a box with the natural height of the contents,
// plus the given 'height'.
func (lst *VList) VPack(height float64, additional bool, l float64) {
	var (
		w float64
		d float64
		x float64

		totStretch = make([]float64, 4)
		totShrink  = make([]float64, 4)
	)

	for _, node := range lst.lst.children {
		switch node := node.(type) {
		case vpacker:
			node.vpackDims(&w, &x, &d, totStretch, totShrink)
		}
	}

	lst.lst.box.width = w
	switch {
	case d > l:
		x += d - l
		lst.lst.box.depth = l
	default:
		lst.lst.box.depth = d
	}

	if additional {
		height += x
	}
	lst.lst.box.height = height
	x = height - x

	switch {
	case x == 0:
		lst.lst.glue.sign = 0
		lst.lst.glue.order = 0
		lst.lst.glue.ratio = 0
	case x > 0:
		lst.lst.setGlue(x, +1, totStretch, "overfull", "VList")
	default:
		lst.lst.setGlue(x, -1, totShrink, "underfull", "VList")
	}
}

func (lst *VList) Kerning(next Node) float64 { return lst.lst.Kerning(next) }
func (lst *VList) Shrink()                   { lst.lst.Shrink() }
func (lst *VList) Grow()                     { lst.lst.Grow() }
func (lst *VList) Render(x, y float64)       { lst.lst.Render(x, y) }

// Width returns the width of this node.
func (lst *VList) Width() float64 { return lst.lst.Width() }

// Height returns the height of this node.
func (lst *VList) Height() float64 { return lst.lst.Height() }

// Depth returns the depth of this node.
func (lst *VList) Depth() float64 { return lst.lst.Depth() }

func (lst *VList) Nodes() []Node    { return lst.lst.Nodes() }
func (lst *VList) GlueOrder() int   { return lst.lst.GlueOrder() }
func (lst *VList) GlueSign() int    { return lst.lst.GlueSign() }
func (lst *VList) GlueSet() float64 { return lst.lst.GlueSet() }

func (lst *VList) hpackDims(width, height, depth *float64, stretch, shrink []float64) {
	lst.lst.hpackDims(width, height, depth, stretch, shrink)
}

func (lst *VList) vpackDims(width, height, depth *float64, stretch, shrink []float64) {
	lst.lst.vpackDims(width, height, depth, stretch, shrink)
}

// Rule is a solid black rectangle.
//
// Like a HList, Rule has a width, a depth and a height.
// However, if any of these dimensions is ∞, the actual value will be
// determined by running the rule up to the boundary of the innermost
// enclosing box.
// This is called a "running dimension".
// The width is never running in an HList; the height and depth are never
// running in a VList.
type Rule struct {
	box Box
	out font.Backend
}

func NewRule(w, h, d float64, state State) *Rule {
	return &Rule{
		box: *newBox(w, h, d),
		out: state.Backend(),
	}
}

func (rule *Rule) String() string {
	return fmt.Sprintf(
		"Rule{w=%g, h=%g, d=%g}",
		rule.Width(), rule.Height(), rule.Depth(),
	)
}

func (rule *Rule) render(x, y, w, h float64) {
	rule.out.RenderRectFilled(x, y, x+w, y+h)
}

func (rule *Rule) Kerning(next Node) float64 { return rule.box.Kerning(next) }
func (rule *Rule) Shrink()                   { rule.box.Shrink() }
func (rule *Rule) Grow()                     { rule.box.Grow() }
func (rule *Rule) Render(x, y float64)       { rule.box.Render(x, y) }

// Width returns the width of this node.
func (rule *Rule) Width() float64 { return rule.box.Width() }

// Height returns the height of this node.
func (rule *Rule) Height() float64 { return rule.box.Height() }

// Depth returns the depth of this node.
func (rule *Rule) Depth() float64 { return rule.box.Depth() }

func (rule *Rule) hpackDims(width, height, depth *float64, stretch, shrink []float64) {
	rule.box.hpackDims(width, height, depth, stretch, shrink)
}

func (rule *Rule) vpackDims(width, height, depth *float64, stretch, shrink []float64) {
	rule.box.vpackDims(width, height, depth, stretch, shrink)
}

// HRule is a horizontal rule.
func HRule(state State, thickness float64) *Rule {
	if thickness < 0 {
		thickness = state.Backend().UnderlineThickness(state.Font, state.DPI)
	}
	var (
		height = 0.5 * thickness
		depth  = 0.5 * thickness
	)
	return NewRule(math.Inf(+1), height, depth, state)
}

// VRule is a vertical rule.
func VRule(state State) *Rule {
	thickness := state.Backend().UnderlineThickness(state.Font, state.DPI)
	return NewRule(thickness, math.Inf(+1), math.Inf(+1), state)
}

type Glue struct {
	size         int
	width        float64
	stretch      float64
	stretchOrder int
	shrink       float64
	shrinkOrder  int
}

func NewGlue(typ string) *Glue {
	switch typ {
	case "fil":
		return newGlue(0, 1, 1, 0, 0)
	case "fill":
		return newGlue(0, 1, 2, 0, 0)
	case "filll":
		return newGlue(0, 1, 3, 0, 0)
	case "neg_fil":
		return newGlue(0, 0, 0, 1, 1)
	case "neg_fill":
		return newGlue(0, 0, 0, 1, 2)
	case "neg_filll":
		return newGlue(0, 0, 0, 1, 3)
	case "empty":
		return &Glue{}
	case "ss":
		return newGlue(0, 1, 1, -1, 1)
	default:
		panic(fmt.Errorf("tex: unknown Glue spec %q", typ))
	}
}

func newGlue(w, st float64, sto int, sh float64, sho int) *Glue {
	return &Glue{
		size:         0,
		width:        w,
		stretch:      st,
		stretchOrder: sto,
		shrink:       sh,
		shrinkOrder:  sho,
	}
}

func (g *Glue) Kerning(next Node) float64 { return 0 }

func (g *Glue) Shrink() {
	g.size--
	if g.size < numSizeLevels {
		g.width *= shrinkFactor
	}
}

func (g *Glue) Grow() {
	g.size++
	g.width *= growFactor
}

func (g *Glue) Render(x, y float64) {}

// Width returns the width of this node.
func (g *Glue) Width() float64 { return g.width }

// Height returns the height of this node.
func (g *Glue) Height() float64 { return 0 }

// Depth returns the depth of this node.
func (g *Glue) Depth() float64 { return 0 }

func (g *Glue) hpackDims(width, height, depth *float64, stretch, shrink []float64) {
	*width += g.width
	stretch[g.stretchOrder] += g.stretch
	shrink[g.shrinkOrder] += g.shrink
}

func (g *Glue) vpackDims(width, height, depth *float64, stretch, shrink []float64) {
	*height += *depth
	*depth = 0
	*height += g.width
	stretch[g.stretchOrder] += g.stretch
	shrink[g.shrinkOrder] += g.shrink
}

// HCentered creates an HList whose contents are centered within
// its enclosing box.
func HCentered(elements []Node) *HList {
	const doKern = false
	nodes := make([]Node, 0, len(elements)+2)
	nodes = append(nodes, NewGlue("ss"))
	nodes = append(nodes, elements...)
	nodes = append(nodes, NewGlue("ss"))
	return HListOf(nodes, doKern)
}

// VCentered creates a VList whose contents are centered within
// its enclosing box.
func VCentered(elements []Node) *VList {
	nodes := make([]Node, 0, len(elements)+2)
	nodes = append(nodes, NewGlue("ss"))
	nodes = append(nodes, elements...)
	nodes = append(nodes, NewGlue("ss"))
	return VListOf(nodes)
}

// Kern is a node with a width to specify a (normally negative) amount of spacing.
//
// This spacing correction appears in horizontal lists between letters
// like A and V, when the font designer decided it looks better to move them
// closer together or further apart.
// A Kern node can also appear in a vertical list, when its width denotes
// spacing in the vertical direction.
type Kern struct {
	size  int
	width float64
}

func NewKern(width float64) *Kern {
	return &Kern{width: width}
}

func (k *Kern) String() string { return fmt.Sprintf("k%.02f", k.width) }

func (k *Kern) Kerning(next Node) float64 { return 0 }

func (k *Kern) Shrink() {
	k.size--
	if k.size < numSizeLevels {
		k.width *= shrinkFactor
	}
}

func (k *Kern) Grow() {
	k.size++
	k.width *= growFactor
}

func (k *Kern) Render(x, y float64) {}

// Width returns the width of this node.
func (k *Kern) Width() float64 { return k.width }

// Height returns the height of this node.
func (k *Kern) Height() float64 { return 0 }

// Depth returns the depth of this node.
func (k *Kern) Depth() float64 { return 0 }

func (k *Kern) hpackDims(width, height, depth *float64, stretch, shrink []float64) {
	*width += k.width
}

func (k *Kern) vpackDims(width, height, depth *float64, stretch, shrink []float64) {
	*height += *depth + k.width
	*depth = 0
}

type SubSuperCluster struct {
	*HList
	//	nucleus interface{} // FIXME
	//	sub     interface{} // FIXME
	//	super   interface{} // FIXME
}

// AutoHeightChar creats a character as close to the given height and depth
// as possible.
func AutoHeightChar(c string, height, depth float64, state State, factor float64) *HList {
	// FIXME(sbinet): implement sized-alternatives-for-symbol
	alts := []struct {
		font string
		sym  string
	}{
		{state.Font.Name, c},
	}

	const math = true
	var (
		xheight = state.Backend().XHeight(state.Font, state.DPI)
		target  = height + depth

		ch    *Char
		shift float64
	)

	for _, v := range alts {
		state.Font.Name = v.font
		ch = NewChar(c, state, math)
		// ensure that size 0 is chosen when the text is regular sized
		// but with descender glyphs by subtracting 0.2*xheight
		if ch.Height()+ch.Depth() >= target-0.2*xheight {
			break
		}
	}

	if state.Font.Name != "" {
		if factor == 0 {
			factor = target / (ch.Height() + ch.Depth())
		}
		state.Font.Size *= factor

		ch = NewChar(c, state, math)
		shift = depth - ch.Depth()
	}

	hlist := HListOf([]Node{ch}, true)
	hlist.lst.shift = shift

	return hlist
}

// Ship boxes to output once boxes have been set up.
//
// Since boxes can be inside of boxes inside of boxes... the main work of
// Ship is done by two mutually recursive routines, hlistOut and vlistOut,
// which traverse the HList and VList nodes inside of horizontal and vertical
// boxes.
type Ship struct {
	maxPush int // deepest nesting of push commands, so far.
	cur     struct {
		s int
		v float64
		h float64
	}
	off struct {
		h float64
		v float64
	}
}

func (ship *Ship) Call(ox, oy float64, box Tree) {
	ship.maxPush = 0
	ship.cur.s = 0
	ship.cur.v = 0
	ship.cur.h = 0
	ship.off.h = ox
	ship.off.v = oy + box.Height()
	ship.hlistOut(box)
}

func (ship *Ship) hlistOut(box Tree) {
	var (
		curG      int
		curGlue   float64
		glueOrder = box.GlueOrder()
		glueSign  = box.GlueSign()
		baseLine  = ship.cur.v
	)

	ship.cur.s++
	ship.maxPush = maxInt(ship.cur.s, ship.maxPush)

	for _, node := range box.Nodes() {
		switch node := node.(type) {
		case *Char:
			node.Render(ship.cur.h+ship.off.h, ship.cur.v+ship.off.v)
			ship.cur.h += node.Width()
		case *Accent:
			node.Render(ship.cur.h+ship.off.h, ship.cur.v+ship.off.v)
			ship.cur.h += node.Width()
		case *Kern:
			ship.cur.h += node.Width()
		case *HList:
			// node623
			switch len(node.Nodes()) {
			case 0:
				ship.cur.h += node.Width()
			default:
				edge := ship.cur.h
				ship.cur.v = baseLine + node.lst.shift
				ship.hlistOut(node)
				ship.cur.h = edge + node.Width()
				ship.cur.v = baseLine
			}
		case *VList:
			// node623
			switch len(node.Nodes()) {
			case 0:
				ship.cur.h += node.Width()
			default:
				edge := ship.cur.h
				ship.cur.v = baseLine + node.lst.shift
				ship.vlistOut(node)
				ship.cur.h = edge + node.Width()
				ship.cur.v = baseLine
			}
		case *Glue:
			// node625
			ruleWidth := node.width - float64(curG)
			if glueSign != 0 { // normal
				switch {
				case glueSign == 1: // stretching
					if node.stretchOrder == glueOrder {
						curGlue += node.stretch
						curG = int(math.Round(clamp(box.GlueSet() * curGlue)))
					}
				case node.shrinkOrder == glueOrder: // shrinking
					curGlue += node.shrink
					curG = int(math.Round(clamp(box.GlueSet() * curGlue)))
				}
			}
			ruleWidth += float64(curG)
			ship.cur.h += ruleWidth
		case Node:
			// node624
			ruleHeight := node.Height()
			ruleDepth := node.Depth()
			ruleWidth := node.Width()
			if math.IsInf(ruleHeight, 0) {
				ruleHeight = box.Height()
			}
			if math.IsInf(ruleDepth, 0) {
				ruleDepth = box.Depth()
			}
			if ruleHeight > 0 && ruleWidth > 0 {
				ship.cur.v = baseLine + ruleDepth
				type renderXYWH interface {
					render(x, y, w, h float64)
				}
				node.(renderXYWH).render(
					ship.cur.h+ship.off.h,
					ship.cur.v+ship.off.v,
					ruleWidth, ruleHeight,
				)
				ship.cur.v = baseLine
			}
			ship.cur.h += ruleWidth
		}
	}
	ship.cur.s--
}

func (ship *Ship) vlistOut(box Tree) {
	var (
		curG      int
		curGlue   float64
		glueOrder = box.GlueOrder()
		glueSign  = box.GlueSign()
		leftEdge  = ship.cur.h
	)

	ship.cur.s++
	ship.maxPush = maxInt(ship.cur.s, ship.maxPush)
	ship.cur.v -= box.Height()

	for _, node := range box.Nodes() {
		switch node := node.(type) {
		case *Kern:
			ship.cur.v += node.Width()
		case *HList:
			switch len(node.Nodes()) {
			case 0:
				ship.cur.v += node.Height() + node.Depth()
			default:
				ship.cur.v += node.Height()
				ship.cur.h = leftEdge + node.lst.shift
				curV := ship.cur.v
				node.lst.box.width = box.Width()
				ship.hlistOut(node)
				ship.cur.v = curV + node.Depth()
				ship.cur.h = leftEdge
			}
		case *VList:
			switch len(node.Nodes()) {
			case 0:
				ship.cur.v += node.Height() + node.Depth()
			default:
				ship.cur.v += node.Height()
				ship.cur.h = leftEdge + node.lst.shift
				curV := ship.cur.v
				node.lst.box.width = box.Width()
				ship.vlistOut(node)
				ship.cur.v = curV + node.Depth()
				ship.cur.h = leftEdge
			}

		case *Glue:
			ruleHeight := node.width - float64(curG)
			if glueSign != 0 { // normal
				switch {
				case glueSign == 1: // stretching
					if node.stretchOrder == glueOrder {
						curGlue += node.stretch
						curG = int(math.Round(clamp(box.GlueSet() * curGlue)))
					}
				case glueOrder == node.shrinkOrder: // shrinking
					curGlue += node.shrink
					curG = int(math.Round(clamp(box.GlueSet() * curGlue)))
				}
			}
			ruleHeight += float64(curG)
			ship.cur.v += ruleHeight
		case *Char:
			panic("tex: Char node found in vlist")
		case *Accent:
			panic("tex: Accent node found in vlist")
		case Node:
			var (
				ruleHeight = node.Height()
				ruleDepth  = node.Depth()
				ruleWidth  = node.Width()
			)
			if math.IsInf(ruleWidth, 0) {
				ruleWidth = box.Width()
			}
			ruleHeight += ruleDepth
			if ruleHeight > 0 && ruleDepth > 0 {
				ship.cur.v += ruleHeight
				type renderXYWH interface {
					render(x, y, w, h float64)
				}
				if p, ok := node.(renderXYWH); ok {
					p.render(
						ship.cur.h+ship.off.h,
						ship.cur.v+ship.off.v,
						ruleWidth, ruleHeight,
					)
				}
			}
		}
	}
	ship.cur.s--
}

type hpacker interface {
	hpackDims(width, height, depth *float64, stretch, shrink []float64)
}

type vpacker interface {
	vpackDims(width, height, depth *float64, stretch, shrink []float64)
}

type Tree interface {
	Node

	Nodes() []Node
	GlueOrder() int
	GlueSign() int
	GlueSet() float64
}

var (
	_ Node = (*Box)(nil)
	_ Node = (*Char)(nil)
	_ Node = (*Accent)(nil)
	_ Node = (*List)(nil)
	_ Node = (*HList)(nil)
	_ Node = (*VList)(nil)
	_ Node = (*Rule)(nil)
	_ Node = (*Glue)(nil)
	_ Node = (*Kern)(nil)
	_ Node = (*SubSuperCluster)(nil)

	_ hpacker = (*Box)(nil)
	_ hpacker = (*Char)(nil)
	_ hpacker = (*Accent)(nil)
	_ hpacker = (*List)(nil)
	_ hpacker = (*HList)(nil)
	_ hpacker = (*VList)(nil)
	_ hpacker = (*Rule)(nil)
	_ hpacker = (*Glue)(nil)
	_ hpacker = (*Kern)(nil)
	_ hpacker = (*SubSuperCluster)(nil)

	_ vpacker = (*Box)(nil)
	_ vpacker = (*Char)(nil)
	_ vpacker = (*Accent)(nil)
	_ vpacker = (*List)(nil)
	_ vpacker = (*HList)(nil)
	_ vpacker = (*VList)(nil)
	_ vpacker = (*Rule)(nil)
	_ vpacker = (*Glue)(nil)
	_ vpacker = (*Kern)(nil)
	_ vpacker = (*SubSuperCluster)(nil)

	_ Tree = (*List)(nil)
	_ Tree = (*HList)(nil)
	_ Tree = (*VList)(nil)
)
