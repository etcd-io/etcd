package v1

// Converts an event object into a response object.
func eventToResponse(event *store.Event) interface{} {
	if !event.Dir {
		response := &store.Response{
			Action:     event.Action,
			Key:        event.Key,
			Value:      event.Value,
			PrevValue:  event.PrevValue,
			Index:      event.Index,
			TTL:        event.TTL,
			Expiration: event.Expiration,
		}

		if response.Action == store.Create || response.Action == store.Update {
			response.Action = "set"
			if response.PrevValue == "" {
				response.NewKey = true
			}
		}

		return response
	} else {
		responses := make([]*store.Response, len(event.KVPairs))

		for i, kv := range event.KVPairs {
			responses[i] = &store.Response{
				Action: event.Action,
				Key:    kv.Key,
				Value:  kv.Value,
				Dir:    kv.Dir,
				Index:  event.Index,
			}
		}
		return responses
	}
}
