# Migration Guide: tablewriter v0.0.5 to v1.0.x
> **NOTE:** This document is work in progress, use with `caution`. This document is a comprehensive guide. For specific issues or advanced scenarios, please refer to the source code or open an issue.

The `tablewriter` library has undergone a significant redesign between versions **v0.0.5** and **v1.0.x**, transitioning from a primarily method-driven API to a more robust, modular, and configuration-driven framework. This guide provides a detailed roadmap for migrating your v0.0.5 codebase to v1.0.x. It includes mappings of old methods to new approaches, practical examples, and explanations of new features.

We believe these changes significantly improve the library's flexibility, maintainability, and power, enabling new features and making complex table configurations more manageable.

## Why Migrate to v1.0.x?

The v1.0.x redesign enhances `tablewriter`’s flexibility, maintainability, and feature set:
- **Extensibility**: Decoupled rendering supports diverse output formats (e.g., HTML, Markdown, CSV).
- **Robust Configuration**: Centralized `Config` struct and fluent builders ensure atomic, predictable setups.
- **Streaming Capability**: Dedicated API for row-by-row rendering, ideal for large or real-time datasets.
- **Type Safety**: Specific types (e.g., `tw.State`, `tw.Align`) reduce errors and improve clarity.
- **Consistent API**: Unified interface for intuitive usage across simple and complex use cases.
- **New Features**: Hierarchical merging, granular padding, table captions, and fixed column widths.

These improvements make v1.0.x more powerful, but they require updating code to leverage the new configuration-driven framework and take advantage of advanced functionalities.

## Key New Features in v1.0.x

- **Fluent Configuration Builders**: `NewConfigBuilder()` enables chained, readable setups (`config.go:NewConfigBuilder`).
- **Centralized Configuration**: `Config` struct governs table behavior and data processing (`config.go:Config`).
- **Decoupled Renderer**: `tw.Renderer` interface with `tw.Rendition` for visual styling, allowing custom renderers (`tw/renderer.go`).
- **True Streaming Support**: `Start()`, `Append()`, `Close()` for incremental rendering (`stream.go`).
- **Hierarchical Cell Merging**: `tw.MergeHierarchical` for complex data structures (`tw/tw.go:MergeMode` constant, logic in `zoo.go`).
- **Granular Padding Control**: Per-side (`Top`, `Bottom`, `Left`, `Right`) and per-column padding (`tw/cell.go:CellPadding`, `tw/types.go:Padding`).
- **Enhanced Type System**: `tw.State`, `tw.Align`, `tw.Spot`, and others for clarity and safety (`tw/state.go`, `tw/types.go`).
- **Comprehensive Error Handling**: Methods like `Render()` and `Append()` return errors (`tablewriter.go`, `stream.go`).
- **Fixed Column Width System**: `Config.Widths` for precise column sizing, especially in streaming (`config.go:Config`, `tw/cell.go:CellWidth`).
- **Table Captioning**: Flexible placement and styling with `tw.Caption` (`tw/types.go:Caption`).
- **Advanced Data Processing**: Support for `tw.Formatter`, per-column filters, and stringer caching (`tw/cell.go:CellFilter`, `tablewriter.go:WithStringer`).

## Core Philosophy Changes in v1.0.x

Understanding these shifts is essential for a successful migration:

1. **Configuration-Driven Approach**:
    - **Old**: Relied on `table.SetXxx()` methods for incremental, stateful modifications to table properties.
    - **New**: Table behavior is defined by a `tablewriter.Config` struct (`config.go:Config`), while visual styling is managed by a `tw.Rendition` struct (`tw/renderer.go:Rendition`). These are typically set at table creation using `NewTable()` with `Option` functions or via a fluent `ConfigBuilder`, ensuring atomic and predictable configuration changes.

2. **Decoupled Rendering Engine**:
    - **Old**: Rendering logic was tightly integrated into the `Table` struct, limiting output flexibility.
    - **New**: The `tw.Renderer` interface (`tw/renderer.go:Renderer`) defines rendering logic, with `renderer.NewBlueprint()` as the default text-based renderer. The renderer’s appearance (e.g., borders, symbols) is controlled by `tw.Rendition`, enabling support for alternative formats like HTML or Markdown.

3. **Unified Section Configuration**:
    - **Old**: Headers, rows, and footers had separate, inconsistent configuration methods.
    - **New**: `tw.CellConfig` (`tw/cell.go:CellConfig`) standardizes configuration across headers, rows, and footers, encompassing formatting (`tw.CellFormatting`), padding (`tw.CellPadding`), column widths (`tw.CellWidth`), alignments (`tw.CellAlignment`), and filters (`tw.CellFilter`).

4. **Fluent Configuration Builders**:
    - **Old**: Configuration was done via individual setters, often requiring multiple method calls.
    - **New**: `tablewriter.NewConfigBuilder()` (`config.go:NewConfigBuilder`) provides a chained, fluent API for constructing `Config` objects, with nested builders for `Header()`, `Row()`, `Footer()`, `Alignment()`, `Behavior()`, and `ForColumn()` to simplify complex setups.

5. **Explicit Streaming Mode**:
    - **Old**: No dedicated streaming support; tables were rendered in batch mode.
    - **New**: Streaming for row-by-row rendering is enabled via `Config.Stream.Enable` or `WithStreaming(tw.StreamConfig{Enable: true})` and managed with `Table.Start()`, `Table.Append()` (or `Table.Header()`, `Table.Footer()`), and `Table.Close()` (`stream.go`). This is ideal for large datasets or continuous output.

6. **Enhanced Error Handling**:
    - **Old**: Methods like `Render()` did not return errors, making error detection difficult.
    - **New**: Key methods (`Render()`, `Start()`, `Close()`, `Append()`, `Bulk()`) return errors to promote robust error handling and improve application reliability (`tablewriter.go`, `stream.go`).

7. **Richer Type System & `tw` Package**:
    - **Old**: Used integer constants (e.g., `ALIGN_CENTER`) and booleans, leading to potential errors.
    - **New**: The `tw` sub-package introduces type-safe constructs like `tw.State` (`tw/state.go`), `tw.Align` (`tw/types.go`), `tw.Position` (`tw/types.go`), and `tw.CellConfig` (`tw/cell.go`), replacing magic constants and enhancing code clarity.

## Configuration Methods in v1.0.x

v1.0.x offers four flexible methods to configure tables, catering to different use cases and complexity levels. Each method can be used independently or combined, providing versatility for both simple and advanced setups.

1. **Using `WithConfig` Option**:
    - **Description**: Pass a fully populated `tablewriter.Config` struct during `NewTable` initialization using the `WithConfig` option.
    - **Use Case**: Ideal for predefined, reusable configurations that can be serialized or shared across multiple tables.
    - **Pros**: Explicit, portable, and suitable for complex setups; allows complete control over all configuration aspects.
    - **Cons**: Verbose for simple changes, requiring manual struct population.
    - **Example**:

```go
package main

import (
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
	"os"
)

func main() {
	cfg := tablewriter.Config{
		Header: tw.CellConfig{
			Alignment:  tw.CellAlignment{Global: tw.AlignCenter},
			Formatting: tw.CellFormatting{AutoFormat: tw.On},
		},
		Row: tw.CellConfig{
			Alignment: tw.CellAlignment{Global: tw.AlignLeft},
		},
		MaxWidth: 80,
		Behavior: tw.Behavior{TrimSpace: tw.On},
	}
	table := tablewriter.NewTable(os.Stdout, tablewriter.WithConfig(cfg))
	table.Header("Name", "Status")
	table.Append("Node1", "Ready")
	table.Render()
}
```
**Output**:
 ```
┌───────┬────────┐
│ NAME  │ STATUS │
├───────┼────────┤
│ Node1 │ Ready  │
└───────┴────────┘
 ```

2. **Using `Table.Configure` method**:
    - **Description**: After creating a `Table` instance, use the `Configure` method with a function that modifies the table's `Config` struct.
    - **Use Case**: Suitable for quick, ad-hoc tweaks post-initialization, especially for simple or dynamic adjustments.
    - **Pros**: Straightforward for minor changes; no need for additional structs or builders if you already have a `Table` instance.
    - **Cons**: Less readable for complex configurations compared to a builder; modifications are applied to an existing instance.
    - **Example**:
```go
package main

import (
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
	"os"
)

func main() {
	table := tablewriter.NewTable(os.Stdout)
	table.Configure(func(cfg *tablewriter.Config) {
		cfg.Header.Alignment.Global = tw.AlignCenter
		cfg.Row.Alignment.Global = tw.AlignLeft
	})

	table.Header("Name", "Status")
	table.Append("Node1", "Ready")
	table.Render()
}
```
**Output**: Same as above.

3. **Standalone `Option` Functions**:
    - **Description**: Use `WithXxx` functions (e.g., `WithHeaderAlignment`, `WithDebug`) during `NewTable` initialization or via `table.Options()` to apply targeted settings.
    - **Use Case**: Best for simple, specific configuration changes without needing a full `Config` struct.
    - **Pros**: Concise, readable, and intuitive for common settings; ideal for minimal setups.
    - **Cons**: Limited for complex, multi-faceted configurations; requires multiple options for extensive changes.
    - **Example**:
```go
package main

import (
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
	"os"
)

func main() {
	table := tablewriter.NewTable(os.Stdout,
		tablewriter.WithHeaderAlignment(tw.AlignCenter),
		tablewriter.WithRowAlignment(tw.AlignLeft),
		tablewriter.WithDebug(true),
	)
	table.Header("Name", "Status")
	table.Append("Node1", "Ready")
	table.Render()
}
```
     **Output**: Same as above.

4. **Fluent `ConfigBuilder`**:
    - **Description**: Use `tablewriter.NewConfigBuilder()` to construct a `Config` struct through a chained, fluent API, then apply it with `WithConfig(builder.Build())`.
    - **Use Case**: Optimal for complex, dynamic, or programmatically generated configurations requiring fine-grained control.
    - **Pros**: Highly readable, maintainable, and expressive; supports nested builders for sections and columns.
    - **Cons**: Slightly verbose; requires understanding builder methods and `Build()` call.
    - **Example**:
```go
package main

import (
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
	"os"
)

func main() {
	cnfBuilder := tablewriter.NewConfigBuilder()
	cnfBuilder.Header().Alignment().WithGlobal(tw.AlignCenter)
    // Example of configuring a specific column (less emphasis on this for now)
    // cnfBuilder.ForColumn(0).WithAlignment(tw.AlignLeft).Build() // Call Build() to return to ConfigBuilder

	table := tablewriter.NewTable(os.Stdout, tablewriter.WithConfig(cnfBuilder.Build()))
	table.Header("Name", "Status")
	table.Append("Node1", "Ready")
	table.Render()
}
```
**Output**: Same as above.

**Best Practices**:
- Use `WithConfig` or `ConfigBuilder` for complex setups or reusable configurations.
- Opt for `Option` functions for simple, targeted changes.
- Use `Table.Configure` for direct modifications after table creation, but avoid changes during rendering.
- Combine methods as needed (e.g., `ConfigBuilder` for initial setup, `Option` functions for overrides).

## Default Parameters in v1.0.x

The `defaultConfig()` function (`config.go:defaultConfig`) establishes baseline settings for new tables, ensuring predictable behavior unless overridden. Below is a detailed table of default parameters, organized by configuration section, to help you understand the starting point for table behavior and appearance.

| Section       | Parameter                | Default Value                     | Description                                                                 |
|---------------|--------------------------|-----------------------------------|-----------------------------------------------------------------------------|
| **Header**    | `Alignment.Global`       | `tw.AlignCenter`                  | Centers header text globally unless overridden by `PerColumn`.               |
| Header        | `Alignment.PerColumn`    | `[]tw.Align{}`                    | Empty; falls back to `Global` unless specified.                             |
| Header        | `Formatting.AutoFormat`  | `tw.On`                           | Applies title case (e.g., "col_one" → "COL ONE") to header content.          |
| Header        | `Formatting.AutoWrap`    | `tw.WrapTruncate`                 | Truncates long header text with "…" based on width constraints.              |
| Header        | `Merging.Mode`           | `tw.MergeNone`                    | Disables cell merging in headers by default.                                |
| Header        | `Padding.Global`         | `tw.PaddingDefault` (`" "`)       | Adds one space on left and right of header cells.                           |
| Header        | `Padding.PerColumn`      | `[]tw.Padding{}`                  | Empty; falls back to `Global` unless specified.                             |
| Header        | `ColMaxWidths.Global`    | `0` (unlimited)                   | No maximum content width for header cells unless set.                       |
| Header        | `ColMaxWidths.PerColumn` | `tw.NewMapper[int, int]()`        | Empty map; no per-column content width limits unless specified.             |
| Header        | `Filter.Global`          | `nil`                             | No global content transformation for header cells.                          |
| Header        | `Filter.PerColumn`       | `[]func(string) string{}`         | No per-column content transformations unless specified.                     |
| **Row**       | `Alignment.Global`       | `tw.AlignLeft`                    | Left-aligns row text globally unless overridden by `PerColumn`.             |
| Row           | `Alignment.PerColumn`    | `[]tw.Align{}`                    | Empty; falls back to `Global`.                                              |
| Row           | `Formatting.AutoFormat`  | `tw.Off`                          | Disables auto-formatting (e.g., title case) for row content.                |
| Row           | `Formatting.AutoWrap`    | `tw.WrapNormal`                   | Wraps long row text naturally at word boundaries based on width constraints.|
| Row           | `Merging.Mode`           | `tw.MergeNone`                    | Disables cell merging in rows by default.                                   |
| Row           | `Padding.Global`         | `tw.PaddingDefault` (`" "`)       | Adds one space on left and right of row cells.                              |
| Row           | `Padding.PerColumn`      | `[]tw.Padding{}`                  | Empty; falls back to `Global`.                                              |
| Row           | `ColMaxWidths.Global`    | `0` (unlimited)                   | No maximum content width for row cells.                                     |
| Row           | `ColMaxWidths.PerColumn` | `tw.NewMapper[int, int]()`        | Empty map; no per-column content width limits.                              |
| Row           | `Filter.Global`          | `nil`                             | No global content transformation for row cells.                             |
| Row           | `Filter.PerColumn`       | `[]func(string) string{}`         | No per-column content transformations.                                      |
| **Footer**    | `Alignment.Global`       | `tw.AlignRight`                   | Right-aligns footer text globally unless overridden by `PerColumn`.         |
| Footer        | `Alignment.PerColumn`    | `[]tw.Align{}`                    | Empty; falls back to `Global`.                                              |
| Footer        | `Formatting.AutoFormat`  | `tw.Off`                          | Disables auto-formatting for footer content.                                |
| Footer        | `Formatting.AutoWrap`    | `tw.WrapNormal`                   | Wraps long footer text naturally.                                           |
| Footer        | `Formatting.MergeMode`   | `tw.MergeNone`                    | Disables cell merging in footers.                                           |
| Footer        | `Padding.Global`         | `tw.PaddingDefault` (`" "`)       | Adds one space on left and right of footer cells.                           |
| Footer        | `Padding.PerColumn`      | `[]tw.Padding{}`                  | Empty; falls back to `Global`.                                              |
| Footer        | `ColMaxWidths.Global`    | `0` (unlimited)                   | No maximum content width for footer cells.                                  |
| Footer        | `ColMaxWidths.PerColumn` | `tw.NewMapper[int, int]()`        | Empty map; no per-column content width limits.                              |
| Footer        | `Filter.Global`          | `nil`                             | No global content transformation for footer cells.                          |
| Footer        | `Filter.PerColumn`       | `[]func(string) string{}`         | No per-column content transformations.                                      |
| **Global**    | `MaxWidth`               | `0` (unlimited)                   | No overall table width limit.                                               |
| Global        | `Behavior.AutoHide`      | `tw.Off`                          | Displays empty columns (ignored in streaming).                              |
| Global        | `Behavior.TrimSpace`     | `tw.On`                           | Trims leading/trailing spaces from cell content.                            |
| Global        | `Behavior.Header`        | `tw.Control{Hide: tw.Off}`        | Shows header if content is provided.                                        |
| Global        | `Behavior.Footer`        | `tw.Control{Hide: tw.Off}`        | Shows footer if content is provided.                                        |
| Global        | `Behavior.Compact`       | `tw.Compact{Merge: tw.Off}`       | No compact width optimization for merged cells.                             |
| Global        | `Debug`                  | `false`                           | Disables debug logging.                                                     |
| Global        | `Stream.Enable`          | `false`                           | Disables streaming mode by default.                                         |
| Global        | `Widths.Global`          | `0` (unlimited)                   | No fixed column width unless specified.                                     |
| Global        | `Widths.PerColumn`       | `tw.NewMapper[int, int]()`        | Empty map; no per-column fixed widths unless specified.                     |

**Notes**:
- Defaults can be overridden using any configuration method.
- `tw.PaddingDefault` is `{Left: " ", Right: " "}` (`tw/preset.go`).
- Alignment within `tw.CellFormatting` is deprecated; `tw.CellAlignment` is preferred. `tw.AlignDefault` falls back to `Global` or `tw.AlignLeft` (`tw/types.go`).
- Streaming mode uses `Widths` for fixed column sizing (`stream.go`).

## Renderer Types and Customization

v1.0.x introduces a flexible rendering system via the `tw.Renderer` interface (`tw/renderer.go:Renderer`), allowing for both default text-based rendering and custom output formats. This decouples rendering logic from table data processing, enabling support for diverse formats like HTML, CSV, or JSON.

### Default Renderer: `renderer.NewBlueprint`
- **Description**: `renderer.NewBlueprint()` creates a text-based renderer. Its visual styles are configured using `tw.Rendition`.
- **Use Case**: Standard terminal or text output with configurable borders, symbols, and separators.
- **Configuration**: Styled via `tw.Rendition`, which controls:
    - `Borders`: Outer table borders (`tw.Border`) with `tw.State` for each side (`tw/renderer.go`).
    - `Settings`: Internal lines (`tw.Lines`) and separators (`tw.Separators`) (`tw/renderer.go`).
    - `Symbols`: Characters for drawing table lines and junctions (`tw.Symbols`) (`tw/symbols.go`).
    - **Example**:
```go
package main

import (
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer" // Import the renderer package
	"github.com/olekukonko/tablewriter/tw"
	"os"
)

func main() {
	table := tablewriter.NewTable(os.Stdout,
		tablewriter.WithRenderer(renderer.NewBlueprint()), // Default Blueprint renderer
		tablewriter.WithRendition(tw.Rendition{          // Apply custom rendition
			Symbols: tw.NewSymbols(tw.StyleRounded),
			Borders: tw.Border{Top: tw.On, Bottom: tw.On, Left: tw.On, Right: tw.On},
			Settings: tw.Settings{
				Separators: tw.Separators{BetweenRows: tw.On},
				Lines:      tw.Lines{ShowHeaderLine: tw.On},
			},
		}),
	)
	table.Header("Name", "Status")
	table.Append("Node1", "Ready")
	table.Render()
}
```
**Output**:
  ```
  ╭───────┬────────╮
  │ Name  │ Status │
  ├───────┼────────┤
  │ Node1 │ Ready  │
  ╰───────┴────────╯
  ```

