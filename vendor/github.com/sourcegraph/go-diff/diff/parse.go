package diff

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ParseMultiFileDiff parses a multi-file unified diff. It returns an error if
// parsing failed as a whole, but does its best to parse as many files in the
// case of per-file errors. If it cannot detect when the diff of the next file
// begins, the hunks are added to the FileDiff of the previous file.
func ParseMultiFileDiff(diff []byte) ([]*FileDiff, error) {
	return ParseMultiFileDiffOptions(diff, ParseOptions{})
}

// ParseMultiFileDiffOptions parses a multi-file unified diff with the given options.
func ParseMultiFileDiffOptions(diff []byte, opts ParseOptions) ([]*FileDiff, error) {
	return NewMultiFileDiffReaderOptions(bytes.NewReader(diff), opts).ReadAllFiles()
}

// NewMultiFileDiffReader returns a new MultiFileDiffReader that reads
// a multi-file unified diff from r.
func NewMultiFileDiffReader(r io.Reader) *MultiFileDiffReader {
	return NewMultiFileDiffReaderOptions(r, ParseOptions{})
}

// NewMultiFileDiffReaderOptions returns a new MultiFileDiffReader that reads
// a multi-file unified diff from r with the given options.
func NewMultiFileDiffReaderOptions(r io.Reader, opts ParseOptions) *MultiFileDiffReader {
	return &MultiFileDiffReader{reader: newLineReaderOptions(r, opts)}
}

// MultiFileDiffReader reads a multi-file unified diff.
type MultiFileDiffReader struct {
	line   int
	offset int64
	reader *lineReader

	// TODO(sqs): line and offset tracking in multi-file diffs is broken; add tests and fix

	// nextFileFirstLine is a line that was read by a HunksReader that
	// was how it determined the hunk was complete. But to determine
	// that, it needed to read the first line of the next file. We
	// store nextFileFirstLine so we can "give the first line back" to
	// the next file.
	nextFileFirstLine []byte
}

// ReadFile reads the next file unified diff (including headers and
// all hunks) from r. If there are no more files in the diff, it
// returns error io.EOF.
func (r *MultiFileDiffReader) ReadFile() (*FileDiff, error) {
	fd, _, err := r.ReadFileWithTrailingContent()
	return fd, err
}

// ReadFileWithTrailingContent reads the next file unified diff (including
// headers and all hunks) from r, also returning any trailing content. If there
// are no more files in the diff, it returns error io.EOF.
func (r *MultiFileDiffReader) ReadFileWithTrailingContent() (*FileDiff, string, error) {
	fr := &FileDiffReader{
		line:           r.line,
		offset:         r.offset,
		reader:         r.reader,
		fileHeaderLine: r.nextFileFirstLine,
	}
	r.nextFileFirstLine = nil

	fd, err := fr.ReadAllHeaders()
	if err != nil {
		switch e := err.(type) {
		case *ParseError:
			if e.Err == ErrNoFileHeader || e.Err == ErrExtendedHeadersEOF {
				// Any non-diff content preceding a valid diff is included in the
				// extended headers of the following diff. In this way, mixed diff /
				// non-diff content can be parsed. Trailing non-diff content is
				// different: it doesn't make sense to return a FileDiff with only
				// extended headers populated. Instead, we return any trailing content
				// in case the caller needs it.
				trailing := ""
				if fd != nil {
					trailing = strings.Join(fd.Extended, "\n")
				}
				return nil, trailing, io.EOF
			}
			return nil, "", err

		case OverflowError:
			r.nextFileFirstLine = []byte(e)
			return fd, "", nil

		default:
			return nil, "", err
		}
	}

	// FileDiff is added/deleted file
	// No further collection of hunks needed
	if fd.NewName == "" {
		return fd, "", nil
	}

	// Before reading hunks, check to see if there are any. If there
	// aren't any, and there's another file after this file in the
	// diff, then the hunks reader will complain ErrNoHunkHeader. It's
	// not easy for us to tell from that error alone if that was
	// caused by the lack of any hunks, or a malformatted hunk, so we
	// need to perform the check here.
	hr := fr.HunksReader()
	line, err := r.reader.readLine()
	if err != nil && err != io.EOF {
		return fd, "", err
	}
	line = bytes.TrimSuffix(line, []byte{'\n'})
	if bytes.HasPrefix(line, hunkPrefix) {
		hr.nextHunkHeaderLine = line
		fd.Hunks, err = hr.ReadAllHunks()
		r.line = fr.line
		r.offset = fr.offset
		if err != nil {
			if e0, ok := err.(*ParseError); ok {
				if e, ok := e0.Err.(*ErrBadHunkLine); ok {
					// This just means we finished reading the hunks for the
					// current file. See the ErrBadHunkLine doc for more info.
					r.nextFileFirstLine = e.Line
					return fd, "", nil
				}
			}
			return nil, "", err
		}
	} else {
		// There weren't any hunks, so that line we peeked ahead at
		// actually belongs to the next file. Put it back.
		r.nextFileFirstLine = line
	}

	return fd, "", nil
}

