package tablewriter

import (
	"github.com/mattn/go-runewidth"
	"github.com/olekukonko/tablewriter/pkg/twwidth"
	"github.com/olekukonko/tablewriter/tw"
)

// WithBorders configures the table's border settings by updating the renderer's border configuration.
// This function is deprecated and will be removed in a future version.
//
// Deprecated: Use [WithRendition] to configure border settings for renderers that support
// [tw.Renditioning], or update the renderer's [tw.RenderConfig] directly via its Config() method.
// This function has no effect if no renderer is set on the table.
//
// Example migration:
//
//	// Old (deprecated)
//	table.Options(WithBorders(tw.Border{Top: true, Bottom: true}))
//	// New (recommended)
//	table.Options(WithRendition(tw.Rendition{Borders: tw.Border{Top: true, Bottom: true}}))
//
// Parameters:
//   - borders: The [tw.Border] configuration to apply to the renderer's borders.
//
// Returns:
//
//	An [Option] that updates the renderer's border settings if a renderer is set.
//	Logs a debug message if debugging is enabled and a renderer is present.
func WithBorders(borders tw.Border) Option {
	return func(target *Table) {
		if target.renderer != nil {
			cfg := target.renderer.Config()
			cfg.Borders = borders
			if target.logger != nil {
				target.logger.Debugf("Option: WithBorders applied to Table: %+v", borders)
			}
		}
	}
}

// Behavior is an alias for [tw.Behavior] to configure table behavior settings.
// This type is deprecated and will be removed in a future version.
//
// Deprecated: Use [tw.Behavior] directly to configure settings such as auto-hiding empty
// columns, trimming spaces, or controlling header/footer visibility.
//
// Example migration:
//
//	// Old (deprecated)
//	var b tablewriter.Behavior = tablewriter.Behavior{AutoHide: tw.On}
//	// New (recommended)
//	var b tw.Behavior = tw.Behavior{AutoHide: tw.On}
type Behavior tw.Behavior

// Settings is an alias for [tw.Settings] to configure renderer settings.
// This type is deprecated and will be removed in a future version.
//
// Deprecated: Use [tw.Settings] directly to configure renderer settings, such as
// separators and line styles.
//
// Example migration:
//
//	// Old (deprecated)
//	var s tablewriter.Settings = tablewriter.Settings{Separator: "|"}
//	// New (recommended)
//	var s tw.Settings = tw.Settings{Separator: "|"}
type Settings tw.Settings

// WithRendererSettings updates the renderer's settings, such as separators and line styles.
// This function is deprecated and will be removed in a future version.
//
// Deprecated: Use [WithRendition] to update renderer settings for renderers that implement
// [tw.Renditioning], or configure the renderer's [tw.Settings] directly via its
// [tw.Renderer.Config] method. This function has no effect if no renderer is set.
//
// Example migration:
//
//	// Old (deprecated)
//	table.Options(WithRendererSettings(tw.Settings{Separator: "|"}))
//	// New (recommended)
//	table.Options(WithRendition(tw.Rendition{Settings: tw.Settings{Separator: "|"}}))
//
// Parameters:
//   - settings: The [tw.Settings] configuration to apply to the renderer.
//
// Returns:
//
//	An [Option] that updates the renderer's settings if a renderer is set.
//	Logs a debug message if debugging is enabled and a renderer is present.
func WithRendererSettings(settings tw.Settings) Option {
	return func(target *Table) {
		if target.renderer != nil {
			cfg := target.renderer.Config()
			cfg.Settings = settings
			if target.logger != nil {
				target.logger.Debugf("Option: WithRendererSettings applied to Table: %+v", settings)
			}
		}
	}
}

// WithAlignment sets the text alignment for footer cells within the formatting configuration.
// This method is deprecated and will be removed in the next version.
//
// Deprecated: Use [FooterConfigBuilder.Alignment] with [AlignmentConfigBuilder.WithGlobal]
// or [AlignmentConfigBuilder.WithPerColumn] to configure footer alignments.
// Alternatively, apply a complete [tw.CellAlignment] configuration using
// [WithFooterAlignmentConfig].
//
// Example migration:
//
//	// Old (deprecated)
//	builder.Footer().Formatting().WithAlignment(tw.AlignRight)
//	// New (recommended)
//	builder.Footer().Alignment().WithGlobal(tw.AlignRight)
//	// Or
//	table.Options(WithFooterAlignmentConfig(tw.CellAlignment{Global: tw.AlignRight}))
//
// Parameters:
//   - align: The [tw.Align] value to set for footer cells. Valid values are
//     [tw.AlignLeft], [tw.AlignRight], [tw.AlignCenter], and [tw.AlignNone].
//     Invalid alignments are ignored.
//
// Returns:
//
//	The [FooterFormattingBuilder] instance for method chaining.
func (ff *FooterFormattingBuilder) WithAlignment(align tw.Align) *FooterFormattingBuilder {
	if align != tw.AlignLeft && align != tw.AlignRight && align != tw.AlignCenter && align != tw.AlignNone {
		return ff
	}
	ff.config.Alignment = align
	return ff
}

