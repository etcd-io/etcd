package tablewriter

import (
	"reflect"

	"github.com/olekukonko/ll"
	"github.com/olekukonko/tablewriter/pkg/twcache"
	"github.com/olekukonko/tablewriter/pkg/twwidth"
	"github.com/olekukonko/tablewriter/tw"
)

// Option defines a function type for configuring a Table instance.
type Option func(target *Table)

// WithAutoHide enables or disables automatic hiding of columns with empty data rows.
// Logs the change if debugging is enabled.
func WithAutoHide(state tw.State) Option {
	return func(target *Table) {
		target.config.Behavior.AutoHide = state
		if target.logger != nil {
			target.logger.Debugf("Option: WithAutoHide applied to Table: %v", state)
		}
	}
}

// WithColumnMax sets a global maximum column width for the table in streaming mode.
// Negative values are ignored, and the change is logged if debugging is enabled.
func WithColumnMax(width int) Option {
	return func(target *Table) {
		if width < 0 {
			return
		}
		target.config.Widths.Global = width
		if target.logger != nil {
			target.logger.Debugf("Option: WithColumnMax applied to Table: %v", width)
		}
	}
}

// WithMaxWidth sets a global maximum table width for the table.
// Negative values are ignored, and the change is logged if debugging is enabled.
func WithMaxWidth(width int) Option {
	return func(target *Table) {
		if width < 0 {
			return
		}
		target.config.MaxWidth = width
		if target.logger != nil {
			target.logger.Debugf("Option: WithTableMax applied to Table: %v", width)
		}
	}
}

// WithWidths sets per-column widths for the table.
// Negative widths are removed, and the change is logged if debugging is enabled.
func WithWidths(width tw.CellWidth) Option {
	return func(target *Table) {
		target.config.Widths = width
		if target.logger != nil {
			target.logger.Debugf("Option: WithColumnWidths applied to Table: %v", width)
		}
	}
}

// WithColumnWidths sets per-column widths for the table.
// Negative widths are removed, and the change is logged if debugging is enabled.
func WithColumnWidths(widths tw.Mapper[int, int]) Option {
	return func(target *Table) {
		for k, v := range widths {
			if v < 0 {
				delete(widths, k)
			}
		}
		target.config.Widths.PerColumn = widths
		if target.logger != nil {
			target.logger.Debugf("Option: WithColumnWidths applied to Table: %v", widths)
		}
	}
}

// WithConfig applies a custom configuration to the table by merging it with the default configuration.
func WithConfig(cfg Config) Option {
	return func(target *Table) {
		target.config = mergeConfig(defaultConfig(), cfg)
	}
}

// WithDebug enables or disables debug logging and adjusts the logger level accordingly.
// Logs the change if debugging is enabled.
func WithDebug(debug bool) Option {
	return func(target *Table) {
		target.config.Debug = debug
	}
}

// WithFooter sets the table footers by calling the Footer method.
func WithFooter(footers []string) Option {
	return func(target *Table) {
		target.Footer(footers)
	}
}

// WithFooterConfig applies a full footer configuration to the table.
// Logs the change if debugging is enabled.
func WithFooterConfig(config tw.CellConfig) Option {
	return func(target *Table) {
		target.config.Footer = config
		if target.logger != nil {
			target.logger.Debug("Option: WithFooterConfig applied to Table.")
		}
	}
}

// WithFooterAlignmentConfig applies a footer alignment configuration to the table.
// Logs the change if debugging is enabled.
func WithFooterAlignmentConfig(alignment tw.CellAlignment) Option {
	return func(target *Table) {
		target.config.Footer.Alignment = alignment
		if target.logger != nil {
			target.logger.Debugf("Option: WithFooterAlignmentConfig applied to Table: %+v", alignment)
		}
	}
}

// Deprecated: Use a ConfigBuilder with .Footer().CellMerging().WithMode(...) instead.
// This option will be removed in a future version.
func WithFooterMergeMode(mergeMode int) Option {
	return func(target *Table) {
		if mergeMode < tw.MergeNone || mergeMode > tw.MergeHierarchical {
			return
		}
		target.config.Footer.Merging.Mode = mergeMode
		target.config.Footer.Formatting.MergeMode = mergeMode
		if target.logger != nil {
			target.logger.Debugf("Option: WithFooterMergeMode applied to Table: %v", mergeMode)
		}
	}
}