// ReadAllFiles reads all file unified diffs (including headers and all
// hunks) remaining in r.
func (r *MultiFileDiffReader) ReadAllFiles() ([]*FileDiff, error) {
	var ds []*FileDiff
	for {
		d, err := r.ReadFile()
		if d != nil {
			ds = append(ds, d)
		}
		if err == io.EOF {
			return ds, nil
		}
		if err != nil {
			return nil, err
		}
	}
}

// ParseFileDiff parses a file unified diff.
func ParseFileDiff(diff []byte) (*FileDiff, error) {
	return ParseFileDiffOptions(diff, ParseOptions{})
}

// ParseFileDiffOptions parses a file unified diff with the given options.
func ParseFileDiffOptions(diff []byte, opts ParseOptions) (*FileDiff, error) {
	return NewFileDiffReaderOptions(bytes.NewReader(diff), opts).Read()
}

// NewFileDiffReader returns a new FileDiffReader that reads a file
// unified diff.
func NewFileDiffReader(r io.Reader) *FileDiffReader {
	return NewFileDiffReaderOptions(r, ParseOptions{})
}

// NewFileDiffReaderOptions returns a new FileDiffReader that reads a file
// unified diff with the given options.
func NewFileDiffReaderOptions(r io.Reader, opts ParseOptions) *FileDiffReader {
	return &FileDiffReader{reader: newLineReaderOptions(r, opts)}
}

// FileDiffReader reads a unified file diff.
type FileDiffReader struct {
	line   int
	offset int64
	reader *lineReader

	// fileHeaderLine is the first file header line, set by:
	//
	// (1) ReadExtendedHeaders if it encroaches on a file header line
	//     (which it must to detect when extended headers are done); or
	// (2) (*MultiFileDiffReader).ReadFile() if it encroaches on a
	//     file header line while reading the previous file's hunks (in a
	//     multi-file diff).
	fileHeaderLine []byte
}

// Read reads a file unified diff, including headers and hunks, from r.
func (r *FileDiffReader) Read() (*FileDiff, error) {
	fd, err := r.ReadAllHeaders()
	if err != nil {
		return nil, err
	}

	fd.Hunks, err = r.HunksReader().ReadAllHunks()
	if err != nil {
		return nil, err
	}

	return fd, nil
}

// ReadAllHeaders reads the file headers and extended headers (if any)
// from a file unified diff. It does not read hunks, and the returned
// FileDiff's Hunks field is nil. To read the hunks, call the
// (*FileDiffReader).HunksReader() method to get a HunksReader and
// read hunks from that.
func (r *FileDiffReader) ReadAllHeaders() (*FileDiff, error) {
	var err error
	fd := &FileDiff{}

	fd.Extended, err = r.ReadExtendedHeaders()
	if pe, ok := err.(*ParseError); ok && pe.Err == ErrExtendedHeadersEOF {
		wasEmpty := handleEmpty(fd)
		if wasEmpty {
			return fd, nil
		}
		return fd, err
	} else if _, ok := err.(OverflowError); ok {
		handleEmpty(fd)
		return fd, err
	} else if err != nil {
		return fd, err
	}

	var origTime, newTime *time.Time
	fd.OrigName, fd.NewName, origTime, newTime, err = r.ReadFileHeaders()
	if err != nil {
		return nil, err
	}
	if origTime != nil {
		fd.OrigTime = origTime
	}
	if newTime != nil {
		fd.NewTime = newTime
	}

	return fd, nil
}

