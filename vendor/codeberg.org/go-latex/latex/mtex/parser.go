// Copyright ©2020 The go-latex Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mtex

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"codeberg.org/go-latex/latex"
	"codeberg.org/go-latex/latex/ast"
	"codeberg.org/go-latex/latex/font"
	"codeberg.org/go-latex/latex/internal/tex2unicode"
	"codeberg.org/go-latex/latex/mtex/symbols"
	"codeberg.org/go-latex/latex/tex"
)

// Parse parses a LaTeX math expression and returns the TeX-like box model
// and an error if any.
func Parse(expr string, fontSize, DPI float64, backend font.Backend) (tex.Node, error) {
	p := newParser(backend)
	return p.parse(expr, fontSize, DPI)
}

type parser struct {
	be font.Backend

	expr   string
	macros map[string]handler
}

func newParser(be font.Backend) *parser {
	p := &parser{
		be:     be,
		macros: make(map[string]handler),
	}
	p.init()

	return p
}

func (p *parser) parse(x string, size, dpi float64) (tex.Node, error) {
	p.expr = x
	node, err := latex.ParseExpr(x)
	if err != nil {
		return nil, fmt.Errorf("could not parse latex expression %q: %w", x, err)
	}

	state := tex.NewState(p.be, font.Font{
		Name: "default",
		Size: size,
		Type: "rm",
	}, dpi)

	v := visitor{p: p, state: state}
	ast.Walk(&v, node)
	nodes := tex.HListOf(v.nodes, true)

	return nodes, nil
}

type visitor struct {
	p     *parser
	nodes []tex.Node
	state tex.State
	math  bool
}

func (v *visitor) Visit(n ast.Node) ast.Visitor {
	switch n := n.(type) {
	case ast.List:
	case *ast.Symbol:
		switch {
		case v.math:
			h := v.p.handler(n.Text)
			if h == nil {
				panic("no handler for symbol [" + n.Text + "]")
			}
			v.nodes = append(v.nodes, h.Handle(v.p, n, v.state, v.math))
		default:
			v.nodes = append(v.nodes, tex.NewChar(string(n.Text), v.state, v.math))
		}
	case *ast.Word:
		var nodes []tex.Node
		for _, x := range n.Text {
			nodes = append(nodes, tex.NewChar(string(x), v.state, v.math))
		}
		v.nodes = append(v.nodes, tex.HListOf(nodes, true))
	case *ast.Literal:
		h := handlerFunc(handleSymbol)
		for _, c := range n.Text {
			n := &ast.Literal{Text: string(c)}
			v.nodes = append(v.nodes, h.Handle(v.p, n, v.state, v.math))
		}

	case *ast.MathExpr:
		oldm := v.math
		oldt := v.state.Font.Type
		v.math = true
		v.state.Font.Type = rcparams("mathtext.default").(string)

		for _, x := range n.List {
			v.Visit(x)
		}
		v.math = oldm
		v.state.Font.Type = oldt
		return nil

	case *ast.Macro:
		if n.Name == nil {
			panic("macro with nil identifier")
		}
		macro := n.Name.Name
		h := v.p.handler(macro)
		if h == nil {
			panic(fmt.Errorf("unknown macro %q", macro))
		}
		v.nodes = append(v.nodes, h.Handle(v.p, n, v.state, v.math))
		return nil

	case nil:
		return v

	default:
		panic(fmt.Errorf("unknown ast node %T", n))
	}
	return v
}

func (p *parser) handleNode(node ast.Node, state tex.State, math bool) tex.Node {
	v := visitor{p: p, state: state, math: math}
	ast.Walk(&v, node)
	return tex.HListOf(v.nodes, true)
}

func (p *parser) handler(name string) handler {
	if _, ok := spaceWidth[name]; ok {
		return handlerFunc(handleSpace)
	}
	if symbols.IsSpaced(name) || symbols.PunctuationSymbols.Has(name) {
		return handlerFunc(handleSymbol)
	}
	if name == `\hspace` {
		return handlerFunc(handleCustomSpace)
	}
	if symbols.FunctionNames.Has(name[1:]) { // drop leading `\`
		return handlerFunc(handleFunction)
	}
	switch name {
	case `\frac`:
		return handlerFunc(handleFrac)
	case `\dfrac`:
		return handlerFunc(handleDFrac)
	case `\tfrac`:
		return handlerFunc(handleTFrac)
	case `\binom`:
		return handlerFunc(handleBinom)
		// case `\genfrac`:
		// 	return handlerFunc(handleGenFrac)
	case `\sqrt`:
		return handlerFunc(handleSqrt)
	case `\overline`:
		return handlerFunc(handleOverline)
	}
	_, ok := p.macros[name]
	if ok {
		return handlerFunc(handleSymbol)
	}
	return nil
}

