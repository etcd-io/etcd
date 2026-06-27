package shorten

type Logger interface {
	Debug(msg string, args ...any)
}

type noopLogger struct{}

func (n *noopLogger) Debug(_ string, _ ...any) {}
