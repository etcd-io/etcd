package twwidth

import (
	"bytes"
	"regexp"
	"strings"
	"sync"

	"github.com/clipperhouse/displaywidth"
	"github.com/mattn/go-runewidth"
	"github.com/olekukonko/tablewriter/pkg/twcache"
)

const (
	cacheCapacity = 8192

	cachePrefix          = "0:"
	cacheEastAsianPrefix = "1:"
)

// Options allows for configuring width calculation on a per-call basis.
type Options struct {
	EastAsianWidth bool

	// Explicitly force box drawing chars to be narrow
	// regardless of EastAsianWidth setting.
	ForceNarrowBorders bool
}

// globalOptions holds the global displaywidth configuration, including East Asian width settings.
var globalOptions Options

// mu protects access to globalOptions for thread safety.
var mu sync.Mutex

// ansi is a compiled regular expression for stripping ANSI escape codes from strings.
var ansi = Filter()

func init() {
	isEastAsian := EastAsianDetect()

	cond := runewidth.NewCondition()
	cond.EastAsianWidth = isEastAsian

	globalOptions = Options{
		EastAsianWidth: isEastAsian,

		// Auto-enable ForceNarrowBorders for edge cases.
		// If EastAsianWidth is ON (e.g. forced via Env Var), but we detect
		// a modern environment, we might technically want to narrow borders
		// while keeping text wide.
		ForceNarrowBorders: isEastAsian && isModernEnvironment(),
	}

	widthCache = twcache.NewLRU[cacheKey, int](cacheCapacity)
}

// Display calculates the visual width of a string using a specific runewidth.Condition.
// Deprecated: use WidthWithOptions with the new twwidth.Options struct instead.
// This function is kept for backward compatibility.
func Display(cond *runewidth.Condition, str string) int {
	opts := Options{EastAsianWidth: cond.EastAsianWidth}
	return WidthWithOptions(str, opts)
}

// Filter compiles and returns a regular expression for matching ANSI escape sequences,
// including CSI (Control Sequence Introducer) and OSC (Operating System Command) sequences.
// The returned regex can be used to strip ANSI codes from strings.
func Filter() *regexp.Regexp {
	regESC := "\x1b" // ASCII escape character
	regBEL := "\x07" // ASCII bell character

	// ANSI string terminator: either ESC+\ or BEL
	regST := "(" + regexp.QuoteMeta(regESC+"\\") + "|" + regexp.QuoteMeta(regBEL) + ")"
	// Control Sequence Introducer (CSI): ESC[ followed by parameters and a final byte
	regCSI := regexp.QuoteMeta(regESC+"[") + "[\x30-\x3f]*[\x20-\x2f]*[\x40-\x7e]"
	// Operating System Command (OSC): ESC] followed by arbitrary content until a terminator
	regOSC := regexp.QuoteMeta(regESC+"]") + ".*?" + regST

	// Combine CSI and OSC patterns into a single regex
	return regexp.MustCompile("(" + regCSI + "|" + regOSC + ")")
}

// GetCacheStats returns current cache statistics
func GetCacheStats() (size, capacity int, hitRate float64) {
	mu.Lock()
	defer mu.Unlock()

	if widthCache == nil {
		return 0, 0, 0
	}
	return widthCache.Len(), widthCache.Cap(), widthCache.HitRate()
}

// IsEastAsian returns the current East Asian width setting.
// This function is thread-safe.
//
// Example:
//
//	if twdw.IsEastAsian() {
//		// Handle East Asian width characters
//	}
func IsEastAsian() bool {
	mu.Lock()
	defer mu.Unlock()
	return globalOptions.EastAsianWidth
}

// SetCondition sets the global East Asian width setting based on a runewidth.Condition.
// Deprecated: use SetOptions with the new twwidth.Options struct instead.
// This function is kept for backward compatibility.
func SetCondition(cond *runewidth.Condition) {
	mu.Lock()
	defer mu.Unlock()
	newEastAsianWidth := cond.EastAsianWidth
	if globalOptions.EastAsianWidth != newEastAsianWidth {
		globalOptions.EastAsianWidth = newEastAsianWidth
		widthCache.Purge()
	}
}

// SetEastAsian enables or disables East Asian width handling globally.
// This function is thread-safe.
//
// Example:
//
//	twdw.SetEastAsian(true) // Enable East Asian width handling
func SetEastAsian(enable bool) {
	SetOptions(Options{EastAsianWidth: enable})
}

// SetForceNarrow to preserve the new flag, or create a new setter
func SetForceNarrow(enable bool) {
	mu.Lock()
	defer mu.Unlock()
	globalOptions.ForceNarrowBorders = enable
	widthCache.Purge() // Clear cache because widths might change
}

