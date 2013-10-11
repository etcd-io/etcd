package v1

// Dispatch the command to leader.
func dispatchCommand(c Command, w http.ResponseWriter, req *http.Request) error {
	return dispatch(c, w, req, nameToEtcdURL)
}

// Dispatches a command to a given URL.
func dispatch(c Command, w http.ResponseWriter, req *http.Request, toURL func(name string) (string, bool)) error {
	r := e.raftServer
	if r.State() == raft.Leader {
		if event, err := r.Do(c); err != nil {
			return err
		} else {
			if event == nil {
				return etcdErr.NewError(300, "Empty result from raft", store.UndefIndex, store.UndefTerm)
			}

			event, _ := event.(*store.Event)

			response := eventToResponse(event)
			bytes, _ := json.Marshal(response)

			w.WriteHeader(http.StatusOK)
			w.Write(bytes)
			return nil

		}

	} else {
		leader := r.Leader()
		// current no leader
		if leader == "" {
			return etcdErr.NewError(300, "", store.UndefIndex, store.UndefTerm)
		}
		url, _ := toURL(leader)

		redirect(url, w, req)

		return nil
	}
}
