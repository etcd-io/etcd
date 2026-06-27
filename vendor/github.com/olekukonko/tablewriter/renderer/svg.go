package renderer

import (
	"fmt"
	"html"
	"io"
	"strings"

	"github.com/olekukonko/ll"

	"github.com/olekukonko/tablewriter/tw"
)

// SVGConfig holds configuration for the SVG renderer.
// Fields include font, colors, padding, and merge rendering options.
// Used to customize SVG output appearance and behavior.
type SVGConfig struct {
	FontFamily              string  // e.g., "Arial, sans-serif"
	FontSize                float64 // Base font size in SVG units
	LineHeightFactor        float64 // Factor for line height (e.g., 1.2)
	Padding                 float64 // Padding inside cells
	StrokeWidth             float64 // Line width for borders
	StrokeColor             string  // Color for strokes (e.g., "black")
	HeaderBG                string  // Background color for header
	RowBG                   string  // Background color for rows
	RowAltBG                string  // Alternating row background color
	FooterBG                string  // Background color for footer
	HeaderColor             string  // Text color for header
	RowColor                string  // Text color for rows
	FooterColor             string  // Text color for footer
	ApproxCharWidthFactor   float64 // Char width relative to FontSize
	MinColWidth             float64 // Minimum column width
	RenderTWConfigOverrides bool    // Override SVG alignments with tablewriter
	Debug                   bool    // Enable debug logging
	ScaleFactor             float64 // Scaling factor for SVG
}

// SVG implements tw.Renderer for SVG output.
// Manages SVG element generation and merge tracking.
type SVG struct {
	config SVGConfig
	trace  []string

	allVisualLineData [][][]string      // [section][line][cell]
	allVisualLineCtx  [][]tw.Formatting // [section][line]Formatting

	maxCols             int
	calculatedColWidths []float64
	svgElements         strings.Builder
	currentY            float64
	dataRowCounter      int
	vMergeTrack         map[int]int // Tracks vertical merge spans
	numVisualRowsDrawn  int
	logger              *ll.Logger
	w                   io.Writer
}

const (
	sectionTypeHeader = 0
	sectionTypeRow    = 1
	sectionTypeFooter = 2
)

// NewSVG creates a new SVG renderer with configuration.
// Parameter configs provides optional SVGConfig; defaults used if empty.
// Returns a configured SVG instance.
func NewSVG(configs ...SVGConfig) *SVG {
	cfg := SVGConfig{
		FontFamily:              "sans-serif",
		FontSize:                12.0,
		LineHeightFactor:        1.4,
		Padding:                 5.0,
		StrokeWidth:             1.0,
		StrokeColor:             "black",
		HeaderBG:                "#F0F0F0",
		RowBG:                   "white",
		RowAltBG:                "#F9F9F9",
		FooterBG:                "#F0F0F0",
		HeaderColor:             "black",
		RowColor:                "black",
		FooterColor:             "black",
		ApproxCharWidthFactor:   0.6,
		MinColWidth:             30.0,
		ScaleFactor:             1.0,
		RenderTWConfigOverrides: true,
		Debug:                   false,
	}
	if len(configs) > 0 {
		userCfg := configs[0]
		if userCfg.FontFamily != tw.Empty {
			cfg.FontFamily = userCfg.FontFamily
		}
		if userCfg.FontSize > 0 {
			cfg.FontSize = userCfg.FontSize
		}
		if userCfg.LineHeightFactor > 0 {
			cfg.LineHeightFactor = userCfg.LineHeightFactor
		}
		if userCfg.Padding >= 0 {
			cfg.Padding = userCfg.Padding
		}
		if userCfg.StrokeWidth > 0 {
			cfg.StrokeWidth = userCfg.StrokeWidth
		}
		if userCfg.StrokeColor != tw.Empty {
			cfg.StrokeColor = userCfg.StrokeColor
		}
		if userCfg.HeaderBG != tw.Empty {
			cfg.HeaderBG = userCfg.HeaderBG
		}
		if userCfg.RowBG != tw.Empty {
			cfg.RowBG = userCfg.RowBG
		}
		cfg.RowAltBG = userCfg.RowAltBG
		if userCfg.FooterBG != tw.Empty {
			cfg.FooterBG = userCfg.FooterBG
		}
		if userCfg.HeaderColor != tw.Empty {
			cfg.HeaderColor = userCfg.HeaderColor
		}
		if userCfg.RowColor != tw.Empty {
			cfg.RowColor = userCfg.RowColor
		}
		if userCfg.FooterColor != tw.Empty {
			cfg.FooterColor = userCfg.FooterColor
		}
		if userCfg.ApproxCharWidthFactor > 0 {
			cfg.ApproxCharWidthFactor = userCfg.ApproxCharWidthFactor
		}
		if userCfg.MinColWidth >= 0 {
			cfg.MinColWidth = userCfg.MinColWidth
		}
		cfg.RenderTWConfigOverrides = userCfg.RenderTWConfigOverrides
		cfg.Debug = userCfg.Debug
	}
	r := &SVG{
		config:            cfg,
		trace:             make([]string, 0, 50),
		allVisualLineData: make([][][]string, 3),
		allVisualLineCtx:  make([][]tw.Formatting, 3),
		vMergeTrack:       make(map[int]int),
		logger:            ll.New("svg").Disable(),
	}
	for i := 0; i < 3; i++ {
		r.allVisualLineData[i] = make([][]string, 0)
		r.allVisualLineCtx[i] = make([]tw.Formatting, 0)
	}
	return r
}

