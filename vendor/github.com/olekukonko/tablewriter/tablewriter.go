package tablewriter

import (
	"bytes"
	"io"
	"math"
	"reflect"
	"runtime"
	"strings"
	"unicode"

	"github.com/olekukonko/errors"
	"github.com/olekukonko/ll"
	"github.com/olekukonko/ll/lh"
	"github.com/olekukonko/tablewriter/pkg/twcache"
	"github.com/olekukonko/tablewriter/pkg/twwarp"
	"github.com/olekukonko/tablewriter/pkg/twwidth"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
)

// Table represents a table instance with content and rendering capabilities.
type Table struct {
	writer       io.Writer           // Destination for table output
	counters     []tw.Counter        // Counters for indices
	rows         [][]string          // Row data, one slice of strings per logical row
	headers      [][]string          // Header content
	footers      [][]string          // Footer content
	headerWidths tw.Mapper[int, int] // Computed widths for header columns
	rowWidths    tw.Mapper[int, int] // Computed widths for row columns
	footerWidths tw.Mapper[int, int] // Computed widths for footer columns
	renderer     tw.Renderer         // Engine for rendering the table
	config       Config              // Table configuration settings
	stringer     any                 // Function to convert rows to strings
	newLine      string              // Newline character (e.g., "\n")
	hasPrinted   bool                // Indicates if the table has been rendered
	logger       *ll.Logger          // Debug trace log
	trace        *bytes.Buffer       // Debug trace log

	// Caption fields
	caption tw.Caption

	// streaming
	streamWidths            tw.Mapper[int, int]           // Fixed column widths for streaming mode, calculated once
	streamFooterLines       [][]string                    // Processed footer lines for streaming, stored until Close().
	headerRendered          bool                          // Tracks if header has been rendered in streaming mode
	firstRowRendered        bool                          // Tracks if the first data row has been rendered in streaming mode
	lastRenderedLineContent []string                      // Content of the very last line rendered (for Previous context in streaming)
	lastRenderedMergeState  tw.Mapper[int, tw.MergeState] // Merge state of the very last line rendered (for Previous context in streaming)
	lastRenderedPosition    tw.Position                   // Position (Header/Row/Footer/Separator) of the last line rendered (for Previous context in streaming)
	streamNumCols           int                           // The derived number of columns in streaming mode
	streamRowCounter        int                           // Counter for rows rendered in streaming mode (0-indexed logical rows)

	// cache
	stringerCache twcache.Cache[reflect.Type, reflect.Value] // Cache for stringer reflection

	batchRenderNumCols      int
	isBatchRenderNumColsSet bool
}

// renderContext holds the core state for rendering the table.
type renderContext struct {
	table           *Table                                      // Reference to the table instance
	renderer        tw.Renderer                                 // Renderer instance
	cfg             tw.Rendition                                // Renderer configuration
	numCols         int                                         // Total number of columns
	headerLines     [][]string                                  // Processed header lines
	rowLines        [][][]string                                // Processed row lines
	footerLines     [][]string                                  // Processed footer lines
	widths          tw.Mapper[tw.Position, tw.Mapper[int, int]] // Widths per section
	footerPrepared  bool                                        // Tracks if footer is prepared
	emptyColumns    []bool                                      // Tracks which original columns are empty (true if empty)
	visibleColCount int                                         // Count of columns that are NOT empty
	logger          *ll.Logger                                  // Debug trace log
}

// mergeContext holds state related to cell merging.
type mergeContext struct {
	headerMerges map[int]tw.MergeState        // Merge states for header columns
	rowMerges    []map[int]tw.MergeState      // Merge states for each row
	footerMerges map[int]tw.MergeState        // Merge states for footer columns
	horzMerges   map[tw.Position]map[int]bool // Tracks horizontal merges (unused)
}

// helperContext holds additional data for rendering helpers.
type helperContext struct {
	position tw.Position // Section being processed (Header, Row, Footer)
	rowIdx   int         // Row index within section
	lineIdx  int         // Line index within row
	location tw.Location // Boundary location (First, Middle, End)
	line     []string    // Current line content
}

// renderMergeResponse holds cell context data from rendering operations.
type renderMergeResponse struct {
	cells        map[int]tw.CellContext // Current line cells
	prevCells    map[int]tw.CellContext // Previous line cells
	nextCells    map[int]tw.CellContext // Next line cells
	location     tw.Location            // Determined Location for this line
	cellsContent []string
}

// NewTable creates a new table instance with specified writer and options.
// Parameters include writer for output and optional configuration options.
// Returns a pointer to the initialized Table instance.
func NewTable(w io.Writer, opts ...Option) *Table {
	t := &Table{
		writer:       w,
		headerWidths: tw.NewMapper[int, int](),
		rowWidths:    tw.NewMapper[int, int](),
		footerWidths: tw.NewMapper[int, int](),
		renderer:     renderer.NewBlueprint(),
		config:       defaultConfig(),
		newLine:      tw.NewLine,
		trace:        &bytes.Buffer{},

		// Streaming
		streamWidths:           tw.NewMapper[int, int](), // Initialize empty mapper for streaming widths
		lastRenderedMergeState: tw.NewMapper[int, tw.MergeState](),
		headerRendered:         false,
		firstRowRendered:       false,
		lastRenderedPosition:   "",
		streamNumCols:          0,
		streamRowCounter:       0,

		//  Cache
		stringerCache: twcache.NewLRU[reflect.Type, reflect.Value](tw.DefaultCacheStringCapacity),
	}

	// set Options
	t.Options(opts...)

	t.logger.Infof("Table initialized with %d options", len(opts))
	return t
}

// NewWriter creates a new table with default settings for backward compatibility.
// It logs the creation if debugging is enabled.
func NewWriter(w io.Writer) *Table {
	t := NewTable(w)
	if t.logger != nil {
		t.logger.Debug("NewWriter created buffered Table")
	}
	return t
}

// Caption sets the table caption (legacy method).
// Defaults to BottomCenter alignment, wrapping to table width.
// Use SetCaptionOptions for more control.
func (t *Table) Caption(caption tw.Caption) *Table { // This is the one we modified
	originalSpot := caption.Spot
	originalAlign := caption.Align

	if caption.Spot == tw.SpotNone {
		caption.Spot = tw.SpotBottomCenter
		t.logger.Debugf("[Table.Caption] Input Spot was SpotNone, defaulting Spot to SpotBottomCenter (%d)", caption.Spot)
	}

	if caption.Align == "" || caption.Align == tw.AlignDefault || caption.Align == tw.AlignNone {
		switch caption.Spot {
		case tw.SpotTopLeft, tw.SpotBottomLeft:
			caption.Align = tw.AlignLeft
		case tw.SpotTopRight, tw.SpotBottomRight:
			caption.Align = tw.AlignRight
		default:
			caption.Align = tw.AlignCenter
		}
		t.logger.Debugf("[Table.Caption] Input Align was empty/default, defaulting Align to %s for Spot %d", caption.Align, caption.Spot)
	}

	t.caption = caption // t.caption on the struct is now updated.
	t.logger.Debugf("Caption method called: Input(Spot:%v, Align:%q), Final(Spot:%v, Align:%q), Text:'%.20s', MaxWidth:%d",
		originalSpot, originalAlign, t.caption.Spot, t.caption.Align, t.caption.Text, t.caption.Width)
	return t
}

// Append adds data to the current row being built for the table.
// This method always contributes to a single logical row in the table.
// To add multiple distinct rows, call Append multiple times (once for each row's data)
// or use the Bulk() method if providing a slice where each element is a row.
func (t *Table) Append(rows ...interface{}) error {
	t.ensureInitialized()

	if t.config.Stream.Enable && t.hasPrinted {
		// Streaming logic remains unchanged, as AutoHeader is a batch-mode concept.
		t.logger.Debugf("Append() called in streaming mode with %d items for a single row", len(rows))
		var rowItemForStream interface{}
		if len(rows) == 1 {
			rowItemForStream = rows[0]
		} else {
			rowItemForStream = rows
		}
		if err := t.streamAppendRow(rowItemForStream); err != nil {
			t.logger.Errorf("Error rendering streaming row: %v", err)
			return errors.Newf("failed to stream append row").Wrap(err)
		}
		return nil
	}

	// Batch Mode Logic
	t.logger.Debugf("Append (Batch) received %d arguments: %v", len(rows), rows)

	var cellsSource interface{}
	if len(rows) == 1 {
		cellsSource = rows[0]
	} else {
		cellsSource = rows
	}
	// Check if we should attempt to auto-generate headers from this append operation.
	// Conditions: AutoHeader is on, no headers are set yet, and this is the first data row.
	isFirstRow := len(t.rows) == 0
	if t.config.Behavior.Structs.AutoHeader.Enabled() && len(t.headers) == 0 && isFirstRow {
		t.logger.Debug("Append: Triggering AutoHeader for the first row.")
		headers := t.extractHeadersFromStruct(cellsSource)
		if len(headers) > 0 {
			// Set the extracted headers. The Header() method handles the rest.
			t.Header(headers)
		}
	}

	cells, err := t.convertCellsToStrings(cellsSource, t.config.Row)
	if err != nil {
		t.logger.Errorf("Append (Batch) failed for cellsSource %v: %v", cellsSource, err)
		return err
	}
	t.rows = append(t.rows, cells)

	t.logger.Debugf("Append (Batch) completed for one row, total rows in table: %d", len(t.rows))
	return nil
}

// Bulk adds multiple rows from a slice to the table.
// If Behavior.AutoHeader is enabled, no headers set, and rows is a slice of structs,
// automatically extracts/sets headers from the first struct.
func (t *Table) Bulk(rows interface{}) error {
	rv := reflect.ValueOf(rows)
	if rv.Kind() != reflect.Slice {
		return errors.Newf("Bulk expects a slice, got %T", rows)
	}
	if rv.Len() == 0 {
		return nil
	}

	// AutoHeader logic remains here, as it's a "Bulk" operation concept.
	if t.config.Behavior.Structs.AutoHeader.Enabled() && len(t.headers) == 0 {
		first := rv.Index(0).Interface()
		// We can now correctly get headers from pointers or embedded structs
		headers := t.extractHeadersFromStruct(first)
		if len(headers) > 0 {
			t.Header(headers)
		}
	}

	// The rest of the logic is now just a loop over Append.
	for i := 0; i < rv.Len(); i++ {
		row := rv.Index(i).Interface()
		if err := t.Append(row); err != nil { // Use Append
			return err
		}
	}
	return nil
}

// Config returns the current table configuration.
// No parameters are required.
// Returns the Config struct with current settings.
func (t *Table) Config() Config {
	return t.config
}

// Configure updates the table's configuration using a provided function.
// Parameter fn is a function that modifies the Config struct.
// Returns the Table instance for method chaining.
func (t *Table) Configure(fn func(cfg *Config)) *Table {
	fn(&t.config) // Let the user modify the config directly
	// Handle any immediate side-effects of config changes, e.g., logger state
	if t.config.Debug {
		t.logger.Enable()
		t.logger.Resume() // in case it was suspended
	} else {
		t.logger.Disable()
		t.logger.Suspend() // suspend totally, especially because of tight loops
	}
	t.logger.Debugf("Configure complete. New t.config: %+v", t.config)
	return t
}

// Debug retrieves the accumulated debug trace logs.
// No parameters are required.
// Returns a slice of debug messages including renderer logs.
func (t *Table) Debug() *bytes.Buffer {
	return t.trace
}

