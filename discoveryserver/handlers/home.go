package handlers

import (
	"net/http"
)

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r,
		"https://github.com/etcd-io/etcd/blob/master/Documentation/v2/clustering.md#public-etcd-discovery-service",
		http.StatusMovedPermanently,
	)
}
