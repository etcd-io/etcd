// Copyright ©2016 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter

import (
	"fmt"
	"image/color"
	"math"
	"sort"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/font"
	"gonum.org/v1/plot/text"
	"gonum.org/v1/plot/tools/bezier"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
)

// A Sankey diagram presents stock and flow data as rectangles representing
// the amount of each stock and lines between the stocks representing the
// amount of each flow.
type Sankey struct {
	// Color specifies the default fill
	// colors for the stocks and flows. If Color is not nil,
	// each stock and flow is rendered filled with Color,
	// otherwise no fill is performed. Colors can be
	// modified for individual stocks and flows.
	Color color.Color

	// StockBarWidth is the widths of the bars representing
	// the stocks. The default value is 15% larger than the
	// height of the stock label text.
	StockBarWidth vg.Length

	// LineStyle specifies the default border
	// line style for the stocks and flows. Styles can be
	// modified for individual stocks and flows.
	LineStyle draw.LineStyle

	// TextStyle specifies the default stock label
	// text style. Styles can be modified for
	// individual stocks.
	TextStyle text.Style

	flows []Flow

	// FlowStyle is a function that specifies the
	// background color and border line style of the
	// flow based on its group name. The default
	// function uses the default Color and LineStyle
	// specified above for all groups.
	FlowStyle func(group string) (color.Color, draw.LineStyle)

	// StockStyle is a function that specifies, for a stock
	// identified by its label and category, the label text
	// to be printed on the plot (lbl), the style of the text (ts),
	// the horizontal and vertical offsets for printing the text (xOff and yOff),
	// the color of the fill for the bar representing the stock (c),
	// and the style of the outline of the bar representing the stock (ls).
	// The default function uses the default TextStyle, color and LineStyle
	// specified above for all stocks; zero horizontal and vertical offsets;
	// and the stock label as the text to be printed on the plot.
	StockStyle func(label string, category int) (lbl string, ts text.Style, xOff, yOff vg.Length, c color.Color, ls draw.LineStyle)

	// stocks arranges the stocks by category.
	// The first key is the category and the seond
	// key is the label.
	stocks map[int]map[string]*stock
}

// StockRange returns the minimum and maximum value on the value axis
// for the stock with the specified label and category.
func (s *Sankey) StockRange(label string, category int) (min, max float64, err error) {
	stk, ok := s.stocks[category][label]
	if !ok {
		return 0, 0, fmt.Errorf("plotter: sankey diagram does not contain stock with label=%s and category=%d", label, category)
	}
	return stk.min, stk.max, nil
}

// stock represents the amount of a stock and its plotting order.
type stock struct {
	// receptorValue and sourceValue are the totals of the values
	// of flows coming into and going out of this stock, respectively.
	receptorValue, sourceValue float64

	// label is the label of this stock, and category represents
	// its placement on the category axis. Together they make up a
	// unique identifier.
	label    string
	category int

	// order is the plotting order of this stock compared
	// to other stocks in the same category.
	order int

	// min represents the beginning of the plotting location
	// on the value axis.
	min float64

	// max is min plus the larger of receptorValue and sourceValue.
	max float64
}

// A Flow represents the amount of an entity flowing between two stocks.
type Flow struct {
	// SourceLabel and ReceptorLabel are the labels
	// of the stocks that originate and receive the flow,
	// respectively.
	SourceLabel, ReceptorLabel string

	// SourceCategory and ReceptorCategory define
	// the locations on the category axis of the stocks that
	// originate and receive the flow, respectively. The
	// SourceCategory must be a lower number than
	// the ReceptorCategory.
	SourceCategory, ReceptorCategory int

	// Value represents the magnitute of the flow.
	// It must be greater than or equal to zero.
	Value float64

	// Group specifies the group that a flow belongs
	// to. It is used in assigning styles to groups
	// and creating legends.
	Group string
}

