/*
Package twwidth provides intelligent East Asian width detection.

In 2025/2026, most modern terminal emulators (VSCode, Windows Terminal, iTerm2,
Alacritty) and modern monospace fonts (Hack, Fira Code, Cascadia Code) treat
box-drawing characters as Single Width, regardless of the underlying OS Locale.

Detection Logic (in order of priority):
- RUNEWIDTH_EASTASIAN environment variable (explicit user override)
- Force Legacy Mode (programmatic override for backward compatibility)
- Modern environment detection (VSCode, Windows Terminal, etc. -> Narrow)
- Locale-based detection (CJK locales in traditional terminals -> Wide)

This prioritization ensures that:
- Users can always override behavior using RUNEWIDTH_EASTASIAN
- Modern development environments work correctly by default
- Traditional CJK terminals maintain compatibility via locale checks

Examples:

	// Force narrow borders (for Hack font in zh_CN)
	RUNEWIDTH_EASTASIAN=0 go run .

	// Force wide borders (for legacy CJK terminals)
	RUNEWIDTH_EASTASIAN=1 go run .
*/
package twwidth

import (
	"os"
	"runtime"
	"strings"
	"sync"
)

// Environment Variable Constants
const (
	EnvLCAll              = "LC_ALL"
	EnvLCCtype            = "LC_CTYPE"
	EnvLang               = "LANG"
	EnvRuneWidthEastAsian = "RUNEWIDTH_EASTASIAN"
	EnvTerm               = "TERM"
	EnvTermProgram        = "TERM_PROGRAM"
	EnvTermProgramWsl     = "TERM_PROGRAM_WSL"
	EnvWTProfile          = "WT_PROFILE_ID" // Windows Terminal
	EnvConEmuANSI         = "ConEmuANSI"    // ConEmu
	EnvAlacritty          = "ALACRITTY_LOG" // Alacritty
	EnvVTEVersion         = "VTE_VERSION"   // GNOME/VTE
)

const (
	overwriteOn  = "override_on"
	overwriteOff = "override_off"

	envModern = "modern_env"
	envCjk    = "locale_cjk"
	envAscii  = "default_ascii"
)

// CJK Language Codes (Prefixes)
// Covers ISO 639-1 (2-letter) and common full names used in some systems.
var cjkPrefixes = []string{
	"zh", "ja", "ko", // Standard: Chinese, Japanese, Korean
	"chi", "zho", // ISO 639-2/B and T for Chinese
	"jpn", "kor", // ISO 639-2 for Japanese, Korean
	"chinese", "japanese", "korean", // Full names (rare but possible in some legacy systems)
}

// CJK Region Codes
// Checks for specific regions that imply CJK font usage (e.g., en_HK).
var cjkRegions = map[string]bool{
	"cn": true, // China
	"tw": true, // Taiwan
	"hk": true, // Hong Kong
	"mo": true, // Macau
	"jp": true, // Japan
	"kr": true, // South Korea
	"kp": true, // North Korea
	"sg": true, // Singapore (Often uses CJK fonts)
}

// Modern environments that should use narrow borders (1-width box chars)
var modernEnvironments = map[string]bool{
	// Terminal programs
	"vscode": true, "visual studio code": true,
	"iterm.app": true, "iterm2": true,
	"windows terminal": true, "windowsterminal": true,
	"alacritty": true, "kitty": true,
	"hyper": true, "tabby": true, "terminus": true, "fluentterminal": true,
	"warp": true, "ghostty": true, "rio": true,
	"jetbrains-jediterm": true,

	// Terminal types (TERM signatures)
	"xterm-kitty": true, "xterm-ghostty": true, "wezterm": true,
}

var (
	eastAsianOnce sync.Once
	eastAsianVal  bool

	// Legacy override control
	// Renamed to cfgMu to avoid conflict with width.go's mu
	cfgMu                sync.RWMutex
	forceLegacyEastAsian = false
)

type Enviroment struct {
	GOOS                string `json:"goos"`
	LC_ALL              string `json:"lc_all"`
	LC_CTYPE            string `json:"lc_ctype"`
	LANG                string `json:"lang"`
	RUNEWIDTH_EASTASIAN string `json:"runewidth_eastasian"`
	TERM                string `json:"term"`
	TERM_PROGRAM        string `json:"term_program"`
}

