package tablewriter

import (
	"github.com/olekukonko/tablewriter/tw"
)

// Config represents the table configuration
type Config struct {
	MaxWidth int
	Header   tw.CellConfig
	Row      tw.CellConfig
	Footer   tw.CellConfig
	Debug    bool
	Stream   tw.StreamConfig
	Behavior tw.Behavior
	Widths   tw.CellWidth
	Counter  tw.Counter
}

// ConfigBuilder provides a fluent interface for building Config
type ConfigBuilder struct {
	config Config
}

// NewConfigBuilder creates a new ConfigBuilder with defaults
func NewConfigBuilder() *ConfigBuilder {
	return &ConfigBuilder{
		config: defaultConfig(),
	}
}

// Build returns the built Config
func (b *ConfigBuilder) Build() Config {
	return b.config
}

// Header returns a HeaderConfigBuilder for header configuration
func (b *ConfigBuilder) Header() *HeaderConfigBuilder {
	return &HeaderConfigBuilder{
		parent: b,
		config: &b.config.Header,
	}
}

// Row returns a RowConfigBuilder for row configuration
func (b *ConfigBuilder) Row() *RowConfigBuilder {
	return &RowConfigBuilder{
		parent: b,
		config: &b.config.Row,
	}
}

// Footer returns a FooterConfigBuilder for footer configuration
func (b *ConfigBuilder) Footer() *FooterConfigBuilder {
	return &FooterConfigBuilder{
		parent: b,
		config: &b.config.Footer,
	}
}

// Behavior returns a BehaviorConfigBuilder for behavior configuration
func (b *ConfigBuilder) Behavior() *BehaviorConfigBuilder {
	return &BehaviorConfigBuilder{
		parent: b,
		config: &b.config.Behavior,
	}
}

// ForColumn returns a ColumnConfigBuilder for column-specific configuration
func (b *ConfigBuilder) ForColumn(col int) *ColumnConfigBuilder {
	return &ColumnConfigBuilder{
		parent: b,
		col:    col,
	}
}

// WithTrimSpace enables or disables automatic trimming of leading/trailing spaces.
// Ignored in streaming mode.
func (b *ConfigBuilder) WithTrimSpace(state tw.State) *ConfigBuilder {
	b.config.Behavior.TrimSpace = state
	return b
}

// WithTrimTab enables or disables automatic trimming of leading/trailing tabs.
// Useful for preserving indentation in code blocks while trimming other whitespace.
func (b *ConfigBuilder) WithTrimTab(state tw.State) *ConfigBuilder {
	b.config.Behavior.TrimTab = state
	return b
}

// WithDebug enables/disables debug logging
func (b *ConfigBuilder) WithDebug(debug bool) *ConfigBuilder {
	b.config.Debug = debug
	return b
}

// WithAutoHide enables or disables automatic hiding of empty columns (ignored in streaming mode).
func (b *ConfigBuilder) WithAutoHide(state tw.State) *ConfigBuilder {
	b.config.Behavior.AutoHide = state
	return b
}

// WithFooterAlignment sets the text alignment for all footer cells.
// Invalid alignments are ignored.
func (b *ConfigBuilder) WithFooterAlignment(align tw.Align) *ConfigBuilder {
	if align != tw.AlignLeft && align != tw.AlignRight && align != tw.AlignCenter && align != tw.AlignNone {
		return b
	}
	b.config.Footer.Alignment.Global = align
	return b
}

// WithFooterAutoFormat enables or disables automatic formatting (e.g., title case) for footer cells.
func (b *ConfigBuilder) WithFooterAutoFormat(autoFormat tw.State) *ConfigBuilder {
	b.config.Footer.Formatting.AutoFormat = autoFormat
	return b
}

// WithFooterAutoWrap sets the wrapping behavior for footer cells (e.g., truncate, normal, break).
// Invalid wrap modes are ignored.
func (b *ConfigBuilder) WithFooterAutoWrap(autoWrap int) *ConfigBuilder {
	if autoWrap < tw.WrapNone || autoWrap > tw.WrapBreak {
		return b
	}
	b.config.Footer.Formatting.AutoWrap = autoWrap
	return b
}

// WithFooterGlobalPadding sets the global padding for all footer cells.
func (b *ConfigBuilder) WithFooterGlobalPadding(padding tw.Padding) *ConfigBuilder {
	b.config.Footer.Padding.Global = padding
	return b
}

// WithFooterMaxWidth sets the maximum content width for footer cells.
// Negative values are ignored.
func (b *ConfigBuilder) WithFooterMaxWidth(maxWidth int) *ConfigBuilder {
	if maxWidth < 0 {
		return b
	}
	b.config.Footer.ColMaxWidths.Global = maxWidth
	return b
}

