package cache

type prefixPred struct{ p string }

func newPrefixPred(p string) func([]byte) bool {
	pp := prefixPred{p: p}
	return pp.match
}

func (pp prefixPred) match(k []byte) bool {
	if pp.p == "" {
		return true
	}
	plen := len(pp.p)
	return len(k) >= plen && string(k[:plen]) == pp.p
}

func (pp prefixPred) Prefix() string { return pp.p }