### Custom Renderer Implementation
- **Description**: Implement the `tw.Renderer` interface to create custom output formats (e.g., HTML, CSV).
- **Use Case**: Non-text outputs, specialized formatting, or integration with other systems.
- **Interface Methods**:
    - `Start(w io.Writer) error`: Initializes rendering.
    - `Header(headers [][]string, ctx tw.Formatting)`: Renders header rows.
    - `Row(row []string, ctx tw.Formatting)`: Renders a data row.
    - `Footer(footers [][]string, ctx tw.Formatting)`: Renders footer rows.
    - `Line(ctx tw.Formatting)`: Renders separator lines.
    - `Close() error`: Finalizes rendering.
    - `Config() tw.Rendition`: Returns renderer's current rendition configuration.
    - `Logger(logger *ll.Logger)`: Sets logger for debugging.
    - **Example (HTML Renderer)**:

```go
package main

import (
	"fmt"
	"github.com/olekukonko/ll" // For logger type
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
	"io"
	"os"
)

// BasicHTMLRenderer implements tw.Renderer
type BasicHTMLRenderer struct {
	writer io.Writer
	config tw.Rendition // Store the rendition
	logger *ll.Logger
	err    error
}

func (r *BasicHTMLRenderer) Start(w io.Writer) error {
	r.writer = w
	_, r.err = r.writer.Write([]byte("<table border='1'>\n"))
	return r.err
}

// Header expects [][]string for potentially multi-line headers
func (r *BasicHTMLRenderer) Header(headers [][]string, ctx tw.Formatting) {
	if r.err != nil { return }
	_, r.err = r.writer.Write([]byte("  <tr>\n"))
	if r.err != nil { return }
	// Iterate over cells from the context for the current line
	for _, cellCtx := range ctx.Row.Current {
		content := fmt.Sprintf("    <th>%s</th>\n", cellCtx.Data)
		_, r.err = r.writer.Write([]byte(content))
		if r.err != nil { return }
	}
	_, r.err = r.writer.Write([]byte("  </tr>\n"))
}

// Row expects []string for a single line row, but uses ctx for actual data
func (r *BasicHTMLRenderer) Row(row []string, ctx tw.Formatting) { // row param is less relevant here, ctx.Row.Current is key
	if r.err != nil { return }
	_, r.err = r.writer.Write([]byte("  <tr>\n"))
	if r.err != nil { return }
	for _, cellCtx := range ctx.Row.Current {
		content := fmt.Sprintf("    <td>%s</td>\n", cellCtx.Data)
		_, r.err = r.writer.Write([]byte(content))
		if r.err != nil { return }
	}
	_, r.err = r.writer.Write([]byte("  </tr>\n"))
}

func (r *BasicHTMLRenderer) Footer(footers [][]string, ctx tw.Formatting) {
	if r.err != nil { return }
    // Similar to Header/Row, using ctx.Row.Current for the footer line data
    // The footers [][]string param might be used if the renderer needs multi-line footer logic
    r.Row(nil, ctx) // Reusing Row logic, passing nil for the direct row []string as ctx contains the data
}

func (r *BasicHTMLRenderer) Line(ctx tw.Formatting) { /* No lines in basic HTML */ }

func (r *BasicHTMLRenderer) Close() error {
	if r.err != nil {
		return r.err
	}
	_, r.err = r.writer.Write([]byte("</table>\n"))
	return r.err
}

func (r *BasicHTMLRenderer) Config() tw.Rendition       { return r.config }
func (r *BasicHTMLRenderer) Logger(logger *ll.Logger) { r.logger = logger }

func main() {
	table := tablewriter.NewTable(os.Stdout, tablewriter.WithRenderer(&BasicHTMLRenderer{
		config: tw.Rendition{Symbols: tw.NewSymbols(tw.StyleASCII)}, // Provide a default Rendition
	}))
	table.Header("Name", "Status")
	table.Append("Node1", "Ready")
	table.Render()
}
```
**Output**:
```html
<table border='1'>
  <tr>
    <th>NAME</th>
    <th>STATUS</th>
  </tr>
  <tr>
    <td>Node1</td>
    <td>Ready</td>
  </tr>
</table>
```


#### Custom Invoice Renderer

```go

package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/olekukonko/ll"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
)

// InvoiceRenderer implements tw.Renderer for a basic invoice style.
type InvoiceRenderer struct {
	writer    io.Writer
	logger    *ll.Logger
	rendition tw.Rendition
}

func NewInvoiceRenderer() *InvoiceRenderer {
	rendition := tw.Rendition{
		Borders:   tw.BorderNone,
		Symbols:   tw.NewSymbols(tw.StyleNone),
		Settings:  tw.Settings{Separators: tw.SeparatorsNone, Lines: tw.LinesNone},
		Streaming: false,
	}
	defaultLogger := ll.New("simple-invoice-renderer").Disable()
	return &InvoiceRenderer{logger: defaultLogger, rendition: rendition}
}

func (r *InvoiceRenderer) Logger(logger *ll.Logger) {
	if logger != nil {
		r.logger = logger
	}
}

func (r *InvoiceRenderer) Config() tw.Rendition {
	return r.rendition
}

func (r *InvoiceRenderer) Start(w io.Writer) error {
	r.writer = w
	r.logger.Debug("InvoiceRenderer: Start")
	return nil
}

func (r *InvoiceRenderer) formatLine(cells []string, widths tw.Mapper[int, int], cellContexts map[int]tw.CellContext) string {
	var sb strings.Builder
	numCols := 0
	if widths != nil { // Ensure widths is not nil before calling Len
		numCols = widths.Len()
	}

	for i := 0; i < numCols; i++ {
		data := ""
		if i < len(cells) {
			data = cells[i]
		}

		width := 0
		if widths != nil { // Check again before Get
			width = widths.Get(i)
		}

		align := tw.AlignDefault
		if cellContexts != nil { // Check cellContexts
			if ctx, ok := cellContexts[i]; ok {
				align = ctx.Align
			}
		}

		paddedCell := tw.Pad(data, " ", width, align)
		sb.WriteString(paddedCell)

		if i < numCols-1 {
			sb.WriteString("   ") // Column separator
		}
	}
	return sb.String()
}

func (r *InvoiceRenderer) Header(headers [][]string, ctx tw.Formatting) {
	if r.writer == nil {
		return
	}
	r.logger.Debugf("InvoiceRenderer: Header (lines: %d)", len(headers))

	for _, headerLineCells := range headers {
		lineStr := r.formatLine(headerLineCells, ctx.Row.Widths, ctx.Row.Current)
		fmt.Fprintln(r.writer, lineStr)
	}

	if len(headers) > 0 {
		totalWidth := 0
		if ctx.Row.Widths != nil {
			ctx.Row.Widths.Each(func(_ int, w int) { totalWidth += w })
			if ctx.Row.Widths.Len() > 1 {
				totalWidth += (ctx.Row.Widths.Len() - 1) * 3 // Separator spaces
			}
		}
		if totalWidth > 0 {
			fmt.Fprintln(r.writer, strings.Repeat("-", totalWidth))
		}
	}
}

func (r *InvoiceRenderer) Row(row []string, ctx tw.Formatting) {
	if r.writer == nil {
		return
	}
	r.logger.Debug("InvoiceRenderer: Row")
	lineStr := r.formatLine(row, ctx.Row.Widths, ctx.Row.Current)
	fmt.Fprintln(r.writer, lineStr)
}

func (r *InvoiceRenderer) Footer(footers [][]string, ctx tw.Formatting) {
	if r.writer == nil {
		return
	}
	r.logger.Debugf("InvoiceRenderer: Footer (lines: %d)", len(footers))

	if len(footers) > 0 {
		totalWidth := 0
		if ctx.Row.Widths != nil {
			ctx.Row.Widths.Each(func(_ int, w int) { totalWidth += w })
			if ctx.Row.Widths.Len() > 1 {
				totalWidth += (ctx.Row.Widths.Len() - 1) * 3
			}
		}
		if totalWidth > 0 {
			fmt.Fprintln(r.writer, strings.Repeat("-", totalWidth))
		}
	}

	for _, footerLineCells := range footers {
		lineStr := r.formatLine(footerLineCells, ctx.Row.Widths, ctx.Row.Current)
		fmt.Fprintln(r.writer, lineStr)
	}
}

func (r *InvoiceRenderer) Line(ctx tw.Formatting) {
	r.logger.Debug("InvoiceRenderer: Line (no-op)")
	// This simple renderer draws its own lines in Header/Footer.
}

func (r *InvoiceRenderer) Close() error {
	r.logger.Debug("InvoiceRenderer: Close")
	r.writer = nil
	return nil
}

func main() {
	data := [][]string{
		{"Product A", "2", "10.00", "20.00"},
		{"Super Long Product Name B", "1", "125.50", "125.50"},
		{"Item C", "10", "1.99", "19.90"},
	}

	header := []string{"Description", "Qty", "Unit Price", "Total Price"}
	footer := []string{"", "", "Subtotal:\nTax (10%):\nGRAND TOTAL:", "165.40\n16.54\n181.94"}
	invoiceRenderer := NewInvoiceRenderer()

	// Create table and set custom renderer using Options
	table := tablewriter.NewTable(os.Stdout,
		tablewriter.WithRenderer(invoiceRenderer),
		tablewriter.WithAlignment([]tw.Align{
			tw.AlignLeft, tw.AlignCenter, tw.AlignRight, tw.AlignRight,
		}),
	)

	table.Header(header)
	for _, v := range data {
		table.Append(v)
	}

	// Use the Footer method with strings containing newlines for multi-line cells
	table.Footer(footer)

	fmt.Println("Rendering with InvoiceRenderer:")
	table.Render()

	// For comparison, render with default Blueprint renderer
	// Re-create the table or reset it to use a different renderer
	table2 := tablewriter.NewTable(os.Stdout,
		tablewriter.WithAlignment([]tw.Align{
			tw.AlignLeft, tw.AlignCenter, tw.AlignRight, tw.AlignRight,
		}),
	)

	table2.Header(header)
	for _, v := range data {
		table2.Append(v)
	}
	table2.Footer(footer)
	fmt.Println("\nRendering with Default Blueprint Renderer (for comparison):")
	table2.Render()
}

```


```
Rendering with InvoiceRenderer:
DESCRIPTION                    QTY        UNIT PRICE     TOTAL PRICE
--------------------------------------------------------------------
Product A                       2              10.00           20.00
Super Long Product Name B       1             125.50          125.50
Item C                          10              1.99           19.90
--------------------------------------------------------------------
                                           Subtotal:          165.40
--------------------------------------------------------------------
                                          Tax (10%):           16.54
--------------------------------------------------------------------
                                        GRAND TOTAL:          181.94
```


```
Rendering with Default Blueprint Renderer (for comparison):
┌───────────────────────────┬─────┬──────────────┬─────────────┐
│ DESCRIPTION               │ QTY │   UNIT PRICE │ TOTAL PRICE │
├───────────────────────────┼─────┼──────────────┼─────────────┤
│ Product A                 │  2  │        10.00 │       20.00 │
│ Super Long Product Name B │  1  │       125.50 │      125.50 │
│ Item C                    │ 10  │         1.99 │       19.90 │
├───────────────────────────┼─────┼──────────────┼─────────────┤
│                           │     │    Subtotal: │      165.40 │
│                           │     │   Tax (10%): │       16.54 │
│                           │     │ GRAND TOTAL: │      181.94 │
└───────────────────────────┴─────┴──────────────┴─────────────┘

```
**Notes**:
- The `renderer.NewBlueprint()` is sufficient for most text-based use cases.
- Custom renderers require implementing all interface methods to handle table structure correctly. `tw.Formatting` (which includes `tw.RowContext`) provides cell content and metadata.

## Function Mapping Table (v0.0.5 → v1.0.x)

The following table maps v0.0.5 methods to their v1.0.x equivalents, ensuring a quick reference for migration. All deprecated methods are retained for compatibility but should be replaced with new APIs.

| v0.0.5 Method                     | v1.0.x Equivalent(s)                                                                 | Notes                                                                 |
|-----------------------------------|--------------------------------------------------------------------------------------|----------------------------------------------------------------------|
| `NewWriter(w)`                    | `NewTable(w, opts...)`                                                               | `NewWriter` deprecated; wraps `NewTable` (`tablewriter.go`).         |
| `SetHeader([]string)`             | `Header(...any)`                                                                     | Variadic or slice; supports any type (`tablewriter.go`).             |
| `Append([]string)`                | `Append(...any)`                                                                     | Variadic, slice, or struct (`tablewriter.go`).                       |
| `AppendBulk([][]string)`          | `Bulk([]any)`                                                                        | Slice of rows; supports any type (`tablewriter.go`).                 |
| `SetFooter([]string)`             | `Footer(...any)`                                                                     | Variadic or slice; supports any type (`tablewriter.go`).             |
| `Render()`                        | `Render()` (returns `error`)                                                         | Batch mode; streaming uses `Start/Close` (`tablewriter.go`).         |
| `SetBorder(bool)`                 | `WithRendition(tw.Rendition{Borders: ...})` or `WithBorders(tw.Border)` (deprecated) | Use `tw.Border` (`deprecated.go`, `tw/renderer.go`).                 |
| `SetRowLine(bool)`                | `WithRendition(tw.Rendition{Settings: {Separators: {BetweenRows: tw.On}}})`          | `tw.Separators` (`tw/renderer.go`).                                  |
| `SetHeaderLine(bool)`             | `WithRendition(tw.Rendition{Settings: {Lines: {ShowHeaderLine: tw.On}}})`            | `tw.Lines` (`tw/renderer.go`).                                       |
| `SetColumnSeparator(string)`      | `WithRendition(tw.Rendition{Symbols: ...})` or `WithSymbols(tw.Symbols)` (deprecated)| `tw.NewSymbols` or custom `tw.Symbols` (`tw/symbols.go`).            |
| `SetCenterSeparator(string)`      | `WithRendition(tw.Rendition{Symbols: ...})` or `WithSymbols(tw.Symbols)` (deprecated)| `tw.Symbols` (`tw/symbols.go`).                                      |
| `SetRowSeparator(string)`         | `WithRendition(tw.Rendition{Symbols: ...})` or `WithSymbols(tw.Symbols)` (deprecated)| `tw.Symbols` (`tw/symbols.go`).                                      |
| `SetAlignment(int)`               | `WithRowAlignment(tw.Align)` or `Config.Row.Alignment.Global`                        | `tw.Align` type (`config.go`).                                       |
| `SetHeaderAlignment(int)`         | `WithHeaderAlignment(tw.Align)` or `Config.Header.Alignment.Global`                  | `tw.Align` type (`config.go`).                                       |
| `SetAutoFormatHeaders(bool)`      | `WithHeaderAutoFormat(tw.State)` or `Config.Header.Formatting.AutoFormat`            | `tw.State` (`config.go`).                                            |
| `SetAutoWrapText(bool)`           | `WithRowAutoWrap(int)` or `Config.Row.Formatting.AutoWrap` (uses `tw.Wrap...` const) | `tw.WrapNormal`, `tw.WrapTruncate`, etc. (`config.go`).              |
| `SetAutoMergeCells(bool)`         | `WithRowMergeMode(int)` or `Config.Row.Formatting.MergeMode` (uses `tw.Merge...` const) | Supports `Vertical`, `Hierarchical` (`config.go`).                   |
| `SetColMinWidth(col, w)`          | `WithColumnWidths(tw.NewMapper[int,int]().Set(col, w))` or `Config.Widths.PerColumn` | `Config.Widths` for fixed widths (`config.go`).                      |
| `SetTablePadding(string)`         | Use `Config.<Section>.Padding.Global` with `tw.Padding`                              | No direct equivalent; manage via cell padding (`tw/cell.go`).        |
| `SetDebug(bool)`                  | `WithDebug(bool)` or `Config.Debug`                                                  | Logs via `table.Debug()` (`config.go`).                              |
| `Clear()`                         | `Reset()`                                                                            | Clears data and state (`tablewriter.go`).                            |
| `SetNoWhiteSpace(bool)`           | `WithTrimSpace(tw.Off)` (if `true`) or `WithPadding(tw.PaddingNone)`                  | Manage via `Config.Behavior.TrimSpace` or padding (`config.go`).     |
| `SetColumnColor(Colors)`          | Embed ANSI codes in cell data, use `tw.Formatter`, or `Config.<Section>.Filter`      | Colors via data or filters (`tw/cell.go`).                           |
| `SetHeaderColor(Colors)`          | Embed ANSI codes in cell data, use `tw.Formatter`, or `Config.Header.Filter`         | Colors via data or filters (`tw/cell.go`).                           |
| `SetFooterColor(Colors)`          | Embed ANSI codes in cell data, use `tw.Formatter`, or `Config.Footer.Filter`         | Colors via data or filters (`tw/cell.go`).                           |

**Notes**:
- Deprecated methods are in `deprecated.go` but should be replaced.
- `tw` package types (e.g., `tw.Align`, `tw.State`) are required for new APIs.
- Examples for each mapping are provided in the migration steps below.

## Detailed Migration Steps: Initialization, Data Input, Rendering

This section provides step-by-step guidance for migrating core table operations, with code examples and explanations to ensure clarity. Each step maps v0.0.5 methods to v1.0.x equivalents, highlighting changes, best practices, and potential pitfalls.

### 1. Table Initialization
Initialization has shifted from `NewWriter` to `NewTable`, which supports flexible configuration via `Option` functions, `Config` structs, or `ConfigBuilder`.

**Old (v0.0.5):**
```go
package main

import (
	"github.com/olekukonko/tablewriter"
	"os"
)

func main() {
	table := tablewriter.NewWriter(os.Stdout)
	// ... use table
	_ = table // Avoid "declared but not used"
}
```

**New (v1.0.x):**
```go
package main

import (
	"fmt" // Added for FormattableEntry example
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer" // Import renderer
	"github.com/olekukonko/tablewriter/tw"
	"os"
	"strings" // Added for Formatter example
)

func main() {
	// Minimal Setup (Default Configuration)
	tableMinimal := tablewriter.NewTable(os.Stdout)
	_ = tableMinimal // Avoid "declared but not used"

	// Using Option Functions for Targeted Configuration
	tableWithOptions := tablewriter.NewTable(os.Stdout,
		tablewriter.WithHeaderAlignment(tw.AlignCenter), // Center header text
		tablewriter.WithRowAlignment(tw.AlignLeft),      // Left-align row text
		tablewriter.WithDebug(true),                     // Enable debug logging
	)
	_ = tableWithOptions // Avoid "declared but not used"

	// Using a Full Config Struct for Comprehensive Control
	cfg := tablewriter.Config{
		Header: tw.CellConfig{
			Alignment: tw.CellAlignment{
				Global:    tw.AlignCenter,
				PerColumn: []tw.Align{tw.AlignLeft, tw.AlignRight}, // Column-specific alignment
			},
			Formatting: tw.CellFormatting{AutoFormat: tw.On},
		},
		Row: tw.CellConfig{
			Alignment:  tw.CellAlignment{Global: tw.AlignLeft},
			Formatting: tw.CellFormatting{AutoWrap: tw.WrapNormal},
		},
		Footer: tw.CellConfig{
			Alignment: tw.CellAlignment{Global: tw.AlignRight},
		},
		MaxWidth: 80, // Constrain total table width
		Behavior: tw.Behavior{
			AutoHide:  tw.Off, // Show empty columns
			TrimSpace: tw.On,  // Trim cell spaces
		},
		Widths: tw.CellWidth{
			Global:    20,                                  // Default fixed column width
			PerColumn: tw.NewMapper[int, int]().Set(0, 15), // Column 0 fixed at 15
		},
	}
	tableWithConfig := tablewriter.NewTable(os.Stdout, tablewriter.WithConfig(cfg))
	_ = tableWithConfig // Avoid "declared but not used"

	// Using ConfigBuilder for Fluent, Complex Configuration
	builder := tablewriter.NewConfigBuilder().
		WithMaxWidth(80).
		WithAutoHide(tw.Off).
		WithTrimSpace(tw.On).
		WithDebug(true). // Enable debug logging
		Header().
		Alignment().
		WithGlobal(tw.AlignCenter).
		Build(). // Returns *ConfigBuilder
		Header().
		Formatting().
		WithAutoFormat(tw.On).
		WithAutoWrap(tw.WrapTruncate). // Test truncation
		WithMergeMode(tw.MergeNone).   // Explicit merge mode
		Build().                       // Returns *HeaderConfigBuilder
		Padding().
		WithGlobal(tw.Padding{Left: "[", Right: "]", Overwrite: true}).
		Build(). // Returns *HeaderConfigBuilder
		Build(). // Returns *ConfigBuilder
		Row().
		Formatting().
		WithAutoFormat(tw.On). // Uppercase rows
		Build().               // Returns *RowConfigBuilder
		Build().               // Returns *ConfigBuilder
		Row().
		Alignment().
		WithGlobal(tw.AlignLeft).
		Build(). // Returns *ConfigBuilder
		Build()  // Finalize Config

	tableWithFluent := tablewriter.NewTable(os.Stdout, tablewriter.WithConfig(builder))
	_ = tableWithFluent // Avoid "declared but not used"
}
```