// State captures the calculated internal state.
type State struct {
	NormalizedLocale   string `json:"normalized_locale"`
	IsCJKLocale        bool   `json:"is_cjk_locale"`
	IsModernEnv        bool   `json:"is_modern_env"`
	LegacyOverrideMode bool   `json:"legacy_override_mode"`
}

// Detection aggregates all debug information regarding East Asian width detection.
type Detection struct {
	AutoUseEastAsian bool       `json:"auto_use_east_asian"`
	DetectionMode    string     `json:"detection_mode"`
	Raw              Enviroment `json:"raw"`
	Derived          State      `json:"derived"`
}

// EastAsianForceLegacy forces the detection logic to ignore modern environment checks.
// It relies solely on Locale detection. This is useful for applications that need
// strict backward compatibility.
//
// Note: This does NOT override RUNEWIDTH_EASTASIAN. User environment variables take precedence.
// This should be called before the first table render.
func EastAsianForceLegacy(force bool) {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	forceLegacyEastAsian = force
}

// EastAsianDetect checks the environment variables to determine if
// East Asian width calculations should be enabled.
func EastAsianDetect() bool {
	eastAsianOnce.Do(func() {
		eastAsianVal = detectEastAsian()
	})
	return eastAsianVal
}

// EastAsianConservative is a stricter version that only defaults to Narrow
// if the terminal is definitely known to be modern (e.g. VSCode, iTerm2).
// It avoids heuristics like checking "xterm" in the TERM variable.
func EastAsianConservative() bool {
	// Check overrides first
	if val, found := checkOverrides(); found {
		return val
	}

	// Stricter modern environment detection
	if isConservativeModernEnvironment() {
		return false
	}

	// Fall back to locale
	return checkLocale()
}

// EastAsianMode returns the decision path used for the current environment.
// Useful for debugging why a specific width was chosen.
func EastAsianMode() string {
	// Check override
	if val, found := checkOverrides(); found {
		if val {
			return overwriteOn
		}
		return overwriteOff
	}

	cfgMu.RLock()
	legacy := forceLegacyEastAsian
	cfgMu.RUnlock()

	if legacy {
		if checkLocale() {
			return envCjk
		}
		return envAscii
	}

	if isModernEnvironment() {
		return envModern
	}

	if checkLocale() {
		return envCjk
	}

	return envAscii
}

// Debugging returns detailed information about the detection decision.
// Useful for users to include in Github issues.
func Debugging() Detection {
	locale := getNormalizedLocale()

	cfgMu.RLock()
	legacy := forceLegacyEastAsian
	cfgMu.RUnlock()

	return Detection{
		AutoUseEastAsian: EastAsianDetect(),
		DetectionMode:    EastAsianMode(),
		Raw: Enviroment{
			GOOS:                runtime.GOOS,
			LC_ALL:              os.Getenv(EnvLCAll),
			LC_CTYPE:            os.Getenv(EnvLCCtype),
			LANG:                os.Getenv(EnvLang),
			RUNEWIDTH_EASTASIAN: os.Getenv(EnvRuneWidthEastAsian),
			TERM:                os.Getenv(EnvTerm),
			TERM_PROGRAM:        os.Getenv(EnvTermProgram),
		},
		Derived: State{
			NormalizedLocale:   locale,
			IsCJKLocale:        isCJKLocale(locale),
			IsModernEnv:        isModernEnvironment(),
			LegacyOverrideMode: legacy,
		},
	}
}

// detectEastAsian evaluates the environment and locale settings to determine if East Asian width rules should apply.
func detectEastAsian() bool {
	// User Override check (Highest Priority)
	if val, found := checkOverrides(); found {
		return val
	}

	// Force Legacy Mode check
	cfgMu.RLock()
	isLegacy := forceLegacyEastAsian
	cfgMu.RUnlock()

	if isLegacy {
		// Legacy mode ignores modern environment checks,
		// relying solely on locale.
		return checkLocale()
	}

	// Modern Environment Detection
	// If modern, we assume Single Width (return false)
	if isModernEnvironment() {
		return false
	}

	// 4. Locale Fallback
	return checkLocale()
}

// checkOverrides looks for RUNEWIDTH_EASTASIAN
func checkOverrides() (bool, bool) {
	if rw := os.Getenv(EnvRuneWidthEastAsian); rw != "" {
		rw = strings.ToLower(rw)
		if rw == "0" || rw == "off" || rw == "false" || rw == "no" {
			return false, true
		}
		if rw == "1" || rw == "on" || rw == "true" || rw == "yes" {
			return true, true
		}
	}
	return false, false
}