// HunksReader returns a new HunksReader that reads hunks from r. The
// HunksReader's line and offset (used in error messages) is set to
// start where the file diff header ended (which means errors have the
// correct position information).
func (r *FileDiffReader) HunksReader() *HunksReader {
	return &HunksReader{
		line:   r.line,
		offset: r.offset,
		reader: r.reader,
	}
}

// ReadFileHeaders reads the unified file diff header (the lines that
// start with "---" and "+++" with the orig/new file names and
// timestamps). Or which starts with "Only in " with dir path and filename.
// "Only in" message is supported in POSIX locale: https://pubs.opengroup.org/onlinepubs/9699919799/utilities/diff.html#tag_20_34_10
func (r *FileDiffReader) ReadFileHeaders() (origName, newName string, origTimestamp, newTimestamp *time.Time, err error) {
	if r.fileHeaderLine != nil {
		if isOnlyMessage, source, filename := parseOnlyInMessage(r.fileHeaderLine); isOnlyMessage {
			return filepath.Join(string(source), string(filename)),
				"", nil, nil, nil
		}
	}
	origName, origTimestamp, err = r.readOneFileHeader([]byte("--- "))
	if err != nil {
		return "", "", nil, nil, err
	}

	newName, newTimestamp, err = r.readOneFileHeader([]byte("+++ "))
	if err != nil {
		return "", "", nil, nil, err
	}

	unquotedOrigName, err := strconv.Unquote(origName)
	if err == nil {
		origName = unquotedOrigName
	}
	unquotedNewName, err := strconv.Unquote(newName)
	if err == nil {
		newName = unquotedNewName
	}

	return origName, newName, origTimestamp, newTimestamp, nil
}

// readOneFileHeader reads one of the file headers (prefix should be
// either "+++ " or "--- ").
func (r *FileDiffReader) readOneFileHeader(prefix []byte) (filename string, timestamp *time.Time, err error) {
	var line []byte

	if r.fileHeaderLine == nil {
		var err error
		line, err = r.reader.readLine()
		if err == io.EOF {
			return "", nil, &ParseError{r.line, r.offset, ErrNoFileHeader}
		} else if err != nil {
			return "", nil, err
		}
	} else {
		line = r.fileHeaderLine
		r.fileHeaderLine = nil
	}

	if !bytes.HasPrefix(line, prefix) {
		return "", nil, &ParseError{r.line, r.offset, ErrBadFileHeader}
	}

	r.offset += int64(len(line))
	r.line++
	line = line[len(prefix):]

	trimmedLine := strings.TrimSpace(string(line)) // filenames that contain spaces may be terminated by a tab
	parts := strings.SplitN(trimmedLine, "\t", 2)
	filename = parts[0]
	if len(parts) == 2 {
		var ts time.Time
		// Timestamp is optional, but this header has it.
		ts, err = time.Parse(diffTimeParseLayout, parts[1])
		if err != nil {
			var err1 error
			ts, err1 = time.Parse(diffTimeParseWithoutTZLayout, parts[1])
			if err1 != nil {
				return "", nil, err
			}
			err = nil
		}
		timestamp = &ts
	}

	return filename, timestamp, err
}

// OverflowError is returned when we have overflowed into the start
// of the next file while reading extended headers.
type OverflowError string

func (e OverflowError) Error() string {
	return fmt.Sprintf("overflowed into next file: %s", string(e))
}