// Deprecated: Use .Footer().CellMerging().WithMode(...) instead. This method will be removed in a future version.
func (b *ConfigBuilder) WithFooterMergeMode(mergeMode int) *ConfigBuilder {
	if mergeMode < tw.MergeNone || mergeMode > tw.MergeHierarchical {
		return b
	}
	b.config.Footer.Merging.Mode = mergeMode
	b.config.Footer.Formatting.MergeMode = mergeMode
	return b
}

// WithHeaderAlignment sets the text alignment for all header cells.
// Invalid alignments are ignored.
func (b *ConfigBuilder) WithHeaderAlignment(align tw.Align) *ConfigBuilder {
	if align != tw.AlignLeft && align != tw.AlignRight && align != tw.AlignCenter && align != tw.AlignNone {
		return b
	}
	b.config.Header.Alignment.Global = align
	return b
}

// WithHeaderAutoFormat enables or disables automatic formatting (e.g., title case) for header cells.
func (b *ConfigBuilder) WithHeaderAutoFormat(autoFormat tw.State) *ConfigBuilder {
	b.config.Header.Formatting.AutoFormat = autoFormat
	return b
}

// WithHeaderAutoWrap sets the wrapping behavior for header cells (e.g., truncate, normal).
// Invalid wrap modes are ignored.
func (b *ConfigBuilder) WithHeaderAutoWrap(autoWrap int) *ConfigBuilder {
	if autoWrap < tw.WrapNone || autoWrap > tw.WrapBreak {
		return b
	}
	b.config.Header.Formatting.AutoWrap = autoWrap
	return b
}

// WithHeaderGlobalPadding sets the global padding for all header cells.
func (b *ConfigBuilder) WithHeaderGlobalPadding(padding tw.Padding) *ConfigBuilder {
	b.config.Header.Padding.Global = padding
	return b
}

// WithHeaderMaxWidth sets the maximum content width for header cells.
// Negative values are ignored.
func (b *ConfigBuilder) WithHeaderMaxWidth(maxWidth int) *ConfigBuilder {
	if maxWidth < 0 {
		return b
	}
	b.config.Header.ColMaxWidths.Global = maxWidth
	return b
}

// Deprecated: Use .Header().CellMerging().WithMode(...) instead. This method will be removed in a future version.
func (b *ConfigBuilder) WithHeaderMergeMode(mergeMode int) *ConfigBuilder {
	if mergeMode < tw.MergeNone || mergeMode > tw.MergeHierarchical {
		return b
	}
	b.config.Header.Merging.Mode = mergeMode
	b.config.Header.Formatting.MergeMode = mergeMode
	return b
}

// WithMaxWidth sets the maximum width for the entire table (0 means unlimited).
// Negative values are treated as 0.
func (b *ConfigBuilder) WithMaxWidth(width int) *ConfigBuilder {
	b.config.MaxWidth = max(width, 0)
	return b
}

// WithRowAlignment sets the text alignment for all row cells.
// Invalid alignments are ignored.
func (b *ConfigBuilder) WithRowAlignment(align tw.Align) *ConfigBuilder {
	if align != tw.AlignLeft && align != tw.AlignRight && align != tw.AlignCenter && align != tw.AlignNone {
		return b
	}
	b.config.Row.Alignment.Global = align
	return b
}

// WithRowAutoFormat enables or disables automatic formatting for row cells.
func (b *ConfigBuilder) WithRowAutoFormat(autoFormat tw.State) *ConfigBuilder {
	b.config.Row.Formatting.AutoFormat = autoFormat
	return b
}

// WithRowAutoWrap sets the wrapping behavior for row cells (e.g., truncate, normal).
// Invalid wrap modes are ignored.
func (b *ConfigBuilder) WithRowAutoWrap(autoWrap int) *ConfigBuilder {
	if autoWrap < tw.WrapNone || autoWrap > tw.WrapBreak {
		return b
	}
	b.config.Row.Formatting.AutoWrap = autoWrap
	return b
}

// WithRowGlobalPadding sets the global padding for all row cells.
func (b *ConfigBuilder) WithRowGlobalPadding(padding tw.Padding) *ConfigBuilder {
	b.config.Row.Padding.Global = padding
	return b
}

// WithRowMaxWidth sets the maximum content width for row cells.
// Negative values are ignored.
func (b *ConfigBuilder) WithRowMaxWidth(maxWidth int) *ConfigBuilder {
	if maxWidth < 0 {
		return b
	}
	b.config.Row.ColMaxWidths.Global = maxWidth
	return b
}

// Deprecated: Use .Row().CellMerging().WithMode(...) instead. This method will be removed in a future version.
func (b *ConfigBuilder) WithRowMergeMode(mergeMode int) *ConfigBuilder {
	if mergeMode < tw.MergeNone || mergeMode > tw.MergeHierarchical {
		return b
	}
	b.config.Row.Merging.Mode = mergeMode
	b.config.Row.Formatting.MergeMode = mergeMode
	return b
}