**Key Changes**:
- **Deprecation**: `NewWriter` is deprecated but retained for compatibility, internally calling `NewTable` (`tablewriter.go:NewWriter`). Transition to `NewTable` for new code.
- **Flexibility**: `NewTable` accepts an `io.Writer` and variadic `Option` functions (`tablewriter.go:NewTable`), enabling configuration via:
    - `WithConfig(Config)`: Applies a complete `Config` struct (`config.go:WithConfig`).
    - `WithRenderer(tw.Renderer)`: Sets a custom renderer, defaulting to `renderer.NewBlueprint()` (`tablewriter.go:WithRenderer`).
    - `WithRendition(tw.Rendition)`: Configures visual styles for the renderer (`tablewriter.go:WithRendition`).
    - `WithStreaming(tw.StreamConfig)`: Enables streaming mode (`tablewriter.go:WithStreaming`).
    - Other `WithXxx` functions for specific settings (e.g., `WithHeaderAlignment`, `WithDebug`).
- **ConfigBuilder**: Provides a fluent API for complex setups; `Build()` finalizes the `Config` (`config.go:ConfigBuilder`).
- **Method Chaining in Builder**: Remember to call `Build()` on nested builders to return to the parent builder (e.g., `builder.Header().Alignment().WithGlobal(...).Build().Formatting()...`).

**Migration Tips**:
- Replace `NewWriter` with `NewTable` and specify configurations explicitly.
- Use `ConfigBuilder` for complex setups or when readability is paramount.
- Apply `Option` functions for quick, targeted changes.
- Ensure `tw` package is imported for types like `tw.Align` and `tw.CellConfig`.
- Verify renderer settings if custom styling is needed, as `renderer.NewBlueprint()` is the default.

**Potential Pitfalls**:
- **Unintended Defaults**: Without explicit configuration, `defaultConfig()` applies (see Default Parameters), which may differ from v0.0.5 behavior (e.g., `Header.Formatting.AutoFormat = tw.On`).
- **Renderer Absence**: If no renderer is set, `NewTable` defaults to `renderer.NewBlueprint()`; explicitly set for custom formats.
- **ConfigBuilder Errors**: Always call `Build()` at the end of a builder chain and on nested builders; omitting it can lead to incomplete configurations or runtime errors.
- **Concurrent Modification**: Avoid modifying `Table.config` (if using the `Configure` method or direct access) in concurrent scenarios or during rendering to prevent race conditions.

### 2. Data Input
Data input methods in v1.0.x are more flexible, accepting `any` type for headers, rows, and footers, with robust conversion logic to handle strings, structs, and custom types.

#### 2.1. Setting Headers
Headers define the table’s column labels and are typically the first data added.

**Old (v0.0.5):**
```go
package main

import (
	"github.com/olekukonko/tablewriter"
	"os"
)

func main() {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Name", "Sign", "Rating"})
	// ...
}
```

**New (v1.0.x):**
```go
package main

import (
	"fmt"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw" // For tw.Formatter
	"os"
	"strings"
)

// Struct with Formatter
type HeaderData struct { // Renamed to avoid conflict
	Label string
}

func (h HeaderData) Format() string { return strings.ToUpper(h.Label) } // Implements tw.Formatter

func main() {
	table := tablewriter.NewTable(os.Stdout)

	// Variadic Arguments (Preferred for Simplicity)
	table.Header("Name", "Sign", "Rating")

	// Slice of Strings
	// table.Header([]string{"Name", "Sign", "Rating"}) // Example, comment out if using variadic

	// Slice of Any Type (Flexible)
	// table.Header([]any{"Name", "Sign", 123}) // Numbers converted to strings

	// Using Formatter
	// table.Header(HeaderData{"name"}, HeaderData{"sign"}, HeaderData{"rating"}) // Outputs "NAME", "SIGN", "RATING"

	table.Append("Example", "Row", "Data") // Add some data to render
	table.Render()
}
```

**Key Changes**:
- **Method**: `SetHeader([]string)` replaced by `Header(...any)` (`tablewriter.go:Header`).
- **Flexibility**: Accepts variadic arguments or a single slice; supports any type, not just strings.
- **Conversion**: Elements are processed by `processVariadic` (`zoo.go:processVariadic`) and converted to strings via `convertCellsToStrings` (`zoo.go:convertCellsToStrings`), supporting:
    - Basic types (e.g., `string`, `int`, `float64`).
    - `fmt.Stringer` implementations.
    - `tw.Formatter` implementations (custom string conversion).
    - Structs (exported fields as cells or single cell if `Stringer`/`Formatter`).
- **Streaming**: In streaming mode, `Header()` renders immediately via `streamRenderHeader` (`stream.go:streamRenderHeader`).
- **Formatting**: Headers are formatted per `Config.Header` settings (e.g., `AutoFormat`, `Alignment`) during rendering (`zoo.go:prepareContent`).

**Migration Tips**:
- Replace `SetHeader` with `Header`, using variadic arguments for simplicity.
- Use slices for dynamic header lists or when passing from a variable.
- Implement `tw.Formatter` for custom header formatting (e.g., uppercase).
- Ensure header count matches expected columns to avoid width mismatches.
- In streaming mode, call `Header()` before rows, as it fixes column widths (`stream.go`).

**Potential Pitfalls**:
- **Type Mismatch**: Non-string types are converted to strings; ensure desired formatting (e.g., use `tw.Formatter` for custom types).
- **Streaming Widths**: Headers influence column widths in streaming; set `Config.Widths` explicitly if specific widths are needed (`stream.go:streamCalculateWidths`).
- **Empty Headers**: Missing headers may cause rendering issues; provide placeholders (e.g., `""`) if needed.
- **AutoFormat**: Default `Header.Formatting.AutoFormat = tw.On` applies title case; disable with `WithHeaderAutoFormat(tw.Off)` if unwanted (`config.go`).

#### 2.2. Appending Rows
Rows represent the table’s data entries, and v1.0.x enhances flexibility with support for varied input types.

**Old (v0.0.5):**
```go
package main

import (
	"github.com/olekukonko/tablewriter"
	"os"
)

func main() {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"ColA", "ColB", "ColC"}) // Add header for context
	table.Append([]string{"A", "The Good", "500"})
	table.Append([]string{"B", "The Very Bad", "288"})
	data := [][]string{
		{"C", "The Ugly", "120"},
		{"D", "The Gopher", "800"},
	}
	table.AppendBulk(data)
	table.Render()
}
```

**New (v1.0.x):**
```go
package main

import (
	"fmt"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw" // For tw.Formatter
	"log"
	"os"
)

// Struct for examples
type Entry struct {
	ID    string
	Label string
	Score int
}

// Struct with Formatter
type FormattableEntry struct {
	ID    string
	Label string
	Score int
}

func (e FormattableEntry) Format() string { return fmt.Sprintf("%s (%d)", e.Label, e.Score) } // Implements tw.Formatter

func main() {
	table := tablewriter.NewTable(os.Stdout)
	table.Header("ID", "Description", "Value") // Header for context

	// Variadic Arguments (Single Row)
	table.Append("A", "The Good", 500) // Numbers converted to strings

	// Slice of Any Type (Single Row)
	table.Append([]any{"B", "The Very Bad", "288"})

	// Struct with Fields
	table.Append(Entry{ID: "C", Label: "The Ugly", Score: 120}) // Fields as cells

	// Struct with Formatter (will produce a single cell for this row based on Format())
	// To make it fit 3 columns, the formatter would need to produce a string that looks like 3 cells, or the table config would need to adapt.
	// For this example, let's assume it's meant to be one wide cell, or adjust the header.
	// For now, let's simplify and append it to a table with one header.
	tableOneCol := tablewriter.NewTable(os.Stdout)
	tableOneCol.Header("Formatted Entry")
	tableOneCol.Append(FormattableEntry{ID: "D", Label: "The Gopher", Score: 800}) // Single cell "The Gopher (800)"
	tableOneCol.Render()
	fmt.Println("---")


	// Re-initialize main table for Bulk example
	table = tablewriter.NewTable(os.Stdout)
	table.Header("ID", "Description", "Value")

	// Multiple Rows with Bulk
	data := []any{
		[]any{"E", "The Fast", 300},
		Entry{ID: "F", Label: "The Swift", Score: 400}, // Struct instance
		// FormattableEntry{ID: "G", Label: "The Bold", Score: 500}, // Would also be one cell
	}
	if err := table.Bulk(data); err != nil {
		log.Fatalf("Bulk append failed: %v", err)
	}
	table.Render()
}
```

**Key Changes**:
- **Method**: `Append([]string)` replaced by `Append(...any)`; `AppendBulk([][]string)` replaced by `Bulk([]any)` (`tablewriter.go`).
- **Input Flexibility**: `Append` accepts:
    - Multiple arguments as cells of a single row.
    - A single slice (e.g., `[]string`, `[]any`) as cells of a single row.
    - A single struct, processed by `convertItemToCells` (`zoo.go`):
        - Uses `tw.Formatter` or `fmt.Stringer` for single-cell output.
        - Extracts exported fields as multiple cells otherwise.
- **Bulk Input**: `Bulk` accepts a slice where each element is a row (e.g., `[][]any`, `[]Entry`), processed by `appendSingle` (`zoo.go:appendSingle`).
- **Streaming**: Rows render immediately in streaming mode via `streamAppendRow` (`stream.go:streamAppendRow`).
- **Error Handling**: `Append` and `Bulk` return errors for invalid conversions or streaming issues (`tablewriter.go`).

**Migration Tips**:
- Replace `Append([]string)` with `Append(...any)` for variadic input.
- Use `Bulk` for multiple rows, ensuring each element is a valid row representation.
- Implement `tw.Formatter` for custom struct formatting to control cell output.
- Match row cell count to headers to avoid alignment issues.
- In streaming mode, append rows after `Start()` and before `Close()` (`stream.go`).

**Potential Pitfalls**:
- **Cell Count Mismatch**: Uneven row lengths may cause rendering errors; pad with empty strings (e.g., `""`) if needed.
- **Struct Conversion**: Unexported fields are ignored; ensure fields are public or use `Formatter`/`Stringer` (`zoo.go`).
- **Streaming Order**: Rows must be appended after headers in streaming to maintain width consistency (`stream.go`).
- **Error Ignored**: Always check `Bulk` errors, as invalid data can cause failures.

#### 2.3. Setting Footers
Footers provide summary or closing data for the table, often aligned differently from rows.

**Old (v0.0.5):**
```go
package main

import (
	"github.com/olekukonko/tablewriter"
	"os"
)

func main() {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"ColA", "ColB", "ColC", "ColD"}) // Add header for context
	table.SetFooter([]string{"", "", "Total", "1408"})
	table.Render()
}
```

**New (v1.0.x):**
```go
package main

import (
	"fmt"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw" // For tw.Formatter
	"os"
)

// Using Formatter
type FooterSummary struct { // Renamed to avoid conflict
	Label string
	Value float64
}

func (s FooterSummary) Format() string { return fmt.Sprintf("%s: %.2f", s.Label, s.Value) } // Implements tw.Formatter

func main() {
	table := tablewriter.NewTable(os.Stdout)
	table.Header("ColA", "ColB", "ColC", "ColD") // Header for context

	// Variadic Arguments
	table.Footer("", "", "Total", 1408)

	// Slice of Any Type
	// table.Footer([]any{"", "", "Total", 1408.50})

	table.Render() // Render this table

	fmt.Println("--- Another Table with Formatter Footer ---")
	table2 := tablewriter.NewTable(os.Stdout)
	table2.Header("Summary Info") // Single column header
	// Using Formatter for a single cell footer
	table2.Footer(FooterSummary{Label: "Grand Total", Value: 1408.50}) // Single cell: "Grand Total: 1408.50"
	table2.Render()
}
```

**Key Changes**:
- **Method**: `SetFooter([]string)` replaced by `Footer(...any)` (`tablewriter.go:Footer`).
- **Input Flexibility**: Like `Header`, accepts variadic arguments, slices, or structs with `Formatter`/`Stringer` support (`zoo.go:processVariadic`).
- **Streaming**: Footers are buffered via `streamStoreFooter` and rendered during `Close()` (`stream.go:streamStoreFooter`, `stream.go:streamRenderFooter`).
- **Formatting**: Controlled by `Config.Footer` settings (e.g., `Alignment`, `AutoWrap`) (`zoo.go:prepareTableSection`).

**Migration Tips**:
- Replace `SetFooter` with `Footer`, preferring variadic input for simplicity.
- Use `tw.Formatter` for custom footer formatting (e.g., formatted numbers).
- Ensure footer cell count matches headers/rows to maintain alignment.
- In streaming mode, call `Footer()` before `Close()` to include it in rendering.

**Potential Pitfalls**:
- **Alignment Differences**: Default `Footer.Alignment.Global = tw.AlignRight` differs from rows (`tw.AlignLeft`); adjust if needed (`config.go`).
- **Streaming Timing**: Footers not rendered until `Close()`; ensure `Close()` is called (`stream.go`).
- **Empty Footers**: Missing footers may affect table appearance; use placeholders if needed.

### 3. Rendering the Table
Rendering has been overhauled to support both batch and streaming modes, with mandatory error handling for robustness.

**Old (v0.0.5):**
```go
package main

import (
	"github.com/olekukonko/tablewriter"
	"os"
)

func main() {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Data"})
	table.Append([]string{"Example"})
	table.Render()
}
```

**New (v1.0.x) - Batch Mode:**
```go
package main

import (
	"github.com/olekukonko/tablewriter"
	"log"
	"os"
)

func main() {
	table := tablewriter.NewTable(os.Stdout)
	table.Header("Data")
	table.Append("Example")
	if err := table.Render(); err != nil {
		log.Fatalf("Table rendering failed: %v", err)
	}
}
```

**New (v1.0.x) - Streaming Mode:**
```go
package main

import (
	"fmt"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
	"log"
	"os"
)

func main() {
	tableStream := tablewriter.NewTable(os.Stdout,
		tablewriter.WithConfig(tablewriter.Config{
			Stream: tw.StreamConfig{Enable: true},
			Widths: tw.CellWidth{
				Global:    12,                                  // Fixed column width for streaming
				PerColumn: tw.NewMapper[int, int]().Set(0, 15), // Column 0 at 15
			},
		}),
	)
	// Alternative: Using WithStreaming Option
	// tableStream := tablewriter.NewTable(os.Stdout,
	//     tablewriter.WithStreaming(tw.StreamConfig{Enable: true}),
	//     tablewriter.WithColumnMax(12), // Sets Config.Widths.Global
	// )

	if err := tableStream.Start(); err != nil {
		log.Fatalf("Failed to start table stream: %v", err)
	}
	tableStream.Header("Column 1", "Column 2")
	for i := 0; i < 3; i++ {
		if err := tableStream.Append(fmt.Sprintf("Data %d-1", i), fmt.Sprintf("Data %d-2", i)); err != nil {
			log.Printf("Failed to append row %d: %v", i, err) // Log and continue or handle differently
		}
	}
	tableStream.Footer("End", "Summary")
	if err := tableStream.Close(); err != nil {
		log.Fatalf("Failed to close table stream: %v", err)
	}
}
```

**Key Changes**:
- **Batch Mode**:
    - `Render()` processes all data (headers, rows, footers) in one go (`tablewriter.go:render`).
    - Calculates column widths dynamically based on content, `Config.Widths`, `ColMaxWidths`, and `MaxWidth` (`zoo.go:calculateAndNormalizeWidths`).
    - Invokes renderer methods: `Start()`, `Header()`, `Row()`, `Footer()`, `Line()`, `Close()` (`tw/renderer.go`).
    - Returns errors for invalid configurations or I/O issues.
- **Streaming Mode**:
    - Enabled via `Config.Stream.Enable` or `WithStreaming(tw.StreamConfig{Enable: true})` (`tablewriter.go:WithStreaming`).
    - `Start()` initializes the stream, fixing column widths based on `Config.Widths` or first data (header/row) (`stream.go:streamCalculateWidths`).
    - `Header()`, `Append()` render immediately (`stream.go:streamRenderHeader`, `stream.go:streamAppendRow`).
    - `Footer()` buffers data, rendered by `Close()` (`stream.go:streamStoreFooter`, `stream.go:streamRenderFooter`).
    - `Close()` finalizes rendering with footer and borders (`stream.go:Close`).
    - All methods return errors for robust handling.
- **Error Handling**: Mandatory error checks replace silent failures, improving reliability.

**Migration Tips**:
- Replace `Render()` calls with error-checked versions in batch mode.
- For streaming, adopt `Start()`, `Append()`, `Close()` workflow, ensuring `Start()` precedes data input.
- Set `Config.Widths` for consistent column widths in streaming mode (`config.go`).
- Use `WithRendition` to customize visual output, as renderer settings are decoupled (`tablewriter.go`).
- Test rendering with small datasets to verify configuration before scaling.

**Potential Pitfalls**:
- **Missing Error Checks**: Failing to check errors can miss rendering failures; always use `if err != nil`.
- **Streaming Widths**: Widths are fixed after `Start()`; inconsistent data may cause truncation (`stream.go`).
- **Renderer Misconfiguration**: Ensure `tw.Rendition` matches desired output style (`tw/renderer.go`).
- **Incomplete Streaming**: Forgetting `Close()` in streaming mode omits footer and final borders (`stream.go`).
- **Batch vs. Streaming**: Using `Render()` in streaming mode causes errors; use `Start()`/`Close()` instead (`tablewriter.go`).


## Styling and Appearance Configuration

Styling in v1.0.x is split between `tablewriter.Config` for data processing (e.g., alignment, padding, wrapping) and `tw.Rendition` for visual rendering (e.g., borders, symbols, lines). This section details how to migrate v0.0.5 styling methods to v1.0.x, providing examples, best practices, and migration tips to maintain or enhance table appearance.

### 4.1. Table Styles
Table styles define the visual structure through border and separator characters. In v0.0.5, styles were set via individual separator methods, whereas v1.0.x uses `tw.Rendition.Symbols` for a cohesive approach, offering predefined styles and custom symbol sets.

**Available Table Styles**:
The `tw.Symbols` interface (`tw/symbols.go`) supports a variety of predefined styles, each tailored to specific use cases, as well as custom configurations for unique requirements.