// Header sets the table's header content, padding to match column count.
// Parameter elements is a slice of strings for header content.
// No return value.
// In streaming mode, this processes and renders the header immediately.
func (t *Table) Header(elements ...any) {
	t.ensureInitialized()
	t.logger.Debugf("Header() method called with raw variadic elements: %v (len %d). Streaming: %v, Started: %v", elements, len(elements), t.config.Stream.Enable, t.hasPrinted)

	// just forget
	if t.config.Behavior.Header.Hide.Enabled() {
		return
	}

	// add come common default
	if t.config.Header.Formatting.AutoFormat == tw.Unknown {
		t.config.Header.Formatting.AutoFormat = tw.On
	}

	if t.config.Stream.Enable && t.hasPrinted {
		//  Streaming Path
		actualCellsToProcess := t.processVariadic(elements)
		headersAsStrings, err := t.convertCellsToStrings(actualCellsToProcess, t.config.Header)
		if err != nil {
			t.logger.Errorf("Header(): Failed to convert header elements to strings for streaming: %v", err)
			headersAsStrings = []string{} // Use empty on error
		}
		errStream := t.streamRenderHeader(headersAsStrings) // streamRenderHeader handles padding to streamNumCols internally
		if errStream != nil {
			t.logger.Errorf("Error rendering streaming header: %v", errStream)
		}
		return
	}

	//  Batch Path
	processedElements := t.processVariadic(elements)
	t.logger.Debugf("Header() (Batch): Effective cells to process: %v", processedElements)

	headersAsStrings, err := t.convertCellsToStrings(processedElements, t.config.Header)
	if err != nil {
		t.logger.Errorf("Header() (Batch): Failed to convert to strings: %v", err)
		t.headers = [][]string{} // Set to empty on error
		return
	}

	// prepareContent uses t.config.Header for AutoFormat and MaxWidth constraints.
	// It processes based on the number of columns in headersAsStrings.
	preparedHeaderLines := t.prepareContent(headersAsStrings, t.config.Header)
	t.headers = preparedHeaderLines // Store directly. Padding to t.maxColumns() will happen in prepareContexts.

	t.logger.Debugf("Header set (batch mode), lines stored: %d. First line if exists: %v", len(t.headers), func() []string {
		if len(t.headers) > 0 {
			return t.headers[0]
		} else {
			return nil
		}
	}())
}

// Footer sets the table's footer content, padding to match column count.
// Parameter footers is a slice of strings for footer content.
// No return value.
// Footer sets the table's footer content.
// Parameter footers is a slice of strings for footer content.
// In streaming mode, this processes and stores the footer for rendering by Close().
func (t *Table) Footer(elements ...any) {
	t.ensureInitialized()
	t.logger.Debugf("Footer() method called with raw variadic elements: %v (len %d). Streaming: %v, Started: %v", elements, len(elements), t.config.Stream.Enable, t.hasPrinted)

	// just forget
	if t.config.Behavior.Footer.Hide.Enabled() {
		return
	}

	if t.config.Stream.Enable && t.hasPrinted {
		//  Streaming Path
		actualCellsToProcess := t.processVariadic(elements)
		footersAsStrings, err := t.convertCellsToStrings(actualCellsToProcess, t.config.Footer)
		if err != nil {
			t.logger.Errorf("Footer(): Failed to convert footer elements to strings for streaming: %v", err)
			footersAsStrings = []string{} // Use empty on error
		}
		errStream := t.streamStoreFooter(footersAsStrings) // streamStoreFooter handles padding to streamNumCols internally
		if errStream != nil {
			t.logger.Errorf("Error processing streaming footer: %v", errStream)
		}
		return
	}

	//  Batch Path
	processedElements := t.processVariadic(elements)
	t.logger.Debugf("Footer() (Batch): Effective cells to process: %v", processedElements)

	footersAsStrings, err := t.convertCellsToStrings(processedElements, t.config.Footer)
	if err != nil {
		t.logger.Errorf("Footer() (Batch): Failed to convert to strings: %v", err)
		t.footers = [][]string{} // Set to empty on error
		return
	}

	preparedFooterLines := t.prepareContent(footersAsStrings, t.config.Footer)
	t.footers = preparedFooterLines // Store directly. Padding to t.maxColumns() will happen in prepareContexts.

	t.logger.Debugf("Footer set (batch mode), lines stored: %d. First line if exists: %v",
		len(t.footers), func() []string {
			if len(t.footers) > 0 {
				return t.footers[0]
			} else {
				return nil
			}
		}())
}

// Options updates the table's Options using a provided function.
// Parameter opts is a function that modifies the Table struct.
// Returns the Table instance for method chaining.
func (t *Table) Options(opts ...Option) *Table {
	// add logger
	if t.logger == nil {
		t.logger = ll.New("table").Handler(lh.NewTextHandler(t.trace))
	}

	// Disable and suspend the logger before applying options to prevent premature
	// debug output from renderer methods (e.g., Blueprint.Rendition) triggered by
	// options like WithRendition. Without this, a previously-enabled logger would
	// still be active on the renderer during option application, causing debug
	// messages even when WithDebug(false) is being applied.
	t.logger.Disable()
	t.logger.Suspend()
	t.renderer.Logger(t.logger)

	// loop through options
	for _, opt := range opts {
		opt(t)
	}

	// force debugging mode if set
	if t.config.Debug {
		t.logger.Enable()
		t.logger.Resume()
	} else {
		t.logger.Disable()
		t.logger.Suspend()
	}

	// Get additional system information for debugging
	goVersion := runtime.Version()
	goOS := runtime.GOOS
	goArch := runtime.GOARCH
	numCPU := runtime.NumCPU()

	// Use the new struct-based info.
	// No type assertions or magic strings needed.
	info := twwidth.Debugging()

	t.logger.Infof("Go Runtime: Version=%s, OS=%s, Arch=%s, CPUs=%d",
		goVersion, goOS, goArch, numCPU)

	t.logger.Infof("Environment: LC_CTYPE=%s, LANG=%s, TERM=%s, TERM_PROGRAM=%s",
		info.Raw.LC_CTYPE,
		info.Raw.LANG,
		info.Raw.TERM,
		info.Raw.TERM_PROGRAM,
	)

	t.logger.Infof("East Asian Detection: Auto=%v, Mode=%s, ModernEnv=%v, CJKLocale=%v",
		info.AutoUseEastAsian,
		info.DetectionMode,
		info.Derived.IsModernEnv,
		info.Derived.IsCJKLocale,
	)

	// send logger to renderer
	t.renderer.Logger(t.logger)
	return t
}

// Reset clears all data (headers, rows, footers, caption) and rendering state
// from the table, allowing the Table instance to be reused for a new table
// with the same configuration and writer.
// It does NOT reset the configuration itself (set by NewTable options or Configure)
// or the underlying io.Writer.
func (t *Table) Reset() {
	t.logger.Debug("Reset() called. Clearing table data and render state.")

	// Clear data slices
	t.rows = nil    // Or t.rows = make([][]string, 0)
	t.headers = nil // Or t.headers = make([][]string, 0)
	t.footers = nil // Or t.footers = make([][]string, 0)

	// Reset width mappers (important for recalculating widths for the new table)
	t.headerWidths = tw.NewMapper[int, int]()
	t.rowWidths = tw.NewMapper[int, int]()
	t.footerWidths = tw.NewMapper[int, int]()

	// Reset caption
	t.caption = tw.Caption{} // Reset to zero value

	// Reset rendering state flags
	t.hasPrinted = false // Critical for allowing Render() or stream Start() again

	// Reset streaming-specific state
	// (Important if the table was used in streaming mode and might be reused in batch or another stream)
	t.streamWidths = tw.NewMapper[int, int]()
	t.streamFooterLines = nil
	t.headerRendered = false
	t.firstRowRendered = false
	t.lastRenderedLineContent = nil
	t.lastRenderedMergeState = tw.NewMapper[int, tw.MergeState]() // Re-initialize
	t.lastRenderedPosition = ""
	t.streamNumCols = 0
	t.streamRowCounter = 0

	// The stringer and its cache are part of the table's configuration,
	if t.stringerCache == nil {
		t.stringerCache = twcache.NewLRU[reflect.Type, reflect.Value](tw.DefaultCacheStringCapacity)
		t.logger.Debug("Reset(): Stringer cache reset to default capacity.")
	} else {
		t.stringerCache.Purge()
		t.logger.Debug("Reset(): Stringer cache cleared.")
	}

	// If the renderer has its own state that needs resetting after a table is done,
	// this would be the place to call a renderer.Reset() method if it existed.
	// Most current renderers are stateless per render call or reset in their Start/Close.
	// For instance, HTML and SVG renderers have their own Reset method.
	// It might be good practice to call it if available.
	if r, ok := t.renderer.(interface{ Reset() }); ok {
		t.logger.Debug("Reset(): Calling Reset() on the current renderer.")
		r.Reset()
	}

	t.logger.Info("Table instance has been reset.")
}

// Render triggers the table rendering process to the configured writer.
// No parameters are required.
// Returns an error if rendering fails.
func (t *Table) Render() error {
	return t.render()
}

// Lines returns the total number of lines rendered.
// This method is only effective if the WithLineCounter() option was used during
// table initialization and must be called *after* Render().
// It actively searches for the default tw.LineCounter among all active counters.
// It returns -1 if the line counter was not enabled.
func (t *Table) Lines() int {
	for _, counter := range t.counters {
		if lc, ok := counter.(*tw.LineCounter); ok {
			return lc.Total()
		}
	}
	// use -1 to indicate no line counter is attached
	return -1
}

// Counters returns the slice of all active counter instances.
// This is useful when multiple counters are enabled.
// It must be called *after* Render().
func (t *Table) Counters() []tw.Counter {
	return t.counters
}

// Trimmer trims whitespace from a string based on the Table’s configuration.
// It conditionally applies trimming based on TrimSpace and TrimTab settings.
//
// Behavior Matrix:
// - TrimSpace=On, TrimTab=On:   Uses strings.TrimSpace (removes all Unicode space including \t).
// - TrimSpace=On, TrimTab=Off:  Removes spaces/newlines but PRESERVES tabs.
// - TrimSpace=Off, TrimTab=On:  Removes only tabs.
// - TrimSpace=Off, TrimTab=Off: Returns string unchanged.
func (t *Table) Trimmer(str string) string {
	trimSpace := t.config.Behavior.TrimSpace.Enabled()
	trimTab := t.config.Behavior.TrimTab.Enabled()

	// Fast Path 1: If both are enabled (Default), use the stdlib optimized TrimSpace
	if trimSpace && trimTab {
		return strings.TrimSpace(str)
	}

	// Fast Path 2: If both are disabled, return raw string
	if !trimSpace && !trimTab {
		return str
	}

	// Granular Trimming via TrimFunc
	return strings.TrimFunc(str, func(r rune) bool {
		if twwidth.IsTab(r) {
			return trimTab // Return true to trim if TrimTab is On
		}
		if trimSpace {
			return unicode.IsSpace(r) // Trim other whitespace if TrimSpace is On
		}
		return false
	})
}

// appendSingle adds a single row to the table's row data.
// Parameter row is the data to append, converted via stringer if needed.
// Returns an error if conversion or appending fails.
func (t *Table) appendSingle(row interface{}) error {
	t.ensureInitialized() // Already here

	if t.config.Stream.Enable && t.hasPrinted { // If streaming is active
		t.logger.Debugf("appendSingle: Dispatching to streamAppendRow for row: %v", row)
		return t.streamAppendRow(row) // Call the streaming render function
	}

	t.logger.Debugf("appendSingle: Processing for batch mode, row: %v", row)
	cells, err := t.convertCellsToStrings(row, t.config.Row)
	if err != nil {
		t.logger.Debugf("Error in convertCellsToStrings (batch mode): %v", err)
		return err
	}
	t.rows = append(t.rows, cells) // Add to batch storage
	t.logger.Debugf("Row appended to batch t.rows, total batch rows: %d", len(t.rows))
	return nil
}