// HeaderConfigBuilder configures header settings
type HeaderConfigBuilder struct {
	parent *ConfigBuilder
	config *tw.CellConfig
}

// Build returns the parent ConfigBuilder
func (h *HeaderConfigBuilder) Build() *ConfigBuilder {
	return h.parent
}

// Alignment returns an AlignmentConfigBuilder for header alignment
func (h *HeaderConfigBuilder) Alignment() *AlignmentConfigBuilder {
	return &AlignmentConfigBuilder{
		parent:  h.parent,
		config:  &h.config.Alignment,
		section: "header",
	}
}

// Formatting returns a HeaderFormattingBuilder for header formatting
func (h *HeaderConfigBuilder) Formatting() *HeaderFormattingBuilder {
	return &HeaderFormattingBuilder{
		parent:  h,
		config:  &h.config.Formatting,
		section: "header",
	}
}

// Merging returns a HeaderMergingBuilder for configuring cell merging.
func (h *HeaderConfigBuilder) Merging() *HeaderMergingBuilder {
	return &HeaderMergingBuilder{
		parent: h,
		config: &h.config.Merging,
	}
}

// Padding returns a HeaderPaddingBuilder for header padding
func (h *HeaderConfigBuilder) Padding() *HeaderPaddingBuilder {
	return &HeaderPaddingBuilder{
		parent:  h,
		config:  &h.config.Padding,
		section: "header",
	}
}

// Filter returns a HeaderFilterBuilder for header filtering
func (h *HeaderConfigBuilder) Filter() *HeaderFilterBuilder {
	return &HeaderFilterBuilder{
		parent:  h,
		config:  &h.config.Filter,
		section: "header",
	}
}

// Callbacks returns a HeaderCallbacksBuilder for header callbacks
func (h *HeaderConfigBuilder) Callbacks() *HeaderCallbacksBuilder {
	return &HeaderCallbacksBuilder{
		parent:  h,
		config:  &h.config.Callbacks,
		section: "header",
	}
}

// RowConfigBuilder configures row settings
type RowConfigBuilder struct {
	parent *ConfigBuilder
	config *tw.CellConfig
}

// Build returns the parent ConfigBuilder
func (r *RowConfigBuilder) Build() *ConfigBuilder {
	return r.parent
}

// Alignment returns an AlignmentConfigBuilder for row alignment
func (r *RowConfigBuilder) Alignment() *AlignmentConfigBuilder {
	return &AlignmentConfigBuilder{
		parent:  r.parent,
		config:  &r.config.Alignment,
		section: "row",
	}
}

// Formatting returns a RowFormattingBuilder for row formatting
func (r *RowConfigBuilder) Formatting() *RowFormattingBuilder {
	return &RowFormattingBuilder{
		parent:  r,
		config:  &r.config.Formatting,
		section: "row",
	}
}

// Merging returns a RowMergingBuilder for configuring cell merging.
func (r *RowConfigBuilder) Merging() *RowMergingBuilder {
	return &RowMergingBuilder{
		parent: r,
		config: &r.config.Merging,
	}
}

// Padding returns a RowPaddingBuilder for row padding
func (r *RowConfigBuilder) Padding() *RowPaddingBuilder {
	return &RowPaddingBuilder{
		parent:  r,
		config:  &r.config.Padding,
		section: "row",
	}
}

// Filter returns a RowFilterBuilder for row filtering
func (r *RowConfigBuilder) Filter() *RowFilterBuilder {
	return &RowFilterBuilder{
		parent:  r,
		config:  &r.config.Filter,
		section: "row",
	}
}

// Callbacks returns a RowCallbacksBuilder for row callbacks
func (r *RowConfigBuilder) Callbacks() *RowCallbacksBuilder {
	return &RowCallbacksBuilder{
		parent:  r,
		config:  &r.config.Callbacks,
		section: "row",
	}
}

// FooterConfigBuilder configures footer settings
type FooterConfigBuilder struct {
	parent *ConfigBuilder
	config *tw.CellConfig
}

// Build returns the parent ConfigBuilder
func (f *FooterConfigBuilder) Build() *ConfigBuilder {
	return f.parent
}

// Alignment returns an AlignmentConfigBuilder for footer alignment
func (f *FooterConfigBuilder) Alignment() *AlignmentConfigBuilder {
	return &AlignmentConfigBuilder{
		parent:  f.parent,
		config:  &f.config.Alignment,
		section: "footer",
	}
}

// Formatting returns a FooterFormattingBuilder for footer formatting
func (f *FooterConfigBuilder) Formatting() *FooterFormattingBuilder {
	return &FooterFormattingBuilder{
		parent:  f,
		config:  &f.config.Formatting,
		section: "footer",
	}
}

