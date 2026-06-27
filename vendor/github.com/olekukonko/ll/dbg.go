package ll

import (
	"container/list"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/olekukonko/ll/lx"
)

// -----------------------------------------------------------------------------
// Global Cache Instance
// -----------------------------------------------------------------------------

// sourceCache caches up to 128 source files using LRU eviction.
var sourceCache = newFileLRU(128)

// -----------------------------------------------------------------------------
// File-Level LRU Cache
// -----------------------------------------------------------------------------

type fileLRU struct {
	capacity int
	mu       sync.Mutex
	list     *list.List
	items    map[string]*list.Element
}

type fileItem struct {
	key   string
	lines []string
}

func newFileLRU(capacity int) *fileLRU {
	if capacity <= 0 {
		capacity = 1
	}
	return &fileLRU{
		capacity: capacity,
		list:     list.New(),
		items:    make(map[string]*list.Element, capacity),
	}
}

// getLine retrieves a specific 1-indexed line from a file.
func (c *fileLRU) getLine(file string, line int) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 1. Cache Hit
	if elem, ok := c.items[file]; ok {
		c.list.MoveToFront(elem)
		item := elem.Value.(*fileItem)
		if item.lines == nil {
			return "", false
		}
		return nthLine(item.lines, line)
	}

	// 2. Cache Miss - Read File
	// Release lock during I/O to avoid blocking other loggers
	c.mu.Unlock()
	data, err := os.ReadFile(file)
	c.mu.Lock()

	// 3. Double-check (another goroutine might have loaded it while unlocked)
	if elem, ok := c.items[file]; ok {
		c.list.MoveToFront(elem)
		item := elem.Value.(*fileItem)
		if item.lines == nil {
			return "", false
		}
		return nthLine(item.lines, line)
	}

	var lines []string
	if err == nil {
		lines = strings.Split(string(data), "\n")
	}

	// 4. Store (Positive or Negative Cache)
	item := &fileItem{
		key:   file,
		lines: lines,
	}
	elem := c.list.PushFront(item)
	c.items[file] = elem

	// 5. Evict if needed
	if c.list.Len() > c.capacity {
		old := c.list.Back()
		if old != nil {
			c.list.Remove(old)
			delete(c.items, old.Value.(*fileItem).key)
		}
	}

	if lines == nil {
		return "", false
	}
	return nthLine(lines, line)
}

// nthLine returns the 1-indexed line from slice.
func nthLine(lines []string, n int) (string, bool) {
	if n <= 0 || n > len(lines) {
		return "", false
	}
	return strings.TrimSuffix(lines[n-1], "\r"), true
}

// -----------------------------------------------------------------------------
// Logger Debug Implementation
// -----------------------------------------------------------------------------

// Dbg logs debug information including source file, line number,
// and the best-effort extracted expression.
//
// Example:
//
//	x := 42
//	logger.Dbg("val", x)
//	Output: [file.go:123] "val" = "val", x = 42
func (l *Logger) Dbg(values ...interface{}) {
	if !l.shouldLog(lx.LevelInfo) {
		return
	}
	l.dbg(2, values...)
}

func (l *Logger) dbg(skip int, values ...interface{}) {
	file, line, ok := callerFrame(skip)
	if !ok {
		// Fallback if we can't get frame
		var sb strings.Builder
		sb.WriteString("[?:?] ")
		for i, v := range values {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf("%+v", v))
		}
		l.log(lx.LevelInfo, lx.ClassText, sb.String(), nil, false)
		return
	}

	shortFile := file
	if idx := strings.LastIndex(file, "/"); idx >= 0 {
		shortFile = file[idx+1:]
	}

	srcLine, hit := sourceCache.getLine(file, line)

	var expr string
	if hit && srcLine != "" {
		// Attempt to extract the text inside Dbg(...)
		if a := strings.Index(srcLine, "Dbg("); a >= 0 {
			rest := srcLine[a+len("Dbg("):]
			if b := strings.LastIndex(rest, ")"); b >= 0 {
				expr = strings.TrimSpace(rest[:b])
			}
		} else {
			// Fallback: extract first (...) group if Dbg isn't explicit prefix
			a := strings.Index(srcLine, "(")
			b := strings.LastIndex(srcLine, ")")
			if a >= 0 && b > a {
				expr = strings.TrimSpace(srcLine[a+1 : b])
			}
		}
	}

	// Format output
	var outBuilder strings.Builder
	outBuilder.WriteString(fmt.Sprintf("[%s:%d] ", shortFile, line))

	// Attempt to split expressions to map 1:1 with values
	var parts []string
	if expr != "" {
		parts = splitExpressions(expr)
	}

	// If the number of extracted expressions matches the number of values,
	// print them as "expr = value". Otherwise, fall back to "expr = val1, val2".
	if len(parts) == len(values) {
		for i, v := range values {
			if i > 0 {
				outBuilder.WriteString(", ")
			}
			outBuilder.WriteString(fmt.Sprintf("%s = %+v", parts[i], v))
		}
	} else {
		if expr != "" {
			outBuilder.WriteString(expr)
			outBuilder.WriteString(" = ")
		}
		for i, v := range values {
			if i > 0 {
				outBuilder.WriteString(", ")
			}
			outBuilder.WriteString(fmt.Sprintf("%+v", v))
		}
	}

	l.log(lx.LevelInfo, lx.ClassDbg, outBuilder.String(), nil, false)
}

// splitExpressions splits a comma-separated string of expressions,
// respecting nested parentheses, brackets, braces, and quotes.
// Example: "a, fn(b, c), d" -> ["a", "fn(b, c)", "d"]
func splitExpressions(s string) []string {
	var parts []string
	var current strings.Builder
	depth := 0       // Tracks nested (), [], {}
	inQuote := false // Tracks string literals
	var quoteChar rune

	for _, r := range s {
		switch {
		case inQuote:
			current.WriteRune(r)
			if r == quoteChar {
				// We rely on the fact that valid Go source won't have unescaped quotes easily
				// accessible here without complex parsing, but for simple Dbg calls this suffices.
				// A robust parser handles `\"`, but simple state toggling covers 99% of debug cases.
				inQuote = false
			}
		case r == '"' || r == '\'':
			inQuote = true
			quoteChar = r
			current.WriteRune(r)
		case r == '(' || r == '{' || r == '[':
			depth++
			current.WriteRune(r)
		case r == ')' || r == '}' || r == ']':
			depth--
			current.WriteRune(r)
		case r == ',' && depth == 0:
			// Split point
			parts = append(parts, strings.TrimSpace(current.String()))
			current.Reset()
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, strings.TrimSpace(current.String()))
	}
	return parts
}

// -----------------------------------------------------------------------------
// Caller Resolution
// -----------------------------------------------------------------------------

// callerFrame walks stack frames until it finds the first frame
// outside the ll package.
func callerFrame(skip int) (file string, line int, ok bool) {
	// +2 to skip callerFrame + dbg itself.
	pcs := make([]uintptr, 32)
	n := runtime.Callers(skip+2, pcs)
	if n == 0 {
		return "", 0, false
	}

	frames := runtime.CallersFrames(pcs[:n])
	for {
		fr, more := frames.Next()
		// fr.Function looks like: "github.com/you/mod/ll.(*Logger).Dbg"
		// We want the first frame that is NOT inside package ll.
		if fr.Function == "" || !strings.Contains(fr.Function, "/ll.") && !strings.Contains(fr.Function, ".ll.") {
			return fr.File, fr.Line, true
		}

		if !more {
			// Fallback: return the last frame we saw
			return fr.File, fr.Line, fr.File != ""
		}
	}
}
