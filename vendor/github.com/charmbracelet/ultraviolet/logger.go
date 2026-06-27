package uv

// Logger is a simple logger interface.
type Logger interface {
	Printf(format string, v ...interface{})
}