// Merging returns a FooterMergingBuilder for configuring cell merging.
func (f *FooterConfigBuilder) Merging() *FooterMergingBuilder {
	return &FooterMergingBuilder{
		parent: f,
		config: &f.config.Merging,
	}
}

// Padding returns a FooterPaddingBuilder for footer padding
func (f *FooterConfigBuilder) Padding() *FooterPaddingBuilder {
	return &FooterPaddingBuilder{
		parent:  f,
		config:  &f.config.Padding,
		section: "footer",
	}
}

// Filter returns a FooterFilterBuilder for footer filtering
func (f *FooterConfigBuilder) Filter() *FooterFilterBuilder {
	return &FooterFilterBuilder{
		parent:  f,
		config:  &f.config.Filter,
		section: "footer",
	}
}

// Callbacks returns a FooterCallbacksBuilder for footer callbacks
func (f *FooterConfigBuilder) Callbacks() *FooterCallbacksBuilder {
	return &FooterCallbacksBuilder{
		parent:  f,
		config:  &f.config.Callbacks,
		section: "footer",
	}
}

// AlignmentConfigBuilder configures alignment settings
type AlignmentConfigBuilder struct {
	parent  *ConfigBuilder
	config  *tw.CellAlignment
	section string
}

// Build returns the parent ConfigBuilder
func (a *AlignmentConfigBuilder) Build() *ConfigBuilder {
	return a.parent
}

// WithGlobal sets global alignment
func (a *AlignmentConfigBuilder) WithGlobal(align tw.Align) *AlignmentConfigBuilder {
	if err := align.Validate(); err == nil {
		a.config.Global = align
	}
	return a
}

// WithPerColumn sets per-column alignments
func (a *AlignmentConfigBuilder) WithPerColumn(alignments []tw.Align) *AlignmentConfigBuilder {
	if len(alignments) > 0 {
		a.config.PerColumn = alignments
	}
	return a
}

// HeaderFormattingBuilder configures header formatting
type HeaderFormattingBuilder struct {
	parent  *HeaderConfigBuilder
	config  *tw.CellFormatting
	section string
}

// Build returns the parent HeaderConfigBuilder
func (hf *HeaderFormattingBuilder) Build() *HeaderConfigBuilder {
	return hf.parent
}

// WithAutoFormat enables/disables auto formatting
func (hf *HeaderFormattingBuilder) WithAutoFormat(autoFormat tw.State) *HeaderFormattingBuilder {
	hf.config.AutoFormat = autoFormat
	return hf
}

// WithAutoWrap sets auto wrap mode
func (hf *HeaderFormattingBuilder) WithAutoWrap(autoWrap int) *HeaderFormattingBuilder {
	if autoWrap >= tw.WrapNone && autoWrap <= tw.WrapBreak {
		hf.config.AutoWrap = autoWrap
	}
	return hf
}

// Deprecated: Use .CellMerging().WithMode(...) instead. This method will be removed in a future version.
func (hf *HeaderFormattingBuilder) WithMergeMode(mergeMode int) *HeaderFormattingBuilder {
	if mergeMode >= tw.MergeNone && mergeMode <= tw.MergeHierarchical {
		hf.parent.config.Merging.Mode = mergeMode
		hf.config.MergeMode = mergeMode
	}
	return hf
}

// RowFormattingBuilder configures row formatting
type RowFormattingBuilder struct {
	parent  *RowConfigBuilder
	config  *tw.CellFormatting
	section string
}

// Build returns the parent RowConfigBuilder
func (rf *RowFormattingBuilder) Build() *RowConfigBuilder {
	return rf.parent
}

// WithAutoFormat enables/disables auto formatting
func (rf *RowFormattingBuilder) WithAutoFormat(autoFormat tw.State) *RowFormattingBuilder {
	rf.config.AutoFormat = autoFormat
	return rf
}

// WithAutoWrap sets auto wrap mode
func (rf *RowFormattingBuilder) WithAutoWrap(autoWrap int) *RowFormattingBuilder {
	if autoWrap >= tw.WrapNone && autoWrap <= tw.WrapBreak {
		rf.config.AutoWrap = autoWrap
	}
	return rf
}

// Deprecated: Use .CellMerging().WithMode(...) instead. This method will be removed in a future version.
func (rf *RowFormattingBuilder) WithMergeMode(mergeMode int) *RowFormattingBuilder {
	if mergeMode >= tw.MergeNone && mergeMode <= tw.MergeHierarchical {
		rf.parent.config.Merging.Mode = mergeMode
		rf.config.MergeMode = mergeMode
	}
	return rf
}