// buildAligns constructs a map of column alignments from configuration.
// Parameter config provides alignment settings for the section.
// Returns a map of column indices to alignment settings.
func (t *Table) buildAligns(config tw.CellConfig) map[int]tw.Align {
	// Start with global alignment, preferring deprecated Formatting.Alignment
	effectiveGlobalAlign := config.Formatting.Alignment
	if effectiveGlobalAlign == tw.Empty || effectiveGlobalAlign == tw.Skip {
		effectiveGlobalAlign = config.Alignment.Global
		if config.Formatting.Alignment != tw.Empty && config.Formatting.Alignment != tw.Skip {
			t.logger.Warnf("Using deprecated CellFormatting.Alignment (%s). Migrate to CellConfig.Alignment.Global.", config.Formatting.Alignment)
		}
	}

	// Use per-column alignments, preferring deprecated ColumnAligns
	effectivePerColumn := config.ColumnAligns
	if len(effectivePerColumn) == 0 && len(config.Alignment.PerColumn) > 0 {
		effectivePerColumn = make([]tw.Align, len(config.Alignment.PerColumn))
		copy(effectivePerColumn, config.Alignment.PerColumn)
		if len(config.ColumnAligns) > 0 {
			t.logger.Warnf("Using deprecated CellConfig.ColumnAligns (%v). Migrate to CellConfig.Alignment.PerColumn.", config.ColumnAligns)
		}
	}

	// Log input for debugging
	t.logger.Debugf("buildAligns INPUT: deprecated Formatting.Alignment=%s, deprecated ColumnAligns=%v, config.Alignment.Global=%s, config.Alignment.PerColumn=%v",
		config.Formatting.Alignment, config.ColumnAligns, config.Alignment.Global, config.Alignment.PerColumn)

	numColsToUse := t.getNumColsToUse()
	colAlignsResult := make(map[int]tw.Align)
	for i := 0; i < numColsToUse; i++ {
		currentAlign := effectiveGlobalAlign
		if i < len(effectivePerColumn) && effectivePerColumn[i] != tw.Empty && effectivePerColumn[i] != tw.Skip {
			currentAlign = effectivePerColumn[i]
		}
		// Skip validation here; rely on rendering to handle invalid alignments
		colAlignsResult[i] = currentAlign
	}

	t.logger.Debugf("Aligns built: %v (length %d)", colAlignsResult, len(colAlignsResult))
	return colAlignsResult
}

// buildPadding constructs a map of column padding settings from configuration.
// Parameter padding provides padding settings for the section.
// Returns a map of column indices to padding settings.
func (t *Table) buildPadding(padding tw.CellPadding) map[int]tw.Padding {
	numColsToUse := t.getNumColsToUse()
	colPadding := make(map[int]tw.Padding)
	for i := 0; i < numColsToUse; i++ {
		if i < len(padding.PerColumn) && padding.PerColumn[i].Paddable() {
			colPadding[i] = padding.PerColumn[i]
		} else {
			colPadding[i] = padding.Global
		}
	}
	t.logger.Debugf("Padding built: %v (length %d)", colPadding, len(colPadding))
	return colPadding
}

// ensureInitialized initializes required fields before use.
// No parameters are required.
// No return value.
func (t *Table) ensureInitialized() {
	if t.headerWidths == nil {
		t.headerWidths = tw.NewMapper[int, int]()
	}
	if t.rowWidths == nil {
		t.rowWidths = tw.NewMapper[int, int]()
	}
	if t.footerWidths == nil {
		t.footerWidths = tw.NewMapper[int, int]()
	}
	if t.renderer == nil {
		t.renderer = renderer.NewBlueprint()
	}
	t.logger.Debug("ensureInitialized called")
}

// finalizeHierarchicalMergeBlock sets Span and End for hierarchical merges.
// Parameters include ctx, mctx, col, startRow, and endRow.
// No return value.
func (t *Table) finalizeHierarchicalMergeBlock(ctx *renderContext, mctx *mergeContext, col, startRow, endRow int) {
	if endRow < startRow {
		ctx.logger.Debugf("Hierarchical merge FINALIZE WARNING: Invalid block col %d, start %d > end %d", col, startRow, endRow)
		return
	}
	if startRow < 0 || endRow < 0 {
		ctx.logger.Debugf("Hierarchical merge FINALIZE WARNING: Negative row indices col %d, start %d, end %d", col, startRow, endRow)
		return
	}
	requiredLen := endRow + 1
	if requiredLen > len(mctx.rowMerges) {
		ctx.logger.Debugf("Hierarchical merge FINALIZE WARNING: rowMerges slice too short (len %d) for endRow %d", len(mctx.rowMerges), endRow)
		return
	}
	if mctx.rowMerges[startRow] == nil {
		mctx.rowMerges[startRow] = make(map[int]tw.MergeState)
	}
	if mctx.rowMerges[endRow] == nil {
		mctx.rowMerges[endRow] = make(map[int]tw.MergeState)
	}

	finalSpan := (endRow - startRow) + 1
	ctx.logger.Debugf("Finalizing H-merge block: col=%d, startRow=%d, endRow=%d, span=%d", col, startRow, endRow, finalSpan)

	startState := mctx.rowMerges[startRow][col]
	if startState.Hierarchical.Present && startState.Hierarchical.Start {
		startState.Hierarchical.Span = finalSpan
		startState.Hierarchical.End = finalSpan == 1
		mctx.rowMerges[startRow][col] = startState
		ctx.logger.Debugf(" -> Updated start state: %+v", startState.Hierarchical)
	} else {
		ctx.logger.Debugf("Hierarchical merge FINALIZE WARNING: col %d, startRow %d was not marked as Present/Start? Current state: %+v. Attempting recovery.", col, startRow, startState.Hierarchical)
		startState.Hierarchical.Present = true
		startState.Hierarchical.Start = true
		startState.Hierarchical.Span = finalSpan
		startState.Hierarchical.End = finalSpan == 1
		mctx.rowMerges[startRow][col] = startState
	}

	if endRow > startRow {
		endState := mctx.rowMerges[endRow][col]
		if endState.Hierarchical.Present && !endState.Hierarchical.Start {
			endState.Hierarchical.End = true
			endState.Hierarchical.Span = finalSpan
			mctx.rowMerges[endRow][col] = endState
			ctx.logger.Debugf(" -> Updated end state: %+v", endState.Hierarchical)
		} else {
			ctx.logger.Debugf("Hierarchical merge FINALIZE WARNING: col %d, endRow %d was not marked as Present/Continuation? Current state: %+v. Attempting recovery.", col, endRow, endState.Hierarchical)
			endState.Hierarchical.Present = true
			endState.Hierarchical.Start = false
			endState.Hierarchical.End = true
			endState.Hierarchical.Span = finalSpan
			mctx.rowMerges[endRow][col] = endState
		}
	} else {
		ctx.logger.Debugf(" -> Span is 1, startRow is also endRow.")
	}
}

// getLevel maps a position to its rendering level.
// Parameter position specifies the section (Header, Row, Footer).
// Returns the corresponding tw.Level (Header, Body, Footer).
func (t *Table) getLevel(position tw.Position) tw.Level {
	switch position {
	case tw.Header:
		return tw.LevelHeader
	case tw.Row:
		return tw.LevelBody
	case tw.Footer:
		return tw.LevelFooter
	default:
		return tw.LevelBody
	}
}

// hasFooterElements checks if the footer has renderable elements.
// No parameters are required.
// Returns true if footer has content or padding, false otherwise.
func (t *Table) hasFooterElements() bool {
	hasContent := len(t.footers) > 0
	hasTopPadding := t.config.Footer.Padding.Global.Top != tw.Empty
	hasBottomPaddingConfig := t.config.Footer.Padding.Global.Bottom != tw.Empty || t.hasPerColumnBottomPadding()
	return hasContent || hasTopPadding || hasBottomPaddingConfig
}

// hasPerColumnBottomPadding checks for per-column bottom padding in footer.
// No parameters are required.
// Returns true if any per-column bottom padding is defined.
func (t *Table) hasPerColumnBottomPadding() bool {
	if t.config.Footer.Padding.PerColumn == nil {
		return false
	}
	for _, pad := range t.config.Footer.Padding.PerColumn {
		if pad.Bottom != tw.Empty {
			return true
		}
	}
	return false
}

// Logger retrieves the table's logger instance.
// No parameters are required.
// Returns the ll.Logger instance used for debug tracing.
func (t *Table) Logger() *ll.Logger {
	return t.logger
}

// Renderer retrieves the current renderer instance used by the table.
// No parameters are required.
// Returns the tw.Renderer interface instance.
func (t *Table) Renderer() tw.Renderer {
	t.logger.Debug("Renderer requested")
	return t.renderer
}

// maxColumns calculates the maximum column count across sections.
// No parameters are required.
// Returns the highest number of columns found.
func (t *Table) maxColumns() int {
	m := 0
	if len(t.headers) > 0 && len(t.headers[0]) > m {
		m = len(t.headers[0])
	}
	for _, row := range t.rows {
		if len(row) > m {
			m = len(row)
		}
	}
	if len(t.footers) > 0 && len(t.footers[0]) > m {
		m = len(t.footers[0])
	}
	t.logger.Debugf("Max columns: %d", m)
	return m
}

// printTopBottomCaption prints the table's caption at the specified top or bottom position.
// It wraps the caption text to fit the table width or a user-defined width, aligns it according
// to the specified alignment, and writes it to the provided writer. If the caption text is empty
// or the spot is invalid, it logs the issue and returns without printing. The function handles
// wrapping errors by falling back to splitting on newlines or using the original text.
func (t *Table) printTopBottomCaption(w io.Writer, actualTableWidth int) {
	t.logger.Debugf("[printCaption Entry] Text=%q, Spot=%v (type %T), Align=%q, UserWidth=%d, ActualTableWidth=%d",
		t.caption.Text, t.caption.Spot, t.caption.Spot, t.caption.Align, t.caption.Width, actualTableWidth)

	currentCaptionSpot := t.caption.Spot
	isValidSpot := currentCaptionSpot >= tw.SpotTopLeft && currentCaptionSpot <= tw.SpotBottomRight
	if t.caption.Text == "" || !isValidSpot {
		t.logger.Debugf("[printCaption] Aborting: Text empty OR Spot invalid...")
		return
	}

	var captionWrapWidth int
	if t.caption.Width > 0 {
		captionWrapWidth = t.caption.Width
		t.logger.Debugf("[printCaption] Using user-defined caption.Width %d for wrapping.", captionWrapWidth)
	} else if actualTableWidth <= 4 {
		captionWrapWidth = twwidth.Width(t.caption.Text)
		t.logger.Debugf("[printCaption] Empty table, no user caption.Width: Using natural caption width %d.", captionWrapWidth)
	} else {
		captionWrapWidth = actualTableWidth
		t.logger.Debugf("[printCaption] Non-empty table, no user caption.Width: Using actualTableWidth %d for wrapping.", captionWrapWidth)
	}

	if captionWrapWidth <= 0 {
		captionWrapWidth = 10
		t.logger.Warnf("[printCaption] captionWrapWidth was %d (<=0). Setting to minimum %d.", captionWrapWidth, 10)
	}
	t.logger.Debugf("[printCaption] Final captionWrapWidth to be used by twwarp: %d", captionWrapWidth)

	wrappedCaptionLines, count := twwarp.WrapString(t.caption.Text, captionWrapWidth)
	if count == 0 {
		t.logger.Errorf("[printCaption] Error from twwarp.WrapString (width %d): %v. Text: %q", captionWrapWidth, count, t.caption.Text)
		if strings.Contains(t.caption.Text, "\n") {
			wrappedCaptionLines = strings.Split(t.caption.Text, "\n")
		} else {
			wrappedCaptionLines = []string{t.caption.Text}
		}
		t.logger.Debugf("[printCaption] Fallback: using %d lines from original text.", len(wrappedCaptionLines))
	}

	if len(wrappedCaptionLines) == 0 && t.caption.Text != "" {
		t.logger.Warn("[printCaption] Wrapping resulted in zero lines for non-empty text. Using fallback.")
		if strings.Contains(t.caption.Text, "\n") {
			wrappedCaptionLines = strings.Split(t.caption.Text, "\n")
		} else {
			wrappedCaptionLines = []string{t.caption.Text}
		}
	} else if t.caption.Text != "" {
		t.logger.Debugf("[printCaption] Wrapped caption into %d lines: %v", len(wrappedCaptionLines), wrappedCaptionLines)
	}

	paddingTargetWidth := actualTableWidth
	if t.caption.Width > 0 {
		paddingTargetWidth = t.caption.Width
	} else if actualTableWidth <= 4 {
		paddingTargetWidth = captionWrapWidth
	}
	t.logger.Debugf("[printCaption] Final paddingTargetWidth for tw.Pad: %d", paddingTargetWidth)

	for i, line := range wrappedCaptionLines {
		align := t.caption.Align
		if align == "" || align == tw.AlignDefault || align == tw.AlignNone {
			switch t.caption.Spot {
			case tw.SpotTopLeft, tw.SpotBottomLeft:
				align = tw.AlignLeft
			case tw.SpotTopRight, tw.SpotBottomRight:
				align = tw.AlignRight
			default:
				align = tw.AlignCenter
			}
			t.logger.Debugf("[printCaption] Line %d: Alignment defaulted to %s based on Spot %v", i, align, t.caption.Spot)
		}
		paddedLine := tw.Pad(line, " ", paddingTargetWidth, align)
		t.logger.Debugf("[printCaption] Printing line %d: InputLine=%q, Align=%s, PaddingTargetWidth=%d, PaddedLine=%q",
			i, line, align, paddingTargetWidth, paddedLine)
		w.Write([]byte(paddedLine))
		w.Write([]byte(tw.NewLine))
	}

	t.logger.Debugf("[printCaption] Finished printing all caption lines.")
}

