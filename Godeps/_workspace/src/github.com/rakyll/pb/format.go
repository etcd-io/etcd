package pb

import (
	"fmt"
	"strings"
	"strconv"
)


const (
	// By default, without type handle
	U_NO = 0
	// Handle as b, Kb, Mb, etc
	U_BYTES = 1
)

// Format integer
func Format(i int64, units int) string {
	switch units {
		case U_BYTES:
			return FormatBytes(i)
	}
	// by default just convert to string
	return strconv.Itoa(int(i))
}


// Convert bytes to human readable string. Like a 2 MiB, 64.2 KiB, 52 B
func FormatBytes(i int64) (result string) {
	switch {
	case i > (1024 * 1024 * 1024 * 1024):
		result = fmt.Sprintf("%#.02f TB", float64(i)/1024/1024/1024/1024)
	case i > (1024 * 1024 * 1024):
		result = fmt.Sprintf("%#.02f GB", float64(i)/1024/1024/1024)
	case i > (1024 * 1024):
		result = fmt.Sprintf("%#.02f MB", float64(i)/1024/1024)
	case i > 1024:
		result = fmt.Sprintf("%#.02f KB", float64(i)/1024)
	default:
		result = fmt.Sprintf("%d B", i)
	}
	result = strings.Trim(result, " ")
	return
}