func (p *parser) init() {
	for _, k := range tex2unicode.Symbols() {
		p.macros[`\`+k] = builtinMacro("")
	}
	for k, v := range builtinMacros {
		p.macros[k] = v
	}
}

func handleSymbol(p *parser, node ast.Node, state tex.State, math bool) tex.Node {
	pos := int(node.Pos())
	sym := ""
	switch node := node.(type) {
	case *ast.Macro:
		sym = node.Name.Name
	case *ast.Symbol:
		sym = node.Text
	case *ast.Word:
		sym = node.Text
	case *ast.Literal:
		sym = node.Text
	default:
		panic("invalid ast Node")
	}
	ch := tex.NewChar(sym, state, math)
	switch {
	case symbols.IsSpaced(sym):
		i := strings.LastIndexFunc(p.expr[:pos], func(r rune) bool {
			return r != ' '
		})
		prev := ""
		if i >= 0 {
			prev = string(p.expr[i])
		}
		switch {
		case symbols.BinaryOperators.Has(sym) && (len(strings.Split(p.expr[:pos], " ")) == 0 ||
			prev == "{" ||
			symbols.LeftDelim.Has(prev)):
			// binary operators at start of string should not be spaced
			return ch
		default:
			return tex.HListOf([]tex.Node{
				p.makeSpace(state, 0.2),
				ch,
				p.makeSpace(state, 0.2),
			}, true)
		}

	case symbols.PunctuationSymbols.Has(sym):
		switch sym {
		case ".":
			pos := strings.Index(p.expr[pos:], sym)
			if (pos > 0 && isdigit(p.expr[pos-1])) &&
				(pos < len(p.expr)-1 && isdigit(p.expr[pos+1])) {
				// do not space dots as decimal separators.
				return ch
			}
			return tex.HListOf([]tex.Node{
				ch,
				p.makeSpace(state, 0.2),
			}, true)
		}
		panic("not implemented")
	}
	return ch
}

var spaceWidth = map[string]float64{
	`\,`:         0.16667,  // 3/18 em = 3 mu
	`\thinspace`: 0.16667,  // 3/18 em = 3 mu
	`\/`:         0.16667,  // 3/18 em = 3 mu
	`\>`:         0.22222,  // 4/18 em = 4 mu
	`\:`:         0.22222,  // 4/18 em = 4 mu
	`\;`:         0.27778,  // 5/18 em = 5 mu
	`\ `:         0.33333,  // 6/18 em = 6 mu
	`~`:          0.33333,  // 6/18 em = 6 mu, nonbreakable
	`\enspace`:   0.5,      // 9/18 em = 9 mu
	`\quad`:      1,        // 1 em = 18 mu
	`\qquad`:     2,        // 2 em = 36 mu
	`\!`:         -0.16667, // -3/18 em = -3 mu

}

func handleSpace(p *parser, node ast.Node, state tex.State, math bool) tex.Node {
	var (
		width float64
		ok    bool
	)
	switch node := node.(type) {
	case *ast.Symbol:
		width, ok = spaceWidth[node.Text]
	case *ast.Macro:
		width, ok = spaceWidth[node.Name.Name]
	default:
		panic(fmt.Errorf("invalid ast node %#v (%T)", node, node))
	}
	if !ok {
		panic(fmt.Errorf("could not find a width for %#v (%T)", node, node))
	}

	return p.makeSpace(state, width)
}

func handleCustomSpace(p *parser, node ast.Node, state tex.State, math bool) tex.Node {
	macro := node.(*ast.Macro)
	arg := macro.Args[0].(*ast.Arg).List[0].(*ast.Literal).Text
	val, err := strconv.ParseFloat(arg, 64)
	if err != nil {
		panic(fmt.Errorf("could not parse customspace: %+v", err))
	}
	return p.makeSpace(state, val)
}

func handleFunction(p *parser, node ast.Node, state tex.State, math bool) tex.Node {
	macro := node.(*ast.Macro)
	state.Font.Type = "rm"
	fun := macro.Name.Name[1:] // drop leading `\`
	nodes := make([]tex.Node, 0, len(fun))
	for _, c := range fun {
		nodes = append(nodes, tex.NewChar(string(c), state, math))
	}
	return tex.HListOf(nodes, true)
}

func handleFrac(p *parser, node ast.Node, state tex.State, math bool) tex.Node {
	var (
		macro     = node.(*ast.Macro)
		thickness = state.Backend().UnderlineThickness(state.Font, state.DPI)
		numNode   = ast.List(macro.Args[0].(*ast.Arg).List)
		denNode   = ast.List(macro.Args[1].(*ast.Arg).List)
	)

	num := p.handleNode(numNode, state, math)
	den := p.handleNode(denNode, state, math)

	// FIXME(sbinet): this should be infered from the context.
	// ie: textStyle    when in $  $ environment.
	//     displayStyle when in \[\] environment.
	sty := textStyle

	return p.genfrac("", "", thickness, sty, num, den, state)
}

func handleDFrac(p *parser, node ast.Node, state tex.State, math bool) tex.Node {
	var (
		macro     = node.(*ast.Macro)
		thickness = state.Backend().UnderlineThickness(state.Font, state.DPI)
		numNode   = ast.List(macro.Args[0].(*ast.Arg).List)
		denNode   = ast.List(macro.Args[1].(*ast.Arg).List)
	)

	num := p.handleNode(numNode, state, math)
	den := p.handleNode(denNode, state, math)

	return p.genfrac("", "", thickness, displayStyle, num, den, state)
}

func handleTFrac(p *parser, node ast.Node, state tex.State, math bool) tex.Node {
	var (
		macro     = node.(*ast.Macro)
		thickness = state.Backend().UnderlineThickness(state.Font, state.DPI)
		numNode   = ast.List(macro.Args[0].(*ast.Arg).List)
		denNode   = ast.List(macro.Args[1].(*ast.Arg).List)
	)

	num := p.handleNode(numNode, state, math)
	den := p.handleNode(denNode, state, math)

	return p.genfrac("", "", thickness, textStyle, num, den, state)
}

func handleBinom(p *parser, node ast.Node, state tex.State, math bool) tex.Node {
	var (
		macro   = node.(*ast.Macro)
		numNode = ast.List(macro.Args[0].(*ast.Arg).List)
		denNode = ast.List(macro.Args[1].(*ast.Arg).List)
	)

	num := p.handleNode(numNode, state, math)
	den := p.handleNode(denNode, state, math)

	return p.genfrac("(", ")", 0, textStyle, num, den, state)
}

func (p *parser) genfrac(ldelim, rdelim string, rule float64, style mathStyleKind, num, den tex.Node, state tex.State) tex.Node {
	thickness := state.Backend().UnderlineThickness(state.Font, state.DPI)

	if style != displayStyle {
		num.Shrink()
		den.Shrink()
	}

	cnum := tex.HCentered([]tex.Node{num})
	cden := tex.HCentered([]tex.Node{den})
	width := math.Max(num.Width(), den.Width())

	const additional = false // i.e.: exactly
	cnum.HPack(width, additional)
	cden.HPack(width, additional)

	vlist := tex.VListOf([]tex.Node{
		cnum,                     // numerator
		tex.VBox(0, thickness*2), // space
		tex.HRule(state, rule),   // rule
		tex.VBox(0, thickness*2), // space
		cden,                     // denominator
	})

	// shift so the fraction line sits in the middle of the '=' sign
	fnt := state.Font
	fnt.Type = rcparams("mathtext.default").(string)
	metrics := state.Backend().Metrics("=", fnt, state.DPI, true)
	shift := cden.Height() - ((metrics.YMax+metrics.YMin)/2 - 3*thickness)
	vlist.SetShift(shift)

	box := tex.HListOf([]tex.Node{vlist, tex.HBox(2 * thickness)}, true)
	if ldelim != "" || rdelim != "" {
		if ldelim == "" {
			ldelim = "."
		}
		if rdelim == "" {
			rdelim = "."
		}
		return p.autoSizedDelimiter(ldelim, []tex.Node{box}, rdelim, state)
	}

	return box
}

func handleSqrt(p *parser, node ast.Node, state tex.State, math bool) tex.Node {
	var (
		macro = node.(*ast.Macro)
		root  tex.Node
		body  *tex.HList
	)
	switch len(macro.Args) {
	case 2:
		root = p.handleNode(
			ast.List(macro.Args[0].(*ast.OptArg).List),
			state, math,
		)
		body = p.handleNode(
			ast.List(macro.Args[1].(*ast.Arg).List),
			state, math,
		).(*tex.HList)
	case 1:
		// ok
		body = p.handleNode(
			ast.List(macro.Args[0].(*ast.Arg).List),
			state, math,
		).(*tex.HList)
	default:
		panic("invalid sqrt")
	}

	thickness := state.Backend().UnderlineThickness(state.Font, state.DPI)

	// determine the height of the body, add a little extra to it so
	// it doesn't seem too cramped.
	height := body.Height() - body.Shift() + 5*thickness
	depth := body.Depth() + body.Shift()
	check := tex.AutoHeightChar(`\__sqrt__`, height, depth, state, 0)
	height = check.Height() - check.Shift()
	depth = check.Depth() + check.Shift()

	// put a little extra space to the left and right of the body
	padded := tex.HListOf([]tex.Node{
		tex.HBox(2 * thickness),
		body,
		tex.HBox(2 * thickness),
	}, true)
	rhs := tex.VListOf([]tex.Node{
		tex.HRule(state, -1),
		tex.NewGlue("fill"),
		padded,
	})

	// stretch the glue between the HRule and the body
	const additional = false
	rhs.VPack(height+(state.Font.Size*state.DPI)/(100*12), additional, depth)

	// add the root and shift it upward so it is above the tick.
	switch root {
	case nil:
		root = tex.HBox(check.Width() * 0.5)
	default:
		root.Shrink()
		root.Shrink()
	}

	vl := tex.VListOf([]tex.Node{
		tex.HListOf([]tex.Node{
			root,
		}, true),
	})
	vl.SetShift(-height * 0.6)

	hl := tex.HListOf([]tex.Node{
		vl, // root
		// negative kerning to put root over tick
		tex.NewKern(-check.Width() * 0.5),
		check,
		rhs,
	}, true)

	return hl
}

func handleOverline(p *parser, node ast.Node, state tex.State, math bool) tex.Node {
	macro := node.(*ast.Macro)
	body := p.handleNode(
		ast.List(macro.Args[0].(*ast.Arg).List),
		state, math,
	).(*tex.HList)

	thickness := state.Backend().UnderlineThickness(state.Font, state.DPI)

	height := body.Height() - body.Shift() + 3*thickness
	depth := body.Depth() + body.Shift()

	// place overline above body
	rhs := tex.VListOf([]tex.Node{
		tex.HRule(state, -1),
		tex.NewGlue("fill"),
		tex.HListOf([]tex.Node{body}, true),
	})

	// stretch the glue between the HRule and the body
	const additional = false
	rhs.VPack(height+(state.Font.Size*state.DPI)/(100*12), additional, depth)

	hl := tex.HListOf([]tex.Node{rhs}, true)
	return hl
}

func (p *parser) makeSpace(state tex.State, percentage float64) *tex.Kern {
	const math = true
	fnt := state.Font
	fnt.Name = "it"
	fnt.Type = rcparams("mathtext.default").(string)
	width := p.be.Metrics("m", fnt, state.DPI, math).Advance
	return tex.NewKern(width * percentage)
}

func (p *parser) autoSizedDelimiter(left string, middle []tex.Node, right string, state tex.State) tex.Node {
	var (
		height float64
		depth  float64
		factor float64 = 1
	)

	if len(middle) > 0 {
		for _, node := range middle {
			height = math.Max(height, node.Height())
			depth = math.Max(depth, node.Depth())
		}
		factor = 0
	}

	var parts []tex.Node
	if left != "." {
		// \left. isn't supposed to produce any symbol
		ahc := tex.AutoHeightChar(left, height, depth, state, factor)
		parts = append(parts, ahc)
	}
	parts = append(parts, middle...)
	if right != "." {
		// \right. isn't supposed to produce any symbol
		ahc := tex.AutoHeightChar(right, height, depth, state, factor)
		parts = append(parts, ahc)
	}
	return tex.HListOf(parts, true)
}

type mathStyleKind int

const (
	displayStyle mathStyleKind = iota
	textStyle
	//scriptStyle       // FIXME
	//scriptScriptStyle // FIXME
)

func rcparams(k string) interface{} {
	switch k {
	case "mathtext.default":
		return "it"
	default:
		panic("unknown rc.params key [" + k + "]")
	}
}

func isdigit(v byte) bool {
	switch v {
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return true
	}
	return false
}
