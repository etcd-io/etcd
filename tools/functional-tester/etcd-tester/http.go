package main

import (
	"encoding/json"
	"net/http"
)

type statusHandler struct {
	status *Status
}

func (sh statusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	en := json.NewEncoder(w)
	err := en.Encode(sh.status.get())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
