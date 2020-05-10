package pkg

type T1 struct{}
type T2 struct{}
type T3 struct{}
type T4 struct{}

func (T1) Write(b []byte) (int, error) {
	b = append(b, '\n') // want `io\.Writer\.Write must not modify the provided buffer`
	_ = b
	return 0, nil
}

func (T2) Write(b []byte) (int, error) {
	b[0] = 0 // want `io\.Writer\.Write must not modify the provided buffer`
	return 0, nil
}

func (T3) Write(b []byte) string {
	b[0] = 0
	return ""
}

func (T4) Write(b []byte, r byte) (int, error) {
	b[0] = r
	return 0, nil
}
