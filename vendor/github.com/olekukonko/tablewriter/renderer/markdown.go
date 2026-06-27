package renderer

import (
	"io"
	"strings"

	"github.com/olekukonko/ll"
	"github.com/olekukonko/tablewriter/pkg/twwidth"

	"github.com/olekukonko/tablewriter/tw"
)

// Markdown renders tables in Markdown format with customizable settings.
type Markdown struct {
	config    tw.Rendition // Rendering configuration
	logger    *ll.Logger   // Debug trace messages
	alignment tw.Alignment // alias of []tw.Align
	w         io.Writer
}

// NewMarkdown initializes a Markdown renderer with defaults tailored for Markdown (e.g., pipes, header separator).
// Only the first config is used if multiple are provided.
func NewMarkdown(configs ...tw.Rendition) *Markdown {
	cfg := defaultBlueprint()
	// Configure Markdown-specific defaults
	cfg.Symbols = tw.NewSymbols(tw.StyleMarkdown)
	cfg.Borders = tw.Border{Left: tw.On, Right: tw.On, Top: tw.Off, Bottom: tw.Off}
	cfg.Settings.Separators.BetweenColumns = tw.On
	cfg.Settings.Separators.BetweenRows = tw.Off
	cfg.Settings.Lines.ShowHeaderLine = tw.On
	cfg.Settings.Lines.ShowTop = tw.Off
	cfg.Settings.Lines.ShowBottom = tw.Off
	cfg.Settings.Lines.ShowFooterLine = tw.Off
	// cfg.Settings.TrimWhitespace = tw.On

	// Apply user overrides
	if len(configs) > 0 {
		cfg = mergeMarkdownConfig(cfg, configs[0])
	}
	return &Markdown{config: cfg, logger: ll.New("markdown").Disable()}
}

// mergeMarkdownConfig combines user-provided config with Markdown defaults, enforcing Markdown-specific settings.
func mergeMarkdownConfig(defaults, overrides tw.Rendition) tw.Rendition {
	if overrides.Borders.Left != 0 {
		defaults.Borders.Left = overrides.Borders.Left
	}
	if overrides.Borders.Right != 0 {
		defaults.Borders.Right = overrides.Borders.Right
	}
	if overrides.Symbols != nil {
		defaults.Symbols = overrides.Symbols
	}
	defaults.Settings = mergeSettings(defaults.Settings, overrides.Settings)
	// Enforce Markdown requirements
	defaults.Settings.Lines.ShowHeaderLine = tw.On
	defaults.Settings.Separators.BetweenColumns = tw.On
	// defaults.Settings.TrimWhitespace = tw.On
	return defaults
}

func (m *Markdown) Logger(logger *ll.Logger) {
	m.logger = logger.Namespace("markdown")
}

// Config returns the renderer's current configuration.
func (m *Markdown) Config() tw.Rendition {
	return m.config
}

// Header renders the Markdown table header and its separator line.
func (m *Markdown) Header(headers [][]string, ctx tw.Formatting) {
	m.resolveAlignment(ctx)
	if len(headers) == 0 || len(headers[0]) == 0 {
		m.logger.Debug("Header: No headers to render")
		return
	}
	m.logger.Debugf("Rendering header with %d lines, widths=%v, current=%v, next=%v", len(headers), ctx.Row.Widths, ctx.Row.Current, ctx.Row.Next)

	// Render header content
	m.renderMarkdownLine(headers[0], ctx, false)

	// Render separator if enabled
	if m.config.Settings.Lines.ShowHeaderLine.Enabled() {
		sepCtx := ctx
		sepCtx.Row.Widths = ctx.Row.Widths
		sepCtx.Row.Current = ctx.Row.Current
		sepCtx.Row.Previous = ctx.Row.Current
		sepCtx.IsSubRow = true
		m.renderMarkdownLine(nil, sepCtx, true)
	}
}

// Row renders a Markdown table data row.
func (m *Markdown) Row(row []string, ctx tw.Formatting) {
	m.resolveAlignment(ctx)
	m.logger.Debugf("Rendering row with data=%v, widths=%v, previous=%v, current=%v, next=%v", row, ctx.Row.Widths, ctx.Row.Previous, ctx.Row.Current, ctx.Row.Next)
	m.renderMarkdownLine(row, ctx, false)
}