// ReadExtendedHeaders reads the extended header lines, if any, from a
// unified diff file (e.g., git's "diff --git a/foo.go b/foo.go", "new
// mode <mode>", "rename from <path>", etc.).
func (r *FileDiffReader) ReadExtendedHeaders() ([]string, error) {
	var xheaders []string
	firstLine := true
	for {
		var line []byte
		if r.fileHeaderLine == nil {
			var err error
			line, err = r.reader.readLine()
			if err == io.EOF {
				return xheaders, &ParseError{r.line, r.offset, ErrExtendedHeadersEOF}
			} else if err != nil {
				return xheaders, err
			}
		} else {
			line = r.fileHeaderLine
			r.fileHeaderLine = nil
		}

		if bytes.HasPrefix(line, []byte("diff --git ")) {
			if firstLine {
				firstLine = false
			} else {
				return xheaders, OverflowError(line)
			}
		}
		if bytes.HasPrefix(line, []byte("--- ")) {
			// We've reached the file header.
			r.fileHeaderLine = line // pass to readOneFileHeader (see fileHeaderLine field doc)
			return xheaders, nil
		}

		// Reached message that file is added/deleted
		if isOnlyInMessage, _, _ := parseOnlyInMessage(line); isOnlyInMessage {
			r.fileHeaderLine = line // pass to readOneFileHeader (see fileHeaderLine field doc)
			return xheaders, nil
		}

		r.line++
		r.offset += int64(len(line))
		xheaders = append(xheaders, string(line))
	}
}

// readQuotedFilename extracts a quoted filename from the beginning of a string,
// returning the unquoted filename and any remaining text after the filename.
func readQuotedFilename(text string) (value string, remainder string, err error) {
	if text == "" || text[0] != '"' {
		return "", "", fmt.Errorf(`string must start with a '"': %s`, text)
	}

	// The end quote is the first quote NOT preceeded by an uneven number of backslashes.
	numberOfBackslashes := 0
	for i, c := range text {
		if c == '"' && i > 0 && numberOfBackslashes%2 == 0 {
			value, err = strconv.Unquote(text[:i+1])
			remainder = text[i+1:]
			return
		} else if c == '\\' {
			numberOfBackslashes++
		} else {
			numberOfBackslashes = 0
		}
	}
	return "", "", fmt.Errorf(`end of string found while searching for '"': %s`, text)
}

// parseDiffGitArgs extracts the two filenames from a 'diff --git' line.
// Returns false on syntax error, true if syntax is valid. Even with a
// valid syntax, it may be impossible to extract filenames; if so, the
// function returns ("", "", true).
func parseDiffGitArgs(diffArgs string) (string, string, bool) {
	diffArgs = strings.TrimSuffix(diffArgs, "\r")
	length := len(diffArgs)
	if length < 3 {
		return "", "", false
	}

	if diffArgs[0] != '"' && diffArgs[length-1] != '"' {
		// Both filenames are unquoted.
		firstSpace := strings.IndexByte(diffArgs, ' ')
		if firstSpace <= 0 || firstSpace == length-1 {
			return "", "", false
		}

		secondSpace := strings.IndexByte(diffArgs[firstSpace+1:], ' ')
		if secondSpace == -1 {
			if diffArgs[firstSpace+1] == '"' {
				// The second filename begins with '"', but doesn't end with one.
				return "", "", false
			}
			return diffArgs[:firstSpace], diffArgs[firstSpace+1:], true
		}

		// One or both filenames contain a space, but the names are
		// unquoted. Here, the 'diff --git' syntax is ambiguous, and
		// we have to obtain the filenames elsewhere (e.g. from the
		// hunk headers or extended headers). HOWEVER, if the file
		// is newly created and empty, there IS no other place to
		// find the filename. In this case, the two filenames are
		// identical (except for the leading 'a/' prefix), and we have
		// to handle that case here.
		first := diffArgs[:length/2]
		second := diffArgs[length/2+1:]

		// If the two strings could be equal, based on length, proceed.
		if length%2 == 1 {
			// If the name minus the a/ b/ prefixes is equal, proceed.
			if len(first) >= 3 && first[1] == '/' && first[1:] == second[1:] {
				return first, second, true
			}
			// If the names don't have the a/ and b/ prefixes and they're equal, proceed.
			if !(first[:2] == "a/" && second[:2] == "b/") && first == second {
				return first, second, true
			}
		}

		// The syntax is (unfortunately) valid, but we could not extract
		// the filenames.
		return "", "", true
	}

	if diffArgs[0] == '"' {
		first, remainder, err := readQuotedFilename(diffArgs)
		if err != nil || len(remainder) < 2 || remainder[0] != ' ' {
			return "", "", false
		}
		if remainder[1] == '"' {
			second, remainder, err := readQuotedFilename(remainder[1:])
			if remainder != "" || err != nil {
				return "", "", false
			}
			return first, second, true
		}
		return first, remainder[1:], true
	}

	// In this case, second argument MUST be quoted (or it's a syntax error)
	i := strings.IndexByte(diffArgs, '"')
	if i == -1 || i+2 >= length || diffArgs[i-1] != ' ' {
		return "", "", false
	}

	second, remainder, err := readQuotedFilename(diffArgs[i:])
	if remainder != "" || err != nil {
		return "", "", false
	}
	return diffArgs[:i-1], second, true
}

