// Copyright 2014 Oleku Konko All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

// This module is a Table Writer  API for the Go Programming Language.
// The protocols were written in pure Go and works on windows and unix systems

// Create & Generate text based table
package tablewriter

import (
	"fmt"
	"io"
	"regexp"
	"strings"
)

const (
	MAX_ROW_WIDTH = 30
)

const (
	CENTRE = "+"
	ROW    = "-"
	COLUMN = "|"
	SPACE  = " "
)

const (
	ALIGN_DEFAULT = iota
	ALIGN_CENTRE
	ALIGN_RIGHT
	ALIGN_LEFT
)

var (
	decimal = regexp.MustCompile(`^\d*\.?\d*$`)
	percent = regexp.MustCompile(`^\d*\.?\d*$%$`)
)

type Table struct {
	out      io.Writer
	rows     [][]string
	lines    [][][]string
	cs       map[int]int
	rs       map[int]int
	headers  []string
	footers  []string
	autoFmt  bool
	autoWrap bool
	mW       int
	pCenter  string
	pRow     string
	pColumn  string
	tColumn  int
	tRow     int
	align    int
	rowLine  bool
	hdrLine  bool
	border   bool
	colSize  int
}

// Start New Table
// Take io.Writer Directly
func NewWriter(writer io.Writer) *Table {
	t := &Table{
		out:      writer,
		rows:     [][]string{},
		lines:    [][][]string{},
		cs:       make(map[int]int),
		rs:       make(map[int]int),
		headers:  []string{},
		footers:  []string{},
		autoFmt:  true,
		autoWrap: true,
		mW:       MAX_ROW_WIDTH,
		pCenter:  CENTRE,
		pRow:     ROW,
		pColumn:  COLUMN,
		tColumn:  -1,
		tRow:     -1,
		align:    ALIGN_DEFAULT,
		rowLine:  false,
		hdrLine:  true,
		border:   true,
		colSize:  -1}
	return t
}

// Render table output
func (t Table) Render() {
	if t.border {
		t.printLine(true)
	}
	t.printHeading()
	t.printRows()

	if !t.rowLine && t.border {
		t.printLine(true)
	}
	t.printFooter()

}

// Set table header
func (t *Table) SetHeader(keys []string) {
	t.colSize = len(keys)
	for i, v := range keys {
		t.parseDimension(v, i, -1)
		t.headers = append(t.headers, v)
	}
}

// Set table Footer
func (t *Table) SetFooter(keys []string) {
	//t.colSize = len(keys)
	for i, v := range keys {
		t.parseDimension(v, i, -1)
		t.footers = append(t.footers, v)
	}
}

// Turn header autoformatting on/off. Default is on (true).
func (t *Table) SetAutoFormatHeaders(auto bool) {
	t.autoFmt = auto
}

// Turn automatic multiline text adjustment on/off. Default is on (true).
func (t *Table) SetAutoWrapText(auto bool) {
	t.autoWrap = auto
}

// Set the Default column width
func (t *Table) SetColWidth(width int) {
	t.mW = width
}

// Set the Column Separator
func (t *Table) SetColumnSeparator(sep string) {
	t.pColumn = sep
}

// Set the Row Separator
func (t *Table) SetRowSeparator(sep string) {
	t.pRow = sep
}

// Set the center Separator
func (t *Table) SetCenterSeparator(sep string) {
	t.pCenter = sep
}

// Set Table Alignment
func (t *Table) SetAlignment(align int) {
	t.align = align
}

// Set Header Line
// This would enable / disable a line after the header
func (t *Table) SetHeaderLine(line bool) {
	t.hdrLine = line
}

// Set Row Line
// This would enable / disable a line on each row of the table
func (t *Table) SetRowLine(line bool) {
	t.rowLine = line
}

// Set Table Border
// This would enable / disable line around the table
func (t *Table) SetBorder(border bool) {
	t.border = border
}

// Append row to table
func (t *Table) Append(row []string) {
	rowSize := len(t.headers)
	if rowSize > t.colSize {
		t.colSize = rowSize
	}

	n := len(t.lines)
	line := [][]string{}
	for i, v := range row {

		// Detect string  width
		// Detect String height
		// Break strings into words
		out := t.parseDimension(v, i, n)

		// Append broken words
		line = append(line, out)
	}
	t.lines = append(t.lines, line)
}

// Allow Support for Bulk Append
// Eliminates repeated for loops
func (t *Table) AppendBulk(rows [][]string) {
	for _, row := range rows {
		t.Append(row)
	}
}

// Print line based on row width
func (t Table) printLine(nl bool) {
	fmt.Fprint(t.out, t.pCenter)
	for i := 0; i < len(t.cs); i++ {
		v := t.cs[i]
		fmt.Fprintf(t.out, "%s%s%s%s",
			t.pRow,
			strings.Repeat(string(t.pRow), v),
			t.pRow,
			t.pCenter)
	}
	if nl {
		fmt.Fprintln(t.out)
	}
}

// Print heading information
func (t Table) printHeading() {
	// Check if headers is available
	if len(t.headers) < 1 {
		return
	}

	// Check if border is set
	// Replace with space if not set
	fmt.Fprint(t.out, ConditionString(t.border, t.pColumn, SPACE))

	// Identify last column
	end := len(t.cs) - 1

	// Print Heading column
	for i := 0; i <= end; i++ {
		v := t.cs[i]
		h := t.headers[i]
		if t.autoFmt {
			h = Title(h)
		}
		pad := ConditionString((i == end && !t.border), SPACE, t.pColumn)
		fmt.Fprintf(t.out, " %s %s",
			Pad(h, SPACE, v),
			pad)
	}
	// Next line
	fmt.Fprintln(t.out)
	if t.hdrLine {
		t.printLine(true)
	}
}