// Footer renders the Markdown table footer.
func (m *Markdown) Footer(footers [][]string, ctx tw.Formatting) {
	m.resolveAlignment(ctx)
	if len(footers) == 0 || len(footers[0]) == 0 {
		m.logger.Debug("Footer: No footers to render")
		return
	}
	m.logger.Debugf("Rendering footer with %d lines, widths=%v, previous=%v, current=%v, next=%v",
		len(footers), ctx.Row.Widths, ctx.Row.Previous, ctx.Row.Current, ctx.Row.Next)
	m.renderMarkdownLine(footers[0], ctx, false)
}

// Line is a no-op for Markdown, as only the header separator is rendered (handled by Header).
func (m *Markdown) Line(ctx tw.Formatting) {
	m.logger.Debugf("Line: Generic Line call received (pos: %s, loc: %s). Markdown ignores these.", ctx.Row.Position, ctx.Row.Location)
}

// Reset clears the renderer's internal state, including debug traces.
func (m *Markdown) Reset() {
	m.logger.Info("Reset: Cleared debug trace")
}

func (m *Markdown) Start(w io.Writer) error {
	m.w = w
	m.logger.Warn("Markdown.Start() called (no-op).")
	return nil
}

func (m *Markdown) Close() error {
	m.logger.Warn("Markdown.Close() called (no-op).")
	return nil
}

func (m *Markdown) resolveAlignment(ctx tw.Formatting) tw.Alignment {
	if len(m.alignment) != 0 {
		return m.alignment
	}

	// get total columns
	total := len(ctx.Row.Current)

	// build default alignment
	for i := 0; i < total; i++ {
		m.alignment = append(m.alignment, tw.AlignNone) // Default to AlignNone
	}

	// add per column alignment if it exists
	for i := 0; i < total; i++ {
		m.alignment[i] = ctx.Row.Current[i].Align
	}

	m.logger.Debugf(" → Align Resolved %s", m.alignment)
	return m.alignment
}

// formatCell formats a Markdown cell's content with padding and alignment, ensuring at least 3 characters wide.
func (m *Markdown) formatCell(content string, width int, align tw.Align, padding tw.Padding) string {
	// if m.config.Settings.TrimWhitespace.Enabled() {
	//	content = strings.TrimSpace(content)
	//}
	contentVisualWidth := twwidth.Width(content)

	// Use specified padding characters or default to spaces
	padLeftChar := padding.Left
	if padLeftChar == tw.Empty {
		padLeftChar = tw.Space
	}
	padRightChar := padding.Right
	if padRightChar == tw.Empty {
		padRightChar = tw.Space
	}

	// Calculate padding widths
	padLeftCharWidth := twwidth.Width(padLeftChar)
	padRightCharWidth := twwidth.Width(padRightChar)
	minWidth := tw.Max(3, contentVisualWidth+padLeftCharWidth+padRightCharWidth)
	targetWidth := tw.Max(width, minWidth)

	// Calculate padding
	totalPaddingNeeded := max(targetWidth-contentVisualWidth, 0)

	var leftPadStr, rightPadStr string
	switch align {
	case tw.AlignRight:
		leftPadCount := tw.Max(0, totalPaddingNeeded-padRightCharWidth)
		rightPadCount := totalPaddingNeeded - leftPadCount
		leftPadStr = strings.Repeat(padLeftChar, leftPadCount)
		rightPadStr = strings.Repeat(padRightChar, rightPadCount)
	case tw.AlignCenter:
		leftPadCount := totalPaddingNeeded / 2
		rightPadCount := totalPaddingNeeded - leftPadCount
		if leftPadCount < padLeftCharWidth && totalPaddingNeeded >= padLeftCharWidth+padRightCharWidth {
			leftPadCount = padLeftCharWidth
			rightPadCount = totalPaddingNeeded - leftPadCount
		}
		if rightPadCount < padRightCharWidth && totalPaddingNeeded >= padLeftCharWidth+padRightCharWidth {
			rightPadCount = padRightCharWidth
			leftPadCount = totalPaddingNeeded - rightPadCount
		}
		leftPadStr = strings.Repeat(padLeftChar, leftPadCount)
		rightPadStr = strings.Repeat(padRightChar, rightPadCount)
	default: // AlignLeft
		rightPadCount := tw.Max(0, totalPaddingNeeded-padLeftCharWidth)
		leftPadCount := totalPaddingNeeded - rightPadCount
		leftPadStr = strings.Repeat(padLeftChar, leftPadCount)
		rightPadStr = strings.Repeat(padRightChar, rightPadCount)
	}

	// Build result
	result := leftPadStr + content + rightPadStr

	// Adjust width if needed
	finalWidth := twwidth.Width(result)
	if finalWidth != targetWidth {
		m.logger.Debugf("Markdown formatCell MISMATCH: content='%s', target_w=%d, paddingL='%s', paddingR='%s', align=%s -> result='%s', result_w=%d",
			content, targetWidth, padding.Left, padding.Right, align, result, finalWidth)
		adjNeeded := targetWidth - finalWidth
		if adjNeeded > 0 {
			adjStr := strings.Repeat(tw.Space, adjNeeded)
			switch align {
			case tw.AlignRight:
				result = adjStr + result
			case tw.AlignCenter:
				leftAdj := adjNeeded / 2
				rightAdj := adjNeeded - leftAdj
				result = strings.Repeat(tw.Space, leftAdj) + result + strings.Repeat(tw.Space, rightAdj)
			default:
				result += adjStr
			}
		} else {
			result = twwidth.Truncate(result, targetWidth)
		}
		m.logger.Debugf("Markdown formatCell Corrected: target_w=%d, result='%s', new_w=%d", targetWidth, result, twwidth.Width(result))
	}

	m.logger.Debugf("Markdown formatCell: content='%s', width=%d, align=%s, paddingL='%s', paddingR='%s' -> '%s' (target %d)",
		content, width, align, padding.Left, padding.Right, result, targetWidth)
	return result
}