// calculateAllColumnWidths computes column widths based on content and merges.
// Uses content length and merge spans; handles horizontal merges by distributing width.
func (s *SVG) calculateAllColumnWidths() {
	s.debug("Calculating column widths")
	tempMaxCols := 0
	for sectionIdx := 0; sectionIdx < 3; sectionIdx++ {
		for lineIdx, lineCtx := range s.allVisualLineCtx[sectionIdx] {
			if lineCtx.Row.Current != nil {
				visualColCount := 0
				for colIdx := 0; colIdx < len(lineCtx.Row.Current); {
					cellCtx := lineCtx.Row.Current[colIdx]
					if cellCtx.Merge.Horizontal.Present && !cellCtx.Merge.Horizontal.Start {
						colIdx++ // Skip non-start merged cells
						continue
					}
					visualColCount++
					span := 1
					if cellCtx.Merge.Horizontal.Present && cellCtx.Merge.Horizontal.Start {
						span = cellCtx.Merge.Horizontal.Span
						if span <= 0 {
							span = 1
						}
					}
					colIdx += span
				}
				s.debug("Section %d, line %d: Visual columns = %d", sectionIdx, lineIdx, visualColCount)
				if visualColCount > tempMaxCols {
					tempMaxCols = visualColCount
				}
			} else if lineIdx < len(s.allVisualLineData[sectionIdx]) {
				if rawDataLen := len(s.allVisualLineData[sectionIdx][lineIdx]); rawDataLen > tempMaxCols {
					tempMaxCols = rawDataLen
				}
			}
		}
	}
	s.maxCols = tempMaxCols
	s.debug("Max columns: %d", s.maxCols)
	if s.maxCols == 0 {
		s.calculatedColWidths = []float64{}
		return
	}
	s.calculatedColWidths = make([]float64, s.maxCols)
	for i := range s.calculatedColWidths {
		s.calculatedColWidths[i] = s.config.MinColWidth
	}

	// Structure to track max width for each merge group
	type mergeKey struct {
		startCol int
		span     int
	}
	maxMergeWidths := make(map[mergeKey]float64)

	processSectionForWidth := func(sectionIdx int) {
		for lineIdx, visualLineData := range s.allVisualLineData[sectionIdx] {
			if lineIdx >= len(s.allVisualLineCtx[sectionIdx]) {
				s.debug("Warning: Missing context for section %d line %d", sectionIdx, lineIdx)
				continue
			}
			lineCtx := s.allVisualLineCtx[sectionIdx][lineIdx]
			currentTableCol := 0
			currentVisualCol := 0
			for currentVisualCol < len(visualLineData) && currentTableCol < s.maxCols {
				cellContent := visualLineData[currentVisualCol]
				cellCtx := tw.CellContext{}
				if lineCtx.Row.Current != nil {
					if c, ok := lineCtx.Row.Current[currentTableCol]; ok {
						cellCtx = c
					}
				}
				hSpan := 1
				if cellCtx.Merge.Horizontal.Present {
					if cellCtx.Merge.Horizontal.Start {
						hSpan = cellCtx.Merge.Horizontal.Span
						if hSpan <= 0 {
							hSpan = 1
						}
					} else {
						currentTableCol++
						continue
					}
				}
				textPixelWidth := s.estimateTextWidth(cellContent)
				contentAndPaddingWidth := textPixelWidth + (2 * s.config.Padding)
				if hSpan == 1 {
					if currentTableCol < len(s.calculatedColWidths) && contentAndPaddingWidth > s.calculatedColWidths[currentTableCol] {
						s.calculatedColWidths[currentTableCol] = contentAndPaddingWidth
					}
				} else {
					totalMergedWidth := contentAndPaddingWidth + (float64(hSpan-1) * s.config.Padding * 2)
					if totalMergedWidth < s.config.MinColWidth*float64(hSpan) {
						totalMergedWidth = s.config.MinColWidth * float64(hSpan)
					}
					if currentTableCol < len(s.calculatedColWidths) {
						key := mergeKey{currentTableCol, hSpan}
						if currentWidth, ok := maxMergeWidths[key]; ok {
							if totalMergedWidth > currentWidth {
								maxMergeWidths[key] = totalMergedWidth
							}
						} else {
							maxMergeWidths[key] = totalMergedWidth
						}
						s.debug("Horizontal merge at col %d, span %d: Total width %.2f", currentTableCol, hSpan, totalMergedWidth)
					}
				}
				currentTableCol += hSpan
				currentVisualCol++
			}
		}
	}
	processSectionForWidth(sectionTypeHeader)
	processSectionForWidth(sectionTypeRow)
	processSectionForWidth(sectionTypeFooter)

	// Apply maximum widths for merged cells
	for key, width := range maxMergeWidths {
		s.calculatedColWidths[key.startCol] = width
		for i := 1; i < key.span && (key.startCol+i) < len(s.calculatedColWidths); i++ {
			s.calculatedColWidths[key.startCol+i] = 0
		}
	}

	for i := range s.calculatedColWidths {
		if s.calculatedColWidths[i] < s.config.MinColWidth && s.calculatedColWidths[i] != 0 {
			s.calculatedColWidths[i] = s.config.MinColWidth
		}
	}
	s.debug("Column widths: %v", s.calculatedColWidths)
}

