package pkg

import "io"

type T struct{}

func (T) Read(b []byte) (int, error) { return 0, nil }
func (T) something() string          { return "non-exported method" }

type V error
type U error

func fn1() {
	var (
		v   interface{}
		err error
	)

	switch v.(type) {
	case io.Reader:
		println("io.Reader")
	case io.ReadCloser: // want `unreachable case clause: io\.Reader will always match before io\.ReadCloser`
		println("io.ReadCloser")
	}

	switch v.(type) {
	case io.Reader:
		println("io.Reader")
	case T: // want `unreachable case clause: io\.Reader will always match before CheckUnreachableTypeCases\.T`
		println("T")
	}

	switch v.(type) {
	case io.Reader:
		println("io.Reader")
	case io.ReadCloser: // want `unreachable case clause: io\.Reader will always match before io\.ReadCloser`
		println("io.ReadCloser")
	case T: // want `unreachable case clause: io\.Reader will always match before CheckUnreachableTypeCases\.T`
		println("T")
	}

	switch v.(type) {
	case io.Reader:
		println("io.Reader")
	case io.ReadCloser, T: // want `unreachable case clause: io\.Reader will always match before io\.ReadCloser`
		println("io.ReadCloser or T")
	}

	switch v.(type) {
	case io.ReadCloser, io.Reader:
		println("io.ReadCloser or io.Reader")
	case T: // want `unreachable case clause: io\.Reader will always match before CheckUnreachableTypeCases\.T`
		println("T")
	}

	switch v.(type) {
	default:
		println("something else")
	case io.Reader:
		println("io.Reader")
	case T: // want `unreachable case clause: io\.Reader will always match before CheckUnreachableTypeCases\.T`
		println("T")
	}

	switch err.(type) {
	case V:
		println("V")
	case U: // want `unreachable case clause: CheckUnreachableTypeCases\.V will always match before CheckUnreachableTypeCases\.U`
		println("U")
	}

	switch err.(type) {
	case U:
		println("U")
	case V: // want `unreachable case clause: CheckUnreachableTypeCases\.U will always match before CheckUnreachableTypeCases\.V`
		println("V")
	}
}

func fn3() {
	var (
		v   interface{}
		err error
	)

	switch v.(type) {
	case T:
		println("T")
	case io.Reader:
		println("io.Reader")
	}

	switch v.(type) {
	case io.ReadCloser:
		println("io.ReadCloser")
	case T:
		println("T")
	}

	switch v.(type) {
	case io.ReadCloser:
		println("io.ReadCloser")
	case io.Reader:
		println("io.Reader")
	}

	switch v.(type) {
	case T:
		println("T")
	}

	switch err.(type) {
	case V, U:
		println("V or U")
	case io.Reader:
		println("io.Reader")
	}

	switch v.(type) {
	default:
		println("something")
	}
}
