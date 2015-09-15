package pb

import (
	"fmt"
	"strconv"
	"strings"
)

type Units int

const (
	// By default, without type handle
	U_NO Units = iota
	// Handle as b, Kb, Mb, etc
	U_BYTES
)

// Format integer
func Format(i int64, units Units) string {
	switch units {
	case U_BYTES:
		return FormatBytes(i)
	default:
		// by default just convert to string
		return strconv.FormatInt(i, 10)
	}
}

// Convert bytes to human readable string. Like a 2 MB, 64.2 KB, 52 B
func FormatBytes(i int64) (result string) {
	switch {
	case i > (1024 * 1024 * 1024 * 1024):
		result = fmt.Sprintf("%.02f TB", float64(i)/1024/1024/1024/1024)
	case i > (1024 * 1024 * 1024):
		result = fmt.Sprintf("%.02f GB", float64(i)/1024/1024/1024)
	case i > (1024 * 1024):
		result = fmt.Sprintf("%.02f MB", float64(i)/1024/1024)
	case i > 1024:
		result = fmt.Sprintf("%.02f KB", float64(i)/1024)
	default:
		result = fmt.Sprintf("%d B", i)
	}
	result = strings.Trim(result, " ")
	return
}