// WithFooterAutoWrap sets the wrapping behavior for footer cells.
// Invalid wrap modes are ignored, and the change is logged if debugging is enabled.
func WithFooterAutoWrap(wrap int) Option {
	return func(target *Table) {
		if wrap < tw.WrapNone || wrap > tw.WrapBreak {
			return
		}
		target.config.Footer.Formatting.AutoWrap = wrap
		if target.logger != nil {
			target.logger.Debugf("Option: WithFooterAutoWrap applied to Table: %v", wrap)
		}
	}
}

// WithFooterFilter sets the filter configuration for footer cells.
// Logs the change if debugging is enabled.
func WithFooterFilter(filter tw.CellFilter) Option {
	return func(target *Table) {
		target.config.Footer.Filter = filter
		if target.logger != nil {
			target.logger.Debug("Option: WithFooterFilter applied to Table.")
		}
	}
}

// WithFooterCallbacks sets the callback configuration for footer cells.
// Logs the change if debugging is enabled.
func WithFooterCallbacks(callbacks tw.CellCallbacks) Option {
	return func(target *Table) {
		target.config.Footer.Callbacks = callbacks
		if target.logger != nil {
			target.logger.Debug("Option: WithFooterCallbacks applied to Table.")
		}
	}
}

// WithFooterPaddingPerColumn sets per-column padding for footer cells.
// Logs the change if debugging is enabled.
func WithFooterPaddingPerColumn(padding []tw.Padding) Option {
	return func(target *Table) {
		target.config.Footer.Padding.PerColumn = padding
		if target.logger != nil {
			target.logger.Debugf("Option: WithFooterPaddingPerColumn applied to Table: %+v", padding)
		}
	}
}

// WithFooterMaxWidth sets the maximum content width for footer cells.
// Negative values are ignored, and the change is logged if debugging is enabled.
func WithFooterMaxWidth(maxWidth int) Option {
	return func(target *Table) {
		if maxWidth < 0 {
			return
		}
		target.config.Footer.ColMaxWidths.Global = maxWidth
		if target.logger != nil {
			target.logger.Debugf("Option: WithFooterMaxWidth applied to Table: %v", maxWidth)
		}
	}
}

// WithHeader sets the table headers by calling the Header method.
func WithHeader(headers []string) Option {
	return func(target *Table) {
		target.Header(headers)
	}
}

// WithHeaderAlignment sets the text alignment for header cells.
// Invalid alignments are ignored, and the change is logged if debugging is enabled.
func WithHeaderAlignment(align tw.Align) Option {
	return func(target *Table) {
		if align != tw.AlignLeft && align != tw.AlignRight && align != tw.AlignCenter && align != tw.AlignNone {
			return
		}
		target.config.Header.Alignment.Global = align
		if target.logger != nil {
			target.logger.Debugf("Option: WithHeaderAlignment applied to Table: %v", align)
		}
	}
}

// WithHeaderAutoWrap sets the wrapping behavior for header cells.
// Invalid wrap modes are ignored, and the change is logged if debugging is enabled.
func WithHeaderAutoWrap(wrap int) Option {
	return func(target *Table) {
		if wrap < tw.WrapNone || wrap > tw.WrapBreak {
			return
		}
		target.config.Header.Formatting.AutoWrap = wrap
		if target.logger != nil {
			target.logger.Debugf("Option: WithHeaderAutoWrap applied to Table: %v", wrap)
		}
	}
}

// Deprecated: Use a ConfigBuilder with .Header().CellMerging().WithMode(...) instead.
// This option will be removed in a future version.
func WithHeaderMergeMode(mergeMode int) Option {
	return func(target *Table) {
		if mergeMode < tw.MergeNone || mergeMode > tw.MergeHierarchical {
			return
		}
		target.config.Header.Merging.Mode = mergeMode
		target.config.Header.Formatting.MergeMode = mergeMode
		if target.logger != nil {
			target.logger.Debugf("Option: WithHeaderMergeMode applied to Table: %v", mergeMode)
		}
	}
}

// WithHeaderFilter sets the filter configuration for header cells.
// Logs the change if debugging is enabled.
func WithHeaderFilter(filter tw.CellFilter) Option {
	return func(target *Table) {
		target.config.Header.Filter = filter
		if target.logger != nil {
			target.logger.Debug("Option: WithHeaderFilter applied to Table.")
		}
	}
}

