package handlers

import (
	"net/http"
)

func HomeHandler(w http.ResponseWriter, r *http.Request) {

	http.Redirect(w, r,
		"https://coreos.com/docs/cluster-management/setup/cluster-discovery/",
		http.StatusMovedPermanently,
	)
}