// prepareContent processes cell content with formatting and wrapping.
// Parameters include cells to process and config for formatting rules.
// Returns a slice of string slices representing processed lines.
func (t *Table) prepareContent(cells []string, config tw.CellConfig) [][]string {
	isStreaming := t.config.Stream.Enable && t.hasPrinted
	t.logger.Debugf("prepareContent: Processing cells=%v (streaming: %v)", cells, isStreaming)
	initialInputCellCount := len(cells)
	result := make([][]string, 0)

	effectiveNumCols := initialInputCellCount
	if isStreaming {
		if t.streamNumCols > 0 {
			effectiveNumCols = t.streamNumCols
			t.logger.Debugf("prepareContent: Streaming mode, using fixed streamNumCols: %d", effectiveNumCols)
			if len(cells) != effectiveNumCols {
				t.logger.Warnf("prepareContent: Streaming mode, input cell count (%d) does not match streamNumCols (%d). Input cells will be padded/truncated.", len(cells), effectiveNumCols)
				if len(cells) < effectiveNumCols {
					paddedCells := make([]string, effectiveNumCols)
					copy(paddedCells, cells)
					for i := len(cells); i < effectiveNumCols; i++ {
						paddedCells[i] = tw.Empty
					}
					cells = paddedCells
				} else if len(cells) > effectiveNumCols {
					cells = cells[:effectiveNumCols]
				}
			}
		} else {
			t.logger.Warnf("prepareContent: Streaming mode enabled but streamNumCols is 0. Using input cell count %d. Stream widths may not be available.", effectiveNumCols)
		}
	}

	if t.config.MaxWidth > 0 && !t.config.Widths.Constrained() {
		if effectiveNumCols > 0 {
			derivedSectionGlobalMaxWidth := int(math.Floor(float64(t.config.MaxWidth) / float64(effectiveNumCols)))
			config.ColMaxWidths.Global = derivedSectionGlobalMaxWidth
			t.logger.Debugf("prepareContent: Table MaxWidth %d active and t.config.Widths not constrained. "+
				"Derived section ColMaxWidths.Global: %d for %d columns. This will be used by calculateContentMaxWidth if no higher priority constraints exist.",
				t.config.MaxWidth, config.ColMaxWidths.Global, effectiveNumCols)
		}
	}

	for i := 0; i < effectiveNumCols; i++ {
		cellContent := ""
		if i < len(cells) {
			cellContent = cells[i]
		} else {
			cellContent = tw.Empty
		}

		cellContent = t.Trimmer(cellContent)

		if strings.Contains(cellContent, twwidth.TabString.String()) {
			// Get the detected width from the singleton
			width := twwidth.TabWidth()
			spaces := strings.Repeat(tw.Space, width)
			cellContent = strings.ReplaceAll(cellContent, twwidth.TabString.String(), spaces)
		}

		colPad := config.Padding.Global
		if i < len(config.Padding.PerColumn) && config.Padding.PerColumn[i].Paddable() {
			colPad = config.Padding.PerColumn[i]
		}

		padLeftWidth := twwidth.Width(colPad.Left)
		padRightWidth := twwidth.Width(colPad.Right)

		effectiveContentMaxWidth := t.calculateContentMaxWidth(i, config, padLeftWidth, padRightWidth, isStreaming)

		if config.Formatting.AutoFormat.Enabled() {
			cellContent = tw.Title(strings.Join(tw.SplitCamelCase(cellContent), tw.Space))
		}

		lines := strings.Split(cellContent, "\n")
		finalLinesForCell := make([]string, 0)
		for _, line := range lines {
			if effectiveContentMaxWidth > 0 {
				switch config.Formatting.AutoWrap {
				case tw.WrapNormal:
					var wrapped []string
					if t.config.Behavior.TrimSpace.Enabled() && t.config.Behavior.TrimTab.Enabled() {
						wrapped, _ = twwarp.WrapString(line, effectiveContentMaxWidth)
					} else {
						wrapped, _ = twwarp.WrapStringWithSpaces(line, effectiveContentMaxWidth)
					}
					finalLinesForCell = append(finalLinesForCell, wrapped...)
				case tw.WrapTruncate:
					if twwidth.Width(line) > effectiveContentMaxWidth {
						ellipsisWidth := twwidth.Width(tw.CharEllipsis)
						if effectiveContentMaxWidth >= ellipsisWidth {
							finalLinesForCell = append(finalLinesForCell, twwidth.Truncate(line, effectiveContentMaxWidth-ellipsisWidth, tw.CharEllipsis))
						} else {
							finalLinesForCell = append(finalLinesForCell, twwidth.Truncate(line, effectiveContentMaxWidth, ""))
						}
					} else {
						finalLinesForCell = append(finalLinesForCell, line)
					}
				case tw.WrapBreak:
					wrapped := make([]string, 0)
					currentLine := line
					breakCharWidth := twwidth.Width(tw.CharBreak)
					for twwidth.Width(currentLine) > effectiveContentMaxWidth {
						targetWidth := max(effectiveContentMaxWidth-breakCharWidth, 0)
						breakPoint := tw.BreakPoint(currentLine, targetWidth)
						runes := []rune(currentLine)
						if breakPoint <= 0 || breakPoint > len(runes) {
							t.logger.Warnf("prepareContent: WrapBreak - Invalid BreakPoint %d for line '%s' at width %d. Attempting manual break.", breakPoint, currentLine, targetWidth)
							actualBreakRuneCount := 0
							tempWidth := 0
							for charIdx, r := range runes {
								runeStr := string(r)
								rw := twwidth.Width(runeStr)
								if tempWidth+rw > targetWidth && charIdx > 0 {
									break
								}
								tempWidth += rw
								actualBreakRuneCount = charIdx + 1
								if tempWidth >= targetWidth && charIdx == 0 {
									break
								}
							}
							if actualBreakRuneCount == 0 && len(runes) > 0 {
								actualBreakRuneCount = 1
							}
							if actualBreakRuneCount > 0 && actualBreakRuneCount <= len(runes) {
								wrapped = append(wrapped, string(runes[:actualBreakRuneCount])+tw.CharBreak)
								currentLine = string(runes[actualBreakRuneCount:])
							} else {
								t.logger.Warnf("prepareContent: WrapBreak - Cannot break line '%s'. Adding as is.", currentLine)
								wrapped = append(wrapped, currentLine)
								currentLine = ""
								break
							}
						} else {
							wrapped = append(wrapped, string(runes[:breakPoint])+tw.CharBreak)
							currentLine = string(runes[breakPoint:])
						}
					}
					if twwidth.Width(currentLine) > 0 {
						wrapped = append(wrapped, currentLine)
					}
					if len(wrapped) == 0 && twwidth.Width(line) > 0 && len(finalLinesForCell) == 0 {
						finalLinesForCell = append(finalLinesForCell, line)
					} else {
						finalLinesForCell = append(finalLinesForCell, wrapped...)
					}
				default:
					finalLinesForCell = append(finalLinesForCell, line)
				}
			} else {
				finalLinesForCell = append(finalLinesForCell, line)
			}
		}

		for len(result) < len(finalLinesForCell) {
			newRow := make([]string, effectiveNumCols)
			for j := range newRow {
				newRow[j] = tw.Empty
			}
			result = append(result, newRow)
		}

		for j := 0; j < len(result); j++ {
			cellLineContent := tw.Empty
			if j < len(finalLinesForCell) {
				cellLineContent = finalLinesForCell[j]
			}
			if i < len(result[j]) {
				result[j][i] = cellLineContent
			} else {
				t.logger.Warnf("prepareContent: Column index %d out of bounds (%d) during result matrix population. EffectiveNumCols: %d. This indicates a logic error.",
					i, len(result[j]), effectiveNumCols)
			}
		}
	}

	t.logger.Debugf("prepareContent: Content prepared, result %d lines.", len(result))
	return result
}

// prepareContexts initializes rendering and merge contexts.
// No parameters are required.
// Returns renderContext, mergeContext, and an error if initialization fails.
func (t *Table) prepareContexts() (*renderContext, *mergeContext, error) {
	numOriginalCols := t.maxColumns()
	t.logger.Debugf("prepareContexts: Original number of columns: %d", numOriginalCols)

	ctx := &renderContext{
		table:    t,
		renderer: t.renderer,
		cfg:      t.renderer.Config(),
		numCols:  numOriginalCols,
		widths: map[tw.Position]tw.Mapper[int, int]{
			tw.Header: tw.NewMapper[int, int](),
			tw.Row:    tw.NewMapper[int, int](),
			tw.Footer: tw.NewMapper[int, int](),
		},
		logger: t.logger,
	}

	// Process raw rows into visual, multi-line rows
	processedRowLines := make([][][]string, len(t.rows))
	for i, rawRow := range t.rows {
		processedRowLines[i] = t.prepareContent(rawRow, t.config.Row)
	}
	ctx.rowLines = processedRowLines

	isEmpty, visibleCount := t.getEmptyColumnInfo(ctx.rowLines, numOriginalCols)
	ctx.emptyColumns = isEmpty
	ctx.visibleColCount = visibleCount

	mctx := &mergeContext{
		headerMerges: make(map[int]tw.MergeState),
		rowMerges:    make([]map[int]tw.MergeState, len(ctx.rowLines)),
		footerMerges: make(map[int]tw.MergeState),
		horzMerges:   make(map[tw.Position]map[int]bool),
	}
	for i := range mctx.rowMerges {
		mctx.rowMerges[i] = make(map[int]tw.MergeState)
	}

	ctx.headerLines = t.headers
	ctx.footerLines = t.footers

	if err := t.calculateAndNormalizeWidths(ctx); err != nil {
		t.logger.Debugf("Error during initial width calculation: %v", err)
		return nil, nil, err
	}
	t.logger.Debugf("Initial normalized widths (before hiding): H=%v, R=%v, F=%v",
		ctx.widths[tw.Header], ctx.widths[tw.Row], ctx.widths[tw.Footer])

	preparedHeaderLines, headerMerges, _ := t.prepareWithMerges(ctx.headerLines, t.config.Header, tw.Header)
	ctx.headerLines = preparedHeaderLines
	mctx.headerMerges = headerMerges

	// Re-process row lines for merges now that widths are known
	processedRowLinesWithMerges := make([][][]string, len(ctx.rowLines))
	for i, row := range ctx.rowLines {
		if mctx.rowMerges[i] == nil {
			mctx.rowMerges[i] = make(map[int]tw.MergeState)
		}
		processedRowLinesWithMerges[i], mctx.rowMerges[i], _ = t.prepareWithMerges(row, t.config.Row, tw.Row)
	}
	ctx.rowLines = processedRowLinesWithMerges

	t.applyHorizontalMerges(tw.Header, ctx, mctx.headerMerges)

	mergeMode := t.config.Row.Merging.Mode
	if mergeMode == 0 {
		mergeMode = t.config.Row.Formatting.MergeMode
	}

	// Now check against the effective mode
	if mergeMode&tw.MergeVertical != 0 {
		t.applyVerticalMerges(ctx, mctx)
	}
	if mergeMode&tw.MergeHierarchical != 0 {
		t.applyHierarchicalMerges(ctx, mctx)
	}

	t.prepareFooter(ctx, mctx)
	t.logger.Debugf("Footer prepared. Widths before hiding: H=%v, R=%v, F=%v",
		ctx.widths[tw.Header], ctx.widths[tw.Row], ctx.widths[tw.Footer])

	if t.config.Behavior.AutoHide.Enabled() {
		t.logger.Debugf("Applying AutoHide: Adjusting widths for empty columns.")
		if ctx.emptyColumns == nil {
			t.logger.Debugf("Warning: ctx.emptyColumns is nil during width adjustment.")
		} else if len(ctx.emptyColumns) != ctx.numCols {
			t.logger.Debugf("Warning: Length mismatch between emptyColumns (%d) and numCols (%d). Skipping adjustment.", len(ctx.emptyColumns), ctx.numCols)
		} else {
			for colIdx := 0; colIdx < ctx.numCols; colIdx++ {
				if ctx.emptyColumns[colIdx] {
					t.logger.Debugf("AutoHide: Hiding column %d by setting width to 0.", colIdx)
					ctx.widths[tw.Header].Set(colIdx, 0)
					ctx.widths[tw.Row].Set(colIdx, 0)
					ctx.widths[tw.Footer].Set(colIdx, 0)
				}
			}
			t.logger.Debugf("Widths after AutoHide adjustment: H=%v, R=%v, F=%v",
				ctx.widths[tw.Header], ctx.widths[tw.Row], ctx.widths[tw.Footer])
		}
	} else {
		t.logger.Debugf("AutoHide is disabled, skipping width adjustment.")
	}
	t.logger.Debugf("prepareContexts completed all stages.")
	return ctx, mctx, nil
}

