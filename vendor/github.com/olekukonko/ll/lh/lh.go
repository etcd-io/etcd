package lh

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/cespare/xxhash/v2"
	"github.com/olekukonko/ll/lx"
)

// rightPad pads a string with spaces on the right to reach the specified length.
// Returns the original string if it's already at or exceeds the target length.
// Uses strings.Builder for efficient memory allocation.
func rightPad(str string, length int) string {
	if len(str) >= length {
		return str
	}
	var sb strings.Builder
	sb.Grow(length)
	sb.WriteString(str)
	sb.WriteString(strings.Repeat(" ", length-len(str)))
	return sb.String()
}

var dedupBufPool = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
}

// defaultDedupKey generates a deduplication key from log level and message.
// Uses FNV-1a hash for speed and good distribution. Override with WithDedupKeyFunc
// to include additional fields like namespace, caller, or structured fields.
func defaultDedupKey(e *lx.Entry) uint64 {
	h := xxhash.New()

	_, _ = h.Write([]byte(e.Level.String()))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(e.Message))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(e.Namespace))
	_, _ = h.Write([]byte{0})

	if len(e.Fields) > 0 {
		m := e.Fields.Map()
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		buf := dedupBufPool.Get().(*bytes.Buffer)
		buf.Reset()
		defer dedupBufPool.Put(buf)

		for _, k := range keys {
			fmt.Fprint(buf, k)
			buf.WriteByte('=')
			fmt.Fprint(buf, m[k])
			buf.WriteByte(0)
		}
		_, _ = h.Write(buf.Bytes())
	}

	return h.Sum64()
}
