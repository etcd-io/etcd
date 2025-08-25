package offline

import (
	"encoding/binary"
	"fmt"
)

const (
	// revBytesLen is the byte length of a normal revision.
	// First 8 bytes is the revision.main in big-endian format. The 9th byte
	// is a '_'. The last 8 bytes is the revision.sub in big-endian format.
	revBytesLen = 8 + 1 + 8
	// markedRevBytesLen is the byte length of marked revision.
	// The first `revBytesLen` bytes represents a normal revision. The last
	// one byte is the mark.
	markedRevBytesLen      = revBytesLen + 1
	markBytePosition       = markedRevBytesLen - 1
	markTombstone     byte = 't'
)

type Revision struct {
	// Main is the main revision of a set of changes that happen atomically.
	Main int64
	// Sub is the sub revision of a change in a set of changes that happen
	// atomically. Each change has different increasing sub revision in that
	// set.
	Sub int64
}

// BucketKey indicates modification of the key-value space.
// The set of changes that share same main revision changes the key-value space atomically.
type BucketKey struct {
	Revision
	tombstone bool
}

func BytesToBucketKey(bytes []byte) BucketKey {
	if (len(bytes) != revBytesLen) && (len(bytes) != markedRevBytesLen) {
		panic(fmt.Sprintf("invalid revision length: %d", len(bytes)))
	}
	if bytes[8] != '_' {
		panic(fmt.Sprintf("invalid separator in bucket key: %q", bytes[8]))
	}
	main := int64(binary.BigEndian.Uint64(bytes[0:8]))
	sub := int64(binary.BigEndian.Uint64(bytes[9:]))
	if main < 0 || sub < 0 {
		panic(fmt.Sprintf("negative revision: main=%d sub=%d", main, sub))
	}
	return BucketKey{
		Revision: Revision{
			Main: main,
			Sub:  sub,
		},
		tombstone: isTombstone(bytes),
	}
}

func isTombstone(b []byte) bool {
	return len(b) == markedRevBytesLen && b[markBytePosition] == markTombstone
}