// prepareFooter processes footer content and applies merges.
// Parameters ctx and mctx hold rendering and merge state.
// No return value.
func (t *Table) prepareFooter(ctx *renderContext, mctx *mergeContext) {
	if len(t.footers) == 0 {
		ctx.logger.Debugf("Skipping footer preparation - no footer data")
		if ctx.widths[tw.Footer] == nil {
			ctx.widths[tw.Footer] = tw.NewMapper[int, int]()
		}
		numCols := ctx.numCols
		for i := 0; i < numCols; i++ {
			ctx.widths[tw.Footer].Set(i, ctx.widths[tw.Row].Get(i))
		}
		t.logger.Debug("Initialized empty footer widths based on row widths: %v", ctx.widths[tw.Footer])
		ctx.footerPrepared = true
		return
	}

	t.logger.Debugf("Preparing footer with merge mode: %d", t.config.Footer.Formatting.MergeMode)
	preparedLines, mergeStates, _ := t.prepareWithMerges(t.footers, t.config.Footer, tw.Footer)
	t.footers = preparedLines
	mctx.footerMerges = mergeStates
	ctx.footerLines = t.footers
	t.logger.Debugf("Base footer widths (normalized from rows/header): %v", ctx.widths[tw.Footer])
	t.applyHorizontalMerges(tw.Footer, ctx, mctx.footerMerges)
	ctx.footerPrepared = true
	t.logger.Debugf("Footer preparation completed. Final footer widths: %v", ctx.widths[tw.Footer])
}

// prepareWithMerges processes content and detects horizontal merges.
// Parameters include content, config, and position (Header, Row, Footer).
// Returns processed lines, merge states, and horizontal merge map.
func (t *Table) prepareWithMerges(content [][]string, config tw.CellConfig, position tw.Position) ([][]string, map[int]tw.MergeState, map[int]bool) {
	t.logger.Debugf("PrepareWithMerges START: position=%s, mergeMode=%d", position, config.Formatting.MergeMode)
	if len(content) == 0 {
		t.logger.Debugf("PrepareWithMerges END: No content.")
		return content, nil, nil
	}

	numCols := 0
	if len(content) > 0 && len(content[0]) > 0 { // Assumes content[0] exists and has items
		numCols = len(content[0])
	} else { // Fallback if first line is empty or content is empty
		for _, line := range content { // Find max columns from any line
			if len(line) > numCols {
				numCols = len(line)
			}
		}
		if numCols == 0 { // If still 0, try table-wide max (batch mode context)
			numCols = t.maxColumns()
		}
	}

	if numCols == 0 {
		t.logger.Debugf("PrepareWithMerges END: numCols is zero.")
		return content, nil, nil
	}

	horzMergeMap := make(map[int]bool)      // Tracks if a column is part of any horizontal merge for this logical row
	mergeMap := make(map[int]tw.MergeState) // Final merge states for this logical row

	// Ensure all lines in 'content' are padded to numCols for consistent processing
	// This result is what will be modified and returned.
	result := make([][]string, len(content))
	for i := range content {
		result[i] = padLine(content[i], numCols)
	}

	if config.Formatting.MergeMode&tw.MergeHorizontal != 0 {
		t.logger.Debugf("Checking for horizontal merges (logical cell comparison) for %d visual lines, %d columns", len(content), numCols)

		// Special handling for footer lead merge (often for "TOTAL" spanning empty cells)
		// This logic only applies if it's a footer and typically to the first (often only) visual line.
		if position == tw.Footer && len(content) > 0 {
			lineIdx := 0                                       // Assume footer lead merge applies to the first visual line primarily
			originalLine := padLine(content[lineIdx], numCols) // Use original content for decision
			currentLineResult := result[lineIdx]               // Modify the result line

			firstContentIdx := -1
			var firstContent string
			for c := 0; c < numCols; c++ {
				if c >= len(originalLine) {
					break
				}
				trimmedVal := t.Trimmer(originalLine[c])

				if trimmedVal != "" && trimmedVal != "-" { // "-" is often a placeholder not to merge over
					firstContentIdx = c
					firstContent = originalLine[c] // Store the raw content for placement
					break
				} else if trimmedVal == "-" { // Stop if we hit a hard non-mergeable placeholder
					break
				}
			}

			if firstContentIdx > 0 { // If content starts after the first column
				span := firstContentIdx + 1 // Merge from col 0 up to and including firstContentIdx
				startCol := 0

				allEmptyBefore := true
				for c := 0; c < firstContentIdx; c++ {
					originalLine[c] = t.Trimmer(originalLine[c])
					if c >= len(originalLine) || originalLine[c] != "" {
						allEmptyBefore = false
						break
					}
				}

				if allEmptyBefore {
					t.logger.Debugf("Footer lead-merge applied line %d: content '%s' from col %d moved to col %d, span %d",
						lineIdx, firstContent, firstContentIdx, startCol, span)

					if startCol < len(currentLineResult) {
						currentLineResult[startCol] = firstContent // Place the original content
					}
					for k := startCol + 1; k < startCol+span; k++ { // Clear out other cells in the span
						if k < len(currentLineResult) {
							currentLineResult[k] = tw.Empty
						}
					}

					// Update mergeMap for all visual lines of this logical row
					for visualLine := 0; visualLine < len(result); visualLine++ {
						// Only apply the data move to the line where it was detected,
						// but the merge state should apply to the logical cell (all its visual lines).
						if visualLine != lineIdx { // For other visual lines, just clear the cells in the span
							if startCol < len(result[visualLine]) {
								result[visualLine][startCol] = tw.Empty // Typically empty for other lines in a lead merge
							}
							for k := startCol + 1; k < startCol+span; k++ {
								if k < len(result[visualLine]) {
									result[visualLine][k] = tw.Empty
								}
							}
						}
					}

					// Set merge state for the starting column
					startState := mergeMap[startCol]
					startState.Horizontal = tw.MergeStateOption{Present: true, Span: span, Start: true, End: (span == 1)}
					mergeMap[startCol] = startState
					horzMergeMap[startCol] = true // Mark this column as processed by a merge

					// Set merge state for subsequent columns in the span
					for k := startCol + 1; k < startCol+span; k++ {
						colState := mergeMap[k]
						colState.Horizontal = tw.MergeStateOption{Present: true, Span: span, Start: false, End: k == startCol+span-1}
						mergeMap[k] = colState
						horzMergeMap[k] = true // Mark as processed
					}
				}
			}
		}

		// Standard horizontal merge logic based on full logical cell content
		col := 0
		for col < numCols {
			if horzMergeMap[col] { // If already part of a footer lead-merge, skip
				col++
				continue
			}

			// Get full content of logical cell 'col'
			var currentLogicalCellContentBuilder strings.Builder
			for lineIdx := 0; lineIdx < len(content); lineIdx++ {
				if col < len(content[lineIdx]) {
					currentLogicalCellContentBuilder.WriteString(content[lineIdx][col])
				}
			}

			currentLogicalCellTrimmed := t.Trimmer(currentLogicalCellContentBuilder.String())
			if currentLogicalCellTrimmed == "" || currentLogicalCellTrimmed == "-" {
				col++
				continue
			}

			span := 1
			for nextCol := col + 1; nextCol < numCols; nextCol++ {
				if horzMergeMap[nextCol] { // Don't merge into an already merged (e.g. footer lead) column
					break
				}
				var nextLogicalCellContentBuilder strings.Builder
				for lineIdx := 0; lineIdx < len(content); lineIdx++ {
					if nextCol < len(content[lineIdx]) {
						nextLogicalCellContentBuilder.WriteString(content[lineIdx][nextCol])
					}
				}

				nextLogicalCellTrimmed := t.Trimmer(nextLogicalCellContentBuilder.String())
				if currentLogicalCellTrimmed == nextLogicalCellTrimmed && nextLogicalCellTrimmed != "-" {
					span++
				} else {
					break
				}
			}

			if span > 1 {
				t.logger.Debugf("Standard horizontal merge (logical cell): startCol %d, span %d for content '%s'", col, span, currentLogicalCellTrimmed)
				startState := mergeMap[col]
				startState.Horizontal = tw.MergeStateOption{Present: true, Span: span, Start: true, End: (span == 1)}
				mergeMap[col] = startState
				horzMergeMap[col] = true

				// For all visual lines, clear out the content of the merged-over cells
				for lineIdx := 0; lineIdx < len(result); lineIdx++ {
					for k := col + 1; k < col+span; k++ {
						if k < len(result[lineIdx]) {
							result[lineIdx][k] = tw.Empty
						}
					}
				}

				// Set merge state for subsequent columns in the span
				for k := col + 1; k < col+span; k++ {
					colState := mergeMap[k]
					colState.Horizontal = tw.MergeStateOption{Present: true, Span: span, Start: false, End: k == col+span-1}
					mergeMap[k] = colState
					horzMergeMap[k] = true
				}
				col += span
			} else {
				col++
			}
		}
	}

	t.logger.Debugf("PrepareWithMerges END: position=%s, lines=%d, mergeMapH: %v", position, len(result), func() map[int]tw.MergeStateOption {
		m := make(map[int]tw.MergeStateOption)
		for k, v := range mergeMap {
			m[k] = v.Horizontal
		}
		return m
	}())
	return result, mergeMap, horzMergeMap
}

