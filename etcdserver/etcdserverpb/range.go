package etcdserverpb

// See github.com/coreos/etcd/wildcard/wildcard.go
// for the definition of the BegEndRange interface.
// The methods allow a uniform treatment of ranges.

// GetBeg implements part of the BegEndRange interface.
func (r *RangeRequest) GetBeg() []byte {
	return r.Key
}

// SetBeg implements part of the BegEndRange interface.
func (r *RangeRequest) SetBeg(beg []byte) {
	r.Key = beg
}

// GetEnd implements part of the BegEndRange interface.
func (r *RangeRequest) GetEnd() []byte {
	return r.RangeEnd
}

// SetEnd implements part of the BegEndRange interface.
func (r *RangeRequest) SetEnd(end []byte) {
	r.RangeEnd = end
}

// GetBeg implements part of the BegEndRange interface.
func (r *DeleteRangeRequest) GetBeg() []byte {
	return r.Key
}

// SetBeg implements part of the BegEndRange interface.
func (r *DeleteRangeRequest) SetBeg(beg []byte) {
	r.Key = beg
}

// GetEnd implements part of the BegEndRange interface.
func (r *DeleteRangeRequest) GetEnd() []byte {
	return r.RangeEnd
}

// SetEnd implements part of the BegEndRange interface.
func (r *DeleteRangeRequest) SetEnd(end []byte) {
	r.RangeEnd = end
}
