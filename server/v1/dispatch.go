package v1

// Dispatch the command to leader.
func dispatchCommand(c Command, w http.ResponseWriter, req *http.Request, s *server.Server) error {
	return dispatch(c, w, req, s, nameToEtcdURL)
}

// Dispatches a command to a given URL.
func dispatch(c Command, w http.ResponseWriter, req *http.Request, s *server.Server, toURL func(name string) (string, bool)) error {
	r := s.raftServer
	if r.State() == raft.Leader {
		event, err := r.Do(c)
		if err != nil {
			return err
		}

		if event == nil {
			return etcdErr.NewError(300, "Empty result from raft", store.UndefIndex, store.UndefTerm)
		}

		event, _ := event.(*store.Event)
		response := eventToResponse(event)
		b, _ := json.Marshal(response)
		w.WriteHeader(http.StatusOK)
		w.Write(b)

		return nil

	} else {
		leader := r.Leader()

		// No leader available.
		if leader == "" {
			return etcdErr.NewError(300, "", store.UndefIndex, store.UndefTerm)
		}

		url, _ := toURL(leader)
		redirect(url, w, req)

		return nil
	}
}