// formatSeparator generates a Markdown separator (e.g., `---`, `:--`, `:-:`) with alignment indicators.
func (m *Markdown) formatSeparator(width int, align tw.Align) string {
	targetWidth := tw.Max(3, width)
	var sb strings.Builder

	switch align {
	case tw.AlignLeft:
		sb.WriteRune(':')
		sb.WriteString(strings.Repeat("-", targetWidth-1))
	case tw.AlignRight:
		sb.WriteString(strings.Repeat("-", targetWidth-1))
		sb.WriteRune(':')
	case tw.AlignCenter:
		sb.WriteRune(':')
		sb.WriteString(strings.Repeat("-", targetWidth-2))
		sb.WriteRune(':')
	case tw.AlignNone:
		sb.WriteString(strings.Repeat("-", targetWidth))
	default:
		sb.WriteString(strings.Repeat("-", targetWidth)) // Fallback
	}

	result := sb.String()
	currentLen := twwidth.Width(result)
	if currentLen < targetWidth {
		result += strings.Repeat("-", targetWidth-currentLen)
	} else if currentLen > targetWidth {
		result = twwidth.Truncate(result, targetWidth)
	}

	m.logger.Debugf("Markdown formatSeparator: width=%d, align=%s -> '%s'", width, align, result)
	return result
}