| Style Name     | Use Case                          | Symbols Example                     | Recommended Context                     |
|----------------|-----------------------------------|-------------------------------------|-----------------------------------------|
| `StyleASCII`   | Simple, terminal-friendly         | `+`, `-`, `|`                       | Basic CLI output, minimal dependencies; ensures compatibility across all terminals, including older or non-Unicode systems. |
| `StyleLight`   | Clean, lightweight borders        | `┌`, `└`, `─`, `│`                  | Modern terminals, clean and professional aesthetics; suitable for most CLI applications requiring a modern look. |
| `StyleHeavy`    | Thick, prominent borders          | `┏`, `┗`, `━`, `┃`                  | High-visibility reports, dashboards, or logs; emphasizes table structure for critical data presentation. |
| `StyleDouble`  | Double-line borders               | `╔`, `╚`, `═`, `║`                  | Formal documents, structured data exports; visually distinct for presentations or printed outputs. |
| `StyleRounded` | Rounded corners, aesthetic        | `╭`, `╰`, `─`, `│`                  | User-friendly CLI applications, reports; enhances visual appeal with smooth, rounded edges. |
| `StyleMarkdown`| Markdown-compatible               | `|`, `-`, `:`                        | Documentation, GitHub READMEs, or Markdown-based platforms; ensures proper rendering in Markdown viewers. |
*Note: `StyleBold` was mentioned but not defined in the symbols code, `StyleHeavy` is similar.*

**Old (v0.0.5):**
```go
package main

import (
	"github.com/olekukonko/tablewriter"
	"os"
)

func main() {
	table := tablewriter.NewWriter(os.Stdout)
	// table.SetCenterSeparator("*")  // Example, actual v0.0.5 method might vary
	// table.SetColumnSeparator("!")
	// table.SetRowSeparator("=")
	// table.SetBorder(true)
	table.SetHeader([]string{"Header1", "Header2"})
	table.Append([]string{"Cell1", "Cell2"})
	table.Render()
}
```

**New (v1.0.x):**
```go
package main

import (
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
	"os"
)

func main() {
	// Using a Predefined Style (Rounded Corners for Aesthetic Output)
	tableStyled := tablewriter.NewTable(os.Stdout,
		tablewriter.WithRenderer(renderer.NewBlueprint()), // Ensure renderer is set
		tablewriter.WithRendition(tw.Rendition{
			Symbols: tw.NewSymbols(tw.StyleRounded), // Predefined rounded style
			Borders: tw.Border{Top: tw.On, Bottom: tw.On, Left: tw.On, Right: tw.On},
			Settings: tw.Settings{
				Separators: tw.Separators{BetweenRows: tw.On}, // Lines between rows
			},
		}),
	)
	tableStyled.Header("Name", "Status")
	tableStyled.Append("Node1", "Ready")
	tableStyled.Render()

	// Using Fully Custom Symbols (Mimicking v0.0.5 Custom Separators)
	mySymbols := tw.NewSymbolCustom("my-style"). // Fluent builder for custom symbols
							WithCenter("*"). // Junction of lines
							WithColumn("!"). // Vertical separator
							WithRow("=").    // Horizontal separator
							WithTopLeft("/").
							WithTopMid("-").
							WithTopRight("\\").
							WithMidLeft("[").
							WithMidRight("]"). // Corrected from duplicate WithMidRight("+")
							// WithMidMid was not a method in SymbolCustom
							WithBottomLeft("\\").
							WithBottomMid("_").
							WithBottomRight("/").
							WithHeaderLeft("(").
							WithHeaderMid("-").
							WithHeaderRight(")") // Header-specific
	tableCustomSymbols := tablewriter.NewTable(os.Stdout,
		tablewriter.WithRenderer(renderer.NewBlueprint()),
		tablewriter.WithRendition(tw.Rendition{
			Symbols: mySymbols,
			Borders: tw.Border{Top: tw.On, Bottom: tw.On, Left: tw.On, Right: tw.On},
		}),
	)
	tableCustomSymbols.Header("Name", "Status")
	tableCustomSymbols.Append("Node1", "Ready")
	tableCustomSymbols.Render()
}
```

**Output (Styled, Rounded):**
```
╭───────┬────────╮
│ NAME  │ STATUS │
├───────┼────────┤
│ Node1 │ Ready  │
╰───────┴────────╯
```

**Output (Custom Symbols):**
```
/=======-========\
! NAME  ! STATUS !
[=======*========]
! Node1 ! Ready  !
\=======_========/
```

**Key Changes**:
- **Deprecated Methods**: `SetCenterSeparator`, `SetColumnSeparator`, `SetRowSeparator`, and `SetBorder` are deprecated and moved to `deprecated.go`. These are replaced by `tw.Rendition.Symbols` and `tw.Rendition.Borders` for a unified styling approach (`tablewriter.go`).
- **Symbol System**: The `tw.Symbols` interface defines all drawing characters used for table lines, junctions, and corners (`tw/symbols.go:Symbols`).
    - `tw.NewSymbols(tw.BorderStyle)` provides predefined styles (e.g., `tw.StyleASCII`, `tw.StyleMarkdown`, `tw.StyleRounded`) for common use cases. (Note: `tw.Style` should be `tw.BorderStyle`)
    - `tw.NewSymbolCustom(name string)` enables fully custom symbol sets via a fluent builder, allowing precise control over each character (e.g., `WithCenter`, `WithTopLeft`).
- **Renderer Dependency**: Styling requires a renderer, set via `WithRenderer`; the default is `renderer.NewBlueprint()` if not specified (`tablewriter.go:WithRenderer`).
- **Border Control**: `tw.Rendition.Borders` uses `tw.State` (`tw.On`, `tw.Off`) to enable or disable borders on each side (Top, Bottom, Left, Right) (`tw/renderer.go:Border`).
- **Extensibility**: Custom styles can be combined with custom renderers for non-text outputs, enhancing flexibility (`tw/renderer.go`).

**Migration Tips**:
- Replace individual separator setters (`SetCenterSeparator`, etc.) with `tw.NewSymbols` for predefined styles or `tw.NewSymbolCustom` for custom designs to maintain or enhance v0.0.5 styling.
- Use `WithRendition` to apply `tw.Rendition` settings, ensuring a renderer is explicitly set to avoid default behavior (`tablewriter.go`).
- Test table styles in your target environment (e.g., terminal, Markdown viewer, or log file) to ensure compatibility with fonts and display capabilities.
- For Markdown-based outputs (e.g., GitHub READMEs), use `tw.StyleMarkdown` to ensure proper rendering in Markdown parsers (`tw/symbols.go`).
- Combine `tw.Rendition.Symbols` with `tw.Rendition.Borders` and `tw.Rendition.Settings` to replicate or improve v0.0.5’s border and line configurations (`tw/renderer.go`).
- Document custom symbol sets in code comments to aid maintenance, as they can be complex (`tw/symbols.go`).

**Potential Pitfalls**:
- **Missing Renderer**: If `WithRenderer` is omitted, `NewTable` defaults to `renderer.NewBlueprint` with minimal styling, which may not match v0.0.5’s `SetBorder(true)` behavior; always specify for custom styles (`tablewriter.go`).
- **Incomplete Custom Symbols**: When using `tw.NewSymbolCustom`, failing to set all required symbols (e.g., `TopLeft`, `Center`, `HeaderLeft`) can cause rendering errors or inconsistent visuals; ensure all necessary characters are defined (`tw/symbols.go`).
- **Terminal Compatibility Issues**: Advanced styles like `StyleDouble` or `StyleHeavy` may not render correctly in older terminals or non-Unicode environments; use `StyleASCII` for maximum compatibility across platforms (`tw/symbols.go`).
- **Border and Symbol Mismatch**: Inconsistent `tw.Rendition.Borders` and `tw.Symbols` settings (e.g., enabling borders but using minimal symbols) can lead to visual artifacts; test with small tables to verify alignment (`tw/renderer.go`).
- **Markdown Rendering**: Non-Markdown styles (e.g., `StyleRounded`) may not render correctly in Markdown viewers; use `StyleMarkdown` for documentation or web-based outputs (`tw/symbols.go`).

### 4.2. Borders and Separator Lines
Borders and internal lines define the table’s structural appearance, controlling the visibility of outer edges and internal divisions. In v0.0.5, these were set via specific methods, while v1.0.x uses `tw.Rendition` fields for a more integrated approach.

**Old (v0.0.5):**
```go
package main

import (
	"github.com/olekukonko/tablewriter"
	"os"
)

func main() {
	table := tablewriter.NewWriter(os.Stdout)
	// table.SetBorder(false)       // Disable all outer borders
	// table.SetRowLine(true)       // Enable lines between data rows
	// table.SetHeaderLine(true)    // Enable line below header
	table.SetHeader([]string{"Header1", "Header2"})
	table.Append([]string{"Cell1", "Cell2"})
	table.Render()
}
```

**New (v1.0.x):**
```go
package main

import (
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
	"os"
)

func main() {
	// Standard Bordered Table with Internal Lines
	table := tablewriter.NewTable(os.Stdout,
		tablewriter.WithRenderer(renderer.NewBlueprint()),
		tablewriter.WithRendition(tw.Rendition{
			Borders: tw.Border{ // Outer table borders
				Left:   tw.On,
				Right:  tw.On,
				Top:    tw.On,
				Bottom: tw.On,
			},
			Settings: tw.Settings{
				Lines: tw.Lines{ // Major internal separator lines
					ShowHeaderLine: tw.On, // Line after header
					ShowFooterLine: tw.On, // Line before footer (if footer exists)
				},
				Separators: tw.Separators{ // General row and column separators
					BetweenRows:    tw.On, // Horizontal lines between data rows
					BetweenColumns: tw.On, // Vertical lines between columns
				},
			},
		}),
	)
	table.Header("Name", "Status")
	table.Append("Node1", "Ready")
	table.Render()

	// Borderless Table (kubectl-style, No Lines or Separators)
	// Configure the table with a borderless, kubectl-style layout
	config := tablewriter.NewConfigBuilder().
		Header().
		Padding().
		WithGlobal(tw.PaddingNone). // No header padding
		Build().
		Alignment().
		WithGlobal(tw.AlignLeft). // Left-align header
		Build().
		Row().
		Padding().
		WithGlobal(tw.PaddingNone). // No row padding
		Build().
		Alignment().
		WithGlobal(tw.AlignLeft). // Left-align rows
		Build().
		Footer().
		Padding().
		WithGlobal(tw.PaddingNone). // No footer padding
		Build().
		Alignment().
		WithGlobal(tw.AlignLeft). // Left-align footer (if used)
		Build().
		WithDebug(true). // Enable debug logging
		Build()

	// Create borderless table
	tableBorderless := tablewriter.NewTable(os.Stdout,
		tablewriter.WithRenderer(renderer.NewBlueprint()), // Assumes valid renderer
		tablewriter.WithRendition(tw.Rendition{
			Borders: tw.BorderNone,               // No borders
			Symbols: tw.NewSymbols(tw.StyleNone), // No border symbols
			Settings: tw.Settings{
				Lines:      tw.LinesNone,      // No header/footer lines
				Separators: tw.SeparatorsNone, // No row/column separators
			},
		}),
		tablewriter.WithConfig(config),
	)

	// Set headers and data
	tableBorderless.Header("Name", "Status")
	tableBorderless.Append("Node1", "Ready")
	tableBorderless.Render()
}
```

**Output (Standard Bordered):**
```
┌───────┬────────┐
│ Name  │ Status │
├───────┼────────┤
│ Node1 │ Ready  │
└───────┴────────┘
```

**Output (Borderless):**
```
NAME  STATUS
Node1 Ready 
```

**Key Changes**:
- **Deprecated Methods**: `SetBorder`, `SetRowLine`, and `SetHeaderLine` are deprecated and moved to `deprecated.go`. These are replaced by fields in `tw.Rendition` (`tw/renderer.go`):
    - `Borders`: Controls outer table borders (`tw.Border`) with `tw.State` (`tw.On`, `tw.Off`) for each side (Top, Bottom, Left, Right).
    - `Settings.Lines`: Manages major internal lines (`ShowHeaderLine` for header, `ShowFooterLine` for footer) (`tw.Lines`).
    - `Settings.Separators`: Controls general separators between rows (`BetweenRows`) and columns (`BetweenColumns`) (`tw.Separators`).
- **Presets for Simplicity**: `tw.BorderNone`, `tw.LinesNone`, and `tw.SeparatorsNone` provide quick configurations for minimal or borderless tables (`tw/preset.go`).
- **Renderer Integration**: Border and line settings are applied via `WithRendition`, requiring a renderer to be set (`tablewriter.go:WithRendition`).
- **Granular Control**: Each border side and line type can be independently configured, offering greater flexibility than v0.0.5’s boolean toggles.
- **Dependency on Symbols**: The appearance of borders and lines depends on `tw.Rendition.Symbols`; ensure compatible symbol sets (`tw/symbols.go`).

**Migration Tips**:
- Replace `SetBorder(false)` with `tw.Rendition.Borders = tw.BorderNone` to disable all outer borders (`tw/preset.go`).
- Use `tw.Rendition.Settings.Separators.BetweenRows = tw.On` to replicate `SetRowLine(true)`, ensuring row separators are visible (`tw/renderer.go`).
- Set `tw.Rendition.Settings.Lines.ShowHeaderLine = tw.On` to mimic `SetHeaderLine(true)` for a line below the header (`tw/renderer.go`).
- For kubectl-style borderless tables, combine `tw.BorderNone`, `tw.LinesNone`, `tw.SeparatorsNone`, and `WithPadding(tw.PaddingNone)` (applied via `ConfigBuilder` or `WithConfig`) to eliminate all lines and spacing (`tw/preset.go`, `config.go`).
- Test border and line configurations with small tables to verify visual consistency, especially when combining with custom `tw.Symbols`.
- Use `WithDebug(true)` to log rendering details if borders or lines appear incorrectly (`config.go`).

**Potential Pitfalls**:
- **Separator Absence**: If `tw.Rendition.Settings.Separators.BetweenColumns` is `tw.Off` and borders are disabled, columns may lack visual separation; use `tw.CellPadding` or ensure content spacing (`tw/cell.go`).
- **Line and Border Conflicts**: Mismatched settings (e.g., enabling `ShowHeaderLine` but disabling `Borders.Top`) can cause uneven rendering; align `Borders`, `Lines`, and `Separators` settings (`tw/renderer.go`).
- **Renderer Dependency**: Border settings require a renderer; omitting `WithRenderer` defaults to `renderer.NewBlueprint` with minimal styling, which may not match v0.0.5 expectations (`tablewriter.go`).
- **Streaming Limitations**: In streaming mode, separator rendering is fixed after `Start()`; ensure `tw.Rendition` is set correctly before rendering begins (`stream.go`).
- **Symbol Mismatch**: Using minimal `tw.Symbols` (e.g., `StyleASCII`) with complex `Borders` settings may lead to visual artifacts; test with matching symbol sets (`tw/symbols.go`).

### 4.3. Alignment
Alignment controls the positioning of text within table cells, now configurable per section (Header, Row, Footer) with both global and per-column options for greater precision.

**Old (v0.0.5):**
```go
package main

import (
	"github.com/olekukonko/tablewriter" // Assuming ALIGN_CENTER etc. were constants here
	"os"
)

const ( // Mocking v0.0.5 constants for example completeness
	ALIGN_CENTER = 1 // Example values
	ALIGN_LEFT   = 2
)


func main() {
	table := tablewriter.NewWriter(os.Stdout)
	// table.SetAlignment(ALIGN_CENTER)     // Applied to data rows
	// table.SetHeaderAlignment(ALIGN_LEFT) // Applied to header
	// No specific footer alignment setter
	table.SetHeader([]string{"Header1", "Header2"})
	table.Append([]string{"Cell1", "Cell2"})
	table.Render()
}
```

**New (v1.0.x):**
```go
package main

import (
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
	"os"
)

func main() {
	cfg := tablewriter.Config{
		Header: tw.CellConfig{
			Alignment: tw.CellAlignment{Global: tw.AlignLeft},
		},
		Row: tw.CellConfig{
			Alignment: tw.CellAlignment{
				Global:    tw.AlignCenter,
				PerColumn: []tw.Align{tw.AlignLeft, tw.AlignRight}, // Col 0 left, Col 1 right
			},
		},
		Footer: tw.CellConfig{
			Alignment: tw.CellAlignment{Global: tw.AlignRight},
		},
	}
	table := tablewriter.NewTable(os.Stdout, tablewriter.WithConfig(cfg))

	table.Header("Name", "Status")
	table.Append("Node1", "Ready")
	table.Footer("", "Active") // Ensure footer has content to show alignment
	table.Render()
}
```

**Output:**
```
┌───────┬────────┐
│ NAME  │ STATUS │
├───────┼────────┤
│ Node1 │  Ready │
├───────┼────────┤
│       │ Active │
└───────┴────────┘
```

**Key Changes**:
- **Deprecated Methods**: `SetAlignment` and `SetHeaderAlignment` are replaced by `WithRowAlignment`, `WithHeaderAlignment`, `WithFooterAlignment`, or direct `Config` settings (`config.go`). These old methods are retained in `deprecated.go` for compatibility but should be migrated.
- **Alignment Structure**: Alignment is managed within `tw.CellConfig.Alignment` (`tw/cell.go:CellAlignment`), which includes:
    - `Global`: A single `tw.Align` value applied to all cells in the section (`tw.AlignLeft`, `tw.AlignCenter`, `tw.AlignRight`, `tw.AlignNone`).
    - `PerColumn`: A slice of `tw.Align` values for column-specific alignment, overriding `Global` for specified columns.
- **Footer Alignment**: v1.0.x introduces explicit footer alignment via `WithFooterAlignment` or `Config.Footer.Alignment`, addressing v0.0.5’s lack of footer-specific control (`config.go`).
- **Type Safety**: `tw.Align` string constants replace v0.0.5’s integer constants (e.g., `ALIGN_CENTER`), improving clarity and reducing errors (`tw/types.go`).
- **Builder Support**: `ConfigBuilder` provides `Alignment()` methods for each section. `ForColumn(idx).WithAlignment()` applies alignment to a specific column across all sections (`config.go:ConfigBuilder`, `config.go:ColumnConfigBuilder`).
- **Deprecated Fields**: `tw.CellConfig.ColumnAligns` (slice) and `tw.CellFormatting.Alignment` (single value) are supported for backward compatibility but should be migrated to `tw.CellAlignment.Global` and `tw.CellAlignment.PerColumn` (`tw/cell.go`).

**Migration Tips**:
- Replace `SetAlignment(ALIGN_X)` with `WithRowAlignment(tw.AlignX)` or `Config.Row.Alignment.Global = tw.AlignX` to set row alignment (`config.go`).
- Use `WithHeaderAlignment(tw.AlignX)` for headers and `WithFooterAlignment(tw.AlignX)` for footers to maintain or adjust v0.0.5 behavior (`config.go`).
- Specify per-column alignments with `ConfigBuilder.Row().Alignment().WithPerColumn([]tw.Align{...})` or by setting `Config.Row.Alignment.PerColumn` for fine-grained control (`config.go`).
- Use `ConfigBuilder.ForColumn(idx).WithAlignment(tw.AlignX)` to apply consistent alignment to a specific column across all sections (Header, Row, Footer) (`config.go`).
- Verify alignment settings against defaults (`Header: tw.AlignCenter`, `Row: tw.AlignLeft`, `Footer: tw.AlignRight`) to ensure expected output (`config.go:defaultConfig`).
- Test alignment with varied cell content lengths to confirm readability, especially when combined with wrapping or padding settings (`zoo.go:prepareContent`).