// checkLocale performs the string analysis on LANG/LC_ALL
func checkLocale() bool {
	locale := getNormalizedLocale()
	if locale == "" {
		return false
	}
	return isCJKLocale(locale)
}

// isModernEnvironment performs comprehensive checks for modern terminal capabilities.
func isModernEnvironment() bool {
	// Check TERM_PROGRAM (Most reliable)
	if termProg := os.Getenv(EnvTermProgram); termProg != "" {
		termProgLower := strings.ToLower(termProg)
		if modernEnvironments[termProgLower] {
			return true
		}
	}

	// Check WSL specific variable
	if os.Getenv(EnvTermProgramWsl) != "" {
		return true
	}

	// Windows Specifics
	if runtime.GOOS == "windows" {
		// Windows Terminal
		if os.Getenv(EnvWTProfile) != "" {
			return true
		}
		// ConEmu/Cmder
		if os.Getenv(EnvConEmuANSI) == "ON" {
			return true
		}
		// Modern Windows console (Windows 10+) check via TERM
		if term := os.Getenv(EnvTerm); term != "" {
			termLower := strings.ToLower(term)
			if strings.Contains(termLower, "xterm") ||
				strings.Contains(termLower, "vt") {
				return true
			}
		}
	}

	// VTE-based terminals (GNOME Terminal, Tilix, etc.)
	if os.Getenv(EnvVTEVersion) != "" {
		return true
	}

	// Check for Alacritty specifically
	if os.Getenv(EnvAlacritty) != "" {
		return true
	}

	// Check TERM for modern terminal signatures
	if term := os.Getenv(EnvTerm); term != "" {
		termLower := strings.ToLower(term)
		// Specific modern terminals often put their name in TERM
		if modernEnvironments[termLower] {
			return true
		}
		// Heuristics for standard modern-capable descriptors
		if strings.Contains(termLower, "xterm") && !strings.Contains(termLower, "xterm-mono") {
			return true
		}
		if strings.Contains(termLower, "screen") ||
			strings.Contains(termLower, "tmux") {
			return true
		}
	}

	return false
}

// isConservativeModernEnvironment performs strict checks only for known modern terminals.
func isConservativeModernEnvironment() bool {
	termProg := strings.ToLower(os.Getenv(EnvTermProgram))

	// Allow-list of definitely modern terminals
	switch termProg {
	case "vscode", "visual studio code":
		return true
	case "iterm.app", "iterm2":
		return true
	case "windows terminal", "windowsterminal":
		return true
	case "alacritty", "wezterm", "kitty", "ghostty":
		return true
	case "warp", "tabby", "hyper":
		return true
	}

	// Windows Terminal via specific Env
	if os.Getenv(EnvWTProfile) != "" {
		return true
	}

	return false
}

// isCJKLocale determines if a given locale string corresponds to a CJK (Chinese, Japanese, Korean) language or region.
func isCJKLocale(locale string) bool {
	// Check Language Prefix
	for _, prefix := range cjkPrefixes {
		if strings.HasPrefix(locale, prefix) {
			return true
		}
	}

	// Check Regions
	parts := strings.Split(locale, "_")
	if len(parts) > 1 {
		for _, part := range parts[1:] {
			if cjkRegions[part] {
				return true
			}
		}
	}

	return false
}

// getNormalizedLocale returns the normalized locale by inspecting environment variables LC_ALL, LC_CTYPE, and LANG.
func getNormalizedLocale() string {
	var locale string
	if loc := os.Getenv(EnvLCAll); loc != "" {
		locale = loc
	} else if loc := os.Getenv(EnvLCCtype); loc != "" {
		locale = loc
	} else if loc := os.Getenv(EnvLang); loc != "" {
		locale = loc
	}

	// Fast fail for empty or standard C/POSIX locales
	if locale == "" || locale == "C" || locale == "POSIX" {
		return ""
	}

	// Strip encoding and modifiers
	if idx := strings.IndexByte(locale, '.'); idx != -1 {
		locale = locale[:idx]
	}
	if idx := strings.IndexByte(locale, '@'); idx != -1 {
		locale = locale[:idx]
	}

	return strings.ToLower(locale)
}