// renderMarkdownLine renders a single Markdown line (header, row, footer, or separator) with pipes and alignment.
func (m *Markdown) renderMarkdownLine(line []string, ctx tw.Formatting, isHeaderSep bool) {
	numCols := 0
	if len(ctx.Row.Widths) > 0 {
		maxKey := -1
		for k := range ctx.Row.Widths {
			if k > maxKey {
				maxKey = k
			}
		}
		numCols = maxKey + 1
	} else if len(ctx.Row.Current) > 0 {
		maxKey := -1
		for k := range ctx.Row.Current {
			if k > maxKey {
				maxKey = k
			}
		}
		numCols = maxKey + 1
	} else if len(line) > 0 && !isHeaderSep {
		numCols = len(line)
	}

	if numCols == 0 && !isHeaderSep {
		m.logger.Debug("renderMarkdownLine: Skipping line with zero columns.")
		return
	}

	var output strings.Builder
	prefix := m.config.Symbols.Column()
	if m.config.Borders.Left == tw.Off {
		prefix = tw.Empty
	}
	suffix := m.config.Symbols.Column()
	if m.config.Borders.Right == tw.Off {
		suffix = tw.Empty
	}
	separator := m.config.Symbols.Column()
	output.WriteString(prefix)

	colIndex := 0
	separatorWidth := twwidth.Width(separator)

	for colIndex < numCols {
		cellCtx, ok := ctx.Row.Current[colIndex]
		align := m.alignment[colIndex]

		defaultPadding := tw.Padding{Left: tw.Space, Right: tw.Space}
		if !ok {
			cellCtx = tw.CellContext{
				Data: tw.Empty, Align: align, Padding: defaultPadding,
				Width: ctx.Row.Widths.Get(colIndex), Merge: tw.MergeState{},
			}
		} else if !cellCtx.Padding.Paddable() {
			cellCtx.Padding = defaultPadding
		}

		// Add separator
		isContinuation := ok && cellCtx.Merge.Horizontal.Present && !cellCtx.Merge.Horizontal.Start
		if colIndex > 0 && !isContinuation {
			output.WriteString(separator)
			m.logger.Debugf("renderMarkdownLine: Added separator '%s' before col %d", separator, colIndex)
		}

		// Calculate width and span
		span := 1
		visualWidth := 0
		isHMergeStart := ok && cellCtx.Merge.Horizontal.Present && cellCtx.Merge.Horizontal.Start
		if isHMergeStart {
			span = cellCtx.Merge.Horizontal.Span
			totalWidth := 0
			for k := 0; k < span && colIndex+k < numCols; k++ {
				colWidth := max(ctx.NormalizedWidths.Get(colIndex+k), 0)
				totalWidth += colWidth
				if k > 0 && separatorWidth > 0 {
					totalWidth += separatorWidth
				}
			}
			visualWidth = totalWidth
			m.logger.Debugf("renderMarkdownLine: HMerge col %d, span %d, visualWidth %d", colIndex, span, visualWidth)
		} else {
			visualWidth = ctx.Row.Widths.Get(colIndex)
			m.logger.Debugf("renderMarkdownLine: Regular col %d, visualWidth %d", colIndex, visualWidth)
		}
		if visualWidth < 0 {
			visualWidth = 0
		}

		// Render segment
		if isContinuation {
			m.logger.Debugf("renderMarkdownLine: Skipping col %d (HMerge continuation)", colIndex)
		} else {
			var formattedSegment string
			if isHeaderSep {
				// Use header's alignment from ctx.Row.Previous
				headerAlign := align
				if headerCellCtx, headerOK := ctx.Row.Previous[colIndex]; headerOK {
					headerAlign = headerCellCtx.Align
					// Preserve tw.AlignNone for separator
					if headerAlign != tw.AlignNone && (headerAlign == tw.Empty || headerAlign == tw.Skip) {
						headerAlign = tw.AlignCenter
					}
				}
				formattedSegment = m.formatSeparator(visualWidth, headerAlign)
			} else {
				content := tw.Empty
				if colIndex < len(line) {
					content = line[colIndex]
				}
				// For rows, use the header's alignment if specified
				rowAlign := align
				if headerCellCtx, headerOK := ctx.Row.Previous[colIndex]; headerOK && !isHeaderSep {
					if headerCellCtx.Align != tw.AlignNone && headerCellCtx.Align != tw.Empty {
						rowAlign = headerCellCtx.Align
					}
				}
				if rowAlign == tw.AlignNone || rowAlign == tw.Empty {
					switch ctx.Row.Position {
					case tw.Header:
						rowAlign = tw.AlignCenter
					case tw.Footer:
						rowAlign = tw.AlignRight
					default:
						rowAlign = tw.AlignLeft
					}
					m.logger.Debugf("renderMarkdownLine: Col %d using default align '%s'", colIndex, rowAlign)
				}
				formattedSegment = m.formatCell(content, visualWidth, rowAlign, cellCtx.Padding)
			}
			output.WriteString(formattedSegment)
			m.logger.Debugf("renderMarkdownLine: Wrote col %d (span %d, width %d): '%s'", colIndex, span, visualWidth, formattedSegment)
		}

		colIndex += span
	}

	output.WriteString(suffix)
	output.WriteString(tw.NewLine)
	m.w.Write([]byte(output.String()))
	m.logger.Debugf("renderMarkdownLine: Final line: %s", strings.TrimSuffix(output.String(), tw.NewLine))
}
