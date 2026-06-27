package ll

import "github.com/olekukonko/ll/lx"

// WithHandler sets the handler for the logger as a functional option for configuring
// a new logger instance.
// Example:
//
//	logger := New("app", WithHandler(lh.NewJSONHandler(os.Stdout)))
func WithHandler(handler lx.Handler) Option {
	return func(l *Logger) {
		l.handler = handler
	}
}

// WithTimestamped returns an Option that configures timestamp settings for the logger's existing handler.
// It enables or disables timestamp logging and optionally sets the timestamp format if the handler
// supports the lx.Timestamper interface. If no handler is set, the function has no effect.
// Parameters:
//
//	enable: Boolean to enable or disable timestamp logging
//	format: Optional string(s) to specify the timestamp format
func WithTimestamped(enable bool, format ...string) Option {
	return func(l *Logger) {
		if l.handler != nil { // Check if a handler is set
			// Verify if the handler supports the lx.Timestamper interface
			if h, ok := l.handler.(lx.Timestamper); ok {
				h.Timestamped(enable, format...) // Apply timestamp settings to the handler
			}
		}
	}
}

// WithLevel sets the minimum log level for the logger as a functional option for
// configuring a new logger instance.
// Example:
//
//	logger := New("app", WithLevel(lx.LevelWarn))
func WithLevel(level lx.LevelType) Option {
	return func(l *Logger) {
		l.level = level
	}
}

// WithStyle sets the namespace formatting style for the logger as a functional option
// for configuring a new logger instance.
// Example:
//
//	logger := New("app", WithStyle(lx.NestedPath))
func WithStyle(style lx.StyleType) Option {
	return func(l *Logger) {
		l.style = style
	}
}

// Functional options (can be passed to New() or applied later)
func WithFatalExits(enabled bool) Option {
	return func(l *Logger) {
		l.fatalExits = enabled
	}
}

func WithFatalStack(enabled bool) Option {
	return func(l *Logger) {
		l.fatalStack = enabled
	}
}
