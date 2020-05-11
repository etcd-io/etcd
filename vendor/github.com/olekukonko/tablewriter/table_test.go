// Copyright 2014 Oleku Konko All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

// This module is a Table Writer  API for the Go Programming Language.
// The protocols were written in pure Go and works on windows and unix systems

package tablewriter

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
)

func ExampleShort() {

	data := [][]string{
		[]string{"A", "The Good", "500"},
		[]string{"B", "The Very very Bad Man", "288"},
		[]string{"C", "The Ugly", "120"},
		[]string{"D", "The Gopher", "800"},
	}

	table := NewWriter(os.Stdout)
	table.SetHeader([]string{"Name", "Sign", "Rating"})

	for _, v := range data {
		table.Append(v)
	}
	table.Render()

}

func ExampleLong() {

	data := [][]string{
		[]string{"Learn East has computers with adapted keyboards with enlarged print etc", "  Some Data  ", " Another Data"},
		[]string{"Instead of lining up the letters all ", "the way across, he splits the keyboard in two", "Like most ergonomic keyboards", "See Data"},
	}

	table := NewWriter(os.Stdout)
	table.SetHeader([]string{"Name", "Sign", "Rating"})
	table.SetCenterSeparator("*")
	table.SetRowSeparator("=")

	for _, v := range data {
		table.Append(v)
	}
	table.Render()

}

func ExampleCSV() {
	table, _ := NewCSV(os.Stdout, "test.csv", true)
	table.SetCenterSeparator("*")
	table.SetRowSeparator("=")

	table.Render()
}

func TestCSVInfo(t *testing.T) {
	table, err := NewCSV(os.Stdout, "test_info.csv", true)
	if err != nil {
		t.Error(err)
		return
	}
	table.SetAlignment(ALIGN_LEFT)
	table.SetBorder(false)
	table.Render()
}

func TestCSVSeparator(t *testing.T) {
	table, err := NewCSV(os.Stdout, "test.csv", true)
	if err != nil {
		t.Error(err)
		return
	}
	table.SetRowLine(true)
	if runewidth.IsEastAsian() {
		table.SetCenterSeparator("＊")
		table.SetColumnSeparator("‡")
	} else {
		table.SetCenterSeparator("*")
		table.SetColumnSeparator("‡")
	}
	table.SetRowSeparator("-")
	table.SetAlignment(ALIGN_LEFT)
	table.Render()
}

func TestNoBorder(t *testing.T) {
	data := [][]string{
		[]string{"1/1/2014", "Domain name", "2233", "$10.98"},
		[]string{"1/1/2014", "January Hosting", "2233", "$54.95"},
		[]string{"", "    (empty)\n    (empty)", "", ""},
		[]string{"1/4/2014", "February Hosting", "2233", "$51.00"},
		[]string{"1/4/2014", "February Extra Bandwidth", "2233", "$30.00"},
		[]string{"1/4/2014", "    (Discount)", "2233", "-$1.00"},
	}

	var buf bytes.Buffer
	table := NewWriter(&buf)
	table.SetAutoWrapText(false)
	table.SetHeader([]string{"Date", "Description", "CV2", "Amount"})
	table.SetFooter([]string{"", "", "Total", "$145.93"}) // Add Footer
	table.SetBorder(false)                                // Set Border to false
	table.AppendBulk(data)                                // Add Bulk Data
	table.Render()

	want := `    DATE   |       DESCRIPTION        |  CV2  | AMOUNT   
+----------+--------------------------+-------+---------+
  1/1/2014 | Domain name              |  2233 | $10.98   
  1/1/2014 | January Hosting          |  2233 | $54.95   
           |     (empty)              |       |          
           |     (empty)              |       |          
  1/4/2014 | February Hosting         |  2233 | $51.00   
  1/4/2014 | February Extra Bandwidth |  2233 | $30.00   
  1/4/2014 |     (Discount)           |  2233 | -$1.00   
+----------+--------------------------+-------+---------+
                                        TOTAL | $145 93  
                                      +-------+---------+
`
	got := buf.String()
	if got != want {
		t.Errorf("border table rendering failed\ngot:\n%s\nwant:\n%s\n", got, want)
	}
}