// render generates the table output using the configured renderer.
// No parameters are required.
// Returns an error if rendering fails in any section.
func (t *Table) render() error {
	t.ensureInitialized()

	// Save the original writer and schedule its restoration upon function exit.
	// This guarantees the table's writer is restored even if errors occur.
	originalWriter := t.writer
	defer func() {
		t.writer = originalWriter
	}()

	// If a counter is active, wrap the writer in a MultiWriter.
	if len(t.counters) > 0 {
		// The slice must be of type io.Writer.
		// Start it with the original destination writer.
		allWriters := []io.Writer{originalWriter}

		// Append each counter to the slice of writers.
		for _, c := range t.counters {
			allWriters = append(allWriters, c)
		}

		// Create a MultiWriter that broadcasts to the original writer AND all counters.
		t.writer = io.MultiWriter(allWriters...)
	}

	if t.config.Stream.Enable {
		t.logger.Warn("Render() called in streaming mode. Use Start/Append/Close methods instead.")
		return errors.New("render called in streaming mode; use Start/Append/Close")
	}

	// Calculate and cache the column count for this specific batch render pass.
	t.batchRenderNumCols = t.maxColumns()
	t.isBatchRenderNumColsSet = true
	defer func() {
		t.isBatchRenderNumColsSet = false
		t.logger.Debugf("Render(): Cleared isBatchRenderNumColsSet to false (batchRenderNumCols was %d).", t.batchRenderNumCols)
	}()

	hasCaption := t.caption.Text != "" && t.caption.Spot != tw.SpotNone
	isTopOrBottomCaption := hasCaption &&
		(t.caption.Spot >= tw.SpotTopLeft && t.caption.Spot <= tw.SpotBottomRight)

	var tableStringBuffer *strings.Builder
	targetWriter := t.writer // Can be the original writer or the MultiWriter.

	// If a caption is present, the main table content must be rendered to an
	// in-memory buffer first to calculate its final width.
	if isTopOrBottomCaption {
		tableStringBuffer = &strings.Builder{}
		targetWriter = tableStringBuffer
		t.logger.Debugf("Top/Bottom caption detected. Rendering table core to buffer first.")
	} else {
		t.logger.Debugf("No caption detected. Rendering table core directly to writer.")
	}

	// Point the table's writer to the target (either the final destination or the buffer).
	t.writer = targetWriter
	ctx, mctx, err := t.prepareContexts()
	if err != nil {
		t.logger.Errorf("prepareContexts failed: %v", err)
		return errors.Newf("failed to prepare table contexts").Wrap(err)
	}

	if err := ctx.renderer.Start(t.writer); err != nil {
		t.logger.Errorf("Renderer Start() error: %v", err)
		return errors.Newf("renderer start failed").Wrap(err)
	}

	renderError := false
	var firstRenderErr error
	renderFuncs := []func(*renderContext, *mergeContext) error{
		t.renderHeader,
		t.renderRow,
		t.renderFooter,
	}
	for i, renderFn := range renderFuncs {
		sectionName := []string{"Header", "Row", "Footer"}[i]
		if renderErr := renderFn(ctx, mctx); renderErr != nil {
			t.logger.Errorf("Renderer section error (%s): %v", sectionName, renderErr)
			if !renderError {
				firstRenderErr = errors.Newf("failed to render %s section", sectionName).Wrap(renderErr)
			}
			renderError = true
			break
		}
	}

	if closeErr := ctx.renderer.Close(); closeErr != nil {
		t.logger.Errorf("Renderer Close() error: %v", closeErr)
		if !renderError {
			firstRenderErr = errors.Newf("renderer close failed").Wrap(closeErr)
		}
		renderError = true
	}

	// Restore the writer to the original for the caption-handling logic.
	// This is necessary because the caption must be written to the final
	// destination, not the temporary buffer used for the table body.
	t.writer = originalWriter

	if renderError {
		return firstRenderErr
	}

	// Caption Handling & Final Output
	if isTopOrBottomCaption {
		renderedTableContent := tableStringBuffer.String()
		t.logger.Debugf("[Render] Table core buffer length: %d", len(renderedTableContent))

		// Handle edge case where table is empty but should have borders.
		shouldHaveBorders := t.renderer != nil && (t.renderer.Config().Borders.Top.Enabled() || t.renderer.Config().Borders.Bottom.Enabled())
		if len(renderedTableContent) == 0 && shouldHaveBorders {
			var sb strings.Builder
			if t.renderer.Config().Borders.Top.Enabled() {
				sb.WriteString("+--+")
				sb.WriteString(t.newLine)
			}
			if t.renderer.Config().Borders.Bottom.Enabled() {
				sb.WriteString("+--+")
			}
			renderedTableContent = sb.String()
			t.logger.Warnf("[Render] Table buffer was empty despite enabled borders. Manually generated minimal output: %q", renderedTableContent)
		}

		actualTableWidth := 0
		trimmedBuffer := strings.TrimRight(renderedTableContent, "\r\n \t")
		for _, line := range strings.Split(trimmedBuffer, "\n") {
			w := twwidth.Width(line)
			if w > actualTableWidth {
				actualTableWidth = w
			}
		}
		t.logger.Debugf("[Render] Calculated actual table width: %d (from content: %q)", actualTableWidth, renderedTableContent)

		isTopCaption := t.caption.Spot >= tw.SpotTopLeft && t.caption.Spot <= tw.SpotTopRight

		if isTopCaption {
			t.logger.Debugf("[Render] Printing Top Caption.")
			t.printTopBottomCaption(t.writer, actualTableWidth)
		}

		if len(renderedTableContent) > 0 {
			t.logger.Debugf("[Render] Printing table content (length %d) to final writer.", len(renderedTableContent))
			t.writer.Write([]byte(renderedTableContent))
			if !isTopCaption && t.caption.Text != "" && !strings.HasSuffix(renderedTableContent, t.newLine) {
				t.writer.Write([]byte(tw.NewLine))
				t.logger.Debugf("[Render] Added trailing newline after table content before bottom caption.")
			}
		} else {
			t.logger.Debugf("[Render] No table content (original buffer or generated) to print.")
		}

		if !isTopCaption {
			t.logger.Debugf("[Render] Calling printTopBottomCaption for Bottom Caption. Width: %d", actualTableWidth)
			t.printTopBottomCaption(t.writer, actualTableWidth)
			t.logger.Debugf("[Render] Returned from printTopBottomCaption for Bottom Caption.")
		}
	}

	t.hasPrinted = true
	t.logger.Info("Render() completed.")
	return nil
}