// WithHeaderCallbacks sets the callback configuration for header cells.
// Logs the change if debugging is enabled.
func WithHeaderCallbacks(callbacks tw.CellCallbacks) Option {
	return func(target *Table) {
		target.config.Header.Callbacks = callbacks
		if target.logger != nil {
			target.logger.Debug("Option: WithHeaderCallbacks applied to Table.")
		}
	}
}

// WithHeaderPaddingPerColumn sets per-column padding for header cells.
// Logs the change if debugging is enabled.
func WithHeaderPaddingPerColumn(padding []tw.Padding) Option {
	return func(target *Table) {
		target.config.Header.Padding.PerColumn = padding
		if target.logger != nil {
			target.logger.Debugf("Option: WithHeaderPaddingPerColumn applied to Table: %+v", padding)
		}
	}
}

// WithHeaderMaxWidth sets the maximum content width for header cells.
// Negative values are ignored, and the change is logged if debugging is enabled.
func WithHeaderMaxWidth(maxWidth int) Option {
	return func(target *Table) {
		if maxWidth < 0 {
			return
		}
		target.config.Header.ColMaxWidths.Global = maxWidth
		if target.logger != nil {
			target.logger.Debugf("Option: WithHeaderMaxWidth applied to Table: %v", maxWidth)
		}
	}
}

// WithRowAlignment sets the text alignment for row cells.
// Invalid alignments are ignored, and the change is logged if debugging is enabled.
func WithRowAlignment(align tw.Align) Option {
	return func(target *Table) {
		if err := align.Validate(); err != nil {
			return
		}
		target.config.Row.Alignment.Global = align
		if target.logger != nil {
			target.logger.Debugf("Option: WithRowAlignment applied to Table: %v", align)
		}
	}
}

// WithRowAutoWrap sets the wrapping behavior for row cells.
// Invalid wrap modes are ignored, and the change is logged if debugging is enabled.
func WithRowAutoWrap(wrap int) Option {
	return func(target *Table) {
		if wrap < tw.WrapNone || wrap > tw.WrapBreak {
			return
		}
		target.config.Row.Formatting.AutoWrap = wrap
		if target.logger != nil {
			target.logger.Debugf("Option: WithRowAutoWrap applied to Table: %v", wrap)
		}
	}
}

// Deprecated: Use a ConfigBuilder with .Row().CellMerging().WithMode(...) instead.
// This option will be removed in a future version.
func WithRowMergeMode(mergeMode int) Option {
	return func(target *Table) {
		if mergeMode < tw.MergeNone || mergeMode > tw.MergeHierarchical {
			return
		}
		target.config.Row.Merging.Mode = mergeMode
		target.config.Row.Formatting.MergeMode = mergeMode
		if target.logger != nil {
			target.logger.Debugf("Option: WithRowMergeMode applied to Table: %v", mergeMode)
		}
	}
}

// WithRowFilter sets the filter configuration for row cells.
// Logs the change if debugging is enabled.
func WithRowFilter(filter tw.CellFilter) Option {
	return func(target *Table) {
		target.config.Row.Filter = filter
		if target.logger != nil {
			target.logger.Debug("Option: WithRowFilter applied to Table.")
		}
	}
}

// WithRowCallbacks sets the callback configuration for row cells.
// Logs the change if debugging is enabled.
func WithRowCallbacks(callbacks tw.CellCallbacks) Option {
	return func(target *Table) {
		target.config.Row.Callbacks = callbacks
		if target.logger != nil {
			target.logger.Debug("Option: WithRowCallbacks applied to Table.")
		}
	}
}

// WithRowPaddingPerColumn sets per-column padding for row cells.
// Logs the change if debugging is enabled.
func WithRowPaddingPerColumn(padding []tw.Padding) Option {
	return func(target *Table) {
		target.config.Row.Padding.PerColumn = padding
		if target.logger != nil {
			target.logger.Debugf("Option: WithRowPaddingPerColumn applied to Table: %+v", padding)
		}
	}
}

// WithHeaderAlignmentConfig applies a header alignment configuration to the table.
// Logs the change if debugging is enabled.
func WithHeaderAlignmentConfig(alignment tw.CellAlignment) Option {
	return func(target *Table) {
		target.config.Header.Alignment = alignment
		if target.logger != nil {
			target.logger.Debugf("Option: WithHeaderAlignmentConfig applied to Table: %+v", alignment)
		}
	}
}

