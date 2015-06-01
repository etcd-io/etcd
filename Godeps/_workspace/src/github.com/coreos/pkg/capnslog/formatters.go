package capnslog

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"time"
)

type Formatter interface {
	Format(pkg string, level LogLevel, depth int, entries ...interface{})
	Flush()
}

func NewStringFormatter(w io.Writer) *StringFormatter {
	return &StringFormatter{
		w: bufio.NewWriter(w),
	}
}

type StringFormatter struct {
	w *bufio.Writer
}

func (s *StringFormatter) Format(pkg string, l LogLevel, i int, entries ...interface{}) {
	now := time.Now()
	y, m, d := now.Date()
	h, min, sec := now.Clock()
	s.w.WriteString(fmt.Sprintf("%d/%02d/%d %02d:%02d:%02d ", y, m, d, h, min, sec))
	s.writeEntries(pkg, l, i, entries...)
}

func (s *StringFormatter) writeEntries(pkg string, _ LogLevel, _ int, entries ...interface{}) {
	if pkg != "" {
		s.w.WriteString(pkg + ": ")
	}
	str := fmt.Sprint(entries...)
	endsInNL := strings.HasSuffix(str, "\n")
	s.w.WriteString(str)
	if !endsInNL {
		s.w.WriteString("\n")
	}
	s.Flush()
}

func (s *StringFormatter) Flush() {
	s.w.Flush()
}