// handleEmpty detects when FileDiff was an empty diff and will not have any hunks
// that follow. It updates fd fields from the parsed extended headers.
func handleEmpty(fd *FileDiff) (wasEmpty bool) {
	lineCount := len(fd.Extended)
	if lineCount > 0 && !strings.HasPrefix(fd.Extended[0], "diff --git ") {
		return false
	}

	lineHasPrefix := func(idx int, prefix string) bool {
		return strings.HasPrefix(fd.Extended[idx], prefix)
	}

	linesHavePrefixes := func(idx1 int, prefix1 string, idx2 int, prefix2 string) bool {
		return lineHasPrefix(idx1, prefix1) && lineHasPrefix(idx2, prefix2)
	}

	isCopy := (lineCount == 4 && linesHavePrefixes(2, "copy from ", 3, "copy to ")) ||
		(lineCount == 6 && linesHavePrefixes(2, "copy from ", 3, "copy to ") && lineHasPrefix(5, "Binary files ")) ||
		(lineCount == 6 && linesHavePrefixes(1, "old mode ", 2, "new mode ") && linesHavePrefixes(4, "copy from ", 5, "copy to "))

	isRename := (lineCount == 4 && linesHavePrefixes(2, "rename from ", 3, "rename to ")) ||
		(lineCount == 5 && linesHavePrefixes(2, "rename from ", 3, "rename to ") && lineHasPrefix(4, "Binary files ")) ||
		(lineCount == 6 && linesHavePrefixes(2, "rename from ", 3, "rename to ") && lineHasPrefix(5, "Binary files ")) ||
		(lineCount == 6 && linesHavePrefixes(1, "old mode ", 2, "new mode ") && linesHavePrefixes(4, "rename from ", 5, "rename to "))

	isDeletedFile := (lineCount == 3 || lineCount == 4 && lineHasPrefix(3, "Binary files ") || lineCount > 4 && lineHasPrefix(3, "GIT binary patch")) &&
		lineHasPrefix(1, "deleted file mode ")

	isNewFile := (lineCount == 3 || lineCount == 4 && lineHasPrefix(3, "Binary files ") || lineCount > 4 && lineHasPrefix(3, "GIT binary patch")) &&
		lineHasPrefix(1, "new file mode ")

	isModeChange := lineCount == 3 && linesHavePrefixes(1, "old mode ", 2, "new mode ")

	isBinaryPatch := lineCount == 3 && lineHasPrefix(2, "Binary files ") || lineCount > 3 && lineHasPrefix(2, "GIT binary patch")

	if !isModeChange && !isCopy && !isRename && !isBinaryPatch && !isNewFile && !isDeletedFile {
		return false
	}

	var success bool
	fd.OrigName, fd.NewName, success = parseDiffGitArgs(fd.Extended[0][len("diff --git "):])
	if isNewFile {
		fd.OrigName = "/dev/null"
	}

	if isDeletedFile {
		fd.NewName = "/dev/null"
	}

	// For ambiguous 'diff --git' lines, try to reconstruct filenames using extended headers.
	if success && (isCopy || isRename) && fd.OrigName == "" && fd.NewName == "" {
		diffArgs := fd.Extended[0][len("diff --git "):]

		tryReconstruct := func(header string, prefix string, whichFile int, result *string) {
			if !strings.HasPrefix(header, prefix) {
				return
			}
			rawFilename := header[len(prefix):]
			rawFilename = strings.TrimSuffix(rawFilename, "\r")

			// extract the filename prefix (e.g. "a/") from the 'diff --git' line.
			var prefixLetterIndex int
			if whichFile == 1 {
				prefixLetterIndex = 0
			} else if whichFile == 2 {
				prefixLetterIndex = len(diffArgs) - len(rawFilename) - 2
			}
			if prefixLetterIndex < 0 || diffArgs[prefixLetterIndex+1] != '/' {
				return
			}

			*result = diffArgs[prefixLetterIndex:prefixLetterIndex+2] + rawFilename
		}

		for _, header := range fd.Extended {
			tryReconstruct(header, "copy from ", 1, &fd.OrigName)
			tryReconstruct(header, "copy to ", 2, &fd.NewName)
			tryReconstruct(header, "rename from ", 1, &fd.OrigName)
			tryReconstruct(header, "rename to ", 2, &fd.NewName)
		}
	}
	return success
}