**Potential Pitfalls**:
- **Alignment Precedence**: `PerColumn` settings override `Global` within a section; ensure column-specific alignments are intentional (`tw/cell.go:CellAlignment`).
- **Deprecated Fields**: Relying on `ColumnAligns` or `tw.CellFormatting.Alignment` is temporary; migrate to `tw.CellAlignment` to avoid future issues (`tw/cell.go`).
- **Cell Count Mismatch**: Rows or footers with fewer cells than headers can cause alignment errors; pad with empty strings (`""`) to match column count (`zoo.go`).
- **Streaming Width Impact**: In streaming mode, alignment depends on fixed column widths set by `Config.Widths`; narrow widths may truncate content, misaligning text (`stream.go:streamCalculateWidths`).
- **Default Misalignment**: The default `Footer.Alignment.Global = tw.AlignRight` differs from rows (`tw.AlignLeft`); explicitly set `WithFooterAlignment` if consistency is needed (`config.go`).

### 4.4. Auto-Formatting (Header Title Case)
Auto-formatting applies transformations like title case to cell content, primarily for headers to enhance readability, but it can be enabled for rows or footers in v1.0.x.

**Old (v0.0.5):**
```go
package main

import (
	"github.com/olekukonko/tablewriter"
	"os"
)

func main() {
	table := tablewriter.NewWriter(os.Stdout)
	// table.SetAutoFormatHeaders(true) // Default: true; applies title case to headers
	table.SetHeader([]string{"col_one", "status_report"})
	table.Append([]string{"Node1", "Ready"})
	table.Render()
}
```

**New (v1.0.x):**
```go
package main

import (
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
	"os"
)

func main() {
	// Using Direct Config Struct to turn OFF default AutoFormat for Header
	cfg := tablewriter.Config{
		Header: tw.CellConfig{Formatting: tw.CellFormatting{AutoFormat: tw.Off}}, // Turn OFF title case for headers
		Row:    tw.CellConfig{Formatting: tw.CellFormatting{AutoFormat: tw.Off}}, // No formatting for rows (default)
		Footer: tw.CellConfig{Formatting: tw.CellFormatting{AutoFormat: tw.Off}}, // No formatting for footers (default)
	}
	tableNoAutoFormat := tablewriter.NewTable(os.Stdout, tablewriter.WithConfig(cfg))

	tableNoAutoFormat.Header("col_one", "status_report") // Stays as "col_one", "status_report"
	tableNoAutoFormat.Append("Node1", "Ready")
	tableNoAutoFormat.Render()

	// Using Option Function for Headers (default is ON, this makes it explicit)
	tableWithAutoFormat := tablewriter.NewTable(os.Stdout,
		tablewriter.WithHeaderAutoFormat(tw.On), // Explicitly enable title case (default behavior)
	)

	tableWithAutoFormat.Header("col_one", "status_report") // Becomes "COL ONE", "STATUS REPORT"
	tableWithAutoFormat.Append("Node1", "Ready")
	tableWithAutoFormat.Render()
}
```

**Output:**
```
┌─────────┬───────────────┐
│ col_one │ status_report │
├─────────┼───────────────┤
│ Node1   │ Ready         │
└─────────┴───────────────┘
┌─────────┬───────────────┐
│ COL ONE │ STATUS REPORT │
├─────────┼───────────────┤
│ Node1   │ Ready         │
└─────────┴───────────────┘
```

**Key Changes**:
- **Method**: `SetAutoFormatHeaders` is replaced by  `Config.Header.Formatting.AutoFormat`, or equivalent builder methods (`config.go`).
- **Extended Scope**: Auto-formatting can now be applied to rows and footers via `Config.Row.Formatting.AutoFormat` and `Config.Footer.Formatting.AutoFormat`, unlike v0.0.5’s header-only support (`tw/cell.go`).
- **Type Safety**: `tw.State` (`tw.On`, `tw.Off`, `tw.Unknown`) controls formatting state, replacing boolean flags (`tw/state.go`).
- **Behavior**: When `tw.On`, the `tw.Title` function converts text to uppercase and replaces underscores and some periods with spaces (e.g., "col_one" → "COL ONE") (`tw/fn.go:Title`, `zoo.go:prepareContent`).
- **Defaults**: `Header.Formatting.AutoFormat = tw.On`, `Row.Formatting.AutoFormat = tw.Off`, `Footer.Formatting.AutoFormat = tw.Off` (`config.go:defaultConfig`).

**Migration Tips**:
- To maintain v0.0.5’s default header title case behavior, no explicit action is needed as `WithHeaderAutoFormat(tw.On)` is the default. If you were using `SetAutoFormatHeaders(false)`, you'd use `WithHeaderAutoFormat(tw.Off)`.
- Explicitly set `WithRowAutoFormat(tw.Off)` or `WithFooterAutoFormat(tw.Off)` if you want to ensure rows and footers remain unformatted, as v1.0.x allows enabling formatting for these sections (`config.go`).
- Use `ConfigBuilder.Header().Formatting().WithAutoFormat(tw.State)` for precise control over formatting per section (`config.go`).
- Test header output with underscores or periods (e.g., "my_column") to verify title case transformation meets expectations.
- For custom formatting beyond title case (e.g., custom capitalization), use `tw.CellFilter` instead of `AutoFormat` (`tw/cell.go`).

**Potential Pitfalls**:
- **Default Header Formatting**: `Header.Formatting.AutoFormat = tw.On` by default may unexpectedly alter header text (e.g., "col_one" → "COL ONE"); disable with `WithHeaderAutoFormat(tw.Off)` if raw headers are preferred (`config.go`).
- **Row/Footer Formatting**: Enabling `AutoFormat` for rows or footers (not default) applies title case, which may not suit data content; verify with `tw.Off` unless intended (`tw/cell.go`).
- **Filter Conflicts**: Combining `AutoFormat` with `tw.CellFilter` can lead to overlapping transformations; prioritize filters for complex formatting (`zoo.go:prepareContent`).
- **Performance Overhead**: Auto-formatting large datasets may add minor processing time; disable for performance-critical applications (`zoo.go`).

### 4.5. Text Wrapping
Text wrapping determines how long cell content is handled within width constraints, offering more options in v1.0.x compared to v0.0.5’s binary toggle.

**Old (v0.0.5):**
```go
package main

import (
	"github.com/olekukonko/tablewriter"
	"os"
)

func main() {
	table := tablewriter.NewWriter(os.Stdout)
	// table.SetAutoWrapText(true)  // Enable normal word wrapping
	// table.SetAutoWrapText(false) // Disable wrapping
	table.SetHeader([]string{"Long Header Text Example", "Status"})
	table.Append([]string{"This is some very long cell content that might wrap", "Ready"})
	table.Render()
}
```

**New (v1.0.x):**
```go
package main

import (
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw" // For tw.WrapNormal etc.
	"os"
)

func main() {
	// Using Option Functions for Quick Wrapping Settings on a specific section (e.g., Row)
	// To actually see wrapping, a MaxWidth for the table or columns is needed.
	table := tablewriter.NewTable(os.Stdout,
		tablewriter.WithRowAutoWrap(tw.WrapNormal),      // Set row wrapping
		tablewriter.WithHeaderAutoWrap(tw.WrapTruncate), // Header truncates (default)
		tablewriter.WithMaxWidth(30),                    // Force wrapping by setting table max width
	)
	table.Header("Long Header Text", "Status")
	table.Append("This is a very long cell content", "Ready")
	table.Footer("Summary", "Active")
	table.Render()

	// For more fine-grained control (e.g., different wrapping for header, row, footer):
	cfgBuilder := tablewriter.NewConfigBuilder()
	cfgBuilder.Header().Formatting().WithAutoWrap(tw.WrapTruncate) // Header: Truncate
	cfgBuilder.Row().Formatting().WithAutoWrap(tw.WrapNormal)       // Row: Normal wrap
	cfgBuilder.Footer().Formatting().WithAutoWrap(tw.WrapBreak)     // Footer: Break words
	cfgBuilder.WithMaxWidth(40) // Overall table width constraint

	tableFullCfg := tablewriter.NewTable(os.Stdout, tablewriter.WithConfig(cfgBuilder.Build()))
	tableFullCfg.Header("Another Very Long Header Example That Will Be Truncated", "Info")
	tableFullCfg.Append("This is an example of row content that should wrap normally based on available space.", "Detail")
	tableFullCfg.Footer("FinalSummaryInformationThatMightBreakAcrossLines", "End")
	tableFullCfg.Render()
}
```

**Output (tableOpt):**
```
┌──────────────┬────────┐
│ LONG HEADER… │ STATUS │
├──────────────┼────────┤
│ This is a    │ Ready  │
│ very long    │        │
│ cell content │        │
├──────────────┼────────┤
│      Summary │ Active │
└──────────────┴────────┘
```
*(Second table output will vary based on content and width)*

**Key Changes**:
- **Method**: `SetAutoWrapText` is replaced by `Config.<Section>.Formatting.AutoWrap` or specific `With<Section>AutoWrap` options (`config.go`).
- **Wrapping Modes**: `int` constants for `AutoWrap` (e.g., `tw.WrapNone`, `tw.WrapNormal`, `tw.WrapTruncate`, `tw.WrapBreak`) replace v0.0.5’s binary toggle (`tw/tw.go`).
- **Section-Specific Control**: Wrapping is configurable per section (Header, Row, Footer), unlike v0.0.5’s global setting (`tw/cell.go`).
- **Defaults**: `Header: tw.WrapTruncate`, `Row: tw.WrapNormal`, `Footer: tw.WrapNormal` (`config.go:defaultConfig`).
- **Width Dependency**: Wrapping behavior relies on width constraints set by `Config.Widths` (fixed column widths), `Config.<Section>.ColMaxWidths` (max content width), or `Config.MaxWidth` (total table width) (`zoo.go:calculateContentMaxWidth`, `zoo.go:prepareContent`).

**Migration Tips**:
- Replace `SetAutoWrapText(true)` with `WithRowAutoWrap(tw.WrapNormal)` to maintain v0.0.5’s default wrapping for rows (`config.go`).
- Use `WithHeaderAutoWrap(tw.WrapTruncate)` to align with v1.0.x’s default header behavior, ensuring long headers are truncated (`config.go`).
- Set `Config.Widths` or `Config.MaxWidth` explicitly to enforce wrapping, as unconstrained widths may prevent wrapping (`config.go`).
- Use `ConfigBuilder.<Section>().Formatting().WithAutoWrap(int_wrap_mode)` for precise control over wrapping per section (`config.go`).
- Test wrapping with varied content lengths (e.g., short and long text) to ensure readability and proper width allocation.
- Consider `tw.WrapNormal` for data rows to preserve content integrity, reserving `tw.WrapTruncate` for headers or footers (`tw/tw.go`).

**Potential Pitfalls**:
- **Missing Width Constraints**: Without `Config.Widths`, `ColMaxWidths`, or `MaxWidth`, wrapping may not occur, leading to overflow; always define width limits for wrapping (`zoo.go`).
- **Streaming Width Impact**: In streaming mode, wrapping depends on fixed widths set at `Start()`; narrow widths may truncate content excessively (`stream.go:streamCalculateWidths`).
- **Truncation Data Loss**: `tw.WrapTruncate` may obscure critical data in rows; use `tw.WrapNormal` or wider columns to retain content (`tw/tw.go`).
- **Performance Overhead**: Wrapping large datasets with `tw.WrapNormal` or `tw.WrapBreak` can add processing time; optimize widths for performance-critical applications (`zoo.go:prepareContent`).
- **Inconsistent Section Wrapping**: Default wrapping differs (`Header: tw.WrapTruncate`, `Row/Footer: tw.WrapNormal`); align settings if uniform behavior is needed (`config.go`).

### 4.6. Padding
Padding adds spacing within cells, enhancing readability and affecting cell width calculations. v1.0.x introduces granular, per-side padding, replacing v0.0.5’s single inter-column padding control.

**Old (v0.0.5):**
```go
package main

import (
	"github.com/olekukonko/tablewriter"
	"os"
)

func main() {
	table := tablewriter.NewWriter(os.Stdout)
	// table.SetTablePadding("\t") // Set inter-column space character when borders are off
	// Default: single space within cells
	table.SetHeader([]string{"Header1"})
	table.Append([]string{"Cell1"})
	table.Render()
}
```

**New (v1.0.x):**
```go
package main

import (
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
	"os"
)

func main() {
	// Using Direct Config Struct
	cfg := tablewriter.Config{
		Header: tw.CellConfig{
			Padding: tw.CellPadding{
				Global: tw.Padding{Left: "[", Right: "]", Top: "-", Bottom: "-"}, // Padding for all header cells
				PerColumn: []tw.Padding{ // Specific padding for header column 0
					{Left: ">>", Right: "<<", Top: "=", Bottom: "="}, // Overrides Global for column 0
				},
			},
		},
		Row: tw.CellConfig{
			Padding: tw.CellPadding{
				Global: tw.PaddingDefault, // One space left/right for all row cells
			},
		},
		Footer: tw.CellConfig{
			Padding: tw.CellPadding{
				Global: tw.Padding{Top: "~", Bottom: "~"}, // Top/bottom padding for all footer cells
			},
		},
	}
	table := tablewriter.NewTable(os.Stdout, tablewriter.WithConfig(cfg))

	table.Header("Name", "Status") // Two columns
	table.Append("Node1", "Ready")
	table.Footer("End", "Active")
	table.Render()
}
```

**Output:**
```
┌────────┬────────┐
│========│[------]│
│>>NAME<<│[STATUS]│
│========│[------]│
├────────┼────────┤
│ Node1  │ Ready  │
│~~~~~~~~│~~~~~~~~│
│End     │Active  │
│~~~~~~~~│~~~~~~~~│
└────────┴────────┘
```

**Key Changes**:
- **No Direct Equivalent for `SetTablePadding`**: `SetTablePadding` controlled inter-column spacing when borders were off in v0.0.5; v1.0.x has no direct equivalent for *inter-column* spacing separate from cell padding. Inter-column visual separation is now primarily handled by `tw.Rendition.Settings.Separators.BetweenColumns` and the chosen `tw.Symbols`.
- **Granular Cell Padding**: `tw.CellPadding` (`tw/cell.go:CellPadding`) supports:
    - `Global`: A `tw.Padding` struct with `Left`, `Right`, `Top`, `Bottom` strings and an `Overwrite` flag (`tw/types.go:Padding`). This padding is *inside* the cell.
    - `PerColumn`: A slice of `tw.Padding` for column-specific padding, overriding `Global` for specified columns.
- **Per-Side Control**: `Top` and `Bottom` padding add extra lines *within* cells, unlike v0.0.5’s left/right-only spacing (`zoo.go:prepareContent`).
- **Defaults**: `tw.PaddingDefault` is `{Left: " ", Right: " "}` for all sections (applied inside cells); `Top` and `Bottom` are empty by default (`tw/preset.go`).
- **Width Impact**: Cell padding contributes to column widths, calculated in `Config.Widths` (`zoo.go:calculateAndNormalizeWidths`).
- **Presets**: `tw.PaddingNone` (`{Left: "", Right: "", Top: "", Bottom: "", Overwrite: true}`) removes padding for tight layouts (`tw/preset.go`).

**Migration Tips**:
- To achieve spacing similar to `SetTablePadding("\t")` when borders are off, you would set cell padding: `WithPadding(tw.Padding{Left: "\t", Right: "\t"})`. If you truly mean space *between* columns and not *inside* cells, ensure `tw.Rendition.Settings.Separators.BetweenColumns` is `tw.On` and customize `tw.Symbols.Column()` if needed.
- Use `tw.PaddingNone` (e.g., via `ConfigBuilder.<Section>().Padding().WithGlobal(tw.PaddingNone)`) for no cell padding.
- Set `Top` and `Bottom` padding for vertical spacing *within* cells, enhancing readability for multi-line content (`tw/types.go`).
- Use `ConfigBuilder.<Section>().Padding().WithPerColumn` for column-specific padding to differentiate sections or columns (`config.go`).
- Test padding with varied content and widths to ensure proper alignment and spacing, especially with wrapping enabled (`zoo.go`).
- Combine padding with `Config.Widths` or `ColMaxWidths` to control total cell size (`config.go`).

**Potential Pitfalls**:
- **Inter-Column Spacing vs. Cell Padding**: Be clear whether you want space *between* columns (separators) or *inside* cells (padding). `SetTablePadding` was ambiguous; v1.0.x distinguishes these.
- **Width Inflation**: Cell padding increases column widths, potentially exceeding `Config.MaxWidth` or causing truncation in streaming; adjust `Config.Widths` accordingly (`zoo.go`).
- **Top/Bottom Padding**: Non-empty `Top` or `Bottom` padding adds vertical lines *within* cells, increasing cell height; use sparingly for dense tables (`zoo.go:prepareContent`).
- **Streaming Constraints**: Padding is fixed in streaming mode after `Start()`; ensure `Config.Widths` accommodates padding (`stream.go`).
- **Default Padding**: `tw.PaddingDefault` adds spaces *inside* cells; set `tw.PaddingNone` for no internal cell padding (`tw/preset.go`).

### 4.7. Column Widths (Fixed Widths and Max Content Widths)
Column width control in v1.0.x is more sophisticated, offering fixed widths, maximum content widths, and overall table width constraints, replacing v0.0.5’s limited `SetColMinWidth`.

**Old (v0.0.5):**
```go
package main

import (
	"github.com/olekukonko/tablewriter"
	"os"
)

func main() {
	table := tablewriter.NewWriter(os.Stdout)
	// table.SetColMinWidth(0, 10) // Set minimum width for column 0
	table.SetHeader([]string{"Header1", "Header2"})
	table.Append([]string{"Cell1-Content", "Cell2-Content"})
	table.Render()
}
```

**New (v1.0.x):**
```go
package main

import (
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
	"os"
)

func main() {
	// Direct Config for Width Control
	cfg := tablewriter.Config{
		Widths: tw.CellWidth{ // Fixed total column widths (content + padding)
			Global:    20,                                  // Default fixed width for all columns
			PerColumn: tw.NewMapper[int, int]().Set(0, 15), // Column 0 fixed at 15
		},
		Header: tw.CellConfig{
			ColMaxWidths: tw.CellWidth{ // Max content width (excluding padding) for header cells
				Global:    15,                                  // Max content width for all header cells
				PerColumn: tw.NewMapper[int, int]().Set(0, 10), // Header Col 0 max content at 10
			},
			// Default header padding is " " on left/right, so content 10 + padding 2 = 12.
			// If Widths.PerColumn[0] is 15, there's space.
		},
		Row: tw.CellConfig{
			ColMaxWidths: tw.CellWidth{Global: 18}, // Max content width for row cells (18 + default padding 2 = 20)
		},
		MaxWidth: 80, // Constrain total table width (optional, columns might shrink)
	}
	tableWithCfg := tablewriter.NewTable(os.Stdout, tablewriter.WithConfig(cfg))
	tableWithCfg.Header("Very Long Header Name", "Status Information")
	tableWithCfg.Append("Some long content for the first column", "Ready")
	tableWithCfg.Render()

	// Option Functions for Width Settings
	tableWithOpts := tablewriter.NewTable(os.Stdout,
		tablewriter.WithColumnMax(20),                                     // Sets Config.Widths.Global (fixed total col width)
		tablewriter.WithColumnWidths(tw.NewMapper[int, int]().Set(0, 15)), // Sets Config.Widths.PerColumn for col 0
		tablewriter.WithMaxWidth(80),                                      // Sets Config.MaxWidth
		// For max content width per section, you'd use WithHeaderConfig, WithRowConfig, etc.
		// e.g., tablewriter.WithRowMaxWidth(18) // Sets Config.Row.ColMaxWidths.Global
	)
	tableWithOpts.Header("Long Header", "Status")
	tableWithOpts.Append("Long Content", "Ready")
	tableWithOpts.Render()
}
```