// NewSankey creates a new Sankey diagram with the specified
// flows and stocks.
func NewSankey(flows ...Flow) (*Sankey, error) {
	var s Sankey

	s.stocks = make(map[int]map[string]*stock)

	s.flows = flows
	for i, f := range flows {
		// Here we make sure the stock categories are in the proper order.
		if f.SourceCategory >= f.ReceptorCategory {
			return nil, fmt.Errorf("plotter: Flow %d SourceCategory (%d) >= ReceptorCategory (%d)", i, f.SourceCategory, f.ReceptorCategory)
		}
		if f.Value < 0 {
			return nil, fmt.Errorf("plotter: Flow %d value (%g) < 0", i, f.Value)
		}

		// Here we initialize the stock holders.
		if _, ok := s.stocks[f.SourceCategory]; !ok {
			s.stocks[f.SourceCategory] = make(map[string]*stock)
		}
		if _, ok := s.stocks[f.ReceptorCategory]; !ok {
			s.stocks[f.ReceptorCategory] = make(map[string]*stock)
		}

		// Here we figure out the plotting order of the stocks.
		if _, ok := s.stocks[f.SourceCategory][f.SourceLabel]; !ok {
			s.stocks[f.SourceCategory][f.SourceLabel] = &stock{
				order:    len(s.stocks[f.SourceCategory]),
				label:    f.SourceLabel,
				category: f.SourceCategory,
			}
		}
		if _, ok := s.stocks[f.ReceptorCategory][f.ReceptorLabel]; !ok {
			s.stocks[f.ReceptorCategory][f.ReceptorLabel] = &stock{
				order:    len(s.stocks[f.ReceptorCategory]),
				label:    f.ReceptorLabel,
				category: f.ReceptorCategory,
			}
		}

		// Here we add the current value to the total value of the stocks
		s.stocks[f.SourceCategory][f.SourceLabel].sourceValue += f.Value
		s.stocks[f.ReceptorCategory][f.ReceptorLabel].receptorValue += f.Value
	}

	s.LineStyle = DefaultLineStyle

	s.TextStyle = text.Style{
		Font:     font.From(DefaultFont, DefaultFontSize),
		Rotation: math.Pi / 2,
		XAlign:   draw.XCenter,
		YAlign:   draw.YCenter,
		Handler:  plot.DefaultTextHandler,
	}
	s.StockBarWidth = s.TextStyle.FontExtents().Height * 1.15

	s.FlowStyle = func(_ string) (color.Color, draw.LineStyle) {
		return s.Color, s.LineStyle
	}

	s.StockStyle = func(label string, category int) (string, text.Style, vg.Length, vg.Length, color.Color, draw.LineStyle) {
		return label, s.TextStyle, 0, 0, s.Color, s.LineStyle
	}

	stocks := s.stockList()
	s.setStockRange(&stocks)

	return &s, nil
}