// FooterFormattingBuilder configures footer formatting
type FooterFormattingBuilder struct {
	parent  *FooterConfigBuilder
	config  *tw.CellFormatting
	section string
}

// Build returns the parent FooterConfigBuilder
func (ff *FooterFormattingBuilder) Build() *FooterConfigBuilder {
	return ff.parent
}

// WithAutoFormat enables/disables auto formatting
func (ff *FooterFormattingBuilder) WithAutoFormat(autoFormat tw.State) *FooterFormattingBuilder {
	ff.config.AutoFormat = autoFormat
	return ff
}

// WithAutoWrap sets auto wrap mode
func (ff *FooterFormattingBuilder) WithAutoWrap(autoWrap int) *FooterFormattingBuilder {
	if autoWrap >= tw.WrapNone && autoWrap <= tw.WrapBreak {
		ff.config.AutoWrap = autoWrap
	}
	return ff
}

// Deprecated: Use .CellMerging().WithMode(...) instead. This method will be removed in a future version.
func (ff *FooterFormattingBuilder) WithMergeMode(mergeMode int) *FooterFormattingBuilder {
	if mergeMode >= tw.MergeNone && mergeMode <= tw.MergeHierarchical {
		ff.parent.config.Merging.Mode = mergeMode
		ff.config.MergeMode = mergeMode
	}
	return ff
}

// HeaderMergingBuilder configures header cell merging
type HeaderMergingBuilder struct {
	parent *HeaderConfigBuilder
	config *tw.CellMerging
}

// Build returns the parent HeaderConfigBuilder.
func (hm *HeaderMergingBuilder) Build() *HeaderConfigBuilder {
	return hm.parent
}

// WithMode sets the merge mode (e.g., tw.MergeHorizontal).
func (hm *HeaderMergingBuilder) WithMode(mode int) *HeaderMergingBuilder {
	hm.config.Mode = mode
	// Also set the deprecated field for backward compatibility
	hm.parent.config.Formatting.MergeMode = mode
	return hm
}

// ByColumnIndex sets specific columns to be merged by their index.
// If not called, merging applies to all columns.
func (hm *HeaderMergingBuilder) ByColumnIndex(indices []int) *HeaderMergingBuilder {
	if len(indices) == 0 {
		hm.config.ByColumnIndex = nil // nil means apply to all
	} else {
		mapper := tw.NewMapper[int, bool]()
		for _, idx := range indices {
			mapper.Set(idx, true)
		}
		hm.config.ByColumnIndex = mapper
	}
	return hm
}

// RowMergingBuilder configures row cell merging
type RowMergingBuilder struct {
	parent *RowConfigBuilder
	config *tw.CellMerging
}

// Build returns the parent RowConfigBuilder.
func (rm *RowMergingBuilder) Build() *RowConfigBuilder {
	return rm.parent
}

// WithMode sets the merge mode (e.g., tw.MergeVertical, tw.MergeHierarchical).
func (rm *RowMergingBuilder) WithMode(mode int) *RowMergingBuilder {
	rm.config.Mode = mode
	// Also set the deprecated field for backward compatibility
	rm.parent.config.Formatting.MergeMode = mode
	return rm
}

// ByColumnIndex sets specific columns to be merged by their index.
// If not called, merging applies to all columns.
func (rm *RowMergingBuilder) ByColumnIndex(indices []int) *RowMergingBuilder {
	if len(indices) == 0 {
		rm.config.ByColumnIndex = nil // nil means apply to all
	} else {
		mapper := tw.NewMapper[int, bool]()
		for _, idx := range indices {
			mapper.Set(idx, true)
		}
		rm.config.ByColumnIndex = mapper
	}
	return rm
}

// FooterMergingBuilder configures footer cell merging
type FooterMergingBuilder struct {
	parent *FooterConfigBuilder
	config *tw.CellMerging
}

// Build returns the parent FooterConfigBuilder.
func (fm *FooterMergingBuilder) Build() *FooterConfigBuilder {
	return fm.parent
}

// WithMode sets the merge mode (e.g., tw.MergeHorizontal).
func (fm *FooterMergingBuilder) WithMode(mode int) *FooterMergingBuilder {
	fm.config.Mode = mode
	// Also set the deprecated field for backward compatibility
	fm.parent.config.Formatting.MergeMode = mode
	return fm
}

// ByColumnIndex sets specific columns to be merged by their index.
// If not called, merging applies to all columns.
func (fm *FooterMergingBuilder) ByColumnIndex(indices []int) *FooterMergingBuilder {
	if len(indices) == 0 {
		fm.config.ByColumnIndex = nil // nil means apply to all
	} else {
		mapper := tw.NewMapper[int, bool]()
		for _, idx := range indices {
			mapper.Set(idx, true)
		}
		fm.config.ByColumnIndex = mapper
	}
	return fm
}

