package ll

import "github.com/olekukonko/ll/lx"

// defaultStore is the global namespace store for enable/disable states.
// It is shared across all Logger instances to manage namespace hierarchy consistently.
// Thread-safe via the lx.Namespace struct’s sync.Map.
var defaultStore = &lx.Namespace{}

// systemActive indicates if the global logging system is active.
// Defaults to true, meaning logging is active unless explicitly shut down.
// Or, default to false and require an explicit ll.Start(). Let's default to true for less surprise.
var systemActive int32 = 1 // 1 for true, 0 for false (for atomic operations)

// Option defines a functional option for configuring a Logger.
type Option func(*Logger)

// reverseString reverses the input string by swapping characters from both ends.
// It converts the string to a rune slice to handle Unicode characters correctly,
// ensuring proper reversal for multi-byte characters.
// Used internally for string manipulation, such as in debugging or log formatting.
func reverseString(s string) string {
	// Convert string to rune slice to handle Unicode characters
	r := []rune(s)
	// Iterate over half the slice, swapping characters from start and end
	for i, j := 0, len(r)-1; i < len(r)/2; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	// Convert rune slice back to string and return
	return string(r)
}

// viewString converts a byte slice to a printable string, replacing non-printable
// characters (ASCII < 32 or > 126) with a dot ('.').
// It ensures safe display of binary data in logs, such as in the Dump method.
// Used for formatting binary data in a human-readable hex/ASCII dump.
func viewString(b []byte) string {
	// Convert byte slice to rune slice via string for processing
	r := []rune(string(b))
	// Replace non-printable characters with '.'
	for i := range r {
		if r[i] < 32 || r[i] > 126 {
			r[i] = '.'
		}
	}
	// Return the resulting printable string
	return string(r)
}