// Plot implements the plot.Plotter interface.
func (s *Sankey) Plot(c draw.Canvas, plt *plot.Plot) {
	trCat, trVal := plt.Transforms(&c)

	// sourceFlowPlaceholder and receptorFlowPlaceholder track
	// the current plotting location during
	// the plotting process.
	sourceFlowPlaceholder := make(map[*stock]float64, len(s.flows))
	receptorFlowPlaceholder := make(map[*stock]float64, len(s.flows))

	// Here we draw the flows.
	for _, f := range s.flows {
		startStock := s.stocks[f.SourceCategory][f.SourceLabel]
		endStock := s.stocks[f.ReceptorCategory][f.ReceptorLabel]
		catStart := trCat(float64(f.SourceCategory)) + s.StockBarWidth/2
		catEnd := trCat(float64(f.ReceptorCategory)) - s.StockBarWidth/2
		valStartLow := trVal(startStock.min + sourceFlowPlaceholder[startStock])
		valEndLow := trVal(endStock.min + receptorFlowPlaceholder[endStock])
		valStartHigh := trVal(startStock.min + sourceFlowPlaceholder[startStock] + f.Value)
		valEndHigh := trVal(endStock.min + receptorFlowPlaceholder[endStock] + f.Value)
		sourceFlowPlaceholder[startStock] += f.Value
		receptorFlowPlaceholder[endStock] += f.Value

		ptsLow := s.bezier(
			vg.Point{X: catStart, Y: valStartLow},
			vg.Point{X: catEnd, Y: valEndLow},
		)
		ptsHigh := s.bezier(
			vg.Point{X: catEnd, Y: valEndHigh},
			vg.Point{X: catStart, Y: valStartHigh},
		)

		color, lineStyle := s.FlowStyle(f.Group)

		// Here we fill the flow polygons.
		if color != nil {
			poly := c.ClipPolygonX(append(ptsLow, ptsHigh...))
			c.FillPolygon(color, poly)
		}

		// Here we draw the flow edges.
		outline := c.ClipLinesX(ptsLow)
		c.StrokeLines(lineStyle, outline...)
		outline = c.ClipLinesX(ptsHigh)
		c.StrokeLines(lineStyle, outline...)
	}

	// Here we draw the stocks.
	for _, stk := range s.stockList() {
		catLoc := trCat(float64(stk.category))
		if !c.ContainsX(catLoc) {
			continue
		}
		catMin, catMax := catLoc-s.StockBarWidth/2, catLoc+s.StockBarWidth/2
		valMin, valMax := trVal(stk.min), trVal(stk.max)

		label, textStyle, xOff, yOff, color, lineStyle := s.StockStyle(stk.label, stk.category)

		// Here we fill the stock bars.
		pts := []vg.Point{
			{X: catMin, Y: valMin},
			{X: catMin, Y: valMax},
			{X: catMax, Y: valMax},
			{X: catMax, Y: valMin},
		}
		if color != nil {
			c.FillPolygon(color, pts) // poly)
		}
		txtPt := vg.Point{X: (catMin+catMax)/2 + xOff, Y: (valMin+valMax)/2 + yOff}
		c.FillText(textStyle, txtPt, label)

		// Here we draw the bottom edge.
		pts = []vg.Point{
			{X: catMin, Y: valMin},
			{X: catMax, Y: valMin},
		}
		c.StrokeLines(lineStyle, pts)

		// Here we draw the top edge plus vertical edges where there are
		// no flows connected.
		pts = []vg.Point{
			{X: catMin, Y: valMax},
			{X: catMax, Y: valMax},
		}
		if stk.receptorValue < stk.sourceValue {
			y := trVal(stk.max - (stk.sourceValue - stk.receptorValue))
			pts = append([]vg.Point{{X: catMin, Y: y}}, pts...)
		} else if stk.sourceValue < stk.receptorValue {
			y := trVal(stk.max - (stk.receptorValue - stk.sourceValue))
			pts = append(pts, vg.Point{X: catMax, Y: y})
		}
		c.StrokeLines(lineStyle, pts)
	}
}

// stockList returns a sorted list of the stocks in the diagram.
func (s *Sankey) stockList() []*stock {
	var stocks []*stock
	for _, ss := range s.stocks {
		for _, sss := range ss {
			stocks = append(stocks, sss)
		}
	}
	sort.Sort(stockSorter(stocks))
	return stocks
}

// stockSorter is a wrapper for a list of *stocks that implements
// sort.Interface.
type stockSorter []*stock

func (s stockSorter) Len() int      { return len(s) }
func (s stockSorter) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s stockSorter) Less(i, j int) bool {
	if s[i].category != s[j].category {
		return s[i].category < s[j].category
	}
	if s[i].order != s[j].order {
		return s[i].order < s[j].order
	}
	panic(fmt.Errorf("plotter: can't sort stocks:\n%+v\n%+v", s[i], s[j]))
}

// setStockRange sets the minimum and maximum values of the stock plotting locations.
func (s *Sankey) setStockRange(stocks *[]*stock) {
	var cat int
	var min float64
	for _, stk := range *stocks {
		if stk.category != cat {
			min = 0
		}
		cat = stk.category
		stk.min = min
		if stk.sourceValue > stk.receptorValue {
			stk.max = stk.min + stk.sourceValue
		} else {
			stk.max = stk.min + stk.receptorValue
		}
		min = stk.max
	}
}

// bezier creates a bezier curve between the begin and end points.
func (s *Sankey) bezier(begin, end vg.Point) []vg.Point {
	// directionOffsetFrac is the fraction of the distance between begin.X and
	// end.X for the bezier control points.
	const directionOffsetFrac = 0.3
	inPts := []vg.Point{
		begin,
		{X: begin.X + (end.X-begin.X)*directionOffsetFrac, Y: begin.Y},
		{X: begin.X + (end.X-begin.X)*(1-directionOffsetFrac), Y: end.Y},
		end,
	}
	curve := bezier.New(inPts...)

	// nPoints is the number of points for bezier interpolation.
	const nPoints = 20
	outPts := make([]vg.Point, nPoints)
	curve.Curve(outPts)
	return outPts
}