// WithHeaderConfig applies a full header configuration to the table.
// Logs the change if debugging is enabled.
func WithHeaderConfig(config tw.CellConfig) Option {
	return func(target *Table) {
		target.config.Header = config
		if target.logger != nil {
			target.logger.Debug("Option: WithHeaderConfig applied to Table.")
		}
	}
}

// WithLogger sets a custom logger for the table and updates the renderer if present.
// Logs the change if debugging is enabled.
func WithLogger(logger *ll.Logger) Option {
	return func(target *Table) {
		target.logger = logger
		if target.logger != nil {
			target.logger.Debug("Option: WithLogger applied to Table.")
			if target.renderer != nil {
				target.renderer.Logger(target.logger)
			}
		}
	}
}

// WithRenderer sets a custom renderer for the table and attaches the logger if present.
// Logs the change if debugging is enabled.
func WithRenderer(f tw.Renderer) Option {
	return func(target *Table) {
		target.renderer = f
		if target.logger != nil {
			target.logger.Debugf("Option: WithRenderer applied to Table: %T", f)
			f.Logger(target.logger)
		}
	}
}

// WithRowConfig applies a full row configuration to the table.
// Logs the change if debugging is enabled.
func WithRowConfig(config tw.CellConfig) Option {
	return func(target *Table) {
		target.config.Row = config
		if target.logger != nil {
			target.logger.Debug("Option: WithRowConfig applied to Table.")
		}
	}
}

// WithRowAlignmentConfig applies a row alignment configuration to the table.
// Logs the change if debugging is enabled.
func WithRowAlignmentConfig(alignment tw.CellAlignment) Option {
	return func(target *Table) {
		target.config.Row.Alignment = alignment
		if target.logger != nil {
			target.logger.Debugf("Option: WithRowAlignmentConfig applied to Table: %+v", alignment)
		}
	}
}

// WithRowMaxWidth sets the maximum content width for row cells.
// Negative values are ignored, and the change is logged if debugging is enabled.
func WithRowMaxWidth(maxWidth int) Option {
	return func(target *Table) {
		if maxWidth < 0 {
			return
		}
		target.config.Row.ColMaxWidths.Global = maxWidth
		if target.logger != nil {
			target.logger.Debugf("Option: WithRowMaxWidth applied to Table: %v", maxWidth)
		}
	}
}

// WithStreaming applies a streaming configuration to the table by merging it with the existing configuration.
// Logs the change if debugging is enabled.
func WithStreaming(c tw.StreamConfig) Option {
	return func(target *Table) {
		target.config.Stream = mergeStreamConfig(target.config.Stream, c)
		if target.logger != nil {
			target.logger.Debug("Option: WithStreaming applied to Table.")
		}
	}
}

// WithStringer sets a custom stringer function for converting row data and clears the stringer cache.
// Logs the change if debugging is enabled.
func WithStringer(stringer interface{}) Option {
	return func(t *Table) {
		t.stringer = stringer
		t.stringerCache = twcache.NewLRU[reflect.Type, reflect.Value](tw.DefaultCacheStringCapacity)
		if t.logger != nil {
			t.logger.Debug("Stringer updated, cache cleared")
		}
	}
}

// WithStringerCache enables the default LRU caching for the stringer function.
// It initializes the cache with a default capacity if one does not already exist.
func WithStringerCache() Option {
	return func(t *Table) {
		// Initialize default cache if strictly necessary (nil),
		// or if you want to ensure the default implementation is used.
		if t.stringerCache == nil {
			// NewLRU returns (Instance, error). We ignore the error here assuming capacity > 0.
			cache := twcache.NewLRU[reflect.Type, reflect.Value](tw.DefaultCacheStringCapacity)
			t.stringerCache = cache
		}

		if t.logger != nil {
			t.logger.Debug("Option: WithStringerCache enabled (Default LRU)")
		}
	}
}

// WithStringerCacheCustom enables caching for the stringer function using a specific implementation.
// Passing nil disables caching entirely.
func WithStringerCacheCustom(cache twcache.Cache[reflect.Type, reflect.Value]) Option {
	return func(t *Table) {
		if cache == nil {
			t.stringerCache = nil
			if t.logger != nil {
				t.logger.Debug("Option: WithStringerCacheCustom called with nil (Caching Disabled)")
			}
			return
		}

		// Set the custom cache and enable the flag
		t.stringerCache = cache

		if t.logger != nil {
			t.logger.Debug("Option: WithStringerCacheCustom enabled")
		}
	}
}

