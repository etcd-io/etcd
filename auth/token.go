package auth

import "errors"

const (
	simpleTokenTyp = "0001"
)

type token struct {
	typ   string
	inner string
}

func tokenDecode(t string) (*token, error) {
	if len(t) < 4 {
		return nil, errors.New("invaild token")
	}
	return &token{
		typ:   t[:4],
		inner: t[4:],
	}, nil
}

func tokenEncode(t *token) string {
	return t.typ + t.inner
}