// WithAlignment sets the text alignment for header cells within the formatting configuration.
// This method is deprecated and will be removed in the next version.
//
// Deprecated: Use [HeaderConfigBuilder.Alignment] with [AlignmentConfigBuilder.WithGlobal]
// or [AlignmentConfigBuilder.WithPerColumn] to configure header alignments.
// Alternatively, apply a complete [tw.CellAlignment] configuration using
// [WithHeaderAlignmentConfig].
//
// Example migration:
//
//	// Old (deprecated)
//	builder.Header().Formatting().WithAlignment(tw.AlignCenter)
//	// New (recommended)
//	builder.Header().Alignment().WithGlobal(tw.AlignCenter)
//	// Or
//	table.Options(WithHeaderAlignmentConfig(tw.CellAlignment{Global: tw.AlignCenter}))
//
// Parameters:
//   - align: The [tw.Align] value to set for header cells. Valid values are
//     [tw.AlignLeft], [tw.AlignRight], [tw.AlignCenter], and [tw.AlignNone].
//     Invalid alignments are ignored.
//
// Returns:
//
//	The [HeaderFormattingBuilder] instance for method chaining.
func (hf *HeaderFormattingBuilder) WithAlignment(align tw.Align) *HeaderFormattingBuilder {
	if align != tw.AlignLeft && align != tw.AlignRight && align != tw.AlignCenter && align != tw.AlignNone {
		return hf
	}
	hf.config.Alignment = align
	return hf
}

// WithAlignment sets the text alignment for row cells within the formatting configuration.
// This method is deprecated and will be removed in the next version.
//
// Deprecated: Use [RowConfigBuilder.Alignment] with [AlignmentConfigBuilder.WithGlobal]
// or [AlignmentConfigBuilder.WithPerColumn] to configure row alignments.
// Alternatively, apply a complete [tw.CellAlignment] configuration using
// [WithRowAlignmentConfig].
//
// Example migration:
//
//	// Old (deprecated)
//	builder.Row().Formatting().WithAlignment(tw.AlignLeft)
//	// New (recommended)
//	builder.Row().Alignment().WithGlobal(tw.AlignLeft)
//	// Or
//	table.Options(WithRowAlignmentConfig(tw.CellAlignment{Global: tw.AlignLeft}))
//
// Parameters:
//   - align: The [tw.Align] value to set for row cells. Valid values are
//     [tw.AlignLeft], [tw.AlignRight], [tw.AlignCenter], and [tw.AlignNone].
//     Invalid alignments are ignored.
//
// Returns:
//
//	The [RowFormattingBuilder] instance for method chaining.
func (rf *RowFormattingBuilder) WithAlignment(align tw.Align) *RowFormattingBuilder {
	if align != tw.AlignLeft && align != tw.AlignRight && align != tw.AlignCenter && align != tw.AlignNone {
		return rf
	}
	rf.config.Alignment = align
	return rf
}

// WithTableMax sets the maximum width of the entire table in characters.
// Negative values are ignored, and the change is logged if debugging is enabled.
// The width constrains the table's rendering, potentially causing text wrapping or truncation
// based on the configuration's wrapping settings (e.g., tw.WrapTruncate).
// If debug logging is enabled via WithDebug(true), the applied width is logged.
//
// Deprecated: Use WithMaxWidth instead, which provides the same functionality with a clearer name
// and consistent naming across the package. For example:
//
//	tablewriter.NewTable(os.Stdout, tablewriter.WithMaxWidth(80))
func WithTableMax(width int) Option {
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

// Deprecated: use WithEastAsian instead.
// WithCondition provides a way to set a custom global runewidth.Condition
// that will be used for all subsequent display width calculations by the twwidth (twdw) package.
//
// The runewidth.Condition object allows for more fine-grained control over how rune widths
// are determined, beyond just toggling EastAsianWidth. This could include settings for
// ambiguous width characters or other future properties of runewidth.Condition.
func WithCondition(cond *runewidth.Condition) Option {
	return func(target *Table) {
		twwidth.SetCondition(cond)
	}
}
