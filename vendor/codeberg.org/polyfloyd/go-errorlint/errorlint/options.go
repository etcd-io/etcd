package errorlint

type Option func()

func WithAllowedErrors(ap []AllowPair) Option {
	return func() {
		allowedMapAppend(ap)
	}
}

func WithAllowedWildcard(ap []AllowPair) Option {
	return func() {
		allowedWildcardAppend(ap)
	}
}

func WithComparison(enabled bool) Option {
	return func() {
		checkComparison = enabled
	}
}

func WithAsserts(enabled bool) Option {
	return func() {
		checkAsserts = enabled
	}
}