// SetOptions sets the global options for width calculation.
// This function is thread-safe.
func SetOptions(opts Options) {
	mu.Lock()
	defer mu.Unlock()
	if globalOptions.EastAsianWidth != opts.EastAsianWidth || globalOptions.ForceNarrowBorders != opts.ForceNarrowBorders {
		globalOptions = opts
		widthCache.Purge()
	}
}

// Truncate shortens a string to fit within a specified visual width, optionally
// appending a suffix (e.g., "..."). It preserves ANSI escape sequences and adds
// a reset sequence (\x1b[0m) if needed to prevent formatting bleed. The function
// respects the global East Asian width setting and is thread-safe.
//
// If maxWidth is negative, an empty string is returned. If maxWidth is zero and
// a suffix is provided, the suffix is returned. If the string's visual width is
// less than or equal to maxWidth, the string (and suffix, if provided and fits)
// is returned unchanged.
//
// Example:
//
//	s := twdw.Truncate("Hello\x1b[31mWorld", 5, "...") // Returns "Hello..."
//	s = twdw.Truncate("Hello", 10) // Returns "Hello"
func Truncate(s string, maxWidth int, suffix ...string) string {
	if maxWidth < 0 {
		return ""
	}

	suffixStr := strings.Join(suffix, "")
	sDisplayWidth := Width(s)              // Uses global cached Width
	suffixDisplayWidth := Width(suffixStr) // Uses global cached Width

	// Case 1: Original string is visually empty.
	if sDisplayWidth == 0 {
		// If suffix is provided and fits within maxWidth (or if maxWidth is generous)
		if len(suffixStr) > 0 && suffixDisplayWidth <= maxWidth {
			return suffixStr
		}
		// If s has ANSI codes (len(s)>0) but maxWidth is 0, can't display them.
		if maxWidth == 0 && len(s) > 0 {
			return ""
		}
		return s // Returns "" or original ANSI codes
	}

	// Case 2: maxWidth is 0, but string has content. Cannot display anything.
	if maxWidth == 0 {
		return ""
	}

	// Case 3: String fits completely or fits with suffix.
	// Here, maxWidth is the total budget for the line.
	if sDisplayWidth <= maxWidth {
		// If the string contains ANSI, we must ensure it ends with a reset
		// to prevent bleeding, even if we don't truncate.
		safeS := s
		if strings.Contains(s, "\x1b") && !strings.HasSuffix(s, "\x1b[0m") {
			safeS += "\x1b[0m"
		}

		if len(suffixStr) == 0 { // No suffix.
			return safeS
		}
		// Suffix is provided. Check if s + suffix fits.
		if sDisplayWidth+suffixDisplayWidth <= maxWidth {
			return safeS + suffixStr
		}
		// s fits, but s + suffix is too long. Return s (with reset if needed).
		return safeS
	}

	// Case 4: String needs truncation (sDisplayWidth > maxWidth).
	// maxWidth is the total budget for the final string (content + suffix).
	mu.Lock()
	currentOpts := globalOptions
	mu.Unlock()

	// Special case for EastAsianDetect true: if only suffix fits, return suffix.
	// This was derived from previous test behavior.
	if len(suffixStr) > 0 && currentOpts.EastAsianWidth {
		provisionalContentWidth := maxWidth - suffixDisplayWidth
		if provisionalContentWidth == 0 { // Exactly enough space for suffix only
			return suffixStr
		}
	}

	// Calculate the budget for the content part, reserving space for the suffix.
	targetContentForIteration := maxWidth
	if len(suffixStr) > 0 {
		targetContentForIteration -= suffixDisplayWidth
	}

	// If content budget is negative, means not even suffix fits (or no suffix and no space).
	// However, if only suffix fits, it should be handled.
	if targetContentForIteration < 0 {
		// Can we still fit just the suffix?
		if len(suffixStr) > 0 && suffixDisplayWidth <= maxWidth {
			if strings.Contains(s, "\x1b[") {
				return "\x1b[0m" + suffixStr
			}
			return suffixStr
		}
		return "" // Cannot fit anything.
	}

	var contentBuf bytes.Buffer
	var currentContentDisplayWidth int
	var ansiSeqBuf bytes.Buffer
	inAnsiSequence := false
	ansiWrittenToContent := false

	for _, r := range s {
		if r == '\x1b' {
			inAnsiSequence = true
			ansiSeqBuf.Reset()
			ansiSeqBuf.WriteRune(r)
		} else if inAnsiSequence {
			ansiSeqBuf.WriteRune(r)
			seqBytes := ansiSeqBuf.Bytes()
			seqLen := len(seqBytes)
			terminated := false
			if seqLen >= 2 {
				introducer := seqBytes[1]
				switch introducer {
				case '[':
					if seqLen >= 3 && r >= 0x40 && r <= 0x7E {
						terminated = true
					}
				case ']':
					if r == '\x07' {
						terminated = true
					} else if seqLen > 1 && seqBytes[seqLen-2] == '\x1b' && r == '\\' { // Check for ST: \x1b\
						terminated = true
					}
				}
			}
			if terminated {
				inAnsiSequence = false
				contentBuf.Write(ansiSeqBuf.Bytes())
				ansiWrittenToContent = true
				ansiSeqBuf.Reset()
			}
		} else { // Normal character
			runeDisplayWidth := calculateRunewidth(r, currentOpts)
			if targetContentForIteration == 0 { // No budget for content at all
				break
			}
			if currentContentDisplayWidth+runeDisplayWidth > targetContentForIteration {
				break
			}
			contentBuf.WriteRune(r)
			currentContentDisplayWidth += runeDisplayWidth
		}
	}

	result := contentBuf.String()

	// Determine if we need to append a reset sequence to prevent color bleeding.
	// This is needed if we wrote any ANSI codes or if the input had active codes
	// that we might have cut off or left open.
	needsReset := false
	if (ansiWrittenToContent || (inAnsiSequence && strings.Contains(s, "\x1b["))) && (currentContentDisplayWidth > 0 || ansiWrittenToContent) {
		if !strings.HasSuffix(result, "\x1b[0m") {
			needsReset = true
		}
	} else if currentContentDisplayWidth > 0 && strings.Contains(result, "\x1b[") && !strings.HasSuffix(result, "\x1b[0m") && strings.Contains(s, "\x1b[") {
		needsReset = true
	}

	if needsReset {
		result += "\x1b[0m"
	}

	// Suffix is added if provided.
	if len(suffixStr) > 0 {
		result += suffixStr
	}
	return result
}

