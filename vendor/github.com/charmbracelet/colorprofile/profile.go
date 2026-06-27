package colorprofile

import (
	"image/color"
	"sync"

	"github.com/charmbracelet/x/ansi"
)

// Profile is a color profile: NoTTY, Ascii, ANSI, ANSI256, or TrueColor.
type Profile byte

const (
	// Unknown is a profile that represents the absence of a profile.
	Unknown Profile = iota
	// NoTTY is a profile with no terminal support.
	NoTTY
	// ASCII is a profile with no color support.
	ASCII
	// ANSI is a profile with 16 colors (4-bit).
	ANSI
	// ANSI256 is a profile with 256 colors (8-bit).
	ANSI256
	// TrueColor is a profile with 16 million colors (24-bit).
	TrueColor
)

// Ascii is an alias for the [ASCII] profile for backwards compatibility.
const Ascii = ASCII //nolint:revive

// String returns the string representation of a Profile.
func (p Profile) String() string {
	switch p {
	case TrueColor:
		return "TrueColor"
	case ANSI256:
		return "ANSI256"
	case ANSI:
		return "ANSI"
	case ASCII:
		return "Ascii"
	case NoTTY:
		return "NoTTY"
	default:
		return "Unknown"
	}
}

var (
	cache = map[Profile]map[color.Color]color.Color{
		ANSI256: {},
		ANSI:    {},
	}
	mu sync.RWMutex
)

// Convert transforms a given Color to a Color supported within the Profile.
func (p Profile) Convert(c color.Color) (cc color.Color) {
	if p <= ASCII {
		return nil
	}
	if p == TrueColor {
		// TrueColor is a passthrough.
		return c
	}

	// Do we have a cached color for this profile and color?
	mu.RLock()
	if c != nil && cache[p] != nil {
		if cc, ok := cache[p][c]; ok {
			mu.RUnlock()
			return cc
		}
	}
	mu.RUnlock()

	// If we don't have a cached color, we need to convert it and cache it.
	defer func() {
		mu.Lock()
		if cc != nil && cache[p] != nil {
			if _, ok := cache[p][c]; !ok {
				cache[p][c] = cc
			}
		}
		mu.Unlock()
	}()

	switch c.(type) {
	case ansi.BasicColor:
		return c

	case ansi.IndexedColor:
		if p == ANSI {
			return ansi.Convert16(c)
		}
		return c

	default:
		switch p {
		case ANSI256:
			return ansi.Convert256(c)
		case ANSI:
			return ansi.Convert16(c)
		default:
			return c
		}
	}
}
