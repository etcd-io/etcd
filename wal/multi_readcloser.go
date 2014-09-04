package wal

import "io"

type multiReadCloser struct {
	closers []io.Closer
	reader  io.Reader
}

func (mc *multiReadCloser) Close() error {
	var err error
	for i := range mc.closers {
		err = mc.closers[i].Close()
	}
	return err
}

func (mc *multiReadCloser) Read(p []byte) (int, error) {
	return mc.reader.Read(p)
}

func MultiReadCloser(readClosers ...io.ReadCloser) io.ReadCloser {
	cs := make([]io.Closer, len(readClosers))
	rs := make([]io.Reader, len(readClosers))
	for i := range readClosers {
		cs[i] = readClosers[i]
		rs[i] = readClosers[i]
	}
	r := io.MultiReader(rs...)
	return &multiReadCloser{cs, r}
}