// Close finalizes SVG rendering and writes output.
// Parameter w is the output w.
// Returns an error if writing fails.
func (s *SVG) Close() error {
	s.debug("Finalizing SVG output")
	s.calculateAllColumnWidths()
	s.renderBufferedData()
	if s.numVisualRowsDrawn == 0 && s.maxCols == 0 {
		fmt.Fprintf(s.w, `<svg xmlns="http://www.w3.org/2000/svg" width="%.2f" height="%.2f"></svg>`, s.config.StrokeWidth*2, s.config.StrokeWidth*2)
		return nil
	}
	totalWidth := s.config.StrokeWidth
	if len(s.calculatedColWidths) > 0 {
		for _, cw := range s.calculatedColWidths {
			colWidth := cw
			if colWidth <= 0 {
				colWidth = s.config.MinColWidth
			}
			totalWidth += colWidth + s.config.StrokeWidth
		}
	} else if s.maxCols > 0 {
		for i := 0; i < s.maxCols; i++ {
			totalWidth += s.config.MinColWidth + s.config.StrokeWidth
		}
	} else {
		totalWidth = s.config.StrokeWidth * 2
	}
	totalHeight := s.currentY
	singleVisualRowHeight := s.config.FontSize*s.config.LineHeightFactor + (2 * s.config.Padding)
	if s.numVisualRowsDrawn == 0 {
		if s.maxCols > 0 {
			totalHeight = s.config.StrokeWidth + singleVisualRowHeight + s.config.StrokeWidth
		} else {
			totalHeight = s.config.StrokeWidth * 2
		}
	}
	fmt.Fprintf(s.w, `<svg xmlns="http://www.w3.org/2000/svg" width="%.2f" height="%.2f" font-family="%s" font-size="%.2f">`,
		totalWidth, totalHeight, html.EscapeString(s.config.FontFamily), s.config.FontSize)
	fmt.Fprintln(s.w)
	fmt.Fprintln(s.w, "<style>text { stroke: none; }</style>")
	if _, err := io.WriteString(s.w, s.svgElements.String()); err != nil {
		fmt.Fprintln(s.w, `</svg>`)
		return fmt.Errorf("failed to write SVG elements: %w", err)
	}
	if s.maxCols > 0 || s.numVisualRowsDrawn > 0 {
		fmt.Fprintf(s.w, `  <g class="table-borders" stroke="%s" stroke-width="%.2f" stroke-linecap="square">`,
			html.EscapeString(s.config.StrokeColor), s.config.StrokeWidth)
		fmt.Fprintln(s.w)
		yPos := s.config.StrokeWidth / 2.0
		borderRowsToDraw := s.numVisualRowsDrawn
		if borderRowsToDraw == 0 && s.maxCols > 0 {
			borderRowsToDraw = 1
		}
		lineStartX := s.config.StrokeWidth / 2.0
		lineEndX := s.config.StrokeWidth / 2.0
		for _, width := range s.calculatedColWidths {
			lineEndX += width + s.config.StrokeWidth
		}
		for i := 0; i <= borderRowsToDraw; i++ {
			fmt.Fprintf(s.w, `    <line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" />%s`,
				lineStartX, yPos, lineEndX, yPos, "\n")
			if i < borderRowsToDraw {
				yPos += singleVisualRowHeight + s.config.StrokeWidth
			}
		}
		xPos := s.config.StrokeWidth / 2.0
		borderLineStartY := s.config.StrokeWidth / 2.0
		borderLineEndY := totalHeight - (s.config.StrokeWidth / 2.0)
		for visualColIdx := 0; visualColIdx <= s.maxCols; visualColIdx++ {
			fmt.Fprintf(s.w, `    <line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" />%s`,
				xPos, borderLineStartY, xPos, borderLineEndY, "\n")
			if visualColIdx < s.maxCols {
				colWidth := s.config.MinColWidth
				if visualColIdx < len(s.calculatedColWidths) && s.calculatedColWidths[visualColIdx] > 0 {
					colWidth = s.calculatedColWidths[visualColIdx]
				}
				xPos += colWidth + s.config.StrokeWidth
			}
		}
		fmt.Fprintln(s.w, "  </g>")
	}
	fmt.Fprintln(s.w, `</svg>`)
	return nil
}