// HeaderPaddingBuilder configures header padding
type HeaderPaddingBuilder struct {
	parent  *HeaderConfigBuilder
	config  *tw.CellPadding
	section string
}

// Build returns the parent HeaderConfigBuilder
func (hp *HeaderPaddingBuilder) Build() *HeaderConfigBuilder {
	return hp.parent
}

// WithGlobal sets global padding
func (hp *HeaderPaddingBuilder) WithGlobal(padding tw.Padding) *HeaderPaddingBuilder {
	hp.config.Global = padding
	return hp
}

// WithPerColumn sets per-column padding
func (hp *HeaderPaddingBuilder) WithPerColumn(padding []tw.Padding) *HeaderPaddingBuilder {
	hp.config.PerColumn = padding
	return hp
}

// AddColumnPadding adds padding for a specific column in the header
func (hp *HeaderPaddingBuilder) AddColumnPadding(padding tw.Padding) *HeaderPaddingBuilder {
	hp.config.PerColumn = append(hp.config.PerColumn, padding)
	return hp
}

// RowPaddingBuilder configures row padding
type RowPaddingBuilder struct {
	parent  *RowConfigBuilder
	config  *tw.CellPadding
	section string
}

// Build returns the parent RowConfigBuilder
func (rp *RowPaddingBuilder) Build() *RowConfigBuilder {
	return rp.parent
}

// WithGlobal sets global padding
func (rp *RowPaddingBuilder) WithGlobal(padding tw.Padding) *RowPaddingBuilder {
	rp.config.Global = padding
	return rp
}

// WithPerColumn sets per-column padding
func (rp *RowPaddingBuilder) WithPerColumn(padding []tw.Padding) *RowPaddingBuilder {
	rp.config.PerColumn = padding
	return rp
}

// AddColumnPadding adds padding for a specific column in the rows
func (rp *RowPaddingBuilder) AddColumnPadding(padding tw.Padding) *RowPaddingBuilder {
	rp.config.PerColumn = append(rp.config.PerColumn, padding)
	return rp
}

// FooterPaddingBuilder configures footer padding
type FooterPaddingBuilder struct {
	parent  *FooterConfigBuilder
	config  *tw.CellPadding
	section string
}

// Build returns the parent FooterConfigBuilder
func (fp *FooterPaddingBuilder) Build() *FooterConfigBuilder {
	return fp.parent
}

// WithGlobal sets global padding
func (fp *FooterPaddingBuilder) WithGlobal(padding tw.Padding) *FooterPaddingBuilder {
	fp.config.Global = padding
	return fp
}

// WithPerColumn sets per-column padding
func (fp *FooterPaddingBuilder) WithPerColumn(padding []tw.Padding) *FooterPaddingBuilder {
	fp.config.PerColumn = padding
	return fp
}

// AddColumnPadding adds padding for a specific column in the footer
func (fp *FooterPaddingBuilder) AddColumnPadding(padding tw.Padding) *FooterPaddingBuilder {
	fp.config.PerColumn = append(fp.config.PerColumn, padding)
	return fp
}

// BehaviorConfigBuilder configures behavior settings
type BehaviorConfigBuilder struct {
	parent *ConfigBuilder
	config *tw.Behavior
}

// Build returns the parent ConfigBuilder
func (bb *BehaviorConfigBuilder) Build() *ConfigBuilder {
	return bb.parent
}

// WithAutoHide enables/disables auto-hide
func (bb *BehaviorConfigBuilder) WithAutoHide(state tw.State) *BehaviorConfigBuilder {
	bb.config.AutoHide = state
	return bb
}

// WithTrimSpace enables/disables trim space
func (bb *BehaviorConfigBuilder) WithTrimSpace(state tw.State) *BehaviorConfigBuilder {
	bb.config.TrimSpace = state
	return bb
}

// WithTrimTab enables/disables trim tab
func (bb *BehaviorConfigBuilder) WithTrimTab(state tw.State) *BehaviorConfigBuilder {
	bb.config.TrimTab = state
	return bb
}

// WithHeaderHide enables/disables header visibility
func (bb *BehaviorConfigBuilder) WithHeaderHide(state tw.State) *BehaviorConfigBuilder {
	bb.config.Header.Hide = state
	return bb
}

// WithFooterHide enables/disables footer visibility
func (bb *BehaviorConfigBuilder) WithFooterHide(state tw.State) *BehaviorConfigBuilder {
	bb.config.Footer.Hide = state
	return bb
}

// WithCompactMerge enables/disables compact width optimization for merged cells
func (bb *BehaviorConfigBuilder) WithCompactMerge(state tw.State) *BehaviorConfigBuilder {
	bb.config.Compact.Merge = state
	return bb
}

