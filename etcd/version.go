package etcd

import (
	"fmt"
	"net/http"
)

const (
	releaseVersion = "0.5rc1+git"
)

func versionHandler(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "etcd %s", releaseVersion)
}