// Print heading information
func (t Table) printFooter() {
	// Check if headers is available
	if len(t.footers) < 1 {
		return
	}

	// Only print line if border is not set
	if !t.border {
		t.printLine(true)
	}
	// Check if border is set
	// Replace with space if not set
	fmt.Fprint(t.out, ConditionString(t.border, t.pColumn, SPACE))

	// Identify last column
	end := len(t.cs) - 1

	// Print Heading column
	for i := 0; i <= end; i++ {
		v := t.cs[i]
		f := t.footers[i]
		if t.autoFmt {
			f = Title(f)
		}
		pad := ConditionString((i == end && !t.border), SPACE, t.pColumn)

		if len(t.footers[i]) == 0 {
			pad = SPACE
		}
		fmt.Fprintf(t.out, " %s %s",
			Pad(f, SPACE, v),
			pad)
	}
	// Next line
	fmt.Fprintln(t.out)
	//t.printLine(true)

	hasPrinted := false

	for i := 0; i <= end; i++ {
		v := t.cs[i]
		pad := t.pRow
		center := t.pCenter
		length := len(t.footers[i])

		if length > 0 {
			hasPrinted = true
		}

		// Set center to be space if length is 0
		if length == 0 && !t.border {
			center = SPACE
		}

		// Print first junction
		if i == 0 {
			fmt.Fprint(t.out, center)
		}

		// Pad With space of length is 0
		if length == 0 {
			pad = SPACE
		}
		// Ignore left space of it has printed before
		if hasPrinted || t.border {
			pad = t.pRow
			center = t.pCenter
		}

		// Change Center start position
		if center == SPACE {
			if i < end && len(t.footers[i+1]) != 0 {
				center = t.pCenter
			}
		}

		// Print the footer
		fmt.Fprintf(t.out, "%s%s%s%s",
			pad,
			strings.Repeat(string(pad), v),
			pad,
			center)

	}

	fmt.Fprintln(t.out)

}

func (t Table) printRows() {
	for i, lines := range t.lines {
		t.printRow(lines, i)
	}

}

// Print Row Information
// Adjust column alignment based on type

func (t Table) printRow(columns [][]string, colKey int) {
	// Get Maximum Height
	max := t.rs[colKey]
	total := len(columns)

	// TODO Fix uneven col size
	// if total < t.colSize {
	//	for n := t.colSize - total; n < t.colSize ; n++ {
	//		columns = append(columns, []string{SPACE})
	//		t.cs[n] = t.mW
	//	}
	//}

	// Pad Each Height
	// pads := []int{}
	pads := []int{}

	for i, line := range columns {
		length := len(line)
		pad := max - length
		pads = append(pads, pad)
		for n := 0; n < pad; n++ {
			columns[i] = append(columns[i], "  ")
		}
	}
	//fmt.Println(max, "\n")
	for x := 0; x < max; x++ {
		for y := 0; y < total; y++ {

			// Check if border is set
			fmt.Fprint(t.out, ConditionString((!t.border && y == 0), SPACE, t.pColumn))

			fmt.Fprintf(t.out, SPACE)
			str := columns[y][x]

			// This would print alignment
			// Default alignment  would use multiple configuration
			switch t.align {
			case ALIGN_CENTRE: //
				fmt.Fprintf(t.out, "%s", Pad(str, SPACE, t.cs[y]))
			case ALIGN_RIGHT:
				fmt.Fprintf(t.out, "%s", PadLeft(str, SPACE, t.cs[y]))
			case ALIGN_LEFT:
				fmt.Fprintf(t.out, "%s", PadRight(str, SPACE, t.cs[y]))
			default:
				if decimal.MatchString(strings.TrimSpace(str)) || percent.MatchString(strings.TrimSpace(str)) {
					fmt.Fprintf(t.out, "%s", PadLeft(str, SPACE, t.cs[y]))
				} else {
					fmt.Fprintf(t.out, "%s", PadRight(str, SPACE, t.cs[y]))

					// TODO Custom alignment per column
					//if max == 1 || pads[y] > 0 {
					//	fmt.Fprintf(t.out, "%s", Pad(str, SPACE, t.cs[y]))
					//} else {
					//	fmt.Fprintf(t.out, "%s", PadRight(str, SPACE, t.cs[y]))
					//}

				}
			}
			fmt.Fprintf(t.out, SPACE)
		}
		// Check if border is set
		// Replace with space if not set
		fmt.Fprint(t.out, ConditionString(t.border, t.pColumn, SPACE))
		fmt.Fprintln(t.out)
	}

	if t.rowLine {
		t.printLine(true)
	}

}

func (t *Table) parseDimension(str string, colKey, rowKey int) []string {
	var (
		raw []string
		max int
	)
	w := DisplayWidth(str)
	// Calculate Width
	// Check if with is grater than maximum width
	if w > t.mW {
		w = t.mW
	}

	// Check if width exists
	v, ok := t.cs[colKey]
	if !ok || v < w || v == 0 {
		t.cs[colKey] = w
	}

	if rowKey == -1 {
		return raw
	}
	// Calculate Height
	if t.autoWrap {
		raw, _ = WrapString(str, t.cs[colKey])
	} else {
		raw = getLines(str)
	}

	for _, line := range raw {
		if w := DisplayWidth(line); w > max {
			max = w
		}
	}

	// Make sure the with is the same length as maximum word
	// Important for cases where the width is smaller than maxu word
	if max > t.cs[colKey] {
		t.cs[colKey] = max
	}

	h := len(raw)
	v, ok = t.rs[rowKey]

	if !ok || v < h || v == 0 {
		t.rs[rowKey] = h
	}
	//fmt.Printf("Raw %+v %d\n", raw, len(raw))
	return raw
}
