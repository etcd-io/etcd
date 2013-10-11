package v1

import (
	"github.com/coreos/etcd/server"
	"github.com/gorilla/mux"
)

// Installs the routes for version 1 of the API on to a server.
func Install(s *server.Server) {
	s.HandleFunc("/v1/keys/{key:.*}", getKeyHandler).Methods("GET")
	s.HandleFunc("/v1/keys/{key:.*}", setKeyHandler).Methods("POST", "PUT")
	s.HandleFunc("/v1/keys/{key:.*}", deleteKeyHandler).Methods("DELETE")

	s.HandleFunc("/v1/watch/{key:.*}", watchKeyHandler).Methods("GET", "POST")
}
