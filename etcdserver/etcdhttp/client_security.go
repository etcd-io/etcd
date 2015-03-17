// Copyright 2015 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package etcdhttp

import (
	"encoding/json"
	"log"
	"net/http"
	"path"
	"strings"

	"github.com/coreos/etcd/etcdserver"
	"github.com/coreos/etcd/etcdserver/etcdhttp/httptypes"
	"github.com/coreos/etcd/etcdserver/security"
)

type securityHandler struct {
	sec         *security.Store
	clusterInfo etcdserver.ClusterInfo
}

func hasWriteRootAccess(sec *security.Store, r *http.Request) bool {
	if r.Method == "GET" || r.Method == "HEAD" {
		return true
	}
	return hasRootAccess(sec, r)
}

func hasRootAccess(sec *security.Store, r *http.Request) bool {
	if sec == nil {
		// No store means no security avaliable, eg, tests.
		return true
	}
	if !sec.SecurityEnabled() {
		return true
	}
	username, password, ok := r.BasicAuth()
	if !ok {
		return false
	}
	if username != "root" {
		log.Printf("security: Attempting to use user %s for resource that requires root.", username)
		return false
	}
	root, err := sec.GetUser("root")
	if err != nil {
		return false
	}
	ok = root.CheckPassword(password)
	if !ok {
		log.Printf("security: Wrong password for user %s", username)
	}
	return ok
}

func hasKeyPrefixAccess(sec *security.Store, r *http.Request, key string) bool {
	if sec == nil {
		// No store means no security avaliable, eg, tests.
		return true
	}
	if !sec.SecurityEnabled() {
		return true
	}
	username, password, ok := r.BasicAuth()
	if !ok {
		return false
	}
	user, err := sec.GetUser(username)
	if err != nil {
		log.Printf("security: No such user: %s.", username)
		return false
	}
	authAsUser := user.CheckPassword(password)
	if !authAsUser {
		log.Printf("security: Incorrect password for user: %s.", username)
		return false
	}
	if user.User == "root" {
		return true
	}
	writeAccess := r.Method != "GET" && r.Method != "HEAD"
	for _, roleName := range user.Roles {
		role, err := sec.GetRole(roleName)
		if err != nil {
			continue
		}
		if role.HasKeyAccess(key, writeAccess) {
			return true
		}
	}
	log.Printf("security: Invalid access for user %s on key %s.", username, key)
	return false
}

func writeNoAuth(w http.ResponseWriter) {
	herr := httptypes.NewHTTPError(http.StatusUnauthorized, "Insufficient credentials")
	herr.WriteTo(w)
}

func handleSecurity(mux *http.ServeMux, sh *securityHandler) {
	mux.HandleFunc(securityPrefix+"/roles", sh.baseRoles)
	mux.HandleFunc(securityPrefix+"/roles/", sh.handleRoles)
	mux.HandleFunc(securityPrefix+"/users", sh.baseUsers)
	mux.HandleFunc(securityPrefix+"/users/", sh.handleUsers)
	mux.HandleFunc(securityPrefix+"/enable", sh.enableDisable)
}

func (sh *securityHandler) baseRoles(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r.Method, "GET") {
		return
	}
	if !hasRootAccess(sh.sec, r) {
		writeNoAuth(w)
		return
	}
	w.Header().Set("X-Etcd-Cluster-ID", sh.clusterInfo.ID().String())
	w.Header().Set("Content-Type", "application/json")
	var rolesCollections struct {
		Roles []string `json:"roles"`
	}
	roles, err := sh.sec.AllRoles()
	if err != nil {
		writeError(w, err)
		return
	}

	rolesCollections.Roles = roles
	err = json.NewEncoder(w).Encode(rolesCollections)
	if err != nil {
		log.Println("etcdhttp: baseRoles error encoding on", r.URL)
	}
}

func (sh *securityHandler) handleRoles(w http.ResponseWriter, r *http.Request) {
	subpath := path.Clean(r.URL.Path[len(securityPrefix):])
	// Split "/roles/rolename/command".
	// First item is an empty string, second is "roles"
	pieces := strings.Split(subpath, "/")
	if len(pieces) != 3 {
	}
	sh.forRole(w, r, pieces[2])
}

func (sh *securityHandler) forRole(w http.ResponseWriter, r *http.Request, role string) {
	if !allowMethod(w, r.Method, "GET", "PUT", "DELETE") {
		return
	}
	if !hasRootAccess(sh.sec, r) {
		writeNoAuth(w)
		return
	}
	w.Header().Set("X-Etcd-Cluster-ID", sh.clusterInfo.ID().String())

	switch r.Method {
	case "GET":
		w.Header().Set("Content-Type", "application/json")
		data, err := sh.sec.GetRole(role)
		if err != nil {
			writeError(w, err)
			return
		}
		err = json.NewEncoder(w).Encode(data)
		if err != nil {
			log.Println("etcdhttp: forRole error encoding on", r.URL)
			return
		}
		return
	case "PUT":
		var in security.Role
		err := json.NewDecoder(r.Body).Decode(&in)
		if err != nil {
			writeError(w, httptypes.NewHTTPError(http.StatusBadRequest, "Invalid JSON in request body."))
			return
		}
		if in.Role != role {
			writeError(w, httptypes.NewHTTPError(400, "Role JSON name does not match the name in the URL"))
			return
		}
		newrole, err := sh.sec.CreateOrUpdateRole(in)
		if err != nil {
			writeError(w, err)
			return
		}
		w.WriteHeader(http.StatusCreated)
		err = json.NewEncoder(w).Encode(newrole)
		if err != nil {
			log.Println("etcdhttp: forRole error encoding on", r.URL)
			return
		}
		return
	case "DELETE":
		err := sh.sec.DeleteRole(role)
		if err != nil {
			writeError(w, err)
			return
		}
	}
}