var (
	// ErrNoFileHeader is when a file unified diff has no file header
	// (i.e., the lines that begin with "---" and "+++").
	ErrNoFileHeader = errors.New("expected file header, got EOF")

	// ErrBadFileHeader is when a file unified diff has a malformed
	// file header (i.e., the lines that begin with "---" and "+++").
	ErrBadFileHeader = errors.New("bad file header")

	// ErrExtendedHeadersEOF is when an EOF was encountered while reading extended file headers, which means that there were no ---/+++ headers encountered before hunks (if any) began.
	ErrExtendedHeadersEOF = errors.New("expected file header while reading extended headers, got EOF")

	// ErrBadOnlyInMessage is when a file have a malformed `only in` message
	// Should be in format `Only in {source}: {filename}`
	ErrBadOnlyInMessage = errors.New("bad 'only in' message")
)

// ParseHunks parses hunks from a unified diff. The diff must consist
// only of hunks and not include a file header; if it has a file
// header, use ParseFileDiff.
func ParseHunks(diff []byte) ([]*Hunk, error) {
	return ParseHunksOptions(diff, ParseOptions{})
}

// ParseHunksOptions parses hunks from a unified diff with the given options.
func ParseHunksOptions(diff []byte, opts ParseOptions) ([]*Hunk, error) {
	r := NewHunksReaderOptions(bytes.NewReader(diff), opts)
	hunks, err := r.ReadAllHunks()
	if err != nil {
		return nil, err
	}
	return hunks, nil
}

// NewHunksReader returns a new HunksReader that reads unified diff hunks
// from r.
func NewHunksReader(r io.Reader) *HunksReader {
	return NewHunksReaderOptions(r, ParseOptions{})
}

// NewHunksReaderOptions returns a new HunksReader that reads unified diff hunks
// from r with the given options.
func NewHunksReaderOptions(r io.Reader, opts ParseOptions) *HunksReader {
	return &HunksReader{reader: newLineReaderOptions(r, opts)}
}

// A HunksReader reads hunks from a unified diff.
type HunksReader struct {
	line   int
	offset int64
	hunk   *Hunk
	reader *lineReader

	nextHunkHeaderLine []byte
}

