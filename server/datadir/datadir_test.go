package datadir_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.etcd.io/etcd/server/v3/datadir"
)

func TestToBackendFileName(t *testing.T) {
	result := datadir.ToBackendFileName("/dir/data-dir")
	assert.Equal(t, "/dir/data-dir/member/snap/db", result)
}

func TestToMemberDir(t *testing.T) {
	result := datadir.ToMemberDir("/dir/data-dir")
	assert.Equal(t, "/dir/data-dir/member", result)
}

func TestToSnapDir(t *testing.T) {
	result := datadir.ToSnapDir("/dir/data-dir")
	assert.Equal(t, "/dir/data-dir/member/snap", result)
}

func TestToWalDir(t *testing.T) {
	result := datadir.ToWalDir("/dir/data-dir")
	assert.Equal(t, "/dir/data-dir/member/wal", result)
}

func TestToWalDirSlash(t *testing.T) {
	result := datadir.ToWalDir("/dir/data-dir/")
	assert.Equal(t, "/dir/data-dir/member/wal", result)
}