// WithTrimSpace sets whether leading and trailing spaces are automatically trimmed.
// Logs the change if debugging is enabled.
func WithTrimSpace(state tw.State) Option {
	return func(target *Table) {
		target.config.Behavior.TrimSpace = state
		if target.logger != nil {
			target.logger.Debugf("Option: WithTrimSpace applied to Table: %v", state)
		}
	}
}

// WithTrimTab sets whether leading and trailing tab characters are automatically trimmed.
// Logs the change if debugging is enabled.
func WithTrimTab(state tw.State) Option {
	return func(target *Table) {
		target.config.Behavior.TrimTab = state
		if target.logger != nil {
			target.logger.Debugf("Option: WithTrimTab applied to Table: %v", state)
		}
	}
}

// WithTrimLine sets whether empty visual lines within a cell are trimmed.
// Logs the change if debugging is enabled.
func WithTrimLine(state tw.State) Option {
	return func(target *Table) {
		target.config.Behavior.TrimLine = state
		if target.logger != nil {
			target.logger.Debugf("Option: WithTrimLine applied to Table: %v", state)
		}
	}
}

// WithHeaderAutoFormat enables or disables automatic formatting for header cells.
// Logs the change if debugging is enabled.
func WithHeaderAutoFormat(state tw.State) Option {
	return func(target *Table) {
		target.config.Header.Formatting.AutoFormat = state
		if target.logger != nil {
			target.logger.Debugf("Option: WithHeaderAutoFormat applied to Table: %v", state)
		}
	}
}

// WithFooterAutoFormat enables or disables automatic formatting for footer cells.
// Logs the change if debugging is enabled.
func WithFooterAutoFormat(state tw.State) Option {
	return func(target *Table) {
		target.config.Footer.Formatting.AutoFormat = state
		if target.logger != nil {
			target.logger.Debugf("Option: WithFooterAutoFormat applied to Table: %v", state)
		}
	}
}

// WithRowAutoFormat enables or disables automatic formatting for row cells.
// Logs the change if debugging is enabled.
func WithRowAutoFormat(state tw.State) Option {
	return func(target *Table) {
		target.config.Row.Formatting.AutoFormat = state
		if target.logger != nil {
			target.logger.Debugf("Option: WithRowAutoFormat applied to Table: %v", state)
		}
	}
}

// WithHeaderControl sets the control behavior for the table header.
// Logs the change if debugging is enabled.
func WithHeaderControl(control tw.Control) Option {
	return func(target *Table) {
		target.config.Behavior.Header = control
		if target.logger != nil {
			target.logger.Debugf("Option: WithHeaderControl applied to Table: %v", control)
		}
	}
}

// WithFooterControl sets the control behavior for the table footer.
// Logs the change if debugging is enabled.
func WithFooterControl(control tw.Control) Option {
	return func(target *Table) {
		target.config.Behavior.Footer = control
		if target.logger != nil {
			target.logger.Debugf("Option: WithFooterControl applied to Table: %v", control)
		}
	}
}

// WithAlignment sets the default column alignment for the header, rows, and footer.
// Logs the change if debugging is enabled.
func WithAlignment(alignment tw.Alignment) Option {
	return func(target *Table) {
		target.config.Header.Alignment.PerColumn = alignment
		target.config.Row.Alignment.PerColumn = alignment
		target.config.Footer.Alignment.PerColumn = alignment
		if target.logger != nil {
			target.logger.Debugf("Option: WithAlignment applied to Table: %+v", alignment)
		}
	}
}

// WithBehavior applies a behavior configuration to the table.
// Logs the change if debugging is enabled.
func WithBehavior(behavior tw.Behavior) Option {
	return func(target *Table) {
		target.config.Behavior = behavior
		if target.logger != nil {
			target.logger.Debugf("Option: WithBehavior applied to Table: %+v", behavior)
		}
	}
}

// WithPadding sets the global padding for the header, rows, and footer.
// Logs the change if debugging is enabled.
func WithPadding(padding tw.Padding) Option {
	return func(target *Table) {
		target.config.Header.Padding.Global = padding
		target.config.Row.Padding.Global = padding
		target.config.Footer.Padding.Global = padding
		if target.logger != nil {
			target.logger.Debugf("Option: WithPadding applied to Table: %+v", padding)
		}
	}
}

