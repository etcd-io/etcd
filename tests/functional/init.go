package test

import (
	"go/build"
	"os"
	"path/filepath"
)

var EtcdBinPath string

func init() {
	// Initialize the 'etcd' binary path or default it to the etcd diretory.
	EtcdBinPath = os.Getenv("ETCD_BIN_PATH")
	if EtcdBinPath == "" {
		EtcdBinPath = filepath.Join(build.Default.GOPATH, "src", "github.com", "coreos", "etcd", "etcd")
	}
}