// Config returns the renderer's configuration.
// No parameters are required.
// Returns a Rendition with border and debug settings.
func (s *SVG) Config() tw.Rendition {
	return tw.Rendition{
		Borders:   tw.Border{Left: tw.On, Right: tw.On, Top: tw.On, Bottom: tw.On},
		Settings:  tw.Settings{},
		Streaming: false,
	}
}

// Debug returns the renderer's debug trace.
// No parameters are required.
// Returns a slice of debug messages.
func (s *SVG) Debug() []string {
	return s.trace
}

// estimateTextWidth estimates text width in SVG units.
// Parameter text is the input string to measure.
// Returns the estimated width based on font size and char factor.
func (s *SVG) estimateTextWidth(text string) float64 {
	runeCount := float64(len([]rune(text)))
	return runeCount * s.config.FontSize * s.config.ApproxCharWidthFactor
}

// Footer buffers footer lines for SVG rendering.
// Parameters include w (w), footers (lines), and ctx (formatting).
// No return value; stores data for later rendering.
func (s *SVG) Footer(footers [][]string, ctx tw.Formatting) {
	s.debug("Buffering %d footer lines", len(footers))
	for i, line := range footers {
		currentCtx := ctx
		currentCtx.IsSubRow = (i > 0)
		s.storeVisualLine(sectionTypeFooter, line, currentCtx)
	}
}

