package tablewriter

import (
	"encoding/csv"
	"io"
	"os"
)

// NewCSV Start A new table by importing from a CSV file
// Takes io.Writer and csv File name
func NewCSV(writer io.Writer, fileName string, hasHeader bool, opts ...Option) (*Table, error) {
	// Open the CSV file
	file, err := os.Open(fileName)
	if err != nil {
		// Log implicitly handled by NewTable if logger is configured via opts
		return nil, err // Return nil *Table on error
	}
	defer file.Close() // Ensure file is closed

	// Create a CSV reader
	csvReader := csv.NewReader(file)

	// Delegate to NewCSVReader, passing through the options
	return NewCSVReader(writer, csvReader, hasHeader, opts...)
}

// NewCSVReader Start a New Table Writer with csv.Reader
// This enables customisation such as reader.Comma = ';'
// See http://golang.org/src/pkg/encoding/csv/reader.go?s=3213:3671#L94
func NewCSVReader(writer io.Writer, csvReader *csv.Reader, hasHeader bool, opts ...Option) (*Table, error) {
	// Create a new table instance using the modern API and provided options.
	// Options configure the table's appearance and behavior (renderer, borders, etc.).
	t := NewTable(writer, opts...) // Logger setup happens here if WithLogger/WithDebug is passed

	// Process header row if specified
	if hasHeader {
		headers, err := csvReader.Read()
		if err != nil {
			// Handle EOF specifically: means the CSV was empty or contained only an empty header line.
			if err == io.EOF {
				t.logger.Debug("NewCSVReader: CSV empty or only header found (EOF after header read attempt).")
				// Return the table configured by opts, but without data/header.
				// It's ready for Render() which will likely output nothing or just borders if configured.
				return t, nil
			}
			// Log other read errors
			t.logger.Errorf("NewCSVReader: Error reading CSV header: %v", err)
			return nil, err // Return nil *Table on critical read error
		}

		// Check if the read header is genuinely empty (e.g., a blank line in the CSV)
		isEmptyHeader := true
		for _, h := range headers {
			if h != "" {
				isEmptyHeader = false
				break
			}
		}

		if !isEmptyHeader {
			t.Header(headers) // Use the Table method to set the header data
			t.logger.Debugf("NewCSVReader: Header set from CSV: %v", headers)
		} else {
			t.logger.Debug("NewCSVReader: Read an empty header line, skipping setting table header.")
		}
	}

	// Process data rows
	rowCount := 0
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break // Reached the end of the CSV data
		}
		if err != nil {
			// Log other read errors during data processing
			t.logger.Errorf("NewCSVReader: Error reading CSV record: %v", err)
			return nil, err // Return nil *Table on critical read error
		}

		// Append the record to the table's internal buffer (for batch rendering).
		// The Table.Append method handles conversion and storage.
		if appendErr := t.Append(record); appendErr != nil {
			t.logger.Errorf("NewCSVReader: Error appending record #%d: %v", rowCount+1, appendErr)
			// Decide if append error is fatal. For now, let's treat it as fatal.
			return nil, appendErr
		}
		rowCount++
	}
	t.logger.Debugf("NewCSVReader: Finished reading CSV. Appended %d data rows.", rowCount)

	// Return the configured and populated table instance, ready for Render() call.
	return t, nil
}