// WithAutoHeader enables/disables automatic header extraction for structs in Bulk.
func (bb *BehaviorConfigBuilder) WithAutoHeader(state tw.State) *BehaviorConfigBuilder {
	bb.config.Structs.AutoHeader = state
	return bb
}

// ColumnConfigBuilder configures column-specific settings
type ColumnConfigBuilder struct {
	parent *ConfigBuilder
	col    int
}

// Build returns the parent ConfigBuilder
func (c *ColumnConfigBuilder) Build() *ConfigBuilder {
	return c.parent
}

// WithAlignment sets alignment for the column
func (c *ColumnConfigBuilder) WithAlignment(align tw.Align) *ColumnConfigBuilder {
	if err := align.Validate(); err == nil {
		// Ensure slices are large enough
		if len(c.parent.config.Header.Alignment.PerColumn) <= c.col {
			newAligns := make([]tw.Align, c.col+1)
			copy(newAligns, c.parent.config.Header.Alignment.PerColumn)
			c.parent.config.Header.Alignment.PerColumn = newAligns
		}
		c.parent.config.Header.Alignment.PerColumn[c.col] = align

		if len(c.parent.config.Row.Alignment.PerColumn) <= c.col {
			newAligns := make([]tw.Align, c.col+1)
			copy(newAligns, c.parent.config.Row.Alignment.PerColumn)
			c.parent.config.Row.Alignment.PerColumn = newAligns
		}
		c.parent.config.Row.Alignment.PerColumn[c.col] = align

		if len(c.parent.config.Footer.Alignment.PerColumn) <= c.col {
			newAligns := make([]tw.Align, c.col+1)
			copy(newAligns, c.parent.config.Footer.Alignment.PerColumn)
			c.parent.config.Footer.Alignment.PerColumn = newAligns
		}
		c.parent.config.Footer.Alignment.PerColumn[c.col] = align
	}
	return c
}

// WithMaxWidth sets max width for the column
func (c *ColumnConfigBuilder) WithMaxWidth(width int) *ColumnConfigBuilder {
	if width >= 0 {
		// Initialize maps if needed
		if c.parent.config.Header.ColMaxWidths.PerColumn == nil {
			c.parent.config.Header.ColMaxWidths.PerColumn = make(tw.Mapper[int, int])
			c.parent.config.Row.ColMaxWidths.PerColumn = make(tw.Mapper[int, int])
			c.parent.config.Footer.ColMaxWidths.PerColumn = make(tw.Mapper[int, int])
		}
		c.parent.config.Header.ColMaxWidths.PerColumn[c.col] = width
		c.parent.config.Row.ColMaxWidths.PerColumn[c.col] = width
		c.parent.config.Footer.ColMaxWidths.PerColumn[c.col] = width
	}
	return c
}

// HeaderFilterBuilder configures header filtering
type HeaderFilterBuilder struct {
	parent  *HeaderConfigBuilder
	config  *tw.CellFilter
	section string
}

// Build returns the parent HeaderConfigBuilder
func (hf *HeaderFilterBuilder) Build() *HeaderConfigBuilder {
	return hf.parent
}

// WithGlobal sets the global filter function for the header
func (hf *HeaderFilterBuilder) WithGlobal(filter func([]string) []string) *HeaderFilterBuilder {
	if filter != nil {
		hf.config.Global = filter
	}
	return hf
}

// WithPerColumn sets per-column filter functions for the header
func (hf *HeaderFilterBuilder) WithPerColumn(filters []func(string) string) *HeaderFilterBuilder {
	if len(filters) > 0 {
		hf.config.PerColumn = filters
	}
	return hf
}

// AddColumnFilter adds a filter function for a specific column in the header
func (hf *HeaderFilterBuilder) AddColumnFilter(filter func(string) string) *HeaderFilterBuilder {
	if filter != nil {
		hf.config.PerColumn = append(hf.config.PerColumn, filter)
	}
	return hf
}

// RowFilterBuilder configures row filtering
type RowFilterBuilder struct {
	parent  *RowConfigBuilder
	config  *tw.CellFilter
	section string
}

// Build returns the parent RowConfigBuilder
func (rf *RowFilterBuilder) Build() *RowConfigBuilder {
	return rf.parent
}

// WithGlobal sets the global filter function for the rows
func (rf *RowFilterBuilder) WithGlobal(filter func([]string) []string) *RowFilterBuilder {
	if filter != nil {
		rf.config.Global = filter
	}
	return rf
}

// WithPerColumn sets per-column filter functions for the rows
func (rf *RowFilterBuilder) WithPerColumn(filters []func(string) string) *RowFilterBuilder {
	if len(filters) > 0 {
		rf.config.PerColumn = filters
	}
	return rf
}

// AddColumnFilter adds a filter function for a specific column in the rows
func (rf *RowFilterBuilder) AddColumnFilter(filter func(string) string) *RowFilterBuilder {
	if filter != nil {
		rf.config.PerColumn = append(rf.config.PerColumn, filter)
	}
	return rf
}