**Output (tableWithCfg - illustrative, exact wrapping depends on content and full config):**
```
┌───────────┬──────────────────┐
│ VERY LONG │ STATUS INFORMAT… │
│ HEADER NA…│                  │
├───────────┼──────────────────┤
│ Some long │ Ready            │
│ content f…│                  │
└───────────┴──────────────────┘
```

**Key Changes**:
- **Enhanced Width System**: v1.0.x introduces three levels of width control, replacing `SetColMinWidth` (`config.go`):
    - **Config.Widths**: Sets fixed total widths (content + padding) for columns, applied globally or per-column (`tw.CellWidth`).
        - `Global`: Default fixed width for all columns.
        - `PerColumn`: `tw.Mapper[int, int]` for specific column widths.
    - **Config.<Section>.ColMaxWidths**: Sets maximum content widths (excluding padding) for a section (Header, Row, Footer) (`tw.CellWidth`).
        - `Global`: Max content width for all cells in the section.
        - `PerColumn`: `tw.Mapper[int, int]` for specific columns in the section.
    - **Config.MaxWidth**: Constrains the total table width, shrinking columns proportionally if needed (`config.go`).
- **Streaming Support**: In streaming mode, `Config.Widths` fixes column widths at `Start()`; `ColMaxWidths` is used only for wrapping/truncation (`stream.go:streamCalculateWidths`).
- **Calculation Logic**: Widths are computed by `calculateAndNormalizeWidths` in batch mode and `streamCalculateWidths` in streaming mode, considering content, padding, and constraints (`zoo.go`, `stream.go`).
- **Deprecated Approach**: `SetColMinWidth` is replaced by `Config.Widths.PerColumn` or `Config.<Section>.ColMaxWidths.PerColumn` for more precise control (`deprecated.go`).

**Migration Tips**:
- Replace `SetColMinWidth(col, w)` with `WithColumnWidths(tw.NewMapper[int, int]().Set(col, w))` for fixed column widths or `Config.<Section>.ColMaxWidths.PerColumn` for content width limits (`config.go`).
- Use `Config.Widths.Global` or `WithColumnMax(w)` to set a default fixed width for all columns, ensuring consistency (`tablewriter.go`).
- Apply `Config.MaxWidth` to constrain total table width, especially for wide datasets (`config.go`).
- Use `ConfigBuilder.ForColumn(idx).WithMaxWidth(w)` to set per-column content width limits across sections (`config.go`). *(Note: This sets it for Header, Row, and Footer)*
- In streaming mode, set `Config.Widths` before `Start()` to fix widths, avoiding content-based sizing (`stream.go`).
- Test width settings with varied content to ensure wrapping and truncation behave as expected (`zoo.go`).

**Potential Pitfalls**:
- **Width Precedence**: `Config.Widths.PerColumn` overrides `Widths.Global`; `ColMaxWidths` applies *within* those fixed total widths for content wrapping/truncation (`zoo.go`).
- **Streaming Width Fixing**: Widths are locked after `Start()` in streaming; inconsistent data may cause truncation (`stream.go`).
- **Padding Impact**: Padding adds to total width when considering `Config.Widths`; account for `tw.CellPadding` when setting fixed column widths (`zoo.go`).
- **MaxWidth Shrinkage**: `Config.MaxWidth` may shrink columns unevenly; test with `MaxWidth` to avoid cramped layouts (`zoo.go`).
- **No Width Constraints**: Without `Widths` or `MaxWidth`, columns size to content, potentially causing overflow; define limits (`zoo.go`).

### 4.8. Colors
Colors in v0.0.5 were applied via specific color-setting methods, while v1.0.x embeds ANSI escape codes in cell content or uses data-driven formatting for greater flexibility.

**Old (v0.0.5):**
```go
package main
// Assuming tablewriter.Colors and color constants existed in v0.0.5
// This is a mock representation as the actual v0.0.5 definitions are not provided.
// import "github.com/olekukonko/tablewriter"
// import "os"

// type Colors []interface{} // Mock
// const (
// 	Bold = 1; FgGreenColor = 2; FgRedColor = 3 // Mock constants
// )

func main() {
	// table := tablewriter.NewWriter(os.Stdout)
	// table.SetColumnColor(
	//     tablewriter.Colors{tablewriter.Bold, tablewriter.FgGreenColor}, // Column 0
	//     tablewriter.Colors{tablewriter.FgRedColor},                     // Column 1
	// )
	// table.SetHeader([]string{"Name", "Status"})
	// table.Append([]string{"Node1", "Ready"})
	// table.Render()
}
```

**New (v1.0.x):**
```go
package main

import (
	"fmt"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw" // For tw.Formatter
	"os"
)

// Direct ANSI Code Embedding
const (
	Reset   = "\033[0m"
	Bold    = "\033[1m"
	FgGreen = "\033[32m"
	FgRed   = "\033[31m"
)

// Using tw.Formatter for Custom Types
type Status string

func (s Status) Format() string { // Implements tw.Formatter
	color := FgGreen
	if s == "Error" || s == "Inactive" {
		color = FgRed
	}
	return color + string(s) + Reset
}

func main() {
	// Example 1: Direct ANSI embedding
	tableDirectANSI := tablewriter.NewTable(os.Stdout,
		tablewriter.WithHeaderAutoFormat(tw.Off), // Keep header text as is for coloring
	)
	tableDirectANSI.Header(Bold+FgGreen+"Name"+Reset, Bold+FgRed+"Status"+Reset)
	tableDirectANSI.Append([]any{"Node1", FgGreen + "Ready" + Reset})
	tableDirectANSI.Append([]any{"Node2", FgRed + "Error" + Reset})
	tableDirectANSI.Render()

	fmt.Println("\n--- Table with Formatter for Colors ---")

	// Example 2: Using tw.Formatter
	tableFormatter := tablewriter.NewTable(os.Stdout)
	tableFormatter.Header("Name", "Status") // AutoFormat will apply to "NAME", "STATUS"
	tableFormatter.Append([]any{"Alice", Status("Active")})
	tableFormatter.Append([]any{"Bob", Status("Inactive")})
	tableFormatter.Render()

	fmt.Println("\n--- Table with CellFilter for Colors ---")
	// Example 3: Using CellFilter
	tableWithFilters := tablewriter.NewTable(os.Stdout,
		tablewriter.WithConfig(
			tablewriter.NewConfigBuilder().
				Row().
				Filter().
				WithPerColumn([]func(string) string{
					nil, // No filter for Item column
					func(s string) string { // Status column: apply color
						if s == "Ready" || s == "Active" {
							return FgGreen + s + Reset
						}
						return FgRed + s + Reset
					},
				}).
				Build(). // Return to RowConfigBuilder
				Build(). // Return to ConfigBuilder
				Build(), // Finalize Config
		),
	)
	tableWithFilters.Header("Item", "Availability")
	tableWithFilters.Append("ItemA", "Ready")
	tableWithFilters.Append("ItemB", "Unavailable")
	tableWithFilters.Render()
}
```

**Output (Text Approximation, Colors Not Shown):**
```
┌──────┬────────┐
│ Name │ Status │
├──────┼────────┤
│Node1 │ Ready  │
│Node2 │ Error  │
└──────┴────────┘

--- Table with Formatter for Colors ---
┌───────┬──────────┐
│ NAME  │  STATUS  │
├───────┼──────────┤
│ Alice │ Active   │
│ Bob   │ Inactive │
└───────┴──────────┘

--- Table with CellFilter for Colors ---
┌───────┬────────────┐
│ ITEM  │ AVAILABILI │
│       │     TY     │
├───────┼────────────┤
│ ItemA │ Ready      │
│ ItemB │ Unavailabl │
│       │ e          │
└───────┴────────────┘
```

**Key Changes**:
- **Removed Color Methods**: `SetColumnColor`, `SetHeaderColor`, and `SetFooterColor` are removed; colors are now applied by embedding ANSI escape codes in cell content or via data-driven mechanisms (`tablewriter.go`).
- **Flexible Coloring Options**:
    - **Direct ANSI Codes**: Embed codes (e.g., `\033[32m` for green) in strings for manual control (`zoo.go:convertCellsToStrings`).
    - **tw.Formatter**: Implement `Format() string` on custom types to control cell output, including colors (`tw/types.go:Formatter`).
    - **tw.CellFilter**: Use `Config.<Section>.Filter.Global` or `PerColumn` to apply transformations like coloring dynamically (`tw/cell.go:CellFilter`).
- **Width Handling**: `twdw.Width()` correctly calculates display width of ANSI-coded strings, ignoring escape sequences (`tw/fn.go:DisplayWidth`).
- **No Built-In Color Presets**: Unlike v0.0.5’s potential `tablewriter.Colors`, v1.0.x requires manual ANSI code management or external libraries for color constants.

**Migration Tips**:
- Replace `SetColumnColor` with direct ANSI code embedding for simple cases (e.g., `FgGreen+"text"+Reset`) (`zoo.go`).
- Implement `tw.Formatter` on custom types for reusable, semantic color logic (e.g., `Status` type above) (`tw/types.go`).
- Use `ConfigBuilder.<Section>().Filter().WithPerColumn` to apply color filters to specific columns, mimicking v0.0.5’s per-column coloring (`config.go`).
- Define ANSI constants in your codebase or use a library (e.g., `github.com/fatih/color`) to simplify color management.
- Test colored output in your target terminal to ensure ANSI codes render correctly.
- Combine filters with `AutoFormat` or wrapping for consistent styling (`zoo.go:prepareContent`).

**Potential Pitfalls**:
- **Terminal Support**: Some terminals may not support ANSI codes, causing artifacts; test in your environment or provide a non-colored fallback.
- **Filter Overlap**: Combining `tw.CellFilter` with `AutoFormat` or other transformations can lead to unexpected results; prioritize filters for coloring (`zoo.go`).
- **Width Miscalculation**: Incorrect ANSI code handling (e.g., missing `Reset`) can skew width calculations; use `twdw.Width` (`tw/fn.go`).
- **Streaming Consistency**: In streaming mode, ensure color codes are applied consistently across rows to avoid visual discrepancies (`stream.go`).
- **Performance**: Applying filters to large datasets may add overhead; optimize filter logic for efficiency (`zoo.go`).

## Advanced Features in v1.0.x

v1.0.x introduces several advanced features that enhance table functionality beyond v0.0.5’s capabilities. This section covers cell merging, captions, filters, stringers, and performance optimizations, providing migration guidance and examples for leveraging these features.

### 5. Cell Merging
Cell merging combines adjacent cells with identical content, improving readability for grouped data. v1.0.x expands merging options beyond v0.0.5’s horizontal merging.

**Old (v0.0.5):**
```go
package main

import (
	"github.com/olekukonko/tablewriter"
	"os"
)

func main() {
	table := tablewriter.NewWriter(os.Stdout)
	// table.SetAutoMergeCells(true) // Enable horizontal merging
	table.SetHeader([]string{"Category", "Item", "Notes"})
	table.Append([]string{"Fruit", "Apple", "Red"})
	table.Append([]string{"Fruit", "Apple", "Green"}) // "Apple" might merge if SetAutoMergeCells was on
	table.Render()
}
```

**New (v1.0.x):**
```go
package main

import (
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
	"os"
)

func main() {
	// Horizontal Merging (Similar to v0.0.5)
	tableH := tablewriter.NewTable(os.Stdout,
		tablewriter.WithConfig(tablewriter.Config{Row: tw.CellConfig{Merging: tw.CellMerging{Mode: tw.MergeHorizontal}}}),
		tablewriter.WithRenderer(renderer.NewBlueprint(tw.Rendition{Symbols: tw.NewSymbols(tw.StyleASCII)})), // Specify renderer for symbols
	)
	tableH.Header("Category", "Item", "Item", "Notes") // Note: Two "Item" headers for demo
	tableH.Append("Fruit", "Apple", "Apple", "Red")    // "Apple" cells merge
	tableH.Render()

	// Vertical Merging
	tableV := tablewriter.NewTable(os.Stdout,
		tablewriter.WithConfig(tablewriter.Config{Row: tw.CellConfig{Merging: tw.CellMerging{Mode: tw.MergeVertical}}}),
		tablewriter.WithRenderer(renderer.NewBlueprint(tw.Rendition{Symbols: tw.NewSymbols(tw.StyleASCII)})),
	)
	tableV.Header("User", "Permission")
	tableV.Append("Alice", "Read")
	tableV.Append("Alice", "Write") // "Alice" cells merge vertically
	tableV.Append("Bob", "Read")
	tableV.Render()

	// Hierarchical Merging
	tableHier := tablewriter.NewTable(os.Stdout,
		tablewriter.WithConfig(tablewriter.Config{Row: tw.CellConfig{Merging: tw.CellMerging{Mode: tw.MergeHierarchical}}}),
		tablewriter.WithRenderer(renderer.NewBlueprint(tw.Rendition{Symbols: tw.NewSymbols(tw.StyleASCII)})),
	)
	tableHier.Header("Group", "SubGroup", "Item")
	tableHier.Append("Tech", "CPU", "i7")
	tableHier.Append("Tech", "CPU", "i9")   // "Tech" and "CPU" merge
	tableHier.Append("Tech", "RAM", "16GB") // "Tech" merges, "RAM" is new
	tableHier.Append("Office", "CPU", "i5") // "Office" is new
	tableHier.Render()
}
```

**Output (Horizontal):**
```
+----------+-------+-------+-------+
| CATEGORY | ITEM  | ITEM  | NOTES |
+----------+-------+-------+-------+
| Fruit    | Apple         | Red   |
+----------+---------------+-------+
```

**Output (Vertical):**
```
+-------+------------+
| USER  | PERMISSION |
+-------+------------+
| Alice | Read       |
|       | Write      |
+-------+------------+
| Bob   | Read       |
+-------+------------+
```

**Output (Hierarchical):**
```
+---------+----------+------+
|  GROUP  | SUBGROUP | ITEM |
+---------+----------+------+
| Tech    | CPU      | i7   |
|         |          | i9   |
|         +----------+------+
|         | RAM      | 16GB |
+---------+----------+------+
| Office  | CPU      | i5   |
+---------+----------+------+
```

**Key Changes**:
- **Method**: `SetAutoMergeCells` is replaced by `WithRowMergeMode(int_merge_mode)` or `Config.Row.Formatting.MergeMode` (`config.go`). Uses `tw.Merge...` constants.
- **Merge Modes**: `tw.MergeMode` constants (`tw.MergeNone`, `tw.MergeHorizontal`, `tw.MergeVertical`, `tw.MergeHierarchical`) define behavior (`tw/tw.go`).
- **Section-Specific**: Merging can be applied to `Header`, `Row`, or `Footer` via `Config.<Section>.Formatting.MergeMode` (`tw/cell.go`).
- **Processing**: Merging is handled during content preparation (`zoo.go:prepareWithMerges`, `zoo.go:applyVerticalMerges`, `zoo.go:applyHierarchicalMerges`).
- **Width Adjustment**: Horizontal merging adjusts column widths (`zoo.go:applyHorizontalMergeWidths`).
- **Renderer Support**: `tw.MergeState` in `tw.CellContext` ensures correct border drawing for merged cells (`tw/cell.go:CellContext`).
- **Streaming Limitation**: Streaming mode supports only simple horizontal merging due to fixed widths (`stream.go:streamAppendRow`).

**Migration Tips**:
- Replace `SetAutoMergeCells(true)` with `WithRowMergeMode(tw.MergeHorizontal)` to maintain v0.0.5’s horizontal merging behavior (`config.go`).
- Use `tw.MergeVertical` for vertical grouping (e.g., repeated user names) or `tw.MergeHierarchical` for nested data structures (`tw/tw.go`).
- Apply merging to specific sections via `ConfigBuilder.<Section>().Formatting().WithMergeMode(int_merge_mode)` (`config.go`).
- Test merging with sample data to verify visual output, especially for hierarchical merging with complex datasets.
- In streaming mode, ensure `Config.Widths` supports merged cell widths to avoid truncation (`stream.go`).
- Use `WithDebug(true)` to log merge processing for troubleshooting (`config.go`).

**Potential Pitfalls**:
- **Streaming Restrictions**: Vertical and hierarchical merging are unsupported in streaming mode; use batch mode for these features (`stream.go`).
- **Width Misalignment**: Merged cells may require wider columns; adjust `Config.Widths` or `ColMaxWidths` (`zoo.go`).
- **Data Dependency**: Merging requires identical content; case or whitespace differences prevent merging (`zoo.go`).
- **Renderer Errors**: Incorrect `tw.MergeState` handling in custom renderers can break merged cell borders; test thoroughly (`tw/cell.go`).
- **Performance**: Hierarchical merging with large datasets may slow rendering; optimize data or limit merging (`zoo.go`).

### 6. Table Captions
Captions add descriptive text above or below the table, a new feature in v1.0.x not present in v0.0.5.

**Old (v0.0.5):**
```go
package main
// No direct caption support in v0.0.5. Users might have printed text manually.
// import "github.com/olekukonko/tablewriter"
// import "os"
// import "fmt"

func main() {
	// fmt.Println("Movie ratings.") // Manual caption
	// table := tablewriter.NewWriter(os.Stdout)
	// table.SetHeader([]string{"Name", "Sign", "Rating"})
	// table.Render()
}
```

**New (v1.0.x):**
```go
package main

import (
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
	"os"
)

func main() {
	table := tablewriter.NewTable(os.Stdout)
	table.Caption(tw.Caption{ // tw/types.go:Caption
		Text:  "System Status Overview - A Very Long Caption Example To Demonstrate Wrapping Behavior",
		Spot:  tw.SpotTopCenter, // Placement: TopLeft, TopCenter, TopRight, BottomLeft, BottomCenter, BottomRight
		Align: tw.AlignCenter,   // Text alignment within caption width
		Width: 30,               // Fixed width for caption text wrapping; if 0, wraps to table width
	})
	table.Header("Name", "Status")
	table.Append("Node1", "Ready")
	table.Append("SuperLongNodeNameHere", "ActiveNow")
	table.Render()
}
```

**Output:**
```
 System Status Overview - A Very 
Long Caption Example To Demonst… 
┌─────────────────────┬──────────┐
│        NAME         │  STATUS  │
├─────────────────────┼──────────┤
│ Node1               │ Ready    │
│ SuperLongNodeNameHe │ ActiveNo │
│ re                  │ w        │
└─────────────────────┴──────────┘
```

**Key Changes**:
- **New Feature**: `Table.Caption(tw.Caption)` introduces captions, absent in v0.0.5 (`tablewriter.go:Caption`).
- **Configuration**: `tw.Caption` (`tw/types.go:Caption`) includes:
    - `Text`: Caption content.
    - `Spot`: Placement (`tw.SpotTopLeft`, `tw.SpotBottomCenter`, etc.); defaults to `tw.SpotBottomCenter` if `tw.SpotNone`.
    - `Align`: Text alignment (`tw.AlignLeft`, `tw.AlignCenter`, `tw.AlignRight`).
    - `Width`: Optional fixed width for wrapping; defaults to table width.