// WithRendition allows updating the active renderer's rendition configuration
// by merging the provided rendition.
// If the renderer does not implement tw.Renditioning, a warning is logged.
// Logs the change if debugging is enabled.
func WithRendition(rendition tw.Rendition) Option {
	return func(target *Table) {
		if target.renderer == nil {
			if target.logger != nil {
				target.logger.Warn("Option: WithRendition: No renderer set on table.")
			}
			return
		}
		if ru, ok := target.renderer.(tw.Renditioning); ok {
			ru.Rendition(rendition)
			if target.logger != nil {
				target.logger.Debugf("Option: WithRendition: Applied to renderer via Renditioning.SetRendition(): %+v", rendition)
			}
		} else if target.logger != nil {
			target.logger.Warnf("Option: WithRendition: Current renderer type %T does not implement tw.Renditioning. Rendition may not be applied as expected.", target.renderer)
		}
	}
}

// WithEastAsian configures the global East Asian width calculation setting.
//   - state=tw.On: Enables East Asian width calculations. CJK and ambiguous characters
//     are typically measured as double width.
//   - state=tw.Off: Disables East Asian width calculations. Characters are generally
//     measured as single width, subject to Unicode standards.
//
// This setting affects all subsequent display width calculations using the twdw package.
func WithEastAsian(state tw.State) Option {
	return func(target *Table) {
		if state.Enabled() {
			twwidth.SetEastAsian(true)
		}
		if state.Disabled() {
			twwidth.SetEastAsian(false)
		}
	}
}

// WithSymbols sets the symbols used for drawing table borders and separators.
// The symbols are applied to the table's renderer configuration, if a renderer is set.
// If no renderer is set (target.renderer is nil), this option has no effect. .
func WithSymbols(symbols tw.Symbols) Option {
	return func(target *Table) {
		if target.renderer != nil {
			cfg := target.renderer.Config()
			cfg.Symbols = symbols

			if ru, ok := target.renderer.(tw.Renditioning); ok {
				ru.Rendition(cfg)
				if target.logger != nil {
					target.logger.Debugf("Option: WithRendition: Applied to renderer via Renditioning.SetRendition(): %+v", cfg)
				}
			} else if target.logger != nil {
				target.logger.Warnf("Option: WithRendition: Current renderer type %T does not implement tw.Renditioning. Rendition may not be applied as expected.", target.renderer)
			}
		}
	}
}

// WithCounters enables line counting by wrapping the table's writer.
// If a custom counter (that implements tw.Counter) is provided, it will be used.
// If the provided counter is nil, a default tw.LineCounter will be used.
// The final count can be retrieved via the table.Lines() method after Render() is called.
func WithCounters(counters ...tw.Counter) Option {
	return func(target *Table) {
		// Iterate through the provided counters and add any non-nil ones.
		for _, c := range counters {
			if c != nil {
				target.counters = append(target.counters, c)
			}
		}
	}
}

// WithLineCounter enables the default line counter.
// A new instance of tw.LineCounter is added to the table's list of counters.
// The total count can be retrieved via the table.Lines() method after Render() is called.
func WithLineCounter() Option {
	return func(target *Table) {
		// Important: Create a new instance so tables don't share counters.
		target.counters = append(target.counters, &tw.LineCounter{})
	}
}

// defaultConfig returns a default Config with sensible settings for headers, rows, footers, and behavior.
func defaultConfig() Config {
	return Config{
		MaxWidth: 0,
		Header: tw.CellConfig{
			Formatting: tw.CellFormatting{
				AutoWrap:   tw.WrapTruncate,
				AutoFormat: tw.On,
				MergeMode:  tw.MergeNone,
			},
			Merging: tw.CellMerging{
				Mode: tw.MergeNone,
			},
			Padding: tw.CellPadding{
				Global: tw.PaddingDefault,
			},
			Alignment: tw.CellAlignment{
				Global:    tw.AlignCenter,
				PerColumn: []tw.Align{},
			},
		},
		Row: tw.CellConfig{
			Formatting: tw.CellFormatting{
				AutoWrap:   tw.WrapNormal,
				AutoFormat: tw.Off,
				MergeMode:  tw.MergeNone,
			},
			Merging: tw.CellMerging{
				Mode: tw.MergeNone,
			},
			Padding: tw.CellPadding{
				Global: tw.PaddingDefault,
			},
			Alignment: tw.CellAlignment{
				Global:    tw.AlignLeft,
				PerColumn: []tw.Align{},
			},
		},
		Footer: tw.CellConfig{
			Formatting: tw.CellFormatting{
				AutoWrap:   tw.WrapNormal,
				AutoFormat: tw.Off,
				MergeMode:  tw.MergeNone,
			},
			Merging: tw.CellMerging{
				Mode: tw.MergeNone,
			},
			Padding: tw.CellPadding{
				Global: tw.PaddingDefault,
			},
			Alignment: tw.CellAlignment{
				Global:    tw.AlignRight,
				PerColumn: []tw.Align{},
			},
		},
		Stream: tw.StreamConfig{
			Enable:        false,
			StrictColumns: false,
		},
		Debug: false,
		Behavior: tw.Behavior{
			AutoHide:  tw.Off,
			TrimSpace: tw.On,
			TrimTab:   tw.On,
			TrimLine:  tw.On,
			Structs: tw.Struct{
				AutoHeader: tw.Off,
				Tags:       []string{"json", "db"},
			},
		},
	}
}

