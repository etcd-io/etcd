package tagalign

type Option func(*Helper)

// WithSort enable tags sort.
// fixedOrder specify the order of tags, the other tags will be sorted by name.
// Sory is disabled by default.
func WithSort(fixedOrder ...string) Option {
	return func(h *Helper) {
		h.sort = true
		h.fixedTagOrder = fixedOrder
	}
}

// WithAlign configure whether enable tags align.
// Align is enabled by default.
func WithAlign(enabled bool) Option {
	return func(h *Helper) {
		h.align = enabled
	}
}

// WithStrictStyle configure whether enable strict style.
// StrictStyle is disabled by default.
// Note: StrictStyle must be used with WithAlign(true) and WithSort(...) together, or it will be ignored.
func WithStrictStyle() Option {
	return func(h *Helper) {
		h.style = StrictStyle
	}
}