func TestWithBorder(t *testing.T) {
	data := [][]string{
		[]string{"1/1/2014", "Domain name", "2233", "$10.98"},
		[]string{"1/1/2014", "January Hosting", "2233", "$54.95"},
		[]string{"", "    (empty)\n    (empty)", "", ""},
		[]string{"1/4/2014", "February Hosting", "2233", "$51.00"},
		[]string{"1/4/2014", "February Extra Bandwidth", "2233", "$30.00"},
		[]string{"1/4/2014", "    (Discount)", "2233", "-$1.00"},
	}

	var buf bytes.Buffer
	table := NewWriter(&buf)
	table.SetAutoWrapText(false)
	table.SetHeader([]string{"Date", "Description", "CV2", "Amount"})
	table.SetFooter([]string{"", "", "Total", "$145.93"}) // Add Footer
	table.AppendBulk(data)                                // Add Bulk Data
	table.Render()

	want := `+----------+--------------------------+-------+---------+
|   DATE   |       DESCRIPTION        |  CV2  | AMOUNT  |
+----------+--------------------------+-------+---------+
| 1/1/2014 | Domain name              |  2233 | $10.98  |
| 1/1/2014 | January Hosting          |  2233 | $54.95  |
|          |     (empty)              |       |         |
|          |     (empty)              |       |         |
| 1/4/2014 | February Hosting         |  2233 | $51.00  |
| 1/4/2014 | February Extra Bandwidth |  2233 | $30.00  |
| 1/4/2014 |     (Discount)           |  2233 | -$1.00  |
+----------+--------------------------+-------+---------+
|                                       TOTAL | $145 93 |
+----------+--------------------------+-------+---------+
`
	got := buf.String()
	if got != want {
		t.Errorf("border table rendering failed\ngot:\n%s\nwant:\n%s\n", got, want)
	}
}

func TestPrintingInMarkdown(t *testing.T) {
	fmt.Println("TESTING")
	data := [][]string{
		[]string{"1/1/2014", "Domain name", "2233", "$10.98"},
		[]string{"1/1/2014", "January Hosting", "2233", "$54.95"},
		[]string{"1/4/2014", "February Hosting", "2233", "$51.00"},
		[]string{"1/4/2014", "February Extra Bandwidth", "2233", "$30.00"},
	}

	var buf bytes.Buffer
	table := NewWriter(&buf)
	table.SetHeader([]string{"Date", "Description", "CV2", "Amount"})
	table.AppendBulk(data) // Add Bulk Data
	table.SetBorders(Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")
	table.Render()

	want := `|   DATE   |       DESCRIPTION        | CV2  | AMOUNT |
|----------|--------------------------|------|--------|
| 1/1/2014 | Domain name              | 2233 | $10.98 |
| 1/1/2014 | January Hosting          | 2233 | $54.95 |
| 1/4/2014 | February Hosting         | 2233 | $51.00 |
| 1/4/2014 | February Extra Bandwidth | 2233 | $30.00 |
`
	got := buf.String()
	if got != want {
		t.Errorf("border table rendering failed\ngot:\n%s\nwant:\n%s\n", got, want)
	}
}

func TestPrintHeading(t *testing.T) {
	var buf bytes.Buffer
	table := NewWriter(&buf)
	table.SetHeader([]string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "a", "b", "c"})
	table.printHeading()
	want := `| 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | A | B | C |
+---+---+---+---+---+---+---+---+---+---+---+---+
`
	got := buf.String()
	if got != want {
		t.Errorf("header rendering failed\ngot:\n%s\nwant:\n%s\n", got, want)
	}
}