// renderFooter renders the table's footer section with borders and padding.
// Parameters ctx and mctx hold rendering and merge state.
// Returns an error if rendering fails.
func (t *Table) renderFooter(ctx *renderContext, mctx *mergeContext) error {
	if !ctx.footerPrepared {
		t.prepareFooter(ctx, mctx)
	}

	f := ctx.renderer
	cfg := ctx.cfg

	hasContent := len(ctx.footerLines) > 0
	hasTopPadding := t.config.Footer.Padding.Global.Top != tw.Empty
	hasBottomPaddingConfig := t.config.Footer.Padding.Global.Bottom != tw.Empty || t.hasPerColumnBottomPadding()
	hasAnyFooterElement := hasContent || hasTopPadding || hasBottomPaddingConfig

	if !hasAnyFooterElement {
		hasContentAbove := len(ctx.rowLines) > 0 || len(ctx.headerLines) > 0
		if hasContentAbove && cfg.Borders.Bottom.Enabled() && cfg.Settings.Lines.ShowBottom.Enabled() {
			ctx.logger.Debugf("Footer is empty, rendering table bottom border based on last row/header")
			var lastLineAboveCtx *helperContext
			var lastLineAligns map[int]tw.Align
			var lastLinePadding map[int]tw.Padding

			if len(ctx.rowLines) > 0 {
				lastRowIdx := len(ctx.rowLines) - 1
				lastRowLineIdx := -1
				var lastRowLine []string
				if lastRowIdx >= 0 && len(ctx.rowLines[lastRowIdx]) > 0 {
					lastRowLineIdx = len(ctx.rowLines[lastRowIdx]) - 1
					lastRowLine = padLine(ctx.rowLines[lastRowIdx][lastRowLineIdx], ctx.numCols)
				} else {
					lastRowLine = make([]string, ctx.numCols)
				}
				lastLineAboveCtx = &helperContext{
					position: tw.Row,
					rowIdx:   lastRowIdx,
					lineIdx:  lastRowLineIdx,
					line:     lastRowLine,
					location: tw.LocationEnd,
				}
				lastLineAligns = t.buildAligns(t.config.Row)
				lastLinePadding = t.buildPadding(t.config.Row.Padding)
			} else {
				lastHeaderLineIdx := -1
				var lastHeaderLine []string
				if len(ctx.headerLines) > 0 {
					lastHeaderLineIdx = len(ctx.headerLines) - 1
					lastHeaderLine = padLine(ctx.headerLines[lastHeaderLineIdx], ctx.numCols)
				} else {
					lastHeaderLine = make([]string, ctx.numCols)
				}
				lastLineAboveCtx = &helperContext{
					position: tw.Header,
					rowIdx:   0,
					lineIdx:  lastHeaderLineIdx,
					line:     lastHeaderLine,
					location: tw.LocationEnd,
				}
				lastLineAligns = t.buildAligns(t.config.Header)
				lastLinePadding = t.buildPadding(t.config.Header.Padding)
			}

			resp := t.buildCellContexts(ctx, mctx, lastLineAboveCtx, lastLineAligns, lastLinePadding)
			ctx.logger.Debugf("Bottom border: Using Widths=%v", ctx.widths[tw.Row])
			f.Line(tw.Formatting{
				Row: tw.RowContext{
					Widths:       ctx.widths[tw.Row],
					Current:      resp.cells,
					Previous:     resp.prevCells,
					Position:     lastLineAboveCtx.position,
					Location:     tw.LocationEnd,
					ColMaxWidths: t.getColMaxWidths(tw.Footer),
				},
				Level:    tw.LevelFooter,
				IsSubRow: false,
			})
		} else {
			ctx.logger.Debugf("Footer is empty and no content above or borders disabled, skipping footer render")
		}
		return nil
	}

	ctx.logger.Debugf("Rendering footer section (has elements)")
	hasContentAbove := len(ctx.rowLines) > 0 || len(ctx.headerLines) > 0
	colAligns := t.buildAligns(t.config.Footer)
	colPadding := t.buildPadding(t.config.Footer.Padding)
	hctx := &helperContext{position: tw.Footer}
	// Declare paddingLineContentForContext with a default value
	paddingLineContentForContext := make([]string, ctx.numCols)

	if hasContentAbove && cfg.Settings.Lines.ShowFooterLine.Enabled() && !hasTopPadding && len(ctx.footerLines) > 0 {
		ctx.logger.Debugf("Rendering footer separator line")
		var lastLineAboveCtx *helperContext
		var lastLineAligns map[int]tw.Align
		var lastLinePadding map[int]tw.Padding
		var lastLinePosition tw.Position

		if len(ctx.rowLines) > 0 {
			lastRowIdx := len(ctx.rowLines) - 1
			lastRowLineIdx := -1
			var lastRowLine []string
			if lastRowIdx >= 0 && len(ctx.rowLines[lastRowIdx]) > 0 {
				lastRowLineIdx = len(ctx.rowLines[lastRowIdx]) - 1
				lastRowLine = padLine(ctx.rowLines[lastRowIdx][lastRowLineIdx], ctx.numCols)
			} else {
				lastRowLine = make([]string, ctx.numCols)
			}
			lastLineAboveCtx = &helperContext{
				position: tw.Row,
				rowIdx:   lastRowIdx,
				lineIdx:  lastRowLineIdx,
				line:     lastRowLine,
				location: tw.LocationMiddle,
			}
			lastLineAligns = t.buildAligns(t.config.Row)
			lastLinePadding = t.buildPadding(t.config.Row.Padding)
			lastLinePosition = tw.Row
		} else {
			lastHeaderLineIdx := -1
			var lastHeaderLine []string
			if len(ctx.headerLines) > 0 {
				lastHeaderLineIdx = len(ctx.headerLines) - 1
				lastHeaderLine = padLine(ctx.headerLines[lastHeaderLineIdx], ctx.numCols)
			} else {
				lastHeaderLine = make([]string, ctx.numCols)
			}
			lastLineAboveCtx = &helperContext{
				position: tw.Header,
				rowIdx:   0,
				lineIdx:  lastHeaderLineIdx,
				line:     lastHeaderLine,
				location: tw.LocationMiddle,
			}
			lastLineAligns = t.buildAligns(t.config.Header)
			lastLinePadding = t.buildPadding(t.config.Header.Padding)
			lastLinePosition = tw.Header
		}

		resp := t.buildCellContexts(ctx, mctx, lastLineAboveCtx, lastLineAligns, lastLinePadding)
		var nextCells map[int]tw.CellContext
		if hasContent {
			nextCells = make(map[int]tw.CellContext)
			for j, cellData := range padLine(ctx.footerLines[0], ctx.numCols) {
				mergeState := tw.MergeState{}
				if mctx.footerMerges != nil {
					mergeState = mctx.footerMerges[j]
				}
				nextCells[j] = tw.CellContext{Data: cellData, Merge: mergeState, Width: ctx.widths[tw.Footer].Get(j)}
			}
		}
		ctx.logger.Debugf("Footer separator: Using Widths=%v", ctx.widths[tw.Row])
		f.Line(tw.Formatting{
			Row: tw.RowContext{
				Widths:       ctx.widths[tw.Row],
				Current:      resp.cells,
				Previous:     resp.prevCells,
				Next:         nextCells,
				Position:     lastLinePosition,
				Location:     tw.LocationMiddle,
				ColMaxWidths: t.getColMaxWidths(tw.Footer),
			},
			Level:     tw.LevelFooter,
			IsSubRow:  false,
			HasFooter: true,
		})
	}

	if hasTopPadding {
		hctx.rowIdx = 0
		hctx.lineIdx = -1
		if !hasContentAbove || !cfg.Settings.Lines.ShowFooterLine.Enabled() {
			hctx.location = tw.LocationFirst
		} else {
			hctx.location = tw.LocationMiddle
		}
		hctx.line = t.buildPaddingLineContents(t.config.Footer.Padding.Global.Top, ctx.widths[tw.Footer], ctx.numCols, mctx.footerMerges)
		ctx.logger.Debugf("Calling renderPadding for Footer Top Padding line: %v (loc: %v)", hctx.line, hctx.location)
		if err := t.renderPadding(ctx, mctx, hctx, t.config.Footer.Padding.Global.Top); err != nil {
			return err
		}
	}

	lastRenderedLineIdx := -2
	if hasTopPadding {
		lastRenderedLineIdx = -1
	}
	for i, line := range ctx.footerLines {
		hctx.rowIdx = 0
		hctx.lineIdx = i
		hctx.line = padLine(line, ctx.numCols)
		isFirstContentLine := i == 0
		isLastContentLine := i == len(ctx.footerLines)-1
		if isFirstContentLine && !hasTopPadding && (!hasContentAbove || !cfg.Settings.Lines.ShowFooterLine.Enabled()) {
			hctx.location = tw.LocationFirst
		} else if isLastContentLine && !hasBottomPaddingConfig {
			hctx.location = tw.LocationEnd
		} else {
			hctx.location = tw.LocationMiddle
		}
		ctx.logger.Debugf("Rendering footer content line %d with location %v", i, hctx.location)
		if err := t.renderLine(ctx, mctx, hctx, colAligns, colPadding); err != nil {
			return err
		}
		lastRenderedLineIdx = i
	}

	if hasBottomPaddingConfig {
		paddingLineContentForContext = make([]string, ctx.numCols)
		formattedPaddingCells := make([]string, ctx.numCols)
		representativePadChar := " "
		ctx.logger.Debugf("Constructing Footer Bottom Padding line content strings")
		for j := 0; j < ctx.numCols; j++ {
			colWd := ctx.widths[tw.Footer].Get(j)
			mergeState := tw.MergeState{}
			if mctx.footerMerges != nil {
				if state, ok := mctx.footerMerges[j]; ok {
					mergeState = state
				}
			}
			if mergeState.Horizontal.Present && !mergeState.Horizontal.Start {
				paddingLineContentForContext[j] = ""
				formattedPaddingCells[j] = ""
				continue
			}
			padChar := " "
			if j < len(t.config.Footer.Padding.PerColumn) && t.config.Footer.Padding.PerColumn[j].Bottom != tw.Empty {
				padChar = t.config.Footer.Padding.PerColumn[j].Bottom
			} else if t.config.Footer.Padding.Global.Bottom != tw.Empty {
				padChar = t.config.Footer.Padding.Global.Bottom
			}
			paddingLineContentForContext[j] = padChar
			if j == 0 || representativePadChar == " " {
				representativePadChar = padChar
			}
			padWidth := max(twwidth.Width(padChar), 1)
			repeatCount := 0
			if colWd > 0 && padWidth > 0 {
				repeatCount = colWd / padWidth
			}
			if colWd > 0 && repeatCount < 1 && padChar != " " {
				repeatCount = 1
			}
			if colWd == 0 {
				repeatCount = 0
			}
			rawPaddingContent := strings.Repeat(padChar, repeatCount)
			currentWd := twwidth.Width(rawPaddingContent)
			if currentWd < colWd {
				rawPaddingContent += strings.Repeat(" ", colWd-currentWd)
			}
			if currentWd > colWd && colWd > 0 {
				rawPaddingContent = twwidth.Truncate(rawPaddingContent, colWd)
			}
			if colWd == 0 {
				rawPaddingContent = ""
			}
			formattedPaddingCells[j] = rawPaddingContent
		}
		ctx.logger.Debugf("Manually rendering Footer Bottom Padding line (char like '%s')", representativePadChar)
		var paddingLineOutput strings.Builder
		if cfg.Borders.Left.Enabled() {
			paddingLineOutput.WriteString(cfg.Symbols.Column())
		}
		for colIdx := 0; colIdx < ctx.numCols; {
			if colIdx > 0 && cfg.Settings.Separators.BetweenColumns.Enabled() {
				shouldAddSeparator := true
				if prevMergeState, ok := mctx.footerMerges[colIdx-1]; ok {
					if prevMergeState.Horizontal.Present && !prevMergeState.Horizontal.End {
						shouldAddSeparator = false
					}
				}
				if shouldAddSeparator {
					paddingLineOutput.WriteString(cfg.Symbols.Column())
				}
			}
			if colIdx < len(formattedPaddingCells) {
				paddingLineOutput.WriteString(formattedPaddingCells[colIdx])
			}
			currentMergeState := tw.MergeState{}
			if mctx.footerMerges != nil {
				if state, ok := mctx.footerMerges[colIdx]; ok {
					currentMergeState = state
				}
			}
			if currentMergeState.Horizontal.Present && currentMergeState.Horizontal.Start {
				colIdx += currentMergeState.Horizontal.Span
			} else {
				colIdx++
			}
		}
		if cfg.Borders.Right.Enabled() {
			paddingLineOutput.WriteString(cfg.Symbols.Column())
		}
		paddingLineOutput.WriteString(t.newLine)
		t.writer.Write([]byte(paddingLineOutput.String()))
		ctx.logger.Debugf("Manually rendered Footer Bottom Padding line: %s", strings.TrimSuffix(paddingLineOutput.String(), t.newLine))
		hctx.rowIdx = 0
		hctx.lineIdx = len(ctx.footerLines)
		hctx.line = paddingLineContentForContext
		hctx.location = tw.LocationEnd
		lastRenderedLineIdx = hctx.lineIdx
	}

	if cfg.Borders.Bottom.Enabled() && cfg.Settings.Lines.ShowBottom.Enabled() {
		ctx.logger.Debugf("Rendering final table bottom border")
		if lastRenderedLineIdx == len(ctx.footerLines) {
			hctx.rowIdx = 0
			hctx.lineIdx = lastRenderedLineIdx
			hctx.line = paddingLineContentForContext
			hctx.location = tw.LocationEnd
			ctx.logger.Debugf("Setting border context based on bottom padding line")
		} else if lastRenderedLineIdx >= 0 {
			hctx.rowIdx = 0
			hctx.lineIdx = lastRenderedLineIdx
			hctx.line = padLine(ctx.footerLines[hctx.lineIdx], ctx.numCols)
			hctx.location = tw.LocationEnd
			ctx.logger.Debugf("Setting border context based on last content line idx %d", hctx.lineIdx)
		} else if lastRenderedLineIdx == -1 {
			hctx.rowIdx = 0
			hctx.lineIdx = -1
			hctx.line = paddingLineContentForContext
			hctx.location = tw.LocationEnd
			ctx.logger.Debugf("Setting border context based on top padding line")
		} else {
			hctx.rowIdx = 0
			hctx.lineIdx = -2
			hctx.line = make([]string, ctx.numCols)
			hctx.location = tw.LocationEnd
			ctx.logger.Debugf("Warning: Cannot determine context for bottom border")
		}
		resp := t.buildCellContexts(ctx, mctx, hctx, colAligns, colPadding)
		ctx.logger.Debugf("Bottom border: Using Widths=%v", ctx.widths[tw.Row])
		f.Line(tw.Formatting{
			Row: tw.RowContext{
				Widths:       ctx.widths[tw.Row],
				Current:      resp.cells,
				Previous:     resp.prevCells,
				Position:     tw.Footer,
				Location:     tw.LocationEnd,
				ColMaxWidths: t.getColMaxWidths(tw.Footer),
			},
			Level:    tw.LevelFooter,
			IsSubRow: false,
		})
	}

	return nil
}

// renderHeader renders the table's header section with borders and padding.
// Parameters ctx and mctx hold rendering and merge state.
// Returns an error if rendering fails.
func (t *Table) renderHeader(ctx *renderContext, mctx *mergeContext) error {
	if len(ctx.headerLines) == 0 {
		return nil
	}
	ctx.logger.Debug("Rendering header section")

	f := ctx.renderer
	cfg := ctx.cfg
	colAligns := t.buildAligns(t.config.Header)
	colPadding := t.buildPadding(t.config.Header.Padding)
	hctx := &helperContext{position: tw.Header}

	if cfg.Borders.Top.Enabled() && cfg.Settings.Lines.ShowTop.Enabled() {
		ctx.logger.Debug("Rendering table top border")
		nextCells := make(map[int]tw.CellContext)
		if len(ctx.headerLines) > 0 {
			for j, cell := range ctx.headerLines[0] {
				nextCells[j] = tw.CellContext{Data: cell, Merge: mctx.headerMerges[j]}
			}
		}
		f.Line(tw.Formatting{
			Row: tw.RowContext{
				Widths:   ctx.widths[tw.Header],
				Next:     nextCells,
				Position: tw.Header,
				Location: tw.LocationFirst,
			},
			Level:    tw.LevelHeader,
			IsSubRow: false,
		})
	}

	if t.config.Header.Padding.Global.Top != tw.Empty {
		hctx.location = tw.LocationFirst
		hctx.line = t.buildPaddingLineContents(t.config.Header.Padding.Global.Top, ctx.widths[tw.Header], ctx.numCols, mctx.headerMerges)
		if err := t.renderPadding(ctx, mctx, hctx, t.config.Header.Padding.Global.Top); err != nil {
			return err
		}
	}

	for i, line := range ctx.headerLines {
		hctx.rowIdx = 0
		hctx.lineIdx = i
		hctx.line = padLine(line, ctx.numCols)
		hctx.location = t.determineLocation(i, len(ctx.headerLines), t.config.Header.Padding.Global.Top, t.config.Header.Padding.Global.Bottom)

		if t.config.Header.Callbacks.Global != nil {
			ctx.logger.Debug("Executing global header callback for line %d", i)
			t.config.Header.Callbacks.Global()
		}
		for colIdx, cb := range t.config.Header.Callbacks.PerColumn {
			if colIdx < ctx.numCols && cb != nil {
				ctx.logger.Debug("Executing per-column header callback for line %d, col %d", i, colIdx)
				cb()
			}
		}

		if err := t.renderLine(ctx, mctx, hctx, colAligns, colPadding); err != nil {
			return err
		}
	}

	if t.config.Header.Padding.Global.Bottom != tw.Empty {
		hctx.location = tw.LocationEnd
		hctx.line = t.buildPaddingLineContents(t.config.Header.Padding.Global.Bottom, ctx.widths[tw.Header], ctx.numCols, mctx.headerMerges)
		if err := t.renderPadding(ctx, mctx, hctx, t.config.Header.Padding.Global.Bottom); err != nil {
			return err
		}
	}

	if cfg.Settings.Lines.ShowHeaderLine.Enabled() && (len(ctx.rowLines) > 0 || len(ctx.footerLines) > 0) {
		ctx.logger.Debug("Rendering header separator line")
		resp := t.buildCellContexts(ctx, mctx, hctx, colAligns, colPadding)

		var nextSectionCells map[int]tw.CellContext
		var nextSectionWidths tw.Mapper[int, int]

		if len(ctx.rowLines) > 0 {
			nextSectionWidths = ctx.widths[tw.Row]
			rowColAligns := t.buildAligns(t.config.Row)
			rowColPadding := t.buildPadding(t.config.Row.Padding)
			firstRowHctx := &helperContext{
				position: tw.Row,
				rowIdx:   0,
				lineIdx:  0,
			}
			if len(ctx.rowLines[0]) > 0 {
				firstRowHctx.line = padLine(ctx.rowLines[0][0], ctx.numCols)
			} else {
				firstRowHctx.line = make([]string, ctx.numCols)
			}
			firstRowResp := t.buildCellContexts(ctx, mctx, firstRowHctx, rowColAligns, rowColPadding)
			nextSectionCells = firstRowResp.cells
		} else if len(ctx.footerLines) > 0 {
			nextSectionWidths = ctx.widths[tw.Row]
			footerColAligns := t.buildAligns(t.config.Footer)
			footerColPadding := t.buildPadding(t.config.Footer.Padding)
			firstFooterHctx := &helperContext{
				position: tw.Footer,
				rowIdx:   0,
				lineIdx:  0,
			}
			if len(ctx.footerLines) > 0 {
				firstFooterHctx.line = padLine(ctx.footerLines[0], ctx.numCols)
			} else {
				firstFooterHctx.line = make([]string, ctx.numCols)
			}
			firstFooterResp := t.buildCellContexts(ctx, mctx, firstFooterHctx, footerColAligns, footerColPadding)
			nextSectionCells = firstFooterResp.cells
		} else {
			nextSectionWidths = ctx.widths[tw.Header]
			nextSectionCells = nil
		}

		f.Line(tw.Formatting{
			Row: tw.RowContext{
				Widths:   nextSectionWidths,
				Current:  resp.cells,
				Previous: resp.prevCells,
				Next:     nextSectionCells,
				Position: tw.Header,
				Location: tw.LocationMiddle,
			},
			Level:    tw.LevelBody,
			IsSubRow: false,
		})
	}
	return nil
}