// DataRange implements the plot.DataRanger interface.
func (s *Sankey) DataRange() (xmin, xmax, ymin, ymax float64) {
	catMin := math.Inf(1)
	catMax := math.Inf(-1)
	for cat := range s.stocks {
		c := float64(cat)
		catMin = math.Min(catMin, c)
		catMax = math.Max(catMax, c)
	}

	stocks := s.stockList()
	valMin := math.Inf(1)
	valMax := math.Inf(-1)
	for _, stk := range stocks {
		valMin = math.Min(valMin, stk.min)
		valMax = math.Max(valMax, stk.max)
	}
	return catMin, catMax, valMin, valMax
}

// GlyphBoxes implements the GlyphBoxer interface.
func (s *Sankey) GlyphBoxes(plt *plot.Plot) []plot.GlyphBox {
	stocks := s.stockList()
	boxes := make([]plot.GlyphBox, 0, len(s.flows)+len(stocks))

	for _, stk := range stocks {
		b1 := plot.GlyphBox{
			X: plt.X.Norm(float64(stk.category)),
			Y: plt.Y.Norm((stk.min + stk.max) / 2),
			Rectangle: vg.Rectangle{
				Min: vg.Point{X: -s.StockBarWidth / 2},
				Max: vg.Point{X: s.StockBarWidth / 2},
			},
		}
		label, textStyle, xOff, yOff, _, _ := s.StockStyle(stk.label, stk.category)
		rect := textStyle.Rectangle(label)
		rect.Min.X += xOff
		rect.Max.X += xOff
		rect.Min.Y += yOff
		rect.Max.Y += yOff
		b2 := plot.GlyphBox{
			X:         plt.X.Norm(float64(stk.category)),
			Y:         plt.Y.Norm((stk.min + stk.max) / 2),
			Rectangle: rect,
		}
		boxes = append(boxes, b1, b2)
	}
	return boxes
}

// Thumbnailers creates a group of objects that can be used to
// add legend entries for the different flow groups in this
// diagram, as well as the flow group labels that correspond to them.
func (s *Sankey) Thumbnailers() (legendLabels []string, thumbnailers []plot.Thumbnailer) {
	type empty struct{}
	flowGroups := make(map[string]empty)
	for _, f := range s.flows {
		flowGroups[f.Group] = empty{}
	}
	legendLabels = make([]string, len(flowGroups))
	thumbnailers = make([]plot.Thumbnailer, len(flowGroups))
	i := 0
	for g := range flowGroups {
		legendLabels[i] = g
		i++
	}
	sort.Strings(legendLabels)

	for i, g := range legendLabels {
		var thmb sankeyFlowThumbnailer
		thmb.Color, thmb.LineStyle = s.FlowStyle(g)
		thumbnailers[i] = plot.Thumbnailer(thmb)
	}
	return
}

// sankeyFlowThumbnailer implements the Thumbnailer interface
// for Sankey flow groups.
type sankeyFlowThumbnailer struct {
	draw.LineStyle
	color.Color
}

// Thumbnail fulfills the plot.Thumbnailer interface.
func (t sankeyFlowThumbnailer) Thumbnail(c *draw.Canvas) {
	// Here we draw the fill.
	pts := []vg.Point{
		{X: c.Min.X, Y: c.Min.Y},
		{X: c.Min.X, Y: c.Max.Y},
		{X: c.Max.X, Y: c.Max.Y},
		{X: c.Max.X, Y: c.Min.Y},
	}
	poly := c.ClipPolygonY(pts)
	c.FillPolygon(t.Color, poly)

	// Here we draw the upper border.
	pts = []vg.Point{
		{X: c.Min.X, Y: c.Max.Y},
		{X: c.Max.X, Y: c.Max.Y},
	}
	outline := c.ClipLinesY(pts)
	c.StrokeLines(t.LineStyle, outline...)

	// Here we draw the lower border.
	pts = []vg.Point{
		{X: c.Min.X, Y: c.Min.Y},
		{X: c.Max.X, Y: c.Min.Y},
	}
	outline = c.ClipLinesY(pts)
	c.StrokeLines(t.LineStyle, outline...)
}