// ReadHunk reads one hunk from r. If there are no more hunks, it
// returns error io.EOF.
func (r *HunksReader) ReadHunk() (*Hunk, error) {
	r.hunk = nil
	lastLineFromOrig := true
	var line []byte
	var err error
	for {
		if r.nextHunkHeaderLine != nil {
			// Use stored hunk header line that was scanned in at the
			// completion of the previous hunk's ReadHunk.
			line = r.nextHunkHeaderLine
			r.nextHunkHeaderLine = nil
		} else {
			line, err = r.reader.readLine()
			if err != nil {
				if err == io.EOF && r.hunk != nil {
					return r.hunk, nil
				}
				return nil, err
			}
		}

		// Record position.
		r.line++
		r.offset += int64(len(line))

		if r.hunk == nil {
			// Check for presence of hunk header.
			if !bytes.HasPrefix(line, hunkPrefix) {
				return nil, &ParseError{r.line, r.offset, ErrNoHunkHeader}
			}

			// Parse hunk header.
			r.hunk = &Hunk{}
			items := []interface{}{
				&r.hunk.OrigStartLine, &r.hunk.OrigLines,
				&r.hunk.NewStartLine, &r.hunk.NewLines,
			}
			header, section, err := normalizeHeader(string(line))
			if err != nil {
				return nil, &ParseError{r.line, r.offset, err}
			}
			n, err := fmt.Sscanf(header, hunkHeader, items...)
			if err != nil {
				return nil, err
			}
			if n < len(items) {
				return nil, &ParseError{r.line, r.offset, &ErrBadHunkHeader{header: string(line)}}
			}

			r.hunk.Section = section
		} else {
			// Read hunk body line.

			// If the line starts with `---` and the next one with `+++` we're
			// looking at a non-extended file header and need to abort.
			if bytes.HasPrefix(line, []byte("---")) {
				ok, err := r.reader.nextLineStartsWith("+++")
				if err != nil {
					return r.hunk, err
				}
				if ok {
					ok2, _ := r.reader.nextNextLineStartsWith(string(hunkPrefix))
					if ok2 {
						return r.hunk, &ParseError{r.line, r.offset, &ErrBadHunkLine{Line: line}}
					}
				}
			}

			// If the line starts with the hunk prefix, this hunk is complete.
			if bytes.HasPrefix(line, hunkPrefix) {
				// But we've already read in the next hunk's
				// header, so we need to be sure that the next call to
				// ReadHunk starts with that header.
				r.nextHunkHeaderLine = line

				// Rewind position.
				r.line--
				r.offset -= int64(len(line))

				return r.hunk, nil
			}

			if len(line) >= 1 && !linePrefix(line[0]) {
				// Bad hunk header line. If we're reading a multi-file
				// diff, this may be the end of the current
				// file. Return a "rich" error that lets our caller
				// handle that case.
				return r.hunk, &ParseError{r.line, r.offset, &ErrBadHunkLine{Line: line}}
			}
			if bytes.Equal(bytes.TrimSuffix(line, []byte("\r")), []byte(noNewlineMessage)) {
				if lastLineFromOrig {
					// Retain the newline in the body (otherwise the
					// diff line would be like "-a+b", where "+b" is
					// the the next line of the new file, which is not
					// validly formatted) but record that the orig had
					// no newline.
					r.hunk.OrigNoNewlineAt = int32(len(r.hunk.Body))
				} else {
					// Remove previous line's newline.
					if len(r.hunk.Body) != 0 {
						r.hunk.Body = r.hunk.Body[:len(r.hunk.Body)-1]
					}
				}
				continue
			}

			if len(line) > 0 {
				lastLineFromOrig = line[0] == '-'
			}

			r.hunk.Body = append(r.hunk.Body, line...)
			r.hunk.Body = append(r.hunk.Body, '\n')
		}
	}
}

const noNewlineMessage = `\ No newline at end of file`

// linePrefixes is the set of all characters a valid line in a diff
// hunk can start with. '\' can appear in diffs when no newline is
// present at the end of a file.
// See: 'http://www.gnu.org/software/diffutils/manual/diffutils.html#Incomplete-Lines'
var linePrefixes = []byte{' ', '-', '+', '\\'}

// linePrefix returns true if 'c' is in 'linePrefixes'.
func linePrefix(c byte) bool {
	for _, p := range linePrefixes {
		if p == c {
			return true
		}
	}
	return false
}