// FooterFilterBuilder configures footer filtering
type FooterFilterBuilder struct {
	parent  *FooterConfigBuilder
	config  *tw.CellFilter
	section string
}

// Build returns the parent FooterConfigBuilder
func (ff *FooterFilterBuilder) Build() *FooterConfigBuilder {
	return ff.parent
}

// WithGlobal sets the global filter function for the footer
func (ff *FooterFilterBuilder) WithGlobal(filter func([]string) []string) *FooterFilterBuilder {
	if filter != nil {
		ff.config.Global = filter
	}
	return ff
}

// WithPerColumn sets per-column filter functions for the footer
func (ff *FooterFilterBuilder) WithPerColumn(filters []func(string) string) *FooterFilterBuilder {
	if len(filters) > 0 {
		ff.config.PerColumn = filters
	}
	return ff
}

// AddColumnFilter adds a filter function for a specific column in the footer
func (ff *FooterFilterBuilder) AddColumnFilter(filter func(string) string) *FooterFilterBuilder {
	if filter != nil {
		ff.config.PerColumn = append(ff.config.PerColumn, filter)
	}
	return ff
}

// HeaderCallbacksBuilder configures header callbacks
type HeaderCallbacksBuilder struct {
	parent  *HeaderConfigBuilder
	config  *tw.CellCallbacks
	section string
}

// Build returns the parent HeaderConfigBuilder
func (hc *HeaderCallbacksBuilder) Build() *HeaderConfigBuilder {
	return hc.parent
}

// WithGlobal sets the global callback function for the header
func (hc *HeaderCallbacksBuilder) WithGlobal(callback func()) *HeaderCallbacksBuilder {
	if callback != nil {
		hc.config.Global = callback
	}
	return hc
}

// WithPerColumn sets per-column callback functions for the header
func (hc *HeaderCallbacksBuilder) WithPerColumn(callbacks []func()) *HeaderCallbacksBuilder {
	if len(callbacks) > 0 {
		hc.config.PerColumn = callbacks
	}
	return hc
}

// AddColumnCallback adds a callback function for a specific column in the header
func (hc *HeaderCallbacksBuilder) AddColumnCallback(callback func()) *HeaderCallbacksBuilder {
	if callback != nil {
		hc.config.PerColumn = append(hc.config.PerColumn, callback)
	}
	return hc
}

// RowCallbacksBuilder configures row callbacks
type RowCallbacksBuilder struct {
	parent  *RowConfigBuilder
	config  *tw.CellCallbacks
	section string
}

// Build returns the parent RowConfigBuilder
func (rc *RowCallbacksBuilder) Build() *RowConfigBuilder {
	return rc.parent
}

// WithGlobal sets the global callback function for the rows
func (rc *RowCallbacksBuilder) WithGlobal(callback func()) *RowCallbacksBuilder {
	if callback != nil {
		rc.config.Global = callback
	}
	return rc
}

// WithPerColumn sets per-column callback functions for the rows
func (rc *RowCallbacksBuilder) WithPerColumn(callbacks []func()) *RowCallbacksBuilder {
	if len(callbacks) > 0 {
		rc.config.PerColumn = callbacks
	}
	return rc
}

// AddColumnCallback adds a callback function for a specific column in the rows
func (rc *RowCallbacksBuilder) AddColumnCallback(callback func()) *RowCallbacksBuilder {
	if callback != nil {
		rc.config.PerColumn = append(rc.config.PerColumn, callback)
	}
	return rc
}

// FooterCallbacksBuilder configures footer callbacks
type FooterCallbacksBuilder struct {
	parent  *FooterConfigBuilder
	config  *tw.CellCallbacks
	section string
}

// Build returns the parent FooterConfigBuilder
func (fc *FooterCallbacksBuilder) Build() *FooterConfigBuilder {
	return fc.parent
}

// WithGlobal sets the global callback function for the footer
func (fc *FooterCallbacksBuilder) WithGlobal(callback func()) *FooterCallbacksBuilder {
	if callback != nil {
		fc.config.Global = callback
	}
	return fc
}

// WithPerColumn sets per-column callback functions for the footer
func (fc *FooterCallbacksBuilder) WithPerColumn(callbacks []func()) *FooterCallbacksBuilder {
	if len(callbacks) > 0 {
		fc.config.PerColumn = callbacks
	}
	return fc
}

// AddColumnCallback adds a callback function for a specific column in the footer
func (fc *FooterCallbacksBuilder) AddColumnCallback(callback func()) *FooterCallbacksBuilder {
	if callback != nil {
		fc.config.PerColumn = append(fc.config.PerColumn, callback)
	}
	return fc
}
