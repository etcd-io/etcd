package foo

type NopReader1 struct{}

func (r *NopReader1) Read(p []byte) (int, error) {
	return 0, nil
}

type NopReader2 struct{}

func (r NopReader2) Read(p []byte) (int, error) {
	return 0, nil
}

type unexportedNopReader {}

func (r *unexportedNopReader) Read(p []byte) (int, error) {
	return 0, nil
}
