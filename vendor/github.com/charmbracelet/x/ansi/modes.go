package ansi

// Modes represents the terminal modes that can be set or reset. By default,
// all modes are [ModeNotRecognized].
type Modes map[Mode]ModeSetting

// Get returns the setting of a terminal mode. If the mode is not set, it
// returns [ModeNotRecognized].
func (m Modes) Get(mode Mode) ModeSetting {
	return m[mode]
}

// Delete deletes a terminal mode. This has the same effect as setting the mode
// to [ModeNotRecognized].
func (m Modes) Delete(mode Mode) {
	delete(m, mode)
}

// Set sets a terminal mode to [ModeSet].
func (m Modes) Set(modes ...Mode) {
	for _, mode := range modes {
		m[mode] = ModeSet
	}
}

// PermanentlySet sets a terminal mode to [ModePermanentlySet].
func (m Modes) PermanentlySet(modes ...Mode) {
	for _, mode := range modes {
		m[mode] = ModePermanentlySet
	}
}

// Reset sets a terminal mode to [ModeReset].
func (m Modes) Reset(modes ...Mode) {
	for _, mode := range modes {
		m[mode] = ModeReset
	}
}

// PermanentlyReset sets a terminal mode to [ModePermanentlyReset].
func (m Modes) PermanentlyReset(modes ...Mode) {
	for _, mode := range modes {
		m[mode] = ModePermanentlyReset
	}
}

// IsSet returns true if the mode is set to [ModeSet] or [ModePermanentlySet].
func (m Modes) IsSet(mode Mode) bool {
	return m[mode].IsSet()
}

// IsPermanentlySet returns true if the mode is set to [ModePermanentlySet].
func (m Modes) IsPermanentlySet(mode Mode) bool {
	return m[mode].IsPermanentlySet()
}

// IsReset returns true if the mode is set to [ModeReset] or [ModePermanentlyReset].
func (m Modes) IsReset(mode Mode) bool {
	return m[mode].IsReset()
}

// IsPermanentlyReset returns true if the mode is set to [ModePermanentlyReset].
func (m Modes) IsPermanentlyReset(mode Mode) bool {
	return m[mode].IsPermanentlyReset()
}