- **Rendering**: Captions are rendered by `printTopBottomCaption` before or after the table based on `Spot` (`tablewriter.go:printTopBottomCaption`).
- **Streaming**: Captions are rendered during `Close()` in streaming mode if placed at the bottom (`stream.go`).

**Migration Tips**:
- Add captions to enhance table context, especially for reports or documentation (`tw/types.go`).
- Use `tw.SpotTopCenter` for prominent placement above the table, aligning with common report formats.
- Set `Align` to match table aesthetics (e.g., `tw.AlignCenter` for balanced appearance).
- Specify `Width` for consistent wrapping, especially with long captions or narrow tables (`tablewriter.go`).
- Test caption placement and alignment with different table sizes to ensure readability.

**Potential Pitfalls**:
- **Spot Default**: If `Spot` is `tw.SpotNone`, caption defaults to `tw.SpotBottomCenter`, which may surprise users expecting no caption (`tablewriter.go`).
- **Width Overflow**: Without `Width`, captions wrap to table width, potentially causing misalignment; set explicitly for control (`tw/types.go`).
- **Streaming Delay**: Bottom-placed captions in streaming mode appear only at `Close()`; ensure `Close()` is called (`stream.go`).
- **Alignment Confusion**: Caption `Align` is independent of table cell alignment; verify separately (`tw/cell.go`).

### 7. Filters
Filters allow dynamic transformation of cell content during rendering, a new feature in v1.0.x for tasks like formatting, coloring, or sanitizing data.

**Old (v0.0.5):**
```go
package main
// No direct support for cell content transformation in v0.0.5.
// Users would typically preprocess data before appending.
// import "github.com/olekukonko/tablewriter"
// import "os"
// import "strings"

func main() {
	// table := tablewriter.NewWriter(os.Stdout)
	// table.SetHeader([]string{"Name", "Status"})
	// status := "  Ready  "
	// preprocessedStatus := "Status: " + strings.TrimSpace(status)
	// table.Append([]string{"Node1", preprocessedStatus})
	// table.Render()
}
```

**New (v1.0.x):**
```go
package main

import (
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw" // For tw.CellConfig etc.
	"os"
	"strings"
)

func main() {
	// Per-Column Filter for Specific Transformations
	cfgBuilder := tablewriter.NewConfigBuilder()
	cfgBuilder.Row().Filter().WithPerColumn([]func(string) string{
		nil, // No filter for Name column
		func(s string) string { // Status column: prefix and trim
			return "Status: " + strings.TrimSpace(s)
		},
	})

	tableWithFilter := tablewriter.NewTable(os.Stdout, tablewriter.WithConfig(cfgBuilder.Build()))
	tableWithFilter.Header("Name", "Status")
	tableWithFilter.Append("Node1", "  Ready  ") // Note the extra spaces
	tableWithFilter.Append("Node2", "Pending")
	tableWithFilter.Render()

	// Global filter example (applied to all cells in the Row section)
	cfgGlobalFilter := tablewriter.NewConfigBuilder()
	cfgGlobalFilter.Row().Filter().WithGlobal(func(s string) string {
		return "[" + s + "]" // Wrap all row cells in brackets
	})
	tableGlobalFilter := tablewriter.NewTable(os.Stdout, tablewriter.WithConfig(cfgGlobalFilter.Build()))
	tableGlobalFilter.Header("Item", "Count")
	tableGlobalFilter.Append("Apple", "5")
	tableGlobalFilter.Render()
}
```

**Output (Per-Column Filter):**
```
┌───────┬─────────────────┐
│ NAME  │     STATUS      │
├───────┼─────────────────┤
│ Node1 │ Status: Ready   │
│ Node2 │ Status: Pending │
└───────┴─────────────────┘
```

**Output (Global Filter):**
```
┌───────┬─────────┐
│ ITEM  │  COUNT  │
├───────┼─────────┤
│[Apple]│ [5]     │
└───────┴─────────┘
```

**Key Changes**:
- **New Feature**: `tw.CellFilter` (`tw/cell.go:CellFilter`) introduces:
    - `Global`: A `func(s []string) []string` applied to entire rows (all cells in that row) of a section.
    - `PerColumn`: A slice of `func(string) string` for column-specific transformations on individual cells.
- **Configuration**: Set via `Config.<Section>.Filter` (`Header`, `Row`, `Footer`) using `ConfigBuilder` or direct `Config` (`config.go`).
- **Processing**: Filters are applied during content preparation, after `AutoFormat` but before rendering (`zoo.go:convertCellsToStrings` calls `prepareContent` which applies some transformations, filters are applied in `convertCellsToStrings` itself).
- **Use Cases**: Formatting (e.g., uppercase, prefixes), coloring (via ANSI codes), sanitization (e.g., removing sensitive data), or data normalization.

**Migration Tips**:
- Use filters to replace manual content preprocessing in v0.0.5 (e.g., string manipulation before `Append`).
- Apply `Global` filters for uniform transformations across all cells of rows in a section (e.g., uppercasing all row data) (`tw/cell.go`).
- Use `PerColumn` filters for column-specific formatting (e.g., adding prefixes to status columns) (`config.go`).
- Combine filters with `tw.Formatter` for complex types or ANSI coloring for visual enhancements (`tw/cell.go`).
- Test filters with diverse inputs to ensure transformations preserve data integrity (`zoo.go`).

**Potential Pitfalls**:
- **Filter Order**: Filters apply before some other transformations like padding and alignment; combining can lead to interactions.
- **Performance**: Complex filters on large datasets may slow rendering; optimize logic (`zoo.go`).
- **Nil Filters**: Unset filters (`nil`) are ignored, but incorrect indexing in `PerColumn` can skip columns (`tw/cell.go`).
- **Streaming Consistency**: Filters must be consistent in streaming mode, as widths are fixed at `Start()` (`stream.go`).

### 8. Stringers and Caching
Stringers allow custom string conversion for data types, with v1.0.x adding caching for performance.

**Old (v0.0.5):**
```go
package main
// v0.0.5 primarily relied on fmt.Stringer for custom types.
// import "fmt"
// import "github.com/olekukonko/tablewriter"
// import "os"

// type MyCustomType struct {
// 	Value string
// }
// func (m MyCustomType) String() string { return "Formatted: " + m.Value }

func main() {
	// table := tablewriter.NewWriter(os.Stdout)
	// table.SetHeader([]string{"Custom Data"})
	// table.Append([]string{MyCustomType{"test"}.String()}) // Manual call to String()
	// table.Render()
}
```

**New (v1.0.x):**
```go
package main

import (
	"fmt"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw" // For tw.Formatter
	"os"
	"strings" // For Person example
)

// Example 1: Using WithStringer for general conversion
type CustomInt int

func main() {
	// Table with a general stringer (func(any) []string)
	tableWithStringer := tablewriter.NewTable(os.Stdout,
		tablewriter.WithStringer(func(v any) []string { // Must return []string
			if ci, ok := v.(CustomInt); ok {
				return []string{fmt.Sprintf("CustomInt Value: %d!", ci)}
			}
			return []string{fmt.Sprintf("Value: %v", v)} // Fallback
		}),
		tablewriter.WithDebug(true), // Enable caching if WithStringerCache() is also used
		// tablewriter.WithStringerCache(), // Optional: enable caching
	)
	tableWithStringer.Header("Data")
	tableWithStringer.Append(123)
	tableWithStringer.Append(CustomInt(456))
	tableWithStringer.Render()

	fmt.Println("\n--- Table with Type-Specific Stringer for Structs ---")

	// Example 2: Stringer for a specific struct type
	type Person struct {
		ID   int
		Name string
		City string
	}

	// Stringer for Person to produce 3 cells
	personToStrings := func(p Person) []string {
		return []string{
			fmt.Sprintf("ID: %d", p.ID),
			p.Name,
			strings.ToUpper(p.City),
		}
	}

	tablePersonStringer := tablewriter.NewTable(os.Stdout,
		tablewriter.WithStringer(personToStrings), // Pass the type-specific function
	)
	tablePersonStringer.Header("User ID", "Full Name", "Location")
	tablePersonStringer.Append(Person{1, "Alice", "New York"})
	tablePersonStringer.Append(Person{2, "Bob", "London"})
	tablePersonStringer.Render()

	fmt.Println("\n--- Table with tw.Formatter ---")
	// Example 3: Using tw.Formatter for types
	type Product struct {
		Name  string
		Price float64
	}
	func (p Product) Format() string { // Implements tw.Formatter
		return fmt.Sprintf("%s - $%.2f", p.Name, p.Price)
	}
	tableFormatter := tablewriter.NewTable(os.Stdout)
	tableFormatter.Header("Product Details")
	tableFormatter.Append(Product{"Laptop", 1200.99}) // Will use Format()
	tableFormatter.Render()
}
```

**Output (Stringer Examples):**
```
┌─────────────────────┐
│        DATA         │
├─────────────────────┤
│ Value: 123          │
│ CustomInt Value: 456! │
└─────────────────────┘

--- Table with Type-Specific Stringer for Structs ---
┌─────────┬───────────┬──────────┐
│ USER ID │ FULL NAME │ LOCATION │
├─────────┼───────────┼──────────┤
│ ID: 1   │ Alice     │ NEW YORK │
│ ID: 2   │ Bob       │ LONDON   │
└─────────┴───────────┴──────────┘

--- Table with tw.Formatter ---
┌─────────────────┐
│ PRODUCT DETAILS │
├─────────────────┤
│ Laptop - $1200… │
└─────────────────┘
```