func TestPrintHeadingWithoutAutoFormat(t *testing.T) {
	var buf bytes.Buffer
	table := NewWriter(&buf)
	table.SetHeader([]string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "a", "b", "c"})
	table.SetAutoFormatHeaders(false)
	table.printHeading()
	want := `| 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | a | b | c |
+---+---+---+---+---+---+---+---+---+---+---+---+
`
	got := buf.String()
	if got != want {
		t.Errorf("header rendering failed\ngot:\n%s\nwant:\n%s\n", got, want)
	}
}

func TestPrintFooter(t *testing.T) {
	var buf bytes.Buffer
	table := NewWriter(&buf)
	table.SetHeader([]string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "a", "b", "c"})
	table.SetFooter([]string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "a", "b", "c"})
	table.printFooter()
	want := `| 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | A | B | C |
+---+---+---+---+---+---+---+---+---+---+---+---+
`
	got := buf.String()
	if got != want {
		t.Errorf("footer rendering failed\ngot:\n%s\nwant:\n%s\n", got, want)
	}
}

func TestPrintFooterWithoutAutoFormat(t *testing.T) {
	var buf bytes.Buffer
	table := NewWriter(&buf)
	table.SetAutoFormatHeaders(false)
	table.SetHeader([]string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "a", "b", "c"})
	table.SetFooter([]string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "a", "b", "c"})
	table.printFooter()
	want := `| 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | a | b | c |
+---+---+---+---+---+---+---+---+---+---+---+---+
`
	got := buf.String()
	if got != want {
		t.Errorf("footer rendering failed\ngot:\n%s\nwant:\n%s\n", got, want)
	}
}

func TestPrintTableWithAndWithoutAutoWrap(t *testing.T) {
	var buf bytes.Buffer
	var multiline = `A multiline
string with some lines being really long.`

	with := NewWriter(&buf)
	with.Append([]string{multiline})
	with.Render()
	want := `+--------------------------------+
| A multiline string with some   |
| lines being really long.       |
+--------------------------------+
`
	got := buf.String()
	if got != want {
		t.Errorf("multiline text rendering with wrapping failed\ngot:\n%s\nwant:\n%s\n", got, want)
	}

	buf.Truncate(0)
	without := NewWriter(&buf)
	without.SetAutoWrapText(false)
	without.Append([]string{multiline})
	without.Render()
	want = `+-------------------------------------------+
| A multiline                               |
| string with some lines being really long. |
+-------------------------------------------+
`
	got = buf.String()
	if got != want {
		t.Errorf("multiline text rendering without wrapping rendering failed\ngot:\n%s\nwant:\n%s\n", got, want)
	}
}

func TestPrintLine(t *testing.T) {
	header := make([]string, 12)
	val := " "
	want := ""
	for i := range header {
		header[i] = val
		want = fmt.Sprintf("%s+-%s-", want, strings.Replace(val, " ", "-", -1))
		val = val + " "
	}
	want = want + "+"
	var buf bytes.Buffer
	table := NewWriter(&buf)
	table.SetHeader(header)
	table.printLine(false)
	got := buf.String()
	if got != want {
		t.Errorf("line rendering failed\ngot:\n%s\nwant:\n%s\n", got, want)
	}
}

func TestAnsiStrip(t *testing.T) {
	header := make([]string, 12)
	val := " "
	want := ""
	for i := range header {
		header[i] = "\033[43;30m" + val + "\033[00m"
		want = fmt.Sprintf("%s+-%s-", want, strings.Replace(val, " ", "-", -1))
		val = val + " "
	}
	want = want + "+"
	var buf bytes.Buffer
	table := NewWriter(&buf)
	table.SetHeader(header)
	table.printLine(false)
	got := buf.String()
	if got != want {
		t.Errorf("line rendering failed\ngot:\n%s\nwant:\n%s\n", got, want)
	}
}