// mergeCellConfig merges a source CellConfig into a destination CellConfig, prioritizing non-default source values.
// It handles deep merging for complex fields like padding and callbacks.
func mergeCellConfig(dst, src tw.CellConfig) tw.CellConfig {
	if src.Formatting.Alignment != tw.Empty {
		dst.Formatting.Alignment = src.Formatting.Alignment
	}

	if src.Formatting.AutoWrap != 0 {
		dst.Formatting.AutoWrap = src.Formatting.AutoWrap
	}
	if src.ColMaxWidths.Global != 0 {
		dst.ColMaxWidths.Global = src.ColMaxWidths.Global
	}

	// Handle merging of the new CellMerging struct and the deprecated MergeMode
	if src.Merging.Mode != 0 {
		dst.Merging.Mode = src.Merging.Mode
		dst.Formatting.MergeMode = src.Merging.Mode
	} else if src.Formatting.MergeMode != 0 {
		dst.Merging.Mode = src.Formatting.MergeMode
		dst.Formatting.MergeMode = src.Formatting.MergeMode
	}

	if src.Merging.ByColumnIndex != nil {
		dst.Merging.ByColumnIndex = src.Merging.ByColumnIndex.Clone()
	}

	dst.Formatting.AutoFormat = src.Formatting.AutoFormat

	if src.Padding.Global.Paddable() {
		dst.Padding.Global = src.Padding.Global
	}

	if len(src.Padding.PerColumn) > 0 {
		if dst.Padding.PerColumn == nil {
			dst.Padding.PerColumn = make([]tw.Padding, len(src.Padding.PerColumn))
		} else if len(src.Padding.PerColumn) > len(dst.Padding.PerColumn) {
			dst.Padding.PerColumn = append(dst.Padding.PerColumn, make([]tw.Padding, len(src.Padding.PerColumn)-len(dst.Padding.PerColumn))...)
		}
		for i, pad := range src.Padding.PerColumn {
			if pad.Paddable() {
				dst.Padding.PerColumn[i] = pad
			}
		}
	}
	if src.Callbacks.Global != nil {
		dst.Callbacks.Global = src.Callbacks.Global
	}
	if len(src.Callbacks.PerColumn) > 0 {
		if dst.Callbacks.PerColumn == nil {
			dst.Callbacks.PerColumn = make([]func(), len(src.Callbacks.PerColumn))
		} else if len(src.Callbacks.PerColumn) > len(dst.Callbacks.PerColumn) {
			dst.Callbacks.PerColumn = append(dst.Callbacks.PerColumn, make([]func(), len(src.Callbacks.PerColumn)-len(dst.Callbacks.PerColumn))...)
		}
		for i, cb := range src.Callbacks.PerColumn {
			if cb != nil {
				dst.Callbacks.PerColumn[i] = cb
			}
		}
	}
	if src.Filter.Global != nil {
		dst.Filter.Global = src.Filter.Global
	}
	if len(src.Filter.PerColumn) > 0 {
		if dst.Filter.PerColumn == nil {
			dst.Filter.PerColumn = make([]func(string) string, len(src.Filter.PerColumn))
		} else if len(src.Filter.PerColumn) > len(dst.Filter.PerColumn) {
			dst.Filter.PerColumn = append(dst.Filter.PerColumn, make([]func(string) string, len(src.Filter.PerColumn)-len(dst.Filter.PerColumn))...)
		}
		for i, filter := range src.Filter.PerColumn {
			if filter != nil {
				dst.Filter.PerColumn[i] = filter
			}
		}
	}

	// Merge Alignment
	if src.Alignment.Global != tw.Empty {
		dst.Alignment.Global = src.Alignment.Global
	}

	if len(src.Alignment.PerColumn) > 0 {
		if dst.Alignment.PerColumn == nil {
			dst.Alignment.PerColumn = make([]tw.Align, len(src.Alignment.PerColumn))
		} else if len(src.Alignment.PerColumn) > len(dst.Alignment.PerColumn) {
			dst.Alignment.PerColumn = append(dst.Alignment.PerColumn, make([]tw.Align, len(src.Alignment.PerColumn)-len(dst.Alignment.PerColumn))...)
		}
		for i, align := range src.Alignment.PerColumn {
			if align != tw.Skip {
				dst.Alignment.PerColumn[i] = align
			}
		}
	}

	if len(src.ColumnAligns) > 0 {
		if dst.ColumnAligns == nil {
			dst.ColumnAligns = make([]tw.Align, len(src.ColumnAligns))
		} else if len(src.ColumnAligns) > len(dst.ColumnAligns) {
			dst.ColumnAligns = append(dst.ColumnAligns, make([]tw.Align, len(src.ColumnAligns)-len(dst.ColumnAligns))...)
		}
		for i, align := range src.ColumnAligns {
			if align != tw.Skip {
				dst.ColumnAligns[i] = align
			}
		}
	}

	if len(src.ColMaxWidths.PerColumn) > 0 {
		if dst.ColMaxWidths.PerColumn == nil {
			dst.ColMaxWidths.PerColumn = make(map[int]int)
		}
		for k, v := range src.ColMaxWidths.PerColumn {
			if v != 0 {
				dst.ColMaxWidths.PerColumn[k] = v
			}
		}
	}
	return dst
}

