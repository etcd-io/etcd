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