func NewCustomizedTable(out io.Writer) *Table {
	table := NewWriter(out)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetBorder(false)
	table.SetAlignment(ALIGN_LEFT)
	table.SetHeader([]string{})
	return table
}

func TestSubclass(t *testing.T) {
	buf := new(bytes.Buffer)
	table := NewCustomizedTable(buf)

	data := [][]string{
		[]string{"A", "The Good", "500"},
		[]string{"B", "The Very very Bad Man", "288"},
		[]string{"C", "The Ugly", "120"},
		[]string{"D", "The Gopher", "800"},
	}

	for _, v := range data {
		table.Append(v)
	}
	table.Render()

	output := string(buf.Bytes())
	want := `  A  The Good               500  
  B  The Very very Bad Man  288  
  C  The Ugly               120  
  D  The Gopher             800  
`
	if output != want {
		t.Error(fmt.Sprintf("Unexpected output '%v' != '%v'", output, want))
	}
}

func TestAutoMergeRows(t *testing.T) {
	data := [][]string{
		[]string{"A", "The Good", "500"},
		[]string{"A", "The Very very Bad Man", "288"},
		[]string{"B", "The Very very Bad Man", "120"},
		[]string{"B", "The Very very Bad Man", "200"},
	}
	var buf bytes.Buffer
	table := NewWriter(&buf)
	table.SetHeader([]string{"Name", "Sign", "Rating"})

	for _, v := range data {
		table.Append(v)
	}
	table.SetAutoMergeCells(true)
	table.Render()
	want := `+------+-----------------------+--------+
| NAME |         SIGN          | RATING |
+------+-----------------------+--------+
| A    | The Good              |    500 |
|      | The Very very Bad Man |    288 |
| B    |                       |    120 |
|      |                       |    200 |
+------+-----------------------+--------+
`
	got := buf.String()
	if got != want {
		t.Errorf("\ngot:\n%s\nwant:\n%s\n", got, want)
	}

	buf.Reset()
	table = NewWriter(&buf)
	table.SetHeader([]string{"Name", "Sign", "Rating"})

	for _, v := range data {
		table.Append(v)
	}
	table.SetAutoMergeCells(true)
	table.SetRowLine(true)
	table.Render()
	want = `+------+-----------------------+--------+
| NAME |         SIGN          | RATING |
+------+-----------------------+--------+
| A    | The Good              |    500 |
+      +-----------------------+--------+
|      | The Very very Bad Man |    288 |
+------+                       +--------+
| B    |                       |    120 |
+      +                       +--------+
|      |                       |    200 |
+------+-----------------------+--------+
`
	got = buf.String()
	if got != want {
		t.Errorf("\ngot:\n%s\nwant:\n%s\n", got, want)
	}

	buf.Reset()
	table = NewWriter(&buf)
	table.SetHeader([]string{"Name", "Sign", "Rating"})

	dataWithlongText := [][]string{
		[]string{"A", "The Good", "500"},
		[]string{"A", "The Very very very very very Bad Man", "288"},
		[]string{"B", "The Very very very very very Bad Man", "120"},
		[]string{"C", "The Very very Bad Man", "200"},
	}
	table.AppendBulk(dataWithlongText)
	table.SetAutoMergeCells(true)
	table.SetRowLine(true)
	table.Render()
	want = `+------+--------------------------------+--------+
| NAME |              SIGN              | RATING |
+------+--------------------------------+--------+
| A    | The Good                       |    500 |
+------+--------------------------------+--------+
| A    | The Very very very very very   |    288 |
|      | Bad Man                        |        |
+------+                                +--------+
| B    |                                |    120 |
|      |                                |        |
+------+--------------------------------+--------+
| C    | The Very very Bad Man          |    200 |
+------+--------------------------------+--------+
`
	got = buf.String()
	if got != want {
		t.Errorf("\ngot:\n%s\nwant:\n%s\n", got, want)
	}
}
