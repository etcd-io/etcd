package store

const (
	Get              = "get"
	Create           = "create"
	Set              = "set"
	Update           = "update"
	Delete           = "delete"
	CompareAndSwap   = "compareAndSwap"
	CompareAndDelete = "compareAndDelete"
	Expire           = "expire"
)

type Event struct {
	Action   string      `json:"action"`
	Node     *NodeExtern `json:"node,omitempty"`
	PrevNode *NodeExtern `json:"prevNode,omitempty"`
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

	if e.Action == Set && e.PrevNode == nil {
		return true
	}

	return false
}

func (e *Event) Index() uint64 {
	return e.Node.ModifiedIndex
}

// Converts an event object into a response object.
func (event *Event) Response(currentIndex uint64) interface{} {
	if !event.Node.Dir {
		response := &Response{
			Action:     event.Action,
			Key:        event.Node.Key,
			Value:      event.Node.Value,
			Index:      event.Node.ModifiedIndex,
			TTL:        event.Node.TTL,
			Expiration: event.Node.Expiration,
		}

		if event.PrevNode != nil {
			response.PrevValue = event.PrevNode.Value
		}

		if currentIndex != 0 {
			response.Index = currentIndex
		}

		if response.Action == Set {
			if response.PrevValue == nil {
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

			if currentIndex != 0 {
				responses[i].Index = currentIndex
			}
		}
		return responses
	}
}