// getSVGAnchorFromTW maps tablewriter alignment to SVG text-anchor.
// Parameter align is the tablewriter alignment setting.
// Returns the corresponding SVG text-anchor value or empty string.
func (s *SVG) getSVGAnchorFromTW(align tw.Align) string {
	switch align {
	case tw.AlignLeft:
		return "start"
	case tw.AlignCenter:
		return "middle"
	case tw.AlignRight:
		return "end"
	case tw.AlignNone, tw.Skip:
		return tw.Empty
	}
	return tw.Empty
}

// Header buffers header lines for SVG rendering.
// Parameters include w (w), headers (lines), and ctx (formatting).
// No return value; stores data for later rendering.
func (s *SVG) Header(headers [][]string, ctx tw.Formatting) {
	s.debug("Buffering %d header lines", len(headers))
	for i, line := range headers {
		currentCtx := ctx
		currentCtx.IsSubRow = i > 0
		s.storeVisualLine(sectionTypeHeader, line, currentCtx)
	}
}

// Line handles border rendering (ignored in SVG renderer).
// Parameters include w (w) and ctx (formatting).
// No return value; SVG borders are drawn in Close.
func (s *SVG) Line(ctx tw.Formatting) {
	s.debug("Line rendering ignored")
}

// padLineSVG pads a line to the specified column count.
// Parameters include line (input strings) and numCols (target length).
// Returns the padded line with empty strings as needed.
func padLineSVG(line []string, numCols int) []string {
	if numCols <= 0 {
		return []string{}
	}
	currentLen := len(line)
	if currentLen == numCols {
		return line
	}
	if currentLen > numCols {
		return line[:numCols]
	}
	padded := make([]string, numCols)
	copy(padded, line)
	return padded
}

// renderBufferedData renders all buffered lines to SVG elements.
// No parameters are required.
// No return value; populates svgElements buffer.
func (s *SVG) renderBufferedData() {
	s.debug("Rendering buffered data")
	s.currentY = s.config.StrokeWidth
	s.dataRowCounter = 0
	s.vMergeTrack = make(map[int]int)
	s.numVisualRowsDrawn = 0
	renderSection := func(sectionIdx int, position tw.Position) {
		for visualLineIdx, visualLineData := range s.allVisualLineData[sectionIdx] {
			if visualLineIdx >= len(s.allVisualLineCtx[sectionIdx]) {
				s.debug("Error: Missing context for section %d line %d", sectionIdx, visualLineIdx)
				continue
			}
			s.renderVisualLine(visualLineData, s.allVisualLineCtx[sectionIdx][visualLineIdx], position)
		}
	}
	renderSection(sectionTypeHeader, tw.Header)
	renderSection(sectionTypeRow, tw.Row)
	renderSection(sectionTypeFooter, tw.Footer)
}