// normalizeHeader takes a header of the form:
// "@@ -linestart[,chunksize] +linestart[,chunksize] @@ section"
// and returns two strings, with the first in the form:
// "@@ -linestart,chunksize +linestart,chunksize @@".
// where linestart and chunksize are both integers. The second is the
// optional section header. chunksize may be omitted from the header
// if its value is 1. normalizeHeader returns an error if the header
// is not in the correct format.
func normalizeHeader(header string) (string, string, error) {
	header = strings.TrimSuffix(header, "\r")
	// Split the header into five parts: the first '@@', the two
	// ranges, the last '@@', and the optional section.
	pieces := strings.SplitN(header, " ", 5)
	if len(pieces) < 4 {
		return "", "", &ErrBadHunkHeader{header: header}
	}

	if pieces[0] != "@@" {
		return "", "", &ErrBadHunkHeader{header: header}
	}
	for i := 1; i < 3; i++ {
		if !strings.ContainsRune(pieces[i], ',') {
			pieces[i] = pieces[i] + ",1"
		}
	}
	if pieces[3] != "@@" {
		return "", "", &ErrBadHunkHeader{header: header}
	}

	var section string
	if len(pieces) == 5 {
		section = pieces[4]
	}
	return strings.Join(pieces, " "), strings.TrimSpace(section), nil
}

// ReadAllHunks reads all remaining hunks from r. A successful call
// returns err == nil, not err == EOF. Because ReadAllHunks is defined
// to read until EOF, it does not treat end of file as an error to be
// reported.
func (r *HunksReader) ReadAllHunks() ([]*Hunk, error) {
	var hunks []*Hunk
	linesRead := int32(0)
	for {
		hunk, err := r.ReadHunk()
		if err == io.EOF {
			return hunks, nil
		}
		if hunk != nil {
			linesRead++ // account for the hunk header line
			hunk.StartPosition = linesRead
			hunks = append(hunks, hunk)
			linesRead += int32(bytes.Count(hunk.Body, []byte{'\n'}))
		}
		if err != nil {
			return hunks, err
		}
	}
}

// parseOnlyInMessage checks if line is a "Only in {source}: {filename}" and returns source and filename
func parseOnlyInMessage(line []byte) (bool, []byte, []byte) {
	if !bytes.HasPrefix(line, onlyInMessagePrefix) {
		return false, nil, nil
	}
	line = line[len(onlyInMessagePrefix):]
	idx := bytes.Index(line, []byte(": "))
	if idx < 0 {
		return false, nil, nil
	}
	filename := bytes.TrimSuffix(line[idx+2:], []byte("\r"))
	return true, line[:idx], filename
}

// A ParseError is a description of a unified diff syntax error.
type ParseError struct {
	Line   int   // Line where the error occurred
	Offset int64 // Offset where the error occurred
	Err    error // The actual error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("line %d, char %d: %s", e.Line, e.Offset, e.Err)
}

// ErrNoHunkHeader indicates that a unified diff hunk header was
// expected but not found during parsing.
var ErrNoHunkHeader = errors.New("no hunk header")

// ErrBadHunkHeader indicates that a malformed unified diff hunk
// header was encountered during parsing.
type ErrBadHunkHeader struct {
	header string
}

func (e *ErrBadHunkHeader) Error() string {
	if e.header == "" {
		return "bad hunk header"
	}
	return "bad hunk header: " + e.header
}

// ErrBadHunkLine is when a line not beginning with ' ', '-', '+', or
// '\' is encountered while reading a hunk. In the context of reading
// a single hunk or file, it is an unexpected error. In a multi-file
// diff, however, it indicates that the current file's diff is
// complete (and remaining diff data will describe another file
// unified diff).
type ErrBadHunkLine struct {
	Line []byte
}

func (e *ErrBadHunkLine) Error() string {
	m := "bad hunk line (does not start with ' ', '-', '+', or '\\')"
	if len(e.Line) == 0 {
		return m
	}
	return m + ": " + string(e.Line)
}
