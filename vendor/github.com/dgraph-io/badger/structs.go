package badger

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"

	"github.com/dgraph-io/badger/y"
)

type valuePointer struct {
	Fid    uint32
	Len    uint32
	Offset uint32
}

func (p valuePointer) Less(o valuePointer) bool {
	if p.Fid != o.Fid {
		return p.Fid < o.Fid
	}
	if p.Offset != o.Offset {
		return p.Offset < o.Offset
	}
	return p.Len < o.Len
}

func (p valuePointer) IsZero() bool {
	return p.Fid == 0 && p.Offset == 0 && p.Len == 0
}

const vptrSize = 12

// Encode encodes Pointer into byte buffer.
func (p valuePointer) Encode(b []byte) []byte {
	binary.BigEndian.PutUint32(b[:4], p.Fid)
	binary.BigEndian.PutUint32(b[4:8], p.Len)
	binary.BigEndian.PutUint32(b[8:12], p.Offset)
	return b[:vptrSize]
}

func (p *valuePointer) Decode(b []byte) {
	p.Fid = binary.BigEndian.Uint32(b[:4])
	p.Len = binary.BigEndian.Uint32(b[4:8])
	p.Offset = binary.BigEndian.Uint32(b[8:12])
}

// header is used in value log as a header before Entry.
type header struct {
	klen      uint32
	vlen      uint32
	expiresAt uint64
	meta      byte
	userMeta  byte
}

const (
	headerBufSize = 18
)

func (h header) Encode(out []byte) {
	y.AssertTrue(len(out) >= headerBufSize)
	binary.BigEndian.PutUint32(out[0:4], h.klen)
	binary.BigEndian.PutUint32(out[4:8], h.vlen)
	binary.BigEndian.PutUint64(out[8:16], h.expiresAt)
	out[16] = h.meta
	out[17] = h.userMeta
}

// Decodes h from buf.
func (h *header) Decode(buf []byte) {
	h.klen = binary.BigEndian.Uint32(buf[0:4])
	h.vlen = binary.BigEndian.Uint32(buf[4:8])
	h.expiresAt = binary.BigEndian.Uint64(buf[8:16])
	h.meta = buf[16]
	h.userMeta = buf[17]
}

// Entry provides Key, Value, UserMeta and ExpiresAt. This struct can be used by the user to set data.
type Entry struct {
	Key       []byte
	Value     []byte
	UserMeta  byte
	ExpiresAt uint64 // time.Unix
	meta      byte

	// Fields maintained internally.
	offset uint32
}

func (e *Entry) estimateSize(threshold int) int {
	if len(e.Value) < threshold {
		return len(e.Key) + len(e.Value) + 2 // Meta, UserMeta
	}
	return len(e.Key) + 12 + 2 // 12 for ValuePointer, 2 for metas.
}

// Encodes e to buf. Returns number of bytes written.
func encodeEntry(e *Entry, buf *bytes.Buffer) (int, error) {
	h := header{
		klen:      uint32(len(e.Key)),
		vlen:      uint32(len(e.Value)),
		expiresAt: e.ExpiresAt,
		meta:      e.meta,
		userMeta:  e.UserMeta,
	}

	var headerEnc [headerBufSize]byte
	h.Encode(headerEnc[:])

	hash := crc32.New(y.CastagnoliCrcTable)

	buf.Write(headerEnc[:])
	hash.Write(headerEnc[:])

	buf.Write(e.Key)
	hash.Write(e.Key)

	buf.Write(e.Value)
	hash.Write(e.Value)

	var crcBuf [4]byte
	binary.BigEndian.PutUint32(crcBuf[:], hash.Sum32())
	buf.Write(crcBuf[:])

	return len(headerEnc) + len(e.Key) + len(e.Value) + len(crcBuf), nil
}

func (e Entry) print(prefix string) {
	fmt.Printf("%s Key: %s Meta: %d UserMeta: %d Offset: %d len(val)=%d",
		prefix, e.Key, e.meta, e.UserMeta, e.offset, len(e.Value))
}