// renderVisualLine renders a single visual line as SVG elements.
// Parameters include lineData (cell content), ctx (formatting), and position (section type).
// No return value; handles horizontal and vertical merges.
func (s *SVG) renderVisualLine(visualLineData []string, ctx tw.Formatting, position tw.Position) {
	if s.maxCols == 0 || len(s.calculatedColWidths) == 0 {
		s.debug("Skipping line rendering: maxCols=%d, widths=%d", s.maxCols, len(s.calculatedColWidths))
		return
	}
	s.numVisualRowsDrawn++
	s.debug("Rendering visual row %d", s.numVisualRowsDrawn)
	singleVisualRowHeight := s.config.FontSize*s.config.LineHeightFactor + (2 * s.config.Padding)
	bgColor := tw.Empty
	textColor := tw.Empty
	defaultTextAnchor := "start"
	switch position {
	case tw.Header:
		bgColor = s.config.HeaderBG
		textColor = s.config.HeaderColor
		defaultTextAnchor = "middle"
	case tw.Footer:
		bgColor = s.config.FooterBG
		textColor = s.config.FooterColor
		defaultTextAnchor = "end"
	default:
		textColor = s.config.RowColor
		if !ctx.IsSubRow {
			if s.config.RowAltBG != tw.Empty && s.dataRowCounter%2 != 0 {
				bgColor = s.config.RowAltBG
			} else {
				bgColor = s.config.RowBG
			}
			s.dataRowCounter++
		} else {
			parentDataRowStripeIndex := max(s.dataRowCounter-1, 0)
			if s.config.RowAltBG != tw.Empty && parentDataRowStripeIndex%2 != 0 {
				bgColor = s.config.RowAltBG
			} else {
				bgColor = s.config.RowBG
			}
		}
	}
	currentX := s.config.StrokeWidth
	currentVisualCellIdx := 0
	for tableColIdx := 0; tableColIdx < s.maxCols; {
		if tableColIdx >= len(s.calculatedColWidths) {
			s.debug("Table Col %d out of bounds for widths", tableColIdx)
			tableColIdx++
			continue
		}
		if remainingVSpan, isMerging := s.vMergeTrack[tableColIdx]; isMerging && remainingVSpan > 1 {
			s.vMergeTrack[tableColIdx]--
			if s.vMergeTrack[tableColIdx] <= 1 {
				delete(s.vMergeTrack, tableColIdx)
			}
			currentX += s.calculatedColWidths[tableColIdx] + s.config.StrokeWidth
			tableColIdx++
			continue
		}
		cellContentFromVisualLine := tw.Empty
		if currentVisualCellIdx < len(visualLineData) {
			cellContentFromVisualLine = visualLineData[currentVisualCellIdx]
		}
		cellCtx := tw.CellContext{}
		if ctx.Row.Current != nil {
			if c, ok := ctx.Row.Current[tableColIdx]; ok {
				cellCtx = c
			}
		}
		textToRender := cellContentFromVisualLine
		if cellCtx.Data != tw.Empty {
			if !((cellCtx.Merge.Vertical.Present && !cellCtx.Merge.Vertical.Start) || (cellCtx.Merge.Hierarchical.Present && !cellCtx.Merge.Hierarchical.Start)) {
				textToRender = cellCtx.Data
			} else {
				textToRender = tw.Empty
			}
		} else if (cellCtx.Merge.Vertical.Present && !cellCtx.Merge.Vertical.Start) || (cellCtx.Merge.Hierarchical.Present && !cellCtx.Merge.Hierarchical.Start) {
			textToRender = tw.Empty
		}
		hSpan := 1
		if cellCtx.Merge.Horizontal.Present {
			if cellCtx.Merge.Horizontal.Start {
				hSpan = cellCtx.Merge.Horizontal.Span
				if hSpan <= 0 {
					hSpan = 1
				}
			} else {
				currentX += s.calculatedColWidths[tableColIdx] + s.config.StrokeWidth
				tableColIdx++
				continue
			}
		}
		vSpan := 1
		isVSpanStart := false
		if cellCtx.Merge.Vertical.Present && cellCtx.Merge.Vertical.Start {
			vSpan = cellCtx.Merge.Vertical.Span
			isVSpanStart = true
		} else if cellCtx.Merge.Hierarchical.Present && cellCtx.Merge.Hierarchical.Start {
			vSpan = cellCtx.Merge.Hierarchical.Span
			isVSpanStart = true
		}
		if vSpan <= 0 {
			vSpan = 1
		}
		rectWidth := 0.0
		for hs := 0; hs < hSpan && (tableColIdx+hs) < s.maxCols; hs++ {
			if (tableColIdx + hs) < len(s.calculatedColWidths) {
				rectWidth += s.calculatedColWidths[tableColIdx+hs]
			} else {
				rectWidth += s.config.MinColWidth
			}
		}
		if hSpan > 1 {
			rectWidth += float64(hSpan-1) * s.config.StrokeWidth
		}
		if rectWidth <= 0 {
			tableColIdx += hSpan
			if hSpan > 0 {
				currentVisualCellIdx++
			}
			continue
		}
		rectHeight := singleVisualRowHeight
		if isVSpanStart && vSpan > 1 {
			rectHeight = float64(vSpan)*singleVisualRowHeight + float64(vSpan-1)*s.config.StrokeWidth
			for hs := 0; hs < hSpan && (tableColIdx+hs) < s.maxCols; hs++ {
				s.vMergeTrack[tableColIdx+hs] = vSpan
			}
			s.debug("Vertical merge at col %d, span %d, height %.2f", tableColIdx, vSpan, rectHeight)
		} else if remainingVSpan, isMerging := s.vMergeTrack[tableColIdx]; isMerging && remainingVSpan > 1 {
			rectHeight = singleVisualRowHeight
			textToRender = tw.Empty
		}
		fmt.Fprintf(&s.svgElements, `  <rect x="%.2f" y="%.2f" width="%.2f" height="%.2f" fill="%s"/>%s`,
			currentX, s.currentY, rectWidth, rectHeight, html.EscapeString(bgColor), "\n")
		cellTextAnchor := defaultTextAnchor
		if s.config.RenderTWConfigOverrides {
			if al := s.getSVGAnchorFromTW(cellCtx.Align); al != tw.Empty {
				cellTextAnchor = al
			}
		}
		textX := currentX + s.config.Padding
		switch cellTextAnchor {
		case "middle":
			textX = currentX + s.config.Padding + (rectWidth-2*s.config.Padding)/2.0
		case "end":
			textX = currentX + rectWidth - s.config.Padding
		}
		textY := s.currentY + rectHeight/2.0
		escapedCell := html.EscapeString(textToRender)
		fmt.Fprintf(&s.svgElements, `  <text x="%.2f" y="%.2f" fill="%s" text-anchor="%s" dominant-baseline="middle">%s</text>%s`,
			textX, textY, html.EscapeString(textColor), cellTextAnchor, escapedCell, "\n")
		currentX += rectWidth + s.config.StrokeWidth
		tableColIdx += hSpan
		currentVisualCellIdx++
	}
	s.currentY += singleVisualRowHeight + s.config.StrokeWidth
}