// mergeConfig merges a source Config into a destination Config, prioritizing non-default source values.
// It performs deep merging for complex types like Header, Row, Footer, and Stream.
func mergeConfig(dst, src Config) Config {
	if src.MaxWidth != 0 {
		dst.MaxWidth = src.MaxWidth
	}

	dst.Debug = src.Debug || dst.Debug
	dst.Behavior.AutoHide = src.Behavior.AutoHide
	dst.Behavior.TrimSpace = src.Behavior.TrimSpace
	dst.Behavior.TrimTab = src.Behavior.TrimTab
	dst.Behavior.Compact = src.Behavior.Compact
	dst.Behavior.Header = src.Behavior.Header
	dst.Behavior.Footer = src.Behavior.Footer
	dst.Behavior.Footer = src.Behavior.Footer

	dst.Behavior.Structs.AutoHeader = src.Behavior.Structs.AutoHeader

	// check lent of tags
	if len(src.Behavior.Structs.Tags) > 0 {
		dst.Behavior.Structs.Tags = src.Behavior.Structs.Tags
	}

	if src.Widths.Global != 0 {
		dst.Widths.Global = src.Widths.Global
	}
	if len(src.Widths.PerColumn) > 0 {
		if dst.Widths.PerColumn == nil {
			dst.Widths.PerColumn = make(map[int]int)
		}
		for k, v := range src.Widths.PerColumn {
			if v != 0 {
				dst.Widths.PerColumn[k] = v
			}
		}
	}

	dst.Header = mergeCellConfig(dst.Header, src.Header)
	dst.Row = mergeCellConfig(dst.Row, src.Row)
	dst.Footer = mergeCellConfig(dst.Footer, src.Footer)
	dst.Stream = mergeStreamConfig(dst.Stream, src.Stream)

	return dst
}

// mergeStreamConfig merges a source StreamConfig into a destination StreamConfig, prioritizing non-default source values.
func mergeStreamConfig(dst, src tw.StreamConfig) tw.StreamConfig {
	if src.Enable {
		dst.Enable = true
	}

	dst.StrictColumns = src.StrictColumns
	return dst
}

// padLine pads a line to the specified column count by appending empty strings as needed.
func padLine(line []string, numCols int) []string {
	if len(line) >= numCols {
		return line
	}
	padded := make([]string, numCols)
	copy(padded, line)
	for i := len(line); i < numCols; i++ {
		padded[i] = tw.Empty
	}
	return padded
}
