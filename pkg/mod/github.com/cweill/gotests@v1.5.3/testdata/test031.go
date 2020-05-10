package testdata

import "io"

func (b *Bar) Write(w io.Writer) error { return nil }

func Write(w io.Writer, data string) error { return nil }

func MultiWrite(w1, w2 io.Writer, data string) (int, string, error) { return 0, "", nil }