// Reset clears the renderer's internal state.
// No parameters are required.
// No return value; prepares for new rendering.
func (s *SVG) Reset() {
	s.debug("Resetting state")
	s.trace = make([]string, 0, 50)
	for i := 0; i < 3; i++ {
		s.allVisualLineData[i] = s.allVisualLineData[i][:0]
		s.allVisualLineCtx[i] = s.allVisualLineCtx[i][:0]
	}
	s.maxCols = 0
	s.calculatedColWidths = nil
	s.svgElements.Reset()
	s.currentY = 0
	s.dataRowCounter = 0
	s.vMergeTrack = make(map[int]int)
	s.numVisualRowsDrawn = 0
}

// Row buffers a row line for SVG rendering.
// Parameters include w (w), rowLine (cells), and ctx (formatting).
// No return value; stores data for later rendering.
func (s *SVG) Row(rowLine []string, ctx tw.Formatting) {
	s.debug("Buffering row line, IsSubRow: %v", ctx.IsSubRow)
	s.storeVisualLine(sectionTypeRow, rowLine, ctx)
}

func (s *SVG) Logger(logger *ll.Logger) {
	s.logger = logger.Namespace("svg")
}

// Start initializes SVG rendering.
// Parameter w is the output w.
// Returns nil; prepares internal state.
func (s *SVG) Start(w io.Writer) error {
	s.w = w
	s.debug("Starting SVG rendering")
	s.Reset()
	return nil
}

// debug logs a message if debugging is enabled.
// Parameters include format string and variadic arguments.
// No return value; appends to trace.
func (s *SVG) debug(format string, a ...interface{}) {
	if s.config.Debug {
		msg := fmt.Sprintf(format, a...)
		s.trace = append(s.trace, "[SVG] "+msg)
	}
}

// storeVisualLine stores a visual line for rendering.
// Parameters include sectionIdx, lineData (cells), and ctx (formatting).
// No return value; buffers data and context.
func (s *SVG) storeVisualLine(sectionIdx int, lineData []string, ctx tw.Formatting) {
	copiedLineData := make([]string, len(lineData))
	copy(copiedLineData, lineData)
	s.allVisualLineData[sectionIdx] = append(s.allVisualLineData[sectionIdx], copiedLineData)
	s.allVisualLineCtx[sectionIdx] = append(s.allVisualLineCtx[sectionIdx], ctx)
	hasCurrent := ctx.Row.Current != nil
	s.debug("Stored line in section %d, has context: %v", sectionIdx, hasCurrent)
}