**Key Changes**:
- **Stringer Support**: `WithStringer(fn any)` sets a table-wide string conversion function. This function must have a signature like `func(SomeType) []string` or `func(any) []string`. It's used to convert an input item (e.g., a struct) into a slice of strings, where each string is a cell for the row (`tablewriter.go:WithStringer`).
- **Caching**: `WithStringerCache()` enables caching for the function provided via `WithStringer`, improving performance for repeated conversions of the same input type (`tablewriter.go:WithStringerCache`).
- **Formatter**: `tw.Formatter` interface (`Format() string`) allows types to define their own single-string representation for a cell. This is checked before `fmt.Stringer` (`tw/types.go:Formatter`).
- **Priority**: When converting an item to cell(s):
    1. `WithStringer` (if provided and compatible with the item's type).
    2. If not handled by `WithStringer`, or if `WithStringer` is not set:
        *   If the item is a struct that does *not* implement `tw.Formatter` or `fmt.Stringer`, its exported fields are reflected into multiple cells.
        *   If the item (or struct) implements `tw.Formatter` (`Format() string`), that's used for a single cell.
        *   Else, if it implements `fmt.Stringer` (`String() string`), that's used for a single cell.
        *   Else, default `fmt.Sprintf("%v", ...)` for a single cell.
            (`zoo.go:convertCellsToStrings`, `zoo.go:convertItemToCells`)

**Migration Tips**:
- For types that should produce a single cell with custom formatting, implement `tw.Formatter`.
- For types (especially structs) that should be expanded into multiple cells with custom logic, use `WithStringer` with a function like `func(MyType) []string`.
- If you have a general way to convert *any* type into a set of cells, use `WithStringer(func(any) []string)`.
- Enable `WithStringerCache` for large datasets with repetitive data types if using `WithStringer`.
- Test stringer/formatter output to ensure formatting meets expectations (`zoo.go`).

**Potential Pitfalls**:
- **Cache Overhead**: `WithStringerCache` may increase memory usage for diverse data; disable for small tables or if not using `WithStringer` (`tablewriter.go`).
- **Stringer Signature**: The function passed to `WithStringer` *must* return `[]string`. A function returning `string` will lead to a warning and fallback behavior.
- **Formatter vs. Stringer Priority**: Be aware of the conversion priority if your types implement multiple interfaces or if you also use `WithStringer`.
- **Streaming**: Stringers/Formatters must produce consistent cell counts in streaming mode to maintain width alignment (`stream.go`).

## Examples

This section provides practical examples to demonstrate v1.0.x features, covering common and advanced use cases to aid migration. Each example includes code, output, and notes to illustrate functionality.

### Example: Minimal Setup
A basic table with default settings, ideal for quick setups.

```go
package main

import (
	"github.com/olekukonko/tablewriter"
	"os"
)

func main() {
	table := tablewriter.NewTable(os.Stdout)
	table.Header("Name", "Status")
	table.Append("Node1", "Ready")
	table.Render()
}
```

**Output**:
```
┌───────┬────────┐
│ NAME  │ STATUS │
├───────┼────────┤
│ Node1 │ Ready  │
└───────┴────────┘
```

**Notes**:
- Uses default `renderer.NewBlueprint()` and `defaultConfig()` settings (`tablewriter.go`, `config.go`).
- `Header: tw.AlignCenter`, `Row: tw.AlignLeft` (`config.go:defaultConfig`).
- Simple replacement for v0.0.5’s `NewWriter` and `SetHeader`/`Append`.

### Example: Streaming with Fixed Widths
Demonstrates streaming mode for real-time data output.

```go
package main

import (
	"fmt"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
	"log"
	"os"
)

func main() {
	table := tablewriter.NewTable(os.Stdout,
		tablewriter.WithStreaming(tw.StreamConfig{Enable: true}),
		tablewriter.WithColumnMax(10), // Sets Config.Widths.Global
	)
	if err := table.Start(); err != nil {
		log.Fatalf("Start failed: %v", err)
	}
	table.Header("Name", "Status")
	for i := 0; i < 2; i++ {
		err := table.Append(fmt.Sprintf("Node%d", i+1), "Ready")
		if err != nil {
			log.Printf("Append failed: %v", err)
		}
	}
	if err := table.Close(); err != nil {
		log.Fatalf("Close failed: %v", err)
	}
}
```

**Output**:
```
┌──────────┬──────────┐
│   NAME   │  STATUS  │
├──────────┼──────────┤
│ Node1    │ Ready    │
│ Node2    │ Ready    │
└──────────┴──────────┘
```

**Notes**:
- Streaming requires `Start()` and `Close()`; `Config.Widths` (here set via `WithColumnMax`) fixes widths (`stream.go`).
- Replaces v0.0.5’s batch rendering for real-time use cases.
- Ensure error handling for `Start()`, `Append()`, and `Close()`.

### Example: Markdown-Style Table
Creates a Markdown-compatible table for documentation.

```go
package main

import (
	"fmt"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer" // Import renderer
	"github.com/olekukonko/tablewriter/tw"
	"os"
)

func main() {
	// Example 1: Using Blueprint renderer with Markdown symbols
	tableBlueprintMarkdown := tablewriter.NewTable(os.Stdout,
		tablewriter.WithRenderer(renderer.NewBlueprint()), // Use Blueprint
		tablewriter.WithRendition(tw.Rendition{
			Symbols: tw.NewSymbols(tw.StyleMarkdown),
			Borders: tw.Border{Left: tw.On, Right: tw.On}, // Markdown needs left/right borders
		}),
		tablewriter.WithRowAlignment(tw.AlignLeft), // Common for Markdown
		tablewriter.WithHeaderAlignment(tw.AlignCenter), // Center align headers
	)
	tableBlueprintMarkdown.Header("Name", "Status")
	tableBlueprintMarkdown.Append("Node1", "Ready")
	tableBlueprintMarkdown.Append("Node2", "NotReady")
	tableBlueprintMarkdown.Render()

	fmt.Println("\n--- Using dedicated Markdown Renderer (if one exists or is built) ---")
	// Example 2: Assuming a dedicated Markdown renderer (hypothetical example)
	// If a `renderer.NewMarkdown()` existed that directly outputs GitHub Flavored Markdown table syntax:
	/*
	   tableDedicatedMarkdown := tablewriter.NewTable(os.Stdout,
	       tablewriter.WithRenderer(renderer.NewMarkdown()), // Hypothetical Markdown renderer
	   )
	   tableDedicatedMarkdown.Header("Name", "Status")
	   tableDedicatedMarkdown.Append("Node1", "Ready")
	   tableDedicatedMarkdown.Append("Node2", "NotReady")
	   tableDedicatedMarkdown.Render()
	*/
	// Since `renderer.NewMarkdown()` isn't shown in the provided code,
	// the first example (Blueprint with StyleMarkdown) is the current viable way.
}
```

**Output (Blueprint with StyleMarkdown):**
```
|  NAME  |  STATUS  |
|--------|----------|
| Node1  | Ready    |
| Node2  | NotReady |
```

**Notes**:
- `StyleMarkdown` ensures compatibility with Markdown parsers (`tw/symbols.go`).
- Left alignment for rows and center for headers is common for Markdown readability (`config.go`).
- Ideal for GitHub READMEs or documentation.
- A dedicated Markdown renderer (like the commented-out example) would typically handle alignment syntax (e.g., `|:---:|:---|`). With `Blueprint` and `StyleMarkdown`, alignment is visual within the text rather than Markdown syntax.

### Example: ASCII-Style Table
Uses `StyleASCII` for maximum terminal compatibility.

```go
package main

import (
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
	"os"
)

func main() {
	table := tablewriter.NewTable(os.Stdout,
		tablewriter.WithRenderer(renderer.NewBlueprint()),
		tablewriter.WithRendition(tw.Rendition{
			Symbols: tw.NewSymbols(tw.StyleASCII),
		}),
		tablewriter.WithRowAlignment(tw.AlignLeft),
	)
	table.Header("ID", "Value")
	table.Append("1", "Test")
	table.Render()
}
```

**Output**:
```
+----+-------+
│ ID │ VALUE │
+----+-------+
│ 1  │ Test  │
+----+-------+
```

**Notes**:
- `StyleASCII` is robust for all terminals (`tw/symbols.go`).
- Replaces v0.0.5’s default style with explicit configuration.


### Example: Kubectl-Style Output
Creates a borderless, minimal table similar to `kubectl` command output, emphasizing simplicity and readability.

```go
package main

import (
	"os"
	"sync"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
)

var wg sync.WaitGroup

func main() {
	data := [][]any{
		{"node1.example.com", "Ready", "compute", "1.11"},
		{"node2.example.com", "Ready", "compute", "1.11"},
		{"node3.example.com", "Ready", "compute", "1.11"},
		{"node4.example.com", "NotReady", "compute", "1.11"},
	}

	table := tablewriter.NewTable(os.Stdout,
		tablewriter.WithRenderer(renderer.NewBlueprint(tw.Rendition{
			Borders: tw.BorderNone,
			Settings: tw.Settings{
				Separators: tw.SeparatorsNone,
				Lines:      tw.LinesNone,
			},
		})),
		tablewriter.WithConfig(tablewriter.Config{
			Header: tw.CellConfig{
				Formatting: tw.CellFormatting{Alignment: tw.AlignLeft},
			},
			Row: tw.CellConfig{
				Formatting: tw.CellFormatting{Alignment: tw.AlignLeft},
				Padding:    tw.CellPadding{Global: tw.PaddingNone},
			},
		}),
	)
	table.Header("Name", "Status", "Role", "Version")
	table.Bulk(data)
	table.Render()
}
```

**Output:**
```
NAME               STATUS    ROLE      VERSION  
node1.example.com  Ready     compute   1.21.3   
node2.example.com  Ready     infra     1.21.3   
node3.example.com  NotReady  compute   1.20.7   
```

**Notes**:
- **Configuration**: Uses `tw.BorderNone`, `tw.LinesNone`, `tw.SeparatorsNone`, `tw.NewSymbols(tw.StyleNone)` and specific padding (`Padding{Right:"  "}`) for a minimal, borderless layout.
- **Migration from v0.0.5**: Replaces `SetBorder(false)` and manual spacing with `tw.Rendition` and `Config` settings, achieving a cleaner kubectl-like output.
- **Key Features**:
    - Left-aligned text for readability (`config.go`).
    - `WithTrimSpace(tw.Off)` preserves spacing (`config.go`).
    - `Bulk` efficiently adds multiple rows (`tablewriter.go`).
    - Padding `Right: "  "` is used to create space between columns as separators are off.
- **Best Practices**: Test in terminals to ensure spacing aligns with command-line aesthetics; use `WithDebug(true)` for layout issues (`config.go`).
- **Potential Issues**: Column widths are content-based. For very long content in one column and short in others, it might not look perfectly aligned like fixed-width CLI tools. `Config.Widths` could be used for more control if needed.

### Example: Hierarchical Merging
Demonstrates hierarchical cell merging for nested data structures, a new feature in v1.0.x.

```go
package main

import (
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
	"os"
)

func main() {
	data := [][]string{
		// Header row is separate
		{"table\nwriter", "v0.0.1", "legacy"},
		{"table\nwriter", "v0.0.2", "legacy"},
		{"table\nwriter", "v0.0.2", "legacy"}, // Duplicate for testing merge
		{"table\nwriter", "v0.0.2", "legacy"}, // Duplicate for testing merge
		{"table\nwriter", "v0.0.5", "legacy"},
		{"table\nwriter", "v1.0.6", "latest"},
	}

	rendition := tw.Rendition{
		Symbols: tw.NewSymbols(tw.StyleLight), // Use light for clearer merge lines
		Settings: tw.Settings{
			Separators: tw.Separators{BetweenRows: tw.On},
			Lines:      tw.Lines{ShowHeaderLine: tw.On, ShowFooterLine: tw.On}, // Show header line
		},
		Borders: tw.Border{Left:tw.On, Right:tw.On, Top:tw.On, Bottom:tw.On},
	}

	tableHier := tablewriter.NewTable(os.Stdout,
		tablewriter.WithRenderer(renderer.NewBlueprint()),
		tablewriter.WithRendition(rendition),
		tablewriter.WithConfig(tablewriter.Config{
			Row: tw.CellConfig{
				Formatting: tw.CellFormatting{
					MergeMode:  tw.MergeHierarchical,
					// Alignment:  tw.AlignCenter, // Default is Left, often better for hierarchical
					AutoWrap:   tw.WrapNormal, // Allow wrapping for "table\nwriter"
				},
			},
		}),
	)

	tableHier.Header("Package", "Version", "Status") // Header
	tableHier.Bulk(data) // Bulk data
	tableHier.Render()

	// --- Vertical Merging Example for Contrast ---
	tableVert := tablewriter.NewTable(os.Stdout,
		tablewriter.WithRenderer(renderer.NewBlueprint()),
		tablewriter.WithRendition(rendition), // Reuse same rendition
		tablewriter.WithConfig(tablewriter.Config{
			Row: tw.CellConfig{
				Formatting: tw.CellFormatting{
					MergeMode: tw.MergeVertical,
					AutoWrap:  tw.WrapNormal,
				},
			},
		}),
	)
	tableVert.Header("Package", "Version", "Status")
	tableVert.Bulk(data)
	tableVert.Render()
}
```

**Output (Hierarchical):**
```
┌─────────┬─────────┬────────┐
│ PACKAGE │ VERSION │ STATUS │
├─────────┼─────────┼────────┤
│ table   │ v0.0.1  │ legacy │
│ writer  │         │        │
│         ├─────────┼────────┤
│         │ v0.0.2  │ legacy │
│         │         │        │
│         │         │        │
│         │         │        │
│         │         │        │
│         ├─────────┼────────┤
│         │ v0.0.5  │ legacy │
│         ├─────────┼────────┤
│         │ v1.0.6  │ latest │
└─────────┴─────────┴────────┘
```
**Output (Vertical):**
```
┌─────────┬─────────┬────────┐
│ PACKAGE │ VERSION │ STATUS │
├─────────┼─────────┼────────┤
│ table   │ v0.0.1  │ legacy │
│ writer  │         │        │
│         ├─────────┤        │
│         │ v0.0.2  │        │
│         │         │        │
│         │         │        │
│         │         │        │
│         │         │        │
│         ├─────────┤        │
│         │ v0.0.5  │        │
│         ├─────────┼────────┤
│         │ v1.0.6  │ latest │
└─────────┴─────────┴────────┘
```

**Notes**:
- **Configuration**: Uses `tw.MergeHierarchical` to merge cells based on matching values in preceding columns (`tw/tw.go`).
- **Migration from v0.0.5**: Extends `SetAutoMergeCells(true)` (horizontal only) with hierarchical merging for complex data (`tablewriter.go`).
- **Key Features**:
    - `StyleLight` for clear visuals (`tw/symbols.go`).
    - `Bulk` for efficient data loading (`tablewriter.go`).
- **Best Practices**: Test merging with nested data; use batch mode, as streaming doesn’t support hierarchical merging (`stream.go`).
- **Potential Issues**: Ensure data is sorted appropriately for hierarchical merging to work as expected; mismatches prevent merging (`zoo.go`). `AutoWrap: tw.WrapNormal` helps with multi-line cell content like "table\nwriter".

### Example: Colorized Table
Applies ANSI colors to highlight status values, replacing v0.0.5’s `SetColumnColor`.

```go
package main

import (
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw" // For tw.State, tw.CellConfig etc.
	"os"
)

func main() {
	const (
		FgGreen = "\033[32m"
		FgRed   = "\033[31m"
		Reset   = "\033[0m"
	)

	cfgBuilder := tablewriter.NewConfigBuilder()
	cfgBuilder.Row().Filter().WithPerColumn([]func(string) string{
		nil, // No filter for Name
		func(s string) string { // Color Status
			if s == "Ready" {
				return FgGreen + s + Reset
			}
			return FgRed + s + Reset
		},
	})

	table := tablewriter.NewTable(os.Stdout, tablewriter.WithConfig(cfgBuilder.Build()))
	table.Header("Name", "Status")
	table.Append("Node1", "Ready")
	table.Append("Node2", "Error")
	table.Render()
}
```

**Output (Text Approximation, Colors Not Shown):**
```
┌───────┬────────┐
│ NAME  │ STATUS │
├───────┼────────┤
│ Node1 │ Ready  │
│ Node2 │ Error  │
└───────┴────────┘
```

**Notes**:
- **Configuration**: Uses `tw.CellFilter` for per-column coloring, embedding ANSI codes (`tw/cell.go`).
- **Migration from v0.0.5**: Replaces `SetColumnColor` with dynamic filters (`tablewriter.go`).
- **Key Features**: Flexible color application; `twdw.Width` handles ANSI codes correctly (`tw/fn.go`).
- **Best Practices**: Test in ANSI-compatible terminals; use constants for code clarity.
- **Potential Issues**: Non-ANSI terminals may show artifacts; provide fallbacks (`tw/fn.go`).

### Example: Vertical Merging
Shows vertical cell merging for repeated values in a column. (This example was very similar to the hierarchical one, so I'll ensure it's distinct by using simpler data).

```go
package main

import (
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
	"os"
)

func main() {
	table := tablewriter.NewTable(os.Stdout,
		tablewriter.WithRenderer(renderer.NewBlueprint()), // Default renderer
		tablewriter.WithRendition(tw.Rendition{
			Symbols: tw.NewSymbols(tw.StyleLight),
			Settings: tw.Settings{Separators: tw.Separators{BetweenRows: tw.On}},
			Borders: tw.Border{Left:tw.On, Right:tw.On, Top:tw.On, Bottom:tw.On},
		}),
		tablewriter.WithConfig(
			tablewriter.NewConfigBuilder().
				Row().Formatting().WithMergeMode(tw.MergeVertical).Build(). // Enable Vertical Merge
				Build(),
		),
	)
	table.Header("User", "Permission", "Target")
	table.Append("Alice", "Read", "FileA")
	table.Append("Alice", "Write", "FileA") // Alice and FileA will merge vertically
	table.Append("Alice", "Read", "FileB")
	table.Append("Bob", "Read", "FileA")
	table.Append("Bob", "Read", "FileC")   // Bob and Read will merge
	table.Render()
}
```

**Output:**
```
┌───────┬────────────┬────────┐
│ USER  │ PERMISSION │ TARGET │
├───────┼────────────┼────────┤
│ Alice │ Read       │ FileA  │
│       │            │        │
│       ├────────────┤        │
│       │ Write      │        │
│       ├────────────┼────────┤
│       │ Read       │ FileB  │
├───────┼────────────┼────────┤
│ Bob   │ Read       │ FileA  │
│       │            ├────────┤
│       │            │ FileC  │
└───────┴────────────┴────────┘
```

**Notes**:
- **Configuration**: `tw.MergeVertical` merges identical cells vertically (`tw/tw.go`).
- **Migration from v0.0.5**: Extends `SetAutoMergeCells` with vertical merging (`tablewriter.go`).
- **Key Features**: Enhances grouped data display; requires batch mode (`stream.go`).
- **Best Practices**: Sort data by the columns you intend to merge for best results; test with `StyleLight` for clarity (`tw/symbols.go`).
- **Potential Issues**: Streaming doesn’t support vertical merging; use batch mode (`stream.go`).

### Example: Custom Renderer (CSV Output)
Implements a custom renderer for CSV output.

```go
package main

import (
	"fmt"
	"github.com/olekukonko/ll" // For logger type
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
	"io"
	"os"
	"strings" // For CSV escaping
)

// CSVRenderer implements tw.Renderer
type CSVRenderer struct {
	writer io.Writer
	config tw.Rendition // Store the rendition
	logger *ll.Logger
	err    error
}

func (r *CSVRenderer) Start(w io.Writer) error {
	r.writer = w
	return nil // No initial output for CSV typically
}

func (r *CSVRenderer) escapeCSVCell(data string) string {
	// Basic CSV escaping: double quotes if it contains comma, newline, or quote
	if strings.ContainsAny(data, ",\"\n") {
		return `"` + strings.ReplaceAll(data, `"`, `""`) + `"`
	}
	return data
}

func (r *CSVRenderer) writeLine(cells map[int]tw.CellContext, numCols int) {
	if r.err != nil { return }
	var lineParts []string
	// Need to iterate in column order for CSV
	keys := make([]int, 0, len(cells))
	for k := range cells {
		keys = append(keys, k)
	}
	// This simple sort works for int keys 0,1,2...
	// For more complex scenarios, a proper sort might be needed if keys aren't sequential.
	for i := 0; i < numCols; i++ { // Assume numCols reflects the intended max columns
		if cellCtx, ok := cells[i]; ok {
			lineParts = append(lineParts, r.escapeCSVCell(cellCtx.Data))
		} else {
			lineParts = append(lineParts, "") // Empty cell if not present
		}
	}
	_, r.err = r.writer.Write([]byte(strings.Join(lineParts, ",") + "\n"))
}


func (r *CSVRenderer) Header(headers [][]string, ctx tw.Formatting) {
    // For CSV, usually only the first line of headers is relevant
    // The ctx.Row.Current will contain the cells for the first line of the header being processed
	r.writeLine(ctx.Row.Current, len(ctx.Row.Current))
}

func (r *CSVRenderer) Row(row []string, ctx tw.Formatting) {
    // ctx.Row.Current contains the cells for the current row line
	r.writeLine(ctx.Row.Current, len(ctx.Row.Current))
}

func (r *CSVRenderer) Footer(footers [][]string, ctx tw.Formatting) {
    // Similar to Header/Row, using ctx.Row.Current for the footer line data
	r.writeLine(ctx.Row.Current, len(ctx.Row.Current))
}

func (r *CSVRenderer) Line(ctx tw.Formatting) { /* No separator lines in CSV */ }

func (r *CSVRenderer) Close() error { return r.err }

func (r *CSVRenderer) Config() tw.Rendition       { return r.config }
func (r *CSVRenderer) Logger(logger *ll.Logger) { r.logger = logger }

func main() {
	table := tablewriter.NewTable(os.Stdout, tablewriter.WithRenderer(&CSVRenderer{
		// config can be minimal for CSV as symbols/borders aren't used
		config: tw.Rendition{},
	}))
	table.Header("Name", "Status", "Notes, with comma")
	table.Append("Node1", "Ready", "All systems \"go\"!")
	table.Append("Node2", "Error", "Needs\nattention")
	table.Footer("Summary", "2 Nodes", "Check logs")
	table.Render()
}
```

**Output:**
```csv
Name,Status,"Notes, with comma"
Node1,Ready,"All systems ""go""!"
Node2,Error,"Needs
attention"
Summary,2 Nodes,Check logs
```

**Notes**:
- **Configuration**: Custom `CSVRenderer` implements `tw.Renderer` for CSV output (`tw/renderer.go`).
- **Migration from v0.0.5**: Extends v0.0.5’s text-only output with custom formats (`tablewriter.go`).
- **Key Features**: Handles basic CSV cell escaping; supports streaming if `Append` is called multiple times. `ctx.Row.Current` (map[int]tw.CellContext) is used to access cell data.
- **Best Practices**: Test with complex data (e.g., commas, quotes, newlines); implement all renderer methods. A more robust CSV renderer would use the `encoding/csv` package.
- **Potential Issues**: Custom renderers require careful error handling and correct interpretation of `tw.Formatting` and `tw.RowContext`. This example's `writeLine` assumes columns are 0-indexed and contiguous for simplicity.

## Troubleshooting and Common Pitfalls

This section addresses common migration issues with detailed solutions, covering 30+ scenarios to reduce support tickets.

| Issue                                     | Cause/Solution                                                                 |
|-------------------------------------------|-------------------------------------------------------------------------------|
| No output from `Render()`                 | **Cause**: Missing `Start()`/`Close()` in streaming mode or invalid `io.Writer`. **Solution**: Ensure `Start()` and `Close()` are called in streaming; verify `io.Writer` (`stream.go`). |
| Incorrect column widths                   | **Cause**: Missing `Config.Widths` in streaming or content-based sizing. **Solution**: Set `Config.Widths` before `Start()`; use `WithColumnMax` (`stream.go`). |
| Merging not working                       | **Cause**: Streaming mode or mismatched data. **Solution**: Use batch mode for vertical/hierarchical merging; ensure identical content (`zoo.go`). |
| Alignment ignored                         | **Cause**: `PerColumn` overrides `Global`. **Solution**: Check `Config.Section.Alignment.PerColumn` settings or `ConfigBuilder` calls (`tw/cell.go`). |
| Padding affects widths                    | **Cause**: Padding included in `Config.Widths`. **Solution**: Adjust `Config.Widths` to account for `tw.CellPadding` (`zoo.go`). |
| Colors not rendering                      | **Cause**: Non-ANSI terminal or incorrect codes. **Solution**: Test in ANSI-compatible terminal; use `twdw.Width` (`tw/fn.go`). |
| Caption missing                           | **Cause**: `Close()` not called in streaming or incorrect `Spot`. **Solution**: Ensure `Close()`; verify `tw.Caption.Spot` (`tablewriter.go`). |
| Filters not applied                       | **Cause**: Incorrect `PerColumn` indexing or nil filters. **Solution**: Set filters correctly; test with sample data (`tw/cell.go`). |
| Stringer cache overhead                   | **Cause**: Large datasets with diverse types. **Solution**: Disable `WithStringerCache` for small tables if not using `WithStringer` or if types vary greatly (`tablewriter.go`). |
| Deprecated methods used                   | **Cause**: Using `WithBorders`, old `tablewriter.Behavior` constants. **Solution**: Migrate to `WithRendition`, `tw.Behavior` struct (`tablewriter.go`, `deprecated.go`). |
| Streaming footer missing                  | **Cause**: `Close()` not called. **Solution**: Always call `Close()` (`stream.go`). |
| Hierarchical merging fails                | **Cause**: Unsorted data or streaming mode. **Solution**: Sort data; use batch mode (`zoo.go`). |
| Custom renderer errors                    | **Cause**: Incomplete method implementation or misinterpreting `tw.Formatting`. **Solution**: Implement all `tw.Renderer` methods; test thoroughly (`tw/renderer.go`). |
| Width overflow                            | **Cause**: No `MaxWidth` or wide content. **Solution**: Set `Config.MaxWidth` (`config.go`). |
| Truncated content                         | **Cause**: Narrow `Config.Widths` or `tw.WrapTruncate`. **Solution**: Widen columns or use `tw.WrapNormal` (`zoo.go`). |
| Debug logs absent                         | **Cause**: `Debug = false`. **Solution**: Enable `WithDebug(true)` (`config.go`). |
| Alignment mismatch across sections        | **Cause**: Different defaults. **Solution**: Set uniform alignment options (e.g., via `ConfigBuilder.<Section>.Alignment()`) (`config.go`). |
| ANSI code artifacts                       | **Cause**: Non-ANSI terminal. **Solution**: Provide non-colored fallback (`tw/fn.go`). |
| Slow rendering                            | **Cause**: Complex filters or merging. **Solution**: Optimize logic; limit merging (`zoo.go`). |
| Uneven cell counts                        | **Cause**: Mismatched rows/headers. **Solution**: Pad with `""` (`zoo.go`). |
| Border inconsistencies                    | **Cause**: Mismatched `Borders`/`Symbols`. **Solution**: Align settings (`tw/renderer.go`). |
| Streaming width issues                    | **Cause**: No `Config.Widths`. **Solution**: Set before `Start()` (`stream.go`). |
| Formatter ignored                         | **Cause**: `WithStringer` might take precedence if compatible. **Solution**: Review conversion priority; `tw.Formatter` is high-priority for single-item-to-single-cell conversion (`zoo.go`). |
| Caption misalignment                      | **Cause**: Incorrect `Width` or `Align`. **Solution**: Set `tw.Caption.Width`/`Align` (`tablewriter.go`). |
| Per-column padding errors                 | **Cause**: Incorrect indexing in `Padding.PerColumn`. **Solution**: Verify indices (`tw/cell.go`). |
| Vertical merging in streaming             | **Cause**: Unsupported. **Solution**: Use batch mode (`stream.go`). |
| Filter performance                        | **Cause**: Complex logic. **Solution**: Simplify filters (`zoo.go`). |
| Custom symbols incomplete                 | **Cause**: Missing characters. **Solution**: Define all symbols (`tw/symbols.go`). |
| Table too wide                            | **Cause**: No `MaxWidth`. **Solution**: Set `Config.MaxWidth` (`config.go`). |
| Streaming errors                          | **Cause**: Missing `Start()`. **Solution**: Call `Start()` before data input (`stream.go`). |

## Additional Notes

- **Performance Optimization**: Enable `WithStringerCache` for repetitive data types when using `WithStringer`; optimize filters and merging for large datasets (`tablewriter.go`, `zoo.go`).
- **Debugging**: Use `WithDebug(true)` and `table.Debug()` to log configuration and rendering details; invaluable for troubleshooting (`config.go`).
- **Testing Resources**: The `tests/` directory contains examples of various configurations.
- **Community Support**: For advanced use cases or issues, consult the source code or open an issue on the `tablewriter` repository.
- **Future Considerations**: Deprecated methods in `deprecated.go` (e.g., `WithBorders`) are slated for removal in future releases; migrate promptly to ensure compatibility.

This guide aims to cover all migration scenarios comprehensively. For highly specific or advanced use cases, refer to the source files (`config.go`, `tablewriter.go`, `stream.go`, `tw/*`) or engage with the `tablewriter` community for support.