// renderLine renders a single line with callbacks and normalized widths.
// Parameters include ctx, mctx, hctx, aligns, and padding for rendering.
// Returns an error if rendering fails.
func (t *Table) renderLine(ctx *renderContext, mctx *mergeContext, hctx *helperContext, aligns map[int]tw.Align, padding map[int]tw.Padding) error {
	resp := t.buildCellContexts(ctx, mctx, hctx, aligns, padding)
	f := ctx.renderer

	isPaddingLine := false
	sectionConfig := t.config.Row
	switch hctx.position {
	case tw.Header:
		sectionConfig = t.config.Header
		isPaddingLine = (hctx.lineIdx == -1 && sectionConfig.Padding.Global.Top != tw.Empty) ||
			(hctx.lineIdx == len(ctx.headerLines) && sectionConfig.Padding.Global.Bottom != tw.Empty)
	case tw.Footer:
		sectionConfig = t.config.Footer
		isPaddingLine = (hctx.lineIdx == -1 && sectionConfig.Padding.Global.Top != tw.Empty) ||
			(hctx.lineIdx == len(ctx.footerLines) && (sectionConfig.Padding.Global.Bottom != tw.Empty || t.hasPerColumnBottomPadding()))
	case tw.Row:
		if hctx.rowIdx >= 0 && hctx.rowIdx < len(ctx.rowLines) {
			isPaddingLine = (hctx.lineIdx == -1 && sectionConfig.Padding.Global.Top != tw.Empty) ||
				(hctx.lineIdx == len(ctx.rowLines[hctx.rowIdx]) && sectionConfig.Padding.Global.Bottom != tw.Empty)
		}
	}

	sectionWidths := ctx.widths[hctx.position]
	normalizedWidths := ctx.widths[tw.Row]

	formatting := tw.Formatting{
		Row: tw.RowContext{
			Widths:       sectionWidths,
			ColMaxWidths: t.getColMaxWidths(hctx.position),
			Current:      resp.cells,
			Previous:     resp.prevCells,
			Next:         resp.nextCells,
			Position:     hctx.position,
			Location:     hctx.location,
		},
		Level:            t.getLevel(hctx.position),
		IsSubRow:         hctx.lineIdx > 0 || isPaddingLine,
		NormalizedWidths: normalizedWidths,
	}

	if hctx.position == tw.Row {
		formatting.HasFooter = len(ctx.footerLines) > 0
	}

	switch hctx.position {
	case tw.Header:
		f.Header([][]string{hctx.line}, formatting)
	case tw.Row:
		f.Row(hctx.line, formatting)
	case tw.Footer:
		f.Footer([][]string{hctx.line}, formatting)
	}
	return nil
}

// renderPadding renders padding lines for a section.
// Parameters include ctx, mctx, hctx, and padChar for padding content.
// Returns an error if rendering fails.
func (t *Table) renderPadding(ctx *renderContext, mctx *mergeContext, hctx *helperContext, padChar string) error {
	ctx.logger.Debug("Rendering padding line for %s (using char like '%s')", hctx.position, padChar)

	colAligns := t.buildAligns(t.config.Row)
	colPadding := t.buildPadding(t.config.Row.Padding)

	switch hctx.position {
	case tw.Header:
		colAligns = t.buildAligns(t.config.Header)
		colPadding = t.buildPadding(t.config.Header.Padding)
	case tw.Footer:
		colAligns = t.buildAligns(t.config.Footer)
		colPadding = t.buildPadding(t.config.Footer.Padding)
	}

	return t.renderLine(ctx, mctx, hctx, colAligns, colPadding)
}

// renderRow renders the table's row section with borders and padding.
// Parameters ctx and mctx hold rendering and merge state.
// Returns an error if rendering fails.
func (t *Table) renderRow(ctx *renderContext, mctx *mergeContext) error {
	if len(ctx.rowLines) == 0 {
		return nil
	}
	ctx.logger.Debugf("Rendering row section (total rows: %d)", len(ctx.rowLines))

	f := ctx.renderer
	cfg := ctx.cfg
	colAligns := t.buildAligns(t.config.Row)
	colPadding := t.buildPadding(t.config.Row.Padding)
	hctx := &helperContext{position: tw.Row}

	footerIsEmptyOrNonExistent := !t.hasFooterElements()
	if len(ctx.headerLines) == 0 && footerIsEmptyOrNonExistent && cfg.Borders.Top.Enabled() && cfg.Settings.Lines.ShowTop.Enabled() {
		ctx.logger.Debug("Rendering table top border (rows only table)")
		nextCells := make(map[int]tw.CellContext)
		if len(ctx.rowLines) > 0 && len(ctx.rowLines[0]) > 0 && len(mctx.rowMerges) > 0 {
			firstLine := ctx.rowLines[0][0]
			firstMerges := mctx.rowMerges[0]
			for j, cell := range padLine(firstLine, ctx.numCols) {
				mergeState := tw.MergeState{}
				if firstMerges != nil {
					mergeState = firstMerges[j]
				}
				nextCells[j] = tw.CellContext{Data: cell, Merge: mergeState, Width: ctx.widths[tw.Row].Get(j)}
			}
		}
		f.Line(tw.Formatting{
			Row: tw.RowContext{
				Widths:   ctx.widths[tw.Row],
				Next:     nextCells,
				Position: tw.Row,
				Location: tw.LocationFirst,
			},
			Level:    tw.LevelHeader,
			IsSubRow: false,
		})
	}

	for i, lines := range ctx.rowLines {
		rowHasTopPadding := t.config.Row.Padding.Global.Top != tw.Empty
		if rowHasTopPadding {
			hctx.rowIdx = i
			hctx.lineIdx = -1
			if i == 0 && len(ctx.headerLines) == 0 {
				hctx.location = tw.LocationFirst
			} else {
				hctx.location = tw.LocationMiddle
			}
			hctx.line = t.buildPaddingLineContents(t.config.Row.Padding.Global.Top, ctx.widths[tw.Row], ctx.numCols, mctx.rowMerges[i])
			ctx.logger.Debug("Calling renderPadding for Row Top Padding (row %d): %v (loc: %v)", i, hctx.line, hctx.location)
			if err := t.renderPadding(ctx, mctx, hctx, t.config.Row.Padding.Global.Top); err != nil {
				return err
			}
		}

		footerExists := t.hasFooterElements()
		rowHasBottomPadding := t.config.Row.Padding.Global.Bottom != tw.Empty
		isLastRow := i == len(ctx.rowLines)-1

		for j, visualLineData := range lines {
			hctx.rowIdx = i
			hctx.lineIdx = j
			hctx.line = padLine(visualLineData, ctx.numCols)

			if t.config.Behavior.TrimLine.Enabled() {
				if j > 0 {
					visualLineHasActualContent := false
					for kCellIdx, cellContentInVisualLine := range hctx.line {
						if t.Trimmer(cellContentInVisualLine) != "" {
							visualLineHasActualContent = true
							ctx.logger.Debug("Visual line [%d][%d] has content in cell %d: '%s'. Not skipping.", i, j, kCellIdx, cellContentInVisualLine)
							break
						}
					}

					if !visualLineHasActualContent {
						ctx.logger.Debug("Skipping visual line [%d][%d] as it's entirely blank after trimming. Line: %q", i, j, hctx.line)
						continue
					}
				}
			}

			isFirstRow := i == 0
			isLastLineOfRow := j == len(lines)-1

			if isFirstRow && j == 0 && !rowHasTopPadding && len(ctx.headerLines) == 0 {
				hctx.location = tw.LocationFirst
			} else if isLastRow && isLastLineOfRow && !rowHasBottomPadding && !footerExists {
				hctx.location = tw.LocationEnd
			} else {
				hctx.location = tw.LocationMiddle
			}

			ctx.logger.Debugf("Rendering row %d line %d with location %v. Content: %q", i, j, hctx.location, hctx.line)
			if err := t.renderLine(ctx, mctx, hctx, colAligns, colPadding); err != nil {
				return err
			}
		}

		if rowHasBottomPadding {
			hctx.rowIdx = i
			hctx.lineIdx = len(lines)
			if isLastRow && !footerExists {
				hctx.location = tw.LocationEnd
			} else {
				hctx.location = tw.LocationMiddle
			}
			hctx.line = t.buildPaddingLineContents(t.config.Row.Padding.Global.Bottom, ctx.widths[tw.Row], ctx.numCols, mctx.rowMerges[i])
			ctx.logger.Debug("Calling renderPadding for Row Bottom Padding (row %d): %v (loc: %v)", i, hctx.line, hctx.location)
			if err := t.renderPadding(ctx, mctx, hctx, t.config.Row.Padding.Global.Bottom); err != nil {
				return err
			}
		}

		if cfg.Settings.Separators.BetweenRows.Enabled() && !isLastRow {
			ctx.logger.Debug("Rendering between-rows separator after logical row %d", i)
			respCurrent := t.buildCellContexts(ctx, mctx, hctx, colAligns, colPadding)

			var nextCellsForSeparator map[int]tw.CellContext = nil
			nextRowIdx := i + 1
			if nextRowIdx < len(ctx.rowLines) && nextRowIdx < len(mctx.rowMerges) {
				hctxNext := &helperContext{position: tw.Row, rowIdx: nextRowIdx, location: tw.LocationMiddle}
				nextRowActualLines := ctx.rowLines[nextRowIdx]
				nextRowMerges := mctx.rowMerges[nextRowIdx]

				if t.config.Row.Padding.Global.Top != tw.Empty {
					hctxNext.lineIdx = -1
					hctxNext.line = t.buildPaddingLineContents(t.config.Row.Padding.Global.Top, ctx.widths[tw.Row], ctx.numCols, nextRowMerges)
				} else if len(nextRowActualLines) > 0 {
					hctxNext.lineIdx = 0
					hctxNext.line = padLine(nextRowActualLines[0], ctx.numCols)
				} else {
					hctxNext.lineIdx = 0
					hctxNext.line = make([]string, ctx.numCols)
				}
				respNext := t.buildCellContexts(ctx, mctx, hctxNext, colAligns, colPadding)
				nextCellsForSeparator = respNext.cells
			} else {
				ctx.logger.Debug("Separator context: No next logical row for separator after row %d.", i)
			}

			f.Line(tw.Formatting{
				Row: tw.RowContext{
					Widths:       ctx.widths[tw.Row],
					Current:      respCurrent.cells,
					Previous:     respCurrent.prevCells,
					Next:         nextCellsForSeparator,
					Position:     tw.Row,
					Location:     tw.LocationMiddle,
					ColMaxWidths: t.getColMaxWidths(tw.Row),
				},
				Level:     tw.LevelBody,
				IsSubRow:  false,
				HasFooter: footerExists,
			})
		}
	}
	return nil
}