// Width calculates the visual width of a string using the global cache for performance.
// It excludes ANSI escape sequences and accounts for the global East Asian width setting.
// This function is thread-safe.
//
// Example:
//
//	width := twdw.Width("Hello\x1b[31mWorld") // Returns 10
func Width(str string) int {
	// Fast path ASCII (Optimization)
	if len(str) == 1 && str[0] < 0x80 {
		// Treat tab as special case even in fast path
		if IsTab(rune(str[0])) {
			return TabWidth()
		}
		return 1
	}

	mu.Lock()
	currentOpts := globalOptions
	mu.Unlock()

	key := cacheKey{
		eastAsian: currentOpts.EastAsianWidth,
		str:       str,
	}

	// Check Cache (Optimization)
	if w, found := widthCache.Get(key); found {
		return w
	}

	//stripped := ansi.ReplaceAllLiteralString(str, "")
	calculatedWidth := 0

	for _, r := range strip(str) {
		calculatedWidth += calculateRunewidth(r, currentOpts)
	}

	// Store in Cache
	widthCache.Add(key, calculatedWidth)
	return calculatedWidth
}

// WidthNoCache calculates the visual width of a string without using the global cache.
//
// Example:
//
//	width := twdw.WidthNoCache("Hello\x1b[31mWorld") // Returns 10
func WidthNoCache(str string) int {
	// This function's behavior is equivalent to a one-shot calculation
	// using the current global options. The WidthWithOptions function
	// does not interact with the cache, thus fulfilling the requirement.
	mu.Lock()
	opts := globalOptions
	mu.Unlock()
	return WidthWithOptions(str, opts)
}

// WidthWithOptions calculates the visual width of a string with specific options,
// bypassing the global settings and cache. This is useful for one-shot calculations
// where global state is not desired.
func WidthWithOptions(str string, opts Options) int {
	// stripped := ansi.ReplaceAllLiteralString(str, "")
	calculatedWidth := 0
	for _, r := range strip(str) {
		calculatedWidth += calculateRunewidth(r, opts)
	}
	return calculatedWidth
}

// calculateRunewidth calculates the width of a single rune based on the provided options.
// It applies narrow overrides for box drawing characters if configured and handles Tabs.
func calculateRunewidth(r rune, opts Options) int {
	if opts.ForceNarrowBorders && isBoxDrawingChar(r) {
		return 1
	}

	// Explicitly handle Tabinal to ensure tables have enough space
	// when TrimTab is Off.
	if IsTab(r) {
		return TabWidth()
	}

	dwOpts := displaywidth.Options{EastAsianWidth: opts.EastAsianWidth}
	return dwOpts.Rune(r)
}

// isBoxDrawingChar checks if a rune is within the Unicode Box Drawing range.
func isBoxDrawingChar(r rune) bool {
	return r >= 0x2500 && r <= 0x257F
}

func strip(s string) string {
	if strings.IndexByte(s, '\x1b') == -1 {
		return s
	}
	return ansi.ReplaceAllLiteralString(s, "")
}
