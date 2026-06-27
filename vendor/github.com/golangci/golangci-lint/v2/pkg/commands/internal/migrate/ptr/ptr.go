package ptr

func Deref[T any](v *T) T {
	if v == nil {
		var zero T
		return zero
	}

	return *v
}

func Pointer[T any](v T) *T { return &v }