func (sh *securityHandler) baseUsers(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r.Method, "GET") {
		return
	}
	if !hasRootAccess(sh.sec, r) {
		writeNoAuth(w)
		return
	}
	w.Header().Set("X-Etcd-Cluster-ID", sh.clusterInfo.ID().String())
	w.Header().Set("Content-Type", "application/json")
	var usersCollections struct {
		Users []string `json:"users"`
	}
	users, err := sh.sec.AllUsers()
	if err != nil {
		writeError(w, err)
		return
	}

	usersCollections.Users = users
	err = json.NewEncoder(w).Encode(usersCollections)
	if err != nil {
		log.Println("etcdhttp: baseUsers error encoding on", r.URL)
	}
}

func (sh *securityHandler) handleUsers(w http.ResponseWriter, r *http.Request) {
	subpath := path.Clean(r.URL.Path[len(securityPrefix):])
	// Split "/users/username/command".
	// First item is an empty string, second is "users"
	pieces := strings.Split(subpath, "/")
	if len(pieces) != 3 {
		writeError(w, httptypes.NewHTTPError(http.StatusBadRequest, "Invalid path"))
		return
	}
	sh.forUser(w, r, pieces[2])
}

func (sh *securityHandler) forUser(w http.ResponseWriter, r *http.Request, user string) {
	if !allowMethod(w, r.Method, "GET", "PUT", "DELETE") {
		return
	}
	if !hasRootAccess(sh.sec, r) {
		writeNoAuth(w)
		return
	}
	w.Header().Set("X-Etcd-Cluster-ID", sh.clusterInfo.ID().String())

	switch r.Method {
	case "GET":
		w.Header().Set("Content-Type", "application/json")
		u, err := sh.sec.GetUser(user)
		if err != nil {
			writeError(w, err)
			return
		}
		u.Password = ""

		err = json.NewEncoder(w).Encode(u)
		if err != nil {
			log.Println("etcdhttp: forUser error encoding on", r.URL)
			return
		}
		return
	case "PUT":
		var u security.User
		err := json.NewDecoder(r.Body).Decode(&u)
		if err != nil {
			writeError(w, httptypes.NewHTTPError(http.StatusBadRequest, "Invalid JSON in request body."))
			return
		}
		if u.User != user {
			writeError(w, httptypes.NewHTTPError(400, "User JSON name does not match the name in the URL"))
			return
		}
		newuser, err := sh.sec.CreateOrUpdateUser(u)
		if err != nil {
			writeError(w, err)
			return
		}
		newuser.Password = ""

		w.WriteHeader(http.StatusCreated)
		err = json.NewEncoder(w).Encode(newuser)
		if err != nil {
			log.Println("etcdhttp: forUser error encoding on", r.URL)
			return
		}
		return
	case "DELETE":
		err := sh.sec.DeleteUser(user)
		if err != nil {
			writeError(w, err)
			return
		}
	}
}

type enabled struct {
	Enabled bool `json:"enabled"`
}

func (sh *securityHandler) enableDisable(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r.Method, "GET", "PUT", "DELETE") {
		return
	}
	if !hasWriteRootAccess(sh.sec, r) {
		writeNoAuth(w)
		return
	}
	w.Header().Set("X-Etcd-Cluster-ID", sh.clusterInfo.ID().String())
	w.Header().Set("Content-Type", "application/json")
	isEnabled := sh.sec.SecurityEnabled()
	switch r.Method {
	case "GET":
		jsonDict := enabled{isEnabled}
		err := json.NewEncoder(w).Encode(jsonDict)
		if err != nil {
			log.Println("etcdhttp: error encoding security state on", r.URL)
		}
	case "PUT":
		var in security.User
		err := json.NewDecoder(r.Body).Decode(&in)
		if err != nil {
			writeError(w, httptypes.NewHTTPError(http.StatusBadRequest, "Invalid JSON in request body."))
			return
		}
		if in.User != "root" {
			writeError(w, httptypes.NewHTTPError(http.StatusBadRequest, "Need to create root user"))
			return
		}
		err = sh.sec.EnableSecurity(in)
		if err != nil {
			writeError(w, err)
			return
		}
	case "DELETE":
		err := sh.sec.DisableSecurity()
		if err != nil {
			writeError(w, err)
			return
		}
	}
}
