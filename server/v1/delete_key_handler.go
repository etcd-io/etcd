package v1

func deleteKeyHandler(w http.ResponseWriter, req *http.Request, e *etcdServer) error {
	key := req.URL.Path[len("/v1/keys/"):]

	debugf("[recv] DELETE %v/v1/keys/%s [%s]", e.url, key, req.RemoteAddr)

	command := &DeleteCommand{
		Key: key,
	}

	return dispatchEtcdCommandV1(command, w, req)
}
