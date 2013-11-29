package store

const (
	Get            = "get"
	Create         = "create"
	Set            = "set"
	Update         = "update"
	Delete         = "delete"
	CompareAndSwap = "compareAndSwap"
	Expire         = "expire"
)

type Event struct {
	Action string      `json:"action"`
	Node   *NodeExtern `json:"node,omitempty"`
}

func newEvent(action string, key string, modifiedIndex, createdIndex uint64) *Event {
	n := &NodeExtern{
		Key:           key,
		ModifiedIndex: modifiedIndex,
		CreatedIndex:  createdIndex,
	}

	return &Event{
		Action: action,
		Node:   n,
	}
}

func (e *Event) IsCreated() bool {
	if e.Action == Create {
		return true
	}

	if e.Action == Set && e.Node.PrevValue == "" {
		return true
	}

	return false
}

func (e *Event) Index() uint64 {
	return e.Node.ModifiedIndex
}

// Converts an event object into a response object.
func (event *Event) Response() interface{} {
	if !event.Node.Dir {
		response := &Response{
			Action:     event.Action,
			Key:        event.Node.Key,
			Value:      event.Node.Value,
			PrevValue:  event.Node.PrevValue,
			Index:      event.Node.ModifiedIndex,
			TTL:        event.Node.TTL,
			Expiration: event.Node.Expiration,
		}

		if response.Action == Set {
			if response.PrevValue == "" {
				response.NewKey = true
			}
		}

		if response.Action == CompareAndSwap || response.Action == Create {
			response.Action = "testAndSet"
		}

		return response
	} else {
		responses := make([]*Response, len(event.Node.Nodes))

		for i, node := range event.Node.Nodes {
			responses[i] = &Response{
				Action: event.Action,
				Key:    node.Key,
				Value:  node.Value,
				Dir:    node.Dir,
				Index:  node.ModifiedIndex,
			}
		}
		return responses
	}
}
