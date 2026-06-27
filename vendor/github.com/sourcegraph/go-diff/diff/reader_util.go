package diff

import (
	"bufio"
	"bytes"
	"errors"
	"io"
)

var ErrLineReaderUninitialized = errors.New("line reader not initialized")

func newLineReader(r io.Reader) *lineReader {
	return &lineReader{reader: bufio.NewReader(r)}
}

func newLineReaderOptions(r io.Reader, opts ParseOptions) *lineReader {
	return &lineReader{
		reader: bufio.NewReader(r),
		keepCR: opts.KeepCR,
	}
}

// lineReader is a wrapper around a bufio.Reader that caches the next line to
// provide lookahead functionality for the next two lines.
type lineReader struct {
	reader *bufio.Reader

	cachedNextLine    []byte
	cachedNextLineErr error

	keepCR bool
}

func (l *lineReader) ensureCachedNextLine() {
	if l.cachedNextLine == nil && l.cachedNextLineErr == nil {
		l.cachedNextLine, l.cachedNextLineErr = readLine(l.reader, l.keepCR)
	}
}

// readLine returns the next unconsumed line and advances the internal cache of
// the lineReader.
func (l *lineReader) readLine() ([]byte, error) {
	l.ensureCachedNextLine()

	if l.cachedNextLineErr != nil {
		return nil, l.cachedNextLineErr
	}

	next := l.cachedNextLine

	l.cachedNextLine, l.cachedNextLineErr = readLine(l.reader, l.keepCR)

	return next, nil
}

// nextLineStartsWith looks at the line that would be returned by the next call
// to readLine to check whether it has the given prefix.
//
// io.EOF and bufio.ErrBufferFull errors are ignored so that the function can
// be used when at the end of the file.
func (l *lineReader) nextLineStartsWith(prefix string) (bool, error) {
	l.ensureCachedNextLine()

	return l.lineHasPrefix(l.cachedNextLine, prefix, l.cachedNextLineErr)
}

// nextNextLineStartsWith checks the prefix of the line *after* the line that
// would be returned by the next readLine.
//
// io.EOF and bufio.ErrBufferFull errors are ignored so that the function can
// be used when at the end of the file.
func (l *lineReader) nextNextLineStartsWith(prefix string) (bool, error) {
	l.ensureCachedNextLine()

	next, err := l.reader.Peek(len(prefix))
	return l.lineHasPrefix(next, prefix, err)
}

// lineHasPrefix checks whether the given line has the given prefix with
// bytes.HasPrefix.
//
// The readErr should be the error that was returned when the line was read.
// lineHasPrefix checks the error to adjust its return value to, e.g., return
// false and ignore the error when readErr is io.EOF.
func (l *lineReader) lineHasPrefix(line []byte, prefix string, readErr error) (bool, error) {
	if readErr != nil {
		if readErr == io.EOF || readErr == bufio.ErrBufferFull {
			return false, nil
		}
		return false, readErr
	}

	return bytes.HasPrefix(line, []byte(prefix)), nil
}

// readLine is a helper that mimics the functionality of calling bufio.Scanner.Scan() and
// bufio.Scanner.Bytes(), but without the token size limitation. It will read and return
// the next line in the Reader with the trailing newline stripped. It will return an
// io.EOF error when there is nothing left to read (at the start of the function call). It
// will return any other errors it receives from the underlying call to ReadBytes.
func readLine(r *bufio.Reader, keepCR bool) ([]byte, error) {
	line, err := r.ReadBytes('\n')
	if err == io.EOF && len(line) == 0 {
		return nil, io.EOF
	}
	if err != nil && err != io.EOF {
		return nil, err
	}
	if line[len(line)-1] == '\n' {
		line = line[:len(line)-1]
	}
	if !keepCR {
		return dropCR(line), nil
	}
	return line, nil
}

// dropCR drops a terminal \r from the data.
func dropCR(data []byte) []byte {
	if len(data) > 0 && data[len(data)-1] == '\r' {
		return data[0 : len(data)-1]
	}
	return data
}
